package logic

import (
	"context"
	"net/http"
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
	paymentOrder := db.PaymentOrder{
		ID:            222,
		UserID:        userID,
		OrderID:       pgtype.Int8{Int64: 111, Valid: true},
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
		PaymentType:   "profit_sharing",
		BusinessType:  "order",
		Amount:        500,
		OutTradeNo:    "RO111202603230001",
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
			return db.CreatePartnerPaymentTxResult{PaymentOrder: paymentOrder, SubMchID: "1900000109"}, nil
		})
	ecommerceClient.EXPECT().
		CreatePartnerJSAPIOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *wechat.PartnerJSAPIOrderRequest) (*wechat.PartnerJSAPIOrderResponse, *wechat.JSAPIPayParams, error) {
			require.Equal(t, "1900000109", req.SubMchID)
			require.Equal(t, "openid-1", req.PayerOpenID)
			require.Equal(t, int64(500), req.TotalAmount)
			require.Equal(t, "Test Merchant - Reservation Adjustment", req.Description)
			require.Equal(t, "order_id:111", req.Attach)
			require.True(t, req.ProfitSharing)
			return &wechat.PartnerJSAPIOrderResponse{PrepayID: "prepay-replace-1"}, &wechat.JSAPIPayParams{}, nil
		})
	store.EXPECT().
		UpdatePaymentOrderPrepayId(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.UpdatePaymentOrderPrepayIdParams) (db.PaymentOrder, error) {
			updated := paymentOrder
			updated.PrepayID = arg.PrepayID
			return updated, nil
		})
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
		GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
		Times(1).
		Return([]db.PaymentOrder{{ID: 444, Status: "paid", OutTradeNo: "out_1", Amount: 1000, PaymentType: "miniprogram", BusinessType: businessTypeReservation, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}}}, nil)
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
			require.Equal(t, "miniprogram", arg.RefundType)
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
		GetPaymentOrdersByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
		Times(1).
		Return([]db.PaymentOrder{{ID: 444, Status: "paid", OutTradeNo: "out_1", Amount: 1000, PaymentType: "profit_sharing", BusinessType: businessTypeReservation, ReservationID: pgtype.Int8{Int64: reservationID, Valid: true}}}, nil)
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

func TestReplaceReservationOrder_RefundInsufficientCoverageReturnsConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	paymentClient := wechatmock.NewMockPaymentClientInterface(ctrl)

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
	store.EXPECT().ReplaceOrderTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReplaceOrderTxParams) (db.ReplaceOrderTxResult, error) {
		newOrder := db.Order{
			ID:                334,
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
		paymentClient,
		nil,
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
