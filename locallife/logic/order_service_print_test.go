package logic

import (
	"context"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type orderPrintTaskSchedulerStub struct {
	inputs []OrderPrintTaskInput
}

type orderServiceEventPublisherStub struct {
	snapshots []orderServiceSnapshotEvent
	pooled    []db.DeliveryPool
}

type orderServiceSnapshotEvent struct {
	MerchantID  int64
	OrderID     int64
	MessageType string
}

func (s *orderServiceEventPublisherStub) PublishMerchantOrderSnapshot(ctx context.Context, merchantID int64, order db.Order, messageType string) {
	s.snapshots = append(s.snapshots, orderServiceSnapshotEvent{
		MerchantID:  merchantID,
		OrderID:     order.ID,
		MessageType: messageType,
	})
}

func (s *orderServiceEventPublisherStub) PublishMerchantUserRiskAlert(ctx context.Context, merchantID int64, alert MerchantUserRiskAlert) {
}

func (s *orderServiceEventPublisherStub) PublishTakeoutOrderPooled(ctx context.Context, order db.Order, poolItem db.DeliveryPool) {
	s.pooled = append(s.pooled, poolItem)
}

func (s *orderPrintTaskSchedulerStub) ScheduleOrderPaymentTimeout(ctx context.Context, orderID int64, atTime time.Time) error {
	return nil
}

func (s *orderPrintTaskSchedulerStub) SchedulePaymentOrderTimeout(ctx context.Context, paymentOrderNo string, atTime time.Time) error {
	return nil
}

func (s *orderPrintTaskSchedulerStub) ScheduleCombinedPaymentOrderTimeout(ctx context.Context, combineOutTradeNo string, atTime time.Time) error {
	return nil
}

func (s *orderPrintTaskSchedulerStub) ScheduleProcessRefund(ctx context.Context, input ProcessRefundTaskInput) error {
	return nil
}

func (s *orderPrintTaskSchedulerStub) ScheduleProfitSharing(ctx context.Context, profitSharingOrderID int64) error {
	return nil
}

func (s *orderPrintTaskSchedulerStub) ScheduleProfitSharingReturnResult(ctx context.Context, input ProfitSharingReturnResultTaskInput) error {
	return nil
}

func (s *orderPrintTaskSchedulerStub) ScheduleOrderPrint(ctx context.Context, input OrderPrintTaskInput) error {
	s.inputs = append(s.inputs, input)
	return nil
}

func TestOrderServiceAcceptMerchantOrder_SchedulesPrintWhenAcceptedTriggerEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	scheduler := &orderPrintTaskSchedulerStub{}
	service := NewOrderService(store, nil, nil, nil, scheduler, nil, nil, nil, nil, nil, nil)

	input := MerchantOrderUpdateInput{MerchantID: 10, OrderID: 20, OperatorID: 30}
	order := db.Order{ID: input.OrderID, MerchantID: input.MerchantID, OrderType: db.OrderTypeTakeout, Status: db.OrderStatusPaid}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetMerchantProfile(gomock.Any(), input.MerchantID).Return(db.GetMerchantProfileRow{MerchantID: input.MerchantID, IsTakeoutSuspended: false}, nil)
	store.EXPECT().AcceptTakeoutOrderTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.AcceptTakeoutOrderTxParams) (db.AcceptTakeoutOrderTxResult, error) {
		updated := order
		updated.Status = db.OrderStatusPreparing
		return db.AcceptTakeoutOrderTxResult{Order: updated}, nil
	})
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), input.MerchantID).Return(db.OrderDisplayConfig{
		MerchantID:        input.MerchantID,
		EnablePrint:       true,
		PrintTakeout:      true,
		PrintDineIn:       true,
		PrintReservation:  true,
		PrintDispatchMode: "single_full",
		PrintTriggerMode:  "accepted",
	}, nil)

	result, err := service.AcceptMerchantOrder(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, db.OrderStatusPreparing, result.Order.Status)
	require.Len(t, scheduler.inputs, 1)
	require.Equal(t, OrderPrintTaskInput{OrderID: input.OrderID, Trigger: "accepted"}, scheduler.inputs[0])
}

func TestOrderServiceAcceptMerchantOrder_BroadcastsTakeoutPoolEntry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	events := &orderServiceEventPublisherStub{}
	service := NewOrderService(store, nil, nil, events, nil, nil, nil, nil, nil, nil, nil)

	input := MerchantOrderUpdateInput{MerchantID: 10, OrderID: 20, OperatorID: 30}
	order := db.Order{ID: input.OrderID, MerchantID: input.MerchantID, OrderType: db.OrderTypeTakeout, Status: db.OrderStatusPaid}
	poolItem := db.DeliveryPool{OrderID: input.OrderID, MerchantID: input.MerchantID}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetMerchantProfile(gomock.Any(), input.MerchantID).Return(db.GetMerchantProfileRow{MerchantID: input.MerchantID, IsTakeoutSuspended: false}, nil)
	store.EXPECT().AcceptTakeoutOrderTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.AcceptTakeoutOrderTxParams) (db.AcceptTakeoutOrderTxResult, error) {
		updated := order
		updated.Status = db.OrderStatusPreparing
		return db.AcceptTakeoutOrderTxResult{Order: updated, PoolItem: poolItem}, nil
	})

	result, err := service.AcceptMerchantOrder(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, db.OrderStatusPreparing, result.Order.Status)
	require.Len(t, events.snapshots, 1)
	require.Equal(t, orderServiceSnapshotEvent{
		MerchantID:  input.MerchantID,
		OrderID:     input.OrderID,
		MessageType: merchantOrderSnapshotMessageTypeOrderUpdate,
	}, events.snapshots[0])
	require.Len(t, events.pooled, 1)
	require.Equal(t, poolItem.OrderID, events.pooled[0].OrderID)
}

func TestOrderServiceMarkMerchantOrderReady_RespectsPrintTrigger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	scheduler := &orderPrintTaskSchedulerStub{}
	service := NewOrderService(store, nil, nil, nil, scheduler, nil, nil, nil, nil, nil, nil)

	input := MerchantOrderUpdateInput{MerchantID: 11, OrderID: 21, OperatorID: 31}
	order := db.Order{ID: input.OrderID, MerchantID: input.MerchantID, OrderType: db.OrderTypeTakeout, Status: db.OrderStatusPreparing}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().MarkTakeoutOrderReadyTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkTakeoutOrderReadyTxParams) (db.MarkTakeoutOrderReadyTxResult, error) {
		updated := order
		updated.Status = db.OrderStatusReady
		return db.MarkTakeoutOrderReadyTxResult{Order: updated}, nil
	})
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), input.MerchantID).Return(db.OrderDisplayConfig{
		MerchantID:        input.MerchantID,
		EnablePrint:       true,
		PrintTakeout:      true,
		PrintDineIn:       true,
		PrintReservation:  true,
		PrintDispatchMode: "single_full",
		PrintTriggerMode:  "ready",
	}, nil)

	result, err := service.MarkMerchantOrderReady(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, db.OrderStatusReady, result.Order.Status)
	require.Len(t, scheduler.inputs, 1)
	require.Equal(t, OrderPrintTaskInput{OrderID: input.OrderID, Trigger: "ready"}, scheduler.inputs[0])
}

func TestOrderServiceMarkMerchantOrderReady_DoesNotBroadcastTakeoutPoolEntry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	events := &orderServiceEventPublisherStub{}
	service := NewOrderService(store, nil, nil, events, nil, nil, nil, nil, nil, nil, nil)

	input := MerchantOrderUpdateInput{MerchantID: 11, OrderID: 21, OperatorID: 31}
	order := db.Order{ID: input.OrderID, MerchantID: input.MerchantID, OrderType: db.OrderTypeTakeout, Status: db.OrderStatusPreparing}

	store.EXPECT().GetOrderForUpdate(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().MarkTakeoutOrderReadyTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkTakeoutOrderReadyTxParams) (db.MarkTakeoutOrderReadyTxResult, error) {
		updated := order
		updated.Status = db.OrderStatusReady
		return db.MarkTakeoutOrderReadyTxResult{Order: updated}, nil
	})

	result, err := service.MarkMerchantOrderReady(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, db.OrderStatusReady, result.Order.Status)
	require.Empty(t, events.pooled)
}

func TestOrderServicePrintMerchantOrder_SchedulesManualPrintWhenEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	scheduler := &orderPrintTaskSchedulerStub{}
	service := NewOrderService(store, nil, nil, nil, scheduler, nil, nil, nil, nil, nil, nil)

	input := MerchantOrderPrintInput{MerchantID: 12, OrderID: 22, OperatorID: 32}
	order := db.Order{ID: input.OrderID, MerchantID: input.MerchantID, OrderType: db.OrderTypeTakeout, Status: db.OrderStatusPreparing}

	store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), input.MerchantID).Return(db.OrderDisplayConfig{
		MerchantID:        input.MerchantID,
		EnablePrint:       true,
		PrintTakeout:      true,
		PrintDineIn:       true,
		PrintReservation:  true,
		PrintDispatchMode: "single_full",
		PrintTriggerMode:  "manual",
	}, nil)

	result, err := service.PrintMerchantOrder(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, order.ID, result.Order.ID)
	require.Len(t, scheduler.inputs, 1)
	require.Equal(t, OrderPrintTaskInput{OrderID: input.OrderID, Trigger: "manual"}, scheduler.inputs[0])
}

func TestOrderServicePrintMerchantOrder_RejectsWhenManualModeDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	scheduler := &orderPrintTaskSchedulerStub{}
	service := NewOrderService(store, nil, nil, nil, scheduler, nil, nil, nil, nil, nil, nil)

	input := MerchantOrderPrintInput{MerchantID: 13, OrderID: 23, OperatorID: 33}
	order := db.Order{ID: input.OrderID, MerchantID: input.MerchantID, OrderType: db.OrderTypeTakeout, Status: db.OrderStatusPreparing}

	store.EXPECT().GetOrder(gomock.Any(), input.OrderID).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), input.MerchantID).Return(db.OrderDisplayConfig{
		MerchantID:        input.MerchantID,
		EnablePrint:       true,
		PrintTakeout:      true,
		PrintDineIn:       true,
		PrintReservation:  true,
		PrintDispatchMode: "single_full",
		PrintTriggerMode:  "accepted",
	}, nil)

	_, err := service.PrintMerchantOrder(context.Background(), input)
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "manual printing is not enabled", reqErr.Err.Error())
	require.Empty(t, scheduler.inputs)
}
