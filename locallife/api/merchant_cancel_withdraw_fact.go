package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

const (
	merchantCancelWithdrawFactConsumerDomain = "merchant_funds_domain"
	merchantCancelWithdrawFactBusinessObject = "merchant_cancel_withdraw_application"
)

func (server *Server) recordMerchantCancelWithdrawQueryFact(ctx context.Context, record db.MerchantCancelWithdrawApplication, queryResp *wechatcontracts.CancelWithdrawQueryResponse) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || queryResp == nil {
		return nil, nil
	}

	outRequestNo := strings.TrimSpace(queryResp.OutRequestNo)
	if outRequestNo == "" {
		outRequestNo = strings.TrimSpace(record.OutRequestNo)
	}
	if outRequestNo == "" {
		return nil, nil
	}

	occurredAt := parseMerchantCancelWithdrawFactTime(queryResp.ModifyTime)
	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityCancelWithdraw,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectCancelWithdraw,
		ExternalObjectKey:    outRequestNo,
		ExternalSecondaryKey: stringPtrIfNotEmpty(strings.TrimSpace(queryResp.ApplymentID)),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerMerchantFunds),
		BusinessObjectType:   paymentFactStringPtr(merchantCancelWithdrawFactBusinessObject),
		BusinessObjectID:     paymentFactInt64Ptr(record.ID),
		UpstreamState:        strings.TrimSpace(queryResp.CancelState),
		TerminalStatus:       merchantCancelWithdrawQueryTerminalStatus(queryResp.CancelState),
		Currency:             "CNY",
		OccurredAt:           occurredAt,
		UpstreamUpdatedAt:    occurredAt,
		RawResource:          merchantCancelWithdrawQueryFactResource(record, queryResp),
		DedupeKey:            merchantCancelWithdrawQueryFactDedupeKey(outRequestNo, queryResp.CancelState, queryResp.WithdrawState, queryResp.ApplymentID),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           merchantCancelWithdrawFactConsumerDomain,
			BusinessObjectType: merchantCancelWithdrawFactBusinessObject,
			BusinessObjectID:   record.ID,
		},
		AllowNonTerminalApplication: true,
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func (server *Server) applyMerchantCancelWithdrawFactApplication(ctx context.Context, application *db.ExternalPaymentFactApplication) (db.MerchantCancelWithdrawApplication, bool, error) {
	if server.paymentFactService == nil || application == nil {
		return db.MerchantCancelWithdrawApplication{}, false, nil
	}

	result, err := server.paymentFactService.ApplyExternalPaymentFactApplication(ctx, application.ID)
	if err != nil {
		return db.MerchantCancelWithdrawApplication{}, false, err
	}
	if result.MerchantCancelWithdraw == nil {
		return db.MerchantCancelWithdrawApplication{}, false, nil
	}
	return result.MerchantCancelWithdraw.Application, true, nil
}

func merchantCancelWithdrawQueryTerminalStatus(cancelState string) string {
	if !logic.MerchantCancelWithdrawIsTerminal(cancelState) {
		return db.ExternalPaymentTerminalStatusProcessing
	}

	switch logic.NormalizeMerchantCancelState(cancelState) {
	case db.MerchantCancelStateFinish:
		return db.ExternalPaymentTerminalStatusSuccess
	default:
		return db.ExternalPaymentTerminalStatusFailed
	}
}

func merchantCancelWithdrawQueryFactDedupeKey(outRequestNo string, cancelState string, withdrawState string, applymentID string) string {
	suffix := strings.TrimSpace(applymentID)
	if suffix == "" {
		suffix = "current"
	}
	return fmt.Sprintf(
		"wechat:query:ecommerce:cancel_withdraw:%s:%s:%s:%s",
		strings.TrimSpace(outRequestNo),
		strings.TrimSpace(cancelState),
		strings.TrimSpace(withdrawState),
		suffix,
	)
}

func merchantCancelWithdrawQueryFactResource(record db.MerchantCancelWithdrawApplication, queryResp *wechatcontracts.CancelWithdrawQueryResponse) []byte {
	confirmCancelURL := ""
	if queryResp.ConfirmCancel != nil {
		confirmCancelURL = strings.TrimSpace(queryResp.ConfirmCancel.ConfirmCancelURL)
	}

	raw, err := json.Marshal(map[string]any{
		"application_id":             record.ID,
		"merchant_id":                record.MerchantID,
		"sub_mch_id":                 strings.TrimSpace(record.SubMchID),
		"out_request_no":             strings.TrimSpace(queryResp.OutRequestNo),
		"applyment_id":               strings.TrimSpace(queryResp.ApplymentID),
		"cancel_state":               strings.TrimSpace(queryResp.CancelState),
		"cancel_state_description":   strings.TrimSpace(queryResp.CancelStateDescription),
		"withdraw":                   strings.TrimSpace(queryResp.Withdraw),
		"withdraw_state":             strings.TrimSpace(queryResp.WithdrawState),
		"withdraw_state_description": strings.TrimSpace(queryResp.WithdrawStateDescription),
		"modify_time":                strings.TrimSpace(queryResp.ModifyTime),
		"confirm_cancel_url":         confirmCancelURL,
		"account_info":               queryResp.AccountInfo,
		"account_withdraw_result":    queryResp.AccountWithdrawResult,
	})
	if err != nil {
		return nil
	}
	return raw
}

func parseMerchantCancelWithdrawFactTime(value string) *time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil
	}
	return &parsed
}
