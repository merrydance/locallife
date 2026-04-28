package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSuspendRider_OfflinesSuspendedRider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	rider := randomRider(101)
	rider.ID = 321
	rider.IsOnline = true
	operator := db.Operator{ID: 22, UserID: 88, RegionID: rider.RegionID.Int64}

	store.EXPECT().
		GetRider(gomock.Any(), rider.ID).
		Return(rider, nil)
	store.EXPECT().
		CheckOperatorManagesRegion(gomock.Any(), db.CheckOperatorManagesRegionParams{OperatorID: operator.ID, RegionID: rider.RegionID.Int64}).
		Return(true, nil)
	store.EXPECT().
		UpdateRiderStatus(gomock.Any(), db.UpdateRiderStatusParams{ID: rider.ID, Status: db.RiderStatusSuspended}).
		Return(db.Rider{
			ID:            rider.ID,
			UserID:        rider.UserID,
			Status:        db.RiderStatusSuspended,
			IsOnline:      true,
			DepositAmount: rider.DepositAmount,
			RegionID:      rider.RegionID,
		}, nil)
	store.EXPECT().
		UpdateRiderOnlineStatus(gomock.Any(), db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: false}).
		Return(db.Rider{
			ID:            rider.ID,
			UserID:        rider.UserID,
			Status:        db.RiderStatusSuspended,
			IsOnline:      false,
			DepositAmount: rider.DepositAmount,
			RegionID:      rider.RegionID,
		}, nil)

	ctx, recorder := newJSONTestContext(http.MethodPost, "/v1/operator/riders/321/suspend", `{"reason":"manual control","duration_hours":24}`)
	ctx.Params = gin.Params{{Key: "id", Value: "321"}}
	ctx.Set(operatorKey, operator)

	server.SuspendRider(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}
