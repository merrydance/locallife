package logic

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	MerchantDocumentReviewCodeBusinessLicenseRequired        = "business_license_required"
	MerchantDocumentReviewCodeFoodLicenseRequired            = "food_license_required"
	MerchantDocumentReviewCodeIDCardFrontRequired            = "id_card_front_required"
	MerchantDocumentReviewCodeIDCardBackRequired             = "id_card_back_required"
	MerchantDocumentReviewCodeBusinessLicenseValidityInvalid = "applyment_business_license_validity_invalid"
	MerchantDocumentReviewCodeApplicationInvalidState        = "application_invalid_state"
	MerchantDocumentReviewCodeLicenseNameUnreadable          = "merchant_business_license_name_unreadable"
	MerchantDocumentReviewCodeFoodPermitNameUnreadable       = "merchant_food_permit_name_unreadable"
	MerchantDocumentReviewCodeFoodPermitNameMismatch         = "merchant_food_permit_name_mismatch"

	merchantOCRReadinessStateReady          = "ready"
	merchantOCRReadinessStatePartial        = "partial"
	merchantOCRReadinessStateProviderFailed = "provider_failed"

	merchantOCRReadinessReasonRequiredFieldMissing = "required_field_missing"
	merchantOCRReadinessReasonFieldUnparseable     = "field_unparseable"
)

type MerchantDocumentReviewInput struct {
	BusinessLicense MerchantReviewBusinessLicenseOCRData
	FoodPermit      MerchantReviewFoodPermitOCRData
	IDCardFront     MerchantReviewIDCardOCRData
	IDCardBack      MerchantReviewIDCardOCRData
}

type MerchantDocumentReviewResult struct {
	LicenseAddress     string
	LicenseName        string
	LicenseLegalPerson string
}

type MerchantDocumentReviewError struct {
	Code    string
	Message string
}

func (err *MerchantDocumentReviewError) Error() string {
	if err == nil {
		return ""
	}
	return err.Message
}

func EvaluateMerchantDocumentReview(input MerchantDocumentReviewInput, now time.Time) (MerchantDocumentReviewResult, error) {
	if err := merchantSubmissionReadinessError(
		input.BusinessLicense.Readiness,
		map[string]string{
			"enterprise_name":      "营业执照企业名称未识别，请重新上传清晰完整的营业执照照片",
			"legal_representative": "营业执照法人姓名未识别，请重新上传清晰完整的营业执照照片",
			"address":              "营业执照注册地址未识别，请重新上传清晰完整的营业执照照片",
			"business_scope":       "营业执照经营范围未识别，请重新上传清晰完整的营业执照照片",
			"valid_period":         "营业执照有效期未识别，请重新上传清晰完整的营业执照照片",
		},
		"营业执照信息未识别，请重新上传清晰的营业执照照片",
		"营业执照信息解析失败，请重新上传清晰完整的营业执照照片",
		"营业执照OCR处理失败，请重新上传清晰完整的营业执照照片",
	); err != nil {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeBusinessLicenseRequired, Message: err.Error()}
	}

	if !merchantIsValidPeriodValid(input.BusinessLicense.ValidPeriod, now) {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeBusinessLicenseValidityInvalid, Message: "营业执照已过期或有效期无法识别，请重新上传在有效期内的营业执照"}
	}
	if !merchantIsCateringBusiness(input.BusinessLicense.EnterpriseName, input.BusinessLicense.BusinessScope) {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeApplicationInvalidState, Message: "营业执照经营范围未识别到餐饮相关内容，请确认经营范围后重试"}
	}

	if err := merchantSubmissionReadinessError(
		input.FoodPermit.Readiness,
		map[string]string{
			"company_name": "食品经营许可证主体名称未识别，请重新上传清晰完整的食品经营许可证照片",
			"valid_to":     "食品经营许可证有效期未识别，请重新上传清晰完整的食品经营许可证照片",
		},
		"食品经营许可证信息未识别，请重新上传清晰完整的食品经营许可证照片",
		"食品经营许可证信息解析失败，请重新上传清晰完整的食品经营许可证照片",
		"食品经营许可证OCR处理失败，请重新上传清晰完整的食品经营许可证照片",
	); err != nil {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeFoodLicenseRequired, Message: err.Error()}
	}
	if !merchantIsFoodPermitValid(input.FoodPermit.ValidTo, now) {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeFoodLicenseRequired, Message: "食品经营许可证已过期或有效期无法识别，请重新上传在有效期内的证件"}
	}

	licenseName := merchantNormalizeCompanyName(input.BusinessLicense.EnterpriseName)
	permitName := merchantNormalizeCompanyName(input.FoodPermit.CompanyName)
	permitOperator := strings.TrimSpace(input.FoodPermit.OperatorName)
	licenseLegalPerson := strings.TrimSpace(input.BusinessLicense.LegalRepresentative)
	if licenseName == "" {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeLicenseNameUnreadable, Message: "未能从营业执照中识别出企业名称，请上传完整清晰的营业执照后重试"}
	}
	matchedByRawText := false
	if permitName != "" && merchantCompanyNamesMatch(licenseName, permitName) {
		matchedByRawText = false
	} else if merchantFoodPermitRawTextContainsCompanyName(input.FoodPermit.RawText, licenseName) {
		permitName = licenseName
		matchedByRawText = true
	} else if permitName == "" && merchantCanUseFoodPermitOperatorFallback(input.FoodPermit.RawText, permitOperator, licenseLegalPerson) {
		permitName = licenseName
		matchedByRawText = true
	}
	if permitName == "" {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeFoodPermitNameUnreadable, Message: "未能从食品经营许可证中识别出主体名称，请上传完整清晰的食品经营许可证后重试"}
	}
	if !merchantCompanyNamesMatch(licenseName, permitName) {
		if merchantIsSuspiciousFoodPermitCompanyName(permitName) {
			return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeFoodPermitNameUnreadable, Message: "未能从食品经营许可证中识别出主体名称，请上传完整清晰的食品经营许可证后重试"}
		}
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeFoodPermitNameMismatch, Message: fmt.Sprintf("食品经营许可证主体名称与营业执照企业名称不一致，请核对后重试。营业执照：%s；食品经营许可证：%s。", licenseName, permitName)}
	}
	if !matchedByRawText && licenseName != permitName {
		if licenseLegalPerson != "" && permitOperator != "" && licenseLegalPerson != permitOperator {
			return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeFoodPermitNameMismatch, Message: fmt.Sprintf("食品经营许可证主体名称与营业执照企业名称未完全一致，且食品经营许可证经营者（%s）与营业执照法人（%s）不一致，请核对证照信息后重试。", permitOperator, licenseLegalPerson)}
		}
	}
	if !merchantIsDateAtLeastDaysAfterNow(input.FoodPermit.ValidTo, 30, now) {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeFoodLicenseRequired, Message: "食品经营许可证有效期需至少超过当前日期30天，请更新证件后重试"}
	}

	if err := merchantSubmissionReadinessError(
		input.IDCardFront.Readiness,
		map[string]string{
			"name":      "身份证姓名未识别，请重新上传清晰的身份证正面照片",
			"id_number": "身份证号未识别，请重新上传清晰的身份证正面照片",
		},
		"身份证正面信息未识别，请重新上传清晰的身份证正面照片",
		"身份证正面信息解析失败，请重新上传清晰的身份证正面照片",
		"身份证正面OCR处理失败，请重新上传清晰的身份证正面照片",
	); err != nil {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeIDCardFrontRequired, Message: err.Error()}
	}

	licenseLegalPerson = strings.TrimSpace(input.BusinessLicense.LegalRepresentative)
	idCardName := strings.TrimSpace(input.IDCardFront.Name)
	if licenseLegalPerson == "" {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeBusinessLicenseRequired, Message: "营业执照法人姓名未识别，请重新上传清晰完整的营业执照照片"}
	}
	if idCardName == "" {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeIDCardFrontRequired, Message: "身份证姓名未识别，请重新上传清晰的身份证正面照片"}
	}
	if licenseLegalPerson != idCardName {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeApplicationInvalidState, Message: fmt.Sprintf("身份证姓名与营业执照法人信息不一致，请核对后重试。身份证：%s；营业执照法人：%s。", idCardName, licenseLegalPerson)}
	}

	if err := merchantSubmissionReadinessError(
		input.IDCardBack.Readiness,
		map[string]string{"valid_date": "身份证有效期未识别，请重新上传清晰的身份证背面照片"},
		"身份证背面信息未识别，请重新上传清晰的身份证背面照片",
		"身份证背面信息解析失败，请重新上传清晰的身份证背面照片",
		"身份证背面OCR处理失败，请重新上传清晰的身份证背面照片",
	); err != nil {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeIDCardBackRequired, Message: err.Error()}
	}
	if !merchantIsIDCardValidPeriodValid(input.IDCardBack.ValidDate, now) {
		return MerchantDocumentReviewResult{}, &MerchantDocumentReviewError{Code: MerchantDocumentReviewCodeIDCardBackRequired, Message: "法人身份证已过期或有效期无法识别，请重新上传在有效期内的身份证背面照片"}
	}

	return MerchantDocumentReviewResult{
		LicenseAddress:     strings.TrimSpace(input.BusinessLicense.Address),
		LicenseName:        licenseName,
		LicenseLegalPerson: licenseLegalPerson,
	}, nil
}

func merchantSubmissionReadinessError(readiness *MerchantReviewOCRReadiness, missingFieldMessages map[string]string, fallbackMissing string, fallbackUnparseable string, fallbackProvider string) error {
	if readiness == nil || readiness.State == "" || readiness.State == merchantOCRReadinessStateReady {
		return nil
	}

	switch readiness.State {
	case merchantOCRReadinessStateProviderFailed:
		return errors.New(fallbackProvider)
	case merchantOCRReadinessStatePartial:
		switch readiness.ReasonCode {
		case merchantOCRReadinessReasonRequiredFieldMissing:
			return errors.New(merchantFirstReadinessFieldMessage(readiness.MissingFields, missingFieldMessages, fallbackMissing))
		case merchantOCRReadinessReasonFieldUnparseable:
			return errors.New(merchantFirstReadinessFieldMessage(readiness.UnparseableFields, missingFieldMessages, fallbackUnparseable))
		default:
			return errors.New(fallbackMissing)
		}
	default:
		return errors.New(fallbackMissing)
	}
}

func merchantFirstReadinessFieldMessage(fields []string, messages map[string]string, fallback string) string {
	for _, field := range fields {
		if msg, ok := messages[field]; ok && msg != "" {
			return msg
		}
	}
	return fallback
}

func merchantCompanyNamesMatch(a, b string) bool {
	if a == b {
		return true
	}
	ra := []rune(a)
	rb := []rune(b)
	if len(ra) > len(rb) {
		ra, rb = rb, ra
	}
	diff := len(rb) - len(ra)
	if diff > 2 {
		return false
	}
	for i, c := range ra {
		if rb[i] != c {
			return false
		}
	}
	return true
}

func merchantFoodPermitRawTextContainsCompanyName(rawText, companyName string) bool {
	normalizedText := merchantNormalizeOCRSearchText(rawText)
	normalizedCompanyName := merchantNormalizeCompanyName(companyName)
	if normalizedText == "" || normalizedCompanyName == "" {
		return false
	}
	return strings.Contains(normalizedText, normalizedCompanyName)
}

func merchantCanUseFoodPermitOperatorFallback(rawText, permitOperator, licenseLegalPerson string) bool {
	if merchantNormalizeCompanyName(permitOperator) == "" || merchantNormalizeCompanyName(licenseLegalPerson) == "" {
		return false
	}
	if merchantNormalizeCompanyName(permitOperator) != merchantNormalizeCompanyName(licenseLegalPerson) {
		return false
	}
	rawText = merchantNormalizeOCRSearchText(rawText)
	if rawText == "" {
		return false
	}
	for _, keyword := range []string{"登记证", "小餐饮", "小作坊"} {
		if strings.Contains(rawText, keyword) {
			return true
		}
	}
	return false
}

func merchantNormalizeOCRSearchText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		" ", "",
		"\t", "",
		"\n", "",
		"\r", "",
		"（", "(",
		"）", ")",
		"：", "",
		":", "",
		"，", "",
		",", "",
		"。", "",
		"、", "",
		"；", "",
		";", "",
		"《", "",
		"》", "",
	)
	return replacer.Replace(text)
}

func merchantIsSuspiciousFoodPermitCompanyName(name string) bool {
	name = merchantNormalizeCompanyName(name)
	if name == "" {
		return true
	}
	if len([]rune(name)) > 30 {
		return true
	}
	for _, keyword := range []string{"地址", "经营场所", "面积", "办理", "许可证", "登记证", "项目", "食品", "小作坊", "小餐饮", "路东", "路西", "请", "《"} {
		if strings.Contains(name, keyword) {
			return true
		}
	}
	return false
}

func merchantNormalizeCompanyName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "\t", "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "（", "(")
	name = strings.ReplaceAll(name, "）", ")")
	return name
}

func merchantIsDateAtLeastDaysAfterNow(dateStr string, days int, now time.Time) bool {
	if strings.TrimSpace(dateStr) == "" {
		return false
	}
	if strings.Contains(dateStr, "长期") || strings.Contains(dateStr, "永久") {
		return true
	}
	parsed, err := parseRiderFlexibleDocumentEndDate(dateStr)
	if err != nil {
		return false
	}
	return parsed.After(now.AddDate(0, 0, days))
}

func merchantIsValidPeriodValid(validPeriod string, now time.Time) bool {
	if strings.TrimSpace(validPeriod) == "" {
		return false
	}
	if strings.Contains(validPeriod, "长期") || strings.Contains(validPeriod, "永久") {
		return true
	}
	expiresAt, err := parseRiderFlexibleDocumentEndDate(validPeriod)
	if err != nil {
		return false
	}
	return expiresAt.After(now)
}

func merchantIsCateringBusiness(enterpriseName, businessScope string) bool {
	text := enterpriseName + " " + businessScope
	for _, keyword := range []string{"餐饮", "餐厅", "饭店", "饭馆", "酒楼", "酒家", "快餐", "小吃", "面馆", "面店", "粉店", "火锅", "烧烤", "串串", "麻辣烫", "奶茶", "茶饮", "咖啡", "甜品", "食品", "食堂", "厨房", "外卖", "菜", "料理", "美食"} {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func merchantIsFoodPermitValid(validTo string, now time.Time) bool {
	if strings.TrimSpace(validTo) == "" {
		return false
	}
	if strings.Contains(validTo, "长期") || strings.Contains(validTo, "永久") {
		return true
	}
	expiresAt, err := parseRiderFlexibleDocumentEndDate(validTo)
	if err != nil {
		return false
	}
	return expiresAt.After(now)
}

func merchantIsIDCardValidPeriodValid(validDate string, now time.Time) bool {
	if strings.TrimSpace(validDate) == "" {
		return false
	}
	if strings.Contains(validDate, "长期") || strings.Contains(validDate, "永久") {
		return true
	}
	expiresAt, err := parseRiderFlexibleDocumentEndDate(validDate)
	if err != nil {
		return false
	}
	return expiresAt.After(now)
}
