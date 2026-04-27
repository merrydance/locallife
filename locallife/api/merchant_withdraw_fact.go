package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/wechat"
)

const (
	merchantWithdrawFactConsumerDomain = "merchant_funds_domain"
	merchantWithdrawFactBusinessObject = "withdrawal_record"
)

func (server *Server) recordMerchantWithdrawQueryFact(ctx context.Context, record db.WithdrawalRecord, accountInfo merchantWithdrawAccountInfo, resp *wechat.EcommerceWithdrawResponse) (*db.ExternalPaymentFactApplication, error) {
	if server.paymentFactService == nil || resp == nil {
		return nil, nil
	}

	outRequestNo := strings.TrimSpace(accountInfo.OutRequestNo)
	if outRequestNo == "" {
		return nil, nil
	}

	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           db.ExternalPaymentCapabilityWithdraw,
		FactSource:           db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:   db.ExternalPaymentObjectWithdraw,
		ExternalObjectKey:    outRequestNo,
		ExternalSecondaryKey: stringPtrIfNotEmpty(strings.TrimSpace(resp.WithdrawID)),
		BusinessOwner:        paymentFactStringPtr(db.ExternalPaymentBusinessOwnerMerchantFunds),
		BusinessObjectType:   paymentFactStringPtr(merchantWithdrawFactBusinessObject),
		BusinessObjectID:     paymentFactInt64Ptr(record.ID),
		UpstreamState:        strings.TrimSpace(resp.Status),
		TerminalStatus:       merchantWithdrawTerminalStatus(resp.Status),
		Amount:               paymentFactInt64Ptr(record.Amount),
		Currency:             "CNY",
		RawResource:          merchantWithdrawQueryFactResource(record, accountInfo, resp),
		DedupeKey:            merchantWithdrawQueryFactDedupeKey(outRequestNo, resp.Status, resp.WithdrawID, resp.Reason),
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

func (server *Server) applyMerchantWithdrawFactApplication(ctx context.Context, application *db.ExternalPaymentFactApplication) (db.WithdrawalRecord, bool, error) {
	if server.paymentFactService == nil || application == nil {
		return db.WithdrawalRecord{}, false, nil
	}

	result, err := server.paymentFactService.ApplyExternalPaymentFactApplication(ctx, application.ID)
	if err != nil {
		return db.WithdrawalRecord{}, false, err
	}
	if result.MerchantWithdraw == nil {
		return db.WithdrawalRecord{}, false, nil
	}
	return result.MerchantWithdraw.WithdrawalRecord, true, nil
}

func merchantWithdrawTerminalStatus(status string) string {
	switch mapWechatWithdrawStatus(status) {
	case "success":
		return db.ExternalPaymentTerminalStatusSuccess
	case "failed":
		return db.ExternalPaymentTerminalStatusFailed
	default:
		return db.ExternalPaymentTerminalStatusProcessing
	}
}

func merchantWithdrawQueryFactDedupeKey(outRequestNo string, status string, withdrawID string, reason string) string {
	suffix := strings.TrimSpace(withdrawID)
	if suffix == "" {
		suffix = strings.TrimSpace(reason)
	}
	if suffix == "" {
		suffix = "current"
	}
	return fmt.Sprintf("wechat:query:ecommerce:withdraw:%s:%s:%s", strings.TrimSpace(outRequestNo), strings.TrimSpace(status), suffix)
}

func merchantWithdrawQueryFactResource(record db.WithdrawalRecord, accountInfo merchantWithdrawAccountInfo, resp *wechat.EcommerceWithdrawResponse) []byte {
	raw, err := json.Marshal(map[string]any{
		"withdrawal_record_id": record.ID,
		"merchant_id":          accountInfo.MerchantID,
		"sub_mch_id":           strings.TrimSpace(accountInfo.SubMchID),
		"out_request_no":       strings.TrimSpace(accountInfo.OutRequestNo),
		"withdraw_id":          strings.TrimSpace(resp.WithdrawID),
		"wechat_status":        strings.TrimSpace(resp.Status),
		"reason":               strings.TrimSpace(resp.Reason),
		"amount":               record.Amount,
	})
	if err != nil {
		return nil
	}
	return raw
}
