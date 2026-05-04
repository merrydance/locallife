package contracts

import (
	"errors"
	"strings"
)

type OfficialBalanceQueryRequest struct {
	Version     string `json:"version"`
	ContractNo  string `json:"contractNo"`
	AccountType int    `json:"accType"`
}

func (r OfficialBalanceQueryRequest) Validate() error {
	if strings.TrimSpace(r.Version) != OfficialOpenAccountVersion {
		return errors.New("baofu balance query version must be 4.1.0")
	}
	if strings.TrimSpace(r.ContractNo) == "" {
		return errors.New("baofu balance query contractNo is required")
	}
	if r.AccountType != OfficialAccountTypePersonal && r.AccountType != OfficialAccountTypeBusiness {
		return errors.New("baofu balance query accType is unsupported")
	}
	return nil
}
