package logic

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func numericFromFloatRecommendation(v float64) pgtype.Numeric {
	const scale = 6
	intVal := new(big.Int).Mul(big.NewInt(int64(v*1e6)), big.NewInt(1))
	intVal.Mul(intVal, big.NewInt(1))
	return pgtype.Numeric{Int: intVal, Exp: -scale, Valid: true}
}

func TestRecommendDeliveryOrders_Basic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetActiveRecommendConfig(gomock.Any()).
		Times(1).
		Return(db.RecommendConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		ListDeliveryPoolNearby(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.ListDeliveryPoolNearbyRow{
			{
				OrderID:            10,
				MerchantID:         20,
				PickupLongitude:    numericFromFloatRecommendation(120.001),
				PickupLatitude:     numericFromFloatRecommendation(30.001),
				DeliveryLongitude:  numericFromFloatRecommendation(120.002),
				DeliveryLatitude:   numericFromFloatRecommendation(30.002),
				Distance:           1000,
				DeliveryFee:        800,
				ExpectedPickupAt:   time.Now(),
				ExpectedDeliveryAt: pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
				ExpiresAt:          time.Now().Add(10 * time.Minute),
				Priority:           1,
				CreatedAt:          time.Now(),
			},
		}, nil)
	store.EXPECT().
		ListRiderActiveDeliveries(gomock.Any(), pgtype.Int8{Int64: 1, Valid: true}).
		Times(1).
		Return([]db.Delivery{}, nil)

	result, err := RecommendDeliveryOrders(context.Background(), store, nil, RecommendDeliveryInput{
		RiderID:  1,
		RiderLat: 30.0,
		RiderLng: 120.0,
	})
	require.NoError(t, err)
	require.Len(t, result.Scored, 1)
	require.Len(t, result.RealDistances, 0)
}
