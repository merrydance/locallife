package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
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
