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
	queryKeyCount := 0
	for _, value := range []string{r.ContractNo, r.LoginNo, r.CertificateNo} {
		if strings.TrimSpace(value) != "" {
			queryKeyCount++
		}
	}
	if queryKeyCount == 0 {
		return errors.New("baofu query account contractNo, loginNo, or certificateNo is required")
	}
	if queryKeyCount > 1 {
		return errors.New("baofu query account must use exactly one query key")
	}
	if strings.TrimSpace(r.CertificateNo) != "" && strings.TrimSpace(r.CertificateType) == "" {
		return errors.New("baofu query account certificateType is required when certificateNo is used")
	}
	return nil
}
