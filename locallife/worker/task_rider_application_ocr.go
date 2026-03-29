package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
	"github.com/rs/zerolog/log"
)

const (
	TaskRiderApplicationIDCardOCR     = "rider_application:ocr_id_card"
	TaskRiderApplicationHealthCertOCR = "rider_application:ocr_health_cert"
)

type riderApplicationOCRPayload struct {
	ApplicationID int64  `json:"application_id"`
	MediaAssetID  int64  `json:"media_asset_id,omitempty"`
	OCRJobID      int64  `json:"ocr_job_id,omitempty"`
	Side          string `json:"side,omitempty"`
}

type riderIDCardOCRData struct {
	Status         string `json:"status,omitempty"`
	Error          string `json:"error,omitempty"`
	ErrorCode      string `json:"error_code,omitempty"`
	AlertEmittedAt string `json:"alert_emitted_at,omitempty"`
	QueuedAt       string `json:"queued_at,omitempty"`
	StartedAt      string `json:"started_at,omitempty"`
	OCRJobID       *int64 `json:"ocr_job_id,omitempty"`
	Name           string `json:"name,omitempty"`
	IDNumber       string `json:"id_number,omitempty"`
	Gender         string `json:"gender,omitempty"`
	Nation         string `json:"nation,omitempty"`
	Address        string `json:"address,omitempty"`
	ValidStart     string `json:"valid_start,omitempty"`
	ValidEnd       string `json:"valid_end,omitempty"`
	OCRAt          string `json:"ocr_at,omitempty"`
}

type riderHealthCertOCRData struct {
	Status         string `json:"status,omitempty"`
	Error          string `json:"error,omitempty"`
	ErrorCode      string `json:"error_code,omitempty"`
	AlertEmittedAt string `json:"alert_emitted_at,omitempty"`
	QueuedAt       string `json:"queued_at,omitempty"`
	StartedAt      string `json:"started_at,omitempty"`
	OCRJobID       *int64 `json:"ocr_job_id,omitempty"`
	Name           string `json:"name,omitempty"`
	IDNumber       string `json:"id_number,omitempty"`
	CertNumber     string `json:"cert_number,omitempty"`
	ValidStart     string `json:"valid_start,omitempty"`
	ValidEnd       string `json:"valid_end,omitempty"`
	OCRAt          string `json:"ocr_at,omitempty"`
}

func decodeWorkerOCRPayload(data []byte, target any) error {
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

func readRiderIDCardOCR(data []byte) riderIDCardOCRData {
	var result riderIDCardOCRData
	if len(data) == 0 {
		return result
	}
	_ = decodeWorkerOCRPayload(data, &result)
	return result
}

func readRiderHealthCertOCR(data []byte) riderHealthCertOCRData {
	var result riderHealthCertOCRData
	if len(data) == 0 {
		return result
	}
	_ = decodeWorkerOCRPayload(data, &result)
	return result
}

func normalizeRiderOCRDateText(value string) string {
	value = strings.ReplaceAll(value, " 年", "年")
	value = strings.ReplaceAll(value, "年 ", "年")
	value = strings.ReplaceAll(value, " 月", "月")
	value = strings.ReplaceAll(value, "月 ", "月")
	value = strings.ReplaceAll(value, " 日", "日")
	value = strings.ReplaceAll(value, "日 ", "日")
	separatorSpaceRegex := regexp.MustCompile(`\s*([./-])\s*`)
	value = separatorSpaceRegex.ReplaceAllString(value, "$1")
	return strings.TrimSpace(value)
}

func applyRiderHealthCertValidPeriod(data *riderHealthCertOCRData, raw string) {
	normalized := normalizeRiderOCRDateText(raw)
	if normalized == "" {
		return
	}

	datePattern := `\d{4}\s*(?:年|[./-])\s*\d{1,2}\s*(?:月|[./-])\s*\d{1,2}\s*日?`
	validRangeRegex := regexp.MustCompile(`(` + datePattern + `)\s*(?:至|到|-|—|~|～)\s*(` + datePattern + `|长期)`)
	if match := validRangeRegex.FindStringSubmatch(normalized); len(match) > 2 {
		data.ValidStart = normalizeRiderOCRDateText(match[1])
		data.ValidEnd = normalizeRiderOCRDateText(match[2])
		return
	}

	if strings.Contains(normalized, "长期") {
		data.ValidEnd = "长期"
		return
	}

	data.ValidEnd = normalized
}

func maskRiderOCRPreview(value string) string {
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

func maskRiderOCRIDPreview(value string) string {
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

func parseRiderHealthCertOCRText(data *riderHealthCertOCRData, text string) {
	idRegex := regexp.MustCompile(`\b\d{17}[0-9Xx]\b`)
	if match := idRegex.FindString(text); match != "" {
		data.IDNumber = strings.ToUpper(match)
	}
	nameRegex := regexp.MustCompile(`(?m)(?:从业人员姓名|持证人|体检者|姓名)\s*[:：]?\s*([^\n\r\s]{2,20})`)
	if match := nameRegex.FindStringSubmatch(text); len(match) > 1 {
		data.Name = strings.TrimSpace(match[1])
	}
	certRegex := regexp.MustCompile(`(?m)(?:健康证号|证书编号|证号|编号)\s*[:：]?\s*([A-Za-z0-9\-]{5,})`)
	if match := certRegex.FindStringSubmatch(text); len(match) > 1 {
		data.CertNumber = strings.TrimSpace(match[1])
	}
	datePattern := `\d{4}\s*(?:年|[./-])\s*\d{1,2}\s*(?:月|[./-])\s*\d{1,2}\s*日?`
	validToPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:有效期至|有效期限至|有效截止日期|截止日期|截止到|到期日期|到期日|有效日期至|有效期到)\s*[:：]?\s*(` + datePattern + `|长期)`),
		regexp.MustCompile(`(?:有效日期|有效期|有效期限)\s*[:：]?\s*` + datePattern + `\s*(?:至|到|-|—|~|～)\s*(` + datePattern + `|长期)`),
	}
	for _, validToRegex := range validToPatterns {
		if match := validToRegex.FindStringSubmatch(text); len(match) > 1 {
			applyRiderHealthCertValidPeriod(data, match[1])
			break
		}
	}
	validRangeRegex := regexp.MustCompile(`(` + datePattern + `)\s*(?:至|到|-|—|~|～)\s*(` + datePattern + `|长期)`)
	if match := validRangeRegex.FindStringSubmatch(text); len(match) > 2 {
		data.ValidStart = normalizeRiderOCRDateText(match[1])
		data.ValidEnd = normalizeRiderOCRDateText(match[2])
	}
	if data.ValidEnd == "" && strings.Contains(text, "长期") {
		data.ValidEnd = "长期"
	}
}

func (distributor *RedisTaskDistributor) DistributeTaskRiderApplicationIDCardOCR(
	ctx context.Context,
	applicationID int64,
	mediaAssetID int64,
	ocrJobID int64,
	side string,
	opts ...asynq.Option,
) error {
	payloadBytes, err := json.Marshal(riderApplicationOCRPayload{
		ApplicationID: applicationID,
		MediaAssetID:  mediaAssetID,
		OCRJobID:      ocrJobID,
		Side:          side,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return distributor.enqueue(ctx, TaskRiderApplicationIDCardOCR, payloadBytes, withDefaultOCRTaskOptions(opts...)...)
}

func (processor *RedisTaskProcessor) ProcessTaskRiderApplicationIDCardOCR(ctx context.Context, task *asynq.Task) error {
	var payload riderApplicationOCRPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}
	if payload.ApplicationID <= 0 || payload.OCRJobID <= 0 || (payload.Side != "Front" && payload.Side != "Back") {
		return fmt.Errorf("invalid rider id card payload: %w", asynq.SkipRetry)
	}
	if processor.ocrService == nil {
		return fmt.Errorf("ocr service not configured for rider id card: %w", asynq.SkipRetry)
	}

	job, err := processor.ocrService.ExecuteJob(ctx, ocr.ExecuteJobParams{
		JobID:      payload.OCRJobID,
		LeaseOwner: "worker:rider_id_card",
	})
	if err != nil {
		alertEmittedAt := processor.publishOCRFailureAlert(ctx, job, err)
		app, getErr := processor.store.GetRiderApplication(ctx, payload.ApplicationID)
		if getErr == nil {
			ocrData := readRiderIDCardOCR(app.IDCardOcr)
			ocrData.Status = string(ocr.JobStatusFailed)
			ocrData.Error = err.Error()
			ocrData.ErrorCode = ocr.ErrorCode(err)
			ocrData.AlertEmittedAt = formatOCRAlertEmittedAt(alertEmittedAt)
			ocrData.QueuedAt = job.CreatedAt.Format(time.RFC3339)
			ocrData.StartedAt = formatPgTimestamp(job.StartedAt)
			ocrData.OCRJobID = int64Ptr(payload.OCRJobID)
			failedJSON, _ := json.Marshal(ocrData)
			_, _ = processor.store.UpdateRiderApplicationIDCard(ctx, db.UpdateRiderApplicationIDCardParams{
				ID:        payload.ApplicationID,
				IDCardOcr: failedJSON,
			})
		}
		processor.writeOCRJobAudit(ctx, job, "ocr_job_failed", ocrFailureAuditMetadata(err))
		return asynqOCRTaskError(job, err)
	}
	observeOCRJob(job)

	normalized, decodeErr := ocr.UnmarshalNormalizedResult(job.NormalizedResult)
	if decodeErr != nil {
		return fmt.Errorf("decode normalized rider id card result: %w", decodeErr)
	}

	app, err := processor.store.GetRiderApplication(ctx, payload.ApplicationID)
	if err != nil {
		return fmt.Errorf("get rider application: %w", err)
	}

	ocrData := readRiderIDCardOCR(app.IDCardOcr)
	ocrData.Status = "done"
	ocrData.Error = ""
	ocrData.QueuedAt = job.CreatedAt.Format(time.RFC3339)
	ocrData.StartedAt = formatPgTimestamp(job.StartedAt)
	ocrData.OCRJobID = int64Ptr(job.ID)
	ocrData.OCRAt = normalized.RecognizedAt.Format(time.RFC3339)

	arg := db.UpdateRiderApplicationIDCardParams{ID: payload.ApplicationID}
	if normalized.IDCard != nil {
		if payload.Side == "Front" {
			ocrData.Name = normalized.IDCard.Name
			ocrData.IDNumber = normalized.IDCard.IDNumber
			ocrData.Gender = normalized.IDCard.Gender
			ocrData.Nation = normalized.IDCard.Ethnicity
			ocrData.Address = normalized.IDCard.Address
			if normalized.IDCard.Name != "" {
				arg.RealName = pgtype.Text{String: normalized.IDCard.Name, Valid: true}
			}
		} else {
			ocrData.ValidEnd = normalized.IDCard.ValidPeriod
		}
	}

	ocrJSON, _ := json.Marshal(ocrData)
	arg.IDCardOcr = ocrJSON
	_, err = processor.store.UpdateRiderApplicationIDCard(ctx, arg)
	if err != nil {
		return fmt.Errorf("update rider application id card: %w", err)
	}
	processor.writeOCRJobAudit(ctx, job, "ocr_job_succeeded", map[string]any{"status": job.Status})

	log.Info().Int64("application_id", payload.ApplicationID).Int64("ocr_job_id", job.ID).Str("side", payload.Side).Msg("✅ rider id card OCR updated from ocr job")
	return nil
}

func (distributor *RedisTaskDistributor) DistributeTaskRiderApplicationHealthCertOCR(
	ctx context.Context,
	applicationID int64,
	mediaAssetID int64,
	ocrJobID int64,
	opts ...asynq.Option,
) error {
	payloadBytes, err := json.Marshal(riderApplicationOCRPayload{
		ApplicationID: applicationID,
		MediaAssetID:  mediaAssetID,
		OCRJobID:      ocrJobID,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return distributor.enqueue(ctx, TaskRiderApplicationHealthCertOCR, payloadBytes, withDefaultOCRTaskOptions(opts...)...)
}

func (processor *RedisTaskProcessor) ProcessTaskRiderApplicationHealthCertOCR(ctx context.Context, task *asynq.Task) error {
	var payload riderApplicationOCRPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}
	if payload.ApplicationID <= 0 || payload.OCRJobID <= 0 {
		return fmt.Errorf("invalid rider health cert payload: %w", asynq.SkipRetry)
	}
	if processor.ocrService == nil {
		return fmt.Errorf("ocr service not configured for rider health cert: %w", asynq.SkipRetry)
	}

	job, err := processor.ocrService.ExecuteJob(ctx, ocr.ExecuteJobParams{JobID: payload.OCRJobID, LeaseOwner: "worker:rider_health_cert"})
	if err != nil {
		alertEmittedAt := processor.publishOCRFailureAlert(ctx, job, err)
		app, getErr := processor.store.GetRiderApplication(ctx, payload.ApplicationID)
		if getErr == nil {
			ocrData := readRiderHealthCertOCR(app.HealthCertOcr)
			ocrData.Status = string(ocr.JobStatusFailed)
			ocrData.Error = err.Error()
			ocrData.ErrorCode = ocr.ErrorCode(err)
			ocrData.AlertEmittedAt = formatOCRAlertEmittedAt(alertEmittedAt)
			ocrData.QueuedAt = job.CreatedAt.Format(time.RFC3339)
			ocrData.StartedAt = formatPgTimestamp(job.StartedAt)
			ocrData.OCRJobID = int64Ptr(payload.OCRJobID)
			failedJSON, _ := json.Marshal(ocrData)
			_, _ = processor.store.UpdateRiderApplicationHealthCert(ctx, db.UpdateRiderApplicationHealthCertParams{ID: payload.ApplicationID, HealthCertOcr: failedJSON})
		}
		processor.writeOCRJobAudit(ctx, job, "ocr_job_failed", ocrFailureAuditMetadata(err))
		return asynqOCRTaskError(job, err)
	}
	observeOCRJob(job)

	normalized, decodeErr := ocr.UnmarshalNormalizedResult(job.NormalizedResult)
	if decodeErr != nil {
		return fmt.Errorf("decode normalized rider health cert result: %w", decodeErr)
	}
	app, err := processor.store.GetRiderApplication(ctx, payload.ApplicationID)
	if err != nil {
		return fmt.Errorf("get rider application: %w", err)
	}
	ocrData := readRiderHealthCertOCR(app.HealthCertOcr)
	ocrData.Status = "done"
	ocrData.Error = ""
	ocrData.QueuedAt = job.CreatedAt.Format(time.RFC3339)
	ocrData.StartedAt = formatPgTimestamp(job.StartedAt)
	ocrData.OCRJobID = int64Ptr(job.ID)
	ocrData.OCRAt = normalized.RecognizedAt.Format(time.RFC3339)
	if normalized.HealthCert != nil {
		if normalized.HealthCert.RawText != "" {
			parseRiderHealthCertOCRText(&ocrData, normalized.HealthCert.RawText)
		}
		if normalized.HealthCert.Name != "" {
			ocrData.Name = normalized.HealthCert.Name
		}
		if normalized.HealthCert.Certificate != "" {
			ocrData.CertNumber = normalized.HealthCert.Certificate
		}
		if normalized.HealthCert.ValidPeriod != "" {
			applyRiderHealthCertValidPeriod(&ocrData, normalized.HealthCert.ValidPeriod)
		}
	} else if normalized.FoodPermit != nil && normalized.FoodPermit.RawText != "" {
		parseRiderHealthCertOCRText(&ocrData, normalized.FoodPermit.RawText)
	}
	validPeriodRaw := ""
	if normalized.HealthCert != nil {
		validPeriodRaw = normalized.HealthCert.ValidPeriod
	}
	log.Info().
		Int64("application_id", payload.ApplicationID).
		Int64("ocr_job_id", job.ID).
		Str("provider", job.Provider).
		Bool("has_health_cert_result", normalized.HealthCert != nil).
		Bool("used_food_permit_fallback", normalized.HealthCert == nil && normalized.FoodPermit != nil && normalized.FoodPermit.RawText != "").
		Bool("has_raw_text", normalized.HealthCert != nil && normalized.HealthCert.RawText != "").
		Bool("has_name", ocrData.Name != "").
		Bool("has_cert_number", ocrData.CertNumber != "").
		Bool("has_valid_end", ocrData.ValidEnd != "").
		Str("valid_period_raw", truncateString(normalizeRiderOCRDateText(validPeriodRaw), 80)).
		Str("name_preview", maskRiderOCRPreview(ocrData.Name)).
		Str("cert_number_preview", maskRiderOCRPreview(ocrData.CertNumber)).
		Str("id_number_preview", maskRiderOCRIDPreview(ocrData.IDNumber)).
		Str("valid_end_preview", truncateString(ocrData.ValidEnd, 80)).
		Msg("rider health cert OCR normalized")
	ocrJSON, _ := json.Marshal(ocrData)
	_, err = processor.store.UpdateRiderApplicationHealthCert(ctx, db.UpdateRiderApplicationHealthCertParams{ID: payload.ApplicationID, HealthCertOcr: ocrJSON})
	if err != nil {
		return fmt.Errorf("update rider application health cert: %w", err)
	}
	processor.writeOCRJobAudit(ctx, job, "ocr_job_succeeded", map[string]any{"status": job.Status})
	log.Info().Int64("application_id", payload.ApplicationID).Int64("ocr_job_id", job.ID).Msg("✅ rider health cert OCR updated from ocr job")
	return nil
}
