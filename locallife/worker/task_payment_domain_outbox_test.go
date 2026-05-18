package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
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
	require.Equal(t, "配送费已到账", payloads[1].Title)
	require.Equal(t, "profit_sharing_order", payloads[1].RelatedType)
	require.Equal(t, int64(3003), payloads[1].RelatedID)
	require.True(t, payloads[1].IgnorePreferences)
	require.Equal(t, int64(3003), payloads[1].ExtraData["profit_sharing_order_id"])
	require.Equal(t, int64(700), payloads[1].ExtraData["rider_amount"])
	require.Equal(t, "SUCCESS", payloads[1].ExtraData["result"])
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
	store.EXPECT().GetPaymentOrder(gomock.Any(), int64(7004)).Return(db.PaymentOrder{ID: 7004, OrderID: pgtype.Int8{Int64: 6004, Valid: true}}, nil)
	distributor.EXPECT().DistributeTaskProcessProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingPayload{}), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.ProfitSharingPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(7004), payload.PaymentOrderID)
		require.Equal(t, int64(6004), payload.OrderID)
		return nil
	})
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
	store.EXPECT().GetPaymentOrder(gomock.Any(), int64(7005)).Return(db.PaymentOrder{ID: 7005, OrderID: pgtype.Int8{Int64: 6005, Valid: true}}, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error {
		require.Len(t, opts, 1)
		require.Equal(t, int64(9105), payload.UserID)
		require.Equal(t, "配送费结算处理中", payload.Title)
		require.NotContains(t, payload.Content, "NO_RELATION")
		require.True(t, payload.IgnorePreferences)
		require.Equal(t, int64(3005), payload.ExtraData["profit_sharing_order_id"])
		require.Equal(t, "FAILED", payload.ExtraData["result"])
		return nil
	})
	distributor.EXPECT().DistributeTaskProcessProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingPayload{}), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.ProfitSharingPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(7005), payload.PaymentOrderID)
		require.Equal(t, int64(6005), payload.OrderID)
		return nil
	})
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_RiderFailedProfitSharingNotificationFailureStillQueuesRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	outbox := buildProfitSharingResultReadyOutbox(t, 910, worker.ProfitSharingResultPayload{
		ProfitSharingOrderID: 3006,
		OutOrderNo:           "PS3006",
		Result:               "FAILED",
		FailReason:           "NO_RELATION",
		MerchantID:           806,
	})

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetMerchant(gomock.Any(), int64(806)).Return(db.Merchant{ID: 806, OwnerUserID: 9006}, nil)
	store.EXPECT().GetProfitSharingOrderByOutOrderNo(gomock.Any(), "PS3006").Return(db.ProfitSharingOrder{
		ID:             3006,
		MerchantID:     806,
		PaymentOrderID: 7006,
		MerchantAmount: 1234,
		RiderID:        pgtype.Int8{Int64: 90, Valid: true},
		RiderAmount:    800,
	}, nil)
	store.EXPECT().GetRider(gomock.Any(), int64(90)).Return(db.Rider{ID: 90, UserID: 9106}, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), int64(7006)).Return(db.PaymentOrder{ID: 7006, OrderID: pgtype.Int8{Int64: 6006, Valid: true}}, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any()).Return(errors.New("redis unavailable"))
	distributor.EXPECT().DistributeTaskProcessProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingPayload{}), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.ProfitSharingPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(7006), payload.PaymentOrderID)
		require.Equal(t, int64(6006), payload.OrderID)
		return nil
	})
	store.EXPECT().MarkPaymentDomainOutboxFailed(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkPaymentDomainOutboxFailedParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, outbox.ID, arg.ID)
		require.True(t, arg.LastError.Valid)
		require.Contains(t, arg.LastError.String, "enqueue rider profit sharing notification")
		return db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusFailed}, nil
	})

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_PublishesOrderPaymentSucceeded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	outbox := buildOrderPaymentSucceededOutbox(t, 910, 401, 501, 601)
	paymentOrder := db.PaymentOrder{ID: 401, OrderID: pgtype.Int8{Int64: 501, Valid: true}, PaymentChannel: db.PaymentChannelEcommerce, BusinessType: db.ExternalPaymentBusinessOwnerOrder, RequiresProfitSharing: true}
	order := db.Order{ID: 501, MerchantID: 601, OrderNo: "ORD501", OrderType: "dinein", Status: db.OrderStatusPaid, Subtotal: 8800, TotalAmount: 8800}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                 801,
		PaymentOrderID:     paymentOrder.ID,
		MerchantID:         order.MerchantID,
		TotalAmount:        order.TotalAmount,
		PlatformCommission: 176,
		OperatorCommission: 264,
		PaymentFee:         26,
		MerchantAmount:     8334,
	}
	merchant := db.Merchant{ID: 601, OwnerUserID: 701}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), int64(401)).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(501)).Return(order, nil)
	distributor.EXPECT().DistributeTaskProcessProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingPayload{}), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.ProfitSharingPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(401), payload.PaymentOrderID)
		require.Equal(t, int64(501), payload.OrderID)
		return nil
	})
	store.EXPECT().GetMerchant(gomock.Any(), int64(601)).Return(merchant, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
		require.Equal(t, merchant.OwnerUserID, payload.UserID)
		require.Equal(t, "order", payload.Type)
		require.Equal(t, order.ID, payload.RelatedID)
		breakdown, ok := payload.ExtraData["fee_breakdown"].(logic.MerchantOrderFeeBreakdown)
		require.True(t, ok)
		require.Equal(t, int64(8800), breakdown.FoodAmount)
		require.Equal(t, int64(8800), breakdown.CustomerPayableAmount)
		require.Equal(t, int64(440), breakdown.PlatformServiceFeeAmount)
		require.Equal(t, int64(26), breakdown.PaymentChannelFeeAmount)
		require.Equal(t, int64(8334), breakdown.MerchantReceivableAmount)
		return nil
	})
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_OrderItemLoadFailureMarksFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	outbox := buildOrderPaymentSucceededOutbox(t, 911, 402, 502, 602)
	paymentOrder := db.PaymentOrder{ID: 402, OrderID: pgtype.Int8{Int64: 502, Valid: true}, PaymentChannel: db.PaymentChannelDirect, BusinessType: db.ExternalPaymentBusinessOwnerOrder}
	order := db.Order{ID: 502, MerchantID: 602, OrderNo: "ORD502", OrderType: "dinein", Status: db.OrderStatusPaid, Subtotal: 9900, TotalAmount: 9900}
	merchant := db.Merchant{ID: 602, OwnerUserID: 702}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), int64(402)).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(502)).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), int64(602)).Return(merchant, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return(nil, errors.New("database unavailable"))
	store.EXPECT().MarkPaymentDomainOutboxFailed(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkPaymentDomainOutboxFailedParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, outbox.ID, arg.ID)
		require.True(t, arg.LastError.Valid)
		require.Contains(t, arg.LastError.String, "load order items for merchant new order snapshot")
		require.True(t, arg.NextRetryAt.Valid)
		return db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusFailed}, nil
	})

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_PublishesReservationPaymentSucceeded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	reservationDate := pgtype.Date{Time: time.Date(2026, 4, 27, 0, 0, 0, 0, time.Local), Valid: true}
	reservationTime := pgtype.Time{Microseconds: int64((18 * time.Hour) + (30 * time.Minute))}
	outbox := buildReservationPaymentSucceededOutbox(t, 912, 403, 503)
	paymentOrder := db.PaymentOrder{ID: 403, ReservationID: pgtype.Int8{Int64: 503, Valid: true}, PaymentChannel: db.PaymentChannelEcommerce, BusinessType: db.ExternalPaymentBusinessOwnerReservation}
	reservation := db.TableReservation{ID: 503, MerchantID: 603, ReservationDate: reservationDate, ReservationTime: reservationTime}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), int64(403)).Return(paymentOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), int64(503)).Return(reservation, nil)
	distributor.EXPECT().DistributeTaskProcessProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingPayload{}), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.ProfitSharingPayload, _ ...asynq.Option) error {
		require.Equal(t, int64(403), payload.PaymentOrderID)
		require.Equal(t, int64(503), payload.ReservationID)
		return nil
	})
	distributor.EXPECT().DistributeTaskReservationNoShowAlert(gomock.Any(), gomock.AssignableToTypeOf(&worker.PayloadReservationNoShowAlert{}), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.PayloadReservationNoShowAlert, opts ...asynq.Option) error {
		require.Equal(t, int64(503), payload.ReservationID)
		require.Len(t, opts, 2)
		return nil
	})
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_BaofuReservationDoesNotEnqueueLegacyWechatProfitSharing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	reservationDate := pgtype.Date{Time: time.Date(2026, 4, 27, 0, 0, 0, 0, time.Local), Valid: true}
	reservationTime := pgtype.Time{Microseconds: int64((18 * time.Hour) + (30 * time.Minute))}
	outbox := buildReservationPaymentSucceededOutbox(t, 914, 404, 504)
	paymentOrder := db.PaymentOrder{
		ID:             404,
		ReservationID:  pgtype.Int8{Int64: 504, Valid: true},
		PaymentChannel: db.PaymentChannelBaofuAggregate,
		BusinessType:   db.ExternalPaymentBusinessOwnerReservation,
	}
	reservation := db.TableReservation{ID: 504, MerchantID: 604, ReservationDate: reservationDate, ReservationTime: reservationTime}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), int64(404)).Return(paymentOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), int64(504)).Return(reservation, nil)
	distributor.EXPECT().DistributeTaskReservationNoShowAlert(gomock.Any(), gomock.AssignableToTypeOf(&worker.PayloadReservationNoShowAlert{}), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.PayloadReservationNoShowAlert, opts ...asynq.Option) error {
		require.Equal(t, int64(504), payload.ReservationID)
		require.Len(t, opts, 2)
		return nil
	})
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_PublishesApplymentActivated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	outbox := buildApplymentActivatedOutbox(t, 913, 551, 651)
	applyment := db.EcommerceApplyment{ID: 551, SubjectType: "merchant", SubjectID: 651, OutRequestNo: "APPLY_M_551", SubMchID: pgtype.Text{String: "sub_mch_551", Valid: true}}
	merchant := db.Merchant{ID: 651, OwnerUserID: 7651, Name: "测试商户开户"}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), int64(551)).Return(applyment, nil)
	store.EXPECT().GetMerchant(gomock.Any(), int64(651)).Return(merchant, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
		require.Equal(t, merchant.OwnerUserID, payload.UserID)
		require.Equal(t, "微信支付开户成功", payload.Title)
		require.Equal(t, applyment.ID, payload.RelatedID)
		return nil
	})
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_PublishesApplymentRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	outbox := buildApplymentTerminalOutbox(t, 914, 552, 652, "rejected", "资料驳回")
	applyment := db.EcommerceApplyment{ID: 552, SubjectType: "merchant", SubjectID: 652, OutRequestNo: "APPLY_M_552", Status: "rejected", RejectReason: pgtype.Text{String: "资料驳回", Valid: true}}
	merchant := db.Merchant{ID: 652, OwnerUserID: 7652, Name: "测试商户驳回"}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), int64(552)).Return(applyment, nil)
	store.EXPECT().GetMerchant(gomock.Any(), int64(652)).Return(merchant, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
		require.Equal(t, merchant.OwnerUserID, payload.UserID)
		require.Equal(t, "微信支付开户被驳回", payload.Title)
		require.Contains(t, payload.Content, "资料驳回")
		return nil
	})
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(worker.TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_PublishesApplymentPending(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	outbox := buildApplymentPendingOutbox(t, 915, 553, 653, "account_need_verify")
	applyment := db.EcommerceApplyment{ID: 553, SubjectType: "merchant", SubjectID: 653, OutRequestNo: "APPLY_M_553", Status: "account_need_verify"}
	merchant := db.Merchant{ID: 653, OwnerUserID: 7653, Name: "测试商户待处理"}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetEcommerceApplyment(gomock.Any(), int64(553)).Return(applyment, nil)
	store.EXPECT().GetMerchant(gomock.Any(), int64(653)).Return(merchant, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
		require.Equal(t, merchant.OwnerUserID, payload.UserID)
		require.Equal(t, "微信支付开户待处理", payload.Title)
		require.Contains(t, payload.Content, "账户验证")
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
	paymentOrder := db.PaymentOrder{ID: 402, OrderID: pgtype.Int8{Int64: 502, Valid: true}, PaymentChannel: db.PaymentChannelEcommerce, BusinessType: db.ExternalPaymentBusinessOwnerOrder}
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

func TestProcessTaskPaymentDomainOutbox_ProfitSharingNotificationFailureMarksFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	outbox := buildProfitSharingResultReadyOutbox(t, 906, worker.ProfitSharingResultPayload{
		ProfitSharingOrderID: 3002,
		OutOrderNo:           "PS3002",
		Result:               "SUCCESS",
		MerchantID:           802,
	})

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetMerchant(gomock.Any(), int64(802)).Return(db.Merchant{ID: 802, OwnerUserID: 9002}, nil)
	store.EXPECT().GetProfitSharingOrderByOutOrderNo(gomock.Any(), "PS3002").Return(db.ProfitSharingOrder{ID: 3002, MerchantID: 802, MerchantAmount: 1234}, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any()).Return(errors.New("redis unavailable"))
	store.EXPECT().MarkPaymentDomainOutboxFailed(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkPaymentDomainOutboxFailedParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, outbox.ID, arg.ID)
		require.True(t, arg.LastError.Valid)
		require.Contains(t, arg.LastError.String, "enqueue profit sharing success notification")
		require.True(t, arg.NextRetryAt.Valid)
		return db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusFailed}, nil
	})

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
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
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, worker.PaymentDomainOutboxEventDispatcherProbe, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 904, EventType: worker.PaymentDomainOutboxEventDispatcherProbe}}, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventOrderPaymentSucceeded, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 910, EventType: db.PaymentDomainOutboxEventOrderPaymentSucceeded}}, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventReservationPaymentSucceeded, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 912, EventType: db.PaymentDomainOutboxEventReservationPaymentSucceeded}}, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventProfitSharingResultReady, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 905, EventType: db.PaymentDomainOutboxEventProfitSharingResultReady}}, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventApplymentActivated, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 913, EventType: db.PaymentDomainOutboxEventApplymentActivated}}, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventApplymentPendingStateReady, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 915, EventType: db.PaymentDomainOutboxEventApplymentPendingStateReady}}, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventApplymentTerminalStateReady, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 914, EventType: db.PaymentDomainOutboxEventApplymentTerminalStateReady}}, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventOrderRefundSucceeded, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 907, EventType: db.PaymentDomainOutboxEventOrderRefundSucceeded}}, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventOrderRefundAbnormal, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 908, EventType: db.PaymentDomainOutboxEventOrderRefundAbnormal}}, nil
		}),
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, db.PaymentDomainOutboxEventReservationRefundAbnormal, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 909, EventType: db.PaymentDomainOutboxEventReservationRefundAbnormal}}, nil
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

	require.Equal(t, []int64{904, 910, 912, 905, 913, 915, 914, 907, 908, 909, 906}, distributor.outboxIDs)
	require.Len(t, distributor.optionCounts, 11)
	require.GreaterOrEqual(t, distributor.optionCounts[0], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[1], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[2], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[3], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[4], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[5], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[6], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[7], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[8], 3)
	require.GreaterOrEqual(t, distributor.optionCounts[9], 3)
}

func TestPaymentDomainOutboxSchedulerRunOnceEnqueuesConfiguredEventTypes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &paymentDomainOutboxSchedulerTestDistributor{}

	gomock.InOrder(
		store.EXPECT().ListPendingPaymentDomainOutboxByEventType(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListPendingPaymentDomainOutboxByEventTypeParams) ([]db.PaymentDomainOutbox, error) {
			require.Equal(t, worker.PaymentDomainOutboxEventDispatcherProbe, arg.EventType)
			require.Equal(t, int32(200), arg.LimitCount)
			require.True(t, arg.NowAt.Valid)
			return []db.PaymentDomainOutbox{{ID: 904, EventType: worker.PaymentDomainOutboxEventDispatcherProbe}}, nil
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

func buildApplymentActivatedOutbox(t *testing.T, outboxID int64, applymentID int64, merchantID int64) db.PaymentDomainOutbox {
	t.Helper()
	rawPayload, err := json.Marshal(map[string]any{
		"applyment_id":                applymentID,
		"merchant_id":                 merchantID,
		"out_request_no":              fmt.Sprintf("APPLY_M_%d", applymentID),
		"sub_mch_id":                  fmt.Sprintf("sub_mch_%d", applymentID),
		"external_payment_fact_id":    7301,
		"payment_fact_application_id": 8301,
	})
	require.NoError(t, err)
	return db.PaymentDomainOutbox{
		ID:            outboxID,
		EventType:     db.PaymentDomainOutboxEventApplymentActivated,
		AggregateType: db.PaymentDomainOutboxAggregateEcommerceApplyment,
		AggregateID:   applymentID,
		Payload:       rawPayload,
		Status:        db.PaymentDomainOutboxStatusProcessing,
	}
}

func buildApplymentTerminalOutbox(t *testing.T, outboxID int64, applymentID int64, merchantID int64, applymentStatus, rejectReason string) db.PaymentDomainOutbox {
	t.Helper()
	rawPayload, err := json.Marshal(map[string]any{
		"applyment_id":                applymentID,
		"merchant_id":                 merchantID,
		"out_request_no":              fmt.Sprintf("APPLY_M_%d", applymentID),
		"applyment_status":            applymentStatus,
		"reject_reason":               rejectReason,
		"external_payment_fact_id":    7302,
		"payment_fact_application_id": 8302,
	})
	require.NoError(t, err)
	return db.PaymentDomainOutbox{
		ID:            outboxID,
		EventType:     db.PaymentDomainOutboxEventApplymentTerminalStateReady,
		AggregateType: db.PaymentDomainOutboxAggregateEcommerceApplyment,
		AggregateID:   applymentID,
		Payload:       rawPayload,
		Status:        db.PaymentDomainOutboxStatusProcessing,
	}
}

func buildApplymentPendingOutbox(t *testing.T, outboxID int64, applymentID int64, merchantID int64, applymentStatus string) db.PaymentDomainOutbox {
	t.Helper()
	rawPayload, err := json.Marshal(map[string]any{
		"applyment_id":                applymentID,
		"merchant_id":                 merchantID,
		"out_request_no":              fmt.Sprintf("APPLY_M_%d", applymentID),
		"applyment_status":            applymentStatus,
		"external_payment_fact_id":    7303,
		"payment_fact_application_id": 8303,
	})
	require.NoError(t, err)
	return db.PaymentDomainOutbox{
		ID:            outboxID,
		EventType:     db.PaymentDomainOutboxEventApplymentPendingStateReady,
		AggregateType: db.PaymentDomainOutboxAggregateEcommerceApplyment,
		AggregateID:   applymentID,
		Payload:       rawPayload,
		Status:        db.PaymentDomainOutboxStatusProcessing,
	}
}
