package contracts

import (
	"errors"
	"net/url"
	"strings"
)

const (
	OfficialBusinessTypeBCT20  = "BCT2.0"
	OfficialOpenAccountVersion = "4.1.0"

	OfficialAccountTypePersonal = 1
	OfficialAccountTypeBusiness = 2

	OfficialCertificateTypeID = "ID"
)

type OfficialOpenAccountRequest struct {
	Version      string `json:"version"`
	AccountType  int    `json:"accType"`
	AccountInfo  any    `json:"accInfo"`
	NoticeURL    string `json:"noticeUrl"`
	BusinessType string `json:"businessType"`
}

type OfficialPersonalAccountInfo struct {
	TransSerialNo   string `json:"transSerialNo"`
	LoginNo         string `json:"loginNo"`
	CustomerName    string `json:"customerName"`
	CertificateType string `json:"certificateType"`
	CertificateNo   string `json:"certificateNo"`
	CardNo          string `json:"cardNo"`
	MobileNo        string `json:"mobileNo"`
	CardUserName    string `json:"cardUserName"`
	NeedUploadFile  bool   `json:"needUploadFile"`
}

type OfficialPersonalTwoFactorAccountInfo struct {
	TransSerialNo   string `json:"transSerialNo"`
	LoginNo         string `json:"loginNo"`
	CustomerName    string `json:"customerName"`
	CertificateType string `json:"certificateType"`
	CertificateNo   string `json:"certificateNo"`
}

type OfficialBusinessAccountInfo struct {
	TransSerialNo       string `json:"transSerialNo"`
	LoginNo             string `json:"loginNo"`
	Email               string `json:"email"`
	SelfEmployed        bool   `json:"selfEmployed"`
	CustomerName        string `json:"customerName"`
	CertificateNo       string `json:"certificateNo"`
	CertificateType     string `json:"certificateType"`
	CorporateName       string `json:"corporateName"`
	CorporateCertType   string `json:"corporateCertType"`
	CorporateCertID     string `json:"corporateCertId"`
	IndustryID          string `json:"industryId"`
	CardNo              string `json:"cardNo"`
	BankName            string `json:"bankName"`
	DepositBankProvince string `json:"depositBankProvince"`
	DepositBankCity     string `json:"depositBankCity"`
	DepositBankName     string `json:"depositBankName"`
	CorporateMobile     string `json:"corporateMobile,omitempty"`
}

func (r OfficialOpenAccountRequest) Validate() error {
	if strings.TrimSpace(r.Version) != OfficialOpenAccountVersion {
		return errors.New("baofu open account version must be 4.1.0")
	}
	if r.AccountType != OfficialAccountTypePersonal && r.AccountType != OfficialAccountTypeBusiness {
		return errors.New("baofu open account accType is unsupported")
	}
	if err := validateOfficialNoticeURL(r.NoticeURL); err != nil {
		return err
	}
	if strings.TrimSpace(r.BusinessType) != OfficialBusinessTypeBCT20 {
		return errors.New("baofu open account businessType must be BCT2.0")
	}
	if r.AccountInfo == nil {
		return errors.New("baofu open account accInfo is required")
	}
	switch info := r.AccountInfo.(type) {
	case OfficialPersonalAccountInfo:
		return info.Validate()
	case *OfficialPersonalAccountInfo:
		if info == nil {
			return errors.New("baofu open account accInfo is required")
		}
		return info.Validate()
	case OfficialPersonalTwoFactorAccountInfo:
		return info.Validate()
	case *OfficialPersonalTwoFactorAccountInfo:
		if info == nil {
			return errors.New("baofu open account accInfo is required")
		}
		return info.Validate()
	case OfficialBusinessAccountInfo:
		return info.Validate()
	case *OfficialBusinessAccountInfo:
		if info == nil {
			return errors.New("baofu open account accInfo is required")
		}
		return info.Validate()
	default:
		return errors.New("baofu open account accInfo type is unsupported")
	}
}

func (i OfficialPersonalAccountInfo) Validate() error {
	if err := validateOfficialPersonalIdentity(i.TransSerialNo, i.LoginNo, i.CustomerName, i.CertificateType, i.CertificateNo); err != nil {
		return err
	}
	if strings.TrimSpace(i.CardNo) == "" {
		return errors.New("baofu open account personal cardNo is required")
	}
	if strings.TrimSpace(i.MobileNo) == "" {
		return errors.New("baofu open account personal mobileNo is required")
	}
	if strings.TrimSpace(i.CardUserName) == "" {
		return errors.New("baofu open account personal cardUserName is required")
	}
	return nil
}

func (i OfficialPersonalTwoFactorAccountInfo) Validate() error {
	return validateOfficialPersonalIdentity(i.TransSerialNo, i.LoginNo, i.CustomerName, i.CertificateType, i.CertificateNo)
}

func (i OfficialBusinessAccountInfo) Validate() error {
	for _, field := range []struct{ name, value string }{
		{"transSerialNo", i.TransSerialNo},
		{"loginNo", i.LoginNo},
		{"email", i.Email},
		{"customerName", i.CustomerName},
		{"certificateNo", i.CertificateNo},
		{"certificateType", i.CertificateType},
		{"corporateName", i.CorporateName},
		{"corporateCertType", i.CorporateCertType},
		{"corporateCertId", i.CorporateCertID},
		{"industryId", i.IndustryID},
		{"cardNo", i.CardNo},
		{"bankName", i.BankName},
		{"depositBankProvince", i.DepositBankProvince},
		{"depositBankCity", i.DepositBankCity},
		{"depositBankName", i.DepositBankName},
	} {
		if strings.TrimSpace(field.value) == "" {
			return errors.New("baofu open account business " + field.name + " is required")
		}
	}
	if err := validateOfficialLoginNo("business", i.LoginNo); err != nil {
		return err
	}
	if i.SelfEmployed && strings.TrimSpace(i.CorporateMobile) == "" {
		return errors.New("baofu open account business corporateMobile is required for selfEmployed")
	}
	return nil
}

func validateOfficialPersonalIdentity(transSerialNo, loginNo, customerName, certificateType, certificateNo string) error {
	for _, field := range []struct{ name, value string }{
		{"transSerialNo", transSerialNo},
		{"loginNo", loginNo},
		{"customerName", customerName},
		{"certificateNo", certificateNo},
	} {
		if strings.TrimSpace(field.value) == "" {
			return errors.New("baofu open account personal " + field.name + " is required")
		}
	}
	if err := validateOfficialLoginNo("personal", loginNo); err != nil {
		return err
	}
	if strings.TrimSpace(certificateType) != OfficialCertificateTypeID {
		return errors.New("baofu open account personal certificateType must be ID")
	}
	return nil
}

func validateOfficialLoginNo(owner string, loginNo string) error {
	if len(strings.TrimSpace(loginNo)) < 11 {
		return errors.New("baofu open account " + owner + " loginNo must be at least 11 characters")
	}
	return nil
}

func validateOfficialNoticeURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || strings.TrimSpace(parsed.Host) == "" {
		return errors.New("baofu open account noticeUrl must be https")
	}
	return nil
}
