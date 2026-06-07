package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
)

func baofuOpeningAccountType(ownerType string, accountOpeningMode string) (string, error) {
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeMerchant:
		switch strings.ToLower(strings.TrimSpace(accountOpeningMode)) {
		case "", db.BaofuAccountTypeBusiness:
			return db.BaofuAccountTypeBusiness, nil
		case db.BaofuAccountTypePersonal:
			return db.BaofuAccountTypePersonal, nil
		default:
			return "", ErrBaofuAccountInvalidOwnerAccount
		}
	case db.BaofuAccountOwnerTypePlatform:
		return db.BaofuAccountTypeBusiness, nil
	case db.BaofuAccountOwnerTypeRider, db.BaofuAccountOwnerTypeOperator:
		return db.BaofuAccountTypePersonal, nil
	default:
		return "", ErrBaofuAccountInvalidOwnerAccount
	}
}

func baofuAccountOpeningModeConflictError() error {
	return NewRequestError(http.StatusConflict, errors.New("该商户已有不同开户方式的宝付账户或开户流程，请在当前流程完成后再切换开户方式"))
}

func baofuAccountOpeningModeForInput(ownerType string, accountType string, input BaofuAccountOpeningProfileInput) string {
	switch strings.TrimSpace(accountType) {
	case db.BaofuAccountTypePersonal:
		if strings.TrimSpace(ownerType) == db.BaofuAccountOwnerTypeMerchant {
			return db.BaofuAccountOpeningModeMerchantPersonalMicro
		}
		return db.BaofuAccountOpeningModePersonal
	case db.BaofuAccountTypeBusiness:
		if strings.TrimSpace(ownerType) == db.BaofuAccountOwnerTypeMerchant && input.SelfEmployed {
			return db.BaofuAccountOpeningModeIndividualBusinessPrivate
		}
		return db.BaofuAccountOpeningModeBusinessPublic
	default:
		return ""
	}
}

func baofuAccountOpeningModeForOwnerAccount(ownerType string, accountType string) string {
	return baofuAccountOpeningModeForInput(ownerType, accountType, BaofuAccountOpeningProfileInput{})
}

func baofuAccountOpeningModeForProfile(profile db.BaofuAccountOpeningProfile) string {
	if mode := normalizeBaofuAccountOpeningMode(profile.OpeningMode); mode != "" {
		return mode
	}
	if mode := baofuAccountOpeningModeFromSnapshot(profile.SourceSnapshot); mode != "" {
		return mode
	}
	if strings.TrimSpace(profile.OwnerType) == db.BaofuAccountOwnerTypeMerchant &&
		strings.TrimSpace(profile.AccountType) == db.BaofuAccountTypeBusiness &&
		baofuAccountOpeningSnapshotSelfEmployed(profile.SourceSnapshot) {
		return db.BaofuAccountOpeningModeIndividualBusinessPrivate
	}
	return baofuAccountOpeningModeForOwnerAccount(profile.OwnerType, profile.AccountType)
}

func baofuAccountOpeningModeForFlow(flow db.BaofuAccountOpeningFlow) string {
	if mode := normalizeBaofuAccountOpeningMode(flow.OpeningMode); mode != "" {
		return mode
	}
	return baofuAccountOpeningModeForOwnerAccount(flow.OwnerType, flow.AccountType)
}

func baofuAccountOpeningModeForBinding(binding db.BaofuAccountBinding) string {
	if mode := normalizeBaofuAccountOpeningMode(binding.OpeningMode); mode != "" {
		return mode
	}
	return baofuAccountOpeningModeForOwnerAccount(binding.OwnerType, binding.AccountType)
}

func baofuAccountOpeningModeFromSnapshot(raw []byte) string {
	var payload struct {
		OpeningMode  string `json:"opening_mode"`
		SelfEmployed bool   `json:"self_employed"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if mode := normalizeBaofuAccountOpeningMode(payload.OpeningMode); mode != "" {
		return mode
	}
	return ""
}

func baofuAccountOpeningSnapshotSelfEmployed(raw []byte) bool {
	var payload struct {
		SelfEmployed bool `json:"self_employed"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	return payload.SelfEmployed
}

func normalizeBaofuAccountOpeningMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case db.BaofuAccountOpeningModePersonal:
		return db.BaofuAccountOpeningModePersonal
	case db.BaofuAccountOpeningModeMerchantPersonalMicro:
		return db.BaofuAccountOpeningModeMerchantPersonalMicro
	case db.BaofuAccountOpeningModeBusinessPublic:
		return db.BaofuAccountOpeningModeBusinessPublic
	case db.BaofuAccountOpeningModeIndividualBusinessPrivate:
		return db.BaofuAccountOpeningModeIndividualBusinessPrivate
	default:
		return ""
	}
}

func baofuOpeningRequiresUserFee(ownerType string) bool {
	return strings.TrimSpace(ownerType) == db.BaofuAccountOwnerTypeRider || strings.TrimSpace(ownerType) == db.BaofuAccountOwnerTypeOperator
}

func baofuOpeningFlowInProviderProgress(state string) bool {
	switch strings.TrimSpace(state) {
	case db.BaofuAccountOpeningStateOpeningProcessing,
		db.BaofuAccountOpeningStateMerchantReportProcessing,
		db.BaofuAccountOpeningStateAppletAuthPending:
		return true
	default:
		return false
	}
}

func baofuVerifyFeeAttach(ownerType string, ownerID int64) string {
	return fmt.Sprintf("business:%s;owner_type:%s;owner_id:%d;purpose:initial_open", db.PaymentBusinessTypeBaofuAccountVerifyFee, strings.TrimSpace(ownerType), ownerID)
}

func baofuOpeningLoginNo(ownerType string, ownerID int64, accountType string, flowID int64) string {
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeMerchant:
		if strings.TrimSpace(accountType) == db.BaofuAccountTypePersonal {
			base := fmt.Sprintf("LLBFOMP%010d", ownerID)
			if flowID <= 0 {
				return base
			}
			suffix := strconv.FormatInt(flowID, 10)
			loginNo := base + "R" + suffix
			if len(loginNo) <= 32 {
				return loginNo
			}
			return base + "R" + strings.ToUpper(strconv.FormatInt(flowID, 36))
		}
		return fmt.Sprintf("LLBFOM%010d", ownerID)
	case db.BaofuAccountOwnerTypePlatform:
		return "LLBFOP0000000000"
	case db.BaofuAccountOwnerTypeRider:
		return fmt.Sprintf("LLBFOR%010d", ownerID)
	case db.BaofuAccountOwnerTypeOperator:
		return fmt.Sprintf("LLBFOO%010d", ownerID)
	default:
		return fmt.Sprintf("LLBFOX%010d", ownerID)
	}
}

func baofuProfileComplete(ownerType string, input BaofuAccountOpeningProfileInput) bool {
	return len(BaofuAccountOpeningInputMissingFields(ownerType, input)) == 0
}

func BaofuAccountOpeningInputMissingFields(ownerType string, input BaofuAccountOpeningProfileInput) []string {
	fields := []baofuAccountOpeningProfileField{
		{code: "legal_name", value: input.LegalName},
		{code: "bank_account_no", value: input.BankAccountNo},
	}
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeMerchant:
		if strings.TrimSpace(input.AccountType) == db.BaofuAccountTypePersonal {
			fields = append(fields,
				baofuAccountOpeningProfileField{code: "id_card_number", value: input.CertificateNo},
				baofuAccountOpeningProfileField{code: "bank_mobile", value: input.BankMobile},
			)
			return missingBaofuProfileFieldCodes(fields)
		}
		fields = append(fields,
			baofuAccountOpeningProfileField{code: "business_license_number", value: input.BusinessLicenseNo},
			baofuAccountOpeningProfileField{code: "legal_person_name", value: input.LegalPersonName},
			baofuAccountOpeningProfileField{code: "legal_person_id_number", value: input.LegalPersonIDNumber},
			baofuAccountOpeningProfileField{code: "email", value: input.Email},
			baofuAccountOpeningProfileField{code: "bank_name", value: input.BankName},
			baofuAccountOpeningProfileField{code: "deposit_bank_province", value: input.DepositBankProvince},
			baofuAccountOpeningProfileField{code: "deposit_bank_city", value: input.DepositBankCity},
			baofuAccountOpeningProfileField{code: "deposit_bank_name", value: input.DepositBankName},
		)
		if !baofuDepositBankLocationLooksConsistent(input.DepositBankProvince, input.DepositBankCity, input.DepositBankName) {
			fields = append(fields, baofuAccountOpeningProfileField{code: "deposit_bank_city", value: ""})
		}
		if baofuAccountOpeningModeForInput(ownerType, input.AccountType, input) == db.BaofuAccountOpeningModeIndividualBusinessPrivate {
			fields = append(fields,
				baofuAccountOpeningProfileField{code: "card_user_name", value: input.CardUserName},
				baofuAccountOpeningProfileField{code: "corporate_mobile", value: input.CorporateMobile},
			)
		}
	case db.BaofuAccountOwnerTypePlatform:
		fields = append(fields,
			baofuAccountOpeningProfileField{code: "business_license_number", value: input.BusinessLicenseNo},
			baofuAccountOpeningProfileField{code: "legal_person_name", value: input.LegalPersonName},
			baofuAccountOpeningProfileField{code: "legal_person_id_number", value: input.LegalPersonIDNumber},
			baofuAccountOpeningProfileField{code: "email", value: input.Email},
			baofuAccountOpeningProfileField{code: "bank_name", value: input.BankName},
			baofuAccountOpeningProfileField{code: "deposit_bank_province", value: input.DepositBankProvince},
			baofuAccountOpeningProfileField{code: "deposit_bank_city", value: input.DepositBankCity},
			baofuAccountOpeningProfileField{code: "deposit_bank_name", value: input.DepositBankName},
		)
		if !baofuDepositBankLocationLooksConsistent(input.DepositBankProvince, input.DepositBankCity, input.DepositBankName) {
			fields = append(fields, baofuAccountOpeningProfileField{code: "deposit_bank_city", value: ""})
		}
		if baofuAccountOpeningModeForInput(ownerType, input.AccountType, input) == db.BaofuAccountOpeningModeIndividualBusinessPrivate {
			fields = append(fields,
				baofuAccountOpeningProfileField{code: "card_user_name", value: input.CardUserName},
				baofuAccountOpeningProfileField{code: "corporate_mobile", value: input.CorporateMobile},
			)
		}
	case db.BaofuAccountOwnerTypeRider, db.BaofuAccountOwnerTypeOperator:
		fields = append(fields,
			baofuAccountOpeningProfileField{code: "id_card_number", value: input.CertificateNo},
			baofuAccountOpeningProfileField{code: "bank_mobile", value: input.BankMobile},
		)
	default:
		return []string{"owner_type"}
	}
	return missingBaofuProfileFieldCodes(fields)
}

func BaofuAccountOpeningProfileMissingFields(profile db.BaofuAccountOpeningProfile) []string {
	fields := []baofuAccountOpeningProfileField{
		{code: "legal_name", value: profile.LegalName.String},
		{code: "bank_account_no", value: profile.BankAccountNoCiphertext.String},
	}
	switch strings.TrimSpace(profile.OwnerType) {
	case db.BaofuAccountOwnerTypeMerchant:
		if strings.TrimSpace(profile.AccountType) == db.BaofuAccountTypePersonal {
			fields = append(fields,
				baofuAccountOpeningProfileField{code: "id_card_number", value: profile.CertificateNoCiphertext.String},
				baofuAccountOpeningProfileField{code: "bank_mobile", value: profile.BankMobileCiphertext.String},
			)
			return missingBaofuProfileFieldCodes(fields)
		}
		fields = append(fields,
			baofuAccountOpeningProfileField{code: "business_license_number", value: profile.CertificateNoCiphertext.String},
			baofuAccountOpeningProfileField{code: "legal_person_name", value: profile.CorporateName.String},
			baofuAccountOpeningProfileField{code: "legal_person_id_number", value: profile.CorporateCertIDCiphertext.String},
			baofuAccountOpeningProfileField{code: "email", value: profile.EmailCiphertext.String},
			baofuAccountOpeningProfileField{code: "bank_name", value: profile.BankName.String},
			baofuAccountOpeningProfileField{code: "deposit_bank_province", value: profile.DepositBankProvince.String},
			baofuAccountOpeningProfileField{code: "deposit_bank_city", value: profile.DepositBankCity.String},
			baofuAccountOpeningProfileField{code: "deposit_bank_name", value: profile.DepositBankName.String},
		)
		if !baofuDepositBankLocationLooksConsistent(profile.DepositBankProvince.String, profile.DepositBankCity.String, profile.DepositBankName.String) {
			fields = append(fields, baofuAccountOpeningProfileField{code: "deposit_bank_city", value: ""})
		}
		if baofuAccountOpeningModeForProfile(profile) == db.BaofuAccountOpeningModeIndividualBusinessPrivate {
			fields = append(fields,
				baofuAccountOpeningProfileField{code: "card_user_name", value: profile.CardUserName.String},
				baofuAccountOpeningProfileField{code: "corporate_mobile", value: profile.CorporateMobileCiphertext.String},
			)
		}
	case db.BaofuAccountOwnerTypePlatform:
		fields = append(fields,
			baofuAccountOpeningProfileField{code: "business_license_number", value: profile.CertificateNoCiphertext.String},
			baofuAccountOpeningProfileField{code: "legal_person_name", value: profile.CorporateName.String},
			baofuAccountOpeningProfileField{code: "legal_person_id_number", value: profile.CorporateCertIDCiphertext.String},
			baofuAccountOpeningProfileField{code: "email", value: profile.EmailCiphertext.String},
			baofuAccountOpeningProfileField{code: "bank_name", value: profile.BankName.String},
			baofuAccountOpeningProfileField{code: "deposit_bank_province", value: profile.DepositBankProvince.String},
			baofuAccountOpeningProfileField{code: "deposit_bank_city", value: profile.DepositBankCity.String},
			baofuAccountOpeningProfileField{code: "deposit_bank_name", value: profile.DepositBankName.String},
		)
		if !baofuDepositBankLocationLooksConsistent(profile.DepositBankProvince.String, profile.DepositBankCity.String, profile.DepositBankName.String) {
			fields = append(fields, baofuAccountOpeningProfileField{code: "deposit_bank_city", value: ""})
		}
		if baofuAccountOpeningModeForProfile(profile) == db.BaofuAccountOpeningModeIndividualBusinessPrivate {
			fields = append(fields,
				baofuAccountOpeningProfileField{code: "card_user_name", value: profile.CardUserName.String},
				baofuAccountOpeningProfileField{code: "corporate_mobile", value: profile.CorporateMobileCiphertext.String},
			)
		}
	case db.BaofuAccountOwnerTypeRider, db.BaofuAccountOwnerTypeOperator:
		fields = append(fields,
			baofuAccountOpeningProfileField{code: "id_card_number", value: profile.CertificateNoCiphertext.String},
			baofuAccountOpeningProfileField{code: "bank_mobile", value: profile.BankMobileCiphertext.String},
		)
	default:
		return []string{"owner_type"}
	}
	return missingBaofuProfileFieldCodes(fields)
}

func baofuDepositBankLocationLooksConsistent(province, city, branch string) bool {
	province = strings.TrimSpace(province)
	city = strings.TrimSpace(city)
	if city == "" {
		return false
	}
	for _, municipality := range []string{"北京市", "天津市", "上海市", "重庆市"} {
		if city == municipality && province != "" && province != municipality {
			return false
		}
	}
	return true
}

func baofuProfileUsesPrivateBusinessCard(profile db.BaofuAccountOpeningProfile) bool {
	if strings.TrimSpace(profile.AccountType) != db.BaofuAccountTypeBusiness {
		return false
	}
	return baofuAccountOpeningModeForProfile(profile) == db.BaofuAccountOpeningModeIndividualBusinessPrivate
}

func missingBaofuProfileFieldCodes(fields []baofuAccountOpeningProfileField) []string {
	missing := make([]string, 0)
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			if _, ok := seen[field.code]; ok {
				continue
			}
			missing = append(missing, field.code)
			seen[field.code] = struct{}{}
		}
	}
	return missing
}

func BaofuAccountOpeningProfilePendingStatusDesc(missingFields []string) string {
	if len(missingFields) == 0 {
		return "请补充开户资料后继续"
	}
	labels := make([]string, 0, len(missingFields))
	for _, field := range missingFields {
		labels = append(labels, baofuAccountOpeningProfileFieldLabel(field))
	}
	return "请补充开户资料：" + strings.Join(labels, "、")
}

func baofuAccountOpeningProfileFieldLabel(field string) string {
	switch strings.TrimSpace(field) {
	case "legal_name":
		return "企业名称/姓名"
	case "business_license_number":
		return "营业执照号"
	case "legal_person_name":
		return "法人姓名"
	case "legal_person_id_number":
		return "法人身份证号"
	case "corporate_mobile":
		return "法人手机号"
	case "email":
		return "联系邮箱"
	case "bank_account_no":
		return "银行卡/对公账号"
	case "card_user_name":
		return "法人银行卡户名"
	case "bank_name":
		return "开户银行"
	case "deposit_bank_province":
		return "开户省份"
	case "deposit_bank_city":
		return "开户城市"
	case "deposit_bank_name":
		return "开户支行"
	case "id_card_number":
		return "身份证号"
	case "bank_mobile":
		return "银行预留手机号"
	default:
		return field
	}
}

func baofuCorporateCertType(accountType string) string {
	if accountType == db.BaofuAccountTypeBusiness {
		return baofucontracts.OfficialCertificateTypeID
	}
	return ""
}

func baofuOpeningResult(flow db.BaofuAccountOpeningFlow, profile db.BaofuAccountOpeningProfile) BaofuAccountOpeningResult {
	result := BaofuAccountOpeningResult{State: strings.TrimSpace(flow.State), Label: baofuOnboardingStateLabel(flow.State), Flow: flow, Profile: profile}
	if strings.TrimSpace(result.State) == db.BaofuAccountOpeningStateFailed {
		result.StatusDesc = BaofuAccountOpeningFailureStatusDesc(result.Flow.FailureCode.String, result.Flow.FailureMessage.String)
	} else if strings.TrimSpace(result.State) == db.BaofuAccountOpeningStateProfilePending ||
		strings.TrimSpace(profile.ProfileStatus) != db.BaofuAccountOpeningProfileStatusComplete {
		result.MissingFields = BaofuAccountOpeningProfileMissingFields(profile)
		result.StatusDesc = BaofuAccountOpeningProfilePendingStatusDesc(result.MissingFields)
	}
	return result
}

func BaofuAccountOpeningFailureStatusDesc(failureCode string, failureMessage ...string) string {
	code := strings.TrimSpace(failureCode)
	for _, message := range failureMessage {
		if text := baofu.UserVisibleUpstreamReason(code, message); text != "" {
			return text
		}
	}
	if code == "" {
		return "开户未通过，请核对资料后重试"
	}
	classified := baofu.ClassifyBaofuError(code, "")
	if msg := strings.TrimSpace(classified.PublicMessage); msg != "" {
		return msg
	}
	return "开户未通过，请核对资料后重试"
}

func baofuOpeningSnapshot(payload map[string]any) []byte {
	body, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{}`)
	}
	return body
}

func baofuOpeningProviderFailureSnapshot(failureCode string, diagnostic []byte) []byte {
	payload := map[string]any{
		"state": db.BaofuAccountOpeningStateFailed,
	}
	if code := strings.TrimSpace(failureCode); code != "" {
		payload["failure_code"] = code
	}
	if providerDiagnostic := baofuOpeningProviderDiagnostic(diagnostic); providerDiagnostic != nil {
		payload["provider_diagnostic"] = providerDiagnostic
	}
	return baofuOpeningSnapshot(payload)
}

func baofuOpeningProviderDiagnostic(raw []byte) map[string]any {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload) == 0 {
		return nil
	}
	safe := make(map[string]any, len(payload))
	for _, key := range []string{
		"provider",
		"capability",
		"operation",
		"http_status",
		"sys_resp_code",
		"sys_resp_desc_present",
		"business_failure",
		"source_path",
		"ret_code",
		"top_error_code",
		"top_error_message_sanitized",
		"top_error_message_present",
		"result_state",
		"result_error_code",
		"result_error_message_sanitized",
		"result_error_message_present",
	} {
		if value, ok := payload[key]; ok {
			if safeValue, ok := baofuSafeDiagnosticValue(key, value); ok {
				safe[key] = safeValue
			}
		}
	}
	if len(safe) == 0 {
		return nil
	}
	return safe
}

func baofuSafeDiagnosticValue(key string, value any) (any, bool) {
	switch key {
	case "provider":
		if text, ok := baofuSafeDiagnosticString(value); ok && text == "baofu" {
			return text, true
		}
	case "capability":
		if text, ok := baofuSafeDiagnosticString(value); ok && text == "account" {
			return text, true
		}
	case "source_path":
		if text, ok := baofuSafeDiagnosticString(value); ok && baofuSafeDiagnosticSourcePath(text) {
			return text, true
		}
	case "operation", "sys_resp_code", "ret_code", "top_error_code", "result_state", "result_error_code":
		if text, ok := baofuSafeDiagnosticString(value); ok && baofuSafeDiagnosticToken(text) {
			return text, true
		}
	case "top_error_message_sanitized", "result_error_message_sanitized":
		if text, ok := baofuSafeDiagnosticString(value); ok {
			if sanitized := baofu.SanitizeUpstreamMessageForRecord(text); sanitized != "" {
				return sanitized, true
			}
		}
	case "http_status":
		if number, ok := baofuSafeDiagnosticNumber(value); ok && number >= 100 && number < 600 {
			return number, true
		}
	case "sys_resp_desc_present", "business_failure", "top_error_message_present", "result_error_message_present":
		if flag, ok := value.(bool); ok && flag {
			return flag, true
		}
	}
	return nil, false
}

func baofuSafeDiagnosticString(value any) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	return text, text != ""
}

func baofuSafeDiagnosticSourcePath(value string) bool {
	switch strings.TrimSpace(value) {
	case "body",
		"body.retCode",
		"body.errorCode",
		"body.state",
		"body.result[0]",
		"body.result[0].errorCode":
		return true
	default:
		return false
	}
}

func baofuSafeDiagnosticToken(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-' || r == '.' || r == ':':
		default:
			return false
		}
	}
	return true
}

func baofuSafeDiagnosticNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func baofuOpeningRequestSnapshot(req baofucontracts.OpenAccountRequest) []byte {
	return baofuOpeningSnapshot(map[string]any{
		"owner_type":           req.OwnerType,
		"owner_id":             req.OwnerID,
		"account_type":         req.AccountType,
		"out_request_no":       req.OutRequestNo,
		"login_no":             req.LoginNo,
		"industry_id":          req.IndustryID,
		"need_upload_file":     false,
		"qualification_upload": false,
	})
}

func encryptOptional(encryptor util.DataEncryptor, plaintext string) (string, error) {
	trimmed := strings.TrimSpace(plaintext)
	if trimmed == "" {
		return "", nil
	}
	return util.EncryptSensitiveField(encryptor, trimmed)
}

func decryptOptional(encryptor util.DataEncryptor, ciphertext string) (string, error) {
	trimmed := strings.TrimSpace(ciphertext)
	if trimmed == "" {
		return "", nil
	}
	return util.DecryptSensitiveField(encryptor, trimmed)
}

func pgText(value string) pgtype.Text {
	value = strings.TrimSpace(value)
	return pgtype.Text{String: value, Valid: value != ""}
}

func firstTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func maskSensitiveTail(value string, visibleTail int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if visibleTail <= 0 {
		return strings.Repeat("*", len(value))
	}
	if len(value) <= visibleTail {
		return strings.Repeat("*", len(value))
	}
	return strings.Repeat("*", len(value)-visibleTail) + value[len(value)-visibleTail:]
}

func maskBankAccount(value string) string {
	return maskSensitiveTail(value, 4)
}

func maskMobile(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 7 {
		return maskSensitiveTail(value, 4)
	}
	return value[:3] + "****" + value[len(value)-4:]
}

func maskEmail(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.SplitN(value, "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return maskSensitiveTail(value, 4)
	}
	return parts[0][:1] + "***@" + parts[1]
}
