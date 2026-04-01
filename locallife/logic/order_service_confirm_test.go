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

type confirmOrderTaskSchedulerStub struct {
	profitSharingCalled bool
	paymentOrderID      int64
	orderID             int64
}

func (s *confirmOrderTaskSchedulerStub) ScheduleOrderPaymentTimeout(ctx context.Context, orderID int64, at time.Time) error {
	return nil
}

func (s *confirmOrderTaskSchedulerStub) SchedulePaymentOrderTimeout(ctx context.Context, paymentOrderNo string, at time.Time) error {
	return nil
}

func (s *confirmOrderTaskSchedulerStub) ScheduleCombinedPaymentOrderTimeout(ctx context.Context, combineOutTradeNo string, at time.Time) error {
	return nil
}

func (s *confirmOrderTaskSchedulerStub) ScheduleProcessRefund(ctx context.Context, input ProcessRefundTaskInput) error {
	return nil
}

func (s *confirmOrderTaskSchedulerStub) ScheduleProfitSharing(ctx context.Context, paymentOrderID, orderID int64) error {
	s.profitSharingCalled = true
	s.paymentOrderID = paymentOrderID
	s.orderID = orderID
	return nil
}

func (s *confirmOrderTaskSchedulerStub) ScheduleProfitSharingReturnResult(ctx context.Context, input ProfitSharingReturnResultTaskInput) error {
	return nil
}

func (s *confirmOrderTaskSchedulerStub) ScheduleOrderPrint(ctx context.Context, input OrderPrintTaskInput) error {
	return nil
}

func TestOrderServiceConfirmOrder_DoesNotScheduleProfitSharing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	taskScheduler := &confirmOrderTaskSchedulerStub{}

	order := db.Order{
		ID:         20,
		UserID:     300,
		MerchantID: 400,
		OrderNo:    "ORDER-002",
		OrderType:  db.OrderTypeTakeout,
		Status:     db.OrderStatusRiderDelivered,
	}
	updated := order
	updated.Status = db.OrderStatusCompleted

	store.EXPECT().
		GetOrder(gomock.Any(), order.ID).
		Times(1).
		Return(order, nil)
	store.EXPECT().
		CompleteTakeoutOrderByUser(gomock.Any(), order.ID).
		Times(1).
		Return(updated, nil)
	store.EXPECT().
		CreateOrderStatusLog(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.OrderStatusLog{}, nil)
	store.EXPECT().
		GetDeliveryByOrderID(gomock.Any(), order.ID).
		Times(1).
		Return(db.Delivery{RiderID: pgtype.Int8{Int64: 808, Valid: true}}, nil)

	service := NewOrderService(store, nil, nil, nil, taskScheduler, nil, nil, nil, nil, nil, nil)

	result, err := service.ConfirmOrder(context.Background(), ConfirmOrderInput{UserID: order.UserID, OrderID: order.ID})
	require.NoError(t, err)
	require.Equal(t, db.OrderStatusCompleted, result.Order.Status)
	require.False(t, result.AlreadyCompleted)
	require.False(t, taskScheduler.profitSharingCalled)
	require.Zero(t, taskScheduler.paymentOrderID)
	require.Zero(t, taskScheduler.orderID)
}
