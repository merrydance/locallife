package logic

import "strings"

func NormalizeApplymentSignState(signState string) string {
	return strings.ToUpper(strings.TrimSpace(signState))
}

func MapWechatApplymentStateToStatus(wechatState string) string {
	switch strings.TrimSpace(wechatState) {
	case "APPLYMENT_STATE_EDITTING", "EDITTING":
		return "pending"
	case "PROCESSING":
		return "submitted"
	case "APPLYMENT_STATE_AUDITING", "AUDITING":
		return "auditing"
	case "CHECKING":
		return "checking"
	case "ACCOUNT_NEED_VERIFY":
		return "account_need_verify"
	case "APPLYMENT_STATE_TO_BE_CONFIRMED":
		return "to_be_confirmed"
	case "APPLYMENT_STATE_TO_BE_SIGNED":
		return "to_be_signed"
	case "NEED_SIGN":
		return "to_be_signed"
	case "APPLYMENT_STATE_SIGNING":
		return "signing"
	case "FINISH":
		return "finish"
	case "APPLYMENT_STATE_FINISHED":
		return "finish"
	case "APPLYMENT_STATE_REJECTED", "REJECTED":
		return "rejected"
	case "APPLYMENT_STATE_FROZEN", "FROZEN":
		return "frozen"
	case "APPLYMENT_STATE_CANCELED", "CANCELED":
		return "canceled"
	default:
		return ""
	}
}

func ResolveWechatApplymentStatus(currentStatus, wechatState, signState string) string {
	resolved := MapWechatApplymentStateToStatus(wechatState)
	if resolved == "" {
		resolved = strings.TrimSpace(currentStatus)
	}
	if resolved == "" {
		return ""
	}

	if NormalizeApplymentSignState(signState) == "UNSIGNED" {
		switch resolved {
		case "submitted", "checking", "auditing", "signing", "to_be_signed":
			return "to_be_signed"
		}
	}

	return resolved
}

func NormalizeResolvedApplymentStatus(status string, hasSubMchID bool) string {
	normalized := strings.TrimSpace(status)
	if normalized == "finish" && !hasSubMchID {
		return "submitted"
	}
	return normalized
}

func ApplymentNeedsSignFollowUp(status, signState string) bool {
	if NormalizeApplymentSignState(signState) != "UNSIGNED" {
		return false
	}

	switch strings.TrimSpace(status) {
	case "submitted", "checking", "auditing", "signing", "to_be_signed":
		return true
	default:
		return false
	}
}
