package contracts

import (
	"errors"
	"strings"
)

type OfficialQueryAccountRequest struct {
	Version         string `json:"version"`
	AccountType     int    `json:"accType"`
	CertificateNo   string `json:"certificateNo,omitempty"`
	CertificateType string `json:"certificateType,omitempty"`
	PlatformNo      string `json:"platformNo,omitempty"`
	LoginNo         string `json:"loginNo,omitempty"`
	ContractNo      string `json:"contractNo,omitempty"`
}

func (r OfficialQueryAccountRequest) Validate() error {
	if strings.TrimSpace(r.Version) != OfficialOpenAccountVersion {
		return errors.New("baofu query account version must be 4.1.0")
	}
	if r.AccountType != OfficialAccountTypePersonal && r.AccountType != OfficialAccountTypeBusiness {
		return errors.New("baofu query account accType is unsupported")
	}
	if strings.TrimSpace(r.ContractNo) == "" && strings.TrimSpace(r.LoginNo) == "" && strings.TrimSpace(r.CertificateNo) == "" {
		return errors.New("baofu query account contractNo, loginNo, or certificateNo is required")
	}
	return nil
}
