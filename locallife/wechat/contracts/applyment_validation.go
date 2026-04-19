package contracts

import (
	"fmt"
	"strings"
	"time"
)

const applymentOutRequestNoMaxLength = 124

type ApplymentRequestValidationError struct {
	Message string
}

func (e *ApplymentRequestValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "validate ecommerce applyment request: failed"
	}
	return e.Message
}

type ApplymentQueryContractError struct {
	Message string
}

func (e *ApplymentQueryContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "validate ecommerce applyment query response: failed"
	}
	return e.Message
}

type EcommerceApplymentQueryValidationError struct {
	Message string
}

func (e *EcommerceApplymentQueryValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "validate ecommerce applyment query request: failed"
	}
	return e.Message
}

type SubMerchantSettlementQueryValidationError struct {
	Message string
}

func (e *SubMerchantSettlementQueryValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "validate sub merchant settlement query request: failed"
	}
	return e.Message
}

type SubMerchantSettlementContractError struct {
	Message string
}

func (e *SubMerchantSettlementContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "validate sub merchant settlement response: failed"
	}
	return e.Message
}

type SubMerchantSettlementApplicationContractError struct {
	Message string
}

func (e *SubMerchantSettlementApplicationContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "validate sub merchant settlement application response: failed"
	}
	return e.Message
}

type SubMerchantSettlementApplicationQueryValidationError struct {
	Message string
}

func (e *SubMerchantSettlementApplicationQueryValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "validate sub merchant settlement application query request: failed"
	}
	return e.Message
}

func ValidateEcommerceApplymentRequest(req *EcommerceApplymentRequest) error {
	if req == nil {
		return newApplymentRequestValidationError("request is nil")
	}
	if strings.TrimSpace(req.OutRequestNo) == "" {
		return newApplymentRequestValidationError("out_request_no is required")
	}
	if len(strings.TrimSpace(req.OutRequestNo)) > applymentOutRequestNoMaxLength {
		return newApplymentRequestValidationError("out_request_no must not exceed %d characters", applymentOutRequestNoMaxLength)
	}
	if req.OrganizationType != ApplymentOrganizationTypeEnterprise && req.OrganizationType != ApplymentOrganizationTypeIndividualBusiness {
		return newApplymentRequestValidationError("organization_type must be %q or %q", ApplymentOrganizationTypeEnterprise, ApplymentOrganizationTypeIndividualBusiness)
	}
	if req.BusinessLicenseInfo == nil {
		return newApplymentRequestValidationError("business_license_info is required for organization_type=%s", req.OrganizationType)
	}
	if strings.TrimSpace(req.BusinessLicenseInfo.BusinessLicenseCopy) == "" {
		return newApplymentRequestValidationError("business_license_info.business_license_copy is required")
	}
	if strings.TrimSpace(req.BusinessLicenseInfo.BusinessLicenseNumber) == "" {
		return newApplymentRequestValidationError("business_license_info.business_license_number is required")
	}
	if strings.TrimSpace(req.BusinessLicenseInfo.MerchantName) == "" {
		return newApplymentRequestValidationError("business_license_info.merchant_name is required")
	}
	if strings.TrimSpace(req.BusinessLicenseInfo.LegalPerson) == "" {
		return newApplymentRequestValidationError("business_license_info.legal_person is required")
	}
	if err := validateApplymentIdentity(req); err != nil {
		return err
	}
	if req.AccountInfo == nil {
		return newApplymentRequestValidationError("account_info is required")
	}
	if strings.TrimSpace(req.AccountInfo.AccountBank) == "" {
		return newApplymentRequestValidationError("account_info.account_bank is required")
	}
	if strings.TrimSpace(req.AccountInfo.AccountName) == "" {
		return newApplymentRequestValidationError("account_info.account_name is required")
	}
	if strings.TrimSpace(req.AccountInfo.AccountNumber) == "" {
		return newApplymentRequestValidationError("account_info.account_number is required")
	}
	switch req.OrganizationType {
	case ApplymentOrganizationTypeEnterprise:
		if req.AccountInfo.BankAccountType != ApplymentBankAccountTypeBusiness {
			return newApplymentRequestValidationError("account_info.bank_account_type must be %q for organization_type=%s", ApplymentBankAccountTypeBusiness, req.OrganizationType)
		}
	case ApplymentOrganizationTypeIndividualBusiness:
		if req.AccountInfo.BankAccountType != ApplymentBankAccountTypeBusiness && req.AccountInfo.BankAccountType != ApplymentBankAccountTypePrivate {
			return newApplymentRequestValidationError("account_info.bank_account_type must be %q or %q for organization_type=%s", ApplymentBankAccountTypeBusiness, ApplymentBankAccountTypePrivate, req.OrganizationType)
		}
	}
	if req.ContactInfo == nil {
		return newApplymentRequestValidationError("contact_info is required")
	}
	if req.ContactInfo.ContactType != ApplymentContactTypeLegal && req.ContactInfo.ContactType != ApplymentContactTypeSuper {
		return newApplymentRequestValidationError("contact_info.contact_type must be %q or %q", ApplymentContactTypeLegal, ApplymentContactTypeSuper)
	}
	if strings.TrimSpace(req.ContactInfo.ContactName) == "" {
		return newApplymentRequestValidationError("contact_info.contact_name is required")
	}
	if strings.TrimSpace(req.ContactInfo.MobilePhone) == "" {
		return newApplymentRequestValidationError("contact_info.mobile_phone is required")
	}
	if req.ContactInfo.ContactType == ApplymentContactTypeSuper {
		if strings.TrimSpace(req.ContactInfo.ContactIDDocType) == "" {
			return newApplymentRequestValidationError("contact_info.contact_id_doc_type is required when contact_type=%s", ApplymentContactTypeSuper)
		}
		if strings.TrimSpace(req.ContactInfo.ContactIDCardNumber) == "" {
			return newApplymentRequestValidationError("contact_info.contact_id_card_number is required when contact_type=%s", ApplymentContactTypeSuper)
		}
		if strings.TrimSpace(req.ContactInfo.ContactIDDocCopy) == "" {
			return newApplymentRequestValidationError("contact_info.contact_id_doc_copy is required when contact_type=%s", ApplymentContactTypeSuper)
		}
		if strings.TrimSpace(req.ContactInfo.ContactIDDocPeriodBegin) == "" {
			return newApplymentRequestValidationError("contact_info.contact_id_doc_period_begin is required when contact_type=%s", ApplymentContactTypeSuper)
		}
		if strings.TrimSpace(req.ContactInfo.ContactIDDocPeriodEnd) == "" {
			return newApplymentRequestValidationError("contact_info.contact_id_doc_period_end is required when contact_type=%s", ApplymentContactTypeSuper)
		}
		if req.ContactInfo.ContactIDDocType != ApplymentIdentificationTypeOverseaPassport && strings.TrimSpace(req.ContactInfo.ContactIDDocCopyBack) == "" {
			return newApplymentRequestValidationError("contact_info.contact_id_doc_copy_back is required when contact_type=%s and contact_id_doc_type is not passport", ApplymentContactTypeSuper)
		}
	}
	if req.SalesSceneInfo == nil {
		return newApplymentRequestValidationError("sales_scene_info is required")
	}
	if strings.TrimSpace(req.SalesSceneInfo.StoreName) == "" {
		return newApplymentRequestValidationError("sales_scene_info.store_name is required")
	}
	if strings.TrimSpace(req.SalesSceneInfo.StoreURL) == "" && strings.TrimSpace(req.SalesSceneInfo.StoreQRCode) == "" {
		return newApplymentRequestValidationError("sales_scene_info.store_url or sales_scene_info.store_qr_code is required")
	}
	if strings.TrimSpace(req.MerchantShortname) == "" {
		return newApplymentRequestValidationError("merchant_shortname is required")
	}
	return nil
}

func ValidateEcommerceApplymentQueryByOutRequestNoResponse(resp *EcommerceApplymentQueryResponse) error {
	return validateEcommerceApplymentQueryResponse(resp, false)
}

func ValidateEcommerceApplymentQueryByIDResponse(resp *EcommerceApplymentQueryResponse) error {
	return validateEcommerceApplymentQueryResponse(resp, true)
}

func ValidateSubMerchantSettlementResponse(resp *SubMerchantSettlementResponse) error {
	if resp == nil {
		return newSubMerchantSettlementContractError("empty response")
	}
	if resp.AccountType != SubMerchantSettlementAccountTypeBusiness && resp.AccountType != SubMerchantSettlementAccountTypePrivate {
		return newSubMerchantSettlementContractError("unsupported account_type %q", resp.AccountType)
	}
	if strings.TrimSpace(resp.AccountBank) == "" {
		return newSubMerchantSettlementContractError("account_bank is required")
	}
	if strings.TrimSpace(resp.AccountNumber) == "" {
		return newSubMerchantSettlementContractError("account_number is required")
	}
	switch resp.VerifyResult {
	case SubMerchantSettlementVerifyResultSuccess, SubMerchantSettlementVerifyResultFail, SubMerchantSettlementVerifyResultVerifying:
	default:
		return newSubMerchantSettlementContractError("unsupported verify_result %q", resp.VerifyResult)
	}
	if resp.VerifyResult == SubMerchantSettlementVerifyResultFail {
		if strings.TrimSpace(resp.VerifyFailReason) == "" {
			return newSubMerchantSettlementContractError("verify_fail_reason is required when verify_result=%s", SubMerchantSettlementVerifyResultFail)
		}
	} else if strings.TrimSpace(resp.VerifyFailReason) != "" {
		return newSubMerchantSettlementContractError("verify_fail_reason is only allowed when verify_result=%s", SubMerchantSettlementVerifyResultFail)
	}
	return nil
}

func ValidateSubMerchantSettlementApplicationResponse(resp *QuerySubMerchantSettlementApplicationResponse) error {
	if resp == nil {
		return newSubMerchantSettlementApplicationContractError("empty response")
	}
	if strings.TrimSpace(resp.AccountName) == "" {
		return newSubMerchantSettlementApplicationContractError("account_name is required")
	}
	if resp.AccountType != SubMerchantSettlementAccountTypeBusiness && resp.AccountType != SubMerchantSettlementAccountTypePrivate {
		return newSubMerchantSettlementApplicationContractError("unsupported account_type %q", resp.AccountType)
	}
	if strings.TrimSpace(resp.AccountBank) == "" {
		return newSubMerchantSettlementApplicationContractError("account_bank is required")
	}
	if strings.TrimSpace(resp.AccountNumber) == "" {
		return newSubMerchantSettlementApplicationContractError("account_number is required")
	}
	switch resp.VerifyResult {
	case SubMerchantSettlementApplicationAuditSuccess, SubMerchantSettlementApplicationAuditing, SubMerchantSettlementApplicationAuditFail:
	default:
		return newSubMerchantSettlementApplicationContractError("unsupported verify_result %q", resp.VerifyResult)
	}
	if resp.VerifyResult == SubMerchantSettlementApplicationAuditFail {
		if strings.TrimSpace(resp.VerifyFailReason) == "" {
			return newSubMerchantSettlementApplicationContractError("verify_fail_reason is required when verify_result=%s", SubMerchantSettlementApplicationAuditFail)
		}
	} else if strings.TrimSpace(resp.VerifyFailReason) != "" {
		return newSubMerchantSettlementApplicationContractError("verify_fail_reason is only allowed when verify_result=%s", SubMerchantSettlementApplicationAuditFail)
	}
	if strings.TrimSpace(resp.VerifyFinishTime) != "" {
		if _, err := time.Parse(time.RFC3339, resp.VerifyFinishTime); err != nil {
			return newSubMerchantSettlementApplicationContractError("verify_finish_time must be RFC3339 when present")
		}
	}
	return nil
}

func validateApplymentIdentity(req *EcommerceApplymentRequest) error {
	idDocType := strings.TrimSpace(req.IDDocType)
	if idDocType == "" || idDocType == ApplymentIdentificationTypeMainlandIDCard {
		if req.IDCardInfo == nil {
			return newApplymentRequestValidationError("id_card_info is required when id_doc_type is empty or %s", ApplymentIdentificationTypeMainlandIDCard)
		}
		if strings.TrimSpace(req.IDCardInfo.IDCardCopy) == "" {
			return newApplymentRequestValidationError("id_card_info.id_card_copy is required")
		}
		if strings.TrimSpace(req.IDCardInfo.IDCardNational) == "" {
			return newApplymentRequestValidationError("id_card_info.id_card_national is required")
		}
		if strings.TrimSpace(req.IDCardInfo.IDCardName) == "" {
			return newApplymentRequestValidationError("id_card_info.id_card_name is required")
		}
		if strings.TrimSpace(req.IDCardInfo.IDCardNumber) == "" {
			return newApplymentRequestValidationError("id_card_info.id_card_number is required")
		}
		if strings.TrimSpace(req.IDCardInfo.IDCardValidTimeBegin) == "" {
			return newApplymentRequestValidationError("id_card_info.id_card_valid_time_begin is required")
		}
		if strings.TrimSpace(req.IDCardInfo.IDCardValidTime) == "" {
			return newApplymentRequestValidationError("id_card_info.id_card_valid_time is required")
		}
		return nil
	}
	if req.IDDocInfo == nil {
		return newApplymentRequestValidationError("id_doc_info is required when id_doc_type=%s", idDocType)
	}
	if strings.TrimSpace(req.IDDocInfo.IDDocName) == "" {
		return newApplymentRequestValidationError("id_doc_info.id_doc_name is required")
	}
	if strings.TrimSpace(req.IDDocInfo.IDDocNumber) == "" {
		return newApplymentRequestValidationError("id_doc_info.id_doc_number is required")
	}
	if strings.TrimSpace(req.IDDocInfo.IDDocCopy) == "" {
		return newApplymentRequestValidationError("id_doc_info.id_doc_copy is required")
	}
	if strings.TrimSpace(req.IDDocInfo.DocPeriodBegin) == "" {
		return newApplymentRequestValidationError("id_doc_info.doc_period_begin is required")
	}
	if strings.TrimSpace(req.IDDocInfo.DocPeriodEnd) == "" {
		return newApplymentRequestValidationError("id_doc_info.doc_period_end is required")
	}
	if idDocType != ApplymentIdentificationTypeOverseaPassport && strings.TrimSpace(req.IDDocInfo.IDDocCopyBack) == "" {
		return newApplymentRequestValidationError("id_doc_info.id_doc_copy_back is required when id_doc_type is not passport")
	}
	return nil
}

func validateEcommerceApplymentQueryResponse(resp *EcommerceApplymentQueryResponse, allowUnsignedSignURL bool) error {
	if resp == nil {
		return newApplymentQueryContractError("empty response")
	}
	if strings.TrimSpace(resp.ApplymentState) == "" {
		return newApplymentQueryContractError("applyment_state is required")
	}
	if !isAllowedApplymentState(resp.ApplymentState) {
		return newApplymentQueryContractError("unsupported applyment_state %q", resp.ApplymentState)
	}
	if strings.TrimSpace(resp.ApplymentStateDesc) == "" {
		return newApplymentQueryContractError("applyment_state_desc is required")
	}
	if strings.TrimSpace(resp.OutRequestNo) == "" {
		return newApplymentQueryContractError("out_request_no is required")
	}
	if resp.ApplymentID <= 0 {
		return newApplymentQueryContractError("applyment_id must be greater than 0")
	}
	if strings.TrimSpace(resp.SignState) != "" && !isAllowedApplymentSignState(resp.SignState) {
		return newApplymentQueryContractError("unsupported sign_state %q", resp.SignState)
	}
	requireSignURL := resp.ApplymentState == ApplymentStateNeedSign || (allowUnsignedSignURL && resp.SignState == ApplymentSignStateUnsigned)
	if requireSignURL {
		if strings.TrimSpace(resp.SignURL) == "" {
			return newApplymentQueryContractError("sign_url is required for current applyment state")
		}
	} else if strings.TrimSpace(resp.SignURL) != "" {
		return newApplymentQueryContractError("sign_url is not allowed for current applyment state")
	}
	if resp.ApplymentState == ApplymentStateNeedSign || resp.ApplymentState == ApplymentStateFinish {
		if strings.TrimSpace(resp.SubMchID) == "" {
			return newApplymentQueryContractError("sub_mchid is required when applyment_state=%s", resp.ApplymentState)
		}
	} else if strings.TrimSpace(resp.SubMchID) != "" {
		return newApplymentQueryContractError("sub_mchid is only allowed when applyment_state=%s or %s", ApplymentStateNeedSign, ApplymentStateFinish)
	}
	if resp.ApplymentState == ApplymentStateAccountNeedVerify {
		if resp.AccountValidation == nil {
			return newApplymentQueryContractError("account_validation is required when applyment_state=%s", ApplymentStateAccountNeedVerify)
		}
		if strings.TrimSpace(resp.AccountValidation.AccountName) == "" {
			return newApplymentQueryContractError("account_validation.account_name is required")
		}
		if resp.AccountValidation.PayAmount <= 0 {
			return newApplymentQueryContractError("account_validation.pay_amount must be greater than 0")
		}
		if strings.TrimSpace(resp.AccountValidation.DestinationAccountNumber) == "" {
			return newApplymentQueryContractError("account_validation.destination_account_number is required")
		}
		if strings.TrimSpace(resp.AccountValidation.DestinationAccountName) == "" {
			return newApplymentQueryContractError("account_validation.destination_account_name is required")
		}
		if strings.TrimSpace(resp.AccountValidation.DestinationAccountBank) == "" {
			return newApplymentQueryContractError("account_validation.destination_account_bank is required")
		}
		if strings.TrimSpace(resp.AccountValidation.City) == "" {
			return newApplymentQueryContractError("account_validation.city is required")
		}
		if strings.TrimSpace(resp.AccountValidation.Remark) == "" {
			return newApplymentQueryContractError("account_validation.remark is required")
		}
		if strings.TrimSpace(resp.AccountValidation.Deadline) == "" {
			return newApplymentQueryContractError("account_validation.deadline is required")
		}
	} else {
		if resp.AccountValidation != nil {
			return newApplymentQueryContractError("account_validation is only allowed when applyment_state=%s", ApplymentStateAccountNeedVerify)
		}
		if strings.TrimSpace(resp.LegalValidationURL) != "" {
			return newApplymentQueryContractError("legal_validation_url is only allowed when applyment_state=%s", ApplymentStateAccountNeedVerify)
		}
	}
	if resp.ApplymentState == ApplymentStateRejected || resp.ApplymentState == ApplymentStateFrozen {
		for index, detail := range resp.AuditDetail {
			if strings.TrimSpace(detail.ParamName) == "" {
				return newApplymentQueryContractError("audit_detail[%d].param_name is required", index)
			}
			if strings.TrimSpace(detail.RejectReason) == "" {
				return newApplymentQueryContractError("audit_detail[%d].reject_reason is required", index)
			}
		}
	} else if len(resp.AuditDetail) > 0 {
		return newApplymentQueryContractError("audit_detail is only allowed when applyment_state=%s or %s", ApplymentStateRejected, ApplymentStateFrozen)
	}
	return nil
}

func isAllowedApplymentState(state string) bool {
	switch state {
	case ApplymentStateChecking,
		ApplymentStateAccountNeedVerify,
		ApplymentStateAuditing,
		ApplymentStateRejected,
		ApplymentStateNeedSign,
		ApplymentStateFinish,
		ApplymentStateFrozen,
		ApplymentStateCanceled:
		return true
	default:
		return false
	}
}

func isAllowedApplymentSignState(state string) bool {
	switch state {
	case ApplymentSignStateUnsigned, ApplymentSignStateSigned, ApplymentSignStateNotSignable:
		return true
	default:
		return false
	}
}

func NewApplymentRequestValidationError(format string, args ...any) error {
	return newApplymentRequestValidationError(format, args...)
}

func NewEcommerceApplymentQueryValidationError(format string, args ...any) error {
	return &EcommerceApplymentQueryValidationError{Message: fmt.Sprintf("validate ecommerce applyment query request: "+format, args...)}
}

func NewApplymentQueryContractError(format string, args ...any) error {
	return newApplymentQueryContractError(format, args...)
}

func NewSubMerchantSettlementQueryValidationError(format string, args ...any) error {
	return &SubMerchantSettlementQueryValidationError{Message: fmt.Sprintf("validate sub merchant settlement query request: "+format, args...)}
}

func NewSubMerchantSettlementContractError(format string, args ...any) error {
	return newSubMerchantSettlementContractError(format, args...)
}

func NewSubMerchantSettlementApplicationQueryValidationError(format string, args ...any) error {
	return &SubMerchantSettlementApplicationQueryValidationError{Message: fmt.Sprintf("validate sub merchant settlement application query request: "+format, args...)}
}

func NewSubMerchantSettlementApplicationContractError(format string, args ...any) error {
	return newSubMerchantSettlementApplicationContractError(format, args...)
}

func newApplymentRequestValidationError(format string, args ...any) error {
	return &ApplymentRequestValidationError{Message: fmt.Sprintf("validate ecommerce applyment request: "+format, args...)}
}

func newApplymentQueryContractError(format string, args ...any) error {
	return &ApplymentQueryContractError{Message: fmt.Sprintf("validate ecommerce applyment query response: "+format, args...)}
}

func newSubMerchantSettlementContractError(format string, args ...any) error {
	return &SubMerchantSettlementContractError{Message: fmt.Sprintf("validate sub merchant settlement response: "+format, args...)}
}

func newSubMerchantSettlementApplicationContractError(format string, args ...any) error {
	return &SubMerchantSettlementApplicationContractError{Message: fmt.Sprintf("validate sub merchant settlement application response: "+format, args...)}
}
