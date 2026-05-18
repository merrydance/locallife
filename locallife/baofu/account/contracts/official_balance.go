package contracts

import (
	"errors"
	"strings"
)

const OfficialBalanceVersion = "4.0.0"

type OfficialBalanceQueryRequest struct {
	Version     string `json:"version"`
	ContractNo  string `json:"contractNo"`
	AccountType int    `json:"accType"`
}

func (r OfficialBalanceQueryRequest) Validate() error {
	if strings.TrimSpace(r.Version) != OfficialBalanceVersion {
		return errors.New("baofu balance query version must be 4.0.0")
	}
	if strings.TrimSpace(r.ContractNo) == "" {
		return errors.New("baofu balance query contractNo is required")
	}
	if err := validateOfficialMaxLength("baofu balance query", "contractNo", r.ContractNo, 32); err != nil {
		return err
	}
	if r.AccountType != OfficialAccountTypePersonal && r.AccountType != OfficialAccountTypeBusiness {
		return errors.New("baofu balance query accType is unsupported")
	}
	return nil
}
