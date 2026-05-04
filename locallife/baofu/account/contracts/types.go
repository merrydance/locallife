package contracts

import (
	"encoding/json"
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
	OwnerType     string
	OwnerID       int64
	AccountType   string
	OutRequestNo  string
	LegalName     string
	CertificateNo string
	BankAccountNo string
	BankMobile    string
}

type QueryAccountRequest struct {
	OutRequestNo string
	ContractNo   string
	AccountType  string
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
}

type WithdrawResult struct {
	TransSerialNo   string
	BaofuWithdrawNo string
	ContractNo      string
	UpstreamState   string
	Status          string
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
