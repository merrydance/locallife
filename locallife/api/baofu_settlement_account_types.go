package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/merrydance/locallife/logic"
)

const (
	baofuSettlementProfileStatusDraft     = "draft"
	baofuSettlementAccountServiceNotReady = "baofu settlement account service is not configured"
	baofuSettlementAccountTypeBusiness    = "ACCOUNT_TYPE_BUSINESS"
	baofuSettlementAccountTypePrivate     = "ACCOUNT_TYPE_PRIVATE"
)

type baofuSettlementAccountScope struct {
	OwnerType           string
	OwnerID             int64
	OwnerUserID         int64
	AccountType         string
	Audience            string
	DefaultProfile      *logic.BaofuAccountOpeningProfileInput
	DefaultProfileMasks *baofuSettlementAccountProfileDefaults
}

type baofuSettlementAccountClientControlledFieldError struct {
	Field string
}

func (err baofuSettlementAccountClientControlledFieldError) Error() string {
	return fmt.Sprintf("baofu settlement account field %s is controlled by server", err.Field)
}

type baofuSettlementAccountRoleFieldError struct {
	Field     string
	Role      string
	OwnerType string
}

func (err baofuSettlementAccountRoleFieldError) Error() string {
	return fmt.Sprintf("baofu settlement account field %s is not allowed for %s", err.Field, err.Role)
}

type baofuSettlementAccountPaymentLoadError struct {
	FlowID         int64
	PaymentOrderID int64
	Err            error
}

func (err baofuSettlementAccountPaymentLoadError) Error() string {
	return fmt.Sprintf("load baofu verify fee payment order %d: %v", err.PaymentOrderID, err.Err)
}

func (err baofuSettlementAccountPaymentLoadError) Unwrap() error {
	return err.Err
}

func isBaofuSettlementAccountServiceNotReady(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "service is not configured") || strings.Contains(message, "client is not configured")
}

type baofuSettlementAccountRequest struct {
	AccountOpeningMode string                               `json:"account_opening_mode"`
	Profile            baofuSettlementAccountProfileRequest `json:"profile"`
}

type baofuSettlementAccountProfileRequest struct {
	LegalName             string `json:"legal_name"`
	AccountName           string `json:"account_name"`
	RealName              string `json:"real_name"`
	BusinessLicenseNumber string `json:"business_license_number"`
	CertificateNo         string `json:"certificate_no"`
	IDCardNumber          string `json:"id_card_number"`
	LegalPersonName       string `json:"legal_person_name"`
	LegalPersonIDNumber   string `json:"legal_person_id_number"`
	CorporateMobile       string `json:"corporate_mobile"`
	Email                 string `json:"email"`
	BankAccountNo         string `json:"bank_account_no"`
	BankAccountNumber     string `json:"bank_account_number"`
	AccountNumber         string `json:"account_number"`
	BankMobile            string `json:"bank_mobile"`
	Mobile                string `json:"mobile"`
	Phone                 string `json:"phone"`
	BankName              string `json:"bank_name"`
	DepositBankProvince   string `json:"deposit_bank_province"`
	DepositBankCity       string `json:"deposit_bank_city"`
	DepositBankName       string `json:"deposit_bank_name"`
	ContactName           string `json:"contact_name"`
	ContactMobile         string `json:"contact_mobile"`
	CardUserName          string `json:"card_user_name"`
	SelfEmployed          bool   `json:"self_employed"`
	SelfEmployedSet       bool   `json:"-"`
}

type baofuSettlementAccountPaymentResponse struct {
	PaymentOrderID int64                 `json:"payment_order_id,omitempty"`
	Amount         int64                 `json:"amount,omitempty"`
	BusinessType   string                `json:"business_type,omitempty"`
	OutTradeNo     string                `json:"out_trade_no,omitempty"`
	PayParams      *miniProgramPayParams `json:"pay_params,omitempty"`
	ExpiresAt      *time.Time            `json:"expires_at,omitempty"`
}

type baofuSettlementAccountProfileDefaults struct {
	Source                        string   `json:"source,omitempty"`
	LegalName                     string   `json:"legal_name,omitempty"`
	CertificateNo                 string   `json:"certificate_no,omitempty"`
	CertificateNoMask             string   `json:"certificate_no_mask,omitempty"`
	BusinessLicenseNumber         string   `json:"business_license_number,omitempty"`
	LegalPersonName               string   `json:"legal_person_name,omitempty"`
	CardUserName                  string   `json:"card_user_name,omitempty"`
	SelfEmployed                  bool     `json:"self_employed,omitempty"`
	LegalPersonIDNumber           string   `json:"legal_person_id_number,omitempty"`
	LegalPersonIDNumberMask       string   `json:"legal_person_id_number_mask,omitempty"`
	CorporateMobile               string   `json:"corporate_mobile,omitempty"`
	CorporateMobileMask           string   `json:"corporate_mobile_mask,omitempty"`
	Email                         string   `json:"email,omitempty"`
	EmailMask                     string   `json:"email_mask,omitempty"`
	BankAccountNo                 string   `json:"bank_account_no,omitempty"`
	BankAccountNoMask             string   `json:"bank_account_no_mask,omitempty"`
	BankMobile                    string   `json:"bank_mobile,omitempty"`
	BankName                      string   `json:"bank_name,omitempty"`
	DepositBankProvince           string   `json:"deposit_bank_province,omitempty"`
	DepositBankCity               string   `json:"deposit_bank_city,omitempty"`
	DepositBankName               string   `json:"deposit_bank_name,omitempty"`
	BankAddressCode               string   `json:"bank_address_code,omitempty"`
	BankBranchID                  string   `json:"bank_branch_id,omitempty"`
	AccountBank                   string   `json:"account_bank,omitempty"`
	AccountBankCode               int64    `json:"account_bank_code,omitempty"`
	BankAlias                     string   `json:"bank_alias,omitempty"`
	BankAliasCode                 string   `json:"bank_alias_code,omitempty"`
	ContactName                   string   `json:"contact_name,omitempty"`
	ContactMobileMask             string   `json:"contact_mobile_mask,omitempty"`
	SettlementAccountAllowedTypes []string `json:"settlement_account_allowed_types,omitempty"`
	HasLegalPersonIDNumber        bool     `json:"has_legal_person_id_number,omitempty"`
	HasCorporateMobile            bool     `json:"has_corporate_mobile,omitempty"`
	HasCertificateNo              bool     `json:"has_certificate_no,omitempty"`
	HasEmail                      bool     `json:"has_email,omitempty"`
	HasBankAccountNo              bool     `json:"has_bank_account_no,omitempty"`
	HasContactMobile              bool     `json:"has_contact_mobile,omitempty"`
	HasSavedSensitiveDefaults     bool     `json:"has_saved_sensitive_defaults,omitempty"`
}

type baofuSettlementAccountResponse struct {
	OwnerType          string                                 `json:"owner_type"`
	OwnerID            int64                                  `json:"owner_id"`
	AccountType        string                                 `json:"account_type"`
	Status             string                                 `json:"status"`
	State              string                                 `json:"state"`
	Label              string                                 `json:"label"`
	StatusDesc         string                                 `json:"status_desc,omitempty"`
	PaymentReady       bool                                   `json:"payment_ready"`
	OpenState          string                                 `json:"open_state,omitempty"`
	ProfileStatus      string                                 `json:"profile_status,omitempty"`
	MissingFields      []string                               `json:"missing_fields,omitempty"`
	FlowID             int64                                  `json:"flow_id,omitempty"`
	FlowState          string                                 `json:"flow_state,omitempty"`
	VerifyFeeAmount    int64                                  `json:"verify_fee_amount,omitempty"`
	PaymentOrderID     int64                                  `json:"payment_order_id,omitempty"`
	Amount             int64                                  `json:"amount,omitempty"`
	BusinessType       string                                 `json:"business_type,omitempty"`
	OutTradeNo         string                                 `json:"out_trade_no,omitempty"`
	PayParams          *miniProgramPayParams                  `json:"pay_params,omitempty"`
	ExpiresAt          *time.Time                             `json:"expires_at,omitempty"`
	Payment            *baofuSettlementAccountPaymentResponse `json:"payment,omitempty"`
	BankCardLast4      string                                 `json:"bank_card_last4,omitempty"`
	BankAccountNoMask  string                                 `json:"bank_account_no_mask,omitempty"`
	BankMobileMask     string                                 `json:"bank_mobile_mask,omitempty"`
	ContactMobileMask  string                                 `json:"contact_mobile_mask,omitempty"`
	EmailMask          string                                 `json:"email_mask,omitempty"`
	WechatSubMchIDMask string                                 `json:"wechat_sub_mch_id_mask,omitempty"`
	ProfileDefaults    *baofuSettlementAccountProfileDefaults `json:"profile_defaults,omitempty"`
	SubmittedAt        *time.Time                             `json:"submitted_at,omitempty"`
	UpdatedAt          *time.Time                             `json:"updated_at,omitempty"`
}
