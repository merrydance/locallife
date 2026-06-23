package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskPaymentDomainOutbox_PublishesProbeEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	outbox := db.PaymentDomainOutbox{
		ID:            901,
		EventType:     worker.PaymentDomainOutboxEventDispatcherProbe,
		AggregateType: "probe",
		AggregateID:   1,
		Status:        db.PaymentDomainOutboxStatusProcessing,
	}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ClaimPaymentDomainOutboxParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, outbox.ID, arg.ID)
		require.True(t, arg.NowAt.Valid)
		return outbox, nil
	})
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_UnsupportedEventMarksFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	outbox := db.PaymentDomainOutbox{
		ID:            902,
		EventType:     "unsupported_payment_domain_event",
		AggregateType: "profit_sharing_order",
		AggregateID:   3001,
		Status:        db.PaymentDomainOutboxStatusProcessing,
	}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().MarkPaymentDomainOutboxFailed(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkPaymentDomainOutboxFailedParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, outbox.ID, arg.ID)
		require.True(t, arg.LastError.Valid)
		require.Contains(t, arg.LastError.String, "unsupported payment domain outbox event type")
		require.True(t, arg.NextRetryAt.Valid)
		require.True(t, arg.NextRetryAt.Time.After(time.Now().UTC()))
		return db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusFailed}, nil
	})

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_PublishesProfitSharingResultReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	outbox := buildProfitSharingResultReadyOutbox(t, 905, worker.ProfitSharingResultPayload{
		ProfitSharingOrderID: 3001,
		OutOrderNo:           "PS3001",
		Result:               "SUCCESS",
		MerchantID:           801,
	})

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetMerchant(gomock.Any(), int64(801)).Return(db.Merchant{ID: 801, OwnerUserID: 9001}, nil)
	store.EXPECT().GetProfitSharingOrderByOutOrderNo(gomock.Any(), "PS3001").Return(db.ProfitSharingOrder{
		ID:                 3001,
		MerchantID:         801,
		MerchantAmount:     1234,
		PlatformCommission: 100,
		OperatorCommission: 50,
		PaymentFee:         8,
	}, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error {
		require.Equal(t, int64(9001), payload.UserID)
		require.Equal(t, "finance", payload.Type)
		require.Equal(t, "订单收入已到账", payload.Title)
		require.Equal(t, int64(3001), payload.RelatedID)
		require.Equal(t, int64(1234), payload.ExtraData["merchant_receivable_amount"])
		require.Equal(t, int64(150), payload.ExtraData["platform_service_fee_amount"])
		require.Equal(t, int64(8), payload.ExtraData["payment_channel_fee_amount"])
		require.NotContains(t, payload.ExtraData, "merchant_amount")
		require.NotContains(t, payload.ExtraData, "platform_commission")
		require.NotContains(t, payload.ExtraData, "operator_commission")
		require.Len(t, opts, 1)
		return nil
	})
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_PublishesRiderProfitSharingResultReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	outbox := buildProfitSharingResultReadyOutbox(t, 907, worker.ProfitSharingResultPayload{
		ProfitSharingOrderID: 3003,
		OutOrderNo:           "PS3003",
		Result:               "SUCCESS",
		MerchantID:           803,
	})

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetMerchant(gomock.Any(), int64(803)).Return(db.Merchant{ID: 803, OwnerUserID: 9003}, nil)
	store.EXPECT().GetProfitSharingOrderByOutOrderNo(gomock.Any(), "PS3003").Return(db.ProfitSharingOrder{
		ID:                 3003,
		MerchantID:         803,
		MerchantAmount:     1234,
		PlatformCommission: 100,
		OperatorCommission: 50,
		RiderID:            pgtype.Int8{Int64: 88, Valid: true},
		RiderAmount:        700,
	}, nil)
	store.EXPECT().GetRider(gomock.Any(), int64(88)).Return(db.Rider{ID: 88, UserID: 9103}, nil)

	var payloads []*worker.SendNotificationPayload
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error {
		payloads = append(payloads, payload)
		require.Len(t, opts, 1)
		return nil
	}).Times(2)
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
	require.Len(t, payloads, 2)
	require.Equal(t, int64(9003), payloads[0].UserID)
	require.Equal(t, int64(9103), payloads[1].UserID)
	require.Equal(t, "finance", payloads[1].Type)
	require.Equal(t, "代取费已到账", payloads[1].Title)
	require.Equal(t, "profit_sharing_order", payloads[1].RelatedType)
	require.Equal(t, int64(3003), payloads[1].RelatedID)
	require.True(t, payloads[1].IgnorePreferences)
	require.Equal(t, int64(3003), payloads[1].ExtraData["profit_sharing_order_id"])
	require.Equal(t, int64(700), payloads[1].ExtraData["rider_amount"])
	require.Equal(t, "SUCCESS", payloads[1].ExtraData["result"])
}

func TestProcessTaskPaymentDomainOutbox_AutoAcceptsPaidOrderAndSchedulesPrint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	order := db.Order{
		ID:                6401,
		MerchantID:        7301,
		UserID:            8101,
		OrderNo:           "ORD6401",
		OrderType:         db.OrderTypeTakeout,
		Status:            db.OrderStatusPaid,
		FulfillmentStatus: db.FulfillmentStatusPendingKitchen,
	}
	paymentOrder := db.PaymentOrder{
		ID:           9101,
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		Status:       "paid",
		Amount:       order.TotalAmount,
	}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             9201,
		PaymentOrderID: paymentOrder.ID,
		MerchantID:     order.MerchantID,
		TotalAmount:    order.TotalAmount,
		MerchantAmount: order.TotalAmount,
	}
	outbox := buildOrderPaymentSucceededOutbox(t, 9301, paymentOrder.ID, order.ID, order.MerchantID)
	config := db.OrderDisplayConfig{
		MerchantID:           order.MerchantID,
		EnablePrint:          true,
		PrintTakeout:         true,
		PrintDineIn:          true,
		PrintReservation:     true,
		PrintDispatchMode:    "single_full",
		PrintTriggerMode:     "accepted",
		AutoAcceptPaidOrders: true,
	}
	printer := db.CloudPrinter{
		ID:           7101,
		MerchantID:   order.MerchantID,
		PrinterType:  "feieyun",
		PrintTakeout: true,
		IsActive:     true,
	}
	acceptedOrder := order
	acceptedOrder.Status = db.OrderStatusPreparing
	acceptedOrder.FulfillmentStatus = db.FulfillmentStatusPreparing
	poolItem := db.DeliveryPool{OrderID: order.ID, MerchantID: order.MerchantID}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), order.MerchantID).Return(config, nil)
	store.EXPECT().ListActiveCloudPrintersByMerchant(gomock.Any(), order.MerchantID).Return([]db.CloudPrinter{printer}, nil)
	store.EXPECT().GetOrderForUpdate(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchantProfile(gomock.Any(), order.MerchantID).Return(db.GetMerchantProfileRow{MerchantID: order.MerchantID}, nil)
	store.EXPECT().AcceptTakeoutOrderTx(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.AcceptTakeoutOrderTxParams) (db.AcceptTakeoutOrderTxResult, error) {
		require.Equal(t, order.ID, arg.OrderID)
		require.Equal(t, db.OrderStatusPaid, arg.OldStatus)
		require.Equal(t, order.MerchantID, arg.OperatorID)
		require.Equal(t, "merchant", arg.OperatorType)
		return db.AcceptTakeoutOrderTxResult{Order: acceptedOrder, PoolItem: poolItem}, nil
	})
	distributor.EXPECT().DistributeTaskPrintOrder(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.PrintOrderPayload, opts ...asynq.Option) error {
		require.Equal(t, order.ID, payload.OrderID)
		require.Equal(t, "accepted", payload.Trigger)
		require.Equal(t, fmt.Sprintf("order:%d:accepted", order.ID), payload.TaskKey)
		requireHasUniqueOption(t, opts)
		return nil
	})
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any()).AnyTimes().Return(nil)
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{}, db.ErrRecordNotFound)
	store.EXPECT().GetDeliveryPoolByOrderID(gomock.Any(), order.ID).Return(db.DeliveryPool{}, db.ErrRecordNotFound)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, OwnerUserID: 9901}, nil).AnyTimes()
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
	}).Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_RetriesAcceptedPrintForAutoAcceptedOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	order := db.Order{
		ID:                6402,
		MerchantID:        7302,
		UserID:            8102,
		OrderNo:           "ORD6402",
		OrderType:         db.OrderTypeTakeout,
		Status:            db.OrderStatusPreparing,
		FulfillmentStatus: db.FulfillmentStatusPreparing,
	}
	paymentOrder := db.PaymentOrder{
		ID:           9102,
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		Status:       "paid",
		Amount:       order.TotalAmount,
	}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             9202,
		PaymentOrderID: paymentOrder.ID,
		MerchantID:     order.MerchantID,
		TotalAmount:    order.TotalAmount,
		MerchantAmount: order.TotalAmount,
	}
	outbox := buildOrderPaymentSucceededOutbox(t, 9302, paymentOrder.ID, order.ID, order.MerchantID)
	config := db.OrderDisplayConfig{
		MerchantID:           order.MerchantID,
		EnablePrint:          true,
		PrintTakeout:         true,
		PrintDineIn:          true,
		PrintReservation:     true,
		PrintDispatchMode:    "single_full",
		PrintTriggerMode:     "accepted",
		AutoAcceptPaidOrders: true,
	}
	printer := db.CloudPrinter{
		ID:           7102,
		MerchantID:   order.MerchantID,
		PrinterType:  "feieyun",
		PrintTakeout: true,
		IsActive:     true,
	}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), order.MerchantID).Return(config, nil)
	store.EXPECT().ListActiveCloudPrintersByMerchant(gomock.Any(), order.MerchantID).Return([]db.CloudPrinter{printer}, nil)
	store.EXPECT().ListPrintLogsByOrder(gomock.Any(), order.ID).Return([]db.ListPrintLogsByOrderRow{}, nil)
	distributor.EXPECT().DistributeTaskPrintOrder(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.PrintOrderPayload, opts ...asynq.Option) error {
		require.Equal(t, order.ID, payload.OrderID)
		require.Equal(t, "accepted", payload.Trigger)
		require.Equal(t, fmt.Sprintf("order:%d:accepted", order.ID), payload.TaskKey)
		requireHasUniqueOption(t, opts)
		return nil
	})
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any()).AnyTimes().Return(nil)
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{}, db.ErrRecordNotFound)
	store.EXPECT().GetDeliveryPoolByOrderID(gomock.Any(), order.ID).Return(db.DeliveryPool{}, db.ErrRecordNotFound)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, OwnerUserID: 9902}, nil).AnyTimes()
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
	}).Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_SkipsAcceptedPrintRetryWhenPrintLogExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	order := db.Order{
		ID:                6404,
		MerchantID:        7304,
		UserID:            8104,
		OrderNo:           "ORD6404",
		OrderType:         db.OrderTypeTakeout,
		Status:            db.OrderStatusPreparing,
		FulfillmentStatus: db.FulfillmentStatusPreparing,
	}
	paymentOrder := db.PaymentOrder{
		ID:           9104,
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		Status:       "paid",
		Amount:       order.TotalAmount,
	}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             9204,
		PaymentOrderID: paymentOrder.ID,
		MerchantID:     order.MerchantID,
		TotalAmount:    order.TotalAmount,
		MerchantAmount: order.TotalAmount,
	}
	outbox := buildOrderPaymentSucceededOutbox(t, 9304, paymentOrder.ID, order.ID, order.MerchantID)
	config := db.OrderDisplayConfig{
		MerchantID:           order.MerchantID,
		EnablePrint:          true,
		PrintTakeout:         true,
		PrintDineIn:          true,
		PrintReservation:     true,
		PrintDispatchMode:    "single_full",
		PrintTriggerMode:     "accepted",
		AutoAcceptPaidOrders: true,
	}
	printer := db.CloudPrinter{
		ID:           7104,
		MerchantID:   order.MerchantID,
		PrinterType:  "feieyun",
		PrintTakeout: true,
		IsActive:     true,
	}
	taskKey := fmt.Sprintf("order:%d:accepted", order.ID)

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), order.MerchantID).Return(config, nil)
	store.EXPECT().ListActiveCloudPrintersByMerchant(gomock.Any(), order.MerchantID).Return([]db.CloudPrinter{printer}, nil)
	store.EXPECT().ListPrintLogsByOrder(gomock.Any(), order.ID).Return([]db.ListPrintLogsByOrderRow{
		{ID: 8804, OrderID: order.ID, PrinterID: 7004, TaskKey: pgtype.Text{String: taskKey, Valid: true}},
	}, nil)
	distributor.EXPECT().DistributeTaskPrintOrder(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any()).AnyTimes().Return(nil)
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{}, db.ErrRecordNotFound)
	store.EXPECT().GetDeliveryPoolByOrderID(gomock.Any(), order.ID).Return(db.DeliveryPool{}, db.ErrRecordNotFound)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, OwnerUserID: 9904}, nil).AnyTimes()
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
	}).Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_TreatsDuplicateAcceptedPrintTaskAsScheduled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	order := db.Order{
		ID:                6405,
		MerchantID:        7305,
		UserID:            8105,
		OrderNo:           "ORD6405",
		OrderType:         db.OrderTypeTakeout,
		Status:            db.OrderStatusPreparing,
		FulfillmentStatus: db.FulfillmentStatusPreparing,
	}
	paymentOrder := db.PaymentOrder{
		ID:           9105,
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		Status:       "paid",
		Amount:       order.TotalAmount,
	}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             9205,
		PaymentOrderID: paymentOrder.ID,
		MerchantID:     order.MerchantID,
		TotalAmount:    order.TotalAmount,
		MerchantAmount: order.TotalAmount,
	}
	outbox := buildOrderPaymentSucceededOutbox(t, 9305, paymentOrder.ID, order.ID, order.MerchantID)
	config := db.OrderDisplayConfig{
		MerchantID:           order.MerchantID,
		EnablePrint:          true,
		PrintTakeout:         true,
		PrintDineIn:          true,
		PrintReservation:     true,
		PrintDispatchMode:    "single_full",
		PrintTriggerMode:     "accepted",
		AutoAcceptPaidOrders: true,
	}
	printer := db.CloudPrinter{
		ID:           7105,
		MerchantID:   order.MerchantID,
		PrinterType:  "feieyun",
		PrintTakeout: true,
		IsActive:     true,
	}
	taskKey := fmt.Sprintf("order:%d:accepted", order.ID)

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), order.MerchantID).Return(config, nil)
	store.EXPECT().ListActiveCloudPrintersByMerchant(gomock.Any(), order.MerchantID).Return([]db.CloudPrinter{printer}, nil)
	store.EXPECT().ListPrintLogsByOrder(gomock.Any(), order.ID).Return([]db.ListPrintLogsByOrderRow{}, nil)
	distributor.EXPECT().DistributeTaskPrintOrder(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.PrintOrderPayload, opts ...asynq.Option) error {
		require.Equal(t, order.ID, payload.OrderID)
		require.Equal(t, "accepted", payload.Trigger)
		require.Equal(t, taskKey, payload.TaskKey)
		requireHasUniqueOption(t, opts)
		return asynq.ErrDuplicateTask
	})
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any()).AnyTimes().Return(nil)
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{}, db.ErrRecordNotFound)
	store.EXPECT().GetDeliveryPoolByOrderID(gomock.Any(), order.ID).Return(db.DeliveryPool{}, db.ErrRecordNotFound)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, OwnerUserID: 9905}, nil).AnyTimes()
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
	}).Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_DoesNotAutoAcceptWhenAcceptedPrintTriggerDisabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	order := db.Order{
		ID:                6403,
		MerchantID:        7303,
		UserID:            8103,
		OrderNo:           "ORD6403",
		OrderType:         db.OrderTypeTakeout,
		Status:            db.OrderStatusPaid,
		FulfillmentStatus: db.FulfillmentStatusPendingKitchen,
	}
	paymentOrder := db.PaymentOrder{
		ID:           9103,
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		Status:       "paid",
		Amount:       order.TotalAmount,
	}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:             9203,
		PaymentOrderID: paymentOrder.ID,
		MerchantID:     order.MerchantID,
		TotalAmount:    order.TotalAmount,
		MerchantAmount: order.TotalAmount,
	}
	outbox := buildOrderPaymentSucceededOutbox(t, 9303, paymentOrder.ID, order.ID, order.MerchantID)
	config := db.OrderDisplayConfig{
		MerchantID:           order.MerchantID,
		EnablePrint:          true,
		PrintTakeout:         true,
		PrintDineIn:          true,
		PrintReservation:     true,
		PrintDispatchMode:    "single_full",
		PrintTriggerMode:     "ready",
		AutoAcceptPaidOrders: true,
	}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), order.MerchantID).Return(config, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any()).AnyTimes().Return(nil)
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(db.Delivery{}, db.ErrRecordNotFound)
	store.EXPECT().GetDeliveryPoolByOrderID(gomock.Any(), order.ID).Return(db.DeliveryPool{}, db.ErrRecordNotFound)
	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(db.Merchant{ID: order.MerchantID, OwnerUserID: 9903}, nil).AnyTimes()
	store.EXPECT().GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
	}).Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_PublishesFailedProfitSharingResultReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	outbox := buildProfitSharingResultReadyOutbox(t, 908, worker.ProfitSharingResultPayload{
		ProfitSharingOrderID: 3004,
		OutOrderNo:           "PS3004",
		Result:               "FAILED",
		FailReason:           "NO_RELATION",
		MerchantID:           804,
	})

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetMerchant(gomock.Any(), int64(804)).Return(db.Merchant{ID: 804, OwnerUserID: 9004}, nil)
	store.EXPECT().GetProfitSharingOrderByOutOrderNo(gomock.Any(), "PS3004").Return(db.ProfitSharingOrder{
		ID:             3004,
		MerchantID:     804,
		PaymentOrderID: 7004,
		MerchantAmount: 1234,
	}, nil)
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_PublishesRiderFailedProfitSharingResultReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	outbox := buildProfitSharingResultReadyOutbox(t, 909, worker.ProfitSharingResultPayload{
		ProfitSharingOrderID: 3005,
		OutOrderNo:           "PS3005",
		Result:               "FAILED",
		FailReason:           "NO_RELATION",
		MerchantID:           805,
	})

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetMerchant(gomock.Any(), int64(805)).Return(db.Merchant{ID: 805, OwnerUserID: 9005}, nil)
	store.EXPECT().GetProfitSharingOrderByOutOrderNo(gomock.Any(), "PS3005").Return(db.ProfitSharingOrder{
		ID:             3005,
		MerchantID:     805,
		PaymentOrderID: 7005,
		MerchantAmount: 1234,
		RiderID:        pgtype.Int8{Int64: 89, Valid: true},
		RiderAmount:    800,
	}, nil)
	store.EXPECT().GetRider(gomock.Any(), int64(89)).Return(db.Rider{ID: 89, UserID: 9105}, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error {
		require.Len(t, opts, 1)
		require.Equal(t, int64(9105), payload.UserID)
		require.Equal(t, "代取费结算处理中", payload.Title)
		require.NotContains(t, payload.Content, "NO_RELATION")
		require.True(t, payload.IgnorePreferences)
		require.Equal(t, int64(3005), payload.ExtraData["profit_sharing_order_id"])
		require.Equal(t, "FAILED", payload.ExtraData["result"])
		return nil
	})
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_OrderPaymentSucceededWithoutDistributorMarksFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	outbox := buildOrderPaymentSucceededOutbox(t, 911, 402, 502, 602)
	paymentOrder := db.PaymentOrder{ID: 402, OrderID: pgtype.Int8{Int64: 502, Valid: true}, PaymentChannel: db.PaymentChannelDirect, BusinessType: db.ExternalPaymentBusinessOwnerOrder}
	order := db.Order{ID: 502, MerchantID: 602, OrderNo: "ORD502", OrderType: "dinein", TotalAmount: 6600}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), int64(402)).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(502)).Return(order, nil)
	store.EXPECT().MarkPaymentDomainOutboxFailed(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkPaymentDomainOutboxFailedParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, outbox.ID, arg.ID)
		require.True(t, arg.LastError.Valid)
		require.Contains(t, arg.LastError.String, "task distributor not configured")
		return db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusFailed}, nil
	})

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_ProfitSharingMissingDistributorMarksFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	outbox := buildProfitSharingResultReadyOutbox(t, 907, worker.ProfitSharingResultPayload{
		ProfitSharingOrderID: 3003,
		OutOrderNo:           "PS3003",
		Result:               "SUCCESS",
		MerchantID:           803,
	})

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetMerchant(gomock.Any(), int64(803)).Return(db.Merchant{ID: 803, OwnerUserID: 9003}, nil)
	store.EXPECT().GetProfitSharingOrderByOutOrderNo(gomock.Any(), "PS3003").Return(db.ProfitSharingOrder{ID: 3003, MerchantID: 803, MerchantAmount: 1234}, nil)
	store.EXPECT().MarkPaymentDomainOutboxFailed(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkPaymentDomainOutboxFailedParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, outbox.ID, arg.ID)
		require.True(t, arg.LastError.Valid)
		require.Contains(t, arg.LastError.String, "task distributor not configured")
		require.True(t, arg.NextRetryAt.Valid)
		return db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusFailed}, nil
	})

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_SkipsUnclaimableOutbox(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(db.PaymentDomainOutbox{}, db.ErrRecordNotFound)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: 903})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_RejectsMissingOutboxID(t *testing.T) {
	processor := worker.NewTestTaskProcessor(nil, nil, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.Error(t, err)
	require.Contains(t, err.Error(), "payment domain outbox id is required")
}

func TestPaymentDomainOutboxSchedulerRunOnceEnqueuesDefaultEventTypes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &paymentDomainOutboxSchedulerTestDistributor{}

	gomock.InOrder(
		store.EXPECT().ReclaimStalePaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReclaimStalePaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, worker.PaymentDomainOutboxEventDispatcherProbe, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.False(t, arg.StaleBefore.IsZero())
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.LastError.Valid)
			require.Contains(t, arg.LastError.String, "stale processing")
			return nil, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, worker.PaymentDomainOutboxEventDispatcherProbe, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 904, EventType: worker.PaymentDomainOutboxEventDispatcherProbe}}, nil
		}),
		store.EXPECT().ReclaimStalePaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReclaimStalePaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.False(t, arg.StaleBefore.IsZero())
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.LastError.Valid)
			require.Contains(t, arg.LastError.String, "stale processing")
			return nil, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 910, EventType: db.PaymentDomainOutboxEventOrderPaymentSucceeded}}, nil
		}),
		store.EXPECT().ReclaimStalePaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReclaimStalePaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventReservationPaymentSucceeded, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.False(t, arg.StaleBefore.IsZero())
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.LastError.Valid)
			require.Contains(t, arg.LastError.String, "stale processing")
			return nil, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventReservationPaymentSucceeded, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 912, EventType: db.PaymentDomainOutboxEventReservationPaymentSucceeded}}, nil
		}),
		store.EXPECT().ReclaimStalePaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReclaimStalePaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventProfitSharingResultReady, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.False(t, arg.StaleBefore.IsZero())
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.LastError.Valid)
			require.Contains(t, arg.LastError.String, "stale processing")
			return nil, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventProfitSharingResultReady, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 905, EventType: db.PaymentDomainOutboxEventProfitSharingResultReady}}, nil
		}),
		store.EXPECT().ReclaimStalePaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReclaimStalePaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.False(t, arg.StaleBefore.IsZero())
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.LastError.Valid)
			require.Contains(t, arg.LastError.String, "stale processing")
			return nil, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 907, EventType: db.PaymentDomainOutboxEventOrderRefundSucceeded}}, nil
		}),
		store.EXPECT().ReclaimStalePaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReclaimStalePaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventOrderRefundAbnormal, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.False(t, arg.StaleBefore.IsZero())
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.LastError.Valid)
			require.Contains(t, arg.LastError.String, "stale processing")
			return nil, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventOrderRefundAbnormal, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 908, EventType: db.PaymentDomainOutboxEventOrderRefundAbnormal}}, nil
		}),
		store.EXPECT().ReclaimStalePaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReclaimStalePaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventReservationRefundAbnormal, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.False(t, arg.StaleBefore.IsZero())
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.LastError.Valid)
			require.Contains(t, arg.LastError.String, "stale processing")
			return nil, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventReservationRefundAbnormal, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 909, EventType: db.PaymentDomainOutboxEventReservationRefundAbnormal}}, nil
		}),
		store.EXPECT().ReclaimStalePaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReclaimStalePaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventRiderDepositRefundAbnormal, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.False(t, arg.StaleBefore.IsZero())
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.LastError.Valid)
			require.Contains(t, arg.LastError.String, "stale processing")
			return nil, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventRiderDepositRefundAbnormal, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 906, EventType: db.PaymentDomainOutboxEventRiderDepositRefundAbnormal}}, nil
		}),
	)

	scheduler := worker.NewPaymentDomainOutboxScheduler(store, distributor)
	scheduler.RunOnce()

	require.Equal(t, []int64{904, 910, 912, 905, 907, 908, 909, 906}, distributor.outboxIDs)
	require.Len(t, distributor.optionCounts, 8)
	require.GreaterOrEqual(t, distributor.optionCounts[0], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[1], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[2], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[3], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[4], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[5], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[6], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[7], 3)
}

func TestPaymentDomainOutboxSchedulerRunOnceEnqueuesConfiguredEventTypes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &paymentDomainOutboxSchedulerTestDistributor{}

	gomock.InOrder(
		store.EXPECT().ReclaimStalePaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReclaimStalePaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, worker.PaymentDomainOutboxEventDispatcherProbe, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.False(t, arg.StaleBefore.IsZero())
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.LastError.Valid)
			require.Contains(t, arg.LastError.String, "stale processing")
			return nil, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, worker.PaymentDomainOutboxEventDispatcherProbe, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 904, EventType: worker.PaymentDomainOutboxEventDispatcherProbe}}, nil
		}),
		store.EXPECT().ReclaimStalePaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReclaimStalePaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventProfitSharingResultReady, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.False(t, arg.StaleBefore.IsZero())
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.LastError.Valid)
			require.Contains(t, arg.LastError.String, "stale processing")
			return nil, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventProfitSharingResultReady, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 905, EventType: db.PaymentDomainOutboxEventProfitSharingResultReady}}, nil
		}),
	)

	scheduler := worker.NewPaymentDomainOutboxSchedulerWithEventTypes(store, distributor, []string{
		worker.PaymentDomainOutboxEventDispatcherProbe,
		db.PaymentDomainOutboxEventProfitSharingResultReady,
	})
	scheduler.RunOnce()

	require.Equal(t, []int64{904, 905}, distributor.outboxIDs)
	require.Len(t, distributor.optionCounts, 2)
	require.GreaterOrEqual(t, distributor.optionCounts[0], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[1], 3)
}

func TestPaymentDomainOutboxSchedulerRunOnceReclaimsStaleProcessingEntries(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &paymentDomainOutboxSchedulerTestDistributor{}

	gomock.InOrder(
		store.EXPECT().ReclaimStalePaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ReclaimStalePaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventProfitSharingResultReady, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.False(t, arg.StaleBefore.IsZero())
			require.True(t, arg.NextRetryAt.Valid)
			require.True(t, arg.LastError.Valid)
			require.Contains(t, arg.LastError.String, "stale processing")
			return []db.PaymentDomainOutbox{{
				ID:            931,
				EventType:     db.PaymentDomainOutboxEventProfitSharingResultReady,
				AggregateType: db.PaymentDomainOutboxAggregateProfitSharingOrder,
				AggregateID:   801,
				Status:        db.PaymentDomainOutboxStatusFailed,
			}}, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventProfitSharingResultReady, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{
				ID:            931,
				EventType:     db.PaymentDomainOutboxEventProfitSharingResultReady,
				AggregateType: db.PaymentDomainOutboxAggregateProfitSharingOrder,
				AggregateID:   801,
				Status:        db.PaymentDomainOutboxStatusFailed,
			}}, nil
		}),
	)

	scheduler := worker.NewPaymentDomainOutboxSchedulerWithEventTypes(store, distributor, []string{
		db.PaymentDomainOutboxEventProfitSharingResultReady,
	})
	scheduler.RunOnce()

	require.Equal(t, []int64{931}, distributor.outboxIDs)
}

func TestPaymentDomainOutboxSchedulerRunOnceSkipsWithoutDistributor(t *testing.T) {
	scheduler := worker.NewPaymentDomainOutboxScheduler(nil, nil)
	scheduler.RunOnce()
}

type paymentDomainOutboxSchedulerTestDistributor struct {
	outboxIDs    []int64
	optionCounts []int
}

func (d *paymentDomainOutboxSchedulerTestDistributor) DistributeTaskProcessPaymentDomainOutbox(_ context.Context, payload *worker.PaymentDomainOutboxPayload, opts ...asynq.Option) error {
	d.outboxIDs = append(d.outboxIDs, payload.OutboxID)
	d.optionCounts = append(d.optionCounts, len(opts))
	return nil
}

func buildProfitSharingResultReadyOutbox(t *testing.T, outboxID int64, payload worker.ProfitSharingResultPayload) db.PaymentDomainOutbox {
	t.Helper()
	rawPayload, err := json.Marshal(payload)
	require.NoError(t, err)
	return db.PaymentDomainOutbox{
		ID:            outboxID,
		EventType:     db.PaymentDomainOutboxEventProfitSharingResultReady,
		AggregateType: db.PaymentDomainOutboxAggregateProfitSharingOrder,
		AggregateID:   payload.ProfitSharingOrderID,
		Payload:       rawPayload,
		Status:        db.PaymentDomainOutboxStatusProcessing,
	}
}

func buildOrderPaymentSucceededOutbox(t *testing.T, outboxID int64, paymentOrderID int64, orderID int64, merchantID int64) db.PaymentDomainOutbox {
	t.Helper()
	rawPayload, err := json.Marshal(map[string]any{
		"payment_order_id":            paymentOrderID,
		"order_id":                    orderID,
		"merchant_id":                 merchantID,
		"order_no":                    fmt.Sprintf("ORD%d", orderID),
		"external_payment_fact_id":    7101,
		"payment_fact_application_id": 8101,
	})
	require.NoError(t, err)
	return db.PaymentDomainOutbox{
		ID:            outboxID,
		EventType:     db.PaymentDomainOutboxEventOrderPaymentSucceeded,
		AggregateType: db.PaymentDomainOutboxAggregatePaymentOrder,
		AggregateID:   paymentOrderID,
		Payload:       rawPayload,
		Status:        db.PaymentDomainOutboxStatusProcessing,
	}
}

func buildReservationPaymentSucceededOutbox(t *testing.T, outboxID int64, paymentOrderID int64, reservationID int64) db.PaymentDomainOutbox {
	t.Helper()
	rawPayload, err := json.Marshal(map[string]any{
		"payment_order_id":            paymentOrderID,
		"reservation_id":              reservationID,
		"business_type":               db.ExternalPaymentBusinessOwnerReservation,
		"external_payment_fact_id":    7201,
		"payment_fact_application_id": 8201,
	})
	require.NoError(t, err)
	return db.PaymentDomainOutbox{
		ID:            outboxID,
		EventType:     db.PaymentDomainOutboxEventReservationPaymentSucceeded,
		AggregateType: db.PaymentDomainOutboxAggregatePaymentOrder,
		AggregateID:   paymentOrderID,
		Payload:       rawPayload,
		Status:        db.PaymentDomainOutboxStatusProcessing,
	}
}

func requireHasUniqueOption(t *testing.T, opts []asynq.Option) {
	t.Helper()
	for _, opt := range opts {
		if opt.Type() == asynq.UniqueOpt {
			return
		}
	}
	require.Fail(t, "expected unique option for accepted print task")
}
