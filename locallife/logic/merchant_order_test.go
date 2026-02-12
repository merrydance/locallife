package logic

import (
	"context"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAcceptMerchantOrder(t *testing.T) {
	input := MerchantOrderUpdateInput{MerchantID: 10, OrderID: 20, OperatorID: 30}
	baseOrder := db.Order{ID: input.OrderID, MerchantID: input.MerchantID, Status: "paid"}

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result MerchantOrderUpdateResult, err error)
	}{
		{
			name: "OrderNotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), input.OrderID).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ MerchantOrderUpdateResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "order not found", reqErr.Err.Error())
			},
		},
		{
			name: "OrderNotBelongToMerchant",
			buildStubs: func(store *mockdb.MockStore) {
				order := baseOrder
				order.MerchantID = input.MerchantID + 1
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), input.OrderID).
					Times(1).
					Return(order, nil)
			},
			check: func(t *testing.T, _ MerchantOrderUpdateResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "order does not belong to your merchant", reqErr.Err.Error())
			},
		},
		{
			name: "InvalidStatus",
			buildStubs: func(store *mockdb.MockStore) {
				order := baseOrder
				order.Status = "pending"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), input.OrderID).
					Times(1).
					Return(order, nil)
			},
			check: func(t *testing.T, _ MerchantOrderUpdateResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "only paid orders can be accepted", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), input.OrderID).
					Times(1).
					Return(baseOrder, nil)
				store.EXPECT().
					UpdateOrderStatusTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.UpdateOrderStatusTxParams) (db.UpdateOrderStatusTxResult, error) {
						require.Equal(t, "preparing", arg.NewStatus)
						require.Equal(t, "merchant", arg.OperatorType)
						require.NotNil(t, arg.NewFulfillmentStatus)
						require.Equal(t, "preparing", *arg.NewFulfillmentStatus)
						updated := baseOrder
						updated.Status = "preparing"
						return db.UpdateOrderStatusTxResult{Order: updated}, nil
					})
			},
			check: func(t *testing.T, result MerchantOrderUpdateResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "preparing", result.Order.Status)
				require.Equal(t, "paid", result.Previous.Status)
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

			result, err := AcceptMerchantOrder(context.Background(), store, input)
			tc.check(t, result, err)
		})
	}
}

func TestRejectMerchantOrder(t *testing.T) {
	input := MerchantOrderUpdateInput{MerchantID: 11, OrderID: 21, OperatorID: 31, Reason: "sold out"}
	baseOrder := db.Order{ID: input.OrderID, MerchantID: input.MerchantID, Status: "paid"}

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result MerchantOrderUpdateResult, err error)
	}{
		{
			name: "InvalidStatus",
			buildStubs: func(store *mockdb.MockStore) {
				order := baseOrder
				order.Status = "preparing"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), input.OrderID).
					Times(1).
					Return(order, nil)
			},
			check: func(t *testing.T, _ MerchantOrderUpdateResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "only paid orders can be rejected", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), input.OrderID).
					Times(1).
					Return(baseOrder, nil)
				store.EXPECT().
					UpdateOrderStatusTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.UpdateOrderStatusTxParams) (db.UpdateOrderStatusTxResult, error) {
						require.Equal(t, "cancelled", arg.NewStatus)
						require.Equal(t, "merchant", arg.OperatorType)
						require.Equal(t, "商户拒单：sold out", arg.Notes)
						require.NotNil(t, arg.NewFulfillmentStatus)
						require.Equal(t, "cancelled", *arg.NewFulfillmentStatus)
						updated := baseOrder
						updated.Status = "cancelled"
						return db.UpdateOrderStatusTxResult{Order: updated}, nil
					})
			},
			check: func(t *testing.T, result MerchantOrderUpdateResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "cancelled", result.Order.Status)
				require.Equal(t, "paid", result.Previous.Status)
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

			result, err := RejectMerchantOrder(context.Background(), store, input)
			tc.check(t, result, err)
		})
	}
}

func TestMarkMerchantOrderReady(t *testing.T) {
	input := MerchantOrderUpdateInput{MerchantID: 12, OrderID: 22, OperatorID: 32}
	baseOrder := db.Order{ID: input.OrderID, MerchantID: input.MerchantID, Status: "preparing"}

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result MerchantOrderUpdateResult, err error)
	}{
		{
			name: "InvalidStatus",
			buildStubs: func(store *mockdb.MockStore) {
				order := baseOrder
				order.Status = "paid"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), input.OrderID).
					Times(1).
					Return(order, nil)
			},
			check: func(t *testing.T, _ MerchantOrderUpdateResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "only preparing orders can be marked as ready", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), input.OrderID).
					Times(1).
					Return(baseOrder, nil)
				store.EXPECT().
					UpdateOrderStatusTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.UpdateOrderStatusTxParams) (db.UpdateOrderStatusTxResult, error) {
						require.Equal(t, "ready", arg.NewStatus)
						require.NotNil(t, arg.NewFulfillmentStatus)
						require.Equal(t, "ready", *arg.NewFulfillmentStatus)
						updated := baseOrder
						updated.Status = "ready"
						return db.UpdateOrderStatusTxResult{Order: updated}, nil
					})
			},
			check: func(t *testing.T, result MerchantOrderUpdateResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "ready", result.Order.Status)
				require.Equal(t, "preparing", result.Previous.Status)
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

			result, err := MarkMerchantOrderReady(context.Background(), store, input)
			tc.check(t, result, err)
		})
	}
}

func TestCompleteMerchantOrder(t *testing.T) {
	input := MerchantOrderUpdateInput{MerchantID: 13, OrderID: 23, OperatorID: 33}
	baseOrder := db.Order{ID: input.OrderID, MerchantID: input.MerchantID, Status: "ready", OrderType: "dine_in"}

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result MerchantOrderUpdateResult, err error)
	}{
		{
			name: "TakeoutNotAllowed",
			buildStubs: func(store *mockdb.MockStore) {
				order := baseOrder
				order.OrderType = "takeout"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), input.OrderID).
					Times(1).
					Return(order, nil)
			},
			check: func(t *testing.T, _ MerchantOrderUpdateResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "takeout orders cannot be completed by merchant", reqErr.Err.Error())
			},
		},
		{
			name: "InvalidStatus",
			buildStubs: func(store *mockdb.MockStore) {
				order := baseOrder
				order.Status = "preparing"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), input.OrderID).
					Times(1).
					Return(order, nil)
			},
			check: func(t *testing.T, _ MerchantOrderUpdateResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "only ready orders can be completed", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), input.OrderID).
					Times(1).
					Return(baseOrder, nil)
				store.EXPECT().
					CompleteOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.CompleteOrderTxParams) (db.CompleteOrderTxResult, error) {
						require.Equal(t, "merchant", arg.OperatorType)
						updated := baseOrder
						updated.Status = "completed"
						return db.CompleteOrderTxResult{Order: updated}, nil
					})
			},
			check: func(t *testing.T, result MerchantOrderUpdateResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "completed", result.Order.Status)
				require.Equal(t, "ready", result.Previous.Status)
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

			result, err := CompleteMerchantOrder(context.Background(), store, input)
			tc.check(t, result, err)
		})
	}
}
