package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
)

const (
	reservationPaymentFactBusinessObjectOrder = "payment_order"
	reservationPaymentFactConsumerDomain      = "reservation_domain"
	reservationPaymentAddonBusinessType       = "reservation_addon"
)

func shouldRecordReservationPaymentRecoveryFact(order db.PaymentOrder) bool {
	return shouldRecordReservationPaymentFactForOrder(order)
}

func shouldRecordReservationPaymentFactForOrder(order db.PaymentOrder) bool {
	return (order.PaymentChannel == db.PaymentChannelEcommerce || order.PaymentChannel == db.PaymentChannelOrdinaryServiceProvider) &&
		order.ReservationID.Valid &&
		(order.BusinessType == db.ExternalPaymentBusinessOwnerReservation || order.BusinessType == reservationPaymentAddonBusinessType)
}

func recordCombinedReservationPaymentQueryFact(ctx context.Context, store db.Store, combined db.CombinedPaymentOrder, paymentOrder db.PaymentOrder, subOrder wechatcontracts.CombineSubOrderResult) (*db.ExternalPaymentFactApplication, error) {
	service := logic.NewPaymentFactService(store)
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              paymentFactChannel(paymentOrder.PaymentChannel, db.PaymentChannelEcommerce),
		Capability:           db.ExternalPaymentCapabilityCombinePayment,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectCombinedPayment,
		ExternalObjectKey:    combined.CombineOutTradeNo,
		ExternalSecondaryKey: orderPaymentStringPtr(subOrder.OutTradeNo),
		BusinessOwner:        orderPaymentStringPtr(db.ExternalPaymentBusinessOwnerReservation),
		BusinessObjectType:   orderPaymentStringPtr(reservationPaymentFactBusinessObjectOrder),
		BusinessObjectID:     orderPaymentInt64Ptr(paymentOrder.ID),
		UpstreamState:        subOrder.TradeState,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               orderPaymentInt64Ptr(subOrder.Amount.TotalAmount),
		Currency:             "CNY",
		OccurredAt:           orderPaymentParseFactTime(subOrder.SuccessTime),
		UpstreamUpdatedAt:    orderPaymentParseFactTime(subOrder.SuccessTime),
		RawResource:          combinedReservationPaymentQueryFactResource(combined, paymentOrder, subOrder),
		DedupeKey:            fmt.Sprintf("wechat:query:%s:combine_reservation_payment:%s:%s:success", paymentFactChannel(paymentOrder.PaymentChannel, db.PaymentChannelEcommerce), combined.CombineOutTradeNo, subOrder.OutTradeNo),
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

func recordReservationPaymentQueryFact(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, queryResp *wechatcontracts.PartnerOrderQueryResponse) (*db.ExternalPaymentFactApplication, error) {
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
		BusinessOwner:        orderPaymentStringPtr(db.ExternalPaymentBusinessOwnerReservation),
		BusinessObjectType:   orderPaymentStringPtr(reservationPaymentFactBusinessObjectOrder),
		BusinessObjectID:     orderPaymentInt64Ptr(paymentOrder.ID),
		UpstreamState:        queryResp.TradeState,
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               orderPaymentInt64Ptr(queryResp.Amount.Total),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          reservationPaymentQueryFactResource(paymentOrder, queryResp),
		DedupeKey:            fmt.Sprintf("wechat:query:%s:partner_reservation_payment:%s:success", paymentFactChannel(paymentOrder.PaymentChannel, db.PaymentChannelEcommerce), objectKey),
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

func recordOrdinaryServiceProviderReservationPaymentTimeoutQueryFact(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, queryResp *ospcontracts.PaymentQueryResponse) (*db.ExternalPaymentFactApplication, error) {
	if !shouldRecordReservationPaymentFactForOrder(paymentOrder) {
		return nil, fmt.Errorf("unsupported ordinary service provider reservation payment timeout fact owner %q for payment order %d", paymentOrder.BusinessType, paymentOrder.ID)
	}
	if queryResp == nil || queryResp.Amount == nil {
		return nil, fmt.Errorf("ordinary service provider reservation payment timeout query response missing amount for payment order %d", paymentOrder.ID)
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
		BusinessOwner:        orderPaymentStringPtr(db.ExternalPaymentBusinessOwnerReservation),
		BusinessObjectType:   orderPaymentStringPtr(reservationPaymentFactBusinessObjectOrder),
		BusinessObjectID:     orderPaymentInt64Ptr(paymentOrder.ID),
		UpstreamState:        string(queryResp.TradeState),
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               orderPaymentInt64Ptr(queryResp.Amount.Total),
		Currency:             "CNY",
		RawResource:          ordinaryServiceProviderReservationPaymentQueryFactResource(paymentOrder, queryResp),
		DedupeKey:            fmt.Sprintf("wechat:query:%s:partner_reservation_payment:%s:success", db.PaymentChannelOrdinaryServiceProvider, objectKey),
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

func recordRecoveredReservationPaymentFact(ctx context.Context, store db.Store, order db.PaymentOrder) (*db.ExternalPaymentFactApplication, error) {
	capability := db.ExternalPaymentCapabilityPartnerJSAPIPayment
	if order.CombinedPaymentID.Valid {
		capability = db.ExternalPaymentCapabilityCombinePayment
	}

	service := logic.NewPaymentFactService(store)
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              paymentFactChannel(order.PaymentChannel, db.PaymentChannelEcommerce),
		Capability:           capability,
		FactSource:           db.ExternalPaymentFactSourceManualReconciliation,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    order.OutTradeNo,
		ExternalSecondaryKey: orderPaymentOptionalStringPtr(order.TransactionID),
		BusinessOwner:        orderPaymentStringPtr(db.ExternalPaymentBusinessOwnerReservation),
		BusinessObjectType:   orderPaymentStringPtr(reservationPaymentFactBusinessObjectOrder),
		BusinessObjectID:     orderPaymentInt64Ptr(order.ID),
		UpstreamState:        "SUCCESS",
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               orderPaymentInt64Ptr(order.Amount),
		Currency:             "CNY",
		RawResource:          recoveredReservationPaymentFactResource(order),
		DedupeKey:            recoveredReservationPaymentFactDedupeKey(order, capability),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           reservationPaymentFactConsumerDomain,
			BusinessObjectType: reservationPaymentFactBusinessObjectOrder,
			BusinessObjectID:   order.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func enqueueReservationPaymentFactApplication(ctx context.Context, distributor any, application *db.ExternalPaymentFactApplication) error {
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

func combinedReservationPaymentQueryFactResource(combined db.CombinedPaymentOrder, paymentOrder db.PaymentOrder, subOrder wechatcontracts.CombineSubOrderResult) []byte {
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

func reservationPaymentQueryFactResource(paymentOrder db.PaymentOrder, queryResp *wechatcontracts.PartnerOrderQueryResponse) []byte {
	payload := map[string]any{
		"payment_order_id": paymentOrder.ID,
		"business_type":    paymentOrder.BusinessType,
		"out_trade_no":     paymentOrder.OutTradeNo,
		"transaction_id":   queryResp.TransactionID,
		"trade_state":      queryResp.TradeState,
		"success_time":     queryResp.SuccessTime,
		"amount":           queryResp.Amount.Total,
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

func ordinaryServiceProviderReservationPaymentQueryFactResource(paymentOrder db.PaymentOrder, queryResp *ospcontracts.PaymentQueryResponse) []byte {
	amount := int64(0)
	if queryResp.Amount != nil {
		amount = queryResp.Amount.Total
	}
	payload := map[string]any{
		"payment_order_id": paymentOrder.ID,
		"business_type":    paymentOrder.BusinessType,
		"out_trade_no":     paymentOrder.OutTradeNo,
		"transaction_id":   queryResp.TransactionID,
		"trade_state":      queryResp.TradeState,
		"amount":           amount,
		"payment_channel":  db.PaymentChannelOrdinaryServiceProvider,
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

func recoveredReservationPaymentFactResource(order db.PaymentOrder) []byte {
	payload := map[string]any{
		"payment_order_id":    order.ID,
		"business_type":       order.BusinessType,
		"out_trade_no":        order.OutTradeNo,
		"transaction_id":      orderPaymentTextValue(order.TransactionID),
		"payment_channel":     order.PaymentChannel,
		"combined_payment_id": orderPaymentInt8Value(order.CombinedPaymentID),
		"recovery_reason":     "paid_unprocessed_scan",
	}
	if order.ReservationID.Valid {
		payload["reservation_id"] = order.ReservationID.Int64
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return raw
}

func recoveredReservationPaymentFactDedupeKey(order db.PaymentOrder, capability string) string {
	return fmt.Sprintf("wechat:manual_reconciliation:%s:reservation_payment:%s:%s:success", paymentFactChannel(order.PaymentChannel, db.PaymentChannelEcommerce), capability, order.OutTradeNo)
}
