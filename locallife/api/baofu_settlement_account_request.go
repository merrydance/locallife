package api

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

func decodeBaofuSettlementAccountRequest(ctx *gin.Context, scope baofuSettlementAccountScope) (baofuSettlementAccountRequest, error) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		return baofuSettlementAccountRequest{}, err
	}
	if strings.TrimSpace(string(body)) == "" {
		return baofuSettlementAccountRequest{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return baofuSettlementAccountRequest{}, err
	}
	if err := rejectClientControlledBaofuSettlementAccountFields(payload); err != nil {
		return baofuSettlementAccountRequest{}, err
	}
	if err := rejectRoleInvalidBaofuSettlementAccountProfileFields(payload, scope); err != nil {
		return baofuSettlementAccountRequest{}, err
	}
	var req baofuSettlementAccountRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return baofuSettlementAccountRequest{}, err
	}
	if req.Profile.isZero() {
		var directProfile baofuSettlementAccountProfileRequest
		if err := json.Unmarshal(body, &directProfile); err != nil {
			return baofuSettlementAccountRequest{}, err
		}
		req.Profile = directProfile
	}
	return req, nil
}

func baofuSettlementAccountDecodeErrorPublicMessage(err error) string {
	var fieldErr baofuSettlementAccountClientControlledFieldError
	if errors.As(err, &fieldErr) {
		return "开户请求包含服务端生成字段，请刷新页面后重试；如持续出现请联系平台处理"
	}
	var roleFieldErr baofuSettlementAccountRoleFieldError
	if errors.As(err, &roleFieldErr) {
		return "开户资料字段不适用于当前角色，请刷新页面后重新提交"
	}
	return "开户请求参数不正确，请刷新页面后重试；如持续出现请联系平台处理"
}

func rejectClientControlledBaofuSettlementAccountFields(payload map[string]any) error {
	for _, field := range []string{
		"out_request_no",
		"login_no",
		"owner_type",
		"owner_id",
		"account_type",
		"certificate_type",
		"industry",
		"industry_id",
		"qualification",
		"qualification_trans_serial_no",
		"qualificationTransSerialNo",
		"platform",
		"platform_no",
		"platformNo",
		"platform_terminal_id",
		"platformTerminalId",
		"self_employed",
		"customer_name",
		"alias_name",
		"corporate_cert_type",
		"corporate_cert_id",
		"corporate_mobile",
	} {
		if hasBaofuSettlementAccountField(payload, field) {
			return baofuSettlementAccountClientControlledFieldError{Field: field}
		}
	}
	return nil
}

func rejectRoleInvalidBaofuSettlementAccountProfileFields(payload map[string]any, scope baofuSettlementAccountScope) error {
	profile, ok := baofuSettlementAccountProfilePayload(payload)
	if !ok {
		return nil
	}
	allowed := baofuSettlementAccountAllowedProfileFields(scope.OwnerType)
	for field := range profile {
		if _, ok := allowed[field]; !ok {
			return baofuSettlementAccountRoleFieldError{
				Field:     field,
				Role:      scope.Audience,
				OwnerType: scope.OwnerType,
			}
		}
	}
	return nil
}

func baofuSettlementAccountProfilePayload(payload map[string]any) (map[string]any, bool) {
	nested, ok := payload["profile"].(map[string]any)
	if ok {
		return nested, true
	}
	if _, hasProfile := payload["profile"]; hasProfile {
		return nil, false
	}
	return payload, true
}

func baofuSettlementAccountAllowedProfileFields(ownerType string) map[string]struct{} {
	fields := []string{"bank_account_no", "bank_account_number", "account_number"}
	switch strings.TrimSpace(ownerType) {
	case db.BaofuAccountOwnerTypeMerchant:
		fields = append(fields,
			"legal_name",
			"business_license_number",
			"legal_person_name",
			"legal_person_id_number",
			"email",
			"bank_name",
			"deposit_bank_province",
			"deposit_bank_city",
			"deposit_bank_name",
			"contact_name",
			"contact_mobile",
		)
	case db.BaofuAccountOwnerTypePlatform:
		fields = append(fields,
			"legal_name",
			"business_license_number",
			"legal_person_name",
			"legal_person_id_number",
			"email",
			"bank_name",
			"deposit_bank_province",
			"deposit_bank_city",
			"deposit_bank_name",
			"contact_name",
			"contact_mobile",
		)
	case db.BaofuAccountOwnerTypeRider, db.BaofuAccountOwnerTypeOperator:
		fields = append(fields,
			"real_name",
			"account_name",
			"legal_name",
			"id_card_number",
			"certificate_no",
			"bank_mobile",
			"mobile",
			"phone",
			"card_user_name",
			"bank_name",
		)
	}
	allowed := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		allowed[field] = struct{}{}
	}
	return allowed
}

func hasBaofuSettlementAccountField(payload map[string]any, field string) bool {
	if _, ok := payload[field]; ok {
		return true
	}
	nested, ok := payload["profile"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = nested[field]
	return ok
}

func (req baofuSettlementAccountRequest) toOpeningProfileInput() *logic.BaofuAccountOpeningProfileInput {
	profile := req.Profile
	if profile.isZero() {
		return nil
	}
	return &logic.BaofuAccountOpeningProfileInput{
		LegalName:           firstNonBlank(profile.LegalName, profile.RealName, profile.AccountName, profile.CardUserName),
		CertificateNo:       firstNonBlank(profile.CertificateNo, profile.IDCardNumber, profile.LegalPersonIDNumber),
		BusinessLicenseNo:   profile.BusinessLicenseNumber,
		LegalPersonName:     profile.LegalPersonName,
		LegalPersonIDNumber: profile.LegalPersonIDNumber,
		Email:               profile.Email,
		BankAccountNo:       firstNonBlank(profile.BankAccountNo, profile.BankAccountNumber, profile.AccountNumber),
		BankMobile:          firstNonBlank(profile.BankMobile, profile.Mobile, profile.Phone),
		BankName:            profile.BankName,
		DepositBankProvince: profile.DepositBankProvince,
		DepositBankCity:     profile.DepositBankCity,
		DepositBankName:     profile.DepositBankName,
		ContactName:         profile.ContactName,
		ContactMobile:       profile.ContactMobile,
		CardUserName:        firstNonBlank(profile.CardUserName, profile.RealName, profile.AccountName, profile.LegalName),
	}
}

func (profile baofuSettlementAccountProfileRequest) isZero() bool {
	return firstNonBlank(
		profile.LegalName,
		profile.AccountName,
		profile.RealName,
		profile.BusinessLicenseNumber,
		profile.CertificateNo,
		profile.IDCardNumber,
		profile.LegalPersonName,
		profile.LegalPersonIDNumber,
		profile.Email,
		profile.BankAccountNo,
		profile.BankAccountNumber,
		profile.AccountNumber,
		profile.BankMobile,
		profile.Mobile,
		profile.Phone,
		profile.BankName,
		profile.DepositBankProvince,
		profile.DepositBankCity,
		profile.DepositBankName,
		profile.ContactName,
		profile.ContactMobile,
		profile.CardUserName,
	) == ""
}
