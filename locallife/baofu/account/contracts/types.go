package contracts

import (
	"encoding/json"
	"errors"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	OpenStateProcessing = "processing"
	OpenStateActive     = "active"
	OpenStateFailed     = "failed"
	OpenStateAbnormal   = "abnormal"
)

type OwnerType string

type OpenAccountRequest struct {
	OwnerType                  string
	OwnerID                    int64
	AccountType                string
	OutRequestNo               string
	LoginNo                    string
	LegalName                  string
	CertificateNo              string
	CertificateType            string
	BankAccountNo              string
	BankMobile                 string
	Email                      string
	SelfEmployed               bool
	CustomerName               string
	AliasName                  string
	CorporateName              string
	CorporateCertType          string
	CorporateCertID            string
	CorporateMobile            string
	IndustryID                 string
	ContactName                string
	ContactMobile              string
	BankName                   string
	DepositBankProvince        string
	DepositBankCity            string
	DepositBankName            string
	RegisterCapital            string
	CardUserName               string
	PlatformNo                 string
	PlatformTerminalID         string
	QualificationTransSerialNo string
}

func (r OpenAccountRequest) Validate() error {
	if strings.TrimSpace(r.OutRequestNo) == "" {
		return errors.New("baofu open account outRequestNo is required")
	}
	switch normalizedAccountType(r.AccountType) {
	case db.BaofuAccountTypePersonal:
		return r.validatePersonalFourFactor()
	case db.BaofuAccountTypeBusiness:
		return r.validateBusiness()
	default:
		return errors.New("baofu open account accountType is unsupported")
	}
}

func (r OpenAccountRequest) validatePersonalFourFactor() error {
	for _, field := range []struct{ name, value string }{
		{"legalName", r.LegalName},
		{"certificateNo", r.CertificateNo},
		{"bankAccountNo", r.BankAccountNo},
		{"bankMobile", r.BankMobile},
	} {
		if strings.TrimSpace(field.value) == "" {
			return errors.New("baofu open account personal " + field.name + " is required")
		}
	}
	return nil
}

func (r OpenAccountRequest) validateBusiness() error {
	customerName := firstTrimmed(r.CustomerName, r.LegalName)
	certificateType := firstTrimmed(r.CertificateType, OfficialBusinessCertificateTypeLicense)
	for _, field := range []struct{ name, value string }{
		{"email", r.Email},
		{"customerName", customerName},
		{"certificateNo", r.CertificateNo},
		{"corporateName", r.CorporateName},
		{"corporateCertType", r.CorporateCertType},
		{"corporateCertId", r.CorporateCertID},
		{"industryId", r.IndustryID},
		{"bankAccountNo", r.BankAccountNo},
		{"bankName", r.BankName},
		{"depositBankProvince", r.DepositBankProvince},
		{"depositBankCity", r.DepositBankCity},
		{"depositBankName", r.DepositBankName},
	} {
		if strings.TrimSpace(field.value) == "" {
			return errors.New("baofu open account business " + field.name + " is required")
		}
	}
	if certificateType != OfficialBusinessCertificateTypeLicense {
		return errors.New("baofu open account business certificateType must be LICENSE")
	}
	if !isOfficialCorporateCertificateType(r.CorporateCertType) {
		return errors.New("baofu open account business corporateCertType is unsupported")
	}
	if r.SelfEmployed && strings.TrimSpace(r.CardUserName) != "" && strings.TrimSpace(r.CorporateMobile) == "" {
		return errors.New("baofu open account business corporateMobile is required for selfEmployed private card")
	}
	return nil
}

func normalizedAccountType(accountType string) string {
	switch strings.ToLower(strings.TrimSpace(accountType)) {
	case db.BaofuAccountTypePersonal:
		return db.BaofuAccountTypePersonal
	case db.BaofuAccountTypeBusiness:
		return db.BaofuAccountTypeBusiness
	default:
		return strings.ToLower(strings.TrimSpace(accountType))
	}
}

func firstTrimmed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

type QueryAccountRequest struct {
	OutRequestNo    string
	LoginNo         string
	ContractNo      string
	AccountType     string
	CertificateNo   string
	CertificateType string
	PlatformNo      string
}

type BalanceQueryRequest struct {
	MerchantID  string
	TerminalID  string
	ContractNo  string
	AccountType string
}

type BalanceResult struct {
	ContractNo         string
	AvailableAmountFen int64
	PendingAmountFen   int64
	LedgerAmountFen    int64
	FrozenAmountFen    int64
	UpstreamAvailable  string
	UpstreamPending    string
	UpstreamLedger     string
	UpstreamFrozen     string
	Raw                json.RawMessage
}

type WithdrawRequest struct {
	MerchantID    string
	TerminalID    string
	ContractNo    string
	TransSerialNo string
	AmountFen     int64
	NotifyURL     string
}

type WithdrawQueryRequest struct {
	MerchantID    string
	TerminalID    string
	TransSerialNo string
	TradeTime     string
}

type WithdrawResult struct {
	TransSerialNo   string
	BaofuWithdrawNo string
	ContractNo      string
	UpstreamState   string
	Status          string
	AmountFen       int64
	FeeFen          int64
	TotalAmountFen  int64
	Remark          string
	Raw             json.RawMessage
}

type AccountResult struct {
	OutRequestNo  string
	ContractNo    string
	SharingMerID  string
	OpenState     string
	UpstreamState string
	FailCode      string
	FailMessage   string
	Raw           json.RawMessage
}

func (r AccountResult) Normalized() AccountResult {
	r.OutRequestNo = strings.TrimSpace(r.OutRequestNo)
	r.ContractNo = strings.TrimSpace(r.ContractNo)
	r.SharingMerID = strings.TrimSpace(r.SharingMerID)
	r.OpenState = strings.TrimSpace(r.OpenState)
	if r.OpenState == "" {
		r.OpenState = OpenStateFromUpstream(r.UpstreamState)
	}
	r.UpstreamState = strings.TrimSpace(r.UpstreamState)
	r.FailCode = strings.TrimSpace(r.FailCode)
	r.FailMessage = strings.TrimSpace(r.FailMessage)
	return r
}

func OpenStateFromUpstream(state string) string {
	switch strings.TrimSpace(state) {
	case "1":
		return OpenStateActive
	case "0":
		return OpenStateFailed
	case "2":
		return OpenStateProcessing
	case "-1":
		return OpenStateAbnormal
	default:
		return OpenStateAbnormal
	}
}

func WithdrawStatusFromUpstream(state string) string {
	switch strings.TrimSpace(state) {
	case "1":
		return db.BaofuWithdrawalStatusSucceeded
	case "0":
		return db.BaofuWithdrawalStatusFailed
	case "2":
		return db.BaofuWithdrawalStatusProcessing
	case "3":
		return db.BaofuWithdrawalStatusReturned
	default:
		return db.BaofuWithdrawalStatusProcessing
	}
}

func WithdrawAcceptanceStatusFromUpstream(state string) string {
	switch strings.TrimSpace(state) {
	case "2":
		return db.BaofuWithdrawalStatusFailed
	default:
		return db.BaofuWithdrawalStatusProcessing
	}
}
