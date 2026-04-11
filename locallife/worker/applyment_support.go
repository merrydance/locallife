package worker

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
)

func mapWechatApplymentStateToStatus(wechatState string) string {
	switch strings.TrimSpace(wechatState) {
	case "APPLYMENT_STATE_EDITTING":
		return "pending"
	case "CHECKING", "ACCOUNT_NEED_VERIFY", "APPLYMENT_STATE_AUDITING", "AUDITING":
		return "auditing"
	case "APPLYMENT_STATE_REJECTED", "REJECTED":
		return "rejected"
	case "APPLYMENT_STATE_TO_BE_CONFIRMED":
		return "to_be_confirmed"
	case "APPLYMENT_STATE_TO_BE_SIGNED", "NEED_SIGN":
		return "to_be_signed"
	case "APPLYMENT_STATE_SIGNING":
		return "signing"
	case "APPLYMENT_STATE_FINISHED", "FINISH":
		return "finish"
	case "APPLYMENT_STATE_FROZEN", "FROZEN":
		return "frozen"
	case "APPLYMENT_STATE_CANCELED", "CANCELED":
		return "canceled"
	default:
		return wechatState
	}
}

func normalizeApplymentFollowUpStatus(status, subMchID string) string {
	if status == "finish" && strings.TrimSpace(subMchID) == "" {
		return "submitted"
	}
	return status
}

func resolveApplymentResultStatus(payload ApplymentResultPayload) string {
	if payload.ApplymentStatus != "" {
		return normalizeApplymentFollowUpStatus(payload.ApplymentStatus, payload.SubMchID)
	}
	return normalizeApplymentFollowUpStatus(mapWechatApplymentStateToStatus(payload.ApplymentState), payload.SubMchID)
}

func applymentStatusNeedsAsyncFollowUp(status string) bool {
	switch status {
	case "finish", "rejected", "to_be_confirmed", "to_be_signed":
		return true
	default:
		return false
	}
}

func applymentStatusNeedsRemoteQuery(status string) bool {
	switch status {
	case "submitted", "auditing", "to_be_confirmed", "to_be_signed", "signing":
		return true
	default:
		return false
	}
}

func getRejectReasonFromApplymentAuditDetail(details []wechat.ApplymentAuditDetail) pgtype.Text {
	if len(details) == 0 {
		return pgtype.Text{}
	}

	parts := make([]string, 0, len(details))
	for _, detail := range details {
		parts = append(parts, fmt.Sprintf("%s: %s", detail.ParamName, detail.RejectReason))
	}

	return pgtype.Text{String: strings.Join(parts, "; "), Valid: true}
}

func textValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func buildApplymentResultPayload(record db.EcommerceApplymentPendingFollowUp, applymentState, applymentStatus, subMchID string) *ApplymentResultPayload {
	return &ApplymentResultPayload{
		ApplymentID:     record.ID,
		OutRequestNo:    record.OutRequestNo,
		ApplymentState:  applymentState,
		ApplymentStatus: normalizeApplymentFollowUpStatus(applymentStatus, subMchID),
		SubMchID:        strings.TrimSpace(subMchID),
		SubjectType:     record.SubjectType,
		SubjectID:       record.SubjectID,
	}
}
