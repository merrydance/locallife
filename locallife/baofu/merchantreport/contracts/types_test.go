package contracts

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWechatMerchantReportRequiresMerchantBCTMerID(t *testing.T) {
	req := WechatMerchantReportRequest{
		MerchantID:    "100000",
		TerminalID:    "200000",
		ReportType:    ReportTypeWechat,
		ReportNo:      "MR202605040001",
		BCTMerchantID: "CM202605040001",
		ReportInfo: WechatReportInfo{
			MerchantName:        "上海某某餐饮有限公司",
			MerchantShortName:   "某某餐饮",
			ServicePhone:        "02112345678",
			ChannelID:           "channel-001",
			ChannelName:         "乐客来福",
			Business:            "758-2",
			ServiceCodes:        []string{WechatServiceTypeApplet},
			AddressInfo:         WechatAddressInfo{ProvinceCode: "310000", CityCode: "310100", DistrictCode: "310115", Address: "世纪大道 1 号"},
			BusinessLicenseType: WechatCertificateTypeNationalLegalMerge,
			BusinessLicense:     "91310000123456789X",
			BankCardInfo:        WechatBankCardInfo{CardName: "上海某某餐饮有限公司", CardNo: "6222000000000000000", BankBranchName: "招商银行上海分行"},
		},
	}
	require.NoError(t, req.Validate())

	req.BCTMerchantID = ""
	require.EqualError(t, req.Validate(), "baofu merchant report bctMerId is required")
}

func TestWechatMerchantReportSerializesOfficialFieldNames(t *testing.T) {
	req := WechatMerchantReportRequest{
		MerchantID:    "100000",
		TerminalID:    "200000",
		ReportType:    ReportTypeWechat,
		ReportNo:      "MR202605040002",
		BCTMerchantID: "CM202605040002",
		ReportInfo: WechatReportInfo{
			MerchantName:        "上海某某餐饮有限公司",
			MerchantShortName:   "某某餐饮",
			ServicePhone:        "02112345678",
			ChannelID:           "channel-001",
			ChannelName:         "乐客来福",
			Business:            "758-2",
			ServiceCodes:        []string{WechatServiceTypeApplet},
			AddressInfo:         WechatAddressInfo{ProvinceCode: "310000", CityCode: "310100", DistrictCode: "310115", Address: "世纪大道 1 号"},
			BusinessLicenseType: WechatCertificateTypeNationalLegalMerge,
			BusinessLicense:     "91310000123456789X",
			BankCardInfo:        WechatBankCardInfo{CardName: "上海某某餐饮有限公司", CardNo: "6222000000000000000", BankBranchName: "招商银行上海分行"},
		},
	}

	body, err := json.Marshal(req)

	require.NoError(t, err)
	require.Contains(t, string(body), `"reportType":"WECHAT"`)
	require.Contains(t, string(body), `"bctMerId":"CM202605040002"`)
	require.Contains(t, string(body), `"merchant_name":"上海某某餐饮有限公司"`)
	require.Contains(t, string(body), `"merchant_shortname":"某某餐饮"`)
	require.Contains(t, string(body), `"service_codes":["APPLET"]`)
	require.Contains(t, string(body), `"province_code":"310000"`)
	require.Contains(t, string(body), `"city_code":"310100"`)
	require.Contains(t, string(body), `"district_code":"310115"`)
	require.Contains(t, string(body), `"card_name":"上海某某餐饮有限公司"`)
	require.Contains(t, string(body), `"card_no":"6222000000000000000"`)
	require.NotContains(t, string(body), `"province"`)
	require.NotContains(t, string(body), `"account_name"`)
	require.NotContains(t, string(body), `"account_no"`)
	require.NotContains(t, string(body), "sharingMerId")
}

func TestWechatMerchantReportRejectsUnsupportedWechatAppendixValues(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*WechatMerchantReportRequest)
		want   string
	}{
		{"unsupported category", func(r *WechatMerchantReportRequest) { r.ReportInfo.Business = "INVALID_CATEGORY" }, "baofu merchant report wechat business is unsupported"},
		{"unsupported certificate", func(r *WechatMerchantReportRequest) { r.ReportInfo.BusinessLicenseType = "PASSPORT" }, "baofu merchant report wechat business_license_type is unsupported"},
		{"missing service codes", func(r *WechatMerchantReportRequest) { r.ReportInfo.ServiceCodes = nil }, "baofu merchant report wechat service_codes are required"},
		{"unsupported service code", func(r *WechatMerchantReportRequest) { r.ReportInfo.ServiceCodes = []string{"NATIVE"} }, "baofu merchant report wechat service_codes contains unsupported value"},
		{"missing address", func(r *WechatMerchantReportRequest) { r.ReportInfo.AddressInfo.Address = "" }, "baofu merchant report wechat address_info.address is required"},
		{"missing card no", func(r *WechatMerchantReportRequest) { r.ReportInfo.BankCardInfo.CardNo = "" }, "baofu merchant report wechat bankcard_info.card_no is required"},
		{"missing card name", func(r *WechatMerchantReportRequest) { r.ReportInfo.BankCardInfo.CardName = "" }, "baofu merchant report wechat bankcard_info.card_name is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validWechatMerchantReportRequestForTest()
			tc.mutate(&req)
			require.EqualError(t, req.Validate(), tc.want)
		})
	}
}

func TestWechatMerchantReportBankBranchIsOptional(t *testing.T) {
	req := validWechatMerchantReportRequestForTest()
	req.ReportInfo.BankCardInfo.BankBranchName = ""
	require.NoError(t, req.Validate())
}

func TestMerchantReportQueryRequiresReportNo(t *testing.T) {
	req := MerchantReportQueryRequest{MerchantID: "100000", TerminalID: "200000", ReportType: ReportTypeWechat, ReportNo: "MR202605040001"}
	require.NoError(t, req.Validate())

	req.ReportNo = ""
	require.EqualError(t, req.Validate(), "baofu merchant report query reportNo is required")
}

func TestBindSubConfigRequiresAppletAppID(t *testing.T) {
	req := BindSubConfigRequest{MerchantID: "100000", TerminalID: "200000", SubMchID: "1900000109", AuthType: AuthTypeApplet, AuthContent: "wx1234567890abcdef", Remark: "LocalLife mini program"}
	require.NoError(t, req.Validate())

	req.AuthContent = ""
	require.EqualError(t, req.Validate(), "baofu bind_sub_config authContent is required for APPLET")

	req = BindSubConfigRequest{MerchantID: "100000", TerminalID: "200000", SubMchID: "1900000109", AuthType: "UNKNOWN", AuthContent: "wx1234567890abcdef", Remark: "LocalLife mini program"}
	require.EqualError(t, req.Validate(), "baofu bind_sub_config authType is unsupported")
}

func TestNormalizeMerchantReportState(t *testing.T) {
	require.Equal(t, ReportStateSucceeded, NormalizeMerchantReportState("SUCCESS"))
	require.Equal(t, ReportStateFailed, NormalizeMerchantReportState("FAIL"))
	require.Equal(t, ReportStateProcessing, NormalizeMerchantReportState("PROCESSING"))
	require.Equal(t, ReportStateUnknown, NormalizeMerchantReportState("unexpected"))
}

func validWechatMerchantReportRequestForTest() WechatMerchantReportRequest {
	return WechatMerchantReportRequest{
		MerchantID:    "100000",
		TerminalID:    "200000",
		ReportType:    ReportTypeWechat,
		ReportNo:      "MR202605040099",
		BCTMerchantID: "CM202605040099",
		ReportInfo: WechatReportInfo{
			MerchantName:        "上海某某餐饮有限公司",
			MerchantShortName:   "某某餐饮",
			ServicePhone:        "02112345678",
			ChannelID:           "channel-001",
			ChannelName:         "乐客来福",
			Business:            "758-2",
			ServiceCodes:        []string{WechatServiceTypeApplet},
			AddressInfo:         WechatAddressInfo{ProvinceCode: "310000", CityCode: "310100", DistrictCode: "310115", Address: "世纪大道 1 号"},
			BusinessLicenseType: WechatCertificateTypeNationalLegalMerge,
			BusinessLicense:     "91310000123456789X",
			BankCardInfo:        WechatBankCardInfo{CardName: "上海某某餐饮有限公司", CardNo: "6222000000000000000", BankBranchName: "招商银行上海分行"},
		},
	}
}

func TestMerchantReportAppendixEnumsAreTypedAllowlists(t *testing.T) {
	cases := []struct {
		name   string
		value  string
		check  func(string) bool
		reject string
	}{
		{"terminal device type", TerminalDeviceTypeStore, IsValidTerminalDeviceType, "99"},
		{"operation flag", OperationFlagCreate, IsValidOperationFlag, "03"},
		{"device status", DeviceStatusEnabled, IsValidDeviceStatus, "99"},
		{"wechat service type micopay", WechatServiceTypeMicropay, IsValidWechatServiceType, "NATIVE"},
		{"alipay service type", AlipayServiceTypeFaceToFace, IsValidAlipayServiceType, "APPLET"},
		{"contact business type", ContactBusinessTypeMerchantContact, IsValidContactBusinessType, "99"},
		{"wechat certificate identity", WechatCertificateTypeIdentityCard, IsValidWechatCertificateType, "PASSPORT"},
		{"alipay certificate", AlipayCertificateTypeInstRegistration, IsValidAlipayCertificateType, "IDENTITY_CARD"},
		{"alipay contact type", AlipayContactTypeLegalPerson, IsValidAlipayContactType, "OWNER"},
		{"site type", SiteTypeMiniProgram, IsValidSiteType, "99"},
		{"indirect level", IndirectLevelM1, IsValidIndirectLevel, "M1"},
		{"merchant status", MerchantStatusEnabled, IsValidMerchantStatus, "99"},
		{"transaction control", TransactionControlAllowed, IsValidTransactionControl, "99"},
		{"auth order state", AuthOrderStateContactConfirm, IsValidAuthOrderState, "UNKNOWN"},
		{"merchant auth state", MerchantAuthStateAuthorized, IsValidMerchantAuthState, "UNKNOWN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, tc.check(tc.value))
			require.False(t, tc.check(tc.reject))
		})
	}
}
