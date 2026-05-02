package api

import (
	"context"
	"encoding/json"
	"fmt"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/rs/zerolog/log"
)

func profitSharingFactResource(resource *wechatcontracts.ProfitSharingNotification, queryResp *wechatcontracts.ProfitSharingQueryResponse, finalResult, finalFailReason string) []byte {
	if resource == nil && queryResp == nil {
		return nil
	}
	var spMchID, subMchID, transactionID, outOrderNo, orderID, successTime string
	if resource != nil {
		spMchID = resource.SPMchID
		subMchID = resource.SubMchID
		transactionID = resource.TransactionID
		outOrderNo = resource.OutOrderNo
		orderID = resource.OrderID
		successTime = resource.SuccessTime
	}
	queryStatus := ""
	finishAmount := int64(0)
	receiverResults := make([]map[string]any, 0)
	if queryResp != nil {
		queryStatus = queryResp.Status
		finishAmount = queryResp.FinishAmount
		receiverResults = make([]map[string]any, 0, len(queryResp.Receivers))
		for _, receiver := range queryResp.Receivers {
			receiverResults = append(receiverResults, map[string]any{
				"type":        receiver.Type,
				"amount":      receiver.Amount,
				"result":      receiver.Result,
				"fail_reason": receiver.FailReason,
				"detail_id":   receiver.DetailID,
			})
		}
	}

	raw, err := json.Marshal(map[string]any{
		"sp_mch_id":        spMchID,
		"sub_mch_id":       subMchID,
		"transaction_id":   transactionID,
		"out_order_no":     outOrderNo,
		"order_id":         orderID,
		"query_status":     queryStatus,
		"result":           finalResult,
		"fail_reason":      finalFailReason,
		"receiver_count":   len(receiverResults),
		"receiver_results": receiverResults,
		"finish_amount":    finishAmount,
		"success_time":     successTime,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_order_no", outOrderNo).Msg("marshal profit sharing fact resource failed")
		return nil
	}
	return raw
}

func profitSharingFactAmount(queryResp *wechatcontracts.ProfitSharingQueryResponse) *int64 {
	if queryResp == nil {
		return nil
	}
	var amount int64
	for _, receiver := range queryResp.Receivers {
		amount += receiver.Amount
	}
	if amount == 0 {
		return nil
	}
	return paymentFactInt64Ptr(amount)
}

func (server *Server) recordProfitSharingCallbackFact(ctx context.Context, channel string, notification wechat.PaymentNotification, profitSharingOrder db.ProfitSharingOrder, resource *wechatcontracts.ProfitSharingNotification, queryResp *wechatcontracts.ProfitSharingQueryResponse, finalResult, finalFailReason string) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || resource == nil || queryResp == nil {
		return nil, nil
	}
	occurredAt := parseWechatFactTime(resource.SuccessTime)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              channel,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        paymentFactStringPtr(notification.ID),
		SourceEventType:      paymentFactStringPtr(notification.EventType),
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharing,
		ExternalObjectKey:    resource.OutOrderNo,
		ExternalSecondaryKey: paymentFactStringPtr(resource.OrderID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerProfitSharing),
		BusinessObjectType:   paymentFactStringPtr(paymentFactBusinessObjectProfitSharingOrder),
		BusinessObjectID:     paymentFactInt64Ptr(profitSharingOrder.ID),
		UpstreamState:        finalResult,
		TerminalStatus:       logic.NormalizeProfitSharingTerminalStatus(finalResult),
		Amount:               profitSharingFactAmount(queryResp),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          profitSharingFactResource(resource, queryResp, finalResult, finalFailReason),
		DedupeKey:            fmt.Sprintf("wechat:callback:%s:profit_sharing:%s", channel, notification.ID),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           paymentFactConsumerProfitSharingDomain,
			BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder,
			BusinessObjectID:   profitSharingOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func ordinaryProfitSharingFactResource(resource *ospcontracts.ProfitSharingNotificationPayload, queryResp *ospcontracts.ProfitSharingOrderResponse, finalResult, finalFailReason string) []byte {
	if resource == nil && queryResp == nil {
		return nil
	}
	var spMchID, subMchID, transactionID, outOrderNo, orderID, successTime string
	if resource != nil {
		spMchID = resource.SpMchID
		subMchID = resource.SubMchID
		transactionID = resource.TransactionID
		outOrderNo = resource.OutOrderNo
		orderID = resource.OrderID
		successTime = resource.SuccessTime
	}
	queryStatus := ""
	receiverResults := make([]map[string]any, 0)
	if queryResp != nil {
		queryStatus = string(queryResp.State)
		receiverResults = make([]map[string]any, 0, len(queryResp.Receivers))
		for _, receiver := range queryResp.Receivers {
			receiverResults = append(receiverResults, map[string]any{
				"type":        receiver.Type,
				"amount":      receiver.Amount,
				"result":      receiver.Result,
				"fail_reason": receiver.FailReason,
				"detail_id":   receiver.DetailID,
			})
		}
	}

	raw, err := json.Marshal(map[string]any{
		"sp_mch_id":        spMchID,
		"sub_mch_id":       subMchID,
		"transaction_id":   transactionID,
		"out_order_no":     outOrderNo,
		"order_id":         orderID,
		"query_status":     queryStatus,
		"result":           finalResult,
		"fail_reason":      finalFailReason,
		"receiver_count":   len(receiverResults),
		"receiver_results": receiverResults,
		"finish_amount":    ordinaryProfitSharingFactAmountValue(queryResp),
		"success_time":     successTime,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_order_no", outOrderNo).Msg("marshal ordinary profit sharing fact resource failed")
		return nil
	}
	return raw
}

func ordinaryProfitSharingFactAmount(queryResp *ospcontracts.ProfitSharingOrderResponse) *int64 {
	amount := ordinaryProfitSharingFactAmountValue(queryResp)
	if amount == 0 {
		return nil
	}
	return paymentFactInt64Ptr(amount)
}

func ordinaryProfitSharingFactAmountValue(queryResp *ospcontracts.ProfitSharingOrderResponse) int64 {
	if queryResp == nil {
		return 0
	}
	var amount int64
	for _, receiver := range queryResp.Receivers {
		amount += receiver.Amount
	}
	return amount
}

func (server *Server) recordOrdinaryProfitSharingCallbackFact(ctx context.Context, notification wechat.PaymentNotification, profitSharingOrder db.ProfitSharingOrder, resource *ospcontracts.ProfitSharingNotificationPayload, queryResp *ospcontracts.ProfitSharingOrderResponse, finalResult, finalFailReason string) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || resource == nil || queryResp == nil {
		return nil, nil
	}
	occurredAt := parseWechatFactTime(resource.SuccessTime)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelOrdinaryServiceProvider,
		Capability:           db.ExternalPaymentCapabilityProfitSharing,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        paymentFactStringPtr(notification.ID),
		SourceEventType:      paymentFactStringPtr(notification.EventType),
		ExternalObjectType:   db.ExternalPaymentObjectProfitSharing,
		ExternalObjectKey:    resource.OutOrderNo,
		ExternalSecondaryKey: paymentFactStringPtr(resource.OrderID),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerProfitSharing),
		BusinessObjectType:   paymentFactStringPtr(paymentFactBusinessObjectProfitSharingOrder),
		BusinessObjectID:     paymentFactInt64Ptr(profitSharingOrder.ID),
		UpstreamState:        finalResult,
		TerminalStatus:       logic.NormalizeProfitSharingTerminalStatus(finalResult),
		Amount:               ordinaryProfitSharingFactAmount(queryResp),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          ordinaryProfitSharingFactResource(resource, queryResp, finalResult, finalFailReason),
		DedupeKey:            fmt.Sprintf("wechat:callback:%s:profit_sharing:%s", db.PaymentChannelOrdinaryServiceProvider, notification.ID),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           paymentFactConsumerProfitSharingDomain,
			BusinessObjectType: paymentFactBusinessObjectProfitSharingOrder,
			BusinessObjectID:   profitSharingOrder.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}
