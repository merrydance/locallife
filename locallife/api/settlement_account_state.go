package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/rs/zerolog/log"
)

const (
	settlementFactBusinessObjectMerchantPaymentConfig = "merchant_payment_config"
	settlementFactBusinessObjectApplyment             = "ordinary_service_provider_applyment"
	settlementFactConsumerDomain                      = "settlement_domain"
)

func buildSettlementAccountStatusDesc(verifyResult, verifyFailReason string) string {
	switch strings.TrimSpace(verifyResult) {
	case "VERIFY_FAIL":
		if reason := strings.TrimSpace(verifyFailReason); reason != "" {
			return fmt.Sprintf("微信提现卡校验失败：%s", reason)
		}
		return "微信提现卡校验失败，请尽快更换银行卡"
	case "VERIFYING":
		return "微信提现卡正在校验中，暂时无法提现，请稍后查看结果"
	case "VERIFY_SUCCESS":
		return "微信提现卡已通过微信校验"
	default:
		return ""
	}
}

func settlementApplicationTrackingTimestamp(submittedAt *time.Time) pgtype.Timestamptz {
	if submittedAt == nil || submittedAt.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *submittedAt, Valid: true}
}

func (server *Server) updateMerchantSettlementApplicationTracking(ctx context.Context, merchantID int64, applicationNo string, submittedAt *time.Time) error {
	trimmedApplicationNo := strings.TrimSpace(applicationNo)
	if trimmedApplicationNo == "" {
		return nil
	}

	_, err := server.store.UpdateMerchantPaymentConfigSettlementApplication(ctx, db.UpdateMerchantPaymentConfigSettlementApplicationParams{
		MerchantID:                             merchantID,
		LatestSettlementApplicationNo:          buildApplymentText(trimmedApplicationNo),
		LatestSettlementApplicationSubmittedAt: settlementApplicationTrackingTimestamp(submittedAt),
	})
	return err
}

func (server *Server) recordMerchantSettlementApplicationQueryFact(ctx context.Context, merchantID int64, paymentConfig db.MerchantPaymentConfig, applicationNo string, resp *ospcontracts.SettlementModificationQueryResponse) (*db.ExternalPaymentFactApplication, error) {
	trimmedSubMchID := strings.TrimSpace(paymentConfig.SubMchID)
	trimmedApplicationNo := strings.TrimSpace(applicationNo)
	if server.paymentFactService == nil || trimmedSubMchID == "" || trimmedApplicationNo == "" || resp == nil {
		return nil, nil
	}

	businessOwner := db.ExternalPaymentBusinessOwnerMerchantFunds
	businessObjectType := settlementFactBusinessObjectMerchantPaymentConfig
	businessObjectID := paymentConfig.ID
	verifyResult := strings.TrimSpace(string(resp.VerifyResult))
	verifyFailReason := strings.TrimSpace(resp.VerifyFailReason)
	verifyFinishTime := strings.TrimSpace(resp.VerifyFinishTime)
	factTime := parseSettlementApplicationFactTime(verifyFinishTime)

	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:                    db.ExternalPaymentProviderWechat,
		Channel:                     db.PaymentChannelOrdinaryServiceProvider,
		Capability:                  db.ExternalPaymentCapabilitySettlement,
		FactSource:                  db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:          db.ExternalPaymentObjectSettlement,
		ExternalObjectKey:           trimmedSubMchID,
		ExternalSecondaryKey:        settlementStringPtr(trimmedApplicationNo),
		BusinessOwner:               settlementStringPtr(businessOwner),
		BusinessObjectType:          settlementStringPtr(businessObjectType),
		BusinessObjectID:            &businessObjectID,
		UpstreamState:               verifyResult,
		TerminalStatus:              settlementApplicationQueryTerminalStatus(verifyResult),
		Currency:                    "CNY",
		OccurredAt:                  factTime,
		UpstreamUpdatedAt:           factTime,
		RawResource:                 settlementApplicationQueryFactResource(merchantID, trimmedSubMchID, trimmedApplicationNo, resp),
		DedupeKey:                   settlementApplicationQueryFactDedupeKey(trimmedSubMchID, trimmedApplicationNo, verifyResult, verifyFinishTime, verifyFailReason),
		AllowNonTerminalApplication: true,
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           settlementFactConsumerDomain,
			BusinessObjectType: settlementFactBusinessObjectMerchantPaymentConfig,
			BusinessObjectID:   paymentConfig.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func (server *Server) recordMerchantSettlementAccountQueryFact(ctx context.Context, merchantID int64, paymentConfig db.MerchantPaymentConfig, latestApplicationNo string, resp *ospcontracts.SettlementQueryResponse) (*db.ExternalPaymentFactApplication, error) {
	trimmedSubMchID := strings.TrimSpace(paymentConfig.SubMchID)
	if server.paymentFactService == nil || trimmedSubMchID == "" || resp == nil {
		return nil, nil
	}

	businessOwner := db.ExternalPaymentBusinessOwnerMerchantFunds
	businessObjectType := settlementFactBusinessObjectMerchantPaymentConfig
	businessObjectID := paymentConfig.ID
	verifyResult := strings.TrimSpace(string(resp.VerifyResult))
	verifyFailReason := strings.TrimSpace(resp.VerifyFailReason)
	trimmedLatestApplicationNo := strings.TrimSpace(latestApplicationNo)
	applicationTarget, err := server.resolveMerchantSettlementVerificationApplicationTarget(ctx, merchantID, trimmedSubMchID)
	if err != nil {
		return nil, err
	}

	result, err := server.paymentFactService.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:                    db.ExternalPaymentProviderWechat,
		Channel:                     db.PaymentChannelOrdinaryServiceProvider,
		Capability:                  db.ExternalPaymentCapabilitySettlement,
		FactSource:                  db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:          db.ExternalPaymentObjectSettlement,
		ExternalObjectKey:           trimmedSubMchID,
		ExternalSecondaryKey:        settlementStringPtr(trimmedLatestApplicationNo),
		BusinessOwner:               settlementStringPtr(businessOwner),
		BusinessObjectType:          settlementStringPtr(businessObjectType),
		BusinessObjectID:            &businessObjectID,
		UpstreamState:               verifyResult,
		TerminalStatus:              settlementAccountQueryTerminalStatus(verifyResult),
		Currency:                    "CNY",
		RawResource:                 settlementAccountQueryFactResource(merchantID, trimmedSubMchID, trimmedLatestApplicationNo, applicationTarget, resp),
		DedupeKey:                   settlementAccountQueryFactDedupeKey(trimmedSubMchID, verifyResult, trimmedLatestApplicationNo, verifyFailReason),
		AllowNonTerminalApplication: true,
		Application:                 applicationTarget,
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func (server *Server) resolveMerchantSettlementVerificationApplicationTarget(ctx context.Context, merchantID int64, subMchID string) (*logic.ExternalPaymentFactApplicationTarget, error) {
	if merchantID == 0 || strings.TrimSpace(subMchID) == "" {
		return nil, nil
	}

	applyment, err := server.store.GetLatestEcommerceApplymentBySubject(ctx, db.GetLatestEcommerceApplymentBySubjectParams{
		SubjectType: "merchant",
		SubjectID:   merchantID,
	})
	if errors.Is(err, db.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest merchant applyment for settlement verification: %w", err)
	}
	if !applyment.SubMchID.Valid || strings.TrimSpace(applyment.SubMchID.String) == "" {
		return nil, nil
	}
	if strings.TrimSpace(applyment.SubMchID.String) != strings.TrimSpace(subMchID) {
		return nil, fmt.Errorf("merchant %d latest applyment %d sub_mch_id %q does not match settlement query sub_mch_id %q", merchantID, applyment.ID, strings.TrimSpace(applyment.SubMchID.String), strings.TrimSpace(subMchID))
	}
	return &logic.ExternalPaymentFactApplicationTarget{
		Consumer:           settlementFactConsumerDomain,
		BusinessObjectType: settlementFactBusinessObjectApplyment,
		BusinessObjectID:   applyment.ID,
	}, nil
}

func settlementAccountQueryTerminalStatus(verifyResult string) string {
	switch strings.TrimSpace(verifyResult) {
	case string(ospcontracts.SettlementVerifyResultSuccess):
		return db.ExternalPaymentTerminalStatusSuccess
	case string(ospcontracts.SettlementVerifyResultFail):
		return db.ExternalPaymentTerminalStatusFailed
	case string(ospcontracts.SettlementVerifyResultIng):
		return db.ExternalPaymentTerminalStatusProcessing
	default:
		return db.ExternalPaymentTerminalStatusUnknown
	}
}

func settlementApplicationQueryTerminalStatus(verifyResult string) string {
	switch strings.TrimSpace(verifyResult) {
	case string(ospcontracts.SettlementAuditResultSuccess):
		return db.ExternalPaymentTerminalStatusSuccess
	case string(ospcontracts.SettlementAuditResultFail):
		return db.ExternalPaymentTerminalStatusFailed
	case string(ospcontracts.SettlementAuditResultAuditing):
		return db.ExternalPaymentTerminalStatusProcessing
	default:
		return db.ExternalPaymentTerminalStatusUnknown
	}
}

func parseSettlementApplicationFactTime(value string) *time.Time {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return nil
	}
	parsedTime, err := time.Parse(time.RFC3339, trimmedValue)
	if err != nil {
		return nil
	}
	return &parsedTime
}

func settlementApplicationQueryFactResource(merchantID int64, subMchID string, applicationNo string, resp *ospcontracts.SettlementModificationQueryResponse) []byte {
	raw, err := json.Marshal(map[string]any{
		"merchant_id":        merchantID,
		"sub_mch_id":         strings.TrimSpace(subMchID),
		"application_no":     strings.TrimSpace(applicationNo),
		"account_name":       strings.TrimSpace(resp.AccountName),
		"account_type":       strings.TrimSpace(string(resp.AccountType)),
		"account_bank":       strings.TrimSpace(resp.AccountBank),
		"bank_name":          strings.TrimSpace(resp.BankName),
		"bank_branch_id":     strings.TrimSpace(resp.BankBranchID),
		"account_number":     strings.TrimSpace(resp.AccountNumber),
		"verify_result":      strings.TrimSpace(string(resp.VerifyResult)),
		"verify_fail_reason": strings.TrimSpace(resp.VerifyFailReason),
		"verify_finish_time": strings.TrimSpace(resp.VerifyFinishTime),
	})
	if err != nil {
		return nil
	}
	return raw
}

func settlementAccountQueryFactResource(merchantID int64, subMchID string, latestApplicationNo string, applicationTarget *logic.ExternalPaymentFactApplicationTarget, resp *ospcontracts.SettlementQueryResponse) []byte {
	resource := map[string]any{
		"merchant_id":           merchantID,
		"sub_mch_id":            strings.TrimSpace(subMchID),
		"latest_application_no": strings.TrimSpace(latestApplicationNo),
		"account_type":          strings.TrimSpace(string(resp.AccountType)),
		"account_bank":          strings.TrimSpace(resp.AccountBank),
		"bank_name":             strings.TrimSpace(resp.BankName),
		"bank_branch_id":        strings.TrimSpace(resp.BankBranchID),
		"account_number":        strings.TrimSpace(resp.AccountNumber),
		"verify_result":         strings.TrimSpace(string(resp.VerifyResult)),
		"verify_fail_reason":    strings.TrimSpace(resp.VerifyFailReason),
	}
	if applicationTarget != nil {
		resource["owner_applyment_id"] = applicationTarget.BusinessObjectID
	}
	raw, err := json.Marshal(map[string]any{
		"merchant_id":           resource["merchant_id"],
		"sub_mch_id":            resource["sub_mch_id"],
		"latest_application_no": resource["latest_application_no"],
		"account_type":          resource["account_type"],
		"account_bank":          resource["account_bank"],
		"bank_name":             resource["bank_name"],
		"bank_branch_id":        resource["bank_branch_id"],
		"account_number":        resource["account_number"],
		"verify_result":         resource["verify_result"],
		"verify_fail_reason":    resource["verify_fail_reason"],
		"owner_applyment_id":    resource["owner_applyment_id"],
	})
	if err != nil {
		return nil
	}
	return raw
}

func settlementAccountQueryFactDedupeKey(subMchID string, verifyResult string, latestApplicationNo string, verifyFailReason string) string {
	suffix := strings.TrimSpace(latestApplicationNo)
	if suffix == "" {
		suffix = strings.TrimSpace(verifyFailReason)
	}
	if suffix == "" {
		suffix = "current"
	}
	return fmt.Sprintf("wechat:query:ordinary_service_provider:settlement_account:%s:%s:%s", strings.TrimSpace(subMchID), strings.TrimSpace(verifyResult), suffix)
}

func settlementApplicationQueryFactDedupeKey(subMchID string, applicationNo string, verifyResult string, verifyFinishTime string, verifyFailReason string) string {
	suffix := strings.TrimSpace(verifyFinishTime)
	if suffix == "" {
		suffix = strings.TrimSpace(verifyFailReason)
	}
	if suffix == "" {
		suffix = "current"
	}
	return fmt.Sprintf("wechat:query:ordinary_service_provider:settlement_application:%s:%s:%s:%s", strings.TrimSpace(subMchID), strings.TrimSpace(applicationNo), strings.TrimSpace(verifyResult), suffix)
}

func settlementStringPtr(value string) *string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return nil
	}
	return &trimmedValue
}

func logSettlementAccountQuerySuccess(subjectType string, subjectID int64, subMchID string, verifyResult string, latestApplicationNo string, hasFailReason bool) {
	log.Info().
		Str("subject_type", subjectType).
		Int64("subject_id", subjectID).
		Str("sub_mch_id", strings.TrimSpace(subMchID)).
		Str("verify_result", strings.TrimSpace(verifyResult)).
		Str("latest_application_no", strings.TrimSpace(latestApplicationNo)).
		Bool("has_verify_fail_reason", hasFailReason).
		Msg("query settlement account succeeded")
}

func logSettlementModifySuccess(subjectType string, subjectID int64, subMchID string, applicationNo string) {
	log.Info().
		Str("subject_type", subjectType).
		Int64("subject_id", subjectID).
		Str("sub_mch_id", strings.TrimSpace(subMchID)).
		Str("application_no", strings.TrimSpace(applicationNo)).
		Msg("modify settlement account succeeded")
}

func logSettlementApplicationQuerySuccess(subjectType string, subjectID int64, subMchID string, applicationNo string, verifyResult string, hasFailReason bool) {
	log.Info().
		Str("subject_type", subjectType).
		Int64("subject_id", subjectID).
		Str("sub_mch_id", strings.TrimSpace(subMchID)).
		Str("application_no", strings.TrimSpace(applicationNo)).
		Str("verify_result", strings.TrimSpace(verifyResult)).
		Bool("has_verify_fail_reason", hasFailReason).
		Msg("query settlement application succeeded")
}
