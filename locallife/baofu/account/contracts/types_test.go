package contracts

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountResultDoesNotFallbackSharingMerIDFromContractNo(t *testing.T) {
	result := AccountResult{ContractNo: "CP123", Raw: json.RawMessage(`{"status":"1"}`)}

	normalized := result.Normalized()

	require.Empty(t, normalized.SharingMerID)
	require.Equal(t, json.RawMessage(`{"status":"1"}`), normalized.Raw)
}

func TestOpenStateFromUpstream(t *testing.T) {
	require.Equal(t, OpenStateActive, OpenStateFromUpstream("1"))
	require.Equal(t, OpenStateFailed, OpenStateFromUpstream("0"))
	require.Equal(t, OpenStateAbnormal, OpenStateFromUpstream("-1"))
	require.Equal(t, OpenStateProcessing, OpenStateFromUpstream("2"))
	require.Equal(t, OpenStateAbnormal, OpenStateFromUpstream("unexpected"))
}

func TestOfficialOpenAccountRequestRequiresBCT20Fields(t *testing.T) {
	req := OfficialOpenAccountRequest{
		Version:      OfficialOpenAccountVersion,
		AccountType:  OfficialAccountTypePersonal,
		NoticeURL:    "https://api.example.com/v1/webhooks/baofu/account/open",
		BusinessType: OfficialBusinessTypeBCT20,
		AccountInfo: OfficialPersonalAccountInfo{
			TransSerialNo:   "OPEN202605040001",
			LoginNo:         "rider13800138000",
			CustomerName:    "张三",
			CertificateType: OfficialCertificateTypeID,
			CertificateNo:   "110101199001011234",
			CardNo:          "6222020000000000000",
			MobileNo:        "13800138000",
			CardUserName:    "张三",
			NeedUploadFile:  false,
		},
	}

	require.NoError(t, req.Validate())

	req.BusinessType = ""
	require.EqualError(t, req.Validate(), "baofu open account businessType must be BCT2.0")
}

func TestOfficialOpenAccountRejectsShortLoginNo(t *testing.T) {
	req := OfficialOpenAccountRequest{
		Version:      OfficialOpenAccountVersion,
		AccountType:  OfficialAccountTypePersonal,
		NoticeURL:    "https://api.example.com/v1/webhooks/baofu/account/open",
		BusinessType: OfficialBusinessTypeBCT20,
		AccountInfo: OfficialPersonalAccountInfo{
			TransSerialNo:   "OPEN202605040002",
			LoginNo:         "short",
			CustomerName:    "李四",
			CertificateType: OfficialCertificateTypeID,
			CertificateNo:   "110101199001011235",
			CardUserName:    "李四",
			CardNo:          "6222020000000000002",
			MobileNo:        "13800138001",
		},
	}

	require.EqualError(t, req.Validate(), "baofu open account personal loginNo must be at least 11 characters")
}

func TestOfficialOpenAccountValidateOfficialMaxLengths(t *testing.T) {
	req := OfficialOpenAccountRequest{
		Version:      OfficialOpenAccountVersion,
		AccountType:  OfficialAccountTypePersonal,
		NoticeURL:    "https://api.example.com/v1/webhooks/baofu/account/open",
		BusinessType: OfficialBusinessTypeBCT20,
		AccountInfo: OfficialPersonalAccountInfo{
			TransSerialNo:   repeatAccountContract("T", 201),
			LoginNo:         "rider13800138000",
			CustomerName:    "张三",
			CertificateType: OfficialCertificateTypeID,
			CertificateNo:   "110101199001011234",
			CardNo:          "6222020000000000000",
			MobileNo:        "13800138000",
			CardUserName:    "张三",
		},
	}

	require.EqualError(t, req.Validate(), "baofu open account personal transSerialNo must be at most 200 characters")
}

func TestOpenAccountRequestRejectsPersonalTwoFactorProductionPath(t *testing.T) {
	req := OpenAccountRequest{
		AccountType:   "personal",
		OutRequestNo:  "OPEN202605040002",
		LegalName:     "李四",
		CertificateNo: "110101199001011235",
	}

	require.EqualError(t, req.Validate(), "baofu open account personal bankAccountNo is required")
}

func TestOfficialPersonalTwoFactorOpenAccountIsNotSupported(t *testing.T) {
	req := OfficialOpenAccountRequest{
		Version:      OfficialOpenAccountVersion,
		AccountType:  OfficialAccountTypePersonal,
		NoticeURL:    "https://api.example.com/v1/webhooks/baofu/account/open",
		BusinessType: OfficialBusinessTypeBCT20,
		AccountInfo: OfficialPersonalTwoFactorAccountInfo{
			TransSerialNo:   "OPEN202605040002",
			LoginNo:         "rider13800138001",
			CustomerName:    "李四",
			CertificateType: OfficialCertificateTypeID,
			CertificateNo:   "110101199001011235",
			CardUserName:    "李四",
		},
	}

	require.EqualError(t, req.Validate(), "baofu open account personal two-factor is not supported")
}

func TestOpenAccountRequestRequiresBusinessOfficialInputFields(t *testing.T) {
	req := OpenAccountRequest{
		AccountType:                "business",
		OutRequestNo:               "OPEN202605040003",
		Email:                      "merchant@example.com",
		CustomerName:               "某某餐饮店",
		CertificateNo:              "91310000123456789X",
		CorporateName:              "王五",
		CorporateCertType:          OfficialCertificateTypeID,
		CorporateCertID:            "110101199001011236",
		IndustryID:                 "5812",
		BankAccountNo:              "6222020000000000001",
		BankName:                   "招商银行",
		DepositBankProvince:        "上海市",
		DepositBankCity:            "上海市",
		DepositBankName:            "招商银行上海分行",
		SelfEmployed:               true,
		CardUserName:               "王五",
		CorporateMobile:            "13800138002",
		PlatformNo:                 "100030218",
		PlatformTerminalID:         "200000001",
		QualificationTransSerialNo: "QUAL202605040001",
	}

	require.NoError(t, req.Validate())

	req.DepositBankName = ""
	require.EqualError(t, req.Validate(), "baofu open account business depositBankName is required")
}

func TestOpenAccountRequestRejectsPlatformAccountType(t *testing.T) {
	req := OpenAccountRequest{
		AccountType:   "platform",
		OutRequestNo:  "OPEN202605040004",
		LegalName:     "平台公司",
		CertificateNo: "91310000123456789X",
	}

	require.EqualError(t, req.Validate(), "baofu open account accountType is unsupported")
}

func TestOfficialBusinessOpenAccountRequiresOfficialFields(t *testing.T) {
	req := OfficialOpenAccountRequest{
		Version:      OfficialOpenAccountVersion,
		AccountType:  OfficialAccountTypeBusiness,
		NoticeURL:    "https://api.example.com/v1/webhooks/baofu/account/open",
		BusinessType: OfficialBusinessTypeBCT20,
		AccountInfo: OfficialBusinessAccountInfo{
			TransSerialNo:       "OPEN202605040003",
			LoginNo:             "merchant-login-001",
			Email:               "merchant@example.com",
			SelfEmployed:        true,
			CustomerName:        "某某餐饮店",
			CertificateNo:       "91310000123456789X",
			CertificateType:     "LICENSE",
			CorporateName:       "王五",
			CorporateCertType:   OfficialCertificateTypeID,
			CorporateCertID:     "110101199001011236",
			IndustryID:          "5812",
			CardNo:              "6222020000000000001",
			BankName:            "招商银行",
			DepositBankProvince: "上海市",
			DepositBankCity:     "上海市",
			DepositBankName:     "招商银行上海分行",
			CorporateMobile:     "13800138002",
		},
	}

	require.NoError(t, req.Validate())

	info := req.AccountInfo.(OfficialBusinessAccountInfo)
	info.CardUserName = "王五"
	info.CorporateMobile = ""
	req.AccountInfo = info
	require.EqualError(t, req.Validate(), "baofu open account business corporateMobile is required for selfEmployed private card")
}

func TestOfficialOpenAccountOptionalAndConditionalFieldsSerializeOfficialNames(t *testing.T) {
	req := OfficialOpenAccountRequest{
		Version:      OfficialOpenAccountVersion,
		AccountType:  OfficialAccountTypeBusiness,
		NoticeURL:    "https://api.example.com/v1/webhooks/baofu/account/open",
		BusinessType: OfficialBusinessTypeBCT20,
		AccountInfo: OfficialBusinessAccountInfo{
			TransSerialNo:              "OPEN202605040003",
			LoginNo:                    "merchant-login-001",
			Email:                      "merchant@example.com",
			SelfEmployed:               true,
			CustomerName:               "某某餐饮店",
			AliasName:                  "某某餐饮",
			CertificateNo:              "91310000123456789X",
			CertificateType:            OfficialBusinessCertificateTypeLicense,
			CorporateName:              "王五",
			CorporateCertType:          OfficialCertificateTypeID,
			CorporateCertID:            "110101199001011236",
			CorporateMobile:            "13800138002",
			IndustryID:                 "5812",
			ContactName:                "赵六",
			ContactMobile:              "13800138003",
			CardNo:                     "6222020000000000001",
			BankName:                   "招商银行",
			DepositBankProvince:        "上海市",
			DepositBankCity:            "上海市",
			DepositBankName:            "招商银行上海分行",
			RegisterCapital:            "10",
			CardUserName:               "王五",
			PlatformNo:                 "100030218",
			PlatformTerminalID:         "200000001",
			QualificationTransSerialNo: "QUAL202605040001",
		},
	}

	body, err := json.Marshal(req)

	require.NoError(t, err)
	require.Contains(t, string(body), `"aliasName":"某某餐饮"`)
	require.Contains(t, string(body), `"contactName":"赵六"`)
	require.Contains(t, string(body), `"registerCapital":"10"`)
	require.Contains(t, string(body), `"cardUserName":"王五"`)
	require.Contains(t, string(body), `"platformNo":"100030218"`)
	require.Contains(t, string(body), `"platformTerminalId":"200000001"`)
	require.Contains(t, string(body), `"qualificationTransSerialNo":"QUAL202605040001"`)
}

func TestOfficialBalanceAmountConvertsYuanToFen(t *testing.T) {
	got, err := YuanStringToFen("123.45")
	require.NoError(t, err)
	require.Equal(t, int64(12345), got)

	got, err = YuanStringToFen("0.01")
	require.NoError(t, err)
	require.Equal(t, int64(1), got)

	_, err = YuanStringToFen("123.456")
	require.EqualError(t, err, "baofu amount supports at most 2 decimal places")
}

func TestOfficialWithdrawAmountConvertsFenToYuan(t *testing.T) {
	got, err := FenToYuanString(12345)
	require.NoError(t, err)
	require.Equal(t, "123.45", got)

	_, err = FenToYuanString(-1)
	require.EqualError(t, err, "baofu amount fen must be non-negative")
}

func TestOfficialQueryBalanceAndWithdrawValidateRequiredFields(t *testing.T) {
	query := OfficialQueryAccountRequest{
		Version:     OfficialQueryAccountVersion,
		AccountType: OfficialAccountTypePersonal,
		ContractNo:  "CM202605040001",
	}
	require.NoError(t, query.Validate())

	query.LoginNo = "OPEN202605040001"
	require.EqualError(t, query.Validate(), "baofu query account contractNo cannot be combined with loginNo identity fields")

	query = OfficialQueryAccountRequest{
		Version:     OfficialQueryAccountVersion,
		AccountType: OfficialAccountTypePersonal,
		LoginNo:     "OPEN202605040001",
	}
	require.EqualError(t, query.Validate(), "baofu query account certificateNo is required when loginNo is used")

	query.Version = OfficialOpenAccountVersion
	query.CertificateNo = "110101199001011234"
	query.CertificateType = OfficialCertificateTypeID
	query.PlatformNo = "100030218"
	require.EqualError(t, query.Validate(), "baofu query account version must be 4.0.0")

	query = OfficialQueryAccountRequest{
		Version:         OfficialQueryAccountVersion,
		AccountType:     OfficialAccountTypePersonal,
		LoginNo:         "OPEN202605040001",
		CertificateNo:   "110101199001011234",
		CertificateType: "PASSPORT",
		PlatformNo:      "100030218",
	}
	require.EqualError(t, query.Validate(), "baofu query account certificateType is unsupported")

	query = OfficialQueryAccountRequest{
		Version:         OfficialQueryAccountVersion,
		AccountType:     OfficialAccountTypePersonal,
		LoginNo:         "OPEN202605040001",
		CertificateNo:   "110101199001011234",
		CertificateType: OfficialCertificateTypeID,
		PlatformNo:      "100030218",
	}
	require.NoError(t, query.Validate())

	balance := OfficialBalanceQueryRequest{
		Version:     OfficialBalanceVersion,
		AccountType: OfficialAccountTypePersonal,
		ContractNo:  "CM202605040001",
	}
	require.NoError(t, balance.Validate())

	balance.Version = OfficialOpenAccountVersion
	require.EqualError(t, balance.Validate(), "baofu balance query version must be 4.0.0")

	withdraw := OfficialWithdrawRequest{
		Version:       OfficialWithdrawVersion,
		ContractNo:    "CM202605040001",
		TransSerialNo: "WD202605040001",
		DealAmount:    "123.45",
		ReturnURL:     "https://api.example.com/v1/webhooks/baofu/withdraw",
	}
	require.NoError(t, withdraw.Validate())

	withdraw.DealAmount = "123.456"
	require.EqualError(t, withdraw.Validate(), "baofu amount supports at most 2 decimal places")

	withdraw.DealAmount = "0.00"
	require.EqualError(t, withdraw.Validate(), "baofu withdraw dealAmount must be positive")
}

func TestOfficialAccountQueryAndWithdrawValidateOfficialMaxLengths(t *testing.T) {
	query := OfficialQueryAccountRequest{
		Version:     OfficialQueryAccountVersion,
		AccountType: OfficialAccountTypePersonal,
		ContractNo:  repeatAccountContract("C", 33),
	}
	require.EqualError(t, query.Validate(), "baofu query account contractNo must be at most 32 characters")

	withdraw := OfficialWithdrawRequest{
		Version:       OfficialWithdrawVersion,
		ContractNo:    "CM202605040001",
		TransSerialNo: repeatAccountContract("W", 51),
		DealAmount:    "123.45",
		ReturnURL:     "https://api.example.com/v1/webhooks/baofu/withdraw",
	}
	require.EqualError(t, withdraw.Validate(), "baofu withdraw transSerialNo must be at most 50 characters")
}

func repeatAccountContract(ch string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += ch
	}
	return out
}
