package api

import (
	"strings"
	"time"

	"github.com/merrydance/locallife/logic"
)

func buildMerchantOCRConfirmation(userID int64, now time.Time, snapshot map[string]string) *OCRConfirmation {
	return &OCRConfirmation{
		ConfirmedBy: userID,
		ConfirmedAt: now.Format(time.RFC3339),
		Source:      "merchant",
		Snapshot:    snapshot,
	}
}

func merchantBusinessLicenseReviewOCRConfirmationValid(userID int64, data logic.MerchantReviewBusinessLicenseOCRData) bool {
	snapshot := merchantBusinessLicenseReviewOCRSnapshot(data)
	if snapshot["enterprise_name"] == "" || (snapshot["credit_code"] == "" && snapshot["reg_num"] == "") {
		return false
	}
	return merchantReviewOCRConfirmationSnapshotMatches(userID, data.Confirmation, snapshot)
}

func merchantFoodPermitReviewOCRConfirmationValid(userID int64, data logic.MerchantReviewFoodPermitOCRData) bool {
	snapshot := merchantFoodPermitReviewOCRSnapshot(data)
	if snapshot["company_name"] == "" || snapshot["permit_no"] == "" {
		return false
	}
	return merchantReviewOCRConfirmationSnapshotMatches(userID, data.Confirmation, snapshot)
}

func merchantBusinessLicenseReviewOCRSnapshot(data logic.MerchantReviewBusinessLicenseOCRData) map[string]string {
	return map[string]string{
		"enterprise_name":      strings.TrimSpace(data.EnterpriseName),
		"credit_code":          strings.TrimSpace(data.CreditCode),
		"reg_num":              strings.TrimSpace(data.RegNum),
		"legal_representative": strings.TrimSpace(data.LegalRepresentative),
		"address":              strings.TrimSpace(data.Address),
		"business_scope":       strings.TrimSpace(data.BusinessScope),
		"valid_period":         strings.TrimSpace(data.ValidPeriod),
	}
}

func merchantFoodPermitReviewOCRSnapshot(data logic.MerchantReviewFoodPermitOCRData) map[string]string {
	return map[string]string{
		"permit_no":     strings.TrimSpace(data.PermitNo),
		"company_name":  strings.TrimSpace(data.CompanyName),
		"operator_name": strings.TrimSpace(data.OperatorName),
		"valid_from":    strings.TrimSpace(data.ValidFrom),
		"valid_to":      strings.TrimSpace(data.ValidTo),
	}
}

func merchantReviewOCRConfirmationSnapshotMatches(userID int64, confirmation *logic.MerchantReviewOCRConfirmation, required map[string]string) bool {
	if confirmation == nil || confirmation.ConfirmedBy != userID || strings.TrimSpace(confirmation.Source) != "merchant" {
		return false
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(confirmation.ConfirmedAt)); err != nil {
		return false
	}
	if len(required) == 0 || len(confirmation.Snapshot) == 0 {
		return false
	}
	for field, current := range required {
		if normalizeMerchantOCRConfirmationValue(field, confirmation.Snapshot[field]) != normalizeMerchantOCRConfirmationValue(field, current) {
			return false
		}
	}
	return true
}

func normalizeMerchantOCRConfirmationValue(field string, value string) string {
	switch field {
	case "credit_code", "reg_num", "permit_no":
		return merchantNormalizeBusinessCreditCode(value)
	default:
		return strings.TrimSpace(value)
	}
}
