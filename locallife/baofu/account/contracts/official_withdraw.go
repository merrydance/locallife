package contracts

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

const OfficialWithdrawVersion = "4.2.0"

type OfficialWithdrawRequest struct {
	Version          string `json:"version"`
	ContractNo       string `json:"contractNo"`
	DirectPlatformNo string `json:"directPlatformNo,omitempty"`
	TransSerialNo    string `json:"transSerialNo"`
	DealAmount       string `json:"dealAmount"`
	ReturnURL        string `json:"returnUrl"`
	FeeMemberID      string `json:"feeMemberId,omitempty"`
	ReqReserved      string `json:"reqReserved,omitempty"`
	TransAbstract    string `json:"transAbstract,omitempty"`
}

type OfficialWithdrawQueryRequest struct {
	Version       string `json:"version"`
	TransSerialNo string `json:"transSerialNo"`
	TradeTime     string `json:"tradeTime"`
}

func (r OfficialWithdrawRequest) Validate() error {
	if strings.TrimSpace(r.Version) != OfficialWithdrawVersion {
		return errors.New("baofu withdraw version must be 4.2.0")
	}
	if strings.TrimSpace(r.ContractNo) == "" {
		return errors.New("baofu withdraw contractNo is required")
	}
	if strings.TrimSpace(r.TransSerialNo) == "" {
		return errors.New("baofu withdraw transSerialNo is required")
	}
	for _, field := range []struct {
		name  string
		value string
		max   int
	}{
		{"contractNo", r.ContractNo, 32},
		{"directPlatformNo", r.DirectPlatformNo, 32},
		{"transSerialNo", r.TransSerialNo, 50},
		{"returnUrl", r.ReturnURL, 256},
		{"feeMemberId", r.FeeMemberID, 32},
		{"reqReserved", r.ReqReserved, 512},
		{"transAbstract", r.TransAbstract, 255},
	} {
		if err := validateOfficialMaxLength("baofu withdraw", field.name, field.value, field.max); err != nil {
			return err
		}
	}
	amountFen, err := YuanStringToFen(r.DealAmount)
	if err != nil {
		return err
	}
	if amountFen <= 0 {
		return errors.New("baofu withdraw dealAmount must be positive")
	}
	parsed, err := url.Parse(strings.TrimSpace(r.ReturnURL))
	if err != nil || parsed.Scheme != "https" || strings.TrimSpace(parsed.Host) == "" {
		return errors.New("baofu withdraw returnUrl must be https")
	}
	return nil
}

func (r OfficialWithdrawQueryRequest) Validate() error {
	if strings.TrimSpace(r.Version) != OfficialWithdrawVersion {
		return errors.New("baofu withdraw query version must be 4.2.0")
	}
	if strings.TrimSpace(r.TransSerialNo) == "" {
		return errors.New("baofu withdraw query transSerialNo is required")
	}
	if err := validateOfficialMaxLength("baofu withdraw query", "transSerialNo", r.TransSerialNo, 50); err != nil {
		return err
	}
	if strings.TrimSpace(r.TradeTime) == "" {
		return errors.New("baofu withdraw query tradeTime is required")
	}
	if err := validateOfficialMaxLength("baofu withdraw query", "tradeTime", r.TradeTime, 10); err != nil {
		return err
	}
	if _, err := time.Parse("2006-01-02", strings.TrimSpace(r.TradeTime)); err != nil {
		return errors.New("baofu withdraw query tradeTime must use yyyy-MM-dd")
	}
	return nil
}
