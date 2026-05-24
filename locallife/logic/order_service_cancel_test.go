package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type cancelOrderTaskSchedulerStub struct {
	err error
}

func (s *cancelOrderTaskSchedulerStub) ScheduleOrderPaymentTimeout(ctx context.Context, orderID int64, at time.Time) error {
	return nil
}

func (s *cancelOrderTaskSchedulerStub) SchedulePaymentOrderTimeout(ctx context.Context, paymentOrderNo string, at time.Time) error {
	return nil
}

func (s *cancelOrderTaskSchedulerStub) ScheduleCombinedPaymentOrderTimeout(ctx context.Context, combineOutTradeNo string, at time.Time) error {
	return nil
}

func (s *cancelOrderTaskSchedulerStub) ScheduleProcessRefund(ctx context.Context, input ProcessRefundTaskInput) error {
	return s.err
}

func (s *cancelOrderTaskSchedulerStub) ScheduleProfitSharing(ctx context.Context, profitSharingOrderID int64) error {
	return nil
}

func (s *cancelOrderTaskSchedulerStub) ScheduleProfitSharingReturnResult(ctx context.Context, input ProfitSharingReturnResultTaskInput) error {
	return nil
}

func (s *cancelOrderTaskSchedulerStub) ScheduleOrderPrint(ctx context.Context, input OrderPrintTaskInput) error {
	return nil
}

type cancelOrderAuditLoggerStub struct {
	entries []AuditLogInput
}

func (s *cancelOrderAuditLoggerStub) Write(ctx context.Context, input AuditLogInput) {
	s.entries = append(s.entries, input)
}

func TestOrderServiceCancelOrder_RefundScheduleFailureWritesAudit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	auditLogger := &cancelOrderAuditLoggerStub{}
	taskScheduler := &cancelOrderTaskSchedulerStub{err: errors.New("queue down")}

	order := db.Order{ID: 31, UserID: 6, MerchantID: 77, Status: db.OrderStatusPaid}

	store.EXPECT().
		GetOrderForUpdate(gomock.Any(), order.ID).
		Return(order, nil)
	store.EXPECT().
		CancelOrderTx(gomock.Any(), gomock.Any()).
		Return(db.CancelOrderTxResult{Order: db.Order{ID: order.ID, UserID: order.UserID, MerchantID: order.MerchantID, Status: db.OrderStatusCancelled}}, nil)
	store.EXPECT().
		GetPaymentOrdersByOrder(gomock.Any(), pgtype.Int8{Int64: order.ID, Valid: true}).
		Return([]db.PaymentOrder{{ID: 202, Status: "paid", Amount: 900}}, nil)

	service := NewOrderService(store, nil, auditLogger, nil, taskScheduler, nil, nil, nil, nil, nil, nil)

	result, err := service.CancelOrder(context.Background(), CancelOrderInput{UserID: order.UserID, OrderID: order.ID, Reason: "change mind"})
	require.NoError(t, err)
	require.NotNil(t, result.Refund)
	require.Len(t, auditLogger.entries, 1)
	require.Equal(t, "order_cancel_refund_schedule_issue", auditLogger.entries[0].Action)
	require.Equal(t, "order", auditLogger.entries[0].TargetType)
	require.NotNil(t, auditLogger.entries[0].TargetID)
	require.Equal(t, order.ID, *auditLogger.entries[0].TargetID)
	require.Equal(t, "schedule_process_refund_failed", auditLogger.entries[0].Metadata["issue"])
	require.Equal(t, int64(202), auditLogger.entries[0].Metadata["payment_order_id"])
	require.Equal(t, int64(900), auditLogger.entries[0].Metadata["refund_amount"])
	require.Equal(t, true, auditLogger.entries[0].Metadata["refund_recovery_expected"])
	require.Equal(t, "queue down", auditLogger.entries[0].Metadata["error"])
}

func TestOrderServiceCancelOrder_MissingSchedulerWritesAudit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	auditLogger := &cancelOrderAuditLoggerStub{}

	order := db.Order{ID: 32, UserID: 8, MerchantID: 78, Status: db.OrderStatusPaid}

	store.EXPECT().
		GetOrderForUpdate(gomock.Any(), order.ID).
		Return(order, nil)
	store.EXPECT().
		CancelOrderTx(gomock.Any(), gomock.Any()).
		Return(db.CancelOrderTxResult{Order: db.Order{ID: order.ID, UserID: order.UserID, MerchantID: order.MerchantID, Status: db.OrderStatusCancelled}}, nil)
	store.EXPECT().
		GetPaymentOrdersByOrder(gomock.Any(), pgtype.Int8{Int64: order.ID, Valid: true}).
		Return([]db.PaymentOrder{{ID: 303, Status: "paid", Amount: 1200}}, nil)

	service := NewOrderService(store, nil, auditLogger, nil, nil, nil, nil, nil, nil, nil, nil)

	result, err := service.CancelOrder(context.Background(), CancelOrderInput{UserID: order.UserID, OrderID: order.ID})
	require.NoError(t, err)
	require.NotNil(t, result.Refund)
	require.Len(t, auditLogger.entries, 1)
	require.Equal(t, "task_scheduler_not_configured", auditLogger.entries[0].Metadata["issue"])
	require.Equal(t, int64(303), auditLogger.entries[0].Metadata["payment_order_id"])
	require.Equal(t, int64(1200), auditLogger.entries[0].Metadata["refund_amount"])
	require.Equal(t, true, auditLogger.entries[0].Metadata["refund_recovery_expected"])
}
