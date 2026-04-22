package logic

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/merrydance/locallife/ocr"
)

type MerchantDocumentReviewPayloads struct {
	BusinessLicenseJSON      []byte
	FoodPermitJSON           []byte
	IDCardFrontJSON          []byte
	IDCardBackJSON           []byte
	FoodPermitNormalizedJSON []byte
}

type MerchantDocumentReviewPayloadResult struct {
	Input                           MerchantDocumentReviewInput
	RepairedFoodPermitJSON          []byte
	FoodPermitNeedsNormalizedRepair bool
}

func BuildMerchantDocumentReviewInputFromPayloads(payloads MerchantDocumentReviewPayloads) (MerchantDocumentReviewPayloadResult, error) {
	result := MerchantDocumentReviewPayloadResult{}

	if err := decodeMerchantDocumentReviewPayload(
		payloads.BusinessLicenseJSON,
		&result.Input.BusinessLicense,
		MerchantDocumentReviewCodeBusinessLicenseRequired,
		"营业执照信息未识别，请重新上传清晰的营业执照照片",
		"营业执照信息解析失败，请重新上传清晰完整的营业执照照片",
	); err != nil {
		return MerchantDocumentReviewPayloadResult{}, err
	}

	if err := decodeMerchantDocumentReviewPayload(
		payloads.FoodPermitJSON,
		&result.Input.FoodPermit,
		MerchantDocumentReviewCodeFoodLicenseRequired,
		"食品经营许可证信息未识别，请重新上传清晰完整的食品经营许可证照片",
		"食品经营许可证信息解析失败，请重新上传清晰完整的食品经营许可证照片",
	); err != nil {
		return MerchantDocumentReviewPayloadResult{}, err
	}

	repairedFoodPermitJSON, err := repairMerchantFoodPermitFromRawText(&result.Input.FoodPermit)
	if err != nil {
		return MerchantDocumentReviewPayloadResult{}, err
	}
	result.RepairedFoodPermitJSON = repairedFoodPermitJSON
	result.FoodPermitNeedsNormalizedRepair = merchantFoodPermitNeedsRepair(result.Input.FoodPermit)

	if err := decodeMerchantDocumentReviewPayload(
		payloads.IDCardFrontJSON,
		&result.Input.IDCardFront,
		MerchantDocumentReviewCodeIDCardFrontRequired,
		"身份证正面信息未识别，请重新上传清晰的身份证正面照片",
		"身份证正面信息解析失败，请重新上传清晰的身份证正面照片",
	); err != nil {
		return MerchantDocumentReviewPayloadResult{}, err
	}

	if err := decodeMerchantDocumentReviewPayload(
		payloads.IDCardBackJSON,
		&result.Input.IDCardBack,
		MerchantDocumentReviewCodeIDCardBackRequired,
		"身份证背面信息未识别，请重新上传清晰的身份证背面照片",
		"身份证背面信息解析失败，请重新上传清晰的身份证背面照片",
	); err != nil {
		return MerchantDocumentReviewPayloadResult{}, err
	}

	return result, nil
}

func RepairMerchantFoodPermitFromNormalized(foodPermit *MerchantReviewFoodPermitOCRData, normalizedResult []byte) ([]byte, bool, error) {
	if foodPermit == nil || len(normalizedResult) == 0 || !merchantFoodPermitNeedsRepair(*foodPermit) {
		return nil, false, nil
	}

	normalized, err := ocr.UnmarshalNormalizedResult(normalizedResult)
	if err != nil {
		return nil, false, err
	}
	if normalized.FoodPermit == nil {
		return nil, false, nil
	}

	changed := populateMerchantFoodPermitFromNormalized(foodPermit, normalized.FoodPermit)
	if merchantFoodPermitNeedsRepair(*foodPermit) && foodPermit.RawText != "" {
		changed = reparseMerchantFoodPermitMissingFields(foodPermit) || changed
	}
	if !changed {
		return nil, false, nil
	}

	payload, err := json.Marshal(foodPermit)
	if err != nil {
		return nil, false, err
	}
	return payload, true, nil
}

func decodeMerchantDocumentReviewPayload(payload []byte, target any, code string, missingMessage string, parseMessage string) error {
	if len(payload) == 0 {
		return &MerchantDocumentReviewError{Code: code, Message: missingMessage}
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return &MerchantDocumentReviewError{Code: code, Message: parseMessage}
	}
	return nil
}

func repairMerchantFoodPermitFromRawText(foodPermit *MerchantReviewFoodPermitOCRData) ([]byte, error) {
	if foodPermit == nil || !merchantFoodPermitNeedsRepair(*foodPermit) || strings.TrimSpace(foodPermit.RawText) == "" {
		return nil, nil
	}

	if !reparseMerchantFoodPermitMissingFields(foodPermit) {
		return nil, nil
	}

	payload, err := json.Marshal(foodPermit)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func merchantFoodPermitNeedsRepair(foodPermit MerchantReviewFoodPermitOCRData) bool {
	return foodPermit.ValidTo == "" || foodPermit.OperatorName == "" || foodPermit.CompanyName == "" || foodPermit.PermitNo == "" || merchantIsSuspiciousFoodPermitCompanyName(foodPermit.CompanyName)
}

func reparseMerchantFoodPermitMissingFields(foodPermit *MerchantReviewFoodPermitOCRData) bool {
	if foodPermit == nil || foodPermit.RawText == "" {
		return false
	}

	changed := false
	rawText := foodPermit.RawText

	if foodPermit.ValidTo == "" {
		dateRe := regexp.MustCompile(`有效期至\s*[:：]?\s*(\d{4}\s*年\s*\d{1,2}\s*月\s*\d{1,2}\s*日)`)
		if matches := dateRe.FindStringSubmatch(rawText); matches != nil {
			normalizedDate := strings.ReplaceAll(matches[1], " 年", "年")
			normalizedDate = strings.ReplaceAll(normalizedDate, "年 ", "年")
			normalizedDate = strings.ReplaceAll(normalizedDate, " 月", "月")
			normalizedDate = strings.ReplaceAll(normalizedDate, "月 ", "月")
			normalizedDate = strings.ReplaceAll(normalizedDate, " 日", "日")
			if normalizedDate != foodPermit.ValidTo {
				foodPermit.ValidTo = normalizedDate
				changed = true
			}
		}
	}

	if foodPermit.PermitNo == "" {
		permitRe := regexp.MustCompile(`(?:JY[0-9]{12,}|(?:登记证编号|证书编号|食品经营许可证编号)\s*[:：]\s*([0-9A-Za-z]{6,}))`)
		if matches := permitRe.FindStringSubmatch(rawText); matches != nil {
			candidate := matches[0]
			if matches[1] != "" {
				candidate = matches[1]
			}
			if candidate != foodPermit.PermitNo {
				foodPermit.PermitNo = candidate
				changed = true
			}
		}
	}

	if foodPermit.CompanyName == "" || merchantIsSuspiciousFoodPermitCompanyName(foodPermit.CompanyName) {
		nameRe := regexp.MustCompile(`(?:经营者名称|单位名称|主体名称|商号名称)\s*[:：]?\s*([^\n\r]{2,30})`)
		if matches := nameRe.FindStringSubmatch(rawText); matches != nil {
			candidate := merchantNormalizeCompanyName(strings.TrimSpace(matches[1]))
			if candidate != "" && !merchantIsSuspiciousFoodPermitCompanyName(candidate) && candidate != foodPermit.CompanyName {
				foodPermit.CompanyName = candidate
				changed = true
			}
		}
	}

	if foodPermit.OperatorName == "" {
		operatorRe := regexp.MustCompile(`经营者姓名\s*[:：]?\s*([^\s\n\r,，。]{2,10})`)
		if matches := operatorRe.FindStringSubmatch(rawText); matches != nil {
			candidate := strings.TrimSpace(matches[1])
			if candidate != "" && candidate != foodPermit.OperatorName {
				foodPermit.OperatorName = candidate
				changed = true
			}
		}
	}

	return changed
}

func populateMerchantFoodPermitFromNormalized(foodPermit *MerchantReviewFoodPermitOCRData, result *ocr.FoodPermitResult) bool {
	if foodPermit == nil || result == nil {
		return false
	}

	changed := false
	if rawText := strings.TrimSpace(result.RawText); rawText != "" && rawText != foodPermit.RawText {
		foodPermit.RawText = rawText
		changed = true
	}
	if licenseNumber := strings.TrimSpace(result.LicenseNumber); licenseNumber != "" && foodPermit.PermitNo == "" {
		foodPermit.PermitNo = licenseNumber
		changed = true
	}
	if companyName := merchantNormalizeCompanyName(strings.TrimSpace(result.BusinessName)); companyName != "" && !merchantIsSuspiciousFoodPermitCompanyName(companyName) && (foodPermit.CompanyName == "" || merchantIsSuspiciousFoodPermitCompanyName(foodPermit.CompanyName)) {
		if companyName != foodPermit.CompanyName {
			foodPermit.CompanyName = companyName
			changed = true
		}
	}
	if operatorName := strings.TrimSpace(result.OperatorName); operatorName != "" && foodPermit.OperatorName == "" {
		foodPermit.OperatorName = operatorName
		changed = true
	}
	validFrom, validTo := parseMerchantFoodPermitValidPeriod(result.ValidPeriod)
	if validFrom != "" && foodPermit.ValidFrom == "" {
		foodPermit.ValidFrom = validFrom
		changed = true
	}
	if validTo != "" && foodPermit.ValidTo == "" {
		foodPermit.ValidTo = validTo
		changed = true
	}
	return changed
}

func parseMerchantFoodPermitValidPeriod(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	raw = strings.ReplaceAll(raw, " 年", "年")
	raw = strings.ReplaceAll(raw, "年 ", "年")
	raw = strings.ReplaceAll(raw, " 月", "月")
	raw = strings.ReplaceAll(raw, "月 ", "月")
	raw = strings.ReplaceAll(raw, " 日", "日")
	raw = strings.ReplaceAll(raw, "日 ", "日")
	if strings.Contains(raw, "长期") || strings.Contains(raw, "永久") {
		return "", "长期"
	}
	datePattern := `\d{4}年\d{1,2}月\d{1,2}日`
	rangeRegex := regexp.MustCompile(`(` + datePattern + `)\s*[至到-]\s*(` + datePattern + `)`)
	if matches := rangeRegex.FindStringSubmatch(raw); len(matches) > 2 {
		return matches[1], matches[2]
	}
	singleRegex := regexp.MustCompile(`(` + datePattern + `)`)
	if matches := singleRegex.FindStringSubmatch(raw); len(matches) > 1 {
		return "", matches[1]
	}
	return "", raw
}