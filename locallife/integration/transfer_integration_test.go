package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	api "github.com/merrydance/locallife/api"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

const testCasbinModelDef = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && r.act == p.act
`

const testCasbinPolicyDef = `
# Role inheritance
g, customer, customer

# Customer policies
p, customer, /v1/dining-sessions/*, POST
`

var (
	integrationOnce       sync.Once
	integrationStore      *db.SQLStore
	integrationServer     *api.Server
	integrationPool       *pgxpool.Pool
	integrationTokenMaker token.Maker
)

func TestTransferDiningSessionTableIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()

	region := createIntegrationRegion(t, store)
	owner := createIntegrationUser(t, store)
	merchant := createIntegrationMerchant(t, store, owner.ID, region.ID)
	user := createIntegrationUser(t, store)

	accessCode := "1234"
	accessHash, err := util.HashPassword(accessCode)
	require.NoError(t, err)

	fromTable := createIntegrationTable(t, store, merchant.ID, pgtype.Text{Valid: false})
	toTable := createIntegrationTable(t, store, merchant.ID, pgtype.Text{String: accessHash, Valid: true})

	session, err := store.CreateDiningSession(ctx, db.CreateDiningSessionParams{
		MerchantID:    merchant.ID,
		TableID:       fromTable.ID,
		ReservationID: pgtype.Int8{Valid: false},
		UserID:        user.ID,
		ActiveOrderID: pgtype.Int8{Valid: false},
		Status:        "open",
	})
	require.NoError(t, err)

	_, err = store.UpdateTableStatus(ctx, db.UpdateTableStatusParams{
		ID:                   fromTable.ID,
		Status:               "occupied",
		CurrentReservationID: pgtype.Int8{},
	})
	require.NoError(t, err)

	body, err := json.Marshal(map[string]any{
		"to_table_id": toTable.ID,
		"table_code":  accessCode,
		"reason":      "集成测试换桌",
	})
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/dining-sessions/%d/transfer-table", session.ID)
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, integrationTokenMaker, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response transferDiningSessionResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, session.ID, response.Session.ID)
	require.Equal(t, toTable.ID, response.ToTable.ID)

	updatedSession, err := store.GetDiningSession(ctx, session.ID)
	require.NoError(t, err)
	require.Equal(t, toTable.ID, updatedSession.TableID)

	updatedTo, err := store.GetTable(ctx, toTable.ID)
	require.NoError(t, err)
	require.Equal(t, "occupied", updatedTo.Status)

	updatedFrom, err := store.GetTable(ctx, fromTable.ID)
	require.NoError(t, err)
	require.Equal(t, "available", updatedFrom.Status)

	var logCount int
	err = integrationPool.QueryRow(ctx, `SELECT COUNT(1) FROM table_transfer_logs WHERE dining_session_id = $1`, session.ID).Scan(&logCount)
	require.NoError(t, err)
	require.Equal(t, 1, logCount)
}

func initIntegrationServer(t *testing.T) (*api.Server, *db.SQLStore) {
	integrationOnce.Do(func() {
		enforceTestCasbin(t)
		ensureIntegrationDatabase(t)
		connPool, err := pgxpool.New(context.Background(), integrationDBSource())
		require.NoError(t, err)
		integrationPool = connPool
		integrationStore = db.NewStore(connPool).(*db.SQLStore)

		config := util.Config{
			TokenSymmetricKey:   util.RandomString(32),
			AccessTokenDuration: time.Minute,
		}
		integrationTokenMaker, err = token.NewPasetoMaker(config.TokenSymmetricKey)
		require.NoError(t, err)

		server, err := api.NewServer(config, integrationStore, nil, nil)
		require.NoError(t, err)
		integrationServer = server
	})

	return integrationServer, integrationStore
}

func enforceTestCasbin(t *testing.T) {
	if api.GetGlobalCasbinEnforcer() != nil {
		return
	}
	enforcer, err := api.NewCasbinEnforcerFromString(testCasbinModelDef, testCasbinPolicyDef)
	require.NoError(t, err)
	api.SetGlobalCasbinEnforcer(enforcer)
}

func resetIntegrationData(t *testing.T) {
	_, err := integrationPool.Exec(context.Background(), `
		TRUNCATE TABLE
			table_transfer_logs,
			billing_group_orders,
			billing_group_members,
			billing_groups,
			dining_sessions,
			table_reservations,
			tables,
			merchants,
			users,
			regions
		RESTART IDENTITY CASCADE;
	`)
	require.NoError(t, err)
}

func ensureIntegrationDatabase(t *testing.T) {
	adminSource := integrationAdminSource()
	dbName := integrationDBName()

	adminPool, err := pgxpool.New(context.Background(), adminSource)
	require.NoError(t, err)
	defer adminPool.Close()

	var exists bool
	err = adminPool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, dbName).Scan(&exists)
	require.NoError(t, err)
	if !exists {
		_, err = adminPool.Exec(context.Background(), fmt.Sprintf(`CREATE DATABASE %s`, dbName))
		require.NoError(t, err)
	}

	migrationURL := integrationMigrationURL(t)
	mig, err := migrate.New(migrationURL, integrationDBSource())
	require.NoError(t, err)
	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		require.NoError(t, err)
	}
}

func integrationDBSource() string {
	if v := os.Getenv("INTEGRATION_DB_SOURCE"); strings.TrimSpace(v) != "" {
		return v
	}
	return "postgresql:///locallife_test?sslmode=disable&host=/var/run/postgresql"
}

func integrationAdminSource() string {
	if v := os.Getenv("INTEGRATION_ADMIN_SOURCE"); strings.TrimSpace(v) != "" {
		return v
	}
	return "postgresql:///postgres?sslmode=disable&host=/var/run/postgresql"
}

func integrationDBName() string {
	if v := os.Getenv("INTEGRATION_DB_NAME"); strings.TrimSpace(v) != "" {
		return v
	}
	parsed, err := url.Parse(integrationDBSource())
	if err == nil {
		name := strings.TrimPrefix(parsed.Path, "/")
		if name != "" {
			return name
		}
	}
	return "locallife_test"
}

func integrationMigrationURL(t *testing.T) string {
	if v := os.Getenv("INTEGRATION_MIGRATION_URL"); strings.TrimSpace(v) != "" {
		return v
	}
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	base := filepath.Dir(file)
	path := filepath.Join(base, "..", "db", "migration")
	absPath, err := filepath.Abs(path)
	require.NoError(t, err)
	return "file://" + absPath
}

func createIntegrationRegion(t *testing.T, store *db.SQLStore) db.Region {
	region, err := store.CreateRegion(context.Background(), db.CreateRegionParams{
		Code:      util.RandomString(6),
		Name:      "集成测试区域",
		Level:     1,
		ParentID:  pgtype.Int8{Valid: false},
		Longitude: pgtype.Numeric{Valid: false},
		Latitude:  pgtype.Numeric{Valid: false},
	})
	require.NoError(t, err)
	return region
}

func createIntegrationUser(t *testing.T, store *db.SQLStore) db.User {
	phone := fmt.Sprintf("138%08d", util.RandomInt(0, 99999999))
	user, err := store.CreateUser(context.Background(), db.CreateUserParams{
		WechatOpenid:  util.RandomString(16),
		WechatUnionid: pgtype.Text{String: util.RandomString(16), Valid: true},
		FullName:      util.RandomString(6),
		Phone:         pgtype.Text{String: phone, Valid: true},
		AvatarUrl:     pgtype.Text{String: "https://example.com/avatar.png", Valid: true},
	})
	require.NoError(t, err)
	return user
}

func createIntegrationMerchant(t *testing.T, store *db.SQLStore, ownerID, regionID int64) db.Merchant {
	merchant, err := store.CreateMerchant(context.Background(), db.CreateMerchantParams{
		OwnerUserID:     ownerID,
		Name:            "集成测试餐厅",
		Description:     pgtype.Text{String: "集成测试", Valid: true},
		LogoUrl:         pgtype.Text{String: "https://example.com/logo.png", Valid: true},
		Phone:           "13800138000",
		Address:         "测试地址",
		Latitude:        pgtype.Numeric{Valid: false},
		Longitude:       pgtype.Numeric{Valid: false},
		Status:          "approved",
		ApplicationData: []byte("{}"),
		RegionID:        regionID,
	})
	require.NoError(t, err)
	return merchant
}

func createIntegrationTable(t *testing.T, store *db.SQLStore, merchantID int64, accessCodeHash pgtype.Text) db.Table {
	table, err := store.CreateTable(context.Background(), db.CreateTableParams{
		MerchantID:     merchantID,
		TableNo:        util.RandomString(4),
		TableType:      "table",
		Capacity:       4,
		Description:    pgtype.Text{String: "集成测试桌台", Valid: true},
		MinimumSpend:   pgtype.Int8{Valid: false},
		QrCodeUrl:      pgtype.Text{Valid: false},
		Status:         "available",
		AccessCodeHash: accessCodeHash,
	})
	require.NoError(t, err)
	return table
}

type apiTestEnvelope struct {
	Code    *int            `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func requireUnmarshalAPIResponseData(t *testing.T, body []byte, target any) {
	var env apiTestEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Code != nil {
		if len(env.Data) == 0 {
			require.NoError(t, json.Unmarshal([]byte("null"), target))
			return
		}
		require.NoError(t, json.Unmarshal(env.Data, target))
		return
	}
	require.NoError(t, json.Unmarshal(body, target))
}

func addAuthorization(t *testing.T, request *http.Request, tokenMaker token.Maker, userID int64, duration time.Duration) {
	accessToken, _, err := tokenMaker.CreateToken(userID, duration, token.TokenTypeAccessToken)
	require.NoError(t, err)
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
}

type transferDiningSessionResponse struct {
	Session struct {
		ID int64 `json:"id"`
	} `json:"session"`
	ToTable struct {
		ID int64 `json:"id"`
	} `json:"to_table"`
}
