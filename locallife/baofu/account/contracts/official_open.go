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

	OfficialCertificateTypeID                  = "ID"
	OfficialBusinessCertificateTypeLicense     = "LICENSE"
	OfficialCorporateCertTypeHongKongMacaoPass = "HONG_KONG_AND_MACAO_PASS"
	OfficialCorporateCertTypeTaiwanPermit      = "TAIWAN_TRAVEL_PERMIT"
	OfficialCorporateCertTypePassport          = "PASSPORT"
)

type OfficialOpenAccountRequest struct {
	Version      string `json:"version"`
	AccountType  int    `json:"accType"`
	AccountInfo  any    `json:"accInfo"`
	NoticeURL    string `json:"noticeUrl"`
	BusinessType string `json:"businessType"`
}

type OfficialPersonalAccountInfo struct {
	TransSerialNo              string `json:"transSerialNo"`
	LoginNo                    string `json:"loginNo"`
	CustomerName               string `json:"customerName"`
	CertificateType            string `json:"certificateType"`
	CertificateNo              string `json:"certificateNo"`
	CardNo                     string `json:"cardNo"`
	MobileNo                   string `json:"mobileNo"`
	CardUserName               string `json:"cardUserName"`
	NeedUploadFile             bool   `json:"needUploadFile"`
	PlatformNo                 string `json:"platformNo,omitempty"`
	PlatformTerminalID         string `json:"platformTerminalId,omitempty"`
	QualificationTransSerialNo string `json:"qualificationTransSerialNo,omitempty"`
}

type OfficialPersonalTwoFactorAccountInfo struct {
	TransSerialNo              string `json:"transSerialNo"`
	LoginNo                    string `json:"loginNo"`
	CustomerName               string `json:"customerName"`
	CertificateType            string `json:"certificateType"`
	CertificateNo              string `json:"certificateNo"`
	CardUserName               string `json:"cardUserName"`
	NeedUploadFile             bool   `json:"needUploadFile"`
	PlatformNo                 string `json:"platformNo,omitempty"`
	PlatformTerminalID         string `json:"platformTerminalId,omitempty"`
	QualificationTransSerialNo string `json:"qualificationTransSerialNo,omitempty"`
}

type OfficialBusinessAccountInfo struct {
	TransSerialNo              string `json:"transSerialNo"`
	LoginNo                    string `json:"loginNo"`
	Email                      string `json:"email"`
	SelfEmployed               bool   `json:"selfEmployed"`
	CustomerName               string `json:"customerName"`
	AliasName                  string `json:"aliasName,omitempty"`
	CertificateNo              string `json:"certificateNo"`
	CertificateType            string `json:"certificateType"`
	CorporateName              string `json:"corporateName"`
	CorporateCertType          string `json:"corporateCertType"`
	CorporateCertID            string `json:"corporateCertId"`
	CorporateMobile            string `json:"corporateMobile,omitempty"`
	IndustryID                 string `json:"industryId"`
	ContactName                string `json:"contactName,omitempty"`
	ContactMobile              string `json:"contactMobile,omitempty"`
	CardNo                     string `json:"cardNo"`
	BankName                   string `json:"bankName"`
	DepositBankProvince        string `json:"depositBankProvince"`
	DepositBankCity            string `json:"depositBankCity"`
	DepositBankName            string `json:"depositBankName"`
	RegisterCapital            string `json:"registerCapital,omitempty"`
	CardUserName               string `json:"cardUserName,omitempty"`
	PlatformNo                 string `json:"platformNo,omitempty"`
	PlatformTerminalID         string `json:"platformTerminalId,omitempty"`
	QualificationTransSerialNo string `json:"qualificationTransSerialNo,omitempty"`
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
	if err := validateOfficialMaxLength("baofu open account", "noticeUrl", r.NoticeURL, 256); err != nil {
		return err
	}
	if strings.TrimSpace(r.BusinessType) != OfficialBusinessTypeBCT20 {
		return errors.New("baofu open account businessType must be BCT2.0")
	}
	if err := validateOfficialMaxLength("baofu open account", "businessType", r.BusinessType, 32); err != nil {
		return err
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
		return errors.New("baofu open account personal two-factor is not supported")
	case *OfficialPersonalTwoFactorAccountInfo:
		if info == nil {
			return errors.New("baofu open account accInfo is required")
		}
		return errors.New("baofu open account personal two-factor is not supported")
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
	for _, field := range []struct {
		name  string
		value string
		max   int
	}{
		{"cardNo", i.CardNo, 128},
		{"mobileNo", i.MobileNo, 64},
		{"cardUserName", i.CardUserName, 20},
		{"platformNo", i.PlatformNo, 32},
		{"platformTerminalId", i.PlatformTerminalID, 32},
		{"qualificationTransSerialNo", i.QualificationTransSerialNo, 128},
	} {
		if err := validateOfficialMaxLength("baofu open account personal", field.name, field.value, field.max); err != nil {
			return err
		}
	}
	return nil
}

func (i OfficialPersonalTwoFactorAccountInfo) Validate() error {
	if err := validateOfficialPersonalIdentity(i.TransSerialNo, i.LoginNo, i.CustomerName, i.CertificateType, i.CertificateNo); err != nil {
		return err
	}
	if strings.TrimSpace(i.CardUserName) == "" {
		return errors.New("baofu open account personal cardUserName is required")
	}
	for _, field := range []struct {
		name  string
		value string
		max   int
	}{
		{"cardUserName", i.CardUserName, 20},
		{"platformNo", i.PlatformNo, 32},
		{"platformTerminalId", i.PlatformTerminalID, 32},
		{"qualificationTransSerialNo", i.QualificationTransSerialNo, 128},
	} {
		if err := validateOfficialMaxLength("baofu open account personal", field.name, field.value, field.max); err != nil {
			return err
		}
	}
	return nil
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
	for _, field := range []struct {
		name  string
		value string
		max   int
	}{
		{"transSerialNo", i.TransSerialNo, 200},
		{"loginNo", i.LoginNo, 32},
		{"email", i.Email, 32},
		{"customerName", i.CustomerName, 64},
		{"aliasName", i.AliasName, 64},
		{"certificateNo", i.CertificateNo, 64},
		{"certificateType", i.CertificateType, 16},
		{"corporateName", i.CorporateName, 20},
		{"corporateCertId", i.CorporateCertID, 64},
		{"corporateMobile", i.CorporateMobile, 64},
		{"industryId", i.IndustryID, 11},
		{"contactName", i.ContactName, 20},
		{"contactMobile", i.ContactMobile, 64},
		{"cardNo", i.CardNo, 128},
		{"bankName", i.BankName, 20},
		{"depositBankProvince", i.DepositBankProvince, 20},
		{"depositBankCity", i.DepositBankCity, 20},
		{"depositBankName", i.DepositBankName, 64},
		{"registerCapital", i.RegisterCapital, 64},
		{"cardUserName", i.CardUserName, 64},
		{"platformNo", i.PlatformNo, 32},
		{"platformTerminalId", i.PlatformTerminalID, 32},
		{"qualificationTransSerialNo", i.QualificationTransSerialNo, 128},
	} {
		if err := validateOfficialMaxLength("baofu open account business", field.name, field.value, field.max); err != nil {
			return err
		}
	}
	if strings.TrimSpace(i.CertificateType) != OfficialBusinessCertificateTypeLicense {
		return errors.New("baofu open account business certificateType must be LICENSE")
	}
	if !isOfficialCorporateCertificateType(i.CorporateCertType) {
		return errors.New("baofu open account business corporateCertType is unsupported")
	}
	if i.SelfEmployed && strings.TrimSpace(i.CardUserName) != "" && strings.TrimSpace(i.CorporateMobile) == "" {
		return errors.New("baofu open account business corporateMobile is required for selfEmployed private card")
	}
	return nil
}

func isOfficialCorporateCertificateType(value string) bool {
	switch strings.TrimSpace(value) {
	case OfficialCertificateTypeID,
		OfficialCorporateCertTypeHongKongMacaoPass,
		OfficialCorporateCertTypeTaiwanPermit,
		OfficialCorporateCertTypePassport:
		return true
	default:
		return false
	}
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
	for _, field := range []struct {
		name  string
		value string
		max   int
	}{
		{"transSerialNo", transSerialNo, 200},
		{"loginNo", loginNo, 32},
		{"customerName", customerName, 32},
		{"certificateType", certificateType, 16},
		{"certificateNo", certificateNo, 64},
	} {
		if err := validateOfficialMaxLength("baofu open account personal", field.name, field.value, field.max); err != nil {
			return err
		}
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
