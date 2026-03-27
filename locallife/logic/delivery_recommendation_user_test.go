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

func numericFromFloatRecommendUser(v float64) pgtype.Numeric {
	const scale = 6
	intVal := new(big.Int).Mul(big.NewInt(int64(v*1e6)), big.NewInt(1))
	intVal.Mul(intVal, big.NewInt(1))
	return pgtype.Numeric{Int: intVal, Exp: -scale, Valid: true}
}

func TestRecommendDeliveryOrdersForUser(t *testing.T) {
	userID := int64(10)

	testCases := []struct {
		name       string
		input      RecommendDeliveryForUserInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result RecommendDeliveryForUserResult, err error)
	}{
		{
			name:  "NotRider",
			input: RecommendDeliveryForUserInput{UserID: userID, RiderLat: 30, RiderLng: 120},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), userID).
					Times(1).
					Return(db.Rider{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ RecommendDeliveryForUserResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "您还不是骑手", reqErr.Err.Error())
			},
		},
		{
			name:  "Offline",
			input: RecommendDeliveryForUserInput{UserID: userID, RiderLat: 30, RiderLng: 120},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), userID).
					Times(1).
					Return(db.Rider{ID: 1, UserID: userID, IsOnline: false}, nil)
			},
			check: func(t *testing.T, _ RecommendDeliveryForUserResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "请先上线", reqErr.Err.Error())
			},
		},
		{
			name:  "NoRegion",
			input: RecommendDeliveryForUserInput{UserID: userID, RiderLat: 30, RiderLng: 120},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), userID).
					Times(1).
					Return(db.Rider{ID: 1, UserID: userID, IsOnline: true}, nil)
				store.EXPECT().
					GetRiderProfile(gomock.Any(), int64(1)).
					Times(1).
					Return(db.RiderProfile{RiderID: 1, IsSuspended: false}, nil)
			},
			check: func(t *testing.T, _ RecommendDeliveryForUserResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "您尚未分配服务区域，请联系管理员", reqErr.Err.Error())
			},
		},
		{
			name:  "Suspended",
			input: RecommendDeliveryForUserInput{UserID: userID, RiderLat: 30, RiderLng: 120},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), userID).
					Times(1).
					Return(db.Rider{ID: 1, UserID: userID, IsOnline: true}, nil)
				store.EXPECT().
					GetRiderProfile(gomock.Any(), int64(1)).
					Times(1).
					Return(db.RiderProfile{
						RiderID:       1,
						IsSuspended:   true,
						SuspendReason: pgtype.Text{String: "claim recovery overdue", Valid: true},
					}, nil)
			},
			check: func(t *testing.T, _ RecommendDeliveryForUserResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "骑手接单已暂停", reqErr.Err.Error())
			},
		},
		{
			name:  "Success",
			input: RecommendDeliveryForUserInput{UserID: userID, RiderLat: 30, RiderLng: 120},
			buildStubs: func(store *mockdb.MockStore) {
				rider := db.Rider{ID: 2, UserID: userID, IsOnline: true, RegionID: pgtype.Int8{Int64: 9, Valid: true}}
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), userID).
					Times(1).
					Return(rider, nil)
				store.EXPECT().
					GetRiderProfile(gomock.Any(), rider.ID).
					Times(1).
					Return(db.RiderProfile{RiderID: rider.ID, IsSuspended: false}, nil)
				store.EXPECT().
					GetActiveRecommendConfig(gomock.Any()).
					Times(1).
					Return(db.RecommendConfig{}, db.ErrRecordNotFound)
				store.EXPECT().
					ListDeliveryPoolNearbyByRegion(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListDeliveryPoolNearbyByRegionRow{
						{
							OrderID:            10,
							MerchantID:         20,
							PickupLongitude:    numericFromFloatRecommendUser(120.001),
							PickupLatitude:     numericFromFloatRecommendUser(30.001),
							DeliveryLongitude:  numericFromFloatRecommendUser(120.002),
							DeliveryLatitude:   numericFromFloatRecommendUser(30.002),
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
					ListRiderActiveDeliveries(gomock.Any(), pgtype.Int8{Int64: 2, Valid: true}).
					Times(1).
					Return([]db.Delivery{}, nil)
			},
			check: func(t *testing.T, result RecommendDeliveryForUserResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(2), result.Rider.ID)
				require.Len(t, result.Recommendations.Scored, 1)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			if tc.buildStubs != nil {
				tc.buildStubs(store)
			}

			result, err := RecommendDeliveryOrdersForUser(context.Background(), store, nil, tc.input)
			tc.check(t, result, err)
		})
	}
}
