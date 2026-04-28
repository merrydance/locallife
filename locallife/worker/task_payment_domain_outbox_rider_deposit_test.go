package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type orderRefundOutboxTestDistributor struct {
	NoopTaskDistributor
	payloads []*SendNotificationPayload
}

func (d *orderRefundOutboxTestDistributor) DistributeTaskSendNotification(_ context.Context, payload *SendNotificationPayload, _ ...asynq.Option) error {
	d.payloads = append(d.payloads, payload)
	return nil
}

func TestProcessTaskPaymentDomainOutbox_PublishesRiderDepositRefundAbnormalAlert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rawPayload, err := json.Marshal(map[string]any{
		"refund_order_id":             44,
		"payment_order_id":            11,
		"out_refund_no":               "RF123",
		"refund_status":               "ABNORMAL",
		"refund_id":                   "WR123",
		"external_payment_fact_id":    704,
		"payment_fact_application_id": 804,
	})
	require.NoError(t, err)

	outbox := db.PaymentDomainOutbox{
		ID:            909,
		EventType:     db.PaymentDomainOutboxEventRiderDepositRefundAbnormal,
		AggregateType: db.PaymentDomainOutboxAggregateRefundOrder,
		AggregateID:   44,
		Payload:       rawPayload,
		Status:        db.PaymentDomainOutboxStatusProcessing,
	}
	refundOrder := db.RefundOrder{
		ID:             44,
		PaymentOrderID: 11,
		OutRefundNo:    "RF123",
		RefundAmount:   1200,
		RefundType:     "rider_deposit",
		RefundID:       pgtype.Text{String: "WR123", Valid: true},
	}
	paymentOrder := db.PaymentOrder{
		ID:           11,
		UserID:       33,
		PaymentType:  "miniprogram",
		BusinessType: db.ExternalPaymentBusinessOwnerRiderDeposit,
		Amount:       1200,
		OutTradeNo:   "OT123",
	}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := NewTestTaskProcessor(store, nil, nil, nil)
	publisher := &testPublisher{}
	processor.pubSubPublisher = publisher
	jsonPayload, err := json.Marshal(PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(TaskProcessPaymentDomainOutbox, jsonPayload))
	require.NoError(t, err)
	require.Equal(t, AlertChannel, publisher.channel)

	var published map[string]any
	require.NoError(t, json.Unmarshal(publisher.payload, &published))
	data := published["data"].(map[string]any)
	require.Equal(t, "退款异常 - 需人工介入", data["title"])
	require.Equal(t, "refund_order", data["related_type"])
	require.Equal(t, float64(44), data["related_id"])
	extra := data["extra"].(map[string]any)
	require.Equal(t, float64(44), extra["refund_order_id"])
	require.Equal(t, float64(11), extra["payment_order_id"])
	require.Equal(t, "WR123", extra["refund_id"])
	require.Equal(t, float64(704), extra["external_payment_fact_id"])
	require.Equal(t, float64(804), extra["payment_fact_application_id"])
}

func TestProcessTaskPaymentDomainOutbox_PublishesReservationRefundAbnormalAlert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rawPayload, err := json.Marshal(map[string]any{
		"refund_order_id":             144,
		"payment_order_id":            111,
		"reservation_id":              222,
		"out_refund_no":               "RF_RES_144",
		"refund_status":               "ABNORMAL",
		"refund_id":                   "WR_RES_144",
		"external_payment_fact_id":    1704,
		"payment_fact_application_id": 1804,
	})
	require.NoError(t, err)

	outbox := db.PaymentDomainOutbox{
		ID:            919,
		EventType:     db.PaymentDomainOutboxEventReservationRefundAbnormal,
		AggregateType: db.PaymentDomainOutboxAggregateRefundOrder,
		AggregateID:   144,
		Payload:       rawPayload,
		Status:        db.PaymentDomainOutboxStatusProcessing,
	}
	refundOrder := db.RefundOrder{
		ID:             144,
		PaymentOrderID: 111,
		OutRefundNo:    "RF_RES_144",
		RefundAmount:   1200,
		RefundType:     "reservation_cancel",
		RefundID:       pgtype.Text{String: "WR_RES_144", Valid: true},
	}
	paymentOrder := db.PaymentOrder{
		ID:             111,
		ReservationID:  pgtype.Int8{Int64: 222, Valid: true},
		PaymentType:    "profit_sharing",
		PaymentChannel: db.PaymentChannelEcommerce,
		BusinessType:   "reservation_addon",
		Amount:         1200,
		OutTradeNo:     "OT_RES_111",
	}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetTableReservation(gomock.Any(), int64(222)).Return(db.TableReservation{ID: 222, MerchantID: 333}, nil)
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := NewTestTaskProcessor(store, nil, nil, nil)
	publisher := &testPublisher{}
	processor.pubSubPublisher = publisher
	jsonPayload, err := json.Marshal(PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(TaskProcessPaymentDomainOutbox, jsonPayload))
	require.NoError(t, err)
	require.Equal(t, AlertChannel, publisher.channel)

	var published map[string]any
	require.NoError(t, json.Unmarshal(publisher.payload, &published))
	data := published["data"].(map[string]any)
	require.Equal(t, "预订退款异常 - 需人工介入", data["title"])
	require.Equal(t, "refund_order", data["related_type"])
	require.Equal(t, float64(144), data["related_id"])
	extra := data["extra"].(map[string]any)
	require.Equal(t, float64(144), extra["refund_order_id"])
	require.Equal(t, float64(111), extra["payment_order_id"])
	require.Equal(t, float64(222), extra["reservation_id"])
	require.Equal(t, float64(333), extra["merchant_id"])
	require.Equal(t, float64(1704), extra["external_payment_fact_id"])
	require.Equal(t, float64(1804), extra["payment_fact_application_id"])
	require.Equal(t, true, extra["abnormal_refund_api_available"])
}

func TestProcessTaskPaymentDomainOutbox_DispatchesOrderRefundSucceededNotification(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &orderRefundOutboxTestDistributor{}
	rawPayload, err := json.Marshal(map[string]any{
		"refund_order_id":             244,
		"payment_order_id":            211,
		"order_id":                    222,
		"user_id":                     333,
		"out_refund_no":               "RF_ORDER_244",
		"refund_amount":               1200,
		"refund_status":               "SUCCESS",
		"refund_id":                   "WR_ORDER_244",
		"external_payment_fact_id":    2704,
		"payment_fact_application_id": 2804,
	})
	require.NoError(t, err)

	outbox := db.PaymentDomainOutbox{ID: 929, EventType: db.PaymentDomainOutboxEventOrderRefundSucceeded, AggregateType: db.PaymentDomainOutboxAggregateRefundOrder, AggregateID: 244, Payload: rawPayload, Status: db.PaymentDomainOutboxStatusProcessing}
	refundOrder := db.RefundOrder{ID: 244, PaymentOrderID: 211, OutRefundNo: "RF_ORDER_244", RefundAmount: 1200, RefundID: pgtype.Text{String: "WR_ORDER_244", Valid: true}}
	paymentOrder := db.PaymentOrder{ID: 211, OrderID: pgtype.Int8{Int64: 222, Valid: true}, UserID: 333, PaymentChannel: db.PaymentChannelEcommerce, BusinessType: db.ExternalPaymentBusinessOwnerOrder}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
	require.Len(t, distributor.payloads, 1)
	require.Equal(t, int64(333), distributor.payloads[0].UserID)
	require.Equal(t, "refund", distributor.payloads[0].Type)
	require.Equal(t, "退款成功", distributor.payloads[0].Title)
	require.Equal(t, refundOrder.ID, distributor.payloads[0].RelatedID)
	require.Equal(t, refundOrder.OutRefundNo, distributor.payloads[0].ExtraData["out_refund_no"])
	require.Equal(t, "WR_ORDER_244", distributor.payloads[0].ExtraData["refund_id"])
	require.Equal(t, int64(1200), distributor.payloads[0].ExtraData["amount"])
}

func TestProcessTaskPaymentDomainOutbox_OrderRefundSucceededWithoutDistributorMarksFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rawPayload, err := json.Marshal(map[string]any{
		"refund_order_id":             245,
		"payment_order_id":            212,
		"order_id":                    223,
		"user_id":                     334,
		"out_refund_no":               "RF_ORDER_245",
		"refund_amount":               1200,
		"refund_status":               "SUCCESS",
		"refund_id":                   "WR_ORDER_245",
		"external_payment_fact_id":    2705,
		"payment_fact_application_id": 2805,
	})
	require.NoError(t, err)

	outbox := db.PaymentDomainOutbox{ID: 930, EventType: db.PaymentDomainOutboxEventOrderRefundSucceeded, AggregateType: db.PaymentDomainOutboxAggregateRefundOrder, AggregateID: 245, Payload: rawPayload, Status: db.PaymentDomainOutboxStatusProcessing}
	refundOrder := db.RefundOrder{ID: 245, PaymentOrderID: 212, OutRefundNo: "RF_ORDER_245", RefundAmount: 1200, RefundID: pgtype.Text{String: "WR_ORDER_245", Valid: true}}
	paymentOrder := db.PaymentOrder{ID: 212, OrderID: pgtype.Int8{Int64: 223, Valid: true}, UserID: 334, PaymentChannel: db.PaymentChannelEcommerce, BusinessType: db.ExternalPaymentBusinessOwnerOrder}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().MarkPaymentDomainOutboxFailed(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.MarkPaymentDomainOutboxFailedParams) (db.PaymentDomainOutbox, error) {
		require.Equal(t, outbox.ID, arg.ID)
		require.True(t, arg.LastError.Valid)
		require.Contains(t, arg.LastError.String, "task distributor not configured")
		require.True(t, arg.NextRetryAt.Valid)
		return db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusFailed}, nil
	})

	processor := NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentDomainOutbox_PublishesOrderRefundAbnormalAlert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	rawPayload, err := json.Marshal(map[string]any{
		"refund_order_id":             344,
		"payment_order_id":            311,
		"order_id":                    322,
		"out_refund_no":               "RF_ORDER_344",
		"refund_status":               "ABNORMAL",
		"refund_id":                   "WR_ORDER_344",
		"external_payment_fact_id":    3704,
		"payment_fact_application_id": 3804,
	})
	require.NoError(t, err)

	outbox := db.PaymentDomainOutbox{ID: 939, EventType: db.PaymentDomainOutboxEventOrderRefundAbnormal, AggregateType: db.PaymentDomainOutboxAggregateRefundOrder, AggregateID: 344, Payload: rawPayload, Status: db.PaymentDomainOutboxStatusProcessing}
	refundOrder := db.RefundOrder{ID: 344, PaymentOrderID: 311, OutRefundNo: "RF_ORDER_344", RefundAmount: 1300, RefundType: "user_cancel", RefundID: pgtype.Text{String: "WR_ORDER_344", Valid: true}}
	paymentOrder := db.PaymentOrder{ID: 311, OrderID: pgtype.Int8{Int64: 322, Valid: true}, PaymentChannel: db.PaymentChannelEcommerce, BusinessType: db.ExternalPaymentBusinessOwnerOrder, OutTradeNo: "OT_311"}

	store.EXPECT().ClaimPaymentDomainOutbox(gomock.Any(), gomock.Any()).Return(outbox, nil)
	store.EXPECT().GetRefundOrder(gomock.Any(), refundOrder.ID).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(322)).Return(db.Order{ID: 322, MerchantID: 355}, nil)
	store.EXPECT().MarkPaymentDomainOutboxPublished(gomock.Any(), outbox.ID).Return(db.PaymentDomainOutbox{ID: outbox.ID, Status: db.PaymentDomainOutboxStatusPublished}, nil)

	processor := NewTestTaskProcessor(store, nil, nil, nil)
	publisher := &testPublisher{}
	processor.pubSubPublisher = publisher
	payload, err := json.Marshal(PaymentDomainOutboxPayload{OutboxID: outbox.ID})
	require.NoError(t, err)
	err = processor.ProcessTaskPaymentDomainOutbox(context.Background(), asynq.NewTask(TaskProcessPaymentDomainOutbox, payload))
	require.NoError(t, err)

	var published map[string]any
	require.NoError(t, json.Unmarshal(publisher.payload, &published))
	data := published["data"].(map[string]any)
	require.Equal(t, "订单退款异常 - 需人工介入", data["title"])
	extra := data["extra"].(map[string]any)
	require.Equal(t, float64(344), extra["refund_order_id"])
	require.Equal(t, float64(311), extra["payment_order_id"])
	require.Equal(t, float64(322), extra["order_id"])
	require.Equal(t, float64(355), extra["merchant_id"])
	require.Equal(t, float64(3704), extra["external_payment_fact_id"])
	require.Equal(t, float64(3804), extra["payment_fact_application_id"])
	require.Equal(t, true, extra["abnormal_refund_api_available"])
}
