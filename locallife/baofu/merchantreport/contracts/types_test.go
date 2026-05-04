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
			AddressInfo:         WechatAddressInfo{Province: "上海市", City: "上海市", District: "浦东新区", Address: "世纪大道 1 号"},
			BusinessLicenseType: WechatCertificateTypeNationalLegalMerge,
			BusinessLicense:     "91310000123456789X",
			BankCardInfo:        WechatBankCardInfo{AccountName: "上海某某餐饮有限公司", AccountNo: "6222000000000000000", BankName: "招商银行", BankBranchName: "招商银行上海分行"},
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
			AddressInfo:         WechatAddressInfo{Province: "上海市", City: "上海市", District: "浦东新区", Address: "世纪大道 1 号"},
			BusinessLicenseType: WechatCertificateTypeNationalLegalMerge,
			BusinessLicense:     "91310000123456789X",
			BankCardInfo:        WechatBankCardInfo{AccountName: "上海某某餐饮有限公司", AccountNo: "6222000000000000000", BankName: "招商银行", BankBranchName: "招商银行上海分行"},
		},
	}

	body, err := json.Marshal(req)

	require.NoError(t, err)
	require.Contains(t, string(body), `"reportType":"WECHAT"`)
	require.Contains(t, string(body), `"bctMerId":"CM202605040002"`)
	require.Contains(t, string(body), `"merchant_name":"上海某某餐饮有限公司"`)
	require.Contains(t, string(body), `"merchant_shortname":"某某餐饮"`)
	require.Contains(t, string(body), `"service_codes":["APPLET"]`)
	require.NotContains(t, string(body), "sharingMerId")
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
}

func TestNormalizeMerchantReportState(t *testing.T) {
	require.Equal(t, ReportStateSucceeded, NormalizeMerchantReportState("SUCCESS"))
	require.Equal(t, ReportStateFailed, NormalizeMerchantReportState("FAIL"))
	require.Equal(t, ReportStateProcessing, NormalizeMerchantReportState("PROCESSING"))
	require.Equal(t, ReportStateUnknown, NormalizeMerchantReportState("unexpected"))
}
