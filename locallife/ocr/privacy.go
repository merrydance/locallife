package ocr

import (
	"encoding/json"
	"strings"
	"time"
)

const idCardRawResultRetention = 7 * 24 * time.Hour

var idCardRawResultMaskers = map[string]func(string) string{
	"name":             maskPersonName,
	"fullname":         maskPersonName,
	"id":               maskIDNumber,
	"id_number":        maskIDNumber,
	"idnumber":         maskIDNumber,
	"id_num":           maskIDNumber,
	"idnum":            maskIDNumber,
	"address":          maskAddress,
	"addr":             maskAddress,
	"birth_date":       redactValue,
	"birthdate":        redactValue,
	"birthday":         redactValue,
	"gender":           redactValue,
	"sex":              redactValue,
	"nation":           redactValue,
	"ethnicity":        redactValue,
	"nationality":      redactValue,
	"authority":        redactValue,
	"issueauthority":   redactValue,
	"issuingauthority": redactValue,
	"valid_date":       redactValue,
	"validdate":        redactValue,
	"valid_period":     redactValue,
	"validperiod":      redactValue,
}

func DefaultRetentionUntil(documentType DocumentType, now time.Time) *time.Time {
	if documentType != DocumentTypeIDCard {
		return nil
	}
	retentionUntil := now.UTC().Add(idCardRawResultRetention)
	return &retentionUntil
}

func SanitizeRawResultForStorage(documentType DocumentType, raw json.RawMessage) json.RawMessage {
	if documentType != DocumentTypeIDCard || len(raw) == 0 {
		return raw
	}

	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		fallback, marshalErr := json.Marshal(map[string]any{
			"document_type": documentType,
			"redacted":      true,
		})
		if marshalErr != nil {
			return json.RawMessage(`{"redacted":true}`)
		}
		return fallback
	}

	sanitized, err := json.Marshal(sanitizeIDCardRawValue("", payload))
	if err != nil {
		return json.RawMessage(`{"redacted":true}`)
	}
	return sanitized
}

func sanitizeIDCardRawValue(key string, value any) any {
	switch typed := value.(type) {
	case map[string]any:
		sanitized := make(map[string]any, len(typed))
		for nestedKey, nestedValue := range typed {
			sanitized[nestedKey] = sanitizeIDCardRawValue(nestedKey, nestedValue)
		}
		return sanitized
	case []any:
		sanitized := make([]any, len(typed))
		for index, item := range typed {
			sanitized[index] = sanitizeIDCardRawValue(key, item)
		}
		return sanitized
	case string:
		if masker := idCardMaskerForKey(key); masker != nil {
			return masker(typed)
		}
		return typed
	default:
		return value
	}
}

func idCardMaskerForKey(key string) func(string) string {
	normalizedKey := normalizeAliyunFieldKey(key)
	if masker, ok := idCardRawResultMaskers[normalizedKey]; ok {
		return masker
	}
	segments := strings.Split(normalizedKey, ".")
	leafKey := segments[len(segments)-1]
	return idCardRawResultMaskers[leafKey]
}

func maskPersonName(value string) string {
	runes := []rune(strings.TrimSpace(value))
	switch len(runes) {
	case 0:
		return ""
	case 1:
		return "*"
	case 2:
		return string(runes[0]) + "*"
	default:
		return string(runes[0]) + strings.Repeat("*", len(runes)-2) + string(runes[len(runes)-1])
	}
}

func maskIDNumber(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return ""
	}
	if len(runes) <= 8 {
		if len(runes) == 1 {
			return "*"
		}
		return string(runes[0]) + strings.Repeat("*", len(runes)-2) + string(runes[len(runes)-1])
	}
	return string(runes[:6]) + strings.Repeat("*", len(runes)-10) + string(runes[len(runes)-4:])
}

func maskAddress(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return ""
	}
	if len(runes) <= 6 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:6]) + strings.Repeat("*", len(runes)-6)
}

func redactValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "[REDACTED]"
}
