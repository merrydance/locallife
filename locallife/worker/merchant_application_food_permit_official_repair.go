package worker

import (
	"context"
	"errors"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/rs/zerolog/log"
)

type merchantFoodPermitOfficialVerifier interface {
	VerifyMerchantFoodPermit(ctx context.Context, rawResult []byte) (logic.MerchantFoodPermitOfficialVerification, error)
}

func (processor *RedisTaskProcessor) repairMerchantFoodPermitOCRDataFromOfficialVerification(ctx context.Context, data *foodPermitOCRData, job db.OcrJob, applicationID int64) bool {
	if processor == nil || data == nil || processor.foodPermitVerifier == nil || !foodPermitOCRNeedsOfficialVerification(*data, job.RawResult) {
		return false
	}
	verification, err := processor.foodPermitVerifier.VerifyMerchantFoodPermit(ctx, job.RawResult)
	if err != nil {
		event := log.Debug()
		if !errors.Is(err, logic.ErrMerchantFoodPermitOfficialVerificationUnavailable) {
			event = log.Warn().Err(err)
		}
		event.
			Int64("application_id", applicationID).
			Int64("ocr_job_id", job.ID).
			Str("provider", job.Provider).
			Msg("food permit official verification unavailable for worker writeback")
		return false
	}
	if !repairFoodPermitOCRDataFromOfficialVerification(data, verification) {
		return false
	}
	log.Info().
		Int64("application_id", applicationID).
		Int64("ocr_job_id", job.ID).
		Str("provider", job.Provider).
		Msg("food permit OCR repaired from official verification before writeback")
	return true
}

func foodPermitOCRNeedsOfficialVerification(data foodPermitOCRData, rawResult []byte) bool {
	if len(rawResult) == 0 {
		return false
	}
	return strings.TrimSpace(data.CompanyName) == "" ||
		foodPermitCompanyNameLooksLikeCertificateText(data.CompanyName)
}

func repairFoodPermitOCRDataFromOfficialVerification(data *foodPermitOCRData, verification logic.MerchantFoodPermitOfficialVerification) bool {
	if data == nil || strings.TrimSpace(verification.CompanyName) == "" {
		return false
	}
	if strings.TrimSpace(data.CompanyName) != "" && !foodPermitCompanyNameLooksLikeCertificateText(data.CompanyName) {
		return false
	}
	changed := false
	if companyName := normalizeOfficialFoodPermitCompanyName(verification.CompanyName); companyName != "" && !isSuspiciousOfficialFoodPermitCompanyName(companyName) {
		if companyName != data.CompanyName {
			data.CompanyName = companyName
			changed = true
		}
	}
	if operatorName := strings.TrimSpace(verification.OperatorName); operatorName != "" && data.OperatorName == "" {
		data.OperatorName = operatorName
		changed = true
	}
	if permitNo := strings.TrimSpace(verification.PermitNo); permitNo != "" && data.PermitNo == "" {
		data.PermitNo = permitNo
		changed = true
	}
	if validTo := strings.TrimSpace(verification.ValidTo); validTo != "" && data.ValidTo == "" {
		data.ValidTo = validTo
		changed = true
	}
	if rawText := buildFoodPermitOfficialVerificationRawText(data.RawText, verification); rawText != "" && rawText != data.RawText {
		data.RawText = rawText
		changed = true
	}
	return changed
}

func normalizeOfficialFoodPermitCompanyName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "\t", "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "（", "(")
	name = strings.ReplaceAll(name, "）", ")")
	return cleanCompanyName(name)
}

func isSuspiciousOfficialFoodPermitCompanyName(name string) bool {
	name = normalizeOfficialFoodPermitCompanyName(name)
	if len([]rune(name)) < 2 || len([]rune(name)) > 30 {
		return true
	}
	return foodPermitCompanyNameLooksLikeCertificateText(name)
}

func foodPermitCompanyNameLooksLikeCertificateText(name string) bool {
	for _, keyword := range []string{"地址", "经营场所", "面积", "办理", "许可证", "登记证", "项目", "小作坊", "小餐饮", "路东", "路西", "路北", "路南", "请", "《", "》"} {
		if strings.Contains(name, keyword) {
			return true
		}
	}
	return false
}

func buildFoodPermitOfficialVerificationRawText(existing string, verification logic.MerchantFoodPermitOfficialVerification) string {
	lines := make([]string, 0, 8)
	if trimmed := strings.TrimSpace(existing); trimmed != "" {
		lines = append(lines, trimmed)
	}
	appendLine := func(label string, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			lines = append(lines, label+"："+value)
		}
	}
	appendLine("官方核验主体名称", verification.CompanyName)
	appendLine("官方核验经营者姓名", verification.OperatorName)
	appendLine("官方核验登记证编号", verification.PermitNo)
	appendLine("官方核验统一社会信用代码", verification.CreditCode)
	appendLine("官方核验经营场所", verification.Address)
	appendLine("官方核验有效期至", verification.ValidTo)
	return strings.Join(lines, "\n")
}
