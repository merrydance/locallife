package logic

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPaymentOrderServiceGetPaymentOrder(t *testing.T) {
	input := GetPaymentOrderInput{UserID: 1001, PaymentOrderID: 2001}

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result GetPaymentOrderResult, err error)
	}{
		{
			name: "NotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ GetPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "payment order not found", reqErr.Err.Error())
			},
		},
		{
			name: "Forbidden",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID + 1}, nil)
			},
			check: func(t *testing.T, _ GetPaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "payment order does not belong to you", reqErr.Err.Error())
			},
		},
		{
			name: "Success",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: paymentStatusPending}, nil)
			},
			check: func(t *testing.T, result GetPaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, input.PaymentOrderID, result.PaymentOrder.ID)
				require.Equal(t, input.UserID, result.PaymentOrder.UserID)
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

			svc := NewPaymentOrderService(store, nil, nil)
			result, err := svc.GetPaymentOrder(context.Background(), input)
			tc.check(t, result, err)
		})
	}
}

func TestPaymentOrderServiceListPaymentOrders(t *testing.T) {
	baseInput := ListPaymentOrdersInput{UserID: 1002, PageID: 1, PageSize: 10}

	testCases := []struct {
		name       string
		input      ListPaymentOrdersInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result ListPaymentOrdersResult, err error)
	}{
		{
			name: "ByOrderID_NotFound_ReturnEmpty",
			input: func() ListPaymentOrdersInput {
				orderID := int64(3001)
				in := baseInput
				in.OrderID = &orderID
				return in
			}(),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
						OrderID:      pgtype.Int8{Int64: 3001, Valid: true},
						BusinessType: businessTypeOrder,
					}).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, result ListPaymentOrdersResult, err error) {
				require.NoError(t, err)
				require.Empty(t, result.PaymentOrders)
				require.Equal(t, int64(0), result.TotalCount)
			},
		},
		{
			name: "ByOrderID_OtherUser_ReturnEmpty",
			input: func() ListPaymentOrdersInput {
				orderID := int64(3002)
				in := baseInput
				in.OrderID = &orderID
				return in
			}(),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
						OrderID:      pgtype.Int8{Int64: 3002, Valid: true},
						BusinessType: businessTypeOrder,
					}).
					Times(1).
					Return(db.PaymentOrder{ID: 4001, UserID: baseInput.UserID + 10}, nil)
			},
			check: func(t *testing.T, result ListPaymentOrdersResult, err error) {
				require.NoError(t, err)
				require.Empty(t, result.PaymentOrders)
				require.Equal(t, int64(0), result.TotalCount)
			},
		},
		{
			name: "ByOrderID_Success",
			input: func() ListPaymentOrdersInput {
				orderID := int64(3003)
				in := baseInput
				in.OrderID = &orderID
				return in
			}(),
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
						OrderID:      pgtype.Int8{Int64: 3003, Valid: true},
						BusinessType: businessTypeOrder,
					}).
					Times(1).
					Return(db.PaymentOrder{ID: 4002, UserID: baseInput.UserID}, nil)
			},
			check: func(t *testing.T, result ListPaymentOrdersResult, err error) {
				require.NoError(t, err)
				require.Len(t, result.PaymentOrders, 1)
				require.Equal(t, int64(4002), result.PaymentOrders[0].ID)
				require.Equal(t, int64(1), result.TotalCount)
			},
		},
		{
			name:  "Paged_Success",
			input: ListPaymentOrdersInput{UserID: baseInput.UserID, PageID: 2, PageSize: 5},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListPaymentOrdersByUser(gomock.Any(), db.ListPaymentOrdersByUserParams{
						UserID: baseInput.UserID,
						Limit:  5,
						Offset: 5,
					}).
					Times(1).
					Return([]db.PaymentOrder{{ID: 5001}, {ID: 5002}}, nil)
			},
			check: func(t *testing.T, result ListPaymentOrdersResult, err error) {
				require.NoError(t, err)
				require.Len(t, result.PaymentOrders, 2)
				require.Equal(t, int64(2), result.TotalCount)
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

			svc := NewPaymentOrderService(store, nil, nil)
			result, err := svc.ListPaymentOrders(context.Background(), tc.input)
			tc.check(t, result, err)
		})
	}
}

func TestPaymentOrderServiceClosePaymentOrder(t *testing.T) {
	input := ClosePaymentOrderInput{UserID: 1003, PaymentOrderID: 2003}

	testCases := []struct {
		name             string
		buildStubs       func(store *mockdb.MockStore, client *mockwechat.MockPaymentClientInterface)
		buildEcomStubs   func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface)
		usePaymentClient bool
		useEcomClient    bool
		check            func(t *testing.T, result ClosePaymentOrderResult, err error)
	}{
		{
			name: "NotFound",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ ClosePaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "payment order not found", reqErr.Err.Error())
			},
		},
		{
			name: "Forbidden",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID + 1, Status: paymentStatusPending}, nil)
			},
			check: func(t *testing.T, _ ClosePaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "payment order does not belong to you", reqErr.Err.Error())
			},
		},
		{
			name: "InvalidStatus",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "paid"}, nil)
			},
			check: func(t *testing.T, _ ClosePaymentOrderResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "only pending payment orders can be closed", reqErr.Err.Error())
			},
		},
		{
			name: "Success_WithoutClient",
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: paymentStatusPending, OutTradeNo: "P202001010000000001", PrepayID: pgtype.Text{Valid: true, String: "prepay"}}, nil)
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "closed"}, nil)
			},
			check: func(t *testing.T, result ClosePaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "closed", result.PaymentOrder.Status)
			},
		},
		{
			name:             "Success_WithClient_CloseOrderCalled",
			usePaymentClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: paymentStatusPending, OutTradeNo: "P202001010000000002", PrepayID: pgtype.Text{Valid: true, String: "prepay"}}, nil)
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "closed"}, nil)
				client.EXPECT().
					CloseOrder(gomock.Any(), "P202001010000000002").
					Times(1).
					Return(nil)
			},
			check: func(t *testing.T, result ClosePaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "closed", result.PaymentOrder.Status)
			},
		},
		{
			name:          "Success_CombinedPayment_CloseCombineOrderCalled",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{
						ID:                input.PaymentOrderID,
						UserID:            input.UserID,
						Status:            paymentStatusPending,
						PaymentType:       "profit_sharing",
						OutTradeNo:        "CP202001010000000003",
						CombinedPaymentID: pgtype.Int8{Int64: 9001, Valid: true},
					}, nil)
			},
			buildEcomStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetCombinedPaymentOrder(gomock.Any(), int64(9001)).
					Times(1).
					Return(db.CombinedPaymentOrder{ID: 9001, CombineOutTradeNo: "OC123", Status: paymentStatusPending}, nil)
				store.EXPECT().
					ListCombinedPaymentSubOrders(gomock.Any(), int64(9001)).
					Times(1).
					Return([]db.CombinedPaymentSubOrder{{SubMchid: "1900000109", OutTradeNo: "CP202001010000000003"}}, nil)
				client.EXPECT().
					CloseCombineOrder(gomock.Any(), "OC123", []wechat.SubOrderClose{{MchID: "1900000109", OutTradeNo: "CP202001010000000003"}}).
					Times(1).
					Return(nil)
				store.EXPECT().
					CloseCombinedPaymentOrderTx(gomock.Any(), db.CloseCombinedPaymentOrderTxParams{
						CombinedPaymentOrderID: 9001,
						SubOrderOutTradeNos:    []string{"CP202001010000000003"},
					}).
					Times(1).
					Return(db.CloseCombinedPaymentOrderTxResult{}, nil)
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "closed"}, nil)
			},
			check: func(t *testing.T, result ClosePaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "closed", result.PaymentOrder.Status)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			paymentClient := mockwechat.NewMockPaymentClientInterface(ctrl)
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			tc.buildStubs(store, paymentClient)
			if tc.buildEcomStubs != nil {
				tc.buildEcomStubs(store, ecommerceClient)
			}

			var clientInterface wechat.PaymentClientInterface
			if tc.usePaymentClient {
				clientInterface = paymentClient
			}
			var ecommerceInterface wechat.EcommerceClientInterface
			if tc.useEcomClient {
				ecommerceInterface = ecommerceClient
			}

			svc := NewPaymentOrderService(store, clientInterface, ecommerceInterface)
			result, err := svc.ClosePaymentOrder(context.Background(), input)
			tc.check(t, result, err)
		})
	}
}
