package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

var membershipSettingsMigrationFixtureCounter atomic.Int64

func TestMembershipSettingsSceneMigrationsCleanDatabaseContract(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 253)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	balanceDefault, bonusDefault := membershipSettingsSceneDefaults(t, pool)
	require.Equal(t, "ARRAY['dine_in'::text, 'takeaway'::text]", balanceDefault)
	require.Equal(t, "ARRAY['dine_in'::text]", bonusDefault)
	requireMembershipSettingsSceneConstraints(t, pool)

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO merchant_membership_settings (merchant_id)
		VALUES ($1)
	`, merchantID)
	require.NoError(t, err)

	var balanceScenes, bonusScenes []string
	err = pool.QueryRow(context.Background(), `
		SELECT balance_usable_scenes, bonus_usable_scenes
		FROM merchant_membership_settings
		WHERE merchant_id = $1
	`, merchantID).Scan(&balanceScenes, &bonusScenes)
	require.NoError(t, err)
	require.Equal(t, []string{"dine_in", "takeaway"}, balanceScenes)
	require.Equal(t, []string{"dine_in"}, bonusScenes)

	_, err = pool.Exec(context.Background(), `
		INSERT INTO merchant_membership_settings (
			merchant_id,
			balance_usable_scenes,
			bonus_usable_scenes
		) VALUES ($1, ARRAY['takeout']::TEXT[], ARRAY['dine_in']::TEXT[])
	`, insertMembershipSettingsMigrationMerchant(t, pool))
	requireMembershipSettingsSceneConstraintError(t, err)
}

func TestMembershipSettingsSceneMigration252CleansHistoricalDirtyRows(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 251)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	firstMerchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	secondMerchantID := insertMembershipSettingsMigrationMerchant(t, pool)

	_, err := pool.Exec(context.Background(), `
		INSERT INTO merchant_membership_settings (
			merchant_id,
			balance_usable_scenes,
			bonus_usable_scenes
		) VALUES
			($1, ARRAY['dine_in', 'takeout', NULL, 'takeaway', 'reservation']::TEXT[], ARRAY['reservation', 'dine_in', NULL]::TEXT[]),
			($2, ARRAY['takeout', 'reservation', NULL]::TEXT[], ARRAY['takeout', NULL]::TEXT[])
	`, firstMerchantID, secondMerchantID)
	require.NoError(t, err)

	runMembershipSettingsMigrations(t, dbSource, 252)
	requireMembershipSettingsSceneConstraints(t, pool)

	rows, err := pool.Query(context.Background(), `
		SELECT merchant_id, balance_usable_scenes, bonus_usable_scenes
		FROM merchant_membership_settings
		ORDER BY merchant_id
	`)
	require.NoError(t, err)
	defer rows.Close()

	got := map[int64]struct {
		balance []string
		bonus   []string
	}{}
	for rows.Next() {
		var merchantID int64
		var balanceScenes []string
		var bonusScenes []string
		require.NoError(t, rows.Scan(&merchantID, &balanceScenes, &bonusScenes))
		got[merchantID] = struct {
			balance []string
			bonus   []string
		}{balance: balanceScenes, bonus: bonusScenes}
	}
	require.NoError(t, rows.Err())
	require.Len(t, got, 2)
	require.Contains(t, got, firstMerchantID)
	require.Contains(t, got, secondMerchantID)
	require.Equal(t, []string{"dine_in", "takeaway"}, got[firstMerchantID].balance)
	require.Equal(t, []string{"dine_in"}, got[firstMerchantID].bonus)
	require.Empty(t, got[secondMerchantID].balance)
	require.Empty(t, got[secondMerchantID].bonus)

	_, err = pool.Exec(context.Background(), `
		UPDATE merchant_membership_settings
		SET balance_usable_scenes = ARRAY['dine_in', NULL]::TEXT[]
		WHERE merchant_id = $1
	`, firstMerchantID)
	requireMembershipSettingsSceneConstraintError(t, err)
}

func TestMembershipSettingsSceneMigration253HardensAlreadyAppliedWeak252Schema(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 251)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO merchant_membership_settings (
			merchant_id,
			balance_usable_scenes,
			bonus_usable_scenes
		) VALUES ($1, ARRAY['takeout', NULL, 'takeaway']::TEXT[], ARRAY['reservation', NULL]::TEXT[])
	`, merchantID)
	require.NoError(t, err)

	forceMembershipSettingsMigrationVersion(t, dbSource, 252)
	runMembershipSettingsMigrations(t, dbSource, 253)

	balanceDefault, bonusDefault := membershipSettingsSceneDefaults(t, pool)
	require.Equal(t, "ARRAY['dine_in'::text, 'takeaway'::text]", balanceDefault)
	require.Equal(t, "ARRAY['dine_in'::text]", bonusDefault)
	requireMembershipSettingsSceneConstraints(t, pool)

	var balanceScenes, bonusScenes []string
	err = pool.QueryRow(context.Background(), `
		SELECT balance_usable_scenes, bonus_usable_scenes
		FROM merchant_membership_settings
		WHERE merchant_id = $1
	`, merchantID).Scan(&balanceScenes, &bonusScenes)
	require.NoError(t, err)
	require.Equal(t, []string{"takeaway"}, balanceScenes)
	require.Empty(t, bonusScenes)

	defaultMerchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	_, err = pool.Exec(context.Background(), `
		INSERT INTO merchant_membership_settings (merchant_id)
		VALUES ($1)
	`, defaultMerchantID)
	require.NoError(t, err)

	err = pool.QueryRow(context.Background(), `
		SELECT balance_usable_scenes, bonus_usable_scenes
		FROM merchant_membership_settings
		WHERE merchant_id = $1
	`, defaultMerchantID).Scan(&balanceScenes, &bonusScenes)
	require.NoError(t, err)
	require.Equal(t, []string{"dine_in", "takeaway"}, balanceScenes)
	require.Equal(t, []string{"dine_in"}, bonusScenes)

	_, err = pool.Exec(context.Background(), `
		INSERT INTO merchant_membership_settings (
			merchant_id,
			balance_usable_scenes,
			bonus_usable_scenes
		) VALUES ($1, ARRAY['reservation']::TEXT[], ARRAY['dine_in']::TEXT[])
	`, insertMembershipSettingsMigrationMerchant(t, pool))
	requireMembershipSettingsSceneConstraintError(t, err)
}

func createMembershipSettingsMigrationDB(t *testing.T) string {
	t.Helper()

	adminSource := membershipSettingsMigrationAdminSource()
	dbName := fmt.Sprintf("locallife_mship_migration_%d_%d", time.Now().UnixNano(), os.Getpid())
	quotedDBName := pgx.Identifier{dbName}.Sanitize()

	adminPool, err := pgxpool.New(context.Background(), adminSource)
	require.NoError(t, err)
	t.Cleanup(adminPool.Close)

	_, err = adminPool.Exec(context.Background(), fmt.Sprintf(`CREATE DATABASE %s`, quotedDBName))
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = adminPool.Exec(context.Background(), `
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_activity
			WHERE datname = $1
		`, dbName)
		_, _ = adminPool.Exec(context.Background(), fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, quotedDBName))
	})

	return membershipSettingsMigrationDBSource(t, dbName)
}

func membershipSettingsMigrationAdminSource() string {
	if v := strings.TrimSpace(os.Getenv("MIGRATION_TEST_ADMIN_SOURCE")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("INTEGRATION_ADMIN_SOURCE")); v != "" {
		return v
	}
	return "postgresql:///postgres?sslmode=disable&host=/var/run/postgresql"
}

func membershipSettingsMigrationDBSource(t *testing.T, dbName string) string {
	t.Helper()

	adminSource := membershipSettingsMigrationAdminSource()
	parsed, err := url.Parse(adminSource)
	require.NoError(t, err)
	parsed.Path = "/" + dbName
	return parsed.String()
}

func membershipSettingsMigrationURL(t *testing.T) string {
	t.Helper()

	if v := strings.TrimSpace(os.Getenv("TEST_MIGRATION_URL")); v != "" {
		return v
	}
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", "migration"))
	require.NoError(t, err)
	return "file://" + path
}

func runMembershipSettingsMigrations(t *testing.T, dbSource string, targetVersion uint) {
	t.Helper()

	mig, err := migrate.New(membershipSettingsMigrationURL(t), dbSource)
	require.NoError(t, err)
	err = mig.Migrate(targetVersion)
	if err != nil && err != migrate.ErrNoChange {
		require.NoError(t, err)
	}
	closeMembershipSettingsMigration(t, mig)
}

func forceMembershipSettingsMigrationVersion(t *testing.T, dbSource string, version int) {
	t.Helper()

	mig, err := migrate.New(membershipSettingsMigrationURL(t), dbSource)
	require.NoError(t, err)
	require.NoError(t, mig.Force(version))
	closeMembershipSettingsMigration(t, mig)
}

func closeMembershipSettingsMigration(t *testing.T, mig *migrate.Migrate) {
	t.Helper()
	if sourceErr, dbErr := mig.Close(); sourceErr != nil || dbErr != nil {
		require.NoError(t, sourceErr)
		require.NoError(t, dbErr)
	}
}

func openMembershipSettingsMigrationPool(t *testing.T, dbSource string) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), dbSource)
	require.NoError(t, err)
	return pool
}

func insertMembershipSettingsMigrationMerchant(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()

	seq := membershipSettingsMigrationFixtureCounter.Add(1)
	suffix := fmt.Sprintf("%d-%d", time.Now().UnixNano(), seq)
	var userID int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO users (wechat_openid, full_name)
		VALUES ($1, $2)
		RETURNING id
	`, fmt.Sprintf("openid-%s", suffix), "migration tester").Scan(&userID)
	require.NoError(t, err)

	var regionID int64
	err = pool.QueryRow(context.Background(), `
		INSERT INTO regions (code, name, level)
		VALUES ($1, $2, 1)
		RETURNING id
	`, fmt.Sprintf("region-%s", suffix), "migration region").Scan(&regionID)
	require.NoError(t, err)

	var merchantID int64
	err = pool.QueryRow(context.Background(), `
		INSERT INTO merchants (
			owner_user_id,
			name,
			phone,
			address,
			status,
			region_id
		) VALUES ($1, $2, $3, $4, 'active', $5)
		RETURNING id
	`, userID, "migration merchant", fmt.Sprintf("13%09d", seq%1000000000), fmt.Sprintf("migration address %s", suffix), regionID).Scan(&merchantID)
	require.NoError(t, err)

	return merchantID
}

func membershipSettingsSceneDefaults(t *testing.T, pool *pgxpool.Pool) (string, string) {
	t.Helper()

	var balanceDefault, bonusDefault string
	err := pool.QueryRow(context.Background(), `
		SELECT
			pg_get_expr(balance_attr.adbin, balance_attr.adrelid),
			pg_get_expr(bonus_attr.adbin, bonus_attr.adrelid)
		FROM pg_attrdef balance_attr
		JOIN pg_attribute balance_col
			ON balance_col.attrelid = balance_attr.adrelid
			AND balance_col.attnum = balance_attr.adnum
			AND balance_col.attname = 'balance_usable_scenes'
		JOIN pg_attrdef bonus_attr
			ON bonus_attr.adrelid = balance_attr.adrelid
		JOIN pg_attribute bonus_col
			ON bonus_col.attrelid = bonus_attr.adrelid
			AND bonus_col.attnum = bonus_attr.adnum
			AND bonus_col.attname = 'bonus_usable_scenes'
		WHERE balance_attr.adrelid = 'merchant_membership_settings'::regclass
	`).Scan(&balanceDefault, &bonusDefault)
	require.NoError(t, err)
	return balanceDefault, bonusDefault
}

func requireMembershipSettingsSceneConstraints(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	rows, err := pool.Query(context.Background(), `
		SELECT conname, pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conrelid = 'merchant_membership_settings'::regclass
			AND conname IN (
				'merchant_membership_settings_balance_scenes_check',
				'merchant_membership_settings_bonus_scenes_check'
			)
		ORDER BY conname
	`)
	require.NoError(t, err)
	defer rows.Close()

	defs := map[string]string{}
	for rows.Next() {
		var name, def string
		require.NoError(t, rows.Scan(&name, &def))
		defs[name] = def
	}
	require.NoError(t, rows.Err())

	require.Contains(t, defs, "merchant_membership_settings_balance_scenes_check")
	require.Contains(t, defs["merchant_membership_settings_balance_scenes_check"], "array_position(balance_usable_scenes, NULL::text) IS NULL")
	require.Contains(t, defs["merchant_membership_settings_balance_scenes_check"], "balance_usable_scenes <@ ARRAY['dine_in'::text, 'takeaway'::text]")
	require.Contains(t, defs, "merchant_membership_settings_bonus_scenes_check")
	require.Contains(t, defs["merchant_membership_settings_bonus_scenes_check"], "array_position(bonus_usable_scenes, NULL::text) IS NULL")
	require.Contains(t, defs["merchant_membership_settings_bonus_scenes_check"], "bonus_usable_scenes <@ ARRAY['dine_in'::text, 'takeaway'::text]")
}

func requireMembershipSettingsSceneConstraintError(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)

	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23514", pgErr.Code)
}
