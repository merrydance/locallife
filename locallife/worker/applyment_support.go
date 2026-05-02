package worker

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	ospcontracts "github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
)

func applymentNeedsSignFollowUp(status, signState string) bool {
	return logic.ApplymentNeedsSignFollowUp(status, signState)
}

func mapWechatApplymentStateToStatus(wechatState string) string {
	return logic.MapWechatApplymentStateToStatus(wechatState)
}

func mapOrdinaryApplymentStateToStatus(state ospcontracts.ApplymentState) string {
	return logic.MapOrdinaryApplymentStateToStatus(state)
}

func normalizeApplymentFollowUpStatus(status, subMchID string) string {
	return logic.NormalizeResolvedApplymentStatus(status, strings.TrimSpace(subMchID) != "")
}

func resolveApplymentResultStatus(payload ApplymentResultPayload) string {
	if payload.ApplymentStatus != "" {
		return logic.NormalizeResolvedApplymentStatus(
			logic.ResolveWechatApplymentStatus(payload.ApplymentStatus, payload.ApplymentState, payload.SignState),
			strings.TrimSpace(payload.SubMchID) != "",
		)
	}
	mappedStatus := mapWechatApplymentStateToStatus(payload.ApplymentState)
	if mappedStatus == "" {
		return ""
	}
	return logic.NormalizeResolvedApplymentStatus(
		logic.ResolveWechatApplymentStatus("", payload.ApplymentState, payload.SignState),
		strings.TrimSpace(payload.SubMchID) != "",
	)
}

func applymentStatusNeedsAsyncFollowUp(status, signState string) bool {
	if applymentNeedsSignFollowUp(status, signState) {
		return true
	}

	switch status {
	case "finish", "rejected", "account_need_verify", "to_be_confirmed", "to_be_signed", "frozen", "canceled":
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

func applymentStatusHandledByFact(status, subMchID string) bool {
	if status == "finish" && strings.TrimSpace(subMchID) != "" {
		return true
	}
	switch status {
	case "account_need_verify", "to_be_confirmed", "to_be_signed", "rejected", "frozen", "canceled":
		return true
	default:
		return false
	}
}

func getRejectReasonFromApplymentAuditDetail(details []wechatcontracts.ApplymentAuditDetail) pgtype.Text {
	if len(details) == 0 {
		return pgtype.Text{}
	}

	parts := make([]string, 0, len(details))
	for _, detail := range details {
		parts = append(parts, fmt.Sprintf("%s: %s", detail.ParamName, detail.RejectReason))
	}

	return pgtype.Text{String: strings.Join(parts, "; "), Valid: true}
}

func getRejectReasonFromOrdinaryApplymentAuditDetail(details []ospcontracts.ApplymentAuditDetail) pgtype.Text {
	if len(details) == 0 {
		return pgtype.Text{}
	}

	parts := make([]string, 0, len(details))
	for _, detail := range details {
		fieldName := strings.TrimSpace(detail.FieldName)
		if fieldName == "" {
			fieldName = strings.TrimSpace(detail.Field)
		}
		reason := strings.TrimSpace(detail.RejectReason)
		if reason == "" {
			continue
		}
		if fieldName == "" {
			parts = append(parts, reason)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", fieldName, reason))
	}
	if len(parts) == 0 {
		return pgtype.Text{}
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
