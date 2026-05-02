package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
)

const (
	orderPaymentFactBusinessObjectOrder = "payment_order"
	orderPaymentFactConsumerDomain      = "order_domain"
)

func enqueueOrderPaymentFactApplication(ctx context.Context, distributor any, application *db.ExternalPaymentFactApplication) error {
	if application == nil {
		return nil
	}
	applicationDistributor, ok := distributor.(PaymentFactApplicationTaskDistributor)
	if !ok {
		return fmt.Errorf("payment fact application distributor not configured")
	}
	return applicationDistributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(QueueCritical),
		asynq.Unique(paymentFactApplicationTaskUnique),
	)
}

func recordCombinedOrderPaymentQueryFact(ctx context.Context, store db.Store, combined db.CombinedPaymentOrder, paymentOrder db.PaymentOrder, subOrder wechatcontracts.CombineSubOrderResult) (*db.ExternalPaymentFactApplication, error) {
	service := logic.NewPaymentFactService(store)
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              paymentFactChannel(paymentOrder.PaymentChannel, db.PaymentChannelEcommerce),
		Capability:           db.ExternalPaymentCapabilityCombinePayment,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectCombinedPayment,
		ExternalObjectKey:    combined.CombineOutTradeNo,
		ExternalSecondaryKey: orderPaymentStringPtr(subOrder.OutTradeNo),
		BusinessOwner:        orderPaymentStringPtr(db.ExternalPaymentBusinessOwnerOrder),
		BusinessObjectType:   orderPaymentStringPtr(orderPaymentFactBusinessObjectOrder),
		BusinessObjectID:     orderPaymentInt64Ptr(paymentOrder.ID),
		UpstreamState:        subOrder.TradeState,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               orderPaymentInt64Ptr(subOrder.Amount.TotalAmount),
		Currency:             "CNY",
		OccurredAt:           orderPaymentParseFactTime(subOrder.SuccessTime),
		UpstreamUpdatedAt:    orderPaymentParseFactTime(subOrder.SuccessTime),
		RawResource:          combinedOrderPaymentQueryFactResource(combined, paymentOrder, subOrder),
		DedupeKey:            fmt.Sprintf("wechat:query:%s:combine_order_payment:%s:%s:success", paymentFactChannel(paymentOrder.PaymentChannel, db.PaymentChannelEcommerce), combined.CombineOutTradeNo, subOrder.OutTradeNo),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           orderPaymentFactConsumerDomain,
			BusinessObjectType: orderPaymentFactBusinessObjectOrder,
			BusinessObjectID:   paymentOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func recordOrderPaymentQueryFact(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, queryResp *wechatcontracts.PartnerOrderQueryResponse) (*db.ExternalPaymentFactApplication, error) {
	objectKey := paymentOrder.OutTradeNo
	if queryResp.OutTradeNo != "" {
		objectKey = queryResp.OutTradeNo
	}
	occurredAt := orderPaymentParseFactTime(queryResp.SuccessTime)
	service := logic.NewPaymentFactService(store)
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              paymentFactChannel(paymentOrder.PaymentChannel, db.PaymentChannelEcommerce),
		Capability:           db.ExternalPaymentCapabilityPartnerJSAPIPayment,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    objectKey,
		ExternalSecondaryKey: orderPaymentStringPtr(queryResp.TransactionID),
		BusinessOwner:        orderPaymentStringPtr(db.ExternalPaymentBusinessOwnerOrder),
		BusinessObjectType:   orderPaymentStringPtr(orderPaymentFactBusinessObjectOrder),
		BusinessObjectID:     orderPaymentInt64Ptr(paymentOrder.ID),
		UpstreamState:        queryResp.TradeState,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               orderPaymentInt64Ptr(queryResp.Amount.Total),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          orderPaymentQueryFactResource(paymentOrder, queryResp),
		DedupeKey:            fmt.Sprintf("wechat:query:%s:partner_order_payment:%s:success", paymentFactChannel(paymentOrder.PaymentChannel, db.PaymentChannelEcommerce), objectKey),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           orderPaymentFactConsumerDomain,
			BusinessObjectType: orderPaymentFactBusinessObjectOrder,
			BusinessObjectID:   paymentOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func recordOrdinaryServiceProviderPaymentTimeoutQueryFact(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, queryResp *ospcontracts.PaymentQueryResponse) (*db.ExternalPaymentFactApplication, error) {
	if paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerOrder {
		return nil, fmt.Errorf("unsupported ordinary service provider payment timeout fact owner %q for payment order %d", paymentOrder.BusinessType, paymentOrder.ID)
	}
	if queryResp == nil || queryResp.Amount == nil {
		return nil, fmt.Errorf("ordinary service provider payment timeout query response missing amount for payment order %d", paymentOrder.ID)
	}
	objectKey := paymentOrder.OutTradeNo
	if queryResp.OutTradeNo != "" {
		objectKey = queryResp.OutTradeNo
	}
	service := logic.NewPaymentFactService(store)
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelOrdinaryServiceProvider,
		Capability:           db.ExternalPaymentCapabilityPartnerJSAPIPayment,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    objectKey,
		ExternalSecondaryKey: orderPaymentStringPtr(queryResp.TransactionID),
		BusinessOwner:        orderPaymentStringPtr(db.ExternalPaymentBusinessOwnerOrder),
		BusinessObjectType:   orderPaymentStringPtr(orderPaymentFactBusinessObjectOrder),
		BusinessObjectID:     orderPaymentInt64Ptr(paymentOrder.ID),
		UpstreamState:        string(queryResp.TradeState),
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               orderPaymentInt64Ptr(queryResp.Amount.Total),
		Currency:             "CNY",
		RawResource:          ordinaryServiceProviderPaymentQueryFactResource(paymentOrder, queryResp),
		DedupeKey:            fmt.Sprintf("wechat:query:%s:partner_order_payment:%s:success", db.PaymentChannelOrdinaryServiceProvider, objectKey),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           orderPaymentFactConsumerDomain,
			BusinessObjectType: orderPaymentFactBusinessObjectOrder,
			BusinessObjectID:   paymentOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func combinedOrderPaymentQueryFactResource(combined db.CombinedPaymentOrder, paymentOrder db.PaymentOrder, subOrder wechatcontracts.CombineSubOrderResult) []byte {
	raw, err := json.Marshal(map[string]any{
		"combined_payment_id":  combined.ID,
		"combine_out_trade_no": combined.CombineOutTradeNo,
		"payment_order_id":     paymentOrder.ID,
		"out_trade_no":         paymentOrder.OutTradeNo,
		"transaction_id":       subOrder.TransactionID,
		"trade_state":          subOrder.TradeState,
		"success_time":         subOrder.SuccessTime,
		"amount":               subOrder.Amount.TotalAmount,
	})
	if err != nil {
		return nil
	}
	return raw
}

func orderPaymentQueryFactResource(paymentOrder db.PaymentOrder, queryResp *wechatcontracts.PartnerOrderQueryResponse) []byte {
	raw, err := json.Marshal(map[string]any{
		"payment_order_id": paymentOrder.ID,
		"out_trade_no":     paymentOrder.OutTradeNo,
		"transaction_id":   queryResp.TransactionID,
		"trade_state":      queryResp.TradeState,
		"success_time":     queryResp.SuccessTime,
		"amount":           queryResp.Amount.Total,
	})
	if err != nil {
		return nil
	}
	return raw
}

func ordinaryServiceProviderPaymentQueryFactResource(paymentOrder db.PaymentOrder, queryResp *ospcontracts.PaymentQueryResponse) []byte {
	amount := int64(0)
	if queryResp.Amount != nil {
		amount = queryResp.Amount.Total
	}
	raw, err := json.Marshal(map[string]any{
		"payment_order_id": paymentOrder.ID,
		"out_trade_no":     paymentOrder.OutTradeNo,
		"transaction_id":   queryResp.TransactionID,
		"trade_state":      queryResp.TradeState,
		"amount":           amount,
		"payment_channel":  db.PaymentChannelOrdinaryServiceProvider,
	})
	if err != nil {
		return nil
	}
	return raw
}

func paymentFactChannel(channel string, fallback string) string {
	if strings.TrimSpace(channel) != "" {
		return channel
	}
	return fallback
}

func recoveredOrderPaymentFactResource(order db.PaymentOrder) []byte {
	raw, err := json.Marshal(map[string]any{
		"payment_order_id":    order.ID,
		"out_trade_no":        order.OutTradeNo,
		"transaction_id":      orderPaymentTextValue(order.TransactionID),
		"business_type":       order.BusinessType,
		"payment_channel":     order.PaymentChannel,
		"combined_payment_id": orderPaymentInt8Value(order.CombinedPaymentID),
		"recovery_reason":     "paid_unprocessed_scan",
	})
	if err != nil {
		return nil
	}
	return raw
}

func recoveredOrderPaymentFactDedupeKey(order db.PaymentOrder, capability string) string {
	return fmt.Sprintf("wechat:manual_reconciliation:ecommerce:%s:%s:success", capability, order.OutTradeNo)
}

func orderPaymentStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func orderPaymentOptionalStringPtr(value pgtype.Text) *string {
	if !value.Valid || value.String == "" {
		return nil
	}
	return &value.String
}

func orderPaymentInt64Ptr(value int64) *int64 {
	return &value
}

func orderPaymentParseFactTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}

func orderPaymentTextValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func orderPaymentInt8Value(value pgtype.Int8) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}
