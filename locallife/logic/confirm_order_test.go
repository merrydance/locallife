package logic

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestConfirmTakeoutOrder(t *testing.T) {
	baseOrder := db.Order{
		ID:         20,
		UserID:     300,
		MerchantID: 400,
		OrderNo:    "ORDER-002",
		OrderType:  "takeout",
		Status:     "rider_delivered",
	}

	testCases := []struct {
		name       string
		input      ConfirmOrderInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result ConfirmOrderResult, err error)
	}{
		{
			name:  "OrderNotFound",
			input: ConfirmOrderInput{UserID: baseOrder.UserID, OrderID: baseOrder.ID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ ConfirmOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "order not found", reqErr.Err.Error())
			},
		},
		{
			name:  "OrderNotBelongToUser",
			input: ConfirmOrderInput{UserID: baseOrder.UserID + 1, OrderID: baseOrder.ID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(baseOrder, nil)
			},
			check: func(t *testing.T, _ ConfirmOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "order does not belong to you", reqErr.Err.Error())
			},
		},
		{
			name:  "NotTakeoutOrder",
			input: ConfirmOrderInput{UserID: baseOrder.UserID, OrderID: baseOrder.ID},
			buildStubs: func(store *mockdb.MockStore) {
				order := baseOrder
				order.OrderType = "dine_in"
				store.EXPECT().
					GetOrder(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(order, nil)
			},
			check: func(t *testing.T, _ ConfirmOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "only takeout orders can be confirmed", reqErr.Err.Error())
			},
		},
		{
			name:  "AlreadyCompleted",
			input: ConfirmOrderInput{UserID: baseOrder.UserID, OrderID: baseOrder.ID},
			buildStubs: func(store *mockdb.MockStore) {
				order := baseOrder
				order.Status = "completed"
				store.EXPECT().
					GetOrder(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(order, nil)
			},
			check: func(t *testing.T, result ConfirmOrderResult, err error) {
				require.NoError(t, err)
				require.True(t, result.AlreadyCompleted)
				require.Equal(t, "completed", result.Order.Status)
				require.Nil(t, result.RiderID)
			},
		},
		{
			name:  "NotReadyStatus",
			input: ConfirmOrderInput{UserID: baseOrder.UserID, OrderID: baseOrder.ID},
			buildStubs: func(store *mockdb.MockStore) {
				order := baseOrder
				order.Status = "paid"
				store.EXPECT().
					GetOrder(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(order, nil)
			},
			check: func(t *testing.T, _ ConfirmOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "order is not ready for confirmation", reqErr.Err.Error())
			},
		},
		{
			name:  "SuccessRiderDelivered",
			input: ConfirmOrderInput{UserID: baseOrder.UserID, OrderID: baseOrder.ID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(baseOrder, nil)
				updated := baseOrder
				updated.Status = "completed"
				store.EXPECT().
					CompleteTakeoutOrderByUser(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(updated, nil)
				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(db.Delivery{RiderID: pgtype.Int8{Int64: 808, Valid: true}}, nil)
			},
			check: func(t *testing.T, result ConfirmOrderResult, err error) {
				require.NoError(t, err)
				require.False(t, result.AlreadyCompleted)
				require.Equal(t, "completed", result.Order.Status)
				require.NotNil(t, result.RiderID)
				require.Equal(t, int64(808), *result.RiderID)
			},
		},
		{
			name:  "SuccessUserDeliveredNoRider",
			input: ConfirmOrderInput{UserID: baseOrder.UserID, OrderID: baseOrder.ID},
			buildStubs: func(store *mockdb.MockStore) {
				order := baseOrder
				order.Status = "user_delivered"
				store.EXPECT().
					GetOrder(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(order, nil)
				updated := order
				updated.Status = "completed"
				store.EXPECT().
					CompleteTakeoutOrderByUser(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(updated, nil)
				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), baseOrder.ID).
					Times(1).
					Return(db.Delivery{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, result ConfirmOrderResult, err error) {
				require.NoError(t, err)
				require.False(t, result.AlreadyCompleted)
				require.Equal(t, "completed", result.Order.Status)
				require.Nil(t, result.RiderID)
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

			result, err := ConfirmTakeoutOrder(context.Background(), store, tc.input)
			tc.check(t, result, err)
		})
	}
}
