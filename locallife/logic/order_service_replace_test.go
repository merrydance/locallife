package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatmock "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type replaceOrderTaskSchedulerStub struct {
	combinedOutTradeNo string
	at                 time.Time
	called             bool
}

func (s *replaceOrderTaskSchedulerStub) ScheduleOrderPaymentTimeout(ctx context.Context, orderID int64, at time.Time) error {
	return nil
}

func (s *replaceOrderTaskSchedulerStub) SchedulePaymentOrderTimeout(ctx context.Context, paymentOrderNo string, at time.Time) error {
	return nil
}

func (s *replaceOrderTaskSchedulerStub) ScheduleCombinedPaymentOrderTimeout(ctx context.Context, combineOutTradeNo string, at time.Time) error {
	s.called = true
	s.combinedOutTradeNo = combineOutTradeNo
	s.at = at
	return nil
}

func (s *replaceOrderTaskSchedulerStub) ScheduleProcessRefund(ctx context.Context, input ProcessRefundTaskInput) error {
	return nil
}

func (s *replaceOrderTaskSchedulerStub) ScheduleProfitSharing(ctx context.Context, paymentOrderID, orderID int64) error {
	return nil
}

func (s *replaceOrderTaskSchedulerStub) ScheduleProfitSharingReturnResult(ctx context.Context, input ProfitSharingReturnResultTaskInput) error {
	return nil
}

func TestOrderServiceReplaceOrderSchedulesCombinedTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := wechatmock.NewMockEcommerceClientInterface(ctrl)
	taskScheduler := &replaceOrderTaskSchedulerStub{}

	orderID := int64(10)
	userID := int64(20)
	reservationID := int64(30)
	merchantID := int64(40)
	tableID := int64(50)
	dishID := int64(60)
	expiresAt := time.Now().Add(30 * time.Minute).Round(time.Second)

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
		Status:      "paid",
	}
	session := db.DiningSession{ID: 77, UserID: userID}
	dish := db.Dish{ID: dishID, MerchantID: merchantID, Name: "Noodles", Price: 1500, IsOnline: true, IsAvailable: true}
	paymentOrder := db.PaymentOrder{
		ID:                222,
		UserID:            userID,
		OrderID:           pgtype.Int8{Int64: 111, Valid: true},
		PaymentType:       "profit_sharing",
		BusinessType:      "order",
		Amount:            500,
		OutTradeNo:        "CP111202603230001",
		Status:            "pending",
		CombinedPaymentID: pgtype.Int8{Int64: 3333, Valid: true},
		ExpiresAt:         pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), orderID).Times(1).Return(oldOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), reservationID).Times(1).Return(reservation, nil)
	store.EXPECT().GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).Times(1).Return(session, nil)
	store.EXPECT().GetDish(gomock.Any(), dishID).Times(1).Return(dish, nil)
	store.EXPECT().ListActiveDiscountRules(gomock.Any(), merchantID).Times(1).Return([]db.DiscountRule{}, nil)
	store.EXPECT().ReplaceOrderTx(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.ReplaceOrderTxParams) (db.ReplaceOrderTxResult, error) {
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
	store.EXPECT().GetUser(gomock.Any(), userID).Times(1).Return(db.User{ID: userID, WechatOpenid: "openid-1"}, nil)
	store.EXPECT().CreateCombinedPaymentTx(gomock.Any(), gomock.Any()).Times(1).Return(db.CreateCombinedPaymentTxResult{
		CombinedPaymentOrder: db.CombinedPaymentOrder{ID: 3333, UserID: userID, CombineOutTradeNo: "RC123"},
		PaymentOrders:        []db.PaymentOrder{paymentOrder},
		OrderInfos: []db.CombinedPaymentOrderInfo{{
			Order:         db.Order{ID: 111, MerchantID: merchantID, TotalAmount: 500},
			PaymentOrder:  paymentOrder,
			PaymentConfig: db.MerchantPaymentConfig{MerchantID: merchantID, SubMchID: "1900000109", Status: "active"},
			Merchant:      db.Merchant{ID: merchantID, Name: "Test Merchant"},
		}},
	}, nil)
	ecommerceClient.EXPECT().CreateCombineOrder(gomock.Any(), gomock.Any()).Times(1).Return(&wechat.CombineOrderResponse{PrepayID: "prepay-replace-1"}, &wechat.JSAPIPayParams{}, nil)
	store.EXPECT().UpdatePaymentOrderPrepayId(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.UpdatePaymentOrderPrepayIdParams) (db.PaymentOrder, error) {
		updated := paymentOrder
		updated.PrepayID = arg.PrepayID
		return updated, nil
	})
	store.EXPECT().UpdateCombinedPaymentOrderPrepay(gomock.Any(), gomock.Any()).Times(1).Return(db.CombinedPaymentOrder{}, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), int64(222)).Times(1).Return(paymentOrder, nil)
	store.EXPECT().GetCombinedPaymentOrder(gomock.Any(), int64(3333)).Times(1).Return(db.CombinedPaymentOrder{ID: 3333, CombineOutTradeNo: "RC123"}, nil)

	service := NewOrderService(store, nil, nil, nil, taskScheduler, nil, nil, ecommerceClient, nil, nil, nil)
	result, err := service.ReplaceOrder(context.Background(), ReplaceOrderInput{
		UserID:  userID,
		OrderID: orderID,
		Items: []OrderItemInput{{
			DishID:   &dishID,
			Quantity: 1,
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, result.PaymentOrderID)
	require.True(t, taskScheduler.called)
	require.Equal(t, "RC123", taskScheduler.combinedOutTradeNo)
	require.True(t, taskScheduler.at.Equal(expiresAt))
}
