package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

func marshalOCRTaskProcessing(startedAt string) []byte {
	b, _ := json.Marshal(map[string]any{
		"status":     "processing",
		"started_at": startedAt,
	})
	return b
}

// getOCRErrorHint 根据错误类型返回用户友好的提示信息
func getOCRErrorHint(err error) string {
	if err == nil {
		return ""
	}
	errStr := err.Error()

	// 图片大小错误
	if errors.Is(err, wechat.ErrImageTooLarge) || strings.Contains(errStr, "too large") {
		return "图片文件过大，请压缩后重新上传（最大2MB）"
	}

	// 文件不存在
	if errors.Is(err, os.ErrNotExist) || strings.Contains(errStr, "not exist") {
		return "图片文件不存在，请重新上传"
	}

	// 微信API错误
	if strings.Contains(errStr, "wechat") {
		if strings.Contains(errStr, "48001") {
			return "微信OCR接口暂时不可用，请稍后重试"
		}
		if strings.Contains(errStr, "45009") {
			return "调用次数超限，请稍后重试"
		}
		if strings.Contains(errStr, "invalid image") || strings.Contains(errStr, "img format") {
			return "图片格式不正确，请上传清晰的JPG或PNG格式照片"
		}
	}

	// 默认提示
	return "识别失败，请确保照片清晰、光线充足后重新上传"
}

func marshalOCRTaskFailed(err error, startedAt string) []byte {
	b, _ := json.Marshal(map[string]any{
		"status":     "failed",
		"error":      err.Error(),
		"user_hint":  getOCRErrorHint(err),
		"started_at": startedAt,
	})
	return b
}

const (
	TaskMerchantApplicationBusinessLicenseOCR = "merchant_application:ocr_business_license"
	TaskMerchantApplicationFoodPermitOCR      = "merchant_application:ocr_food_permit"
	TaskMerchantApplicationIDCardOCR          = "merchant_application:ocr_id_card"
)

type merchantApplicationOCRPayload struct {
	ApplicationID int64  `json:"application_id"`
	ImagePath     string `json:"image_path"`
	Side          string `json:"side,omitempty"` // Front/Back
}

func (distributor *RedisTaskDistributor) DistributeTaskMerchantApplicationBusinessLicenseOCR(
	ctx context.Context,
	applicationID int64,
	imagePath string,
	opts ...asynq.Option,
) error {
	payloadBytes, err := json.Marshal(merchantApplicationOCRPayload{
		ApplicationID: applicationID,
		ImagePath:     imagePath,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return distributor.enqueue(ctx, TaskMerchantApplicationBusinessLicenseOCR, payloadBytes, opts...)
}

func (distributor *RedisTaskDistributor) DistributeTaskMerchantApplicationFoodPermitOCR(
	ctx context.Context,
	applicationID int64,
	imagePath string,
	opts ...asynq.Option,
) error {
	payloadBytes, err := json.Marshal(merchantApplicationOCRPayload{
		ApplicationID: applicationID,
		ImagePath:     imagePath,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return distributor.enqueue(ctx, TaskMerchantApplicationFoodPermitOCR, payloadBytes, opts...)
}

func (distributor *RedisTaskDistributor) DistributeTaskMerchantApplicationIDCardOCR(
	ctx context.Context,
	applicationID int64,
	imagePath string,
	side string,
	opts ...asynq.Option,
) error {
	payloadBytes, err := json.Marshal(merchantApplicationOCRPayload{
		ApplicationID: applicationID,
		ImagePath:     imagePath,
		Side:          side,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return distributor.enqueue(ctx, TaskMerchantApplicationIDCardOCR, payloadBytes, opts...)
}

func (distributor *RedisTaskDistributor) enqueue(ctx context.Context, taskType string, payload []byte, opts ...asynq.Option) error {
	task := asynq.NewTask(taskType, payload, opts...)
	info, err := distributor.client.EnqueueContext(ctx, task)
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
	if processor.wechatClient == nil {
		startedAt := time.Now().Format(time.RFC3339)
		_, _ = processor.store.UpdateMerchantApplicationBusinessLicense(ctx, db.UpdateMerchantApplicationBusinessLicenseParams{
			ID:                 payload.ApplicationID,
			BusinessLicenseOcr: marshalOCRTaskFailed(fmt.Errorf("wechat client not configured"), startedAt),
		})
		return fmt.Errorf("wechat client not configured: %w", asynq.SkipRetry)
	}

	startedAt := time.Now().Format(time.RFC3339)
	_, _ = processor.store.UpdateMerchantApplicationBusinessLicense(ctx, db.UpdateMerchantApplicationBusinessLicenseParams{
		ID:                 payload.ApplicationID,
		BusinessLicenseOcr: marshalOCRTaskProcessing(startedAt),
	})

	// 使用图片压缩器读取并压缩图片（如果需要）
	compressor := util.NewImageCompressor(util.DefaultMaxOCRBytes)
	imgData, wasCompressed, err := compressor.CompressFileForOCR(payload.ImagePath)
	if err != nil {
		ocrErr := fmt.Errorf("图片处理失败: %w", err)
		_, _ = processor.store.UpdateMerchantApplicationBusinessLicense(ctx, db.UpdateMerchantApplicationBusinessLicenseParams{
			ID:                 payload.ApplicationID,
			BusinessLicenseOcr: marshalOCRTaskFailed(ocrErr, startedAt),
		})
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("compress image: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("compress image: %w", err)
	}

	if wasCompressed {
		log.Info().
			Int64("application_id", payload.ApplicationID).
			Str("image_path", payload.ImagePath).
			Int("compressed_size", len(imgData)).
			Msg("business license image compressed for OCR")
	}

	// 使用压缩后的图片数据调用 OCR
	imgFile := util.NewBytesFile(imgData)
	ocrResult, err := processor.wechatClient.OCRBusinessLicense(ctx, imgFile)
	if err != nil {
		_, _ = processor.store.UpdateMerchantApplicationBusinessLicense(ctx, db.UpdateMerchantApplicationBusinessLicenseParams{
			ID:                 payload.ApplicationID,
			BusinessLicenseOcr: marshalOCRTaskFailed(err, startedAt),
		})
		if errors.Is(err, wechat.ErrImageTooLarge) {
			return fmt.Errorf("wechat OCRBusinessLicense: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("wechat OCRBusinessLicense: %w", err)
	}

	ocrData := map[string]any{
		"status":                "done",
		"started_at":            startedAt,
		"reg_num":               ocrResult.RegNum,
		"enterprise_name":       ocrResult.EnterpriseEName,
		"legal_representative":  ocrResult.LegalRepresentative,
		"type_of_enterprise":    ocrResult.TypeOfEnterprise,
		"address":               ocrResult.Address,
		"business_scope":        ocrResult.BusinessScope,
		"registered_capital":    ocrResult.RegisteredCapital,
		"valid_period":          ocrResult.ValidPeriod,
		"credit_code":           ocrResult.CreditCode,
		"ocr_at":                time.Now().Format(time.RFC3339),
	}
	ocrJSON, _ := json.Marshal(ocrData)

	arg := db.UpdateMerchantApplicationBusinessLicenseParams{ID: payload.ApplicationID}
	arg.BusinessLicenseOcr = ocrJSON

	if ocrResult.CreditCode != "" {
		arg.BusinessLicenseNumber = pgtype.Text{String: ocrResult.CreditCode, Valid: true}
	} else if ocrResult.RegNum != "" {
		arg.BusinessLicenseNumber = pgtype.Text{String: ocrResult.RegNum, Valid: true}
	}
	if ocrResult.BusinessScope != "" {
		arg.BusinessScope = pgtype.Text{String: ocrResult.BusinessScope, Valid: true}
	}

	_, err = processor.store.UpdateMerchantApplicationBusinessLicense(ctx, arg)
	if err != nil {
		return fmt.Errorf("update merchant application business license: %w", err)
	}

	log.Info().Int64("application_id", payload.ApplicationID).Msg("✅ business license OCR updated")
	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskMerchantApplicationFoodPermitOCR(ctx context.Context, task *asynq.Task) error {
	payload, err := processor.parseMerchantApplicationOCRPayload(task)
	if err != nil {
		return err
	}
	if processor.wechatClient == nil {
		startedAt := time.Now().Format(time.RFC3339)
		_, _ = processor.store.UpdateMerchantApplicationFoodPermit(ctx, db.UpdateMerchantApplicationFoodPermitParams{
			ID:            payload.ApplicationID,
			FoodPermitOcr: marshalOCRTaskFailed(fmt.Errorf("wechat client not configured"), startedAt),
		})
		return fmt.Errorf("wechat client not configured: %w", asynq.SkipRetry)
	}

	startedAt := time.Now().Format(time.RFC3339)
	_, _ = processor.store.UpdateMerchantApplicationFoodPermit(ctx, db.UpdateMerchantApplicationFoodPermitParams{
		ID:            payload.ApplicationID,
		FoodPermitOcr: marshalOCRTaskProcessing(startedAt),
	})

	// 使用图片压缩器读取并压缩图片（如果需要）
	compressor := util.NewImageCompressor(util.DefaultMaxOCRBytes)
	imgData, wasCompressed, err := compressor.CompressFileForOCR(payload.ImagePath)
	if err != nil {
		ocrErr := fmt.Errorf("图片处理失败: %w", err)
		_, _ = processor.store.UpdateMerchantApplicationFoodPermit(ctx, db.UpdateMerchantApplicationFoodPermitParams{
			ID:            payload.ApplicationID,
			FoodPermitOcr: marshalOCRTaskFailed(ocrErr, startedAt),
		})
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("compress image: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("compress image: %w", err)
	}

	if wasCompressed {
		log.Info().
			Int64("application_id", payload.ApplicationID).
			Str("image_path", payload.ImagePath).
			Int("compressed_size", len(imgData)).
			Msg("food permit image compressed for OCR")
	}

	// 使用压缩后的图片数据调用 OCR
	imgFile := util.NewBytesFile(imgData)
	ocrResult, err := processor.wechatClient.OCRPrintedText(ctx, imgFile)
	if err != nil {
		_, _ = processor.store.UpdateMerchantApplicationFoodPermit(ctx, db.UpdateMerchantApplicationFoodPermitParams{
			ID:            payload.ApplicationID,
			FoodPermitOcr: marshalOCRTaskFailed(err, startedAt),
		})
		if errors.Is(err, wechat.ErrImageTooLarge) {
			return fmt.Errorf("wechat OCRPrintedText: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("wechat OCRPrintedText: %w", err)
	}

	rawText := ocrResult.GetAllText()
	ocrData := foodPermitOCRData{
		Status:    "done",
		StartedAt: startedAt,
		RawText:   rawText,
		OCRAt:     time.Now().Format(time.RFC3339),
	}
	parseFoodPermitOCRText(&ocrData, rawText)
	ocrJSON, _ := json.Marshal(ocrData)

	arg := db.UpdateMerchantApplicationFoodPermitParams{ID: payload.ApplicationID}
	arg.FoodPermitOcr = ocrJSON

	_, err = processor.store.UpdateMerchantApplicationFoodPermit(ctx, arg)
	if err != nil {
		return fmt.Errorf("update merchant application food permit: %w", err)
	}

	log.Info().Int64("application_id", payload.ApplicationID).Msg("✅ food permit OCR updated")
	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskMerchantApplicationIDCardOCR(ctx context.Context, task *asynq.Task) error {
	payload, err := processor.parseMerchantApplicationOCRPayload(task)
	if err != nil {
		return err
	}
	if processor.wechatClient == nil {
		startedAt := time.Now().Format(time.RFC3339)
		failed := marshalOCRTaskFailed(fmt.Errorf("wechat client not configured"), startedAt)
		switch payload.Side {
		case "Front":
			_, _ = processor.store.UpdateMerchantApplicationIDCardFront(ctx, db.UpdateMerchantApplicationIDCardFrontParams{ID: payload.ApplicationID, IDCardFrontOcr: failed})
		case "Back":
			_, _ = processor.store.UpdateMerchantApplicationIDCardBack(ctx, db.UpdateMerchantApplicationIDCardBackParams{ID: payload.ApplicationID, IDCardBackOcr: failed})
		}
		return fmt.Errorf("wechat client not configured: %w", asynq.SkipRetry)
	}
	if payload.Side != "Front" && payload.Side != "Back" {
		return fmt.Errorf("invalid side: %w", asynq.SkipRetry)
	}

	startedAt := time.Now().Format(time.RFC3339)
	processing := marshalOCRTaskProcessing(startedAt)
	switch payload.Side {
	case "Front":
		_, _ = processor.store.UpdateMerchantApplicationIDCardFront(ctx, db.UpdateMerchantApplicationIDCardFrontParams{ID: payload.ApplicationID, IDCardFrontOcr: processing})
	case "Back":
		_, _ = processor.store.UpdateMerchantApplicationIDCardBack(ctx, db.UpdateMerchantApplicationIDCardBackParams{ID: payload.ApplicationID, IDCardBackOcr: processing})
	}

	// 使用图片压缩器读取并压缩图片（如果需要）
	compressor := util.NewImageCompressor(util.DefaultMaxOCRBytes)
	imgData, wasCompressed, err := compressor.CompressFileForOCR(payload.ImagePath)
	if err != nil {
		ocrErr := fmt.Errorf("图片处理失败: %w", err)
		failed := marshalOCRTaskFailed(ocrErr, startedAt)
		switch payload.Side {
		case "Front":
			_, _ = processor.store.UpdateMerchantApplicationIDCardFront(ctx, db.UpdateMerchantApplicationIDCardFrontParams{ID: payload.ApplicationID, IDCardFrontOcr: failed})
		case "Back":
			_, _ = processor.store.UpdateMerchantApplicationIDCardBack(ctx, db.UpdateMerchantApplicationIDCardBackParams{ID: payload.ApplicationID, IDCardBackOcr: failed})
		}
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("compress image: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("compress image: %w", err)
	}

	if wasCompressed {
		log.Info().
			Int64("application_id", payload.ApplicationID).
			Str("image_path", payload.ImagePath).
			Str("side", payload.Side).
			Int("compressed_size", len(imgData)).
			Msg("id card image compressed for OCR")
	}

	// 使用压缩后的图片数据调用 OCR
	imgFile := util.NewBytesFile(imgData)
	ocrResult, err := processor.wechatClient.OCRIDCard(ctx, imgFile, payload.Side)
	if err != nil {
		failed := marshalOCRTaskFailed(err, startedAt)
		if payload.Side == "Front" {
			_, _ = processor.store.UpdateMerchantApplicationIDCardFront(ctx, db.UpdateMerchantApplicationIDCardFrontParams{ID: payload.ApplicationID, IDCardFrontOcr: failed})
		} else {
			_, _ = processor.store.UpdateMerchantApplicationIDCardBack(ctx, db.UpdateMerchantApplicationIDCardBackParams{ID: payload.ApplicationID, IDCardBackOcr: failed})
		}
		if errors.Is(err, wechat.ErrImageTooLarge) {
			return fmt.Errorf("wechat OCRIDCard: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("wechat OCRIDCard: %w", err)
	}

	ocrData := map[string]any{"status": "done", "started_at": startedAt, "ocr_at": time.Now().Format(time.RFC3339)}
	if payload.Side == "Front" {
		ocrData["name"] = ocrResult.Name
		ocrData["id_number"] = ocrResult.ID
		ocrData["gender"] = ocrResult.Gender
		ocrData["nation"] = ocrResult.Nation
		ocrData["address"] = ocrResult.Addr
	} else {
		ocrData["valid_date"] = ocrResult.ValidDate
	}
	ocrJSON, _ := json.Marshal(ocrData)

	if payload.Side == "Front" {
		arg := db.UpdateMerchantApplicationIDCardFrontParams{ID: payload.ApplicationID}
		arg.IDCardFrontOcr = ocrJSON
		if ocrResult.Name != "" {
			arg.LegalPersonName = pgtype.Text{String: ocrResult.Name, Valid: true}
		}
		if ocrResult.ID != "" {
			arg.LegalPersonIDNumber = pgtype.Text{String: ocrResult.ID, Valid: true}
		}

		_, err = processor.store.UpdateMerchantApplicationIDCardFront(ctx, arg)
		if err != nil {
			return fmt.Errorf("update merchant application id card front: %w", err)
		}
	} else {
		arg := db.UpdateMerchantApplicationIDCardBackParams{ID: payload.ApplicationID}
		arg.IDCardBackOcr = ocrJSON
		_, err = processor.store.UpdateMerchantApplicationIDCardBack(ctx, arg)
		if err != nil {
			return fmt.Errorf("update merchant application id card back: %w", err)
		}
	}

	log.Info().Int64("application_id", payload.ApplicationID).Str("side", payload.Side).Msg("✅ id card OCR updated")
	return nil
}

func (processor *RedisTaskProcessor) parseMerchantApplicationOCRPayload(task *asynq.Task) (*merchantApplicationOCRPayload, error) {
	var payload merchantApplicationOCRPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}
	payload.ImagePath = strings.TrimSpace(payload.ImagePath)
	if payload.ApplicationID <= 0 || payload.ImagePath == "" {
		return nil, fmt.Errorf("invalid payload: %w", asynq.SkipRetry)
	}
	return &payload, nil
}

// foodPermitOCRData is a minimal copy of the API response struct for storage.
// Kept in worker package to avoid import cycles.
type foodPermitOCRData struct {
	Status    string `json:"status,omitempty"`
	Error     string `json:"error,omitempty"`
	QueuedAt  string `json:"queued_at,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	RawText     string `json:"raw_text,omitempty"`
	PermitNo    string `json:"permit_no,omitempty"`
	CompanyName string `json:"company_name,omitempty"`
	ValidFrom   string `json:"valid_from,omitempty"`
	ValidTo     string `json:"valid_to,omitempty"`
	OCRAt       string `json:"ocr_at,omitempty"`
}

func parseFoodPermitOCRText(data *foodPermitOCRData, text string) {
	// 企业名称匹配 - 使用多种模式尝试提取
	namePatterns := []*regexp.Regexp{
		// 模式1: 标准格式 "经营者名称：XXX"
		regexp.MustCompile(`(?m)(?:经营者名称|单位名称|名\s*称)\s*[:：]?\s*([^\n\r]{2,50})`),
		// 模式2: "主体名称：XXX"
		regexp.MustCompile(`(?m)主体名称\s*[:：]?\s*([^\n\r]{2,50})`),
		// 模式3: "社会信用代码"后面的企业名称行
		regexp.MustCompile(`(?m)统一社会信用代码[^\n]*\n\s*([^\n\r]{2,50})`),
		// 模式4: 包含餐饮/食品关键词的行（可能直接是企业名）
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

	// 许可证编号
	permitNoRegex := regexp.MustCompile(`JY[0-9]{12,}`)
	if match := permitNoRegex.FindString(text); match != "" {
		data.PermitNo = match
	}

	// 有效期 - 长期
	if strings.Contains(text, "长期") {
		data.ValidTo = "长期"
		return
	}

	// 有效期至
	validToPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:有效期至|至|有效期限至)\s*[:：]?\s*(\d{4}年\d{1,2}月\d{1,2}日)`),
		regexp.MustCompile(`(?:有效日期|有效期)\s*[:：]?\s*\d{4}年\d{1,2}月\d{1,2}日\s*[至到-]\s*(\d{4}年\d{1,2}月\d{1,2}日)`),
	}

	for _, validToRegex := range validToPatterns {
		if match := validToRegex.FindStringSubmatch(text); len(match) > 1 {
			data.ValidTo = match[1]
			break
		}
	}

	// 有效期范围
	validRangeRegex := regexp.MustCompile(`(\d{4}年\d{1,2}月\d{1,2}日)\s*[至到-]\s*(\d{4}年\d{1,2}月\d{1,2}日)`)
	if match := validRangeRegex.FindStringSubmatch(text); len(match) > 2 {
		data.ValidFrom = match[1]
		data.ValidTo = match[2]
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
