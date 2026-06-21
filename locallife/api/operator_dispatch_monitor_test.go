package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetOperatorPendingDispatchSummaryAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{ID: 81, UserID: user.ID, RegionID: 66, Status: "active"}
	now := time.Now()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	expectOperatorManagesRegion(store, operator, 66, true)
	store.EXPECT().
		GetOperatorPendingDispatchSummary(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, arg db.GetOperatorPendingDispatchSummaryParams) (db.GetOperatorPendingDispatchSummaryRow, error) {
			require.Equal(t, int64(66), arg.RegionID)
			require.Equal(t, operatorPendingDispatchStatus, arg.Status)
			require.WithinDuration(t, now.Add(-3*time.Minute), arg.TimeoutBefore, 2*time.Second)
			return db.GetOperatorPendingDispatchSummaryRow{
				RegionID:                  66,
				RegionName:                "测试区域",
				PendingTotal:              12,
				TimeoutOverThresholdTotal: 4,
				OldestWaitSeconds:         int64(420),
				LatestRefreshAt:           now,
			}, nil
		})

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/regions/66/delivery-pool/summary", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response operatorPendingDispatchSummaryResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, int64(66), response.RegionID)
	require.Equal(t, "测试区域", response.RegionName)
	require.Equal(t, int64(12), response.PendingTotal)
	require.Equal(t, int64(4), response.TimeoutOver3mTotal)
	require.Equal(t, int64(420), response.OldestWaitSeconds)
	require.WithinDuration(t, now, response.LatestRefreshAt, time.Second)
}

func TestListOperatorPendingDispatchesAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{ID: 82, UserID: user.ID, RegionID: 66, Status: "active"}
	now := time.Now()
	expectedPickup := pgtype.Timestamptz{Time: now.Add(10 * time.Minute), Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	expectOperatorManagesRegion(store, operator, 66, true)
	store.EXPECT().
		ListOperatorPendingDispatches(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ interface{}, arg db.ListOperatorPendingDispatchesParams) ([]db.ListOperatorPendingDispatchesRow, error) {
			require.Equal(t, int64(66), arg.RegionID)
			require.Equal(t, operatorPendingDispatchStatus, arg.Status)
			require.Equal(t, int32(20), arg.Limit)
			require.Equal(t, int32(20), arg.Offset)
			require.WithinDuration(t, now.Add(-3*time.Minute), arg.TimeoutBefore, 2*time.Second)
			return []db.ListOperatorPendingDispatchesRow{{
				DeliveryID:             501,
				OrderID:                601,
				OrderNo:                "ORD-001",
				MerchantID:             701,
				MerchantName:           "测试商户",
				RegionID:               66,
				RegionName:             "测试区域",
				WaitSeconds:            305,
				DeliveryFee:            800,
				ExpectedPickupAt:       expectedPickup,
				IsTimeoutOverThreshold: true,
			}}, nil
		})
	store.EXPECT().
		CountOperatorPendingDispatches(gomock.Any(), db.CountOperatorPendingDispatchesParams{RegionID: 66, Status: operatorPendingDispatchStatus}).
		Return(int64(35), nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/regions/66/delivery-pool?page=2&limit=20", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response listOperatorPendingDispatchesResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.Equal(t, int64(35), response.Total)
	require.Equal(t, int32(2), response.Page)
	require.Equal(t, int32(20), response.Limit)
	require.Len(t, response.Items, 1)
	require.Equal(t, int64(501), response.Items[0].DeliveryID)
	require.Equal(t, "ORD-001", response.Items[0].OrderNo)
	require.True(t, response.Items[0].IsTimeoutOver3m)
	require.NotNil(t, response.Items[0].ExpectedPickupAt)
	require.WithinDuration(t, expectedPickup.Time, *response.Items[0].ExpectedPickupAt, time.Second)
}

func TestGetOperatorPendingDispatchSummaryAPI_DeniesUnmanagedRegion(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{ID: 83, UserID: user.ID, RegionID: 66, Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	expectOperatorManagesRegion(store, operator, 99, false)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/regions/99/delivery-pool/summary", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestGetOperatorPendingDispatchSummaryAPI_AllowsLegacyPrimaryRegionWithoutRelation(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{ID: 84, UserID: user.ID, RegionID: 66, Status: "active"}
	now := time.Now()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	store.EXPECT().
		CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
			OperatorID: operator.ID,
			RegionID:   operator.RegionID,
		}).
		Return(false, nil)
	store.EXPECT().
		GetOperatorRegion(gomock.Any(), db.GetOperatorRegionParams{
			OperatorID: operator.ID,
			RegionID:   operator.RegionID,
		}).
		Return(db.OperatorRegion{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetOperatorPendingDispatchSummary(gomock.Any(), gomock.Any()).
		Return(db.GetOperatorPendingDispatchSummaryRow{
			RegionID:                  operator.RegionID,
			RegionName:                "测试区域",
			PendingTotal:              0,
			TimeoutOverThresholdTotal: 0,
			OldestWaitSeconds:         int64(0),
			LatestRefreshAt:           now,
		}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/regions/66/delivery-pool/summary", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestListOperatorPendingDispatchesAPI_AllowsLegacyPrimaryRegionWithoutRelation(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{ID: 87, UserID: user.ID, RegionID: 66, Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	store.EXPECT().
		CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
			OperatorID: operator.ID,
			RegionID:   operator.RegionID,
		}).
		Return(false, nil)
	store.EXPECT().
		GetOperatorRegion(gomock.Any(), db.GetOperatorRegionParams{
			OperatorID: operator.ID,
			RegionID:   operator.RegionID,
		}).
		Return(db.OperatorRegion{}, db.ErrRecordNotFound)
	store.EXPECT().
		ListOperatorPendingDispatches(gomock.Any(), gomock.Any()).
		Return([]db.ListOperatorPendingDispatchesRow{}, nil)
	store.EXPECT().
		CountOperatorPendingDispatches(gomock.Any(), db.CountOperatorPendingDispatchesParams{
			RegionID: operator.RegionID,
			Status:   operatorPendingDispatchStatus,
		}).
		Return(int64(0), nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/regions/66/delivery-pool?page=1&limit=20", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestGetOperatorPendingDispatchSummaryAPI_DeniesSuspendedRegionRelation(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{ID: 85, UserID: user.ID, RegionID: 66, Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	store.EXPECT().
		CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
			OperatorID: operator.ID,
			RegionID:   operator.RegionID,
		}).
		Return(false, nil)
	store.EXPECT().
		GetOperatorRegion(gomock.Any(), db.GetOperatorRegionParams{
			OperatorID: operator.ID,
			RegionID:   operator.RegionID,
		}).
		Return(db.OperatorRegion{
			OperatorID: operator.ID,
			RegionID:   operator.RegionID,
			Status:     db.OperatorRegionStatusSuspended,
		}, nil)
	store.EXPECT().
		GetOperatorPendingDispatchSummary(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/regions/66/delivery-pool/summary", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestGetOperatorPendingDispatchSummaryAPI_ReturnsInternalErrorWhenLegacyRelationLookupFails(t *testing.T) {
	user, _ := randomUser(t)
	operator := db.Operator{ID: 86, UserID: user.ID, RegionID: 66, Status: "active"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	store.EXPECT().
		CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{
			OperatorID: operator.ID,
			RegionID:   operator.RegionID,
		}).
		Return(false, nil)
	store.EXPECT().
		GetOperatorRegion(gomock.Any(), db.GetOperatorRegionParams{
			OperatorID: operator.ID,
			RegionID:   operator.RegionID,
		}).
		Return(db.OperatorRegion{}, errors.New("operator region lookup unavailable"))
	store.EXPECT().
		GetOperatorPendingDispatchSummary(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operator/regions/66/delivery-pool/summary", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}
