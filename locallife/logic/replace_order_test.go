package logic

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatmock "github.com/merrydance/locallife/wechat/mock"
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
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)

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
	paymentOrder := db.PaymentOrder{
		ID:                222,
		UserID:            userID,
		OrderID:           pgtype.Int8{Int64: 111, Valid: true},
		PaymentType:       "profit_sharing",
		BusinessType:      "order",
		Amount:            500,
		OutTradeNo:        "CP111202603230001",
		CombinedPaymentID: pgtype.Int8{Int64: 3333, Valid: true},
	}
	store.EXPECT().
		CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateCombinedPaymentTxParams) (db.CreateCombinedPaymentTxResult, error) {
			require.Equal(t, userID, arg.UserID)
			require.Equal(t, []int64{111}, arg.OrderIDs)
			return db.CreateCombinedPaymentTxResult{
				CombinedPaymentOrder: db.CombinedPaymentOrder{ID: 3333, UserID: userID, CombineOutTradeNo: arg.CombineOutTradeNo},
				PaymentOrders:        []db.PaymentOrder{paymentOrder},
				OrderInfos: []db.CombinedPaymentOrderInfo{{
					Order:         db.Order{ID: 111, MerchantID: merchantID, TotalAmount: 500},
					PaymentOrder:  paymentOrder,
					PaymentConfig: db.MerchantPaymentConfig{MerchantID: merchantID, SubMchID: "1900000109", Status: "active"},
					Merchant:      db.Merchant{ID: merchantID, Name: "Test Merchant"},
				}},
			}, nil
		})
	ecommerceClient.EXPECT().
		CreateCombineOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *wechat.CombineOrderRequest) (*wechat.CombineOrderResponse, *wechat.JSAPIPayParams, error) {
			require.Equal(t, "openid-1", req.PayerOpenID)
			require.Len(t, req.SubOrders, 1)
			require.Equal(t, int64(500), req.SubOrders[0].Amount)
			return &wechat.CombineOrderResponse{PrepayID: "prepay-replace-1"}, &wechat.JSAPIPayParams{}, nil
		})
	store.EXPECT().
		UpdatePaymentOrderPrepayId(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.UpdatePaymentOrderPrepayIdParams) (db.PaymentOrder, error) {
			updated := paymentOrder
			updated.PrepayID = arg.PrepayID
			return updated, nil
		})
	store.EXPECT().
		UpdateCombinedPaymentOrderPrepay(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CombinedPaymentOrder{}, nil)

	result, err := ReplaceReservationOrder(
		context.Background(),
		store,
		nil,
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
	require.NoError(t, err)
	require.Equal(t, int64(500), result.Delta)
	require.NotNil(t, result.PaymentOrderID)
	require.Equal(t, int64(222), *result.PaymentOrderID)
	require.False(t, result.RefundInitiated)
}

func TestReplaceReservationOrder_RefundDirect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := wechatmock.NewMockPaymentClientInterface(ctrl)

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
		ReplaceOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.ReplaceOrderTxParams) (db.ReplaceOrderTxResult, error) {
			require.Equal(t, int64(500), arg.CreateOrderParams.TotalAmount)
			require.Equal(t, "paid", arg.CreateOrderParams.Status)
			require.Equal(t, "pending_kitchen", arg.CreateOrderParams.FulfillmentStatus)
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
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: oldOrder.ID, Valid: true},
			BusinessType: "order",
		}).
		Times(1).
		Return(db.PaymentOrder{ID: 444, Status: "paid", OutTradeNo: "out_1", Amount: 1000, PaymentType: "miniprogram"}, nil)
	store.EXPECT().
		CreateRefundOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderParams) (db.RefundOrder, error) {
			require.Equal(t, int64(500), arg.RefundAmount)
			require.Equal(t, int64(444), arg.PaymentOrderID)
			return db.RefundOrder{ID: 555}, nil
		})
	paymentClient.EXPECT().
		CreateRefund(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *wechat.RefundRequest) (*wechat.RefundResponse, error) {
			require.Equal(t, "out_1", req.OutTradeNo)
			require.Equal(t, int64(500), req.RefundAmount)
			return &wechat.RefundResponse{Status: wechat.RefundStatusSuccess}, nil
		})
	store.EXPECT().
		UpdateRefundOrderToSuccess(gomock.Any(), int64(555)).
		Times(1).
		Return(db.RefundOrder{}, nil)

	result, err := ReplaceReservationOrder(
		context.Background(),
		store,
		paymentClient,
		nil,
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
	require.Equal(t, int64(-500), result.Delta)
	require.Nil(t, result.PaymentOrderID)
	require.True(t, result.RefundInitiated)
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
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: oldOrder.ID, Valid: true},
			BusinessType: "order",
		}).
		Times(1).
		Return(db.PaymentOrder{ID: 444, Status: "paid", OutTradeNo: "out_1", Amount: 1000, PaymentType: "profit_sharing"}, nil)
	store.EXPECT().
		CreateRefundOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateRefundOrderParams) (db.RefundOrder, error) {
			require.Equal(t, int64(500), arg.RefundAmount)
			require.Equal(t, int64(444), arg.PaymentOrderID)
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
			return &wechat.EcommerceRefundResponse{Status: wechat.RefundStatusSuccess}, nil
		})
	store.EXPECT().
		UpdateRefundOrderToSuccess(gomock.Any(), int64(555)).
		Times(1).
		Return(db.RefundOrder{}, nil)

	result, err := ReplaceReservationOrder(
		context.Background(),
		store,
		nil,
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
