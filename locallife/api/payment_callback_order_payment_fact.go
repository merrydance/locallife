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
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/merrydance/locallife/worker"
)

const (
	orderPaymentFactConsumerDomain      = "order_domain"
	orderPaymentFactBusinessObjectOrder = "payment_order"
)

func paymentOrderUsesMainBusinessPaymentChannel(paymentOrder db.PaymentOrder) bool {
	return db.PaymentOrderUsesEcommerceChannel(paymentOrder) || db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder)
}

func paymentCallbackFactDedupeChannel(paymentOrder db.PaymentOrder) string {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		return db.PaymentChannelOrdinaryServiceProvider
	}
	return db.PaymentChannelEcommerce
}

func (server *Server) recordOrderPaymentCallbackFact(ctx context.Context, notification wechat.PaymentNotification, paymentOrder db.PaymentOrder, resource *wechatcontracts.PartnerPaymentNotificationResource) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || resource == nil {
		return nil, nil
	}
	occurredAt := parseWechatFactTime(resource.SuccessTime)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              paymentCallbackFactDedupeChannel(paymentOrder),
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
		DedupeKey:            fmt.Sprintf("wechat:callback:%s:order_payment:%s", paymentCallbackFactDedupeChannel(paymentOrder), notification.ID),
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

func (server *Server) recordOrderPaymentCallbackFactByChannel(ctx context.Context, notification wechat.PaymentNotification, paymentOrder db.PaymentOrder, resource *wechatcontracts.PartnerPaymentNotificationResource) (*db.ExternalPaymentFactApplication, error) {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		return server.recordOrdinaryOrderPaymentCallbackFact(ctx, notification, paymentOrder, ordinaryPaymentNotificationFromPartnerResource(resource))
	}
	return server.recordOrderPaymentCallbackFact(ctx, notification, paymentOrder, resource)
}

func (server *Server) recordOrdinaryOrderPaymentCallbackFact(ctx context.Context, notification wechat.PaymentNotification, paymentOrder db.PaymentOrder, resource *ospcontracts.PaymentNotificationPayload) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || resource == nil {
		return nil, nil
	}
	occurredAt := parseWechatFactTime(resource.SuccessTime)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelOrdinaryServiceProvider,
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
		UpstreamState:        string(resource.TradeState),
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               paymentFactInt64Ptr(ordinaryPaymentNotificationAmountTotal(resource)),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          ordinaryOrderPaymentCallbackFactResource(paymentOrder, resource),
		DedupeKey:            fmt.Sprintf("wechat:callback:%s:order_payment:%s", db.PaymentChannelOrdinaryServiceProvider, notification.ID),
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
		Channel:              paymentCallbackFactDedupeChannel(paymentOrder),
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
		DedupeKey:            fmt.Sprintf("wechat:callback:%s:combine_order_payment:%s:%s", paymentCallbackFactDedupeChannel(paymentOrder), notification.ID, subOrder.OutTradeNo),
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

func (server *Server) recordCombinedOrderPaymentCallbackFactByChannel(ctx context.Context, notification *wechat.PaymentNotification, combined db.CombinedPaymentOrder, paymentOrder db.PaymentOrder, subOrder wechatcontracts.CombinePaymentNotificationSubOrder) (*db.ExternalPaymentFactApplication, error) {
	if db.PaymentOrderUsesOrdinaryServiceProviderChannel(paymentOrder) {
		return server.recordOrdinaryCombinedOrderPaymentCallbackFact(ctx, notification, combined, paymentOrder, ordinaryCombineOrderStateFromLegacy(subOrder))
	}
	return server.recordCombinedOrderPaymentCallbackFact(ctx, notification, combined, paymentOrder, subOrder)
}

func (server *Server) recordOrdinaryCombinedOrderPaymentCallbackFact(ctx context.Context, notification *wechat.PaymentNotification, combined db.CombinedPaymentOrder, paymentOrder db.PaymentOrder, subOrder ospcontracts.CombineOrderState) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil {
		return nil, nil
	}
	if notification == nil {
		return nil, fmt.Errorf("notification is required")
	}
	occurredAt := parseWechatFactTime(subOrder.SuccessTime)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelOrdinaryServiceProvider,
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
		UpstreamState:        string(subOrder.TradeState),
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               paymentFactInt64Ptr(subOrder.Amount.TotalAmount),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          ordinaryCombinedOrderPaymentCallbackFactResource(combined, paymentOrder, subOrder),
		DedupeKey:            fmt.Sprintf("wechat:callback:%s:combine_order_payment:%s:%s", db.PaymentChannelOrdinaryServiceProvider, notification.ID, subOrder.OutTradeNo),
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

func ordinaryPaymentNotificationAmountTotal(resource *ospcontracts.PaymentNotificationPayload) int64 {
	if resource == nil || resource.Amount == nil {
		return 0
	}
	return resource.Amount.Total
}

func ordinaryOrderPaymentCallbackFactResource(paymentOrder db.PaymentOrder, resource *ospcontracts.PaymentNotificationPayload) []byte {
	raw, err := json.Marshal(map[string]any{
		"payment_order_id": paymentOrder.ID,
		"out_trade_no":     paymentOrder.OutTradeNo,
		"transaction_id":   resource.TransactionID,
		"trade_state":      resource.TradeState,
		"trade_state_desc": resource.TradeStateDesc,
		"success_time":     resource.SuccessTime,
		"amount":           ordinaryPaymentNotificationAmountTotal(resource),
		"sp_mchid":         resource.SpMchID,
		"sub_mchid":        resource.SubMchID,
	})
	if err != nil {
		return nil
	}
	return raw
}

func ordinaryCombinedOrderPaymentCallbackFactResource(combined db.CombinedPaymentOrder, paymentOrder db.PaymentOrder, subOrder ospcontracts.CombineOrderState) []byte {
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

func ordinaryPaymentNotificationFromPartnerResource(resource *wechatcontracts.PartnerPaymentNotificationResource) *ospcontracts.PaymentNotificationPayload {
	if resource == nil {
		return nil
	}
	return &ospcontracts.PaymentNotificationPayload{
		SpAppID:        resource.SpAppID,
		SpMchID:        resource.SpMchID,
		SubAppID:       resource.SubAppID,
		SubMchID:       resource.SubMchID,
		OutTradeNo:     resource.OutTradeNo,
		TransactionID:  resource.TransactionID,
		TradeType:      resource.TradeType,
		TradeState:     ospcontracts.PaymentTradeState(resource.TradeState),
		TradeStateDesc: resource.TradeStateDesc,
		BankType:       resource.BankType,
		Attach:         resource.Attach,
		SuccessTime:    resource.SuccessTime,
		Payer: ospcontracts.PaymentPayer{
			SpOpenID:  resource.Payer.SpOpenID,
			SubOpenID: resource.Payer.SubOpenID,
		},
		Amount: &ospcontracts.PaymentAmount{
			Total:         resource.Amount.Total,
			PayerTotal:    resource.Amount.PayerTotal,
			Currency:      ospcontracts.Currency(resource.Amount.Currency),
			PayerCurrency: ospcontracts.Currency(resource.Amount.PayerCurrency),
		},
	}
}

func ordinaryCombineOrderStateFromLegacy(subOrder wechatcontracts.CombinePaymentNotificationSubOrder) ospcontracts.CombineOrderState {
	return ospcontracts.CombineOrderState{
		MchID:         subOrder.MchID,
		SubMchID:      subOrder.SubMchID,
		SubAppID:      subOrder.SubAppID,
		OutTradeNo:    subOrder.OutTradeNo,
		TransactionID: subOrder.TransactionID,
		TradeType:     subOrder.TradeType,
		TradeState:    ospcontracts.PaymentTradeState(subOrder.TradeState),
		BankType:      subOrder.BankType,
		Attach:        subOrder.Attach,
		SuccessTime:   subOrder.SuccessTime,
		Amount: ospcontracts.CombineAmount{
			TotalAmount:    subOrder.Amount.TotalAmount,
			PayerAmount:    subOrder.Amount.PayerAmount,
			Currency:       ospcontracts.Currency(subOrder.Amount.Currency),
			PayerCurrency:  ospcontracts.Currency(subOrder.Amount.PayerCurrency),
			SettlementRate: subOrder.Amount.SettlementRate,
		},
	}
}
