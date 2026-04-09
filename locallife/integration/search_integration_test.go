package integration

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

type integrationSearchMerchantResponse struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Distance *int     `json:"distance"`
	Tags     []string `json:"tags"`
	RegionID int64    `json:"region_id"`
}

type integrationSearchMerchantListResponse struct {
	Merchants []integrationSearchMerchantResponse `json:"merchants"`
	Total     int64                               `json:"total"`
	PageID    int32                               `json:"page_id"`
	PageSize  int32                               `json:"page_size"`
}

func TestSearchMerchantsTagDistanceIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	ctx := context.Background()
	region := createIntegrationRegion(t, store)
	consumer := createIntegrationUser(t, store)

	nearOwner := createIntegrationUser(t, store)
	nearMerchant := createIntegrationSearchableMerchant(t, store, nearOwner.ID, region.ID, "近场川菜馆", 39.9010, 116.4010)

	farOwner := createIntegrationUser(t, store)
	farMerchant := createIntegrationSearchableMerchant(t, store, farOwner.ID, region.ID, "远场川菜馆", 39.9300, 116.4300)

	otherOwner := createIntegrationUser(t, store)
	otherMerchant := createIntegrationSearchableMerchant(t, store, otherOwner.ID, region.ID, "其他品类馆", 39.9005, 116.4005)

	targetTag, err := store.CreateTag(ctx, db.CreateTagParams{
		Name:      "川菜-集成-" + util.RandomString(6),
		Type:      "merchant",
		SortOrder: 1,
		Status:    "active",
	})
	require.NoError(t, err)

	otherTag, err := store.CreateTag(ctx, db.CreateTagParams{
		Name:      "甜品-集成-" + util.RandomString(6),
		Type:      "merchant",
		SortOrder: 2,
		Status:    "active",
	})
	require.NoError(t, err)

	require.NoError(t, store.AddMerchantTag(ctx, db.AddMerchantTagParams{MerchantID: nearMerchant.ID, TagID: targetTag.ID}))
	require.NoError(t, store.AddMerchantTag(ctx, db.AddMerchantTagParams{MerchantID: farMerchant.ID, TagID: targetTag.ID}))
	require.NoError(t, store.AddMerchantTag(ctx, db.AddMerchantTagParams{MerchantID: otherMerchant.ID, TagID: otherTag.ID}))

	request, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/search/merchants?region_id=%d&tag_id=%d&sort_by=distance&page_id=1&page_size=10&user_latitude=39.9000&user_longitude=116.4000", region.ID, targetTag.ID),
		nil,
	)
	require.NoError(t, err)
	addAuthorization(t, request, integrationTokenMaker, consumer.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp integrationSearchMerchantListResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(2), resp.Total)
	require.Equal(t, int32(1), resp.PageID)
	require.Equal(t, int32(10), resp.PageSize)
	require.Len(t, resp.Merchants, 2)

	require.Equal(t, nearMerchant.ID, resp.Merchants[0].ID)
	require.Equal(t, farMerchant.ID, resp.Merchants[1].ID)
	require.NotNil(t, resp.Merchants[0].Distance)
	require.NotNil(t, resp.Merchants[1].Distance)
	require.Less(t, *resp.Merchants[0].Distance, *resp.Merchants[1].Distance)
	require.Contains(t, resp.Merchants[0].Tags, targetTag.Name)
	require.Contains(t, resp.Merchants[1].Tags, targetTag.Name)
	require.NotEqual(t, otherMerchant.ID, resp.Merchants[0].ID)
	require.NotEqual(t, otherMerchant.ID, resp.Merchants[1].ID)
}

func TestSearchMerchantsKeywordDistanceIntegration(t *testing.T) {
	server, store := initIntegrationServer(t)
	resetIntegrationData(t)

	region := createIntegrationRegion(t, store)
	consumer := createIntegrationUser(t, store)

	nearOwner := createIntegrationUser(t, store)
	nearMerchant := createIntegrationSearchableMerchant(t, store, nearOwner.ID, region.ID, "近场火锅店", 39.9010, 116.4010)

	farOwner := createIntegrationUser(t, store)
	farMerchant := createIntegrationSearchableMerchant(t, store, farOwner.ID, region.ID, "远场火锅城", 39.9300, 116.4300)

	otherOwner := createIntegrationUser(t, store)
	otherMerchant := createIntegrationSearchableMerchant(t, store, otherOwner.ID, region.ID, "轻食沙拉馆", 39.9005, 116.4005)

	request, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/search/merchants?keyword=火锅&region_id=%d&sort_by=distance&page_id=1&page_size=10&user_latitude=39.9000&user_longitude=116.4000", region.ID),
		nil,
	)
	require.NoError(t, err)
	addAuthorization(t, request, integrationTokenMaker, consumer.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp integrationSearchMerchantListResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(2), resp.Total)
	require.Equal(t, int32(1), resp.PageID)
	require.Equal(t, int32(10), resp.PageSize)
	require.Len(t, resp.Merchants, 2)

	require.Equal(t, nearMerchant.ID, resp.Merchants[0].ID)
	require.Equal(t, farMerchant.ID, resp.Merchants[1].ID)
	require.NotNil(t, resp.Merchants[0].Distance)
	require.NotNil(t, resp.Merchants[1].Distance)
	require.Less(t, *resp.Merchants[0].Distance, *resp.Merchants[1].Distance)
	require.Contains(t, resp.Merchants[0].Name, "火锅")
	require.Contains(t, resp.Merchants[1].Name, "火锅")
	require.NotEqual(t, otherMerchant.ID, resp.Merchants[0].ID)
	require.NotEqual(t, otherMerchant.ID, resp.Merchants[1].ID)
}

func createIntegrationSearchableMerchant(t *testing.T, store *db.SQLStore, ownerID, regionID int64, name string, latitude, longitude float64) db.Merchant {
	t.Helper()

	merchant, err := store.CreateMerchant(context.Background(), db.CreateMerchantParams{
		OwnerUserID:     ownerID,
		Name:            name,
		Description:     pgtype.Text{String: "搜索集成测试商户", Valid: true},
		Phone:           fmt.Sprintf("138%08d", util.RandomInt(0, 99999999)),
		Address:         "搜索测试地址-" + util.RandomString(6),
		Latitude:        integrationNumeric(t, latitude),
		Longitude:       integrationNumeric(t, longitude),
		Status:          "active",
		ApplicationData: []byte("{}"),
		RegionID:        regionID,
	})
	require.NoError(t, err)

	_, err = store.UpdateMerchantIsOpen(context.Background(), db.UpdateMerchantIsOpenParams{
		ID:          merchant.ID,
		IsOpen:      true,
		AutoCloseAt: pgtype.Timestamptz{Valid: false},
	})
	require.NoError(t, err)

	return merchant
}

func integrationNumeric(t *testing.T, value float64) pgtype.Numeric {
	t.Helper()

	return pgtype.Numeric{
		Int:   big.NewInt(int64(value * 1000000)),
		Exp:   -6,
		Valid: true,
	}
}
