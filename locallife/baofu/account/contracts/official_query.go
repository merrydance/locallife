package contracts

import (
	"errors"
	"strings"
)

const OfficialQueryAccountVersion = "4.0.0"

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
	if strings.TrimSpace(r.Version) != OfficialQueryAccountVersion {
		return errors.New("baofu query account version must be 4.0.0")
	}
	if r.AccountType != OfficialAccountTypePersonal && r.AccountType != OfficialAccountTypeBusiness {
		return errors.New("baofu query account accType is unsupported")
	}
	contractNo := strings.TrimSpace(r.ContractNo)
	loginNo := strings.TrimSpace(r.LoginNo)
	certificateNo := strings.TrimSpace(r.CertificateNo)
	certificateType := strings.TrimSpace(r.CertificateType)
	platformNo := strings.TrimSpace(r.PlatformNo)
	if contractNo != "" {
		if err := validateOfficialMaxLength("baofu query account", "contractNo", contractNo, 32); err != nil {
			return err
		}
		if loginNo != "" || certificateNo != "" || certificateType != "" || platformNo != "" {
			return errors.New("baofu query account contractNo cannot be combined with loginNo identity fields")
		}
		return nil
	}
	if loginNo == "" {
		return errors.New("baofu query account contractNo or loginNo is required")
	}
	if certificateNo == "" {
		return errors.New("baofu query account certificateNo is required when loginNo is used")
	}
	if certificateType == "" {
		return errors.New("baofu query account certificateType is required when loginNo is used")
	}
	if platformNo == "" {
		return errors.New("baofu query account platformNo is required when loginNo is used")
	}
	for _, field := range []struct {
		name  string
		value string
		max   int
	}{
		{"loginNo", loginNo, 128},
		{"certificateNo", certificateNo, 64},
		{"certificateType", certificateType, 16},
		{"platformNo", platformNo, 32},
	} {
		if err := validateOfficialMaxLength("baofu query account", field.name, field.value, field.max); err != nil {
			return err
		}
	}
	if certificateType != OfficialCertificateTypeID && certificateType != OfficialBusinessCertificateTypeLicense {
		return errors.New("baofu query account certificateType is unsupported")
	}
	return nil
}
