package logic

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type distanceMatrixClient struct {
	result *maps.DistanceMatrixResult
	err    error
}

func (c *distanceMatrixClient) GetBicyclingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return nil, nil
}

func (c *distanceMatrixClient) GetWalkingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return nil, nil
}

func (c *distanceMatrixClient) GetDrivingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return nil, nil
}

func (c *distanceMatrixClient) GetDistanceMatrix(ctx context.Context, froms, tos []maps.Location, mode string) (*maps.DistanceMatrixResult, error) {
	return c.result, c.err
}

func (c *distanceMatrixClient) Geocode(ctx context.Context, address string) (*maps.GeocodeResult, error) {
	return nil, nil
}

func (c *distanceMatrixClient) ReverseGeocode(ctx context.Context, location maps.Location) (*maps.ReverseGeocodeResult, error) {
	return nil, nil
}

func numericFromFloatEstimate(v float64) pgtype.Numeric {
	const scale = 6
	intVal := new(big.Int).Mul(big.NewInt(int64(v*1e6)), big.NewInt(1))
	intVal.Mul(intVal, big.NewInt(1))
	return pgtype.Numeric{Int: intVal, Exp: -scale, Valid: true}
}

func TestUpdateDeliveryEstimatedTime_SkipsWithoutRiderLocation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	result, err := UpdateDeliveryEstimatedTime(context.Background(), store, DeliveryEstimateInput{
		Delivery: db.Delivery{ID: 1},
		Rider:    db.Rider{},
		Merchant: db.Merchant{},
	})
	require.NoError(t, err)
	require.True(t, result.Skipped)
}

func TestUpdateDeliveryEstimatedTime_UsesDistanceMatrix(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	now := time.Now()
	delivery := db.Delivery{
		ID:                10,
		Distance:          2000,
		EstimatedPickupAt: pgtype.Timestamptz{Time: now.Add(5 * time.Minute), Valid: true},
		DeliveryLatitude:  numericFromFloatEstimate(30.0),
		DeliveryLongitude: numericFromFloatEstimate(120.0),
	}
	rider := db.Rider{
		CurrentLatitude:  numericFromFloatEstimate(30.0),
		CurrentLongitude: numericFromFloatEstimate(120.0),
	}
	merchant := db.Merchant{
		Latitude:  numericFromFloatEstimate(30.001),
		Longitude: numericFromFloatEstimate(120.001),
	}

	matrix := &maps.DistanceMatrixResult{Rows: []maps.DistanceMatrixRow{
		{Elements: []maps.DistanceMatrixElement{{Distance: 1000, Duration: 600}}},
		{Elements: []maps.DistanceMatrixElement{{Distance: 2000, Duration: 900}}},
	}}
	client := &distanceMatrixClient{result: matrix}

	store.EXPECT().
		UpdateDeliveryEstimatedTime(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Delivery{}, nil)

	result, err := UpdateDeliveryEstimatedTime(context.Background(), store, DeliveryEstimateInput{
		Delivery:                delivery,
		Rider:                   rider,
		Merchant:                merchant,
		RiderSpeedMetersPerHour: 12000,
		MinTotalMinutes:         10,
		MapClient:               client,
	})
	require.NoError(t, err)
	require.True(t, result.Updated)
	require.Equal(t, 1000, result.RiderToMerchantDistance)
	require.Equal(t, 2000, result.MerchantToCustomerDistance)
	require.InDelta(t, 30.0, result.TotalMinutes, 0.5)
}

func TestUpdateDeliveryEstimatedTime_FallbackHaversine(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	delivery := db.Delivery{
		ID:                20,
		Distance:          3000,
		DeliveryLatitude:  numericFromFloatEstimate(30.0),
		DeliveryLongitude: numericFromFloatEstimate(120.0),
	}
	rider := db.Rider{
		CurrentLatitude:  numericFromFloatEstimate(30.0),
		CurrentLongitude: numericFromFloatEstimate(120.0),
	}
	merchant := db.Merchant{
		Latitude:  numericFromFloatEstimate(30.02),
		Longitude: numericFromFloatEstimate(120.02),
	}

	store.EXPECT().
		UpdateDeliveryEstimatedTime(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.Delivery{}, nil)

	result, err := UpdateDeliveryEstimatedTime(context.Background(), store, DeliveryEstimateInput{
		Delivery:                delivery,
		Rider:                   rider,
		Merchant:                merchant,
		RiderSpeedMetersPerHour: 12000,
		MinTotalMinutes:         10,
		MapClient:               nil,
	})
	require.NoError(t, err)
	require.True(t, result.Updated)
	require.Greater(t, result.RiderToMerchantDistance, 0)
	require.Greater(t, result.TotalMinutes, 0.0)
}
