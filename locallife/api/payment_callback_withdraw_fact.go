package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

func (server *Server) recordMerchantWithdrawCallbackFact(ctx context.Context, notification wechat.PaymentNotification, record db.WithdrawalRecord, resource *wechatcontracts.WithdrawNotificationResource) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || resource == nil {
		return nil, nil
	}

	occurredAt := parseWechatFactTime(resource.UpdateTime)
	if occurredAt == nil {
		occurredAt = parseWechatFactTime(resource.CreateTime)
	}
	amount := record.Amount
	if resource.Amount > 0 {
		amount = resource.Amount
	}

	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityWithdraw,
		FactSource:           db.ExternalPaymentFactSourceCallback,
		SourceEventID:        paymentFactStringPtr(notification.ID),
		SourceEventType:      paymentFactStringPtr(notification.EventType),
		ExternalObjectType:   db.ExternalPaymentObjectWithdraw,
		ExternalObjectKey:    strings.TrimSpace(resource.OutRequestNo),
		ExternalSecondaryKey: paymentFactStringPtr(strings.TrimSpace(resource.WithdrawID)),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerMerchantFunds),
		BusinessObjectType:   paymentFactStringPtr(merchantWithdrawFactBusinessObject),
		BusinessObjectID:     paymentFactInt64Ptr(record.ID),
		UpstreamState:        strings.TrimSpace(resource.Status),
		TerminalStatus:       merchantWithdrawCallbackTerminalStatus(resource.Status),
		Amount:               paymentFactInt64Ptr(amount),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          merchantWithdrawCallbackFactResource(record, resource),
		DedupeKey:            fmt.Sprintf("wechat:callback:ecommerce:withdraw:%s", notification.ID),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           merchantWithdrawFactConsumerDomain,
			BusinessObjectType: merchantWithdrawFactBusinessObject,
			BusinessObjectID:   record.ID,
		},
		AllowNonTerminalApplication: true,
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func merchantWithdrawCallbackTerminalStatus(status string) string {
	switch mapWechatWithdrawStatus(status) {
	case "success":
		return db.ExternalPaymentTerminalStatusSuccess
	case "failed":
		return db.ExternalPaymentTerminalStatusFailed
	default:
		return db.ExternalPaymentTerminalStatusProcessing
	}
}

func merchantWithdrawCallbackFactResource(record db.WithdrawalRecord, resource *wechatcontracts.WithdrawNotificationResource) []byte {
	raw, err := json.Marshal(map[string]any{
		"withdrawal_record_id": record.ID,
		"sub_mch_id":           strings.TrimSpace(resource.SubMchID),
		"out_request_no":       strings.TrimSpace(resource.OutRequestNo),
		"withdraw_id":          strings.TrimSpace(resource.WithdrawID),
		"wechat_status":        strings.TrimSpace(resource.Status),
		"reason":               strings.TrimSpace(resource.Reason),
		"amount":               record.Amount,
		"create_time":          strings.TrimSpace(resource.CreateTime),
		"update_time":          strings.TrimSpace(resource.UpdateTime),
	})
	if err != nil {
		return nil
	}
	return raw
}
