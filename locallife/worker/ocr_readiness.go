package worker

import "strings"

const (
	ocrReadinessStateReady          = "ready"
	ocrReadinessStatePartial        = "partial"
	ocrReadinessStateProviderFailed = "provider_failed"

	ocrReadinessReasonOK                   = "ok"
	ocrReadinessReasonRequiredFieldMissing = "required_field_missing"
	ocrReadinessReasonProviderError        = "provider_error"
)

type ocrReadiness struct {
	State             string   `json:"state,omitempty"`
	ReasonCode        string   `json:"reason_code,omitempty"`
	RequiredFields    []string `json:"required_fields,omitempty"`
	MissingFields     []string `json:"missing_fields,omitempty"`
	UnparseableFields []string `json:"unparseable_fields,omitempty"`
}

func buildOCRReadiness(requiredFields []string, hasField func(string) bool) *ocrReadiness {
	missing := make([]string, 0, len(requiredFields))
	for _, field := range requiredFields {
		if !hasField(field) {
			missing = append(missing, field)
		}
	}
	if len(missing) == 0 {
		return &ocrReadiness{
			State:          ocrReadinessStateReady,
			ReasonCode:     ocrReadinessReasonOK,
			RequiredFields: append([]string(nil), requiredFields...),
		}
	}
	return &ocrReadiness{
		State:          ocrReadinessStatePartial,
		ReasonCode:     ocrReadinessReasonRequiredFieldMissing,
		RequiredFields: append([]string(nil), requiredFields...),
		MissingFields:  missing,
	}
}

func failedOCRReadiness(errorCode string) *ocrReadiness {
	if strings.TrimSpace(errorCode) == "" {
		errorCode = ocrReadinessReasonProviderError
	}
	return &ocrReadiness{
		State:      ocrReadinessStateProviderFailed,
		ReasonCode: errorCode,
	}
}

func buildMerchantBusinessLicenseReadiness(data map[string]string) *ocrReadiness {
	required := []string{"enterprise_name", "legal_representative", "address", "business_scope", "valid_period"}
	return buildOCRReadiness(required, func(field string) bool {
		return strings.TrimSpace(data[field]) != ""
	})
}

func buildMerchantFoodPermitReadiness(companyName string, operatorName string, rawText string, validTo string) *ocrReadiness {
	required := []string{"company_name", "valid_to"}
	return buildOCRReadiness(required, func(field string) bool {
		switch field {
		case "company_name":
			return strings.TrimSpace(companyName) != "" || strings.TrimSpace(operatorName) != "" || strings.TrimSpace(rawText) != ""
		case "valid_to":
			return strings.TrimSpace(validTo) != ""
		default:
			return false
		}
	})
}

func buildMerchantIDCardReadiness(name string, idNumber string, validDate string, side string) *ocrReadiness {
	required := []string{"name", "id_number"}
	if strings.EqualFold(side, "Back") {
		required = []string{"valid_date"}
	}
	return buildOCRReadiness(required, func(field string) bool {
		switch field {
		case "name":
			return strings.TrimSpace(name) != ""
		case "id_number":
			return strings.TrimSpace(idNumber) != ""
		case "valid_date":
			return strings.TrimSpace(validDate) != ""
		default:
			return false
		}
	})
}

func buildRiderIDCardReadiness(name string, idNumber string, validEnd string) *ocrReadiness {
	required := []string{"name", "id_number", "valid_end"}
	return buildOCRReadiness(required, func(field string) bool {
		switch field {
		case "name":
			return strings.TrimSpace(name) != ""
		case "id_number":
			return strings.TrimSpace(idNumber) != ""
		case "valid_end":
			return strings.TrimSpace(validEnd) != ""
		default:
			return false
		}
	})
}

func buildRiderHealthCertReadiness(name string, validEnd string) *ocrReadiness {
	required := []string{"name", "valid_end"}
	return buildOCRReadiness(required, func(field string) bool {
		switch field {
		case "name":
			return strings.TrimSpace(name) != ""
		case "valid_end":
			return strings.TrimSpace(validEnd) != ""
		default:
			return false
		}
	})
}
