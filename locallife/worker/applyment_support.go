package worker

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
)

func normalizeApplymentSignState(signState string) string {
	return strings.ToUpper(strings.TrimSpace(signState))
}

func applymentNeedsSignFollowUp(status, signState string) bool {
	if normalizeApplymentSignState(signState) != "UNSIGNED" {
		return false
	}

	switch status {
	case "submitted", "checking", "auditing", "signing", "to_be_signed":
		return true
	default:
		return false
	}
}

func mapWechatApplymentStateToStatus(wechatState string) string {
	switch strings.TrimSpace(wechatState) {
	case "APPLYMENT_STATE_EDITTING":
		return "pending"
	case "CHECKING":
		return "checking"
	case "ACCOUNT_NEED_VERIFY":
		return "account_need_verify"
	case "APPLYMENT_STATE_AUDITING", "AUDITING":
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
		return ""
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
		status := normalizeApplymentFollowUpStatus(payload.ApplymentStatus, payload.SubMchID)
		if applymentNeedsSignFollowUp(status, payload.SignState) {
			return "to_be_signed"
		}
		return status
	}
	mappedStatus := mapWechatApplymentStateToStatus(payload.ApplymentState)
	if mappedStatus == "" {
		return ""
	}
	status := normalizeApplymentFollowUpStatus(mappedStatus, payload.SubMchID)
	if applymentNeedsSignFollowUp(status, payload.SignState) {
		return "to_be_signed"
	}
	return status
}

func applymentStatusNeedsAsyncFollowUp(status, signState string) bool {
	if applymentNeedsSignFollowUp(status, signState) {
		return true
	}

	switch status {
	case "finish", "rejected", "account_need_verify", "to_be_confirmed", "to_be_signed":
		return true
	default:
		return false
	}
}

func applymentStatusNeedsRemoteQuery(status string) bool {
	switch status {
	case "submitted", "checking", "auditing", "account_need_verify", "to_be_confirmed", "to_be_signed", "signing":
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

func buildApplymentResultPayload(record db.EcommerceApplymentPendingFollowUp, applymentState, applymentStatus, signState, subMchID string) *ApplymentResultPayload {
	return &ApplymentResultPayload{
		ApplymentID:     record.ID,
		OutRequestNo:    record.OutRequestNo,
		ApplymentState:  applymentState,
		ApplymentStatus: normalizeApplymentFollowUpStatus(applymentStatus, subMchID),
		SignState:       strings.TrimSpace(signState),
		SubMchID:        strings.TrimSpace(subMchID),
		SubjectType:     record.SubjectType,
		SubjectID:       record.SubjectID,
	}
}
