package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPaymentOrderServiceCreatePaymentOrder_RecreatesPendingOrderWhenAmountChanged(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		Status:      "pending",
		TotalAmount: 1000,
		BalancePaid: 300,
	}
	existingPayment := db.PaymentOrder{
		ID:                4001,
		UserID:            input.UserID,
		Status:            paymentStatusPending,
		PaymentType:       "profit_sharing",
		Amount:            1000,
		OutTradeNo:        "old-out-trade-no",
		CombinedPaymentID: pgtype.Int8{Int64: 5001, Valid: true},
	}
	newPayment := db.PaymentOrder{
		ID:          4002,
		UserID:      input.UserID,
		Status:      paymentStatusPending,
		PaymentType: "profit_sharing",
		Amount:      700,
		OutTradeNo:  "new-out-trade-no",
		Attach:      pgtype.Text{String: "order_id:2001;sub_mchid:sub-new", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetOrder(gomock.Any(), input.OrderID).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Times(1).
		Return(existingPayment, nil)
	store.EXPECT().
		UpdatePaymentOrderToClosed(gomock.Any(), int64(4001)).
		Times(1).
		Return(db.PaymentOrder{ID: 4001, UserID: input.UserID, Status: "closed"}, nil)
	store.EXPECT().
		UpdateCombinedPaymentOrderToClosed(gomock.Any(), int64(5001)).
		Times(1).
		Return(db.CombinedPaymentOrder{ID: 5001, Status: "closed"}, nil)
	store.EXPECT().
		GetUser(gomock.Any(), input.UserID).
		Times(1).
		Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), order.MerchantID).
		Times(1).
		Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().
		CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreatePartnerPaymentTxParams) (db.CreatePartnerPaymentTxResult, error) {
			require.Equal(t, input.UserID, arg.UserID)
			require.Equal(t, order.MerchantID, arg.MerchantID)
			require.Equal(t, input.OrderID, arg.OrderID)
			require.Equal(t, businessTypeOrder, arg.BusinessType)
			require.Equal(t, int64(700), arg.Amount)
			require.Equal(t, "order_id:2001", arg.Attach)
			return db.CreatePartnerPaymentTxResult{
				PaymentOrder: newPayment,
				SubMchID:     "sub-new",
			}, nil
		})
	ecommerceClient.EXPECT().
		CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *wechat.PartnerJSAPIOrderRequest) (*wechat.PartnerJSAPIOrderResponse, *wechat.JSAPIPayParams, error) {
			require.Equal(t, "sub-new", req.SubMchID)
			require.Equal(t, int64(700), req.TotalAmount)
			require.Equal(t, "Merchant A - Order Payment", req.Description)
			require.Equal(t, "order_id:2001;sub_mchid:sub-new", req.Attach)
			return &wechat.PartnerJSAPIOrderResponse{PrepayID: "prepay-new"}, &wechat.JSAPIPayParams{NonceStr: "nonce"}, nil
		})
	store.EXPECT().
		UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
			ID:       newPayment.ID,
			PrepayID: pgtype.Text{String: "prepay-new", Valid: true},
		}).
		Times(1).
		Return(db.PaymentOrder{ID: newPayment.ID, Amount: newPayment.Amount, PrepayID: pgtype.Text{String: "prepay-new", Valid: true}}, nil)

	svc := NewPaymentOrderService(store, nil, ecommerceClient)
	result, err := svc.CreatePaymentOrder(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(700), result.PaymentOrder.Amount)
	require.True(t, result.PaymentOrder.PrepayID.Valid)
	require.NotNil(t, result.PayParams)
}

func TestPaymentOrderServiceCreatePaymentOrder_PartnerPrepayUpdateFailureClosesByOutTradeNoFirst(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		Status:      "pending",
		TotalAmount: 1000,
	}
	txPayment := db.PaymentOrder{
		ID:          4002,
		UserID:      input.UserID,
		Status:      paymentStatusPending,
		PaymentType: "profit_sharing",
		Amount:      1000,
		OutTradeNo:  "new-out-trade-no",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetOrder(gomock.Any(), input.OrderID).
		Return(order, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetUser(gomock.Any(), input.UserID).
		Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), order.MerchantID).
		Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().
		CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).
		Return(db.CreatePartnerPaymentTxResult{
			PaymentOrder: txPayment,
			SubMchID:     "sub-new",
		}, nil)
	ecommerceClient.EXPECT().
		CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).
		Return(&wechat.PartnerJSAPIOrderResponse{PrepayID: "prepay-new"}, &wechat.JSAPIPayParams{NonceStr: "nonce"}, nil)
	store.EXPECT().
		UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
			ID:       txPayment.ID,
			PrepayID: pgtype.Text{String: "prepay-new", Valid: true},
		}).
		Return(db.PaymentOrder{}, errors.New("write failed"))
	store.EXPECT().
		UpdatePaymentOrderToFailed(gomock.Any(), txPayment.ID).
		Return(db.PaymentOrder{ID: txPayment.ID, Status: "failed"}, nil)
	ecommerceClient.EXPECT().
		ClosePartnerOrder(gomock.Any(), txPayment.OutTradeNo, "sub-new").
		Return(nil)

	svc := NewPaymentOrderService(store, nil, ecommerceClient)
	_, err := svc.CreatePaymentOrder(context.Background(), input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "update prepay id")
}

func TestMapReservationEcommerceError_ChangedTargetReturnsConflict(t *testing.T) {
	err := mapReservationEcommerceError(errors.New("reservation 42 payable amount changed"))
	reqErr := assertRequestError(t, err)
	require.Equal(t, 409, reqErr.Status)
	require.Equal(t, "payment target changed, please retry", reqErr.Err.Error())
}

func TestPaymentOrderServiceCreatePaymentOrder_ReservationPendingModeMismatchSupersedesExisting(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeReservation,
		ClientIP:     "127.0.0.1",
	}
	reservation := db.TableReservation{
		ID:            input.OrderID,
		UserID:        input.UserID,
		MerchantID:    3001,
		Status:        "pending",
		PaymentMode:   paymentModeDeposit,
		DepositAmount: 1000,
	}
	existingPayment := db.PaymentOrder{
		ID:          4001,
		UserID:      input.UserID,
		Status:      paymentStatusPending,
		PaymentType: "profit_sharing",
		Amount:      1000,
		OutTradeNo:  "old-out-trade-no",
		Attach:      pgtype.Text{String: "reservation_id:2001", Valid: true},
	}
	newPayment := db.PaymentOrder{
		ID:          4002,
		UserID:      input.UserID,
		Status:      paymentStatusPending,
		PaymentType: "profit_sharing",
		Amount:      1000,
		OutTradeNo:  "new-out-trade-no",
		Attach:      pgtype.Text{String: buildReservationPaymentAttach(input.OrderID, paymentModeDeposit) + ";sub_mchid:sub-new", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().GetTableReservation(gomock.Any(), input.OrderID).Return(reservation, nil)
	store.EXPECT().GetLatestPaymentOrderByReservation(gomock.Any(), db.GetLatestPaymentOrderByReservationParams{
		ReservationID: pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType:  businessTypeReservation,
	}).Return(existingPayment, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), existingPayment.ID).Return(db.PaymentOrder{ID: existingPayment.ID, Status: "closed"}, nil)
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), reservation.MerchantID).Return(db.Merchant{ID: reservation.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreatePartnerPaymentTxParams) (db.CreatePartnerPaymentTxResult, error) {
		require.Equal(t, buildReservationPaymentAttach(input.OrderID, paymentModeDeposit), arg.Attach)
		require.Equal(t, paymentModeDeposit, arg.PaymentMode)
		return db.CreatePartnerPaymentTxResult{PaymentOrder: newPayment, SubMchID: "sub-new"}, nil
	})
	ecommerceClient.EXPECT().CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechat.PartnerJSAPIOrderRequest) (*wechat.PartnerJSAPIOrderResponse, *wechat.JSAPIPayParams, error) {
		require.Equal(t, buildReservationPaymentAttach(input.OrderID, paymentModeDeposit)+";sub_mchid:sub-new", req.Attach)
		require.True(t, req.ProfitSharing)
		return &wechat.PartnerJSAPIOrderResponse{PrepayID: "prepay-new"}, &wechat.JSAPIPayParams{NonceStr: "nonce"}, nil
	})
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
		ID:       newPayment.ID,
		PrepayID: pgtype.Text{String: "prepay-new", Valid: true},
	}).Return(db.PaymentOrder{ID: newPayment.ID, PrepayID: pgtype.Text{String: "prepay-new", Valid: true}}, nil)

	svc := NewPaymentOrderService(store, nil, ecommerceClient)
	result, err := svc.CreatePaymentOrder(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(4002), result.PaymentOrder.ID)
	require.NotNil(t, result.PayParams)
}

func TestPaymentOrderServiceCreatePaymentOrder_TakeawayUsesPartnerSingleWithoutProfitSharing(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		OrderType:   orderTypeTakeaway,
		Status:      "pending",
		TotalAmount: 1000,
	}
	txPayment := db.PaymentOrder{
		ID:          4002,
		UserID:      input.UserID,
		Status:      paymentStatusPending,
		PaymentType: "profit_sharing",
		Amount:      1000,
		OutTradeNo:  "new-out-trade-no",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: businessTypeOrder,
	}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Return(db.CreatePartnerPaymentTxResult{PaymentOrder: txPayment, SubMchID: "sub-new"}, nil)
	ecommerceClient.EXPECT().CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechat.PartnerJSAPIOrderRequest) (*wechat.PartnerJSAPIOrderResponse, *wechat.JSAPIPayParams, error) {
		require.False(t, req.ProfitSharing)
		require.Equal(t, "Merchant A - Order Payment", req.Description)
		return &wechat.PartnerJSAPIOrderResponse{PrepayID: "prepay-new"}, &wechat.JSAPIPayParams{NonceStr: "nonce"}, nil
	})
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
		ID:       txPayment.ID,
		PrepayID: pgtype.Text{String: "prepay-new", Valid: true},
	}).Return(db.PaymentOrder{ID: txPayment.ID, PrepayID: pgtype.Text{String: "prepay-new", Valid: true}}, nil)

	svc := NewPaymentOrderService(store, nil, ecommerceClient)
	result, err := svc.CreatePaymentOrder(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, txPayment.ID, result.PaymentOrder.ID)
	require.NotNil(t, result.PayParams)
}

func TestPaymentOrderServiceCreatePaymentOrder_ReservationLinkedDineInUsesProfitSharingSinglePay(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:            input.OrderID,
		UserID:        input.UserID,
		MerchantID:    3001,
		OrderType:     orderTypeDineIn,
		ReservationID: pgtype.Int8{Int64: 9001, Valid: true},
		Status:        "pending",
		TotalAmount:   1000,
	}
	txPayment := db.PaymentOrder{
		ID:            4002,
		UserID:        input.UserID,
		Status:        paymentStatusPending,
		PaymentType:   "profit_sharing",
		Amount:        1000,
		OutTradeNo:    "new-out-trade-no",
		ReservationID: pgtype.Int8{Int64: 9001, Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: businessTypeOrder,
	}).Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetUser(gomock.Any(), input.UserID).Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Return(db.CreatePartnerPaymentTxResult{PaymentOrder: txPayment, SubMchID: "sub-new"}, nil)
	ecommerceClient.EXPECT().CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechat.PartnerJSAPIOrderRequest) (*wechat.PartnerJSAPIOrderResponse, *wechat.JSAPIPayParams, error) {
		require.True(t, req.ProfitSharing)
		require.Equal(t, "Merchant A - Order Payment", req.Description)
		return &wechat.PartnerJSAPIOrderResponse{PrepayID: "prepay-new"}, &wechat.JSAPIPayParams{NonceStr: "nonce"}, nil
	})
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
		ID:       txPayment.ID,
		PrepayID: pgtype.Text{String: "prepay-new", Valid: true},
	}).Return(db.PaymentOrder{ID: txPayment.ID, PrepayID: pgtype.Text{String: "prepay-new", Valid: true}}, nil)

	svc := NewPaymentOrderService(store, nil, ecommerceClient)
	result, err := svc.CreatePaymentOrder(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, txPayment.ID, result.PaymentOrder.ID)
	require.NotNil(t, result.PayParams)
}

func TestPaymentOrderServiceCreatePaymentOrder_ConcurrentPendingOrderWithoutPayParamsReturnsConflict(t *testing.T) {
	input := CreatePaymentOrderInput{
		UserID:       1001,
		OrderID:      2001,
		PaymentType:  paymentTypeMiniProgram,
		BusinessType: businessTypeOrder,
		ClientIP:     "127.0.0.1",
	}
	order := db.Order{
		ID:          input.OrderID,
		UserID:      input.UserID,
		MerchantID:  3001,
		Status:      "pending",
		TotalAmount: 1000,
	}
	pendingPayment := db.PaymentOrder{
		ID:          4002,
		UserID:      input.UserID,
		Status:      paymentStatusPending,
		PaymentType: "profit_sharing",
		Amount:      1000,
		OutTradeNo:  "new-out-trade-no",
		PrepayID:    pgtype.Text{String: "prepay-pending", Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		GetOrder(gomock.Any(), input.OrderID).
		Return(order, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetUser(gomock.Any(), input.UserID).
		Return(db.User{ID: input.UserID, WechatOpenid: "openid"}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), order.MerchantID).
		Return(db.Merchant{ID: order.MerchantID, Name: "Merchant A"}, nil)
	store.EXPECT().
		CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).
		Return(db.CreatePartnerPaymentTxResult{}, db.ErrOrderPendingPaymentConflict)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
			BusinessType: businessTypeOrder,
		}).
		Times(outTradeNoMaxRetry).
		Return(pendingPayment, nil)
	ecommerceClient.EXPECT().
		GenerateJSAPIPayParams("prepay-pending").
		Times(outTradeNoMaxRetry).
		Return(nil, errors.New("signing unavailable"))

	svc := NewPaymentOrderService(store, nil, ecommerceClient)
	_, err := svc.CreatePaymentOrder(context.Background(), input)
	reqErr := assertRequestError(t, err)
	require.Equal(t, 409, reqErr.Status)
	require.Equal(t, "payment order is still preparing, please retry", reqErr.Err.Error())
}

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
					CloseCombineOrder(gomock.Any(), "OC123", []wechat.SubOrderClose{{SubMchID: "1900000109", OutTradeNo: "CP202001010000000003"}}).
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
		{
			name:          "Success_PartnerSingle_ClosePartnerOrderCalledWithOutTradeNoFirst",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{
						ID:          input.PaymentOrderID,
						UserID:      input.UserID,
						Status:      paymentStatusPending,
						PaymentType: "profit_sharing",
						OutTradeNo:  "RS202001010000000003",
						OrderID:     pgtype.Int8{Int64: 7001, Valid: true},
						PrepayID:    pgtype.Text{String: "prepay", Valid: true},
					}, nil)
			},
			buildEcomStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					GetOrder(gomock.Any(), int64(7001)).
					Times(1).
					Return(db.Order{ID: 7001, MerchantID: 8801}, nil)
				store.EXPECT().
					GetMerchantPaymentConfig(gomock.Any(), int64(8801)).
					Times(1).
					Return(db.MerchantPaymentConfig{MerchantID: 8801, SubMchID: "1900000109"}, nil)
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "closed"}, nil)
				client.EXPECT().
					ClosePartnerOrder(gomock.Any(), "RS202001010000000003", "1900000109").
					Times(1).
					Return(nil)
			},
			check: func(t *testing.T, result ClosePaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, "closed", result.PaymentOrder.Status)
			},
		},
		{
			name:          "PartnerSingle_ResolveSubMchFailureStillClosesLocalState",
			useEcomClient: true,
			buildStubs: func(store *mockdb.MockStore, client *mockwechat.MockPaymentClientInterface) {
				store.EXPECT().
					GetPaymentOrder(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{
						ID:          input.PaymentOrderID,
						UserID:      input.UserID,
						Status:      paymentStatusPending,
						PaymentType: "profit_sharing",
						OutTradeNo:  "RS202001010000000004",
						OrderID:     pgtype.Int8{Int64: 7002, Valid: true},
						PrepayID:    pgtype.Text{String: "prepay", Valid: true},
					}, nil)
			},
			buildEcomStubs: func(store *mockdb.MockStore, client *mockwechat.MockEcommerceClientInterface) {
				store.EXPECT().
					UpdatePaymentOrderToClosed(gomock.Any(), input.PaymentOrderID).
					Times(1).
					Return(db.PaymentOrder{ID: input.PaymentOrderID, UserID: input.UserID, Status: "closed"}, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), int64(7002)).
					Times(1).
					Return(db.Order{}, errors.New("resolve failed"))
			},
			check: func(t *testing.T, result ClosePaymentOrderResult, err error) {
				require.NoError(t, err)
				require.Equal(t, input.PaymentOrderID, result.PaymentOrder.ID)
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
