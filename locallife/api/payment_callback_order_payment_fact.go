package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/merrydance/locallife/worker"
)

const (
	orderPaymentFactConsumerDomain      = "order_domain"
	orderPaymentFactBusinessObjectOrder = "payment_order"
)

func (server *Server) recordOrderPaymentCallbackFact(ctx context.Context, notification wechat.PaymentNotification, paymentOrder db.PaymentOrder, resource *wechatcontracts.PartnerPaymentNotificationResource) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || resource == nil {
		return nil, nil
	}
	occurredAt := parseWechatFactTime(resource.SuccessTime)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityPartnerJSAPIPayment,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        paymentFactStringPtr(notification.ID),
		SourceEventType:      paymentFactStringPtr(notification.EventType),
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    paymentOrder.OutTradeNo,
		ExternalSecondaryKey: paymentFactStringPtr(resource.TransactionID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerOrder),
		BusinessObjectType:   paymentFactStringPtr(orderPaymentFactBusinessObjectOrder),
		BusinessObjectID:     paymentFactInt64Ptr(paymentOrder.ID),
		UpstreamState:        resource.TradeState,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               paymentFactInt64Ptr(resource.Amount.Total),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          orderPaymentCallbackFactResource(paymentOrder, resource),
		DedupeKey:            fmt.Sprintf("wechat:callback:ecommerce:order_payment:%s", notification.ID),
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

func (server *Server) recordCombinedOrderPaymentCallbackFact(ctx context.Context, notification *wechat.PaymentNotification, combined db.CombinedPaymentOrder, paymentOrder db.PaymentOrder, subOrder wechatcontracts.CombinePaymentNotificationSubOrder) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil {
		return nil, nil
	}
	if notification == nil {
		return nil, fmt.Errorf("notification is required")
	}
	occurredAt := parseWechatFactTime(subOrder.SuccessTime)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityCombinePayment,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        paymentFactStringPtr(notification.ID),
		SourceEventType:      paymentFactStringPtr(notification.EventType),
		ExternalObjectType:   db.ExternalPaymentObjectCombinedPayment,
		ExternalObjectKey:    combined.CombineOutTradeNo,
		ExternalSecondaryKey: paymentFactStringPtr(subOrder.OutTradeNo),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerOrder),
		BusinessObjectType:   paymentFactStringPtr(orderPaymentFactBusinessObjectOrder),
		BusinessObjectID:     paymentFactInt64Ptr(paymentOrder.ID),
		UpstreamState:        subOrder.TradeState,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               paymentFactInt64Ptr(subOrder.Amount.TotalAmount),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          combinedOrderPaymentCallbackFactResource(combined, paymentOrder, subOrder),
		DedupeKey:            fmt.Sprintf("wechat:callback:ecommerce:combine_order_payment:%s:%s", notification.ID, subOrder.OutTradeNo),
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

func (server *Server) enqueueOrderPaymentFactApplication(ctx context.Context, application *db.ExternalPaymentFactApplication) error {
	if application == nil {
		return nil
	}
	if server.taskDistributor == nil {
		return fmt.Errorf("task distributor not configured")
	}
	applicationDistributor, ok := server.taskDistributor.(worker.PaymentFactApplicationTaskDistributor)
	if !ok {
		return fmt.Errorf("payment fact application distributor not configured")
	}
	return applicationDistributor.DistributeTaskProcessPaymentFactApplication(
		ctx,
		&worker.PaymentFactApplicationPayload{ApplicationID: application.ID},
		asynq.MaxRetry(5),
		asynq.Queue(worker.QueueCritical),
		asynq.Unique(30*time.Second),
	)
}

func orderPaymentCallbackFactResource(paymentOrder db.PaymentOrder, resource *wechatcontracts.PartnerPaymentNotificationResource) []byte {
	raw, err := json.Marshal(map[string]any{
		"payment_order_id": paymentOrder.ID,
		"out_trade_no":     paymentOrder.OutTradeNo,
		"transaction_id":   resource.TransactionID,
		"trade_state":      resource.TradeState,
		"trade_state_desc": resource.TradeStateDesc,
		"success_time":     resource.SuccessTime,
		"amount":           resource.Amount.Total,
		"sp_mchid":         resource.SpMchID,
		"sub_mchid":        resource.SubMchID,
	})
	if err != nil {
		return nil
	}
	return raw
}

func combinedOrderPaymentCallbackFactResource(combined db.CombinedPaymentOrder, paymentOrder db.PaymentOrder, subOrder wechatcontracts.CombinePaymentNotificationSubOrder) []byte {
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
