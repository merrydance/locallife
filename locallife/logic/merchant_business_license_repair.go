package logic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type merchantRawJSONFieldCollector map[string]string

func merchantExtractBusinessLicenseValidPeriodFromRawResult(rawResult []byte) string {
	if len(rawResult) == 0 {
		return ""
	}

	fields := merchantCollectRawJSONStringFields(rawResult)
	validPeriod := merchantFirstRawJSONField(fields, "valid_period", "validperiod", "period")
	if strings.TrimSpace(validPeriod) != "" {
		return merchantNormalizeBusinessLicenseValidPeriod(validPeriod)
	}

	return merchantBuildBusinessLicenseValidPeriod(
		merchantFirstRawJSONField(fields, "validfromdate", "valid_from_date", "registrationdate", "registration_date", "validfrom", "valid_from"),
		merchantFirstRawJSONField(fields, "validtodate", "valid_to_date", "validto", "valid_to"),
	)
}

func merchantBuildBusinessLicenseValidPeriod(validFromDate string, validToDate string) string {
	formattedFrom := merchantNormalizeBusinessLicenseDate(validFromDate)
	formattedTo := merchantNormalizeBusinessLicenseDate(validToDate)

	switch {
	case formattedFrom == "" && formattedTo == "":
		return ""
	case formattedTo == "长期":
		if formattedFrom != "" {
			return formattedFrom + "至长期"
		}
		return "长期"
	case formattedFrom != "" && formattedTo != "":
		return formattedFrom + "至" + formattedTo
	case formattedTo != "":
		return formattedTo
	default:
		return "长期"
	}
}

func repairMerchantBusinessLicenseReadinessAfterValidPeriod(businessLicense *MerchantReviewBusinessLicenseOCRData) {
	if businessLicense == nil || businessLicense.Readiness == nil || strings.TrimSpace(businessLicense.ValidPeriod) == "" {
		return
	}

	businessLicense.Readiness.MissingFields = merchantRemoveReadinessField(businessLicense.Readiness.MissingFields, "valid_period")
	businessLicense.Readiness.UnparseableFields = merchantRemoveReadinessField(businessLicense.Readiness.UnparseableFields, "valid_period")
	if len(businessLicense.Readiness.MissingFields) == 0 && len(businessLicense.Readiness.UnparseableFields) == 0 {
		businessLicense.Readiness.State = merchantOCRReadinessStateReady
		businessLicense.Readiness.ReasonCode = "ok"
	}
}

func merchantRemoveReadinessField(fields []string, target string) []string {
	if len(fields) == 0 {
		return fields
	}
	filtered := fields[:0]
	for _, field := range fields {
		if field != target {
			filtered = append(filtered, field)
		}
	}
	return filtered
}

func merchantNormalizeBusinessLicenseValidPeriod(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "至") {
		return trimmed
	}
	if strings.Contains(trimmed, "长期") || strings.Contains(trimmed, "永久") {
		return "长期"
	}
	if formatted, ok := merchantParseBusinessLicenseDate(trimmed); ok {
		return formatted
	}
	return trimmed
}

func merchantNormalizeBusinessLicenseDate(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "长期") || strings.Contains(trimmed, "永久") {
		return "长期"
	}
	if trimmed == "29991231" || trimmed == "99991231" {
		return "长期"
	}
	if formatted, ok := merchantParseBusinessLicenseDate(trimmed); ok {
		return formatted
	}
	return trimmed
}

func merchantParseBusinessLicenseDate(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	if len(trimmed) == 8 {
		allDigits := true
		for _, r := range trimmed {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			if parsed, err := time.Parse("20060102", trimmed); err == nil {
				return parsed.Format("2006年01月02日"), true
			}
		}
	}
	normalized := strings.NewReplacer(
		"年", "-",
		"月", "-",
		"日", "",
		"/", "-",
		".", "-",
		" ", "",
	).Replace(trimmed)
	parsed, err := time.Parse("2006-1-2", normalized)
	if err != nil {
		return "", false
	}
	return parsed.Format("2006年01月02日"), true
}

func merchantCollectRawJSONStringFields(rawResult []byte) merchantRawJSONFieldCollector {
	if len(rawResult) == 0 {
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(rawResult))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return nil
	}

	fields := make(merchantRawJSONFieldCollector)
	merchantCollectRawJSONFieldValues("", payload, fields)
	return fields
}

func merchantCollectRawJSONFieldValues(prefix string, value any, fields merchantRawJSONFieldCollector) {
	switch v := value.(type) {
	case map[string]any:
		for key, nested := range v {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			merchantCollectRawJSONFieldValues(next, nested, fields)
		}
	case []any:
		for _, item := range v {
			merchantCollectRawJSONFieldValues(prefix, item, fields)
		}
	case string:
		if nested, ok := merchantDecodeRawJSONEmbeddedJSON(v); ok {
			merchantCollectRawJSONFieldValues(prefix, nested, fields)
		}
		merchantStoreRawJSONFieldValue(prefix, v, fields)
	case float64:
		merchantStoreRawJSONFieldValue(prefix, fmt.Sprintf("%v", v), fields)
	case json.Number:
		merchantStoreRawJSONFieldValue(prefix, v.String(), fields)
	case bool:
		merchantStoreRawJSONFieldValue(prefix, fmt.Sprintf("%t", v), fields)
	}
}

func merchantDecodeRawJSONEmbeddedJSON(value string) (any, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, false
	}
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return nil, false
	}
	var nested any
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&nested); err != nil {
		return nil, false
	}
	return nested, true
}

func merchantStoreRawJSONFieldValue(prefix, value string, fields merchantRawJSONFieldCollector) {
	value = strings.TrimSpace(value)
	if prefix == "" || value == "" {
		return
	}
	fullKey := merchantNormalizeRawJSONFieldKey(prefix)
	if fullKey != "" && fields[fullKey] == "" {
		fields[fullKey] = value
	}
	segments := strings.Split(fullKey, ".")
	leafKey := segments[len(segments)-1]
	if leafKey != "" && fields[leafKey] == "" {
		fields[leafKey] = value
	}
}

func merchantNormalizeRawJSONFieldKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	replacer := strings.NewReplacer("-", "_", " ", "_", "[", ".", "]", "", "__", "_")
	key = replacer.Replace(key)
	return strings.Trim(key, "._")
}

func merchantFirstRawJSONField(fields merchantRawJSONFieldCollector, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fields[merchantNormalizeRawJSONFieldKey(key)]); value != "" {
			return value
		}
	}
	return ""
}
