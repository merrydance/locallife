package logic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/baofu"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
)

func baofuOpeningAccountType(ownerType string) (string, error) {
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeMerchant, db.BaofuAccountOwnerTypePlatform:
		return db.BaofuAccountTypeBusiness, nil
	case db.BaofuAccountOwnerTypeRider, db.BaofuAccountOwnerTypeOperator:
		return db.BaofuAccountTypePersonal, nil
	default:
		return "", ErrBaofuAccountInvalidOwnerAccount
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

func baofuOpeningLoginNo(ownerType string, ownerID int64) string {
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeMerchant:
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
	case db.BaofuAccountOwnerTypeMerchant, db.BaofuAccountOwnerTypePlatform:
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
		if input.SelfEmployed && strings.TrimSpace(input.CardUserName) != "" {
			fields = append(fields, baofuAccountOpeningProfileField{code: "corporate_mobile", value: input.CorporateMobile})
		}
	case db.BaofuAccountOwnerTypeRider, db.BaofuAccountOwnerTypeOperator:
		fields = append(fields,
			baofuAccountOpeningProfileField{code: "id_card_number", value: firstTrimmed(input.CertificateNo, input.LegalPersonIDNumber)},
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
	case db.BaofuAccountOwnerTypeMerchant, db.BaofuAccountOwnerTypePlatform:
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
		if baofuProfileUsesPrivateBusinessCard(profile) {
			fields = append(fields, baofuAccountOpeningProfileField{code: "corporate_mobile", value: profile.CorporateMobileCiphertext.String})
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

func baofuProfileUsesPrivateBusinessCard(profile db.BaofuAccountOpeningProfile) bool {
	if strings.TrimSpace(profile.AccountType) != db.BaofuAccountTypeBusiness {
		return false
	}
	if strings.TrimSpace(profile.CardUserName.String) == "" {
		return false
	}
	var payload struct {
		SelfEmployed bool `json:"self_employed"`
	}
	if err := json.Unmarshal(profile.SourceSnapshot, &payload); err != nil {
		return false
	}
	return payload.SelfEmployed
}

func missingBaofuProfileFieldCodes(fields []baofuAccountOpeningProfileField) []string {
	missing := make([]string, 0)
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.code)
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
		result.StatusDesc = BaofuAccountOpeningFailureStatusDesc(result.Flow.FailureCode.String)
	} else if strings.TrimSpace(result.State) == db.BaofuAccountOpeningStateProfilePending ||
		strings.TrimSpace(profile.ProfileStatus) != db.BaofuAccountOpeningProfileStatusComplete {
		result.MissingFields = BaofuAccountOpeningProfileMissingFields(profile)
		result.StatusDesc = BaofuAccountOpeningProfilePendingStatusDesc(result.MissingFields)
	}
	return result
}

func BaofuAccountOpeningFailureStatusDesc(failureCode string) string {
	code := strings.TrimSpace(failureCode)
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
