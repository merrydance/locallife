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
	TaskMerchantApplicationBusinessLicenseOCR = "merchant_application:ocr_business_license"
	TaskMerchantApplicationFoodPermitOCR      = "merchant_application:ocr_food_permit"
	TaskMerchantApplicationIDCardOCR          = "merchant_application:ocr_id_card"
)

type merchantApplicationOCRPayload struct {
	ApplicationID int64  `json:"application_id"`
	MediaAssetID  int64  `json:"media_asset_id,omitempty"`
	OCRJobID      int64  `json:"ocr_job_id,omitempty"`
	Side          string `json:"side,omitempty"` // Front/Back
}

func summarizeMerchantBusinessLicenseOCRData(data map[string]any) map[string]any {
	return map[string]any{
		"status":                       stringValueFromMap(data, "status"),
		"enterprise_name_present":      hasStringValueInMap(data, "enterprise_name"),
		"credit_code_present":          hasStringValueInMap(data, "credit_code"),
		"registration_number_present":  hasStringValueInMap(data, "reg_num"),
		"legal_representative_present": hasStringValueInMap(data, "legal_representative"),
		"address_present":              hasStringValueInMap(data, "address"),
		"business_scope_present":       hasStringValueInMap(data, "business_scope"),
		"valid_period_present":         hasStringValueInMap(data, "valid_period"),
	}
}

func summarizeMerchantFoodPermitOCRData(data foodPermitOCRData) map[string]any {
	return map[string]any{
		"status":                data.Status,
		"raw_text_present":      strings.TrimSpace(data.RawText) != "",
		"raw_text_length":       len(strings.TrimSpace(data.RawText)),
		"permit_no_present":     strings.TrimSpace(data.PermitNo) != "",
		"company_name_present":  strings.TrimSpace(data.CompanyName) != "",
		"operator_name_present": strings.TrimSpace(data.OperatorName) != "",
		"valid_from_present":    strings.TrimSpace(data.ValidFrom) != "",
		"valid_to_present":      strings.TrimSpace(data.ValidTo) != "",
	}
}

func summarizeMerchantIDCardOCRData(data merchantIDCardOCRData) map[string]any {
	return map[string]any{
		"status":             data.Status,
		"name_present":       strings.TrimSpace(data.Name) != "",
		"id_number_present":  strings.TrimSpace(data.IDNumber) != "",
		"gender_present":     strings.TrimSpace(data.Gender) != "",
		"nation_present":     strings.TrimSpace(data.Nation) != "",
		"address_present":    strings.TrimSpace(data.Address) != "",
		"valid_date_present": strings.TrimSpace(data.ValidDate) != "",
	}
}

func hasStringValueInMap(data map[string]any, key string) bool {
	return strings.TrimSpace(stringValueFromMap(data, key)) != ""
}

func stringValueFromMap(data map[string]any, key string) string {
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func (distributor *RedisTaskDistributor) DistributeTaskMerchantApplicationBusinessLicenseOCR(
	ctx context.Context,
	applicationID int64,
	mediaAssetID int64,
	ocrJobID int64,
	opts ...asynq.Option,
) error {
	payloadBytes, err := json.Marshal(merchantApplicationOCRPayload{
		ApplicationID: applicationID,
		MediaAssetID:  mediaAssetID,
		OCRJobID:      ocrJobID,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return distributor.enqueue(ctx, TaskMerchantApplicationBusinessLicenseOCR, payloadBytes, withDefaultOCRTaskOptions(opts...)...)
}

func (distributor *RedisTaskDistributor) DistributeTaskMerchantApplicationFoodPermitOCR(
	ctx context.Context,
	applicationID int64,
	mediaAssetID int64,
	ocrJobID int64,
	opts ...asynq.Option,
) error {
	payloadBytes, err := json.Marshal(merchantApplicationOCRPayload{
		ApplicationID: applicationID,
		MediaAssetID:  mediaAssetID,
		OCRJobID:      ocrJobID,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return distributor.enqueue(ctx, TaskMerchantApplicationFoodPermitOCR, payloadBytes, withDefaultOCRTaskOptions(opts...)...)
}

func (distributor *RedisTaskDistributor) DistributeTaskMerchantApplicationIDCardOCR(
	ctx context.Context,
	applicationID int64,
	mediaAssetID int64,
	ocrJobID int64,
	side string,
	opts ...asynq.Option,
) error {
	payloadBytes, err := json.Marshal(merchantApplicationOCRPayload{
		ApplicationID: applicationID,
		MediaAssetID:  mediaAssetID,
		OCRJobID:      ocrJobID,
		Side:          side,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return distributor.enqueue(ctx, TaskMerchantApplicationIDCardOCR, payloadBytes, withDefaultOCRTaskOptions(opts...)...)
}

func (distributor *RedisTaskDistributor) enqueue(ctx context.Context, taskType string, payload []byte, opts ...asynq.Option) error {
	task := asynq.NewTask(taskType, payload, opts...)
	info, err := distributor.enqueueTask(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}
	log.Info().Str("type", task.Type()).Str("queue", info.Queue).Msg("enqueued task")
	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskMerchantApplicationBusinessLicenseOCR(ctx context.Context, task *asynq.Task) error {
	payload, err := processor.parseMerchantApplicationOCRPayload(task)
	if err != nil {
		return err
	}
	if payload.OCRJobID <= 0 {
		return fmt.Errorf("business license ocr task requires ocr_job_id: %w", asynq.SkipRetry)
	}
	if processor.ocrService == nil {
		return fmt.Errorf("ocr service not configured for business license: %w", asynq.SkipRetry)
	}
	return processor.processMerchantApplicationBusinessLicenseOCRJob(ctx, payload)
}

func (processor *RedisTaskProcessor) processMerchantApplicationBusinessLicenseOCRJob(ctx context.Context, payload *merchantApplicationOCRPayload) error {
	job, err := processor.ocrService.ExecuteJob(ctx, ocr.ExecuteJobParams{
		JobID:      payload.OCRJobID,
		LeaseOwner: "worker:merchant_business_license",
	})
	if err != nil {
		alertEmittedAt := processor.publishOCRFailureAlert(ctx, job, err)
		failed := map[string]any{
			"status":           string(ocr.JobStatusFailed),
			"queued_at":        job.CreatedAt.Format(time.RFC3339),
			"started_at":       formatPgTimestamp(job.StartedAt),
			"ocr_job_id":       payload.OCRJobID,
			"error_code":       ocr.ErrorCode(err),
			"alert_emitted_at": formatOCRAlertEmittedAt(alertEmittedAt),
			"error":            err.Error(),
		}
		failedJSON, _ := json.Marshal(failed)
		_, _ = processor.store.UpdateMerchantApplicationBusinessLicense(ctx, db.UpdateMerchantApplicationBusinessLicenseParams{
			ID:                 payload.ApplicationID,
			BusinessLicenseOcr: failedJSON,
		})
		processor.writeOCRJobAudit(ctx, job, "ocr_job_failed", ocrFailureAuditMetadata(err))
		return asynqOCRTaskError(job, err)
	}
	observeOCRJob(job)
	normalized, decodeErr := ocr.UnmarshalNormalizedResult(job.NormalizedResult)
	if decodeErr != nil {
		return fmt.Errorf("decode normalized business license result: %w", decodeErr)
	}
	ocrData := map[string]any{
		"status":     "done",
		"queued_at":  job.CreatedAt.Format(time.RFC3339),
		"started_at": formatPgTimestamp(job.StartedAt),
		"ocr_job_id": job.ID,
		"ocr_at":     normalized.RecognizedAt.Format(time.RFC3339),
	}
	arg := db.UpdateMerchantApplicationBusinessLicenseParams{ID: payload.ApplicationID}
	if normalized.BusinessLicense != nil {
		validPeriod := normalizeValidPeriod(normalized.BusinessLicense.ValidPeriod)
		ocrData["reg_num"] = normalized.BusinessLicense.RegistrationNumber
		ocrData["enterprise_name"] = normalized.BusinessLicense.EnterpriseName
		ocrData["legal_representative"] = normalized.BusinessLicense.LegalRepresentative
		ocrData["address"] = normalized.BusinessLicense.Address
		ocrData["business_scope"] = normalized.BusinessLicense.BusinessScope
		ocrData["valid_period"] = validPeriod
		ocrData["credit_code"] = normalized.BusinessLicense.CreditCode
		if normalized.BusinessLicense.CreditCode != "" {
			arg.BusinessLicenseNumber = pgtype.Text{String: normalized.BusinessLicense.CreditCode, Valid: true}
		} else if normalized.BusinessLicense.RegistrationNumber != "" {
			arg.BusinessLicenseNumber = pgtype.Text{String: normalized.BusinessLicense.RegistrationNumber, Valid: true}
		}
		if normalized.BusinessLicense.BusinessScope != "" {
			arg.BusinessScope = pgtype.Text{String: normalized.BusinessLicense.BusinessScope, Valid: true}
		}
	}
	log.Info().
		Int64("application_id", payload.ApplicationID).
		Int64("ocr_job_id", job.ID).
		Str("document_type", job.DocumentType).
		Str("provider", job.Provider).
		Interface("ocr_summary", summarizeMerchantBusinessLicenseOCRData(ocrData)).
		Msg("merchant application business license ocr summary")
	ocrJSON, _ := json.Marshal(ocrData)
	arg.BusinessLicenseOcr = ocrJSON
	_, err = processor.store.UpdateMerchantApplicationBusinessLicense(ctx, arg)
	if err != nil {
		return fmt.Errorf("update merchant application business license: %w", err)
	}
	processor.writeOCRJobAudit(ctx, job, "ocr_job_succeeded", map[string]any{"status": job.Status})
	log.Info().Int64("application_id", payload.ApplicationID).Int64("ocr_job_id", job.ID).Msg("✅ business license OCR updated from ocr job")
	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskMerchantApplicationFoodPermitOCR(ctx context.Context, task *asynq.Task) error {
	payload, err := processor.parseMerchantApplicationOCRPayload(task)
	if err != nil {
		return err
	}
	if payload.OCRJobID <= 0 {
		return fmt.Errorf("food permit ocr task requires ocr_job_id: %w", asynq.SkipRetry)
	}
	if processor.ocrService == nil {
		return fmt.Errorf("ocr service not configured for food permit: %w", asynq.SkipRetry)
	}
	return processor.processMerchantApplicationFoodPermitOCRJob(ctx, payload)
}

func (processor *RedisTaskProcessor) processMerchantApplicationFoodPermitOCRJob(ctx context.Context, payload *merchantApplicationOCRPayload) error {
	job, err := processor.ocrService.ExecuteJob(ctx, ocr.ExecuteJobParams{
		JobID:      payload.OCRJobID,
		LeaseOwner: "worker:merchant_food_permit",
	})
	if err != nil {
		alertEmittedAt := processor.publishOCRFailureAlert(ctx, job, err)
		failed := foodPermitOCRData{
			Status:         string(ocr.JobStatusFailed),
			QueuedAt:       job.CreatedAt.Format(time.RFC3339),
			StartedAt:      formatPgTimestamp(job.StartedAt),
			OCRJobID:       int64Ptr(payload.OCRJobID),
			Error:          err.Error(),
			ErrorCode:      ocr.ErrorCode(err),
			AlertEmittedAt: formatOCRAlertEmittedAt(alertEmittedAt),
		}
		failedJSON, _ := json.Marshal(failed)
		_, _ = processor.store.UpdateMerchantApplicationFoodPermit(ctx, db.UpdateMerchantApplicationFoodPermitParams{
			ID:            payload.ApplicationID,
			FoodPermitOcr: failedJSON,
		})
		processor.writeOCRJobAudit(ctx, job, "ocr_job_failed", ocrFailureAuditMetadata(err))
		return asynqOCRTaskError(job, err)
	}
	observeOCRJob(job)
	normalized, decodeErr := ocr.UnmarshalNormalizedResult(job.NormalizedResult)
	if decodeErr != nil {
		return fmt.Errorf("decode normalized food permit result: %w", decodeErr)
	}
	ocrData := foodPermitOCRData{
		Status:    "done",
		QueuedAt:  job.CreatedAt.Format(time.RFC3339),
		StartedAt: formatPgTimestamp(job.StartedAt),
		OCRJobID:  int64Ptr(job.ID),
		OCRAt:     normalized.RecognizedAt.Format(time.RFC3339),
	}
	if normalized.FoodPermit != nil {
		ocrData.RawText = normalized.FoodPermit.RawText
		parseFoodPermitOCRText(&ocrData, normalized.FoodPermit.RawText)
	}
	log.Info().
		Int64("application_id", payload.ApplicationID).
		Int64("ocr_job_id", job.ID).
		Str("document_type", job.DocumentType).
		Str("provider", job.Provider).
		Interface("ocr_summary", summarizeMerchantFoodPermitOCRData(ocrData)).
		Msg("merchant application food permit ocr summary")
	ocrJSON, _ := json.Marshal(ocrData)
	_, err = processor.store.UpdateMerchantApplicationFoodPermit(ctx, db.UpdateMerchantApplicationFoodPermitParams{
		ID:            payload.ApplicationID,
		FoodPermitOcr: ocrJSON,
	})
	if err != nil {
		return fmt.Errorf("update merchant application food permit: %w", err)
	}
	processor.writeOCRJobAudit(ctx, job, "ocr_job_succeeded", map[string]any{"status": job.Status})
	log.Info().Int64("application_id", payload.ApplicationID).Int64("ocr_job_id", job.ID).Msg("✅ food permit OCR updated from ocr job")
	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskMerchantApplicationIDCardOCR(ctx context.Context, task *asynq.Task) error {
	payload, err := processor.parseMerchantApplicationOCRPayload(task)
	if err != nil {
		return err
	}
	if payload.Side != "Front" && payload.Side != "Back" {
		return fmt.Errorf("invalid side: %w", asynq.SkipRetry)
	}
	if payload.OCRJobID <= 0 {
		return fmt.Errorf("id card ocr task requires ocr_job_id: %w", asynq.SkipRetry)
	}
	if processor.ocrService == nil {
		return fmt.Errorf("ocr service not configured for id card: %w", asynq.SkipRetry)
	}
	return processor.processMerchantApplicationIDCardOCRJob(ctx, payload)
}

func (processor *RedisTaskProcessor) processMerchantApplicationIDCardOCRJob(ctx context.Context, payload *merchantApplicationOCRPayload) error {
	job, err := processor.ocrService.ExecuteJob(ctx, ocr.ExecuteJobParams{
		JobID:      payload.OCRJobID,
		LeaseOwner: "worker:merchant_id_card",
	})
	if err != nil {
		alertEmittedAt := processor.publishOCRFailureAlert(ctx, job, err)
		failed := merchantIDCardOCRData{
			Status:         string(ocr.JobStatusFailed),
			QueuedAt:       job.CreatedAt.Format(time.RFC3339),
			StartedAt:      formatPgTimestamp(job.StartedAt),
			OCRJobID:       int64Ptr(payload.OCRJobID),
			Error:          err.Error(),
			ErrorCode:      ocr.ErrorCode(err),
			AlertEmittedAt: formatOCRAlertEmittedAt(alertEmittedAt),
		}
		failedJSON, _ := json.Marshal(failed)
		if payload.Side == "Front" {
			_, _ = processor.store.UpdateMerchantApplicationIDCardFront(ctx, db.UpdateMerchantApplicationIDCardFrontParams{ID: payload.ApplicationID, IDCardFrontOcr: failedJSON})
		} else {
			_, _ = processor.store.UpdateMerchantApplicationIDCardBack(ctx, db.UpdateMerchantApplicationIDCardBackParams{ID: payload.ApplicationID, IDCardBackOcr: failedJSON})
		}
		processor.writeOCRJobAudit(ctx, job, "ocr_job_failed", ocrFailureAuditMetadata(err))
		return asynqOCRTaskError(job, err)
	}
	observeOCRJob(job)
	normalized, decodeErr := ocr.UnmarshalNormalizedResult(job.NormalizedResult)
	if decodeErr != nil {
		return fmt.Errorf("decode normalized id card result: %w", decodeErr)
	}
	ocrData := merchantIDCardOCRData{
		Status:    "done",
		QueuedAt:  job.CreatedAt.Format(time.RFC3339),
		StartedAt: formatPgTimestamp(job.StartedAt),
		OCRJobID:  int64Ptr(job.ID),
		OCRAt:     normalized.RecognizedAt.Format(time.RFC3339),
	}
	if normalized.IDCard != nil {
		ocrData.Name = normalized.IDCard.Name
		ocrData.IDNumber = normalized.IDCard.IDNumber
		ocrData.Gender = normalized.IDCard.Gender
		ocrData.Nation = normalized.IDCard.Ethnicity
		ocrData.Address = normalized.IDCard.Address
		ocrData.ValidDate = normalized.IDCard.ValidPeriod
	}
	log.Info().
		Int64("application_id", payload.ApplicationID).
		Int64("ocr_job_id", job.ID).
		Str("document_type", job.DocumentType).
		Str("provider", job.Provider).
		Str("side", payload.Side).
		Interface("ocr_summary", summarizeMerchantIDCardOCRData(ocrData)).
		Msg("merchant application id card ocr summary")
	ocrJSON, _ := json.Marshal(ocrData)
	if payload.Side == "Front" {
		arg := db.UpdateMerchantApplicationIDCardFrontParams{ID: payload.ApplicationID, IDCardFrontOcr: ocrJSON}
		if ocrData.Name != "" {
			arg.LegalPersonName = pgtype.Text{String: ocrData.Name, Valid: true}
		}
		if ocrData.IDNumber != "" {
			arg.LegalPersonIDNumber = pgtype.Text{String: ocrData.IDNumber, Valid: true}
		}
		_, err = processor.store.UpdateMerchantApplicationIDCardFront(ctx, arg)
		if err != nil {
			return fmt.Errorf("update merchant application id card front: %w", err)
		}
	} else {
		_, err = processor.store.UpdateMerchantApplicationIDCardBack(ctx, db.UpdateMerchantApplicationIDCardBackParams{ID: payload.ApplicationID, IDCardBackOcr: ocrJSON})
		if err != nil {
			return fmt.Errorf("update merchant application id card back: %w", err)
		}
	}
	processor.writeOCRJobAudit(ctx, job, "ocr_job_succeeded", map[string]any{"status": job.Status})
	log.Info().Int64("application_id", payload.ApplicationID).Int64("ocr_job_id", job.ID).Str("side", payload.Side).Msg("✅ id card OCR updated from ocr job")
	return nil
}

func (processor *RedisTaskProcessor) parseMerchantApplicationOCRPayload(task *asynq.Task) (*merchantApplicationOCRPayload, error) {
	var payload merchantApplicationOCRPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}
	if payload.ApplicationID <= 0 || (payload.MediaAssetID <= 0 && payload.OCRJobID <= 0) {
		return nil, fmt.Errorf("invalid payload: %w", asynq.SkipRetry)
	}
	return &payload, nil
}

// foodPermitOCRData is a minimal copy of the API response struct for storage.
// Kept in worker package to avoid import cycles.
type foodPermitOCRData struct {
	Status         string `json:"status,omitempty"`
	Error          string `json:"error,omitempty"`
	ErrorCode      string `json:"error_code,omitempty"`
	AlertEmittedAt string `json:"alert_emitted_at,omitempty"`
	QueuedAt       string `json:"queued_at,omitempty"`
	StartedAt      string `json:"started_at,omitempty"`
	OCRJobID       *int64 `json:"ocr_job_id,omitempty"`
	RawText        string `json:"raw_text,omitempty"`
	PermitNo       string `json:"permit_no,omitempty"`
	CompanyName    string `json:"company_name,omitempty"`
	OperatorName   string `json:"operator_name,omitempty"` // 经营者/法定代表人姓名
	ValidFrom      string `json:"valid_from,omitempty"`
	ValidTo        string `json:"valid_to,omitempty"`
	OCRAt          string `json:"ocr_at,omitempty"`
}

type merchantIDCardOCRData struct {
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
	ValidDate      string `json:"valid_date,omitempty"`
	OCRAt          string `json:"ocr_at,omitempty"`
}

func int64Ptr(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

func formatPgTimestamp(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format(time.RFC3339)
}

func parseFoodPermitOCRText(data *foodPermitOCRData, text string) {
	// 企业名称匹配 - 使用多种模式尝试提取
	namePatterns := []*regexp.Regexp{
		// 模式1: 标准格式 "经营者名称：XXX"
		regexp.MustCompile(`(?m)(?:经营者名称|单位名称|名\s*称)\s*[:：]?\s*([^\n\r]{2,50})`),
		// 模式2: "主体名称：XXX"
		regexp.MustCompile(`(?m)主体名称\s*[:：]?\s*([^\n\r]{2,50})`),
		// 模式3: "商号名称：XXX"（小餐饮/小作坊登记证格式）
		regexp.MustCompile(`(?m)商号名称\s*[:：]?\s*([^\n\r]{2,50})`),
		// 模式4: "社会信用代码"后面的企业名称行
		regexp.MustCompile(`(?m)统一社会信用代码[^\n]*\n\s*([^\n\r]{2,50})`),
		// 模式5: 包含餐饮/食品关键词的行（可能直接是企业名）
		regexp.MustCompile(`(?m)^([^\n\r]{0,20}(?:餐饮|食品|饮品|酒店|饭店|餐厅)[^\n\r]{0,30})$`),
	}

	for _, nameRegex := range namePatterns {
		if match := nameRegex.FindStringSubmatch(text); len(match) > 1 {
			name := strings.TrimSpace(match[1])
			name = strings.ReplaceAll(name, " ", "")
			name = strings.ReplaceAll(name, "\t", "")
			// 清洗企业名称：在常见企业结尾词处截断
			name = cleanCompanyName(name)
			// 排除明显不是企业名的结果
			if name != "" && len(name) >= 4 && !strings.HasPrefix(name, "JY") && !strings.HasPrefix(name, "9") {
				data.CompanyName = name
				break
			}
		}
	}

	// 如果未能提取企业名称，记录日志便于迭代优化
	if data.CompanyName == "" {
		log.Warn().
			Str("raw_text_preview", truncateString(text, 200)).
			Msg("food permit company name extraction failed")
	}

	// 经营者姓名（个体工商户/小餐饮登记证格式）
	operatorRegex := regexp.MustCompile(`经营者姓名\s*[:：]?\s*([^\s\n\r,，。]{2,10})`)
	if m := operatorRegex.FindStringSubmatch(text); len(m) > 1 {
		data.OperatorName = strings.TrimSpace(m[1])
	}

	// 许可证编号 - 支持食品经营许可证(JY开头)和小餐饮/小作坊登记证(纯数字)
	permitNoRegex := regexp.MustCompile(`JY[0-9]{12,}`)
	if match := permitNoRegex.FindString(text); match != "" {
		data.PermitNo = match
	}
	if data.PermitNo == "" {
		// 小餐饮/小作坊登记证编号格式，如 "登记证编号:2130528020946"
		regNoRegex := regexp.MustCompile(`(?:登记证编号|证书编号|编号)\s*[:：]\s*([0-9]{8,})`)
		if match := regNoRegex.FindStringSubmatch(text); len(match) > 1 {
			data.PermitNo = match[1]
		}
	}

	// 有效期 - 长期
	if strings.Contains(text, "长期") {
		data.ValidTo = "长期"
		return
	}

	// normalizeDate 去除OCR在年月日汉字前后插入的空格，如 "2027 年01月08日" → "2027年01月08日"
	normalizeDate := func(s string) string {
		s = strings.ReplaceAll(s, " 年", "年")
		s = strings.ReplaceAll(s, "年 ", "年")
		s = strings.ReplaceAll(s, " 月", "月")
		s = strings.ReplaceAll(s, "月 ", "月")
		s = strings.ReplaceAll(s, " 日", "日")
		s = strings.ReplaceAll(s, "日 ", "日")
		return strings.TrimSpace(s)
	}

	// 有效期至 - 允许年月日前后有空格（OCR常见问题）
	datePattern := `\d{4}\s*年\s*\d{1,2}\s*月\s*\d{1,2}\s*日`
	validToPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:有效期至|有效期限至)\s*[:：]?\s*(` + datePattern + `)`),
		regexp.MustCompile(`(?:有效日期|有效期)\s*[:：]?\s*` + datePattern + `\s*[至到-]\s*(` + datePattern + `)`),
	}

	for _, validToRegex := range validToPatterns {
		if match := validToRegex.FindStringSubmatch(text); len(match) > 1 {
			data.ValidTo = normalizeDate(match[1])
			break
		}
	}

	// 有效期范围（如 "2021年01月08日至2027年01月08日"）- 允许年月日前后有空格
	validRangeRegex := regexp.MustCompile(`(` + datePattern + `)\s*[至到-]\s*(` + datePattern + `)`)
	if match := validRangeRegex.FindStringSubmatch(text); len(match) > 2 {
		data.ValidFrom = normalizeDate(match[1])
		data.ValidTo = normalizeDate(match[2])
	}
}

// truncateString 截断字符串用于日志
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// cleanCompanyName 清洗企业名称，在常见企业名结尾词处截断
// 解决OCR文本没有换行符导致匹配过多内容的问题
func cleanCompanyName(name string) string {
	if name == "" {
		return ""
	}

	// 常见企业名称结尾词（按优先级排序）
	endings := []string{
		"有限责任公司",
		"有限公司",
		"股份有限公司",
		"股份公司",
		"责任公司",
		"集团公司",
		"集团",
		"公司",
		"餐饮店",
		"餐厅",
		"饭店",
		"酒店",
		"饭馆",
		"餐馆",
		"小吃店",
		"面馆",
		"菜馆",
		"火锅店",
		"烧烤店",
		"奶茶店",
		"咖啡店",
		"便利店",
		"超市",
		"商店",
		"店",
		"馆",
		"坊",
		"阁",
		"轩",
	}

	// 查找第一个匹配的结尾词，在其后截断
	for _, ending := range endings {
		idx := strings.Index(name, ending)
		if idx > 0 {
			// 在结尾词后面截断
			return name[:idx+len(ending)]
		}
	}

	// 如果没有找到常见结尾，检查是否超长（可能包含了无关内容）
	// 通常企业名不会超过50个字符
	runes := []rune(name)
	if len(runes) > 50 {
		return string(runes[:50])
	}

	return name
}

// normalizeValidPeriod 归一化营业执照营业期限字段。
// 部分营业执照（如个体工商户）仅登记注册日期（成立日期），无明确到期日，
// 在微信 OCR 接口中 valid_period 可能为空或仅含单个起始日期。
// 按产品设计，此类情况一律视为「长期有效」。
func normalizeValidPeriod(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "长期"
	}
	// 已明确包含长期/永久关键字，直接返回
	if strings.Contains(raw, "长期") || strings.Contains(raw, "永久") {
		return raw
	}
	// 包含范围分隔符「至」，说明是有明确起止日期的固定期限，不做转换
	if strings.Contains(raw, "至") {
		return raw
	}
	// 仅含单个日期（注册/成立日期），无终止日期 → 视为长期
	dateRegex := regexp.MustCompile(`\d{4}年\d{1,2}月\d{1,2}日`)
	if dateRegex.MatchString(raw) {
		return "长期"
	}
	return raw
}
