package contracts

import (
	"fmt"
	"net/url"
	"strings"
)

type Currency string

const CurrencyCNY Currency = "CNY"

type ValidationError struct {
	Field   string
	Code    string
	Message string
}

func (e ValidationError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type ContactType string

const (
	ContactTypeLegal ContactType = "LEGAL"
	ContactTypeSuper ContactType = "SUPER"
)

type IdentificationType string

const (
	IdentificationTypeIDCard           IdentificationType = "IDENTIFICATION_TYPE_IDCARD"
	IdentificationTypeOverseaPassport  IdentificationType = "IDENTIFICATION_TYPE_OVERSEA_PASSPORT"
	IdentificationTypeHongKongPassport IdentificationType = "IDENTIFICATION_TYPE_HONGKONG_PASSPORT"
	IdentificationTypeMacaoPassport    IdentificationType = "IDENTIFICATION_TYPE_MACAO_PASSPORT"
	IdentificationTypeTaiwanPassport   IdentificationType = "IDENTIFICATION_TYPE_TAIWAN_PASSPORT"
	IdentificationTypeForeignResident  IdentificationType = "IDENTIFICATION_TYPE_FOREIGN_RESIDENT"
	IdentificationTypeHongKongMacaoID  IdentificationType = "IDENTIFICATION_TYPE_HONGKONG_MACAO_RESIDENT"
	IdentificationTypeTaiwanResident   IdentificationType = "IDENTIFICATION_TYPE_TAIWAN_RESIDENT"
)

type SubjectType string

const (
	SubjectTypeIndividual  SubjectType = "SUBJECT_TYPE_INDIVIDUAL"
	SubjectTypeEnterprise  SubjectType = "SUBJECT_TYPE_ENTERPRISE"
	SubjectTypeGovernment  SubjectType = "SUBJECT_TYPE_GOVERNMENT"
	SubjectTypeInstitution SubjectType = "SUBJECT_TYPE_INSTITUTIONS"
	SubjectTypeOthers      SubjectType = "SUBJECT_TYPE_OTHERS"
)

type BankAccountType string

const (
	BankAccountTypeCorporate BankAccountType = "BANK_ACCOUNT_TYPE_CORPORATE"
	BankAccountTypePersonal  BankAccountType = "BANK_ACCOUNT_TYPE_PERSONAL"
	BankAccountTypeBusiness  BankAccountType = "ACCOUNT_TYPE_BUSINESS"
	BankAccountTypePrivate   BankAccountType = "ACCOUNT_TYPE_PRIVATE"
)

type ApplymentSalesSceneType string

const (
	SalesSceneStore       ApplymentSalesSceneType = "SALES_SCENES_STORE"
	SalesSceneMP          ApplymentSalesSceneType = "SALES_SCENES_MP"
	SalesSceneMiniProgram ApplymentSalesSceneType = "SALES_SCENES_MINI_PROGRAM"
	SalesSceneWeb         ApplymentSalesSceneType = "SALES_SCENES_WEB"
	SalesSceneApp         ApplymentSalesSceneType = "SALES_SCENES_APP"
	SalesSceneWework      ApplymentSalesSceneType = "SALES_SCENES_WEWORK"
)

type ApplymentSubmitRequest struct {
	BusinessCode    string                   `json:"business_code,omitempty"`
	ContactInfo     ApplymentContactInfo     `json:"contact_info,omitempty"`
	SubjectInfo     ApplymentSubjectInfo     `json:"subject_info,omitempty"`
	BusinessInfo    ApplymentBusinessInfo    `json:"business_info,omitempty"`
	SettlementInfo  ApplymentSettlementInfo  `json:"settlement_info,omitempty"`
	BankAccountInfo ApplymentBankAccountInfo `json:"bank_account_info,omitempty"`
	AdditionInfo    *ApplymentAdditionInfo   `json:"addition_info,omitempty"`
}

type ApplymentSubmitResponse struct {
	ApplymentID int64 `json:"applyment_id,omitempty"`
}

type ApplymentContactInfo struct {
	ContactType                 ContactType        `json:"contact_type,omitempty"`
	ContactName                 string             `json:"contact_name,omitempty"`
	ContactIDDocType            IdentificationType `json:"contact_id_doc_type,omitempty"`
	ContactIDNumber             string             `json:"contact_id_number,omitempty"`
	ContactIDDocCopy            string             `json:"contact_id_doc_copy,omitempty"`
	ContactIDDocCopyBack        string             `json:"contact_id_doc_copy_back,omitempty"`
	ContactPeriodBegin          string             `json:"contact_period_begin,omitempty"`
	ContactPeriodEnd            string             `json:"contact_period_end,omitempty"`
	BusinessAuthorizationLetter string             `json:"business_authorization_letter,omitempty"`
	OpenID                      string             `json:"openid,omitempty"`
	MobilePhone                 string             `json:"mobile_phone,omitempty"`
	ContactEmail                string             `json:"contact_email,omitempty"`
}

type ApplymentSubjectInfo struct {
	SubjectType            SubjectType                      `json:"subject_type,omitempty"`
	FinanceInstitution     bool                             `json:"finance_institution,omitempty"`
	BusinessLicenseInfo    *ApplymentBusinessLicenseInfo    `json:"business_license_info,omitempty"`
	CertificateInfo        *ApplymentCertificateInfo        `json:"certificate_info,omitempty"`
	CertificateLetterCopy  string                           `json:"certificate_letter_copy,omitempty"`
	FinanceInstitutionInfo *ApplymentFinanceInstitutionInfo `json:"finance_institution_info,omitempty"`
	IdentityInfo           ApplymentIdentityInfo            `json:"identity_info,omitempty"`
	UBOInfoList            []ApplymentUBOInfo               `json:"ubo_info_list,omitempty"`
}

type ApplymentBusinessLicenseInfo struct {
	LicenseCopy    string `json:"license_copy,omitempty"`
	LicenseNumber  string `json:"license_number,omitempty"`
	MerchantName   string `json:"merchant_name,omitempty"`
	LegalPerson    string `json:"legal_person,omitempty"`
	LicenseAddress string `json:"license_address,omitempty"`
	PeriodBegin    string `json:"period_begin,omitempty"`
	PeriodEnd      string `json:"period_end,omitempty"`
}

type ApplymentCertificateInfo struct {
	CertCopy       string `json:"cert_copy,omitempty"`
	CertType       string `json:"cert_type,omitempty"`
	CertNumber     string `json:"cert_number,omitempty"`
	MerchantName   string `json:"merchant_name,omitempty"`
	CompanyAddress string `json:"company_address,omitempty"`
	LegalPerson    string `json:"legal_person,omitempty"`
	PeriodBegin    string `json:"period_begin,omitempty"`
	PeriodEnd      string `json:"period_end,omitempty"`
}

type ApplymentFinanceInstitutionInfo struct {
	FinanceType        string   `json:"finance_type,omitempty"`
	FinanceLicensePics []string `json:"finance_license_pics,omitempty"`
}

type ApplymentIdentityInfo struct {
	IDHolderType        ContactType          `json:"id_holder_type,omitempty"`
	IDDocType           IdentificationType   `json:"id_doc_type,omitempty"`
	AuthorizeLetterCopy string               `json:"authorize_letter_copy,omitempty"`
	IDCardInfo          *ApplymentIDCardInfo `json:"id_card_info,omitempty"`
	IDDocInfo           *ApplymentIDDocInfo  `json:"id_doc_info,omitempty"`
	Owner               bool                 `json:"owner,omitempty"`
}

type ApplymentIDCardInfo struct {
	IDCardCopy      string `json:"id_card_copy,omitempty"`
	IDCardNational  string `json:"id_card_national,omitempty"`
	IDCardName      string `json:"id_card_name,omitempty"`
	IDCardNumber    string `json:"id_card_number,omitempty"`
	IDCardAddress   string `json:"id_card_address,omitempty"`
	CardPeriodBegin string `json:"card_period_begin,omitempty"`
	CardPeriodEnd   string `json:"card_period_end,omitempty"`
}

type ApplymentIDDocInfo struct {
	IDDocCopy      string `json:"id_doc_copy,omitempty"`
	IDDocCopyBack  string `json:"id_doc_copy_back,omitempty"`
	IDDocName      string `json:"id_doc_name,omitempty"`
	IDDocNumber    string `json:"id_doc_number,omitempty"`
	IDDocAddress   string `json:"id_doc_address,omitempty"`
	DocPeriodBegin string `json:"doc_period_begin,omitempty"`
	DocPeriodEnd   string `json:"doc_period_end,omitempty"`
}

type ApplymentUBOInfo struct {
	UBOIDDocType     IdentificationType `json:"ubo_id_doc_type,omitempty"`
	UBOIDDocCopy     string             `json:"ubo_id_doc_copy,omitempty"`
	UBOIDDocCopyBack string             `json:"ubo_id_doc_copy_back,omitempty"`
	UBOIDDocName     string             `json:"ubo_id_doc_name,omitempty"`
	UBOIDDocNumber   string             `json:"ubo_id_doc_number,omitempty"`
	UBOIDDocAddress  string             `json:"ubo_id_doc_address,omitempty"`
	UBOPeriodBegin   string             `json:"ubo_period_begin,omitempty"`
	UBOPeriodEnd     string             `json:"ubo_period_end,omitempty"`
}

type ApplymentBusinessInfo struct {
	MerchantShortname string             `json:"merchant_shortname,omitempty"`
	ServicePhone      string             `json:"service_phone,omitempty"`
	SalesInfo         ApplymentSalesInfo `json:"sales_info,omitempty"`
}

type ApplymentSalesInfo struct {
	SalesScenesType []ApplymentSalesSceneType `json:"sales_scenes_type,omitempty"`
	MiniProgramInfo *ApplymentMiniProgramInfo `json:"mini_program_info,omitempty"`
	StoreInfo       *ApplymentStoreInfo       `json:"biz_store_info,omitempty"`
	MPInfo          *ApplymentMPInfo          `json:"mp_info,omitempty"`
	WebInfo         *ApplymentWebInfo         `json:"web_info,omitempty"`
	AppInfo         *ApplymentAppInfo         `json:"app_info,omitempty"`
	WeworkInfo      *ApplymentWeworkInfo      `json:"wework_info,omitempty"`
}

type ApplymentMiniProgramInfo struct {
	MiniProgramAppID    string   `json:"mini_program_appid,omitempty"`
	MiniProgramSubAppID string   `json:"mini_program_sub_appid,omitempty"`
	MiniProgramPics     []string `json:"mini_program_pics,omitempty"`
}

type ApplymentStoreInfo struct {
	StoreName        string   `json:"biz_store_name,omitempty"`
	AddressCode      string   `json:"biz_address_code,omitempty"`
	StoreAddress     string   `json:"biz_store_address,omitempty"`
	StoreEntrancePic []string `json:"store_entrance_pic,omitempty"`
	IndoorPic        []string `json:"indoor_pic,omitempty"`
	BizSubAppID      string   `json:"biz_sub_appid,omitempty"`
}

type ApplymentMPInfo struct {
	MPAppID    string   `json:"mp_appid,omitempty"`
	MPSubAppID string   `json:"mp_sub_appid,omitempty"`
	MPPics     []string `json:"mp_pics,omitempty"`
}

type ApplymentWebInfo struct {
	Domain           string `json:"domain,omitempty"`
	WebAuthorisation string `json:"web_authorisation,omitempty"`
	WebAppID         string `json:"web_appid,omitempty"`
}

type ApplymentAppInfo struct {
	AppAppID    string   `json:"app_appid,omitempty"`
	AppSubAppID string   `json:"app_sub_appid,omitempty"`
	AppPics     []string `json:"app_pics,omitempty"`
}

type ApplymentWeworkInfo struct {
	SubCorpID  string   `json:"sub_corp_id,omitempty"`
	WeworkPics []string `json:"wework_pics,omitempty"`
}

type ApplymentSettlementInfo struct {
	SettlementID         string   `json:"settlement_id,omitempty"`
	QualificationType    string   `json:"qualification_type,omitempty"`
	Qualifications       []string `json:"qualifications,omitempty"`
	ActivitiesID         string   `json:"activities_id,omitempty"`
	ActivitiesRate       string   `json:"activities_rate,omitempty"`
	ActivitiesAdditions  []string `json:"activities_additions,omitempty"`
	DebitActivitiesRate  string   `json:"debit_activities_rate,omitempty"`
	CreditActivitiesRate string   `json:"credit_activities_rate,omitempty"`
}

type ApplymentBankAccountInfo struct {
	BankAccountType BankAccountType `json:"bank_account_type,omitempty"`
	AccountName     string          `json:"account_name,omitempty"`
	AccountBank     string          `json:"account_bank,omitempty"`
	BankAddressCode string          `json:"bank_address_code,omitempty"`
	BankBranchID    string          `json:"bank_branch_id,omitempty"`
	BankName        string          `json:"bank_name,omitempty"`
	AccountNumber   string          `json:"account_number,omitempty"`
}

type ApplymentAdditionInfo struct {
	LegalPersonCommitment string   `json:"legal_person_commitment,omitempty"`
	LegalPersonVideo      string   `json:"legal_person_video,omitempty"`
	BusinessAdditionPics  []string `json:"business_addition_pics,omitempty"`
	BusinessAdditionMsg   string   `json:"business_addition_msg,omitempty"`
}

type ApplymentQueryByIDRequest struct {
	ApplymentID int64 `json:"applyment_id,omitempty"`
}

type ApplymentQueryByBusinessCodeRequest struct {
	BusinessCode string `json:"business_code,omitempty"`
}

type ApplymentState string

const (
	ApplymentStateEditing       ApplymentState = "APPLYMENT_STATE_EDITTING"
	ApplymentStateAuditing      ApplymentState = "APPLYMENT_STATE_AUDITING"
	ApplymentStateRejected      ApplymentState = "APPLYMENT_STATE_REJECTED"
	ApplymentStateToBeConfirmed ApplymentState = "APPLYMENT_STATE_TO_BE_CONFIRMED"
	ApplymentStateToBeSigned    ApplymentState = "APPLYMENT_STATE_TO_BE_SIGNED"
	ApplymentStateSigning       ApplymentState = "APPLYMENT_STATE_SIGNING"
	ApplymentStateFinished      ApplymentState = "APPLYMENT_STATE_FINISHED"
	ApplymentStateCanceled      ApplymentState = "APPLYMENT_STATE_CANCELED"
)

type ApplymentQueryResponse struct {
	ApplymentID       int64                  `json:"applyment_id,omitempty"`
	BusinessCode      string                 `json:"business_code,omitempty"`
	SubMchID          string                 `json:"sub_mchid,omitempty"`
	SignURL           string                 `json:"sign_url,omitempty"`
	ApplymentState    ApplymentState         `json:"applyment_state,omitempty"`
	ApplymentStateMsg string                 `json:"applyment_state_msg,omitempty"`
	AuditDetail       []ApplymentAuditDetail `json:"audit_detail,omitempty"`
}

type ApplymentAuditDetail struct {
	Field        string `json:"field,omitempty"`
	FieldName    string `json:"field_name,omitempty"`
	RejectReason string `json:"reject_reason,omitempty"`
}

func (r ApplymentSubmitRequest) Validate() error {
	if err := requireString("business_code", r.BusinessCode); err != nil {
		return err
	}
	if err := r.ContactInfo.validate(); err != nil {
		return err
	}
	if err := r.SubjectInfo.validate(); err != nil {
		return err
	}
	if err := r.BusinessInfo.validate(); err != nil {
		return err
	}
	if err := r.SettlementInfo.validate(); err != nil {
		return err
	}
	return r.BankAccountInfo.validate()
}

func (i ApplymentContactInfo) validate() error {
	if i.ContactType != ContactTypeLegal && i.ContactType != ContactTypeSuper {
		return invalidEnum("contact_info.contact_type")
	}
	for _, field := range []struct{ name, value string }{
		{"contact_info.contact_name", i.ContactName},
		{"contact_info.mobile_phone", i.MobilePhone},
		{"contact_info.contact_email", i.ContactEmail},
	} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	return nil
}

func (i ApplymentSubjectInfo) validate() error {
	if i.SubjectType != SubjectTypeIndividual && i.SubjectType != SubjectTypeEnterprise {
		return ValidationError{Field: "subject_info.subject_type", Code: "unsupported_subject_type", Message: "ordinary service provider applyment supports only individual business and enterprise subjects"}
	}
	if i.BusinessLicenseInfo == nil {
		return missing("subject_info.business_license_info")
	}
	if err := i.BusinessLicenseInfo.validate(); err != nil {
		return err
	}
	return i.IdentityInfo.validate()
}

func (i ApplymentBusinessLicenseInfo) validate() error {
	for _, field := range []struct{ name, value string }{
		{"subject_info.business_license_info.license_copy", i.LicenseCopy},
		{"subject_info.business_license_info.license_number", i.LicenseNumber},
		{"subject_info.business_license_info.merchant_name", i.MerchantName},
		{"subject_info.business_license_info.legal_person", i.LegalPerson},
	} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	return nil
}

func (i ApplymentIdentityInfo) validate() error {
	if i.IDDocType == "" {
		return missing("subject_info.identity_info.id_doc_type")
	}
	if i.IDDocType == IdentificationTypeIDCard && i.IDCardInfo == nil {
		return missing("subject_info.identity_info.id_card_info")
	}
	if i.IDDocType != IdentificationTypeIDCard && i.IDDocInfo == nil {
		return missing("subject_info.identity_info.id_doc_info")
	}
	return nil
}

func (i ApplymentBusinessInfo) validate() error {
	for _, field := range []struct{ name, value string }{
		{"business_info.merchant_shortname", i.MerchantShortname},
		{"business_info.service_phone", i.ServicePhone},
	} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	return i.SalesInfo.validate()
}

func (i ApplymentSalesInfo) validate() error {
	if len(i.SalesScenesType) == 0 {
		return missing("business_info.sales_info.sales_scenes_type")
	}
	if salesSceneSelected(i.SalesScenesType, SalesSceneMiniProgram) {
		if i.MiniProgramInfo == nil {
			return missing("business_info.sales_info.mini_program_info")
		}
		if strings.TrimSpace(i.MiniProgramInfo.MiniProgramAppID) == "" && strings.TrimSpace(i.MiniProgramInfo.MiniProgramSubAppID) == "" {
			return missing("business_info.sales_info.mini_program_info.mini_program_appid")
		}
	}
	return nil
}

func (i ApplymentSettlementInfo) validate() error {
	if err := requireString("settlement_info.settlement_id", i.SettlementID); err != nil {
		return err
	}
	return requireString("settlement_info.qualification_type", i.QualificationType)
}

func (i ApplymentBankAccountInfo) validate() error {
	if i.BankAccountType != BankAccountTypeCorporate && i.BankAccountType != BankAccountTypePersonal {
		return invalidEnum("bank_account_info.bank_account_type")
	}
	for _, field := range []struct{ name, value string }{
		{"bank_account_info.account_name", i.AccountName},
		{"bank_account_info.account_bank", i.AccountBank},
		{"bank_account_info.account_number", i.AccountNumber},
	} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	return nil
}

type SettlementQueryRequest struct {
	SubMchID          string            `json:"sub_mchid,omitempty"`
	AccountNumberRule AccountNumberRule `json:"account_number_rule,omitempty"`
}

type AccountNumberRule string

const (
	AccountNumberRuleMaskV1 AccountNumberRule = "ACCOUNT_NUMBER_RULE_MASK_V1"
	AccountNumberRuleMaskV2 AccountNumberRule = "ACCOUNT_NUMBER_RULE_MASK_V2"
)

type SettlementVerifyResult string

const (
	SettlementVerifyResultSuccess SettlementVerifyResult = "VERIFY_SUCCESS"
	SettlementVerifyResultFail    SettlementVerifyResult = "VERIFY_FAIL"
	SettlementVerifyResultIng     SettlementVerifyResult = "VERIFYING"
)

type SettlementAuditResult string

const (
	SettlementAuditResultSuccess  SettlementAuditResult = "AUDIT_SUCCESS"
	SettlementAuditResultAuditing SettlementAuditResult = "AUDITING"
	SettlementAuditResultFail     SettlementAuditResult = "AUDIT_FAIL"
)

type SettlementQueryResponse struct {
	SubMchID         string                 `json:"sub_mchid,omitempty"`
	AccountType      BankAccountType        `json:"account_type,omitempty"`
	AccountName      string                 `json:"account_name,omitempty"`
	AccountBank      string                 `json:"account_bank,omitempty"`
	BankName         string                 `json:"bank_name,omitempty"`
	BankBranchID     string                 `json:"bank_branch_id,omitempty"`
	AccountNumber    string                 `json:"account_number,omitempty"`
	VerifyResult     SettlementVerifyResult `json:"verify_result,omitempty"`
	VerifyFailReason string                 `json:"verify_fail_reason,omitempty"`
}

type SettlementModifyRequest struct {
	SubMchID      string          `json:"sub_mchid,omitempty"`
	AccountType   BankAccountType `json:"account_type,omitempty"`
	AccountName   string          `json:"account_name,omitempty"`
	AccountBank   string          `json:"account_bank,omitempty"`
	BankName      string          `json:"bank_name,omitempty"`
	BankBranchID  string          `json:"bank_branch_id,omitempty"`
	AccountNumber string          `json:"account_number,omitempty"`
}

type SettlementModifyResponse struct {
	ApplicationNo string `json:"application_no,omitempty"`
	ApplicationID string `json:"application_id,omitempty"`
}

type SettlementModificationQueryRequest struct {
	SubMchID          string            `json:"sub_mchid,omitempty"`
	ApplicationNo     string            `json:"application_no,omitempty"`
	ApplicationID     string            `json:"application_id,omitempty"`
	AccountNumberRule AccountNumberRule `json:"account_number_rule,omitempty"`
}

type SettlementModificationState string

const (
	SettlementModificationStateAuditing SettlementModificationState = "AUDITING"
	SettlementModificationStateRejected SettlementModificationState = "REJECTED"
	SettlementModificationStateFinished SettlementModificationState = "FINISHED"
)

type SettlementModificationQueryResponse struct {
	ApplicationNo    string                `json:"application_no,omitempty"`
	ApplicationID    string                `json:"application_id,omitempty"`
	AccountName      string                `json:"account_name,omitempty"`
	AccountType      BankAccountType       `json:"account_type,omitempty"`
	AccountBank      string                `json:"account_bank,omitempty"`
	BankName         string                `json:"bank_name,omitempty"`
	BankBranchID     string                `json:"bank_branch_id,omitempty"`
	AccountNumber    string                `json:"account_number,omitempty"`
	VerifyResult     SettlementAuditResult `json:"verify_result,omitempty"`
	VerifyFailReason string                `json:"verify_fail_reason,omitempty"`
	VerifyFinishTime string                `json:"verify_finish_time,omitempty"`
	State            SettlementModificationState
	RejectReason     string `json:"reject_reason,omitempty"`
}

type AccountWillingnessSubmitRequest struct {
	BusinessCode string `json:"business_code,omitempty"`
	SubMchID     string `json:"sub_mchid,omitempty"`
	ContactInfo  string `json:"contact_info,omitempty"`
}

type AccountWillingnessSubmitResponse struct {
	ApplymentID int64 `json:"applyment_id,omitempty"`
}

type AccountWillingnessCancelRequest struct {
	BusinessCode string `json:"business_code,omitempty"`
}

type AccountWillingnessCancelResponse struct {
	BusinessCode string `json:"business_code,omitempty"`
}

type AccountWillingnessQueryRequest struct {
	BusinessCode string `json:"business_code,omitempty"`
}

type AccountWillingnessState string

const (
	AccountWillingnessStateEditing                  AccountWillingnessState = "APPLYMENT_STATE_EDITTING"
	AccountWillingnessStateWaitingForAudit          AccountWillingnessState = "APPLYMENT_STATE_WAITTING_FOR_AUDIT"
	AccountWillingnessStateWaitingForConfirmContact AccountWillingnessState = "APPLYMENT_STATE_WAITTING_FOR_CONFIRM_CONTACT"
	AccountWillingnessStateWaitingForConfirmLegal   AccountWillingnessState = "APPLYMENT_STATE_WAITTING_FOR_CONFIRM_LEGALPERSON"
	AccountWillingnessStatePassed                   AccountWillingnessState = "APPLYMENT_STATE_PASSED"
	AccountWillingnessStateRejected                 AccountWillingnessState = "APPLYMENT_STATE_REJECTED"
	AccountWillingnessStateFreezed                  AccountWillingnessState = "APPLYMENT_STATE_FREEZED"
	AccountWillingnessStateCanceled                 AccountWillingnessState = "APPLYMENT_STATE_CANCELED"
)

type AccountWillingnessQueryResponse struct {
	BusinessCode   string                  `json:"business_code,omitempty"`
	ApplymentID    int64                   `json:"applyment_id,omitempty"`
	ApplymentState AccountWillingnessState `json:"applyment_state,omitempty"`
	State          AccountWillingnessState `json:"state,omitempty"`
	QRCodeData     string                  `json:"qrcode_data,omitempty"`
	RejectParam    string                  `json:"reject_param,omitempty"`
	RejectReason   string                  `json:"reject_reason,omitempty"`
}

type AccountAuthorizeStateRequest struct {
	SubMchID string `json:"sub_mchid,omitempty"`
}

type AccountAuthorizeStateResponse struct {
	SubMchID       string                `json:"sub_mchid,omitempty"`
	AuthorizeState AccountAuthorizeState `json:"authorize_state,omitempty"`
	Authorized     bool                  `json:"authorized,omitempty"`
}

type AccountAuthorizeState string

const (
	AccountAuthorizeStateUnauthorized AccountAuthorizeState = "AUTHORIZE_STATE_UNAUTHORIZED"
	AccountAuthorizeStateAuthorized   AccountAuthorizeState = "AUTHORIZE_STATE_AUTHORIZED"
)

type MerchantLimitationQueryRequest struct {
	SubMchID string `json:"sub_mchid,omitempty"`
}

type MerchantLimitationQueryResponse struct {
	SubMchID               string                          `json:"sub_mchid,omitempty"`
	MchID                  string                          `json:"mchid,omitempty"`
	LimitedFunctions       []MerchantLimitedFunction       `json:"limited_functions,omitempty"`
	OtherLimitedFunctions  string                          `json:"other_limited_functions,omitempty"`
	RecoverySpecifications []MerchantRecoverySpecification `json:"recovery_specifications,omitempty"`
	Limitations            []MerchantLimitation            `json:"limitations,omitempty"`
}

type MerchantLimitedFunction string

const (
	MerchantLimitedNoTransactionAndRecharge MerchantLimitedFunction = "NO_TRANSACTION_AND_RECHARGE"
	MerchantLimitedNoPayment                MerchantLimitedFunction = "NO_PAYMENT"
	MerchantLimitedNoWithdrawal             MerchantLimitedFunction = "NO_WITHDRAWAL"
	MerchantLimitedNoRefund                 MerchantLimitedFunction = "NO_REFUND"
	MerchantLimitedNoTransaction            MerchantLimitedFunction = "NO_TRANSACTION"
	MerchantLimitedNoProfitSharing          MerchantLimitedFunction = "NO_PROFIT_SHARING"
)

type MerchantRecoverySpecification struct {
	LimitationCaseID         string                    `json:"limitation_case_id,omitempty"`
	LimitationReasonType     string                    `json:"limitation_reason_type,omitempty"`
	LimitationReason         string                    `json:"limitation_reason,omitempty"`
	LimitationReasonDescribe string                    `json:"limitation_reason_describe,omitempty"`
	RelateLimitations        []MerchantLimitedFunction `json:"relate_limitations,omitempty"`
	OtherRelateLimitations   string                    `json:"other_relate_limitations,omitempty"`
	RecoverWay               string                    `json:"recover_way,omitempty"`
	RecoverWayParam          string                    `json:"recover_way_param,omitempty"`
	RecoverHelpURL           string                    `json:"recover_help_url,omitempty"`
	LimitationActionType     string                    `json:"limitation_action_type,omitempty"`
	LimitationStartDate      string                    `json:"limitation_start_date,omitempty"`
	LimitationDate           string                    `json:"limitation_date,omitempty"`
}

type MerchantLimitation struct {
	Capability     string   `json:"capability,omitempty"`
	Limited        bool     `json:"limited,omitempty"`
	Reason         string   `json:"reason,omitempty"`
	RecoverActions []string `json:"recover_actions,omitempty"`
}

type ViolationNotificationConfigRequest struct {
	NotifyURL string `json:"notify_url,omitempty"`
}

type ViolationNotificationConfigResponse struct {
	NotifyURL string `json:"notify_url,omitempty"`
}

type InactiveMerchantIdentityVerificationCreateRequest struct {
	SubMchID     string `json:"sub_mchid,omitempty"`
	BusinessCode string `json:"business_code,omitempty"`
}

type InactiveMerchantIdentityVerificationCreateResponse struct {
	VerificationID string `json:"verification_id,omitempty"`
	VerifyID       string `json:"verify_id,omitempty"`
}

type InactiveMerchantIdentityVerificationQueryRequest struct {
	SubMchID       string `json:"sub_mchid,omitempty"`
	VerificationID string `json:"verification_id,omitempty"`
	VerifyID       string `json:"verify_id,omitempty"`
}

type InactiveMerchantIdentityVerificationState string

const (
	InactiveMerchantIdentityVerificationProcessing InactiveMerchantIdentityVerificationState = "PROCESSING"
	InactiveMerchantIdentityVerificationSuccess    InactiveMerchantIdentityVerificationState = "SUCCESS"
	InactiveMerchantIdentityVerificationFailed     InactiveMerchantIdentityVerificationState = "FAIL"
)

type InactiveMerchantIdentityVerificationQueryResponse struct {
	SubMchID       string                                    `json:"sub_mchid,omitempty"`
	VerificationID string                                    `json:"verification_id,omitempty"`
	VerifyID       string                                    `json:"verify_id,omitempty"`
	State          InactiveMerchantIdentityVerificationState `json:"state,omitempty"`
	FailReason     string                                    `json:"fail_reason,omitempty"`
	Reason         string                                    `json:"reason,omitempty"`
	CreateTime     string                                    `json:"create_time,omitempty"`
	FinishTime     string                                    `json:"finish_time,omitempty"`
}

type MediaUploadResponse struct {
	MediaID string `json:"media_id,omitempty"`
}

type CapitalBank struct {
	BankAlias       string `json:"bank_alias,omitempty"`
	BankAliasCode   string `json:"bank_alias_code,omitempty"`
	AccountBank     string `json:"account_bank,omitempty"`
	AccountBankCode int64  `json:"account_bank_code,omitempty"`
	NeedBankBranch  bool   `json:"need_bank_branch,omitempty"`
}

type CapitalBankListResponse struct {
	TotalCount int64         `json:"total_count,omitempty"`
	Count      int64         `json:"count,omitempty"`
	Offset     int64         `json:"offset,omitempty"`
	Links      *PageLinks    `json:"links,omitempty"`
	Data       []CapitalBank `json:"data,omitempty"`
}

type CapitalBankAccountSearchResponse struct {
	TotalCount int64         `json:"total_count,omitempty"`
	Count      int64         `json:"count,omitempty"`
	Data       []CapitalBank `json:"data,omitempty"`
}

type CapitalProvince struct {
	ProvinceName string `json:"province_name,omitempty"`
	ProvinceCode int    `json:"province_code,omitempty"`
}

type CapitalProvinceListResponse struct {
	TotalCount int64             `json:"total_count,omitempty"`
	Count      int64             `json:"count,omitempty"`
	Data       []CapitalProvince `json:"data,omitempty"`
}

type CapitalCity struct {
	CityName string `json:"city_name,omitempty"`
	CityCode int    `json:"city_code,omitempty"`
}

type CapitalCityListResponse struct {
	TotalCount int64         `json:"total_count,omitempty"`
	Count      int64         `json:"count,omitempty"`
	Data       []CapitalCity `json:"data,omitempty"`
}

type CapitalBranch struct {
	BankBranchName string `json:"bank_branch_name,omitempty"`
	BankBranchID   string `json:"bank_branch_id,omitempty"`
}

type CapitalBranchListResponse struct {
	TotalCount      int64           `json:"total_count,omitempty"`
	Count           int64           `json:"count,omitempty"`
	Offset          int64           `json:"offset,omitempty"`
	Links           *PageLinks      `json:"links,omitempty"`
	Data            []CapitalBranch `json:"data,omitempty"`
	AccountBank     string          `json:"account_bank,omitempty"`
	AccountBankCode int64           `json:"account_bank_code,omitempty"`
	BankAlias       string          `json:"bank_alias,omitempty"`
	BankAliasCode   string          `json:"bank_alias_code,omitempty"`
}

type PageLinks struct {
	Next string `json:"next,omitempty"`
	Prev string `json:"prev,omitempty"`
	Self string `json:"self,omitempty"`
}

type PaymentPrepayRequest struct {
	SpAppID       string             `json:"sp_appid,omitempty"`
	SpMchID       string             `json:"sp_mchid,omitempty"`
	SubAppID      string             `json:"sub_appid,omitempty"`
	SubMchID      string             `json:"sub_mchid,omitempty"`
	Description   string             `json:"description,omitempty"`
	OutTradeNo    string             `json:"out_trade_no,omitempty"`
	TimeExpire    string             `json:"time_expire,omitempty"`
	Attach        string             `json:"attach,omitempty"`
	NotifyURL     string             `json:"notify_url,omitempty"`
	GoodsTag      string             `json:"goods_tag,omitempty"`
	SettleInfo    *PaymentSettleInfo `json:"settle_info,omitempty"`
	SupportFapiao bool               `json:"support_fapiao,omitempty"`
	Amount        PaymentAmount      `json:"amount,omitempty"`
	Payer         PaymentPayer       `json:"payer,omitempty"`
	Detail        *PaymentDetail     `json:"detail,omitempty"`
	SceneInfo     *PaymentSceneInfo  `json:"scene_info,omitempty"`
}

type PaymentPrepayResponse struct {
	PrepayID string `json:"prepay_id,omitempty"`
}

const JSAPIPaySignTypeRSA = "RSA"

// JSAPIPayParams is the ordinary service provider contract returned to wx.requestPayment.
type JSAPIPayParams struct {
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}

type PaymentSettleInfo struct {
	ProfitSharing bool `json:"profit_sharing,omitempty"`
}

type PaymentAmount struct {
	Total         int64    `json:"total,omitempty"`
	PayerTotal    int64    `json:"payer_total,omitempty"`
	Currency      Currency `json:"currency,omitempty"`
	PayerCurrency Currency `json:"payer_currency,omitempty"`
}

type PaymentPayer struct {
	SpOpenID  string `json:"sp_openid,omitempty"`
	SubOpenID string `json:"sub_openid,omitempty"`
}

type PaymentDetail struct {
	CostPrice   int64                `json:"cost_price,omitempty"`
	InvoiceID   string               `json:"invoice_id,omitempty"`
	GoodsDetail []PaymentGoodsDetail `json:"goods_detail,omitempty"`
}

type PaymentGoodsDetail struct {
	MerchantGoodsID  string `json:"merchant_goods_id,omitempty"`
	WechatpayGoodsID string `json:"wechatpay_goods_id,omitempty"`
	GoodsName        string `json:"goods_name,omitempty"`
	Quantity         int64  `json:"quantity,omitempty"`
	UnitPrice        int64  `json:"unit_price,omitempty"`
}

type PaymentSceneInfo struct {
	PayerClientIP string            `json:"payer_client_ip,omitempty"`
	DeviceID      string            `json:"device_id,omitempty"`
	StoreInfo     *PaymentStoreInfo `json:"store_info,omitempty"`
}

type PaymentStoreInfo struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	AreaCode string `json:"area_code,omitempty"`
	Address  string `json:"address,omitempty"`
}

type PaymentPromotionDetail struct {
	CouponID            string                        `json:"coupon_id,omitempty"`
	Name                string                        `json:"name,omitempty"`
	Scope               string                        `json:"scope,omitempty"`
	Type                string                        `json:"type,omitempty"`
	Amount              int64                         `json:"amount,omitempty"`
	StockID             string                        `json:"stock_id,omitempty"`
	WechatpayContribute int64                         `json:"wechatpay_contribute,omitempty"`
	MerchantContribute  int64                         `json:"merchant_contribute,omitempty"`
	OtherContribute     int64                         `json:"other_contribute,omitempty"`
	Currency            string                        `json:"currency,omitempty"`
	GoodsDetail         []PaymentPromotionGoodsDetail `json:"goods_detail,omitempty"`
}

type PaymentPromotionGoodsDetail struct {
	GoodsID        string `json:"goods_id,omitempty"`
	Quantity       int64  `json:"quantity,omitempty"`
	UnitPrice      int64  `json:"unit_price,omitempty"`
	DiscountAmount int64  `json:"discount_amount,omitempty"`
	GoodsRemark    string `json:"goods_remark,omitempty"`
}

type PaymentQueryRequest struct {
	SubMchID      string `json:"sub_mchid,omitempty"`
	TransactionID string `json:"transaction_id,omitempty"`
	OutTradeNo    string `json:"out_trade_no,omitempty"`
}

type PaymentTradeState string

const (
	PaymentTradeStateSuccess    PaymentTradeState = "SUCCESS"
	PaymentTradeStateRefund     PaymentTradeState = "REFUND"
	PaymentTradeStateNotPay     PaymentTradeState = "NOTPAY"
	PaymentTradeStateClosed     PaymentTradeState = "CLOSED"
	PaymentTradeStateRevoked    PaymentTradeState = "REVOKED"
	PaymentTradeStateUserPaying PaymentTradeState = "USERPAYING"
	PaymentTradeStatePayError   PaymentTradeState = "PAYERROR"
)

type PaymentQueryResponse struct {
	SpAppID         string                   `json:"sp_appid,omitempty"`
	SpMchID         string                   `json:"sp_mchid,omitempty"`
	SubAppID        string                   `json:"sub_appid,omitempty"`
	SubMchID        string                   `json:"sub_mchid,omitempty"`
	OutTradeNo      string                   `json:"out_trade_no,omitempty"`
	TransactionID   string                   `json:"transaction_id,omitempty"`
	TradeType       string                   `json:"trade_type,omitempty"`
	TradeState      PaymentTradeState        `json:"trade_state,omitempty"`
	TradeStateDesc  string                   `json:"trade_state_desc,omitempty"`
	BankType        string                   `json:"bank_type,omitempty"`
	Attach          string                   `json:"attach,omitempty"`
	SuccessTime     string                   `json:"success_time,omitempty"`
	Payer           PaymentPayer             `json:"payer,omitempty"`
	Amount          *PaymentAmount           `json:"amount,omitempty"`
	SceneInfo       *PaymentSceneInfo        `json:"scene_info,omitempty"`
	PromotionDetail []PaymentPromotionDetail `json:"promotion_detail,omitempty"`
}

type PaymentCloseRequest struct {
	SpMchID    string `json:"sp_mchid,omitempty"`
	SubMchID   string `json:"sub_mchid,omitempty"`
	OutTradeNo string `json:"out_trade_no,omitempty"`
}

func (r PaymentPrepayRequest) Validate() error {
	for _, field := range []struct{ name, value string }{
		{"sp_appid", r.SpAppID},
		{"sp_mchid", r.SpMchID},
		{"sub_mchid", r.SubMchID},
		{"description", r.Description},
		{"out_trade_no", r.OutTradeNo},
	} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	if err := validateHTTPSURL("notify_url", r.NotifyURL, true); err != nil {
		return err
	}
	if err := r.Amount.validate("amount", true); err != nil {
		return err
	}
	if strings.TrimSpace(r.Payer.SpOpenID) == "" && strings.TrimSpace(r.Payer.SubOpenID) == "" {
		return missing("payer.sp_openid_or_sub_openid")
	}
	if strings.TrimSpace(r.Payer.SubOpenID) != "" && strings.TrimSpace(r.SubAppID) == "" {
		return missing("sub_appid")
	}
	return nil
}

func (a PaymentAmount) validate(field string, allowEmptyCurrency bool) error {
	if err := requirePositiveInt(field+".total", a.Total); err != nil {
		return err
	}
	return validateCurrency(field+".currency", a.Currency, !allowEmptyCurrency)
}

type CombinePrepayRequest struct {
	CombineAppID      string            `json:"combine_appid,omitempty"`
	CombineMchID      string            `json:"combine_mchid,omitempty"`
	CombineOutTradeNo string            `json:"combine_out_trade_no,omitempty"`
	CombinePayerInfo  CombinePayerInfo  `json:"combine_payer_info,omitempty"`
	SceneInfo         *CombineSceneInfo `json:"scene_info,omitempty"`
	SubOrders         []CombineSubOrder `json:"sub_orders,omitempty"`
	TimeExpire        string            `json:"time_expire,omitempty"`
	NotifyURL         string            `json:"notify_url,omitempty"`
}

type CombinePrepayResponse struct {
	PrepayID string `json:"prepay_id,omitempty"`
}

type CombinePayerInfo struct {
	OpenID    string `json:"openid,omitempty"`
	SubOpenID string `json:"sub_openid,omitempty"`
}

type CombineSceneInfo struct {
	DeviceID      string `json:"device_id,omitempty"`
	PayerClientIP string `json:"payer_client_ip,omitempty"`
}

type CombineSubOrder struct {
	MchID       string             `json:"mchid,omitempty"`
	SubMchID    string             `json:"sub_mchid,omitempty"`
	SubAppID    string             `json:"sub_appid,omitempty"`
	OutTradeNo  string             `json:"out_trade_no,omitempty"`
	Amount      CombineAmount      `json:"amount,omitempty"`
	Attach      string             `json:"attach,omitempty"`
	Description string             `json:"description,omitempty"`
	Detail      string             `json:"detail,omitempty"`
	GoodsTag    string             `json:"goods_tag,omitempty"`
	SettleInfo  *CombineSettleInfo `json:"settle_info,omitempty"`
}

type CombineAmount struct {
	TotalAmount    int64    `json:"total_amount,omitempty"`
	PayerAmount    int64    `json:"payer_amount,omitempty"`
	Currency       Currency `json:"currency,omitempty"`
	PayerCurrency  Currency `json:"payer_currency,omitempty"`
	SettlementRate int64    `json:"settlement_rate,omitempty"`
}

type CombineSettleInfo struct {
	ProfitSharing bool `json:"profit_sharing,omitempty"`
}

type CombineQueryRequest struct {
	CombineMchID      string `json:"combine_mchid,omitempty"`
	CombineOutTradeNo string `json:"combine_out_trade_no,omitempty"`
}

type CombineQueryResponse struct {
	CombineAppID      string              `json:"combine_appid,omitempty"`
	CombineMchID      string              `json:"combine_mchid,omitempty"`
	CombineOutTradeNo string              `json:"combine_out_trade_no,omitempty"`
	TradeState        PaymentTradeState   `json:"trade_state,omitempty"`
	SubOrders         []CombineOrderState `json:"sub_orders,omitempty"`
}

type CombineOrderState struct {
	MchID           string                   `json:"mchid,omitempty"`
	SubMchID        string                   `json:"sub_mchid,omitempty"`
	SubAppID        string                   `json:"sub_appid,omitempty"`
	SubOpenID       string                   `json:"sub_openid,omitempty"`
	OutTradeNo      string                   `json:"out_trade_no,omitempty"`
	TransactionID   string                   `json:"transaction_id,omitempty"`
	TradeType       string                   `json:"trade_type,omitempty"`
	TradeState      PaymentTradeState        `json:"trade_state,omitempty"`
	BankType        string                   `json:"bank_type,omitempty"`
	Attach          string                   `json:"attach,omitempty"`
	SuccessTime     string                   `json:"success_time,omitempty"`
	Amount          CombineAmount            `json:"amount,omitempty"`
	PromotionDetail []PaymentPromotionDetail `json:"promotion_detail,omitempty"`
}

type CombineCloseRequest struct {
	CombineAppID      string                 `json:"combine_appid,omitempty"`
	CombineMchID      string                 `json:"combine_mchid,omitempty"`
	CombineOutTradeNo string                 `json:"combine_out_trade_no,omitempty"`
	SubOrders         []CombineCloseSubOrder `json:"sub_orders,omitempty"`
}

type CombineCloseSubOrder struct {
	MchID      string `json:"mchid,omitempty"`
	OutTradeNo string `json:"out_trade_no,omitempty"`
	SubMchID   string `json:"sub_mchid,omitempty"`
	SubAppID   string `json:"sub_appid,omitempty"`
}

func (r CombinePrepayRequest) Validate() error {
	for _, field := range []struct{ name, value string }{
		{"combine_appid", r.CombineAppID},
		{"combine_mchid", r.CombineMchID},
		{"combine_out_trade_no", r.CombineOutTradeNo},
	} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	if len(r.SubOrders) == 0 || len(r.SubOrders) > 50 {
		return ValidationError{Field: "sub_orders", Code: "invalid_count", Message: "combine payment requires 1 to 50 sub orders"}
	}
	if strings.TrimSpace(r.CombinePayerInfo.OpenID) == "" && strings.TrimSpace(r.CombinePayerInfo.SubOpenID) == "" {
		return missing("combine_payer_info.openid_or_sub_openid")
	}
	if err := validateHTTPSURL("notify_url", r.NotifyURL, false); err != nil {
		return err
	}
	subAppIDCount := 0
	for index, order := range r.SubOrders {
		prefix := fmt.Sprintf("sub_orders[%d]", index)
		if strings.TrimSpace(order.SubAppID) != "" {
			subAppIDCount++
		}
		if strings.TrimSpace(order.MchID) != strings.TrimSpace(r.CombineMchID) {
			return ValidationError{Field: prefix + ".mchid", Code: "mch_id_mismatch", Message: "sub order mchid must equal combine_mchid"}
		}
		if err := order.validate(prefix); err != nil {
			return err
		}
	}
	if subAppIDCount > 1 {
		return ValidationError{Field: "sub_orders.sub_appid", Code: "too_many_sub_appids", Message: "only one combine sub order may carry sub_appid"}
	}
	if strings.TrimSpace(r.CombinePayerInfo.SubOpenID) != "" && subAppIDCount == 0 {
		return missing("sub_orders.sub_appid")
	}
	return nil
}

func (o CombineSubOrder) validate(prefix string) error {
	for _, field := range []struct{ name, value string }{
		{prefix + ".sub_mchid", o.SubMchID},
		{prefix + ".out_trade_no", o.OutTradeNo},
		{prefix + ".attach", o.Attach},
		{prefix + ".description", o.Description},
	} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	if err := requirePositiveInt(prefix+".amount.total_amount", o.Amount.TotalAmount); err != nil {
		return err
	}
	return validateCurrency(prefix+".amount.currency", o.Amount.Currency, true)
}

type RefundCreateRequest struct {
	SubMchID      string              `json:"sub_mchid,omitempty"`
	TransactionID string              `json:"transaction_id,omitempty"`
	OutTradeNo    string              `json:"out_trade_no,omitempty"`
	OutRefundNo   string              `json:"out_refund_no,omitempty"`
	Reason        string              `json:"reason,omitempty"`
	NotifyURL     string              `json:"notify_url,omitempty"`
	Amount        RefundAmountRequest `json:"amount,omitempty"`
	GoodsDetail   []RefundGoodsDetail `json:"goods_detail,omitempty"`
}

type RefundAmountRequest struct {
	Refund   int64                 `json:"refund,omitempty"`
	From     []RefundFundsFromItem `json:"from,omitempty"`
	Total    int64                 `json:"total,omitempty"`
	Currency Currency              `json:"currency,omitempty"`
}

type RefundFundsFromItem struct {
	Account RefundFundsFromAccount `json:"account,omitempty"`
	Amount  int64                  `json:"amount,omitempty"`
}

type RefundFundsFromAccount string

const (
	RefundFundsFromAvailable   RefundFundsFromAccount = "AVAILABLE"
	RefundFundsFromUnavailable RefundFundsFromAccount = "UNAVAILABLE"
)

type RefundGoodsDetail struct {
	MerchantGoodsID  string `json:"merchant_goods_id,omitempty"`
	WechatpayGoodsID string `json:"wechatpay_goods_id,omitempty"`
	GoodsName        string `json:"goods_name,omitempty"`
	UnitPrice        int64  `json:"unit_price,omitempty"`
	RefundAmount     int64  `json:"refund_amount,omitempty"`
	RefundQuantity   int64  `json:"refund_quantity,omitempty"`
}

type RefundQueryRequest struct {
	SubMchID    string `json:"sub_mchid,omitempty"`
	RefundID    string `json:"refund_id,omitempty"`
	OutRefundNo string `json:"out_refund_no,omitempty"`
}

type RefundStatus string

const (
	RefundStatusSuccess    RefundStatus = "SUCCESS"
	RefundStatusClosed     RefundStatus = "CLOSED"
	RefundStatusProcessing RefundStatus = "PROCESSING"
	RefundStatusAbnormal   RefundStatus = "ABNORMAL"
)

type RefundChannel string

const (
	RefundChannelOriginal      RefundChannel = "ORIGINAL"
	RefundChannelBalance       RefundChannel = "BALANCE"
	RefundChannelOtherBalance  RefundChannel = "OTHER_BALANCE"
	RefundChannelOtherBankcard RefundChannel = "OTHER_BANKCARD"
)

type RefundFundsAccount string

const (
	RefundFundsAccountUnsettled   RefundFundsAccount = "UNSETTLED"
	RefundFundsAccountAvailable   RefundFundsAccount = "AVAILABLE"
	RefundFundsAccountUnavailable RefundFundsAccount = "UNAVAILABLE"
	RefundFundsAccountOperation   RefundFundsAccount = "OPERATION"
	RefundFundsAccountBasic       RefundFundsAccount = "BASIC"
	RefundFundsAccountECNYBasic   RefundFundsAccount = "ECNY_BASIC"
)

type RefundResponse struct {
	RefundID            string             `json:"refund_id,omitempty"`
	OutRefundNo         string             `json:"out_refund_no,omitempty"`
	TransactionID       string             `json:"transaction_id,omitempty"`
	OutTradeNo          string             `json:"out_trade_no,omitempty"`
	Channel             RefundChannel      `json:"channel,omitempty"`
	UserReceivedAccount string             `json:"user_received_account,omitempty"`
	SuccessTime         string             `json:"success_time,omitempty"`
	CreateTime          string             `json:"create_time,omitempty"`
	Status              RefundStatus       `json:"status,omitempty"`
	FundsAccount        RefundFundsAccount `json:"funds_account,omitempty"`
	Amount              *RefundAmount      `json:"amount,omitempty"`
	PromotionDetail     []RefundPromotion  `json:"promotion_detail,omitempty"`
}

type RefundAmount struct {
	Total            int64                 `json:"total,omitempty"`
	Refund           int64                 `json:"refund,omitempty"`
	From             []RefundFundsFromItem `json:"from,omitempty"`
	PayerTotal       int64                 `json:"payer_total,omitempty"`
	PayerRefund      int64                 `json:"payer_refund,omitempty"`
	SettlementRefund int64                 `json:"settlement_refund,omitempty"`
	SettlementTotal  int64                 `json:"settlement_total,omitempty"`
	DiscountRefund   int64                 `json:"discount_refund,omitempty"`
	Currency         Currency              `json:"currency,omitempty"`
	RefundFee        int64                 `json:"refund_fee,omitempty"`
}

type RefundPromotion struct {
	PromotionID  string              `json:"promotion_id,omitempty"`
	Scope        string              `json:"scope,omitempty"`
	Type         string              `json:"type,omitempty"`
	Amount       int64               `json:"amount,omitempty"`
	RefundAmount int64               `json:"refund_amount,omitempty"`
	GoodsDetail  []RefundGoodsDetail `json:"goods_detail,omitempty"`
}

func (r RefundCreateRequest) Validate() error {
	if err := requireString("sub_mchid", r.SubMchID); err != nil {
		return err
	}
	if strings.TrimSpace(r.TransactionID) == "" && strings.TrimSpace(r.OutTradeNo) == "" {
		return missing("transaction_id_or_out_trade_no")
	}
	if err := requireString("out_refund_no", r.OutRefundNo); err != nil {
		return err
	}
	if err := validateHTTPSURL("notify_url", r.NotifyURL, false); err != nil {
		return err
	}
	if err := requirePositiveInt("amount.refund", r.Amount.Refund); err != nil {
		return err
	}
	if err := requirePositiveInt("amount.total", r.Amount.Total); err != nil {
		return err
	}
	return validateCurrency("amount.currency", r.Amount.Currency, true)
}

type ReceiverType string

const (
	ReceiverTypeMerchantID        ReceiverType = "MERCHANT_ID"
	ReceiverTypePersonalOpenID    ReceiverType = "PERSONAL_OPENID"
	ReceiverTypePersonalSubOpenID ReceiverType = "PERSONAL_SUB_OPENID"
)

type ProfitSharingReceiverRelationType string

const (
	ProfitSharingRelationServiceProvider ProfitSharingReceiverRelationType = "SERVICE_PROVIDER"
	ProfitSharingRelationStore           ProfitSharingReceiverRelationType = "STORE"
	ProfitSharingRelationStaff           ProfitSharingReceiverRelationType = "STAFF"
	ProfitSharingRelationPartner         ProfitSharingReceiverRelationType = "PARTNER"
	ProfitSharingRelationUser            ProfitSharingReceiverRelationType = "USER"
	ProfitSharingRelationSupplier        ProfitSharingReceiverRelationType = "SUPPLIER"
	ProfitSharingRelationDistributor     ProfitSharingReceiverRelationType = "DISTRIBUTOR"
)

type ProfitSharingReceiverAddRequest struct {
	SubMchID       string                            `json:"sub_mchid,omitempty"`
	AppID          string                            `json:"appid,omitempty"`
	Type           ReceiverType                      `json:"type,omitempty"`
	Account        string                            `json:"account,omitempty"`
	Name           string                            `json:"name,omitempty"`
	RelationType   ProfitSharingReceiverRelationType `json:"relation_type,omitempty"`
	CustomRelation string                            `json:"custom_relation,omitempty"`
}

type ProfitSharingReceiverDeleteRequest struct {
	SubMchID string       `json:"sub_mchid,omitempty"`
	AppID    string       `json:"appid,omitempty"`
	Type     ReceiverType `json:"type,omitempty"`
	Account  string       `json:"account,omitempty"`
}

type ProfitSharingReceiverResponse struct {
	SubMchID       string                            `json:"sub_mchid,omitempty"`
	Type           ReceiverType                      `json:"type,omitempty"`
	Account        string                            `json:"account,omitempty"`
	Name           string                            `json:"name,omitempty"`
	RelationType   ProfitSharingReceiverRelationType `json:"relation_type,omitempty"`
	CustomRelation string                            `json:"custom_relation,omitempty"`
}

type ProfitSharingOrderRequest struct {
	SubMchID        string                  `json:"sub_mchid,omitempty"`
	AppID           string                  `json:"appid,omitempty"`
	SubAppID        string                  `json:"sub_appid,omitempty"`
	TransactionID   string                  `json:"transaction_id,omitempty"`
	OutOrderNo      string                  `json:"out_order_no,omitempty"`
	Receivers       []ProfitSharingReceiver `json:"receivers,omitempty"`
	UnfreezeUnsplit bool                    `json:"unfreeze_unsplit"`
}

type ProfitSharingReceiver struct {
	Type        ReceiverType `json:"type,omitempty"`
	Account     string       `json:"account,omitempty"`
	Name        string       `json:"name,omitempty"`
	Amount      int64        `json:"amount,omitempty"`
	Description string       `json:"description,omitempty"`
}

type ProfitSharingQueryRequest struct {
	SubMchID      string `json:"sub_mchid,omitempty"`
	TransactionID string `json:"transaction_id,omitempty"`
	OutOrderNo    string `json:"out_order_no,omitempty"`
}

type ProfitSharingOrderState string

const (
	ProfitSharingOrderStateProcessing ProfitSharingOrderState = "PROCESSING"
	ProfitSharingOrderStateFinished   ProfitSharingOrderState = "FINISHED"
)

type ProfitSharingReceiverResult string

const (
	ProfitSharingReceiverResultPending ProfitSharingReceiverResult = "PENDING"
	ProfitSharingReceiverResultSuccess ProfitSharingReceiverResult = "SUCCESS"
	ProfitSharingReceiverResultClosed  ProfitSharingReceiverResult = "CLOSED"
)

type ProfitSharingFailReason string

const (
	ProfitSharingFailReasonAccountAbnormal             ProfitSharingFailReason = "ACCOUNT_ABNORMAL"
	ProfitSharingFailReasonNoRelation                  ProfitSharingFailReason = "NO_RELATION"
	ProfitSharingFailReasonReceiverHighRisk            ProfitSharingFailReason = "RECEIVER_HIGH_RISK"
	ProfitSharingFailReasonReceiverRealNameNotVerified ProfitSharingFailReason = "RECEIVER_REAL_NAME_NOT_VERIFIED"
	ProfitSharingFailReasonNoAuth                      ProfitSharingFailReason = "NO_AUTH"
	ProfitSharingFailReasonReceiverReceiptLimit        ProfitSharingFailReason = "RECEIVER_RECEIPT_LIMIT"
	ProfitSharingFailReasonPayerAccountAbnormal        ProfitSharingFailReason = "PAYER_ACCOUNT_ABNORMAL"
	ProfitSharingFailReasonInvalidRequest              ProfitSharingFailReason = "INVALID_REQUEST"
)

type ProfitSharingOrderResponse struct {
	SubMchID      string                        `json:"sub_mchid,omitempty"`
	TransactionID string                        `json:"transaction_id,omitempty"`
	OutOrderNo    string                        `json:"out_order_no,omitempty"`
	OrderID       string                        `json:"order_id,omitempty"`
	State         ProfitSharingOrderState       `json:"state,omitempty"`
	Receivers     []ProfitSharingReceiverDetail `json:"receivers,omitempty"`
}

type ProfitSharingReceiverDetail struct {
	Amount      int64                       `json:"amount,omitempty"`
	Description string                      `json:"description,omitempty"`
	Type        ReceiverType                `json:"type,omitempty"`
	Account     string                      `json:"account,omitempty"`
	Result      ProfitSharingReceiverResult `json:"result,omitempty"`
	FailReason  ProfitSharingFailReason     `json:"fail_reason,omitempty"`
	CreateTime  string                      `json:"create_time,omitempty"`
	FinishTime  string                      `json:"finish_time,omitempty"`
	DetailID    string                      `json:"detail_id,omitempty"`
}

type ProfitSharingReturnRequest struct {
	SubMchID    string `json:"sub_mchid,omitempty"`
	OrderID     string `json:"order_id,omitempty"`
	OutOrderNo  string `json:"out_order_no,omitempty"`
	OutReturnNo string `json:"out_return_no,omitempty"`
	ReturnMchID string `json:"return_mchid,omitempty"`
	Amount      int64  `json:"amount,omitempty"`
	Description string `json:"description,omitempty"`
}

type ProfitSharingReturnQueryRequest struct {
	SubMchID    string `json:"sub_mchid,omitempty"`
	OrderID     string `json:"order_id,omitempty"`
	OutOrderNo  string `json:"out_order_no,omitempty"`
	OutReturnNo string `json:"out_return_no,omitempty"`
}

type ProfitSharingReturnState string

const (
	ProfitSharingReturnStateProcessing ProfitSharingReturnState = "PROCESSING"
	ProfitSharingReturnStateSuccess    ProfitSharingReturnState = "SUCCESS"
	ProfitSharingReturnStateFailed     ProfitSharingReturnState = "FAILED"
)

type ProfitSharingReturnResponse struct {
	SubMchID    string                   `json:"sub_mchid,omitempty"`
	OrderID     string                   `json:"order_id,omitempty"`
	OutOrderNo  string                   `json:"out_order_no,omitempty"`
	OutReturnNo string                   `json:"out_return_no,omitempty"`
	ReturnID    string                   `json:"return_id,omitempty"`
	ReturnMchID string                   `json:"return_mchid,omitempty"`
	Amount      int64                    `json:"amount,omitempty"`
	Description string                   `json:"description,omitempty"`
	State       ProfitSharingReturnState `json:"state,omitempty"`
	FailReason  string                   `json:"fail_reason,omitempty"`
	CreateTime  string                   `json:"create_time,omitempty"`
	FinishTime  string                   `json:"finish_time,omitempty"`
}

type ProfitSharingUnfreezeRequest struct {
	SubMchID      string `json:"sub_mchid,omitempty"`
	TransactionID string `json:"transaction_id,omitempty"`
	OutOrderNo    string `json:"out_order_no,omitempty"`
	Description   string `json:"description,omitempty"`
}

type ProfitSharingUnfreezeResponse struct {
	SubMchID      string                  `json:"sub_mchid,omitempty"`
	TransactionID string                  `json:"transaction_id,omitempty"`
	OutOrderNo    string                  `json:"out_order_no,omitempty"`
	OrderID       string                  `json:"order_id,omitempty"`
	State         ProfitSharingOrderState `json:"state,omitempty"`
}

type ProfitSharingRemainingAmountRequest struct {
	SubMchID      string `json:"sub_mchid,omitempty"`
	TransactionID string `json:"transaction_id,omitempty"`
}

type ProfitSharingRemainingAmountResponse struct {
	SubMchID      string `json:"sub_mchid,omitempty"`
	TransactionID string `json:"transaction_id,omitempty"`
	Amount        int64  `json:"amount,omitempty"`
}

func (r ProfitSharingOrderRequest) Validate() error {
	for _, field := range []struct{ name, value string }{
		{"sub_mchid", r.SubMchID},
		{"transaction_id", r.TransactionID},
		{"out_order_no", r.OutOrderNo},
	} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	if len(r.Receivers) == 0 || len(r.Receivers) > 50 {
		return ValidationError{Field: "receivers", Code: "invalid_count", Message: "profit sharing requires 1 to 50 receivers"}
	}
	for index, receiver := range r.Receivers {
		if err := receiver.validate(fmt.Sprintf("receivers[%d]", index)); err != nil {
			return err
		}
	}
	return nil
}

func (r ProfitSharingReceiver) validate(prefix string) error {
	if r.Type != ReceiverTypeMerchantID && r.Type != ReceiverTypePersonalOpenID && r.Type != ReceiverTypePersonalSubOpenID {
		return invalidEnum(prefix + ".type")
	}
	for _, field := range []struct{ name, value string }{
		{prefix + ".account", r.Account},
		{prefix + ".description", r.Description},
	} {
		if err := requireString(field.name, field.value); err != nil {
			return err
		}
	}
	if r.Type == ReceiverTypeMerchantID {
		if err := requireString(prefix+".name", r.Name); err != nil {
			return err
		}
	}
	return requirePositiveInt(prefix+".amount", r.Amount)
}

func salesSceneSelected(scenes []ApplymentSalesSceneType, target ApplymentSalesSceneType) bool {
	for _, scene := range scenes {
		if scene == target {
			return true
		}
	}
	return false
}

func requireString(field string, value string) error {
	if strings.TrimSpace(value) == "" {
		return missing(field)
	}
	return nil
}

func requirePositiveInt(field string, value int64) error {
	if value <= 0 {
		return ValidationError{Field: field, Code: "must_be_positive", Message: "must be greater than zero"}
	}
	return nil
}

func validateHTTPSURL(field string, value string, required bool) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if required {
			return missing(field)
		}
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ValidationError{Field: field, Code: "https_url_required", Message: "must be an https URL"}
	}
	return nil
}

func validateCurrency(field string, value Currency, required bool) error {
	if value == "" {
		if required {
			return missing(field)
		}
		return nil
	}
	if value != CurrencyCNY {
		return ValidationError{Field: field, Code: "unsupported_currency", Message: "must be CNY"}
	}
	return nil
}

func missing(field string) ValidationError {
	return ValidationError{Field: field, Code: "required", Message: "is required"}
}

func invalidEnum(field string) ValidationError {
	return ValidationError{Field: field, Code: "invalid_enum", Message: "contains an unsupported enum value"}
}
