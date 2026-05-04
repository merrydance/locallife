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

func TestOfficialPersonalTwoFactorOpenAccountAllowsNoBankCard(t *testing.T) {
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
		},
	}

	require.NoError(t, req.Validate())
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
		Version:     OfficialOpenAccountVersion,
		AccountType: OfficialAccountTypePersonal,
		ContractNo:  "CM202605040001",
	}
	require.NoError(t, query.Validate())

	balance := OfficialBalanceQueryRequest{
		Version:     OfficialOpenAccountVersion,
		AccountType: OfficialAccountTypePersonal,
		ContractNo:  "CM202605040001",
	}
	require.NoError(t, balance.Validate())

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
}
