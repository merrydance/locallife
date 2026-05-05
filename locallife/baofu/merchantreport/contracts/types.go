package contracts

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

type WechatMerchantReportRequest struct {
	AgentMerchantID string           `json:"agentMerId,omitempty"`
	AgentTerminalID string           `json:"agentTerId,omitempty"`
	MerchantID      string           `json:"merId"`
	TerminalID      string           `json:"terId"`
	ReportType      string           `json:"reportType"`
	ReportNo        string           `json:"reportNo"`
	BCTMerchantID   string           `json:"bctMerId"`
	ReportInfo      WechatReportInfo `json:"reportInfo"`
}

type WechatReportInfo struct {
	MerchantName        string             `json:"merchant_name"`
	MerchantShortName   string             `json:"merchant_shortname"`
	ServicePhone        string             `json:"service_phone"`
	Contact             string             `json:"contact,omitempty"`
	ContactPhone        string             `json:"contact_phone,omitempty"`
	ContactEmail        string             `json:"contact_email,omitempty"`
	ChannelID           string             `json:"channel_id"`
	ChannelName         string             `json:"channel_name"`
	Business            string             `json:"business"`
	ServiceCodes        []string           `json:"service_codes"`
	AddressInfo         WechatAddressInfo  `json:"address_info"`
	BusinessLicenseType string             `json:"business_license_type"`
	BusinessLicense     string             `json:"business_license"`
	BankCardInfo        WechatBankCardInfo `json:"bankcard_info"`
}

type WechatAddressInfo struct {
	ProvinceCode string `json:"province_code"`
	CityCode     string `json:"city_code"`
	DistrictCode string `json:"district_code"`
	Address      string `json:"address"`
	Longitude    string `json:"longitude,omitempty"`
	Latitude     string `json:"latitude,omitempty"`
	Type         string `json:"type,omitempty"`
}

type WechatBankCardInfo struct {
	CardName       string `json:"card_name"`
	CardNo         string `json:"card_no"`
	BankBranchName string `json:"bank_branch_name,omitempty"`
}

func WechatMiniProgramPaymentServiceCodes() []string {
	return []string{WechatServiceTypeJSAPI, WechatServiceTypeApplet}
}

type MerchantReportQueryRequest struct {
	AgentMerchantID string `json:"agentMerId,omitempty"`
	AgentTerminalID string `json:"agentTerId,omitempty"`
	MerchantID      string `json:"merId"`
	TerminalID      string `json:"terId"`
	ReportType      string `json:"reportType"`
	ReportNo        string `json:"reportNo"`
}

type MerchantReportResult struct {
	MerchantID         string          `json:"merId,omitempty"`
	TerminalID         string          `json:"terId,omitempty"`
	ReportType         string          `json:"reportType,omitempty"`
	ReportNo           string          `json:"reportNo,omitempty"`
	ReportState        string          `json:"reportState,omitempty"`
	SubMchID           string          `json:"subMchId,omitempty"`
	PlatformBizNo      string          `json:"platformBizNo,omitempty"`
	ResultCode         string          `json:"resultCode,omitempty"`
	ErrorCode          string          `json:"errCode,omitempty"`
	ErrorMessage       string          `json:"errMsg,omitempty"`
	ChannelReturnParam json.RawMessage `json:"channelRetParam,omitempty"`
	Raw                json.RawMessage `json:"-"`
}

func (r MerchantReportResult) NormalizedReportState() string {
	return NormalizeMerchantReportState(r.ReportState)
}

func (r MerchantReportResult) Normalized() MerchantReportResult {
	r.SubMchID = strings.TrimSpace(firstNonEmpty(r.SubMchID, subMchIDFromChannelReturnParam(r.ChannelReturnParam)))
	return r
}

func (r MerchantReportResult) ValidateMerchantReportResponse() error {
	return r.validateMerchantReportResponse()
}

func (r MerchantReportResult) ValidateMerchantReportQueryResponse() error {
	return r.validateMerchantReportResponse()
}

func (r MerchantReportResult) validateMerchantReportResponse() error {
	if strings.TrimSpace(r.MerchantID) == "" {
		return errors.New("baofu merchant report response merId is required")
	}
	if strings.TrimSpace(r.TerminalID) == "" {
		return errors.New("baofu merchant report response terId is required")
	}
	if strings.TrimSpace(r.ReportType) == "" {
		return errors.New("baofu merchant report response reportType is required")
	}
	if !IsValidReportType(r.ReportType) {
		return errors.New("baofu merchant report response reportType is unsupported")
	}
	if strings.TrimSpace(r.ReportNo) == "" {
		return errors.New("baofu merchant report response reportNo is required")
	}
	return validateBusinessResultCode("baofu merchant report response", r.ResultCode)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func subMchIDFromChannelReturnParam(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		var encoded string
		if json.Unmarshal(raw, &encoded) != nil {
			return ""
		}
		if json.Unmarshal([]byte(encoded), &payload) != nil {
			return ""
		}
	}
	for _, key := range []string{"sub_mch_id", "subMchId"} {
		if value, ok := payload[key]; ok {
			return strings.TrimSpace(toString(value))
		}
	}
	return ""
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

type BindSubConfigRequest struct {
	AgentMerchantID string `json:"agentMerId,omitempty"`
	AgentTerminalID string `json:"agentTerId,omitempty"`
	MerchantID      string `json:"merId"`
	TerminalID      string `json:"terId"`
	SubMchID        string `json:"subMchId"`
	AuthType        string `json:"authType"`
	AuthContent     string `json:"authContent"`
	Remark          string `json:"remark"`
}

type BindSubConfigResult struct {
	MerchantID   string          `json:"merId,omitempty"`
	TerminalID   string          `json:"terId,omitempty"`
	SubMchID     string          `json:"subMchId,omitempty"`
	AuthType     string          `json:"authType,omitempty"`
	AuthContent  string          `json:"authContent,omitempty"`
	ResultCode   string          `json:"resultCode,omitempty"`
	ErrorCode    string          `json:"errCode,omitempty"`
	ErrorMessage string          `json:"errMsg,omitempty"`
	Raw          json.RawMessage `json:"-"`
}

func (r BindSubConfigResult) ValidateBindSubConfigResponse() error {
	if strings.TrimSpace(r.MerchantID) == "" {
		return errors.New("baofu bind_sub_config response merId is required")
	}
	if strings.TrimSpace(r.TerminalID) == "" {
		return errors.New("baofu bind_sub_config response terId is required")
	}
	return validateBusinessResultCode("baofu bind_sub_config response", r.ResultCode)
}

func (r WechatMerchantReportRequest) Validate() error {
	if err := validateMerchantTerminal(r.MerchantID, r.TerminalID, "baofu merchant report"); err != nil {
		return err
	}
	if !IsValidReportType(r.ReportType) {
		return errors.New("baofu merchant report reportType is unsupported")
	}
	if strings.ToUpper(strings.TrimSpace(r.ReportType)) != ReportTypeWechat {
		return errors.New("baofu merchant report reportType must be WECHAT")
	}
	if strings.TrimSpace(r.ReportNo) == "" {
		return errors.New("baofu merchant report reportNo is required")
	}
	if strings.TrimSpace(r.BCTMerchantID) == "" {
		return errors.New("baofu merchant report bctMerId is required")
	}
	return r.ReportInfo.Validate()
}

func (i WechatReportInfo) Validate() error {
	for _, field := range []struct{ name, value string }{
		{"merchant_name", i.MerchantName},
		{"merchant_shortname", i.MerchantShortName},
		{"service_phone", i.ServicePhone},
		{"channel_id", i.ChannelID},
		{"channel_name", i.ChannelName},
		{"business", i.Business},
		{"business_license_type", i.BusinessLicenseType},
		{"business_license", i.BusinessLicense},
	} {
		if strings.TrimSpace(field.value) == "" {
			return errors.New("baofu merchant report wechat " + field.name + " is required")
		}
	}
	if !IsValidWechatCategory(i.Business) {
		return errors.New("baofu merchant report wechat business is unsupported")
	}
	if !IsValidWechatCertificateType(i.BusinessLicenseType) {
		return errors.New("baofu merchant report wechat business_license_type is unsupported")
	}
	if len(i.ServiceCodes) == 0 {
		return errors.New("baofu merchant report wechat service_codes are required")
	}
	for _, serviceCode := range i.ServiceCodes {
		if !IsValidWechatServiceType(serviceCode) {
			return errors.New("baofu merchant report wechat service_codes contains unsupported value")
		}
	}
	if err := i.AddressInfo.Validate(); err != nil {
		return err
	}
	return i.BankCardInfo.Validate()
}

func (i WechatAddressInfo) Validate() error {
	for _, field := range []struct{ name, value string }{
		{"province_code", i.ProvinceCode},
		{"city_code", i.CityCode},
		{"district_code", i.DistrictCode},
		{"address", i.Address},
	} {
		if strings.TrimSpace(field.value) == "" {
			return errors.New("baofu merchant report wechat address_info." + field.name + " is required")
		}
	}
	return nil
}

func (i WechatBankCardInfo) Validate() error {
	for _, field := range []struct{ name, value string }{
		{"card_name", i.CardName},
		{"card_no", i.CardNo},
	} {
		if strings.TrimSpace(field.value) == "" {
			return errors.New("baofu merchant report wechat bankcard_info." + field.name + " is required")
		}
	}
	return nil
}

func (r MerchantReportQueryRequest) Validate() error {
	if err := validateMerchantTerminal(r.MerchantID, r.TerminalID, "baofu merchant report query"); err != nil {
		return err
	}
	if !IsValidReportType(r.ReportType) {
		return errors.New("baofu merchant report query reportType is unsupported")
	}
	if strings.TrimSpace(r.ReportNo) == "" {
		return errors.New("baofu merchant report query reportNo is required")
	}
	return nil
}

func (r BindSubConfigRequest) Validate() error {
	if err := validateMerchantTerminal(r.MerchantID, r.TerminalID, "baofu bind_sub_config"); err != nil {
		return err
	}
	if strings.TrimSpace(r.SubMchID) == "" {
		return errors.New("baofu bind_sub_config subMchId is required")
	}
	if !IsValidAuthType(r.AuthType) {
		return errors.New("baofu bind_sub_config authType is unsupported")
	}
	if strings.TrimSpace(r.AuthContent) == "" {
		if strings.ToUpper(strings.TrimSpace(r.AuthType)) == AuthTypeApplet {
			return errors.New("baofu bind_sub_config authContent is required for APPLET")
		}
		return errors.New("baofu bind_sub_config authContent is required")
	}
	if strings.TrimSpace(r.Remark) == "" {
		return errors.New("baofu bind_sub_config remark is required")
	}
	return nil
}

func validateMerchantTerminal(merchantID, terminalID, prefix string) error {
	if strings.TrimSpace(merchantID) == "" {
		return errors.New(prefix + " merId is required")
	}
	if strings.TrimSpace(terminalID) == "" {
		return errors.New(prefix + " terId is required")
	}
	return nil
}

func validateBusinessResultCode(prefix string, resultCode string) error {
	switch strings.ToUpper(strings.TrimSpace(resultCode)) {
	case "SUCCESS", "FAIL":
		return nil
	case "":
		return errors.New(prefix + " resultCode is required")
	default:
		return errors.New(prefix + " resultCode is unsupported")
	}
}
