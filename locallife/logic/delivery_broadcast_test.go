package logic

import (
	"context"
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func numericFromFloatBroadcast(v float64) pgtype.Numeric {
	const scale = 6
	intVal := new(big.Int).Mul(big.NewInt(int64(v*1e6)), big.NewInt(1))
	return pgtype.Numeric{Int: intVal, Exp: -scale, Valid: true}
}

func TestListNearbyBroadcastRiders_StopsWhenEnoughRiders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	broadcast := NewDeliveryBroadcastLogic(store, nil)

	store.EXPECT().
		ListNearbyRiders(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, params db.ListNearbyRidersParams) ([]db.ListNearbyRidersRow, error) {
			require.Equal(t, 30.1, params.CenterLat)
			require.Equal(t, 120.2, params.CenterLng)
			require.Equal(t, broadcastStartDistanceMeters, params.MaxDistance)
			return []db.ListNearbyRidersRow{{ID: 1}, {ID: 2}, {ID: 3}}, nil
		})

	riders, err := broadcast.listNearbyBroadcastRiders(context.Background(), 30.1, 120.2)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2, 3}, []int64{riders[0].ID, riders[1].ID, riders[2].ID})
}

func TestListNearbyBroadcastRiders_ExpandsAndDeduplicates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	broadcast := NewDeliveryBroadcastLogic(store, nil)

	callCount := 0
	store.EXPECT().
		ListNearbyRiders(gomock.Any(), gomock.Any()).
		Times(2).
		DoAndReturn(func(_ context.Context, params db.ListNearbyRidersParams) ([]db.ListNearbyRidersRow, error) {
			callCount++
			switch callCount {
			case 1:
				require.Equal(t, broadcastStartDistanceMeters, params.MaxDistance)
				return []db.ListNearbyRidersRow{{ID: 1}}, nil
			case 2:
				require.Equal(t, broadcastStartDistanceMeters+broadcastStepDistanceMeters, params.MaxDistance)
				return []db.ListNearbyRidersRow{{ID: 1}, {ID: 2}, {ID: 3}}, nil
			default:
				t.Fatalf("unexpected call %d", callCount)
				return nil, nil
			}
		})

	riders, err := broadcast.listNearbyBroadcastRiders(context.Background(), 30.1, 120.2)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2, 3}, []int64{riders[0].ID, riders[1].ID, riders[2].ID})
}

func TestBroadcastNewOrderNotification_SkipsWhenPickupCoordinatesMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	broadcast := NewDeliveryBroadcastLogic(store, nil)

	err := broadcast.BroadcastNewOrderNotification(context.Background(), db.DeliveryPool{OrderID: 11}, "merchant")
	require.NoError(t, err)
}

func TestBroadcastNewOrderNotification_UsesNearbyRiders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	broadcast := NewDeliveryBroadcastLogic(store, nil)

	store.EXPECT().
		ListNearbyRiders(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.ListNearbyRidersRow{{ID: 1}, {ID: 2}, {ID: 3}}, nil)

	err := broadcast.BroadcastNewOrderNotification(context.Background(), db.DeliveryPool{
		OrderID:         11,
		PickupLongitude: numericFromFloatBroadcast(120.2),
		PickupLatitude:  numericFromFloatBroadcast(30.1),
	}, "merchant")
	require.NoError(t, err)
}
