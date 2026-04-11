package logic

import (
	"context"
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
	"github.com/stretchr/testify/require"
)

type fakeMapClient struct {
	route *maps.RouteResult
	err   error
}

func (f *fakeMapClient) GetBicyclingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return f.route, f.err
}

func (f *fakeMapClient) GetWalkingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return nil, nil
}

func (f *fakeMapClient) GetDrivingRoute(ctx context.Context, from, to maps.Location) (*maps.RouteResult, error) {
	return nil, nil
}

func (f *fakeMapClient) GetDistanceMatrix(ctx context.Context, froms, tos []maps.Location, mode string) (*maps.DistanceMatrixResult, error) {
	return nil, nil
}

func (f *fakeMapClient) Geocode(ctx context.Context, address string) (*maps.GeocodeResult, error) {
	return nil, nil
}

func (f *fakeMapClient) ReverseGeocode(ctx context.Context, location maps.Location) (*maps.ReverseGeocodeResult, error) {
	return nil, nil
}

func numericFromFloat(v float64) pgtype.Numeric {
	const scale = 6
	intVal := new(big.Int).Mul(big.NewInt(int64(v*1e6)), big.NewInt(1))
	intVal.Mul(intVal, big.NewInt(1))
	return pgtype.Numeric{Int: intVal, Exp: -scale, Valid: true}
}

func TestComputeDeliveryQuote(t *testing.T) {
	merchant := db.Merchant{
		ID:        1,
		RegionID:  2,
		Latitude:  numericFromFloat(30.0),
		Longitude: numericFromFloat(120.0),
	}
	address := db.UserAddress{
		UserID:    10,
		RegionID:  2,
		Latitude:  numericFromFloat(30.001),
		Longitude: numericFromFloat(120.001),
	}

	testCases := []struct {
		name  string
		input DeliveryQuoteInput
		mapc  maps.TencentMapClientInterface
		calc  DeliveryFeeCalculator
		check func(t *testing.T, result DeliveryQuoteResult, err error)
	}{
		{
			name:  "NonTakeout",
			input: DeliveryQuoteInput{UserID: address.UserID, OrderType: "dine_in", Subtotal: 1000, Merchant: merchant, Address: address},
			calc: func(ctx context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
				return DeliveryFeeComputation{}, nil
			},
			check: func(t *testing.T, result DeliveryQuoteResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(0), result.Fee)
			},
		},
		{
			name:  "AddressNotBelong",
			input: DeliveryQuoteInput{UserID: address.UserID + 1, OrderType: "takeout", Subtotal: 1000, Merchant: merchant, Address: address},
			check: func(t *testing.T, _ DeliveryQuoteResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "address does not belong to you", reqErr.Err.Error())
			},
		},
		{
			name: "InvalidLocation",
			input: DeliveryQuoteInput{
				UserID:    address.UserID,
				OrderType: "takeout",
				Subtotal:  1000,
				Merchant:  db.Merchant{ID: 1},
				Address:   address,
			},
			check: func(t *testing.T, _ DeliveryQuoteResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "invalid address or merchant location", reqErr.Err.Error())
			},
		},
		{
			name:  "MissingCalculator",
			input: DeliveryQuoteInput{UserID: address.UserID, OrderType: "takeout", Subtotal: 1000, Merchant: merchant, Address: address},
			check: func(t *testing.T, _ DeliveryQuoteResult, err error) {
				require.Error(t, err)
				require.Equal(t, "delivery fee calculator is required", err.Error())
			},
		},
		{
			name:  "MapClientRoute",
			input: DeliveryQuoteInput{UserID: address.UserID, OrderType: "takeout", Subtotal: 1000, Merchant: merchant, Address: address},
			mapc:  &fakeMapClient{route: &maps.RouteResult{Distance: 2000, Duration: 600}},
			calc: func(ctx context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
				require.Equal(t, int32(2000), distance)
				return DeliveryFeeComputation{Fee: 500, Discount: 50}, nil
			},
			check: func(t *testing.T, result DeliveryQuoteResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int32(2000), result.Distance)
				require.Equal(t, int32(600), result.Duration)
				require.Equal(t, int64(500), result.Fee)
				require.Equal(t, int64(50), result.Discount)
			},
		},
		{
			name:  "FallbackMinDistance",
			input: DeliveryQuoteInput{UserID: address.UserID, OrderType: "takeout", Subtotal: 1000, Merchant: merchant, Address: address},
			calc: func(ctx context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
				require.Equal(t, int32(minDeliveryDistanceMeters), distance)
				return DeliveryFeeComputation{Fee: 300, Discount: 0}, nil
			},
			check: func(t *testing.T, result DeliveryQuoteResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int32(minDeliveryDistanceMeters), result.Distance)
				require.Equal(t, int64(300), result.Fee)
			},
		},
		{
			name:  "Suspended",
			input: DeliveryQuoteInput{UserID: address.UserID, OrderType: "takeout", Subtotal: 1000, Merchant: merchant, Address: address},
			calc: func(ctx context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
				return DeliveryFeeComputation{Suspended: true}, nil
			},
			check: func(t *testing.T, _ DeliveryQuoteResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "delivery suspended", reqErr.Err.Error())
			},
		},
		{
			name:  "SuspendedWithReason",
			input: DeliveryQuoteInput{UserID: address.UserID, OrderType: "takeout", Subtotal: 1000, Merchant: merchant, Address: address},
			calc: func(ctx context.Context, regionID, merchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
				return DeliveryFeeComputation{Suspended: true, SuspendReason: "policy"}, nil
			},
			check: func(t *testing.T, _ DeliveryQuoteResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "policy", reqErr.Err.Error())
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := ComputeDeliveryQuote(context.Background(), tc.input, tc.mapc, tc.calc)
			tc.check(t, result, err)
		})
	}
}
