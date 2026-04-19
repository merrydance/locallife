package contracts

// 官方文档：POST /v3/ecommerce/applyments/
type EcommerceApplymentRequest struct {
	OutRequestNo           string                           `json:"out_request_no"`
	OrganizationType       string                           `json:"organization_type"`
	FinanceInstitution     *bool                            `json:"finance_institution,omitempty"`
	BusinessLicenseInfo    *ApplymentBusinessLicenseInfo    `json:"business_license_info,omitempty"`
	FinanceInstitutionInfo *ApplymentFinanceInstitutionInfo `json:"finance_institution_info,omitempty"`
	IDHolderType           string                           `json:"id_holder_type,omitempty"`
	IDDocType              string                           `json:"id_doc_type,omitempty"`
	AuthorizeLetterCopy    string                           `json:"authorize_letter_copy,omitempty"`
	IDCardInfo             *ApplymentIDCardInfo             `json:"id_card_info,omitempty"`
	IDDocInfo              *ApplymentIDDocInfo              `json:"id_doc_info,omitempty"`
	Owner                  *bool                            `json:"owner,omitempty"`
	UBOInfoList            []ApplymentUBOInfo               `json:"ubo_info_list,omitempty"`
	AccountInfo            *ApplymentBankAccountInfo        `json:"account_info"`
	ContactInfo            *ApplymentContactInfo            `json:"contact_info"`
	SalesSceneInfo         *ApplymentSalesSceneInfo         `json:"sales_scene_info"`
	SettlementInfo         *ApplymentSettlementInfo         `json:"settlement_info,omitempty"`
	MerchantShortname      string                           `json:"merchant_shortname"`
	Qualifications         []string                         `json:"qualifications,omitempty"`
	BusinessAdditionPics   []string                         `json:"business_addition_pics,omitempty"`
	BusinessAdditionDesc   string                           `json:"business_addition_desc,omitempty"`
}

type ApplymentBusinessLicenseInfo struct {
	CertType              string `json:"cert_type,omitempty"`
	BusinessLicenseCopy   string `json:"business_license_copy"`
	BusinessLicenseNumber string `json:"business_license_number"`
	MerchantName          string `json:"merchant_name"`
	LegalPerson           string `json:"legal_person"`
	CompanyAddress        string `json:"company_address,omitempty"`
	BusinessTime          string `json:"business_time,omitempty"`
}

type ApplymentFinanceInstitutionInfo struct {
	FinanceType        string   `json:"finance_type"`
	FinanceLicensePics []string `json:"finance_license_pics"`
}

type ApplymentIDCardInfo struct {
	IDCardCopy           string `json:"id_card_copy"`
	IDCardNational       string `json:"id_card_national"`
	IDCardName           string `json:"id_card_name"`
	IDCardNumber         string `json:"id_card_number"`
	IDCardValidTimeBegin string `json:"id_card_valid_time_begin"`
	IDCardValidTime      string `json:"id_card_valid_time"`
}

type ApplymentIDDocInfo struct {
	IDDocName      string `json:"id_doc_name"`
	IDDocNumber    string `json:"id_doc_number"`
	IDDocCopy      string `json:"id_doc_copy"`
	IDDocCopyBack  string `json:"id_doc_copy_back,omitempty"`
	DocPeriodBegin string `json:"doc_period_begin"`
	DocPeriodEnd   string `json:"doc_period_end"`
}

type ApplymentUBOInfo struct {
	UBOIDDocType        string `json:"ubo_id_doc_type,omitempty"`
	UBOIDDocCopy        string `json:"ubo_id_doc_copy,omitempty"`
	UBOIDDocCopyBack    string `json:"ubo_id_doc_copy_back,omitempty"`
	UBOIDDocName        string `json:"ubo_id_doc_name,omitempty"`
	UBOIDDocNumber      string `json:"ubo_id_doc_number,omitempty"`
	UBOIDDocAddress     string `json:"ubo_id_doc_address,omitempty"`
	UBOIDDocPeriodBegin string `json:"ubo_id_doc_period_begin,omitempty"`
	UBOIDDocPeriodEnd   string `json:"ubo_id_doc_period_end,omitempty"`
}

type ApplymentBankAccountInfo struct {
	BankAccountType string `json:"bank_account_type"`
	AccountBank     string `json:"account_bank"`
	AccountName     string `json:"account_name"`
	BankAddressCode string `json:"bank_address_code,omitempty"`
	BankBranchID    string `json:"bank_branch_id,omitempty"`
	BankName        string `json:"bank_name,omitempty"`
	AccountNumber   string `json:"account_number"`
}

type ApplymentContactInfo struct {
	ContactType                 string `json:"contact_type"`
	ContactName                 string `json:"contact_name"`
	ContactIDDocType            string `json:"contact_id_doc_type,omitempty"`
	ContactIDCardNumber         string `json:"contact_id_card_number,omitempty"`
	ContactIDDocCopy            string `json:"contact_id_doc_copy,omitempty"`
	ContactIDDocCopyBack        string `json:"contact_id_doc_copy_back,omitempty"`
	ContactIDDocPeriodBegin     string `json:"contact_id_doc_period_begin,omitempty"`
	ContactIDDocPeriodEnd       string `json:"contact_id_doc_period_end,omitempty"`
	BusinessAuthorizationLetter string `json:"business_authorization_letter,omitempty"`
	MobilePhone                 string `json:"mobile_phone"`
}

type ApplymentSalesSceneInfo struct {
	StoreName           string `json:"store_name"`
	StoreURL            string `json:"store_url,omitempty"`
	StoreQRCode         string `json:"store_qr_code,omitempty"`
	MiniProgramSubAppID string `json:"mini_program_sub_appid,omitempty"`
}

type ApplymentSettlementInfo struct {
	SettlementID      int64  `json:"settlement_id,omitempty"`
	QualificationType string `json:"qualification_type,omitempty"`
}

// 接口路径：GET /v3/capital/capitallhh/banks/personal-banking
// 接口路径：GET /v3/capital/capitallhh/banks/corporate-banking
type CapitalBankListRequest struct {
	Offset int
	Limit  int
}

type CapitalBank struct {
	BankAlias       string `json:"bank_alias"`
	BankAliasCode   string `json:"bank_alias_code"`
	AccountBank     string `json:"account_bank"`
	AccountBankCode int64  `json:"account_bank_code"`
	NeedBankBranch  bool   `json:"need_bank_branch"`
}

type CapitalPaginationLinks struct {
	Next string `json:"next,omitempty"`
	Prev string `json:"prev,omitempty"`
	Self string `json:"self,omitempty"`
}

type CapitalBankListResponse struct {
	TotalCount int                    `json:"total_count"`
	Count      int                    `json:"count"`
	Data       []CapitalBank          `json:"data,omitempty"`
	Offset     int                    `json:"offset"`
	Links      CapitalPaginationLinks `json:"links"`
}

// 接口路径：GET /v3/capital/capitallhh/banks/search-banks-by-bank-account
type CapitalBankAccountSearchRequest struct {
	AccountNumber string
}

type CapitalBankAccountSearchResponse struct {
	TotalCount int           `json:"total_count"`
	Data       []CapitalBank `json:"data,omitempty"`
}

// 接口路径：GET /v3/capital/capitallhh/areas/provinces
type CapitalProvince struct {
	ProvinceName string `json:"province_name"`
	ProvinceCode int    `json:"province_code"`
}

type CapitalProvinceListResponse struct {
	Data       []CapitalProvince `json:"data,omitempty"`
	TotalCount int               `json:"total_count"`
}

// 接口路径：GET /v3/capital/capitallhh/areas/provinces/{province_code}/cities
type CapitalCityListRequest struct {
	ProvinceCode int
}

type CapitalCity struct {
	CityName string `json:"city_name"`
	CityCode int    `json:"city_code"`
}

type CapitalCityListResponse struct {
	Data       []CapitalCity `json:"data,omitempty"`
	TotalCount int           `json:"total_count"`
}

// 官方文档：GET /v3/capital/capitallhh/banks/{bank_alias_code}/branches
type CapitalBranchListRequest struct {
	BankAliasCode string
	CityCode      int
	Offset        int
	Limit         int
}

type CapitalBranch struct {
	BankBranchName string `json:"bank_branch_name"`
	BankBranchID   string `json:"bank_branch_id"`
}

type CapitalBranchListResponse struct {
	TotalCount      int                    `json:"total_count"`
	Count           int                    `json:"count"`
	Data            []CapitalBranch        `json:"data,omitempty"`
	Offset          int                    `json:"offset"`
	Links           CapitalPaginationLinks `json:"links"`
	AccountBank     string                 `json:"account_bank"`
	AccountBankCode int64                  `json:"account_bank_code"`
	BankAlias       string                 `json:"bank_alias"`
	BankAliasCode   string                 `json:"bank_alias_code"`
}

// 官方文档：POST /v3/ecommerce/applyments/
type EcommerceApplymentResponse struct {
	ApplymentID  int64  `json:"applyment_id"`
	OutRequestNo string `json:"out_request_no"`
}

// 官方文档：GET /v3/ecommerce/applyments/out-request-no/{out_request_no}
type EcommerceApplymentQueryByOutRequestNoRequest struct {
	OutRequestNo string
}

// 官方文档：GET /v3/ecommerce/applyments/{applyment_id}
type EcommerceApplymentQueryByIDRequest struct {
	ApplymentID int64
}

type EcommerceApplymentAccountValidation struct {
	AccountName              string `json:"account_name,omitempty"`
	AccountNo                string `json:"account_no,omitempty"`
	PayAmount                int64  `json:"pay_amount,omitempty"`
	DestinationAccountNumber string `json:"destination_account_number,omitempty"`
	DestinationAccountName   string `json:"destination_account_name,omitempty"`
	DestinationAccountBank   string `json:"destination_account_bank,omitempty"`
	City                     string `json:"city,omitempty"`
	Remark                   string `json:"remark,omitempty"`
	Deadline                 string `json:"deadline,omitempty"`
	RawAccountName           string `json:"-"`
	RawAccountNo             string `json:"-"`
}

type ApplymentAuditDetail struct {
	ParamName    string `json:"param_name"`
	RejectReason string `json:"reject_reason"`
}

// 官方文档：GET /v3/ecommerce/applyments/out-request-no/{out_request_no}
// 官方文档：GET /v3/ecommerce/applyments/{applyment_id}
type EcommerceApplymentQueryResponse struct {
	ApplymentState     string                               `json:"applyment_state"`
	ApplymentStateDesc string                               `json:"applyment_state_desc"`
	SignURL            string                               `json:"sign_url,omitempty"`
	SubMchID           string                               `json:"sub_mchid,omitempty"`
	AccountValidation  *EcommerceApplymentAccountValidation `json:"account_validation,omitempty"`
	AuditDetail        []ApplymentAuditDetail               `json:"audit_detail,omitempty"`
	LegalValidationURL string                               `json:"legal_validation_url,omitempty"`
	OutRequestNo       string                               `json:"out_request_no"`
	ApplymentID        int64                                `json:"applyment_id"`
	SignState          string                               `json:"sign_state,omitempty"`
}

// 官方文档：GET /v3/apply4sub/sub_merchants/{sub_mchid}/settlement
type QuerySubMerchantSettlementRequest struct {
	SubMchID          string
	AccountNumberRule string
}

// 官方文档：GET /v3/apply4sub/sub_merchants/{sub_mchid}/settlement
type SubMerchantSettlementResponse struct {
	AccountType      string `json:"account_type"`
	AccountBank      string `json:"account_bank"`
	BankName         string `json:"bank_name,omitempty"`
	BankBranchID     string `json:"bank_branch_id,omitempty"`
	AccountNumber    string `json:"account_number"`
	VerifyResult     string `json:"verify_result"`
	VerifyFailReason string `json:"verify_fail_reason,omitempty"`
}

// 官方文档：POST /v3/apply4sub/sub_merchants/{sub_mchid}/modify-settlement
type ModifySubMerchantSettlementRequest struct {
	AccountType   string `json:"account_type"`
	AccountBank   string `json:"account_bank"`
	BankName      string `json:"bank_name,omitempty"`
	BankBranchID  string `json:"bank_branch_id,omitempty"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name,omitempty"`
}

type ModifySubMerchantSettlementResponse struct {
	ApplicationNo string `json:"application_no"`
}

// 官方文档：GET /v3/apply4sub/sub_merchants/{sub_mchid}/application/{application_no}
type QuerySubMerchantSettlementApplicationRequest struct {
	SubMchID          string
	ApplicationNo     string
	AccountNumberRule string
}

// 官方文档：GET /v3/apply4sub/sub_merchants/{sub_mchid}/application/{application_no}
type QuerySubMerchantSettlementApplicationResponse struct {
	AccountName      string `json:"account_name"`
	AccountType      string `json:"account_type"`
	AccountBank      string `json:"account_bank"`
	BankName         string `json:"bank_name,omitempty"`
	BankBranchID     string `json:"bank_branch_id,omitempty"`
	AccountNumber    string `json:"account_number"`
	VerifyResult     string `json:"verify_result"`
	VerifyFailReason string `json:"verify_fail_reason,omitempty"`
	VerifyFinishTime string `json:"verify_finish_time,omitempty"`
}

// 官方文档：POST /v3/merchant/media/upload
type ImageUploadRequest struct {
	File []byte                  `json:"file"`
	Meta MerchantMediaUploadMeta `json:"meta"`
}

// 官方文档：POST /v3/merchant/media/upload
type MerchantMediaUploadMeta struct {
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
}

type ImageUploadResponse struct {
	MediaID string `json:"media_id"`
}

const (
	ApplymentOrganizationTypeIndividualBusiness = "4"
	ApplymentOrganizationTypeEnterprise         = "2"
)

const (
	ApplymentFinanceTypeBankAgent      = "BANK_AGENT"
	ApplymentFinanceTypePaymentAgent   = "PAYMENT_AGENT"
	ApplymentFinanceTypeInsurance      = "INSURANCE"
	ApplymentFinanceTypeTradeAndSettle = "TRADE_AND_SETTLE"
	ApplymentFinanceTypeOther          = "OTHER"
)

const (
	ApplymentIDHolderTypeLegal = "LEGAL"
	ApplymentIDHolderTypeSuper = "SUPER"
)

const (
	ApplymentContactTypeLegal = "65"
	ApplymentContactTypeSuper = "66"
)

const (
	ApplymentBankAccountTypeBusiness = "74"
	ApplymentBankAccountTypePrivate  = "75"
)

const (
	SubMerchantSettlementAccountTypeBusiness = "ACCOUNT_TYPE_BUSINESS"
	SubMerchantSettlementAccountTypePrivate  = "ACCOUNT_TYPE_PRIVATE"
)

const (
	ApplymentStateChecking          = "CHECKING"
	ApplymentStateAccountNeedVerify = "ACCOUNT_NEED_VERIFY"
	ApplymentStateAuditing          = "AUDITING"
	ApplymentStateRejected          = "REJECTED"
	ApplymentStateNeedSign          = "NEED_SIGN"
	ApplymentStateFinish            = "FINISH"
	ApplymentStateFrozen            = "FROZEN"
	ApplymentStateCanceled          = "CANCELED"
)

const (
	ApplymentSignStateUnsigned    = "UNSIGNED"
	ApplymentSignStateSigned      = "SIGNED"
	ApplymentSignStateNotSignable = "NOT_SIGNABLE"
)

const (
	ApplymentIdentificationTypeMainlandIDCard        = "IDENTIFICATION_TYPE_MAINLAND_IDCARD"
	ApplymentIdentificationTypeOverseaPassport       = "IDENTIFICATION_TYPE_OVERSEA_PASSPORT"
	ApplymentIdentificationTypeHongKong              = "IDENTIFICATION_TYPE_HONGKONG"
	ApplymentIdentificationTypeMacao                 = "IDENTIFICATION_TYPE_MACAO"
	ApplymentIdentificationTypeTaiwan                = "IDENTIFICATION_TYPE_TAIWAN"
	ApplymentIdentificationTypeForeignResident       = "IDENTIFICATION_TYPE_FOREIGN_RESIDENT"
	ApplymentIdentificationTypeHongKongMacaoResident = "IDENTIFICATION_TYPE_HONGKONG_MACAO_RESIDENT"
	ApplymentIdentificationTypeTaiwanResident        = "IDENTIFICATION_TYPE_TAIWAN_RESIDENT"
)

const (
	ApplymentCertificateType2388 = "CERTIFICATE_TYPE_2388"
	ApplymentCertificateType2389 = "CERTIFICATE_TYPE_2389"
	ApplymentCertificateType2394 = "CERTIFICATE_TYPE_2394"
	ApplymentCertificateType2395 = "CERTIFICATE_TYPE_2395"
	ApplymentCertificateType2396 = "CERTIFICATE_TYPE_2396"
	ApplymentCertificateType2399 = "CERTIFICATE_TYPE_2399"
	ApplymentCertificateType2400 = "CERTIFICATE_TYPE_2400"
	ApplymentCertificateType2520 = "CERTIFICATE_TYPE_2520"
	ApplymentCertificateType2521 = "CERTIFICATE_TYPE_2521"
	ApplymentCertificateType2522 = "CERTIFICATE_TYPE_2522"
)

const (
	SubMerchantSettlementAccountNumberRuleMaskV1 = "ACCOUNT_NUMBER_RULE_MASK_V1"
	SubMerchantSettlementAccountNumberRuleMaskV2 = "ACCOUNT_NUMBER_RULE_MASK_V2"
)

const (
	SubMerchantSettlementVerifyResultSuccess   = "VERIFY_SUCCESS"
	SubMerchantSettlementVerifyResultFail      = "VERIFY_FAIL"
	SubMerchantSettlementVerifyResultVerifying = "VERIFYING"
)

const (
	SubMerchantSettlementApplicationAuditSuccess = "AUDIT_SUCCESS"
	SubMerchantSettlementApplicationAuditing     = "AUDITING"
	SubMerchantSettlementApplicationAuditFail    = "AUDIT_FAIL"
)
