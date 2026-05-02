package logic

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatmock "github.com/merrydance/locallife/wechat/mock"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestReplaceReservationOrder_OrderNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetOrderForUpdate(gomock.Any(), int64(10)).
		Times(1).
		Return(db.Order{}, db.ErrRecordNotFound)

	_, err := ReplaceReservationOrder(
		context.Background(),
		store,
		nil,
		ReplaceOrderInput{UserID: 1, OrderID: 10},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
	)
	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "order not found", reqErr.Err.Error())
}

func TestReplaceReservationOrder_DeltaPositive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{createPaymentResponse: &ospcontracts.PaymentPrepayResponse{PrepayID: "prepay-replace-1"}}

	orderID := int64(10)
	userID := int64(20)
	reservationID := int64(30)
	merchantID := int64(40)
	tableID := int64(50)
	dishID := int64(60)

	oldOrder := db.Order{
		ID:            orderID,
		UserID:        userID,
		OrderType:     "reservation",
		Status:        "paid",
		TotalAmount:   1000,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}
	reservation := db.TableReservation{
		ID:          reservationID,
		UserID:      userID,
		MerchantID:  merchantID,
		TableID:     tableID,
		PaymentMode: "full",
		Status:      "paid",
	}
	session := db.DiningSession{ID: 77, UserID: userID}
	dish := db.Dish{ID: dishID, MerchantID: merchantID, Name: "Noodles", Price: 1500, IsOnline: true, IsAvailable: true}
	paymentOrder := db.PaymentOrder{
		ID:             222,
		OrderID:        pgtype.Int8{Int64: 111, Valid: true},
		ReservationID:  pgtype.Int8{Int64: reservationID, Valid: true},
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		BusinessType:   "order",
		Amount:         500,
		OutTradeNo:     "RO111202603230001",
	}

	store.EXPECT().
		GetOrderForUpdate(gomock.Any(), orderID).
		Times(1).
		Return(oldOrder, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), reservationID).
		Times(1).
		Return(reservation, nil)
	store.EXPECT().
		GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetDish(gomock.Any(), dishID).
		Times(1).
		Return(dish, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
		Return([]db.DiscountRule{}, nil)
	store.EXPECT().
		ReplaceOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.ReplaceOrderTxParams) (db.ReplaceOrderTxResult, error) {
			require.NotEmpty(t, arg.CreateOrderParams.OrderNo)
			require.Equal(t, merchantID, arg.CreateOrderParams.MerchantID)
			require.Equal(t, int64(1500), arg.CreateOrderParams.TotalAmount)
			require.Equal(t, "pending", arg.CreateOrderParams.Status)
			require.Equal(t, "scheduled", arg.CreateOrderParams.FulfillmentStatus)
			newOrder := db.Order{
				ID:                111,
				UserID:            arg.CreateOrderParams.UserID,
				MerchantID:        arg.CreateOrderParams.MerchantID,
				OrderType:         arg.CreateOrderParams.OrderType,
				Status:            arg.CreateOrderParams.Status,
				FulfillmentStatus: arg.CreateOrderParams.FulfillmentStatus,
				ReservationID:     arg.CreateOrderParams.ReservationID,
				Subtotal:          arg.CreateOrderParams.Subtotal,
				DiscountAmount:    arg.CreateOrderParams.DiscountAmount,
				TotalAmount:       arg.CreateOrderParams.TotalAmount,
			}
			return db.ReplaceOrderTxResult{NewOrder: newOrder}, nil
		})
	store.EXPECT().
		GetUser(gomock.Any(), userID).
		Times(1).
		Return(db.User{ID: userID, WechatOpenid: "openid-1"}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(db.Merchant{ID: merchantID, Name: "Test Merchant"}, nil)
	store.EXPECT().
		CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreatePartnerPaymentTxParams) (db.CreatePartnerPaymentTxResult, error) {
			require.Equal(t, userID, arg.UserID)
			require.Equal(t, merchantID, arg.MerchantID)
			require.Equal(t, int64(111), arg.OrderID)
			require.Equal(t, reservationID, arg.ReservationID)
			require.Equal(t, businessTypeOrder, arg.BusinessType)
			require.Equal(t, int64(500), arg.Amount)
			require.Equal(t, "order_id:111", arg.Attach)
			return db.CreatePartnerPaymentTxResult{PaymentOrder: paymentOrder, SubMchID: "sub-ordinary-replace"}, nil
		})
	store.EXPECT().
		UpdatePaymentOrderPrepayId(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.UpdatePaymentOrderPrepayIdParams) (db.PaymentOrder, error) {
			updated := paymentOrder
			updated.PrepayID = arg.PrepayID
			return updated, nil
		})
	expectReplaceReservationPaymentCommand(t, store, paymentOrder.ID, paymentOrder.OutTradeNo, "prepay-replace-1", db.ExternalPaymentBusinessOwnerReservation, db.ExternalPaymentCommandStatusAccepted, "", db.PaymentChannelOrdinaryServiceProvider, 9801)
	result, err := ReplaceReservationOrderWithOrdinaryServiceProvider(
		context.Background(),
		store,
		nil,
		ordinaryClient,
		ReplaceOrderInput{
			UserID:  userID,
			OrderID: orderID,
			Items: []OrderItemInput{
				{DishID: &dishID, Quantity: 1},
			},
		},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
	)
	require.NoError(t, err)
	require.Equal(t, int64(500), result.Delta)
	require.NotNil(t, result.PaymentOrderID)
	require.Equal(t, int64(222), *result.PaymentOrderID)
	require.False(t, result.RefundInitiated)
	require.NotNil(t, ordinaryClient.createPaymentRequest)
	require.Equal(t, "sub-ordinary-replace", ordinaryClient.createPaymentRequest.SubMchID)
	require.Equal(t, "openid-1", ordinaryClient.createPaymentRequest.Payer.SpOpenID)
	require.Equal(t, int64(500), ordinaryClient.createPaymentRequest.Amount.Total)
	require.Equal(t, "Test Merchant - Reservation Adjustment", ordinaryClient.createPaymentRequest.Description)
	require.Equal(t, "order_id:111", ordinaryClient.createPaymentRequest.Attach)
	require.True(t, ordinaryClient.createPaymentRequest.SettleInfo.ProfitSharing)
}

func TestCreateReplaceOrderOrdinaryServiceProviderPayment_LogsCleanupFailureAfterPrepayUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{}
	userID := int64(20)
	order := db.Order{ID: 117, MerchantID: 40, OrderType: "dine_in", ReservationID: pgtype.Int8{Int64: 30, Valid: true}}
	paymentOrder := db.PaymentOrder{ID: 227, OrderID: pgtype.Int8{Int64: order.ID, Valid: true}, ReservationID: order.ReservationID, PaymentChannel: db.PaymentChannelOrdinaryServiceProvider, Amount: 500, OutTradeNo: "RO117202604250001"}

	store.EXPECT().GetUser(gomock.Any(), userID).Return(db.User{ID: userID, WechatOpenid: "openid-1"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, Name: "Test Merchant"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Return(db.CreatePartnerPaymentTxResult{PaymentOrder: paymentOrder, SubMchID: "sub-ordinary-replace"}, nil)
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), db.UpdatePaymentOrderPrepayIdParams{
		ID:       paymentOrder.ID,
		PrepayID: pgtype.Text{String: "prepay-ordinary", Valid: true},
	}).Return(db.PaymentOrder{}, errors.New("update prepay failed"))
	store.EXPECT().UpdatePaymentOrderToFailed(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{}, errors.New("mark replace failed"))

	_, err := createReplaceOrderOrdinaryServiceProviderPayment(context.Background(), store, ordinaryClient, userID, order, 500)

	require.Error(t, err)
	require.Contains(t, err.Error(), "update prepay id")
	require.Contains(t, logs.String(), "failed to mark replace reservation payment order failed after prepay update failure")
	require.Contains(t, logs.String(), "mark replace failed")
	require.NotNil(t, ordinaryClient.closePaymentRequest)
}

func TestReplaceReservationOrder_DeltaPositivePaymentRejectedRecordsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{createPaymentErr: &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "PARAM_ERROR", Message: "invalid request"}}

	orderID := int64(15)
	userID := int64(25)
	reservationID := int64(35)
	merchantID := int64(45)
	tableID := int64(55)
	dishID := int64(65)
	paymentOrder := db.PaymentOrder{
		ID:             225,
		OrderID:        pgtype.Int8{Int64: 115, Valid: true},
		ReservationID:  pgtype.Int8{Int64: reservationID, Valid: true},
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		BusinessType:   businessTypeOrder,
		Amount:         500,
		OutTradeNo:     "RO115202604250001",
	}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), orderID).Return(db.Order{ID: orderID, UserID: userID, OrderType: "reservation", Status: "paid", TotalAmount: 1000, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}}, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(db.TableReservation{ID: reservationID, UserID: userID, MerchantID: merchantID, TableID: tableID, PaymentMode: "full", Status: "paid"}, nil)
	store.EXPECT().GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return(db.DiningSession{ID: 85, UserID: userID}, nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(db.Dish{ID: dishID, MerchantID: merchantID, Name: "Tea", Price: 1500, IsOnline: true, IsAvailable: true}, nil)
	store.EXPECT().ListActiveDiscountRules(gomock.Any(), merchantID).Return([]db.DiscountRule{}, nil)
	store.EXPECT().ReplaceOrderTx(gomock.Any(), gomock.Any()).Return(db.ReplaceOrderTxResult{NewOrder: db.Order{ID: 115, UserID: userID, MerchantID: merchantID, OrderType: "dine_in", Status: "pending", ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}, TotalAmount: 1500}}, nil)
	store.EXPECT().GetUser(gomock.Any(), userID).Return(db.User{ID: userID, WechatOpenid: "openid-25"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchantID).Return(db.Merchant{ID: merchantID, Name: "Test Merchant"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Return(db.CreatePartnerPaymentTxResult{PaymentOrder: paymentOrder, SubMchID: "sub-ordinary-replace"}, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{ID: paymentOrder.ID, Status: "closed"}, nil)
	expectReplaceReservationPaymentCommand(t, store, paymentOrder.ID, paymentOrder.OutTradeNo, "", db.ExternalPaymentBusinessOwnerReservation, db.ExternalPaymentCommandStatusRejected, "PARAM_ERROR", db.PaymentChannelOrdinaryServiceProvider, 9802)

	_, err := ReplaceReservationOrderWithOrdinaryServiceProvider(
		context.Background(),
		store,
		nil,
		ordinaryClient,
		ReplaceOrderInput{UserID: userID, OrderID: orderID, Items: []OrderItemInput{{DishID: &dishID, Quantity: 1}}},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
	)
	require.Error(t, err)
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, reqErr.Status)
	require.Contains(t, reqErr.Err.Error(), "请联系平台处理")
}

func TestReplaceReservationOrder_DeltaPositivePaymentRejectedSkipsCommandWhenCloseFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{createPaymentErr: &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "PARAM_ERROR", Message: "invalid request"}}

	orderID := int64(16)
	userID := int64(26)
	reservationID := int64(36)
	merchantID := int64(46)
	tableID := int64(56)
	dishID := int64(66)
	paymentOrder := db.PaymentOrder{ID: 226, OrderID: pgtype.Int8{Int64: 116, Valid: true}, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}, PaymentType: "profit_sharing", PaymentChannel: db.PaymentChannelOrdinaryServiceProvider, BusinessType: businessTypeOrder, Amount: 500, OutTradeNo: "RO116202604250001"}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), orderID).Return(db.Order{ID: orderID, UserID: userID, OrderType: "reservation", Status: "paid", TotalAmount: 1000, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}}, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(db.TableReservation{ID: reservationID, UserID: userID, MerchantID: merchantID, TableID: tableID, PaymentMode: "full", Status: "paid"}, nil)
	store.EXPECT().GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return(db.DiningSession{ID: 86, UserID: userID}, nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(db.Dish{ID: dishID, MerchantID: merchantID, Name: "Tea", Price: 1500, IsOnline: true, IsAvailable: true}, nil)
	store.EXPECT().ListActiveDiscountRules(gomock.Any(), merchantID).Return([]db.DiscountRule{}, nil)
	store.EXPECT().ReplaceOrderTx(gomock.Any(), gomock.Any()).Return(db.ReplaceOrderTxResult{NewOrder: db.Order{ID: 116, UserID: userID, MerchantID: merchantID, OrderType: "dine_in", Status: "pending", ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}, TotalAmount: 1500}}, nil)
	store.EXPECT().GetUser(gomock.Any(), userID).Return(db.User{ID: userID, WechatOpenid: "openid-26"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchantID).Return(db.Merchant{ID: merchantID, Name: "Test Merchant"}, nil)
	store.EXPECT().CreatePartnerPaymentTx(gomock.Any(), gomock.Any()).Return(db.CreatePartnerPaymentTxResult{PaymentOrder: paymentOrder, SubMchID: "sub-ordinary-replace"}, nil)
	store.EXPECT().UpdatePaymentOrderToClosed(gomock.Any(), paymentOrder.ID).Return(db.PaymentOrder{}, errors.New("close failed"))

	_, err := ReplaceReservationOrderWithOrdinaryServiceProvider(
		context.Background(),
		store,
		nil,
		ordinaryClient,
		ReplaceOrderInput{UserID: userID, OrderID: orderID, Items: []OrderItemInput{{DishID: &dishID, Quantity: 1}}},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
	)
	require.Error(t, err)
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, reqErr.Status)
	require.Contains(t, reqErr.Err.Error(), "请联系平台处理")
}

func TestReplaceReservationOrder_DeltaPositiveMissingOrdinaryClientFailsBeforeReplacement(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	orderID := int64(18)
	userID := int64(28)
	reservationID := int64(38)
	merchantID := int64(48)
	tableID := int64(58)
	dishID := int64(68)

	store.EXPECT().GetOrderForUpdate(gomock.Any(), orderID).Return(db.Order{
		ID:            orderID,
		UserID:        userID,
		OrderType:     "reservation",
		Status:        "paid",
		TotalAmount:   1000,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(db.TableReservation{
		ID:          reservationID,
		UserID:      userID,
		MerchantID:  merchantID,
		TableID:     tableID,
		PaymentMode: "full",
		Status:      "paid",
	}, nil)
	store.EXPECT().GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return(db.DiningSession{ID: 84, UserID: userID}, nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(db.Dish{ID: dishID, MerchantID: merchantID, Name: "Tea", Price: 1500, IsOnline: true, IsAvailable: true}, nil)
	store.EXPECT().ListActiveDiscountRules(gomock.Any(), merchantID).Return([]db.DiscountRule{}, nil)

	_, err := ReplaceReservationOrderWithOrdinaryServiceProvider(
		context.Background(),
		store,
		nil,
		nil,
		ReplaceOrderInput{UserID: userID, OrderID: orderID, Items: []OrderItemInput{{DishID: &dishID, Quantity: 1}}},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ordinary service provider client")
	require.Contains(t, err.Error(), "not configured")
}

func TestReplaceReservationOrder_RejectsNonWechatServiceProviderRefundChain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)

	orderID := int64(12)
	userID := int64(22)
	reservationID := int64(32)
	merchantID := int64(42)
	tableID := int64(52)
	dishID := int64(62)

	oldOrder := db.Order{
		ID:            orderID,
		UserID:        userID,
		OrderType:     "reservation",
		Status:        "paid",
		TotalAmount:   1000,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}
	reservation := db.TableReservation{
		ID:          reservationID,
		UserID:      userID,
		MerchantID:  merchantID,
		TableID:     tableID,
		PaymentMode: "full",
		Status:      "confirmed",
	}
	session := db.DiningSession{ID: 78, UserID: userID}
	dish := db.Dish{ID: dishID, MerchantID: merchantID, Name: "Rice", Price: 500, IsOnline: true, IsAvailable: true}

	store.EXPECT().
		GetOrderForUpdate(gomock.Any(), orderID).
		Times(1).
		Return(oldOrder, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), reservationID).
		Times(1).
		Return(reservation, nil)
	store.EXPECT().
		GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetDish(gomock.Any(), dishID).
		Times(1).
		Return(dish, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
		Return([]db.DiscountRule{}, nil)
	store.EXPECT().
		GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
		Times(1).
		Return([]db.PaymentOrder{{ID: 444, Status: "paid", OutTradeNo: "out_1", Amount: 1000, PaymentType: "miniprogram", BusinessType: businessTypeReservation, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}}}, nil)
	store.EXPECT().
		GetTotalRefundedByPaymentOrder(gomock.Any(), int64(444)).
		Times(1).
		Return(int64(0), nil)

	_, err := ReplaceReservationOrder(
		context.Background(),
		store,
		ecommerceClient,
		ReplaceOrderInput{
			UserID:  userID,
			OrderID: orderID,
			Items: []OrderItemInput{
				{DishID: &dishID, Quantity: 1},
			},
		},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
	)
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.Equal(t, "当前主营业务支付单不属于微信服务商链路，无法处理改菜退款，请联系平台处理", reqErr.Err.Error())
}

func TestReplaceReservationOrder_RefundProfitSharing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)

	orderID := int64(12)
	userID := int64(22)
	reservationID := int64(32)
	merchantID := int64(42)
	tableID := int64(52)
	dishID := int64(62)

	oldOrder := db.Order{
		ID:            orderID,
		UserID:        userID,
		MerchantID:    merchantID,
		OrderType:     "reservation",
		Status:        "paid",
		TotalAmount:   1000,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}
	reservation := db.TableReservation{
		ID:          reservationID,
		UserID:      userID,
		MerchantID:  merchantID,
		TableID:     tableID,
		PaymentMode: "full",
		Status:      "confirmed",
	}
	session := db.DiningSession{ID: 78, UserID: userID}
	dish := db.Dish{ID: dishID, MerchantID: merchantID, Name: "Rice", Price: 500, IsOnline: true, IsAvailable: true}
	capturedOutRefundNo := ""

	store.EXPECT().
		GetOrderForUpdate(gomock.Any(), orderID).
		Times(1).
		Return(oldOrder, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), reservationID).
		Times(1).
		Return(reservation, nil)
	store.EXPECT().
		GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetDish(gomock.Any(), dishID).
		Times(1).
		Return(dish, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
		Return([]db.DiscountRule{}, nil)
	store.EXPECT().
		ReplaceOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.ReplaceOrderTxParams) (db.ReplaceOrderTxResult, error) {
			newOrder := db.Order{
				ID:                333,
				UserID:            arg.CreateOrderParams.UserID,
				MerchantID:        arg.CreateOrderParams.MerchantID,
				OrderType:         arg.CreateOrderParams.OrderType,
				Status:            arg.CreateOrderParams.Status,
				FulfillmentStatus: arg.CreateOrderParams.FulfillmentStatus,
				Subtotal:          arg.CreateOrderParams.Subtotal,
				DiscountAmount:    arg.CreateOrderParams.DiscountAmount,
				TotalAmount:       arg.CreateOrderParams.TotalAmount,
			}
			return db.ReplaceOrderTxResult{NewOrder: newOrder}, nil
		})
	store.EXPECT().
		GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
		Times(1).
		Return([]db.PaymentOrder{{ID: 444, Status: "paid", OutTradeNo: "out_1", Amount: 1000, PaymentType: "profit_sharing", PaymentChannel: db.PaymentChannelEcommerce, BusinessType: businessTypeReservation, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}}}, nil)
	store.EXPECT().
		GetTotalRefundedByPaymentOrder(gomock.Any(), int64(444)).
		Times(1).
		Return(int64(0), nil)
	store.EXPECT().
		CreateRefundOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderParams) (db.RefundOrder, error) {
			require.Equal(t, int64(500), arg.RefundAmount)
			require.Equal(t, int64(444), arg.PaymentOrderID)
			require.Equal(t, "profit_sharing", arg.RefundType)
			require.NotEmpty(t, arg.OutRefundNo)
			capturedOutRefundNo = arg.OutRefundNo
			return db.RefundOrder{ID: 555}, nil
		})
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), merchantID).
		Times(1).
		Return(db.MerchantPaymentConfig{MerchantID: merchantID, SubMchID: "1900000109", Status: "active"}, nil)
	ecommerceClient.EXPECT().
		CreateEcommerceRefund(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *wechat.EcommerceRefundRequest) (*wechat.EcommerceRefundResponse, error) {
			require.Equal(t, "1900000109", req.SubMchID)
			require.Equal(t, "out_1", req.OutTradeNo)
			require.Equal(t, int64(500), req.RefundAmount)
			return &wechat.EcommerceRefundResponse{RefundID: "erefund_replace_1"}, nil
		})
	store.EXPECT().
		UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
			ID:       555,
			RefundID: pgtype.Text{String: "erefund_replace_1", Valid: true},
		}).
		Times(1).
		Return(db.RefundOrder{}, nil)
	expectReplaceReservationRefundCommand(t, store, 555, &capturedOutRefundNo, "erefund_replace_1", db.ExternalPaymentCommandStatusAccepted, "", db.PaymentChannelEcommerce, db.ExternalPaymentCapabilityEcommerceRefund, 9301)

	result, err := ReplaceReservationOrder(
		context.Background(),
		store,
		ecommerceClient,
		ReplaceOrderInput{
			UserID:  userID,
			OrderID: orderID,
			Items: []OrderItemInput{{
				DishID:   &dishID,
				Quantity: 1,
			}},
		},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
	)
	require.NoError(t, err)
	require.Equal(t, int64(-500), result.Delta)
	require.Nil(t, result.PaymentOrderID)
	require.True(t, result.RefundInitiated)
}

func TestReplaceReservationOrder_OrdinaryRefundAcceptedRecordsOrdinaryCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ordinaryClient := &fakeOrdinaryPaymentClient{createRefundResponse: &ospcontracts.RefundResponse{RefundID: "orefund_replace_1"}}

	orderID := int64(16)
	userID := int64(26)
	reservationID := int64(36)
	merchantID := int64(46)
	tableID := int64(56)
	dishID := int64(66)
	capturedOutRefundNo := ""

	oldOrder := db.Order{
		ID:            orderID,
		UserID:        userID,
		MerchantID:    merchantID,
		OrderType:     "reservation",
		Status:        "paid",
		TotalAmount:   1000,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}
	reservation := db.TableReservation{ID: reservationID, UserID: userID, MerchantID: merchantID, TableID: tableID, PaymentMode: "full", Status: "confirmed"}
	session := db.DiningSession{ID: 82, UserID: userID}
	dish := db.Dish{ID: dishID, MerchantID: merchantID, Name: "Rice", Price: 500, IsOnline: true, IsAvailable: true}
	paymentOrder := db.PaymentOrder{
		ID:             448,
		Status:         "paid",
		OutTradeNo:     "out_ordinary_replace",
		Amount:         1000,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		BusinessType:   businessTypeReservation,
		ReservationID:  pgtype.Int8{Int64: reservationID, Valid: true},
	}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), orderID).Return(oldOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return(session, nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(dish, nil)
	store.EXPECT().ListActiveDiscountRules(gomock.Any(), merchantID).Return([]db.DiscountRule{}, nil)
	store.EXPECT().ReplaceOrderTx(gomock.Any(), gomock.Any()).Return(db.ReplaceOrderTxResult{NewOrder: db.Order{ID: 337, TotalAmount: 500}}, nil)
	store.EXPECT().GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return([]db.PaymentOrder{paymentOrder}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(0), nil)
	store.EXPECT().CreateRefundOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderParams) (db.RefundOrder, error) {
		require.Equal(t, int64(500), arg.RefundAmount)
		require.Equal(t, paymentOrder.ID, arg.PaymentOrderID)
		require.NotEmpty(t, arg.OutRefundNo)
		capturedOutRefundNo = arg.OutRefundNo
		return db.RefundOrder{ID: 558}, nil
	})
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchantID).Return(db.MerchantPaymentConfig{MerchantID: merchantID, SubMchID: "sub-ordinary", Status: "active"}, nil)
	store.EXPECT().UpdateRefundOrderToProcessing(gomock.Any(), db.UpdateRefundOrderToProcessingParams{
		ID:       558,
		RefundID: pgtype.Text{String: "orefund_replace_1", Valid: true},
	}).Return(db.RefundOrder{ID: 558, Status: "processing"}, nil)
	expectReplaceReservationRefundCommand(t, store, 558, &capturedOutRefundNo, "orefund_replace_1", db.ExternalPaymentCommandStatusAccepted, "", db.PaymentChannelOrdinaryServiceProvider, db.ExternalPaymentCapabilityPartnerRefund, 9303)

	result, err := ReplaceReservationOrderWithOrdinaryServiceProvider(
		context.Background(),
		store,
		nil,
		ordinaryClient,
		ReplaceOrderInput{UserID: userID, OrderID: orderID, Items: []OrderItemInput{{DishID: &dishID, Quantity: 1}}},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
	)
	require.NoError(t, err)
	require.Equal(t, int64(-500), result.Delta)
	require.True(t, result.RefundInitiated)
	require.NotNil(t, ordinaryClient.createRefundRequest)
	require.Equal(t, "sub-ordinary", ordinaryClient.createRefundRequest.SubMchID)
	require.Equal(t, paymentOrder.OutTradeNo, ordinaryClient.createRefundRequest.OutTradeNo)
	require.Equal(t, capturedOutRefundNo, ordinaryClient.createRefundRequest.OutRefundNo)
	require.Equal(t, ordinaryClient.RefundNotifyURL(), ordinaryClient.createRefundRequest.NotifyURL)
	require.Equal(t, int64(500), ordinaryClient.createRefundRequest.Amount.Refund)
	require.Equal(t, paymentOrder.Amount, ordinaryClient.createRefundRequest.Amount.Total)
}

func TestReplaceReservationOrder_OrdinaryRefundMissingClientFailsBeforeReplacement(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	orderID := int64(17)
	userID := int64(27)
	reservationID := int64(37)
	merchantID := int64(47)
	tableID := int64(57)
	dishID := int64(67)

	oldOrder := db.Order{
		ID:            orderID,
		UserID:        userID,
		MerchantID:    merchantID,
		OrderType:     "reservation",
		Status:        "paid",
		TotalAmount:   1000,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}
	reservation := db.TableReservation{ID: reservationID, UserID: userID, MerchantID: merchantID, TableID: tableID, PaymentMode: "full", Status: "confirmed"}
	session := db.DiningSession{ID: 83, UserID: userID}
	dish := db.Dish{ID: dishID, MerchantID: merchantID, Name: "Rice", Price: 500, IsOnline: true, IsAvailable: true}
	paymentOrder := db.PaymentOrder{
		ID:             449,
		Status:         "paid",
		OutTradeNo:     "out_ordinary_replace_missing_client",
		Amount:         1000,
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelOrdinaryServiceProvider,
		BusinessType:   businessTypeReservation,
		ReservationID:  pgtype.Int8{Int64: reservationID, Valid: true},
	}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), orderID).Return(oldOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return(session, nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(dish, nil)
	store.EXPECT().ListActiveDiscountRules(gomock.Any(), merchantID).Return([]db.DiscountRule{}, nil)
	store.EXPECT().GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return([]db.PaymentOrder{paymentOrder}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(int64(0), nil)

	_, err := ReplaceReservationOrderWithOrdinaryServiceProvider(
		context.Background(),
		store,
		nil,
		nil,
		ReplaceOrderInput{UserID: userID, OrderID: orderID, Items: []OrderItemInput{{DishID: &dishID, Quantity: 1}}},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ordinary service provider client")
	require.Contains(t, err.Error(), "not configured")
}

func TestReplaceReservationOrder_RefundProfitSharingWechatRejectedRecordsCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)

	orderID := int64(13)
	userID := int64(23)
	reservationID := int64(33)
	merchantID := int64(43)
	tableID := int64(53)
	dishID := int64(63)
	capturedOutRefundNo := ""

	oldOrder := db.Order{
		ID:            orderID,
		UserID:        userID,
		MerchantID:    merchantID,
		OrderType:     "reservation",
		Status:        "paid",
		TotalAmount:   1000,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}
	reservation := db.TableReservation{ID: reservationID, UserID: userID, MerchantID: merchantID, TableID: tableID, PaymentMode: "full", Status: "confirmed"}
	session := db.DiningSession{ID: 80, UserID: userID}
	dish := db.Dish{ID: dishID, MerchantID: merchantID, Name: "Noodle", Price: 500, IsOnline: true, IsAvailable: true}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), orderID).Return(oldOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return(session, nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(dish, nil)
	store.EXPECT().ListActiveDiscountRules(gomock.Any(), merchantID).Return([]db.DiscountRule{}, nil)
	store.EXPECT().ReplaceOrderTx(gomock.Any(), gomock.Any()).Return(db.ReplaceOrderTxResult{NewOrder: db.Order{ID: 335, TotalAmount: 500}}, nil)
	store.EXPECT().GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return([]db.PaymentOrder{{ID: 446, Status: "paid", OutTradeNo: "out_reject", Amount: 1000, PaymentType: "profit_sharing", PaymentChannel: db.PaymentChannelEcommerce, BusinessType: businessTypeReservation, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}}}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), int64(446)).Return(int64(0), nil)
	store.EXPECT().CreateRefundOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderParams) (db.RefundOrder, error) {
		require.Equal(t, int64(500), arg.RefundAmount)
		require.Equal(t, int64(446), arg.PaymentOrderID)
		require.NotEmpty(t, arg.OutRefundNo)
		capturedOutRefundNo = arg.OutRefundNo
		return db.RefundOrder{ID: 556}, nil
	})
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchantID).Return(db.MerchantPaymentConfig{MerchantID: merchantID, SubMchID: "1900000109", Status: "active"}, nil)
	ecommerceClient.EXPECT().CreateEcommerceRefund(gomock.Any(), gomock.Any()).Return(nil, &wechat.WechatPayError{StatusCode: http.StatusServiceUnavailable, Code: "SYSTEM_ERROR", Message: "system busy"})
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), int64(556)).Return(db.RefundOrder{ID: 556, Status: "failed"}, nil)
	expectReplaceReservationRefundCommand(t, store, 556, &capturedOutRefundNo, "", db.ExternalPaymentCommandStatusRejected, "SYSTEM_ERROR", db.PaymentChannelEcommerce, db.ExternalPaymentCapabilityEcommerceRefund, 9302)

	_, err := ReplaceReservationOrder(
		context.Background(),
		store,
		ecommerceClient,
		ReplaceOrderInput{UserID: userID, OrderID: orderID, Items: []OrderItemInput{{DishID: &dishID, Quantity: 1}}},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
	)
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, reqErr.Status)
}

func TestReplaceReservationOrder_RefundProfitSharingWechatRejectedSkipsCommandWhenFailedUpdateFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)

	orderID := int64(14)
	userID := int64(24)
	reservationID := int64(34)
	merchantID := int64(44)
	tableID := int64(54)
	dishID := int64(64)

	oldOrder := db.Order{
		ID:            orderID,
		UserID:        userID,
		MerchantID:    merchantID,
		OrderType:     "reservation",
		Status:        "paid",
		TotalAmount:   1000,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}
	reservation := db.TableReservation{ID: reservationID, UserID: userID, MerchantID: merchantID, TableID: tableID, PaymentMode: "full", Status: "confirmed"}
	session := db.DiningSession{ID: 81, UserID: userID}
	dish := db.Dish{ID: dishID, MerchantID: merchantID, Name: "Dumpling", Price: 500, IsOnline: true, IsAvailable: true}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), orderID).Return(oldOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return(session, nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(dish, nil)
	store.EXPECT().ListActiveDiscountRules(gomock.Any(), merchantID).Return([]db.DiscountRule{}, nil)
	store.EXPECT().ReplaceOrderTx(gomock.Any(), gomock.Any()).Return(db.ReplaceOrderTxResult{NewOrder: db.Order{ID: 336, TotalAmount: 500}}, nil)
	store.EXPECT().GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return([]db.PaymentOrder{{ID: 447, Status: "paid", OutTradeNo: "out_reject_db_fail", Amount: 1000, PaymentType: "profit_sharing", PaymentChannel: db.PaymentChannelEcommerce, BusinessType: businessTypeReservation, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}}}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), int64(447)).Return(int64(0), nil)
	store.EXPECT().CreateRefundOrder(gomock.Any(), gomock.Any()).Return(db.RefundOrder{ID: 557, OutRefundNo: "replace_refund_db_fail"}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchantID).Return(db.MerchantPaymentConfig{MerchantID: merchantID, SubMchID: "1900000109", Status: "active"}, nil)
	ecommerceClient.EXPECT().CreateEcommerceRefund(gomock.Any(), gomock.Any()).Return(nil, &wechat.WechatPayError{StatusCode: http.StatusServiceUnavailable, Code: "SYSTEM_ERROR", Message: "system busy"})
	store.EXPECT().UpdateRefundOrderToFailed(gomock.Any(), int64(557)).Return(db.RefundOrder{}, errors.New("failed update unavailable"))

	_, err := ReplaceReservationOrder(
		context.Background(),
		store,
		ecommerceClient,
		ReplaceOrderInput{UserID: userID, OrderID: orderID, Items: []OrderItemInput{{DishID: &dishID, Quantity: 1}}},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
	)
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, reqErr.Status)
}

func TestReplaceReservationOrder_RefundInsufficientCoverageReturnsConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)

	orderID := int64(15)
	userID := int64(25)
	reservationID := int64(35)
	merchantID := int64(45)
	tableID := int64(55)
	dishID := int64(65)

	oldOrder := db.Order{
		ID:            orderID,
		UserID:        userID,
		OrderType:     "reservation",
		Status:        "paid",
		TotalAmount:   1000,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}
	reservation := db.TableReservation{
		ID:          reservationID,
		UserID:      userID,
		MerchantID:  merchantID,
		TableID:     tableID,
		PaymentMode: "full",
		Status:      "confirmed",
	}
	session := db.DiningSession{ID: 79, UserID: userID}
	dish := db.Dish{ID: dishID, MerchantID: merchantID, Name: "Soup", Price: 500, IsOnline: true, IsAvailable: true}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), orderID).Return(oldOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Return(reservation, nil)
	store.EXPECT().GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return(session, nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Return(dish, nil)
	store.EXPECT().ListActiveDiscountRules(gomock.Any(), merchantID).Return([]db.DiscountRule{}, nil)
	store.EXPECT().GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Return([]db.PaymentOrder{{
		ID:            445,
		Status:        "paid",
		OutTradeNo:    "out_2",
		Amount:        1000,
		PaymentType:   "miniprogram",
		BusinessType:  businessTypeReservation,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}}, nil)
	store.EXPECT().GetTotalRefundedByPaymentOrder(gomock.Any(), int64(445)).Return(int64(600), nil)

	_, err := ReplaceReservationOrder(
		context.Background(),
		store,
		ecommerceClient,
		ReplaceOrderInput{
			UserID:  userID,
			OrderID: orderID,
			Items:   []OrderItemInput{{DishID: &dishID, Quantity: 1}},
		},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) { return nil, 0, nil },
	)
	reqErr := assertRequestError(t, err)
	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.Equal(t, "reservation refund funding chain changed, please retry", reqErr.Err.Error())
}

func expectReplaceReservationRefundCommand(t *testing.T, store *mockdb.MockStore, refundOrderID int64, outRefundNo *string, secondaryKey string, status string, errorCode string, expectedChannel string, expectedCapability string, commandID int64) {
	t.Helper()

	store.EXPECT().
		CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
			require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
			require.Equal(t, expectedChannel, arg.Channel)
			require.Equal(t, expectedCapability, arg.Capability)
			require.Equal(t, db.ExternalPaymentCommandTypeCreateRefund, arg.CommandType)
			require.Equal(t, db.ExternalPaymentBusinessOwnerReservation, arg.BusinessOwner)
			require.True(t, arg.BusinessObjectType.Valid)
			require.Equal(t, "refund_order", arg.BusinessObjectType.String)
			require.True(t, arg.BusinessObjectID.Valid)
			require.Equal(t, refundOrderID, arg.BusinessObjectID.Int64)
			require.Equal(t, db.ExternalPaymentObjectRefund, arg.ExternalObjectType)
			require.NotNil(t, outRefundNo)
			require.NotEmpty(t, *outRefundNo)
			require.Equal(t, *outRefundNo, arg.ExternalObjectKey)
			require.Equal(t, status, arg.CommandStatus)
			require.Contains(t, string(arg.ResponseSnapshot), *outRefundNo)
			if secondaryKey != "" {
				require.True(t, arg.ExternalSecondaryKey.Valid)
				require.Equal(t, secondaryKey, arg.ExternalSecondaryKey.String)
				require.Contains(t, string(arg.ResponseSnapshot), secondaryKey)
			}
			if errorCode != "" {
				require.True(t, arg.LastErrorCode.Valid)
				require.Equal(t, errorCode, arg.LastErrorCode.String)
				require.Contains(t, string(arg.ResponseSnapshot), errorCode)
			}
			require.NotContains(t, string(arg.ResponseSnapshot), "paySign")
			return db.ExternalPaymentCommand{ID: commandID}, nil
		})
}

func expectReplaceReservationPaymentCommand(t *testing.T, store *mockdb.MockStore, paymentOrderID int64, outTradeNo string, secondaryKey string, businessOwner string, status string, errorCode string, expectedChannel string, commandID int64) {
	t.Helper()

	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, expectedChannel, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityPartnerJSAPIPayment, arg.Capability)
		require.Equal(t, db.ExternalPaymentCommandTypeCreatePayment, arg.CommandType)
		require.Equal(t, businessOwner, arg.BusinessOwner)
		require.True(t, arg.BusinessObjectType.Valid)
		require.Equal(t, "payment_order", arg.BusinessObjectType.String)
		require.True(t, arg.BusinessObjectID.Valid)
		require.Equal(t, paymentOrderID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectPayment, arg.ExternalObjectType)
		require.Equal(t, outTradeNo, arg.ExternalObjectKey)
		require.Equal(t, status, arg.CommandStatus)
		require.Contains(t, string(arg.ResponseSnapshot), outTradeNo)
		if secondaryKey != "" {
			require.True(t, arg.ExternalSecondaryKey.Valid)
			require.Equal(t, secondaryKey, arg.ExternalSecondaryKey.String)
			require.Contains(t, string(arg.ResponseSnapshot), secondaryKey)
		}
		if errorCode != "" {
			require.True(t, arg.LastErrorCode.Valid)
			require.Equal(t, errorCode, arg.LastErrorCode.String)
			require.Contains(t, string(arg.ResponseSnapshot), errorCode)
		}
		require.NotContains(t, string(arg.ResponseSnapshot), "paySign")
		return db.ExternalPaymentCommand{ID: commandID}, nil
	})
}
