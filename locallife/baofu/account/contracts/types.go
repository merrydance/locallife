package contracts

import (
	"encoding/json"
	"strings"
)

const (
	OpenStateProcessing = "processing"
	OpenStateActive     = "active"
	OpenStateFailed     = "failed"
	OpenStateAbnormal   = "abnormal"
)

type OwnerType string

type OpenAccountRequest struct {
	OwnerType      string
	OwnerID        int64
	AccountType    string
	OutRequestNo   string
	LegalName      string
	CertificateNo  string
	BankAccountNo  string
	BankMobile     string
	WechatSubMchID string
}

type QueryAccountRequest struct {
	OutRequestNo string
	ContractNo   string
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
	if r.SharingMerID == "" {
		r.SharingMerID = r.ContractNo
	}
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
