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
	reservationPaymentFactConsumerDomain      = "reservation_domain"
	reservationPaymentFactBusinessObjectOrder = "payment_order"
	reservationAddonBusinessType              = "reservation_addon"
)

func shouldRecordReservationPaymentFact(paymentOrder db.PaymentOrder) bool {
	return paymentOrder.PaymentChannel == db.PaymentChannelEcommerce &&
		paymentOrder.ReservationID.Valid &&
		(paymentOrder.BusinessType == db.ExternalPaymentBusinessOwnerReservation || paymentOrder.BusinessType == reservationAddonBusinessType)
}

func (server *Server) recordReservationPaymentCallbackFact(ctx context.Context, notification wechat.PaymentNotification, paymentOrder db.PaymentOrder, resource *wechatcontracts.PartnerPaymentNotificationResource) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || resource == nil || !shouldRecordReservationPaymentFact(paymentOrder) {
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
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerReservation),
		BusinessObjectType:   paymentFactStringPtr(reservationPaymentFactBusinessObjectOrder),
		BusinessObjectID:     paymentFactInt64Ptr(paymentOrder.ID),
		UpstreamState:        resource.TradeState,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               paymentFactInt64Ptr(resource.Amount.Total),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          reservationPaymentCallbackFactResource(paymentOrder, resource),
		DedupeKey:            fmt.Sprintf("wechat:callback:ecommerce:reservation_payment:%s", notification.ID),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           reservationPaymentFactConsumerDomain,
			BusinessObjectType: reservationPaymentFactBusinessObjectOrder,
			BusinessObjectID:   paymentOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func (server *Server) recordCombinedReservationPaymentCallbackFact(ctx context.Context, notification *wechat.PaymentNotification, combined db.CombinedPaymentOrder, paymentOrder db.PaymentOrder, subOrder wechatcontracts.CombinePaymentNotificationSubOrder) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || !shouldRecordReservationPaymentFact(paymentOrder) {
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
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerReservation),
		BusinessObjectType:   paymentFactStringPtr(reservationPaymentFactBusinessObjectOrder),
		BusinessObjectID:     paymentFactInt64Ptr(paymentOrder.ID),
		UpstreamState:        subOrder.TradeState,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               paymentFactInt64Ptr(subOrder.Amount.TotalAmount),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          combinedReservationPaymentCallbackFactResource(combined, paymentOrder, subOrder),
		DedupeKey:            fmt.Sprintf("wechat:callback:ecommerce:combine_reservation_payment:%s:%s", notification.ID, subOrder.OutTradeNo),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           reservationPaymentFactConsumerDomain,
			BusinessObjectType: reservationPaymentFactBusinessObjectOrder,
			BusinessObjectID:   paymentOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func (server *Server) enqueueReservationPaymentFactApplication(ctx context.Context, application *db.ExternalPaymentFactApplication) error {
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

func reservationPaymentCallbackFactResource(paymentOrder db.PaymentOrder, resource *wechatcontracts.PartnerPaymentNotificationResource) []byte {
	payload := map[string]any{
		"payment_order_id": paymentOrder.ID,
		"business_type":    paymentOrder.BusinessType,
		"out_trade_no":     paymentOrder.OutTradeNo,
		"transaction_id":   resource.TransactionID,
		"trade_state":      resource.TradeState,
		"trade_state_desc": resource.TradeStateDesc,
		"success_time":     resource.SuccessTime,
		"amount":           resource.Amount.Total,
		"sp_mchid":         resource.SpMchID,
		"sub_mchid":        resource.SubMchID,
	}
	if paymentOrder.ReservationID.Valid {
		payload["reservation_id"] = paymentOrder.ReservationID.Int64
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return raw
}

func combinedReservationPaymentCallbackFactResource(combined db.CombinedPaymentOrder, paymentOrder db.PaymentOrder, subOrder wechatcontracts.CombinePaymentNotificationSubOrder) []byte {
	payload := map[string]any{
		"combined_payment_id":  combined.ID,
		"combine_out_trade_no": combined.CombineOutTradeNo,
		"payment_order_id":     paymentOrder.ID,
		"business_type":        paymentOrder.BusinessType,
		"out_trade_no":         paymentOrder.OutTradeNo,
		"transaction_id":       subOrder.TransactionID,
		"trade_state":          subOrder.TradeState,
		"success_time":         subOrder.SuccessTime,
		"amount":               subOrder.Amount.TotalAmount,
	}
	if paymentOrder.ReservationID.Valid {
		payload["reservation_id"] = paymentOrder.ReservationID.Int64
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return raw
}
