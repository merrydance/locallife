package api

import (
	"encoding/json"
	"strings"
)

// IDCardOCRData 身份证OCR识别数据
type IDCardOCRData struct {
	Status         string        `json:"status,omitempty"`
	Error          string        `json:"error,omitempty"`
	ErrorCode      string        `json:"error_code,omitempty"`
	AlertEmittedAt string        `json:"alert_emitted_at,omitempty"`
	Readiness      *OCRReadiness `json:"readiness,omitempty"`
	QueuedAt       string        `json:"queued_at,omitempty"`
	StartedAt      string        `json:"started_at,omitempty"`
	OCRJobID       *int64        `json:"ocr_job_id,omitempty"`
	Name           string        `json:"name,omitempty"`        // 姓名
	IDNumber       string        `json:"id_number,omitempty"`   // 身份证号
	Gender         string        `json:"gender,omitempty"`      // 性别
	Nation         string        `json:"nation,omitempty"`      // 民族
	Address        string        `json:"address,omitempty"`     // 地址
	ValidStart     string        `json:"valid_start,omitempty"` // 有效期起始
	ValidEnd       string        `json:"valid_end,omitempty"`   // 有效期截止（"长期" 或日期）
	OCRAt          string        `json:"ocr_at,omitempty"`      // OCR识别时间
}

// HealthCertOCRData 健康证OCR识别数据
type HealthCertOCRData struct {
	Status         string         `json:"status,omitempty"`
	Error          string         `json:"error,omitempty"`
	ErrorCode      string         `json:"error_code,omitempty"`
	AlertEmittedAt string         `json:"alert_emitted_at,omitempty"`
	Readiness      *OCRReadiness  `json:"readiness,omitempty"`
	Correction     *OCRCorrection `json:"correction,omitempty"`
	QueuedAt       string         `json:"queued_at,omitempty"`
	StartedAt      string         `json:"started_at,omitempty"`
	OCRJobID       *int64         `json:"ocr_job_id,omitempty"`
	Name           string         `json:"name,omitempty"`        // 姓名
	IDNumber       string         `json:"id_number,omitempty"`   // 身份证号
	CertNumber     string         `json:"cert_number,omitempty"` // 证书编号
	ValidStart     string         `json:"valid_start,omitempty"` // 有效期起始
	ValidEnd       string         `json:"valid_end,omitempty"`   // 有效期截止
	OCRAt          string         `json:"ocr_at,omitempty"`      // OCR识别时间
}

type OCRCorrection struct {
	CorrectedBy int64             `json:"corrected_by,omitempty"`
	CorrectedAt string            `json:"corrected_at,omitempty"`
	Source      string            `json:"source,omitempty"`
	Fields      []string          `json:"fields,omitempty"`
	Previous    map[string]string `json:"previous,omitempty"`
}

type OCRConfirmation struct {
	ConfirmedBy int64             `json:"confirmed_by,omitempty"`
	ConfirmedAt string            `json:"confirmed_at,omitempty"`
	Source      string            `json:"source,omitempty"`
	Snapshot    map[string]string `json:"snapshot,omitempty"`
}

func decodeOCRPayload(data []byte, target any) error {
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, target); err == nil {
		return nil
	}

	var embedded string
	if err := json.Unmarshal(data, &embedded); err != nil {
		return err
	}
	if strings.TrimSpace(embedded) == "" {
		return nil
	}
	return json.Unmarshal([]byte(embedded), target)
}

func decodeIDCardOCRData(data []byte) (*IDCardOCRData, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var payload IDCardOCRData
	if err := decodeOCRPayload(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func decodeHealthCertOCRData(data []byte) (*HealthCertOCRData, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var payload HealthCertOCRData
	if err := decodeOCRPayload(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func normalizePersonName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "\t", "")
	return name
}

func buildHealthCertOCRReadinessForAPI(name string, validEnd string) *OCRReadiness {
	required := []string{"name", "valid_end"}
	missing := make([]string, 0, len(required))
	if strings.TrimSpace(name) == "" {
		missing = append(missing, "name")
	}
	if strings.TrimSpace(validEnd) == "" {
		missing = append(missing, "valid_end")
	}
	if len(missing) == 0 {
		return &OCRReadiness{
			State:          ocrReadinessStateReady,
			ReasonCode:     "ok",
			RequiredFields: required,
		}
	}
	return &OCRReadiness{
		State:          ocrReadinessStatePartial,
		ReasonCode:     ocrReadinessReasonRequiredFieldMissing,
		RequiredFields: required,
		MissingFields:  missing,
	}
}
