package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUrgeOrder(t *testing.T) {
	now := time.Date(2026, 2, 12, 10, 0, 0, 0, time.UTC)
	window := 5 * time.Minute

	baseOrder := db.Order{
		ID:         10,
		UserID:     100,
		MerchantID: 200,
		OrderNo:    "ORDER-001",
		Status:     "paid",
	}

	testCases := []struct {
		name       string
		input      UrgeOrderInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result UrgeOrderResult, err error)
	}{
		{
			name: "OrderNotFound",
			input: UrgeOrderInput{
				UserID:          baseOrder.UserID,
				OrderID:         baseOrder.ID,
				RateLimitWindow: window,
				RateLimitMax:    3,
				Now:             now,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ UrgeOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "order not found", reqErr.Err.Error())
			},
		},
		{
			name: "OrderNotBelongToUser",
			input: UrgeOrderInput{
				UserID:          baseOrder.UserID + 1,
				OrderID:         baseOrder.ID,
				RateLimitWindow: window,
				RateLimitMax:    3,
				Now:             now,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(baseOrder, nil)
			},
			check: func(t *testing.T, _ UrgeOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "order does not belong to you", reqErr.Err.Error())
			},
		},
		{
			name: "RateLimitExceeded",
			input: UrgeOrderInput{
				UserID:          baseOrder.UserID,
				OrderID:         baseOrder.ID,
				RateLimitWindow: window,
				RateLimitMax:    3,
				Now:             now,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(baseOrder, nil)
				store.EXPECT().
					CountRecentOrderStatusLogs(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(3), nil)
			},
			check: func(t *testing.T, _ UrgeOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 429, reqErr.Status)
				require.Equal(t, "催单过于频繁，请5分钟后再试", reqErr.Err.Error())
			},
		},
		{
			name: "InvalidStatus",
			input: UrgeOrderInput{
				UserID:          baseOrder.UserID,
				OrderID:         baseOrder.ID,
				RateLimitWindow: window,
				RateLimitMax:    3,
				Now:             now,
			},
			buildStubs: func(store *mockdb.MockStore) {
				order := baseOrder
				order.Status = "pending"
				store.EXPECT().
					GetOrder(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(order, nil)
				store.EXPECT().
					CountRecentOrderStatusLogs(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			check: func(t *testing.T, _ UrgeOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "order cannot be urged in current status", reqErr.Err.Error())
			},
		},
		{
			name: "SuccessPaid",
			input: UrgeOrderInput{
				UserID:          baseOrder.UserID,
				OrderID:         baseOrder.ID,
				RateLimitWindow: window,
				RateLimitMax:    3,
				Now:             now,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(baseOrder, nil)
				store.EXPECT().
					CountRecentOrderStatusLogs(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
			},
			check: func(t *testing.T, result UrgeOrderResult, err error) {
				require.NoError(t, err)
				require.True(t, result.NotifyMerchant)
				require.Nil(t, result.RiderID)
				require.Equal(t, baseOrder.ID, result.Order.ID)
			},
		},
		{
			name: "SuccessDeliveringWithRider",
			input: UrgeOrderInput{
				UserID:          baseOrder.UserID,
				OrderID:         baseOrder.ID,
				RateLimitWindow: window,
				RateLimitMax:    3,
				Now:             now,
			},
			buildStubs: func(store *mockdb.MockStore) {
				order := baseOrder
				order.Status = "delivering"
				store.EXPECT().
					GetOrder(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(order, nil)
				store.EXPECT().
					CountRecentOrderStatusLogs(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(db.Delivery{RiderID: pgtype.Int8{Int64: 900, Valid: true}}, nil)
				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
			},
			check: func(t *testing.T, result UrgeOrderResult, err error) {
				require.NoError(t, err)
				require.False(t, result.NotifyMerchant)
				require.NotNil(t, result.RiderID)
				require.Equal(t, int64(900), *result.RiderID)
				require.Equal(t, baseOrder.ID, result.Order.ID)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			result, err := UrgeOrder(context.Background(), store, tc.input)
			tc.check(t, result, err)
		})
	}
}
