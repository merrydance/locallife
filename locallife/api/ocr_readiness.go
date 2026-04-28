package api

import "errors"

const (
	ocrReadinessStateReady          = "ready"
	ocrReadinessStatePartial        = "partial"
	ocrReadinessStateProviderFailed = "provider_failed"

	ocrReadinessReasonRequiredFieldMissing = "required_field_missing"
	ocrReadinessReasonFieldUnparseable     = "field_unparseable"
)

type OCRReadiness struct {
	State             string   `json:"state,omitempty"`
	ReasonCode        string   `json:"reason_code,omitempty"`
	RequiredFields    []string `json:"required_fields,omitempty"`
	MissingFields     []string `json:"missing_fields,omitempty"`
	UnparseableFields []string `json:"unparseable_fields,omitempty"`
}

func submissionReadinessError(
	readiness *OCRReadiness,
	missingFieldMessages map[string]string,
	fallbackMissing string,
	fallbackUnparseable string,
	fallbackProvider string,
) error {
	if readiness == nil || readiness.State == "" || readiness.State == ocrReadinessStateReady {
		return nil
	}

	switch readiness.State {
	case ocrReadinessStateProviderFailed:
		return errors.New(fallbackProvider)
	case ocrReadinessStatePartial:
		switch readiness.ReasonCode {
		case ocrReadinessReasonRequiredFieldMissing:
			return errors.New(firstReadinessFieldMessage(readiness.MissingFields, missingFieldMessages, fallbackMissing))
		case ocrReadinessReasonFieldUnparseable:
			return errors.New(firstReadinessFieldMessage(readiness.UnparseableFields, missingFieldMessages, fallbackUnparseable))
		default:
			return errors.New(fallbackMissing)
		}
	default:
		return errors.New(fallbackMissing)
	}
}

func firstReadinessFieldMessage(fields []string, messages map[string]string, fallback string) string {
	for _, field := range fields {
		if msg, ok := messages[field]; ok && msg != "" {
			return msg
		}
	}
	return fallback
}
