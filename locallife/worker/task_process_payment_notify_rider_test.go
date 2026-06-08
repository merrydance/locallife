package worker

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
	"github.com/merrydance/locallife/websocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type publishedMessageRecord struct {
	channel string
	payload []byte
}

type recordingPublisher struct {
	records []publishedMessageRecord
}

type merchantNotificationRecorder struct {
	NoopTaskDistributor
	payload *SendNotificationPayload
}

func (d *merchantNotificationRecorder) DistributeTaskSendNotification(_ context.Context, payload *SendNotificationPayload, _ ...asynq.Option) error {
	d.payload = payload
	return nil
}

func (p *recordingPublisher) Publish(_ context.Context, channel string, payload []byte) error {
	p.records = append(p.records, publishedMessageRecord{
		channel: channel,
		payload: append([]byte(nil), payload...),
	})
	return nil
}

func notifyMerchantFeeBreakdownOrder() db.Order {
	return db.Order{
		ID:                  501,
		MerchantID:          601,
		OrderNo:             "ORD501",
		OrderType:           "takeout",
		Status:              db.OrderStatusPaid,
		Subtotal:            10000,
		DiscountAmount:      300,
		VoucherAmount:       200,
		DeliveryFee:         800,
		DeliveryFeeDiscount: 0,
		TotalAmount:         10300,
	}
}

func notifyMerchantFeeBreakdownProfitSharingOrder(order db.Order) db.ProfitSharingOrder {
	return db.ProfitSharingOrder{
		ID:                 801,
		PaymentOrderID:     701,
		MerchantID:         order.MerchantID,
		TotalAmount:        order.TotalAmount,
		PlatformCommission: 190,
		OperatorCommission: 285,
		PaymentFee:         31,
		MerchantAmount:     9794,
		RiderGrossAmount:   order.DeliveryFee,
		RiderPaymentFee:    5,
		RiderAmount:        order.DeliveryFee - 5,
	}
}

func TestNotifyRidersNewDelivery_PublishesStructuredPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	publisher := &recordingPublisher{}
	processor := NewTestTaskProcessor(store, nil, nil, nil)
	processor.pubSubPublisher = publisher

	order := db.Order{ID: 101, MerchantID: 201, DeliveryFee: riderHighValueDeliveryFeeThreshold, OrderType: "takeout"}
	merchant := db.Merchant{ID: 201, Name: "测试商户"}
	delivery := &db.Delivery{
		ID:                  301,
		PickupAddress:       "取餐点A",
		DeliveryAddress:     "送达点B",
		EstimatedDeliveryAt: pgtype.Timestamptz{Time: time.Date(2026, 4, 26, 12, 30, 0, 0, time.UTC), Valid: true},
	}
	poolItem := &db.DeliveryPool{
		ID:               401,
		Distance:         1800,
		Priority:         2,
		ExpectedPickupAt: time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC),
		CreatedAt:        time.Date(2026, 4, 26, 11, 50, 0, 0, time.UTC),
	}

	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().ListNearbyRiders(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListNearbyRidersParams) ([]db.ListNearbyRidersRow, error) {
		require.Equal(t, riderDeliverySearchStartDistanceM, arg.MaxDistance)
		return []db.ListNearbyRidersRow{{ID: 11}, {ID: 22}, {ID: 33}}, nil
	})

	processor.notifyRidersNewDelivery(context.Background(), order, delivery, poolItem)

	require.Len(t, publisher.records, 3)
	for index, riderID := range []int64{11, 22, 33} {
		require.Equal(t, fmt.Sprintf("%s%d", riderNotificationChannelPrefix, riderID), publisher.records[index].channel)

		var pushMsg websocket.NotificationPushMessage
		require.NoError(t, json.Unmarshal(publisher.records[index].payload, &pushMsg))
		require.Equal(t, riderNotificationEntityType, pushMsg.EntityType)
		require.Equal(t, riderID, pushMsg.EntityID)
		require.Equal(t, websocket.MessageTypeDeliveryPoolNew, pushMsg.Message.Type)

		var payload riderDeliveryOrderNotificationPayload
		require.NoError(t, json.Unmarshal(pushMsg.Message.Data, &payload))
		require.Equal(t, riderNewDeliveryOrderPayloadType, payload.Type)
		require.Equal(t, order.ID, payload.OrderID)
		require.Equal(t, delivery.ID, payload.DeliveryID)
		require.Equal(t, merchant.ID, payload.MerchantID)
		require.Equal(t, merchant.Name, payload.MerchantName)
		require.Equal(t, delivery.PickupAddress, payload.PickupAddress)
		require.Equal(t, delivery.DeliveryAddress, payload.DeliveryAddress)
		require.Equal(t, order.DeliveryFee, payload.DeliveryFee)
		require.Equal(t, poolItem.Distance, payload.Distance)
		require.Equal(t, poolItem.Priority, payload.Priority)
		require.Equal(t, poolItem.ExpectedPickupAt, payload.ExpectedPickupAt)
		require.Equal(t, delivery.EstimatedDeliveryAt.Time, payload.ExpectedDeliveryAt)
		require.Equal(t, poolItem.CreatedAt, payload.CreatedAt)
		require.True(t, payload.IsHighValue)
	}
}

func TestNotifyMerchantNewOrder_PublishesMerchantAppPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &merchantNotificationRecorder{}
	publisher := &recordingPublisher{}
	processor := NewTestTaskProcessor(store, distributor, nil, nil)
	processor.pubSubPublisher = publisher

	order := notifyMerchantFeeBreakdownOrder()
	merchant := db.Merchant{ID: 601, OwnerUserID: 701, Name: "测试商户"}
	paymentOrder := db.PaymentOrder{
		ID:                    701,
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Amount:                order.TotalAmount,
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	profitSharingOrder := notifyMerchantFeeBreakdownProfitSharingOrder(order)

	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{{
		ID:             801,
		OrderID:        order.ID,
		DishID:         pgtype.Int8{Int64: 901, Valid: true},
		Name:           "测试菜品",
		UnitPrice:      2800,
		Quantity:       2,
		Subtotal:       5600,
		Customizations: []byte(`{"501":601,"502":602,"meta_specs":"大份 / 少辣"}`),
	}}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)

	err := processor.notifyMerchantNewOrder(context.Background(), order)
	require.NoError(t, err)

	require.NotNil(t, distributor.payload)
	require.Equal(t, merchant.OwnerUserID, distributor.payload.UserID)
	require.Equal(t, "🆕 新订单", distributor.payload.Title)
	require.Equal(t, "merchant:new_order:501", distributor.payload.ExtraData["message_id"])
	require.Equal(t, "测试商户", distributor.payload.ExtraData["shop_name"])
	breakdown, ok := distributor.payload.ExtraData["fee_breakdown"].(logic.MerchantOrderFeeBreakdown)
	require.True(t, ok)
	require.Equal(t, int64(10000), breakdown.FoodAmount)
	require.Equal(t, int64(475), breakdown.PlatformServiceFeeAmount)
	require.Equal(t, int64(31), breakdown.PaymentChannelFeeAmount)
	require.Equal(t, int64(800), breakdown.RiderGrossAmount)
	require.Equal(t, int64(5), breakdown.RiderPaymentFeeAmount)
	require.Equal(t, int64(795), breakdown.RiderNetEarningsAmount)

	require.Len(t, publisher.records, 1)
	require.Equal(t, "notification:merchant:601", publisher.records[0].channel)

	var push websocket.NotificationPushMessage
	require.NoError(t, json.Unmarshal(publisher.records[0].payload, &push))
	require.Equal(t, websocket.EntityMerchant, push.EntityType)
	require.Equal(t, merchant.ID, push.EntityID)
	require.Equal(t, "merchant:new_order:501", push.Message.ID)
	require.Equal(t, "new_order", push.Message.Type)

	var data map[string]any
	require.NoError(t, json.Unmarshal(push.Message.Data, &data))
	require.Equal(t, "merchant:new_order:501", data["message_id"])
	require.Equal(t, "new_order", data["event"])
	require.Equal(t, float64(order.ID), data["order_id"])
	require.Equal(t, "新订单", data["title"])
	require.Equal(t, float64(order.TotalAmount), data["amount"])
	require.Equal(t, merchant.Name, data["shop_name"])
	feeBreakdown, ok := data["fee_breakdown"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(10000), feeBreakdown["food_amount"])
	require.Equal(t, float64(475), feeBreakdown["platform_service_fee_amount"])
	require.Equal(t, float64(31), feeBreakdown["payment_channel_fee_amount"])
	require.Equal(t, float64(800), feeBreakdown["rider_gross_amount"])
	require.Equal(t, float64(5), feeBreakdown["rider_payment_fee_amount"])
	require.Equal(t, float64(795), feeBreakdown["rider_net_earnings_amount"])
	require.NotContains(t, feeBreakdown, "provider_payment_fee")
	require.NotContains(t, feeBreakdown, "provider_payment_fee_rate_bps")
	require.NotContains(t, feeBreakdown, "operator_commission")
	require.NotContains(t, feeBreakdown, "platform_commission")
	require.Nil(t, data["items_load_failed"])
	items, ok := data["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "测试菜品", item["name"])
	require.Equal(t, "大份 / 少辣", item["specs_text"])
}

func TestNotifyMerchantNewOrder_PublishesWhenPaidOrderWasCancelled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &merchantNotificationRecorder{}
	publisher := &recordingPublisher{}
	processor := NewTestTaskProcessor(store, distributor, nil, nil)
	processor.pubSubPublisher = publisher

	order := notifyMerchantFeeBreakdownOrder()
	order.Status = db.OrderStatusCancelled
	merchant := db.Merchant{ID: order.MerchantID, OwnerUserID: 701, Name: "测试商户"}
	paymentOrder := db.PaymentOrder{
		ID:                    701,
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Amount:                order.TotalAmount,
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	profitSharingOrder := notifyMerchantFeeBreakdownProfitSharingOrder(order)

	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)

	err := processor.notifyMerchantNewOrder(context.Background(), order)
	require.NoError(t, err)

	require.NotNil(t, distributor.payload)
	require.Equal(t, merchant.OwnerUserID, distributor.payload.UserID)
	breakdown, ok := distributor.payload.ExtraData["fee_breakdown"].(logic.MerchantOrderFeeBreakdown)
	require.True(t, ok)
	require.Equal(t, order.TotalAmount, breakdown.CustomerPayableAmount)
	require.Len(t, publisher.records, 1)
}

func TestNotifyMerchantNewOrder_ReturnsErrorBeforePublishingWhenItemsFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &merchantNotificationRecorder{}
	publisher := &recordingPublisher{}
	processor := NewTestTaskProcessor(store, distributor, nil, nil)
	processor.pubSubPublisher = publisher

	order := db.Order{
		ID:          502,
		MerchantID:  602,
		OrderNo:     "ORD502",
		OrderType:   "takeout",
		Status:      db.OrderStatusPaid,
		Subtotal:    9900,
		TotalAmount: 9900,
	}
	merchant := db.Merchant{ID: 602, OwnerUserID: 702, Name: "测试商户"}
	paymentOrder := db.PaymentOrder{
		ID:                    702,
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Amount:                order.TotalAmount,
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                 802,
		PaymentOrderID:     paymentOrder.ID,
		MerchantID:         order.MerchantID,
		TotalAmount:        order.TotalAmount,
		PlatformCommission: 0,
		OperatorCommission: 0,
		PaymentFee:         0,
		MerchantAmount:     order.TotalAmount,
	}

	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return(nil, errors.New("database unavailable"))

	err := processor.notifyMerchantNewOrder(context.Background(), order)

	require.ErrorContains(t, err, "load order items for merchant new order snapshot")
	require.Nil(t, distributor.payload)
	require.Empty(t, publisher.records)
}

func TestNotifyMerchantNewOrder_ReturnsErrorBeforePublishingWhenFeeBreakdownMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &merchantNotificationRecorder{}
	publisher := &recordingPublisher{}
	processor := NewTestTaskProcessor(store, distributor, nil, nil)
	processor.pubSubPublisher = publisher

	order := notifyMerchantFeeBreakdownOrder()
	order.ID = 504
	order.OrderNo = "ORD504"
	order.MerchantID = 604
	merchant := db.Merchant{ID: 604, OwnerUserID: 704, Name: "测试商户"}
	paymentOrder := db.PaymentOrder{
		ID:                    704,
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Amount:                order.TotalAmount,
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}

	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(db.ProfitSharingOrder{}, db.ErrRecordNotFound)

	err := processor.notifyMerchantNewOrder(context.Background(), order)

	require.ErrorContains(t, err, "build merchant new order fee breakdown")
	require.Nil(t, distributor.payload)
	require.Empty(t, publisher.records)
}

func TestNotifyMerchantNewOrder_SkipsCancelledLegacyOrderWhenFeeBreakdownMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &merchantNotificationRecorder{}
	publisher := &recordingPublisher{}
	processor := NewTestTaskProcessor(store, distributor, nil, nil)
	processor.pubSubPublisher = publisher

	order := notifyMerchantFeeBreakdownOrder()
	order.ID = 505
	order.OrderNo = "ORD505"
	order.MerchantID = 605
	order.Status = db.OrderStatusCancelled
	merchant := db.Merchant{ID: 605, OwnerUserID: 705, Name: "测试商户"}
	paymentOrder := db.PaymentOrder{
		ID:                    705,
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Amount:                order.TotalAmount,
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}

	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(db.ProfitSharingOrder{}, db.ErrRecordNotFound)

	err := processor.notifyMerchantNewOrder(context.Background(), order)

	require.NoError(t, err)
	require.Nil(t, distributor.payload)
	require.Empty(t, publisher.records)
}

func TestNotifyMerchantNewOrder_ReturnsErrorForCancelledOrderWhenPaymentOrderMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &merchantNotificationRecorder{}
	publisher := &recordingPublisher{}
	processor := NewTestTaskProcessor(store, distributor, nil, nil)
	processor.pubSubPublisher = publisher

	order := notifyMerchantFeeBreakdownOrder()
	order.ID = 506
	order.OrderNo = "ORD506"
	order.MerchantID = 606
	order.Status = db.OrderStatusCancelled
	merchant := db.Merchant{ID: 606, OwnerUserID: 706, Name: "测试商户"}

	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)

	err := processor.notifyMerchantNewOrder(context.Background(), order)

	require.ErrorContains(t, err, "get latest payment order by order")
	require.Nil(t, distributor.payload)
	require.Empty(t, publisher.records)
}

func TestNotifyMerchantNewOrder_ReturnsErrorBeforePublishingWhenPaymentOrderUnpaid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &merchantNotificationRecorder{}
	publisher := &recordingPublisher{}
	processor := NewTestTaskProcessor(store, distributor, nil, nil)
	processor.pubSubPublisher = publisher

	order := db.Order{ID: 503, MerchantID: 603, OrderNo: "ORD503", OrderType: "takeout", Status: db.OrderStatusCancelled, TotalAmount: 9900}
	merchant := db.Merchant{ID: 603, OwnerUserID: 703, Name: "测试商户"}
	paymentOrder := db.PaymentOrder{
		ID:                    703,
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Amount:                order.TotalAmount,
		Status:                "pending",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}

	store.EXPECT().GetMerchant(gomock.Any(), order.MerchantID).Return(merchant, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(paymentOrder, nil)

	err := processor.notifyMerchantNewOrder(context.Background(), order)

	require.ErrorContains(t, err, "merchant new order notification requires paid payment order")
	require.Nil(t, distributor.payload)
	require.Empty(t, publisher.records)
}
