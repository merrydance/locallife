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
		CreatePaymentOrder(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreatePaymentOrderParams) (db.PaymentOrder, error) {
			require.Equal(t, int64(500), arg.Amount)
			require.Equal(t, userID, arg.UserID)
			require.True(t, arg.OrderID.Valid)
			require.Equal(t, int64(111), arg.OrderID.Int64)
			require.Equal(t, "miniprogram", arg.PaymentType)
			require.Equal(t, "order", arg.BusinessType)
			return db.PaymentOrder{ID: 222}, nil
		})

	result, err := ReplaceReservationOrder(
		context.Background(),
		store,
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
	require.Equal(t, int64(500), result.Delta)
	require.NotNil(t, result.PaymentOrderID)
	require.Equal(t, int64(222), *result.PaymentOrderID)
	require.False(t, result.RefundInitiated)
}

func TestReplaceReservationOrder_Refund(t *testing.T) {
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
		Return(db.PaymentOrder{ID: 444, Status: "paid", OutTradeNo: "out_1", Amount: 1000}, nil)
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
