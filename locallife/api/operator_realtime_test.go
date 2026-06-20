package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
)

func TestGetOperatorRealtimeStatsAPI_UsesPendingApprovalForPendingRiders(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	regionID := operator.RegionID

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	expectOperatorManagesRegion(store, operator, regionID, true)

	store.EXPECT().
		CountMerchantsByRegionWithStatus(gomock.Any(), db.CountMerchantsByRegionWithStatusParams{
			RegionID: regionID,
			Column2:  db.MerchantStatusApproved,
		}).
		Return(int64(7), nil)
	store.EXPECT().
		CountRidersByRegionWithStatus(gomock.Any(), db.CountRidersByRegionWithStatusParams{
			RegionID: pgtype.Int8{Int64: regionID, Valid: true},
			Status:   db.RiderStatusActive,
		}).
		Return(int64(5), nil)
	store.EXPECT().
		CountMerchantsByRegionWithStatus(gomock.Any(), db.CountMerchantsByRegionWithStatusParams{
			RegionID: regionID,
			Column2:  "pending",
		}).
		Return(int64(3), nil)
	store.EXPECT().
		CountRidersByRegionWithStatus(gomock.Any(), db.CountRidersByRegionWithStatusParams{
			RegionID: pgtype.Int8{Int64: regionID, Valid: true},
			Status:   "pending",
		}).
		AnyTimes().
		Return(int64(99), nil)
	store.EXPECT().
		CountRidersByRegionWithStatus(gomock.Any(), db.CountRidersByRegionWithStatusParams{
			RegionID: pgtype.Int8{Int64: regionID, Valid: true},
			Status:   db.RiderStatusPendingApproval,
		}).
		AnyTimes().
		Return(int64(2), nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/v1/operator/stats/realtime", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp operatorRealtimeStatsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int32(7), resp.ActiveMerchantCount)
	require.Equal(t, int32(5), resp.ActiveRiderCount)
	require.Equal(t, int32(3), resp.PendingMerchantCount)
	require.Equal(t, int32(2), resp.PendingRiderCount)
}
