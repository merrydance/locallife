package account

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/merrydance/locallife/baofu"
	"github.com/merrydance/locallife/baofu/account/contracts"
	"github.com/stretchr/testify/require"
)

func TestAccountClientQueryBalancePostsOfficialUnionGatewayRequest(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{"retCode": "SUCCESS", "contractNo": "CM202605040001", "availableBal": "123.45", "pendingBal": "1.00", "currBal": "124.45", "freezeBal": "0.00"}}
	client := NewClient(testBaofuRootClient(t, doer))

	result, err := client.QueryBalance(context.Background(), contracts.BalanceQueryRequest{MerchantID: "102004465", TerminalID: "200005200", ContractNo: "CM202605040001"})

	require.NoError(t, err)
	require.Equal(t, int64(12345), result.AvailableAmountFen)
	require.Equal(t, baofu.SandboxAccountGatewayBaseURL+"/T-1001-013-06/transReq.do", doer.request.URL.Scheme+"://"+doer.request.URL.Host+doer.request.URL.Path)
	require.Empty(t, doer.requestBody)
	query := doer.request.URL.Query()
	require.Equal(t, "102004465", query.Get("memberId"))
	require.Equal(t, "200005200", query.Get("terminalId"))
	require.Equal(t, baofu.UnionGWVerifyTypeRSA, query.Get("verifyType"))
	require.NotEmpty(t, query.Get("content"))
	require.Empty(t, query.Get("veryfyString"))
	plaintext, err := baofu.DecodeUnionGWVerifyType1Content(doer.baofuPublicPEM, query.Get("content"))
	require.NoError(t, err)
	var env baofu.UnionGWPlaintextEnvelope
	require.NoError(t, json.Unmarshal(plaintext, &env))
	require.Equal(t, "102004465", env.Header.MemberID)
	require.Equal(t, "200005200", env.Header.TerminalID)
	require.Equal(t, "T-1001-013-06", env.Header.ServiceType)
	require.Equal(t, baofu.UnionGWVerifyTypeRSA, env.Header.VerifyType)
	require.Contains(t, string(env.Body), `"version":"4.0.0"`)
	require.Contains(t, string(env.Body), `"contractNo":"CM202605040001"`)
}

func TestAccountClientQueryBalanceParsesNumericOfficialAmounts(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{
		"retCode":      1,
		"contractNo":   "CP610000000000542938",
		"availableBal": 0,
		"pendingBal":   1.23,
		"currBal":      1.23,
		"freezeBal":    0,
	}}
	client := NewClient(testBaofuRootClient(t, doer))

	result, err := client.QueryBalance(context.Background(), contracts.BalanceQueryRequest{
		MerchantID:  "102004465",
		TerminalID:  "200005200",
		ContractNo:  "CP610000000000542938",
		AccountType: "personal",
	})

	require.NoError(t, err)
	require.Equal(t, "CP610000000000542938", result.ContractNo)
	require.Equal(t, int64(0), result.AvailableAmountFen)
	require.Equal(t, int64(123), result.PendingAmountFen)
	require.Equal(t, int64(123), result.LedgerAmountFen)
	require.Equal(t, int64(0), result.FrozenAmountFen)
	require.Equal(t, "0", result.UpstreamAvailable)
}

func TestAccountClientQueryBalanceDefaultsMissingOptionalAmounts(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{
		"retCode":      1,
		"contractNo":   "CP610000000000542938",
		"availableBal": 0,
		"currBal":      0,
		"freezeBal":    0,
		"errorCode":    "SUCCESS",
	}}
	client := NewClient(testBaofuRootClient(t, doer))

	result, err := client.QueryBalance(context.Background(), contracts.BalanceQueryRequest{
		MerchantID:  "102004465",
		TerminalID:  "200005200",
		ContractNo:  "CP610000000000542938",
		AccountType: "personal",
	})

	require.NoError(t, err)
	require.Equal(t, int64(0), result.AvailableAmountFen)
	require.Equal(t, int64(0), result.PendingAmountFen)
	require.Equal(t, int64(0), result.LedgerAmountFen)
	require.Equal(t, int64(0), result.FrozenAmountFen)
}

func TestAccountClientQueryBalanceKeepsRequestedContractWhenResponseOmitsIt(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{
		"retCode":      1,
		"availableBal": 0,
		"currBal":      0,
	}}
	client := NewClient(testBaofuRootClient(t, doer))

	result, err := client.QueryBalance(context.Background(), contracts.BalanceQueryRequest{
		MerchantID:  "102004465",
		TerminalID:  "200005200",
		ContractNo:  "CP610000000000542938",
		AccountType: "personal",
	})

	require.NoError(t, err)
	require.Equal(t, "CP610000000000542938", result.ContractNo)
}

func TestAccountClientOpenAccountUsesConfiguredNotifyBaseURL(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: accountOpenAcceptedResponseForTest("OPEN202605040001", "测试用户")}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.OpenAccount(context.Background(), contracts.OpenAccountRequest{
		AccountType:   "personal",
		OutRequestNo:  "OPEN202605040001",
		LoginNo:       "LLBFOR0000000001",
		LegalName:     "测试用户",
		CertificateNo: "110101199001010011",
		BankAccountNo: "6222020202020202020",
		BankMobile:    "13800138000",
	})

	require.NoError(t, err)
	env := accountRequestEnvelopeForTest(t, doer)
	require.Equal(t, "T-1001-013-01", env.Header.ServiceType)
	require.JSONEq(t, `{"noticeUrl":"https://api.example.com/v1/webhooks/baofu/account/open"}`, partialJSONForAccountTest(t, env.Body, "noticeUrl"))
	require.NotContains(t, string(env.Body), "placeholder.local")
}

func TestAccountClientOpenAndQueryAccountDefaultToPayoutIdentity(t *testing.T) {
	openDoer := &accountRecordingDoer{responseBody: accountOpenAcceptedResponseForTest("OPEN202605040001", "测试用户")}
	openClient := NewClient(testBaofuRootClient(t, openDoer))

	_, err := openClient.OpenAccount(context.Background(), contracts.OpenAccountRequest{
		AccountType:   "personal",
		OutRequestNo:  "OPEN202605040001",
		LoginNo:       "LLBFOR0000000001",
		LegalName:     "测试用户",
		CertificateNo: "110101199001010011",
		BankAccountNo: "6222020202020202020",
		BankMobile:    "13800138000",
	})

	require.NoError(t, err)
	openQuery := openDoer.request.URL.Query()
	require.Equal(t, "102004466", openQuery.Get("memberId"))
	require.Equal(t, "200005201", openQuery.Get("terminalId"))
	openEnv := accountRequestEnvelopeForTest(t, openDoer)
	require.Equal(t, "102004466", openEnv.Header.MemberID)
	require.Equal(t, "200005201", openEnv.Header.TerminalID)

	queryDoer := &accountRecordingDoer{responseBody: map[string]any{"retCode": 1, "result": []map[string]any{{"transSerialNo": "OPEN202605040001", "contractNo": "CM202605040001", "state": 1}}}}
	queryClient := NewClient(testBaofuRootClient(t, queryDoer))

	_, err = queryClient.QueryAccount(context.Background(), contracts.QueryAccountRequest{
		ContractNo:  "CM202605040001",
		AccountType: "personal",
	})

	require.NoError(t, err)
	queryValues := queryDoer.request.URL.Query()
	require.Equal(t, "102004466", queryValues.Get("memberId"))
	require.Equal(t, "200005201", queryValues.Get("terminalId"))
	queryEnv := accountRequestEnvelopeForTest(t, queryDoer)
	require.Equal(t, "102004466", queryEnv.Header.MemberID)
	require.Equal(t, "200005201", queryEnv.Header.TerminalID)
}

func TestAccountClientOpenAccountRejectsPersonalTwoFactorBeforeHTTP(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{"retCode": "1"}}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.OpenAccount(context.Background(), contracts.OpenAccountRequest{
		AccountType:   "personal",
		OutRequestNo:  "OPEN202605040001",
		LegalName:     "测试用户",
		CertificateNo: "110101199001010011",
	})

	require.EqualError(t, err, "baofu open account personal bankAccountNo is required")
	require.Nil(t, doer.request)
}

func TestAccountClientOpenAccountMapsCompleteBusinessInputToOfficialDTO(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: accountOpenAcceptedResponseForTest("OPEN202605040003", "某某餐饮店")}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.OpenAccount(context.Background(), contracts.OpenAccountRequest{
		AccountType:                "business",
		OutRequestNo:               "OPEN202605040003",
		LoginNo:                    "LLBFOM0000000003",
		Email:                      "merchant@example.com",
		SelfEmployed:               true,
		CustomerName:               "某某餐饮店",
		AliasName:                  "某某餐饮",
		CertificateNo:              "91310000123456789X",
		CertificateType:            contracts.OfficialBusinessCertificateTypeLicense,
		CorporateName:              "王五",
		CorporateCertType:          contracts.OfficialCertificateTypeID,
		CorporateCertID:            "110101199001011236",
		CorporateMobile:            "13800138002",
		IndustryID:                 "5812",
		ContactName:                "赵六",
		ContactMobile:              "13800138003",
		BankAccountNo:              "6222020000000000001",
		BankName:                   "招商银行",
		DepositBankProvince:        "上海市",
		DepositBankCity:            "上海市",
		DepositBankName:            "招商银行上海分行",
		RegisterCapital:            "10",
		CardUserName:               "王五",
		PlatformNo:                 "100030218",
		PlatformTerminalID:         "200000001",
		QualificationTransSerialNo: "QUAL202605040001",
	})

	require.NoError(t, err)
	env := accountRequestEnvelopeForTest(t, doer)
	require.Equal(t, "T-1001-013-01", env.Header.ServiceType)
	require.JSONEq(t, `{
			"version":"4.1.0",
			"accType":2,
			"businessType":"BCT2.0",
			"accInfo":{
				"transSerialNo":"OPEN202605040003",
				"loginNo":"LLBFOM0000000003",
				"email":"merchant@example.com",
				"selfEmployed":true,
				"customerName":"某某餐饮店",
				"aliasName":"某某餐饮",
			"certificateNo":"91310000123456789X",
			"certificateType":"LICENSE",
			"corporateName":"王五",
			"corporateCertType":"ID",
			"corporateCertId":"110101199001011236",
			"corporateMobile":"13800138002",
			"industryId":"5812",
			"contactName":"赵六",
			"contactMobile":"13800138003",
			"cardNo":"6222020000000000001",
			"bankName":"招商银行",
			"depositBankProvince":"上海市",
				"depositBankCity":"上海市",
				"depositBankName":"招商银行上海分行",
				"registerCapital":"10",
				"cardUserName":"王五"
			}
	}`, partialJSONForAccountTest(t, env.Body, "version", "accType", "businessType", "accInfo"))
	require.NotContains(t, string(env.Body), "platformNo")
	require.NotContains(t, string(env.Body), "platformTerminalId")
	require.NotContains(t, string(env.Body), "qualificationTransSerialNo")
}

func TestAccountClientOpenAccountParsesOfficialResultArray(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{
		"retCode":   1,
		"errorCode": "BF0001",
		"errorMsg":  "上游资料校验失败",
		"result": []map[string]any{{
			"transSerialNo": "OPEN202605050001",
			"loginNo":       "OPEN202605050001",
			"customerName":  "测试用户",
			"contractNo":    "",
			"state":         -1,
			"errorCode":     "BF0001",
			"errorMsg":      "上游资料校验失败",
		}},
	}}
	client := NewClient(testBaofuRootClient(t, doer))

	result, err := client.OpenAccount(context.Background(), contracts.OpenAccountRequest{
		AccountType:   "personal",
		OutRequestNo:  "OPEN202605050001",
		LegalName:     "测试用户",
		CertificateNo: "110101199001010011",
		BankAccountNo: "6222020202020202020",
		BankMobile:    "13800138000",
	})

	require.NoError(t, err)
	require.Equal(t, "OPEN202605050001", result.OutRequestNo)
	require.Equal(t, contracts.OpenStateAbnormal, result.OpenState)
	require.Empty(t, result.ContractNo)
	require.Equal(t, "BF0001", result.FailCode)
	require.Equal(t, "上游资料校验失败", result.FailMessage)
}

func TestAccountClientOpenAccountRejectsOfficialSuccessMissingState(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{
		"retCode": 1,
		"result": []map[string]any{{
			"transSerialNo": "OPEN202605050001",
			"loginNo":       "OPEN202605050001",
			"customerName":  "测试用户",
			"contractNo":    "CP610000000000542938",
		}},
	}}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.OpenAccount(context.Background(), contracts.OpenAccountRequest{
		AccountType:   "personal",
		OutRequestNo:  "OPEN202605050001",
		LegalName:     "测试用户",
		CertificateNo: "110101199001010011",
		BankAccountNo: "6222020202020202020",
		BankMobile:    "13800138000",
	})

	requireAccountProviderContractError(t, err, "T-1001-013-01", "baofu open account response state is required")
}

func TestAccountClientQueryAccountUsesPersonalAccountType(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{"retCode": 1, "result": map[string]any{"contractNo": "CM202605050001"}}}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.QueryAccount(context.Background(), contracts.QueryAccountRequest{
		OutRequestNo:    "OPEN202605050001",
		LoginNo:         "LLBFOR0000000001",
		AccountType:     "personal",
		CertificateNo:   "110101199001011234",
		CertificateType: contracts.OfficialCertificateTypeID,
	})

	require.NoError(t, err)
	env := accountRequestEnvelopeForTest(t, doer)
	require.Equal(t, "T-1001-013-03", env.Header.ServiceType)
	require.JSONEq(t, `{"version":"4.0.0","accType":1,"loginNo":"LLBFOR0000000001","certificateNo":"110101199001011234","certificateType":"ID"}`, partialJSONForAccountTest(t, env.Body, "version", "accType", "loginNo", "certificateNo", "certificateType"))
	require.NotContains(t, string(env.Body), "platformNo")
}

func TestAccountClientQueryAccountCanSendOfficialCredentialFields(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{"retCode": 1, "result": map[string]any{"contractNo": "CP610000000000542938"}}}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.QueryAccount(context.Background(), contracts.QueryAccountRequest{
		OutRequestNo:    "OPEN202605050001",
		LoginNo:         "LLBFOR0000000001",
		AccountType:     "personal",
		CertificateNo:   "110101199001011234",
		CertificateType: contracts.OfficialCertificateTypeID,
	})

	require.NoError(t, err)
	env := accountRequestEnvelopeForTest(t, doer)
	require.Equal(t, "T-1001-013-03", env.Header.ServiceType)
	require.JSONEq(t, `{"version":"4.0.0","accType":1,"loginNo":"LLBFOR0000000001","certificateNo":"110101199001011234","certificateType":"ID"}`, partialJSONForAccountTest(t, env.Body, "version", "accType", "loginNo", "certificateNo", "certificateType"))
	require.NotContains(t, string(env.Body), "platformNo")
}

func TestAccountClientQueryAccountTreatsContractOnlySuccessAsActive(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{
		"retCode":   1,
		"errorCode": "SUCCESS",
		"result": []map[string]any{{
			"transSerialNo": "OPEN202605050001",
			"contractNo":    "CP610000000000542938",
		}},
	}}
	client := NewClient(testBaofuRootClient(t, doer))

	result, err := client.QueryAccount(context.Background(), contracts.QueryAccountRequest{
		ContractNo:  "CP610000000000542938",
		AccountType: "personal",
	})

	require.NoError(t, err)
	require.Equal(t, "OPEN202605050001", result.OutRequestNo)
	require.Equal(t, "CP610000000000542938", result.ContractNo)
	require.Equal(t, contracts.OpenStateActive, result.OpenState)
	require.Equal(t, "1", result.UpstreamState)
	require.Empty(t, result.FailCode)
}

func TestAccountClientQueryAccountRejectsOfficialSuccessMissingContractNo(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{
		"retCode": 1,
		"result":  map[string]any{"contractName": "测试账户"},
	}}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.QueryAccount(context.Background(), contracts.QueryAccountRequest{
		ContractNo:  "CP610000000000542938",
		AccountType: "personal",
	})

	requireAccountProviderContractError(t, err, "T-1001-013-03", "baofu query account response contractNo is required")
}

func TestAccountClientCreateWithdrawTreatsSyncAcceptedAsProcessing(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{
		"retCode":       1,
		"contractNo":    "CP610000000000542938",
		"state":         1,
		"transSerialNo": "WD202605050001",
		"transRemark":   "",
	}}
	client := NewClient(testBaofuRootClient(t, doer))

	result, err := client.CreateWithdraw(context.Background(), contracts.WithdrawRequest{
		MerchantID:    "102004466",
		TerminalID:    "200005201",
		ContractNo:    "CP610000000000542938",
		TransSerialNo: "WD202605050001",
		AmountFen:     100,
		NotifyURL:     "https://api.example.com/v1/webhooks/baofu/account/withdraw",
	})

	require.NoError(t, err)
	require.Equal(t, "processing", result.Status)
	require.Equal(t, "1", result.UpstreamState)
	env := accountRequestEnvelopeForTest(t, doer)
	require.Equal(t, "T-1001-013-14", env.Header.ServiceType)
	require.JSONEq(t, `{"version":"4.2.0","contractNo":"CP610000000000542938","transSerialNo":"WD202605050001","dealAmount":"1.00","returnUrl":"https://api.example.com/v1/webhooks/baofu/account/withdraw"}`, partialJSONForAccountTest(t, env.Body, "version", "contractNo", "transSerialNo", "dealAmount", "returnUrl"))
}

func TestAccountClientQueryWithdrawSendsOfficialTradeTimeAndParsesAmounts(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{
		"retCode":             1,
		"memberId":            "102004466",
		"contractNo":          "CP610000000000542938",
		"state":               1,
		"orderId":             21273130,
		"transSerialNo":       "WD202605050001",
		"transMoney":          10.01,
		"transFee":            1,
		"transferTotalAmount": 10.01,
		"transRemark":         "成功",
	}}
	client := NewClient(testBaofuRootClient(t, doer))

	result, err := client.QueryWithdraw(context.Background(), contracts.WithdrawQueryRequest{
		MerchantID:    "102004466",
		TerminalID:    "200005201",
		TransSerialNo: "WD202605050001",
		TradeTime:     "2026-05-05",
	})

	require.NoError(t, err)
	require.Equal(t, "21273130", result.BaofuWithdrawNo)
	require.Equal(t, int64(1001), result.AmountFen)
	require.Equal(t, int64(100), result.FeeFen)
	require.Equal(t, int64(1001), result.TotalAmountFen)
	require.Equal(t, "成功", result.Remark)
	require.Equal(t, "succeeded", result.Status)
	env := accountRequestEnvelopeForTest(t, doer)
	require.Equal(t, "T-1001-013-15", env.Header.ServiceType)
	require.JSONEq(t, `{"version":"4.2.0","transSerialNo":"WD202605050001","tradeTime":"2026-05-05"}`, partialJSONForAccountTest(t, env.Body, "version", "transSerialNo", "tradeTime"))
}

func TestAccountClientQueryWithdrawRejectsInvalidOfficialAmount(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{
		"retCode":             1,
		"memberId":            "102004466",
		"contractNo":          "CP610000000000542938",
		"state":               1,
		"orderId":             21273130,
		"transSerialNo":       "WD202605050001",
		"transMoney":          "not-a-money",
		"transFee":            "1.00",
		"transferTotalAmount": "10.01",
	}}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.QueryWithdraw(context.Background(), contracts.WithdrawQueryRequest{
		MerchantID:    "102004466",
		TerminalID:    "200005201",
		TransSerialNo: "WD202605050001",
		TradeTime:     "2026-05-05",
	})

	requireAccountProviderContractError(t, err, "T-1001-013-15", "baofu amount must be a decimal number")
}

func requireAccountProviderContractError(t *testing.T, err error, operation string, cause string) {
	t.Helper()
	require.Error(t, err)
	var providerErr *baofu.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, operation, providerErr.Operation)
	require.Contains(t, errors.Unwrap(providerErr).Error(), cause)
}

func accountOpenAcceptedResponseForTest(transSerialNo string, customerName string) map[string]any {
	return map[string]any{
		"retCode": 1,
		"result": []map[string]any{{
			"transSerialNo": transSerialNo,
			"loginNo":       transSerialNo,
			"customerName":  customerName,
			"state":         2,
		}},
	}
}

func TestAccountClientQueryWithdrawRejectsMissingTradeTimeBeforeHTTP(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{"retCode": 1}}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.QueryWithdraw(context.Background(), contracts.WithdrawQueryRequest{
		MerchantID:    "102004466",
		TerminalID:    "200005201",
		TransSerialNo: "WD202605050001",
	})

	require.EqualError(t, err, "baofu withdraw query tradeTime is required")
	require.Nil(t, doer.request)
}

func TestAccountClientQueryBalanceUsesPersonalAccountType(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{"retCode": 1, "contractNo": "CM202605040001", "availableBal": "123.45", "pendingBal": "1.00", "currBal": "124.45", "freezeBal": "0.00"}}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.QueryBalance(context.Background(), contracts.BalanceQueryRequest{
		MerchantID:  "102004465",
		TerminalID:  "200005200",
		ContractNo:  "CM202605040001",
		AccountType: "personal",
	})

	require.NoError(t, err)
	env := accountRequestEnvelopeForTest(t, doer)
	require.Equal(t, "T-1001-013-06", env.Header.ServiceType)
	require.JSONEq(t, `{"version":"4.0.0","accType":1,"contractNo":"CM202605040001"}`, partialJSONForAccountTest(t, env.Body, "version", "accType", "contractNo"))
}

func TestAccountClientReturnsProviderErrorForBusinessFailure(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{"retCode": 0, "errorCode": "BF00061", "errorMsg": "上游原始四要素错误"}}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.QueryBalance(context.Background(), contracts.BalanceQueryRequest{MerchantID: "102004465", TerminalID: "200005200", ContractNo: "CM202605040001"})

	require.Error(t, err)
	require.NotContains(t, err.Error(), "上游原始四要素错误")
	var providerErr *baofu.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "BF00061", providerErr.UpstreamCode)
	require.Equal(t, "上游原始四要素错误", providerErr.UpstreamMessage)
	require.Equal(t, "身份或银行卡信息核验未通过，请核对后重新提交", providerErr.Frontend.Message)
}

func TestAccountClientReturnsProviderErrorWhenErrorCodeHasNoRetCode(t *testing.T) {
	doer := &accountRecordingDoer{responseBody: map[string]any{"errorCode": "BF0005", "errorMsg": "上游处理中"}}
	client := NewClient(testBaofuRootClient(t, doer))

	_, err := client.QueryAccount(context.Background(), contracts.QueryAccountRequest{ContractNo: "CP610000000000542938"})

	require.Error(t, err)
	require.NotContains(t, err.Error(), "上游处理中")
	var providerErr *baofu.ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, "BF0005", providerErr.UpstreamCode)
	require.Equal(t, "上游处理中", providerErr.UpstreamMessage)
	require.Equal(t, "支付通道处理中，请稍后重试", providerErr.Frontend.Message)
	require.True(t, providerErr.Frontend.Retryable)
}

type accountRecordingDoer struct {
	request         *http.Request
	requestBody     []byte
	responseBody    map[string]any
	baofuPrivatePEM string
	baofuPublicPEM  string
}

func (d *accountRecordingDoer) Do(req *http.Request) (*http.Response, error) {
	d.request = req
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	d.requestBody = body
	query := req.URL.Query()
	plain, err := baofu.DecodeUnionGWVerifyType1Content(d.baofuPublicPEM, query.Get("content"))
	if err != nil {
		return nil, err
	}
	var requestEnvelope baofu.UnionGWPlaintextEnvelope
	if err := json.Unmarshal(plain, &requestEnvelope); err != nil {
		return nil, err
	}
	responsePlain, err := baofu.CanonicalJSON(baofu.UnionGWPlaintextEnvelope{
		Header: baofu.UnionGWHeader{
			MemberID:       query.Get("memberId"),
			TerminalID:     query.Get("terminalId"),
			ServiceType:    requestEnvelope.Header.ServiceType,
			SystemRespCode: baofu.UnionGWSystemRespSuccess,
			SystemRespDesc: "",
		},
		Body: mustAccountResponseRaw(d.responseBody),
	})
	if err != nil {
		return nil, err
	}
	content, err := baofu.EncodeUnionGWVerifyType1Content(d.baofuPrivatePEM, responsePlain)
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(content))), Header: make(http.Header)}, nil
}

func testBaofuRootClient(t *testing.T, doer baofu.HTTPDoer) *baofu.Client {
	t.Helper()
	privatePEM, publicPEM := generateClientTestKeyPair(t)
	if recorder, ok := doer.(*accountRecordingDoer); ok {
		recorder.baofuPrivatePEM = privatePEM
		recorder.baofuPublicPEM = publicPEM
	}
	client, err := baofu.NewClient(baofu.Config{Environment: baofu.BaofuEnvironmentSandbox, CollectMerchantID: "102004465", CollectTerminalID: "200005200", PayoutMerchantID: "102004466", PayoutTerminalID: "200005201", AppID: "wx1234567890abcdef", PrivateKeyPEM: privatePEM, BaofuPublicKeyPEM: publicPEM, NotifyBaseURL: "https://api.example.com/v1/webhooks/baofu", SignSerialNo: "1", EncryptionSerialNo: "1", Timeout: 5 * time.Second}, doer)
	require.NoError(t, err)
	return client
}

func generateClientTestKeyPair(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})), string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
}

func mustAccountResponseRaw(value map[string]any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func accountRequestEnvelopeForTest(t *testing.T, doer *accountRecordingDoer) baofu.UnionGWPlaintextEnvelope {
	t.Helper()
	require.NotNil(t, doer)
	require.NotNil(t, doer.request)
	plaintext, err := baofu.DecodeUnionGWVerifyType1Content(doer.baofuPublicPEM, doer.request.URL.Query().Get("content"))
	require.NoError(t, err)
	var env baofu.UnionGWPlaintextEnvelope
	require.NoError(t, json.Unmarshal(plaintext, &env))
	return env
}

func partialJSONForAccountTest(t *testing.T, raw json.RawMessage, keys ...string) string {
	t.Helper()
	var full map[string]any
	require.NoError(t, json.Unmarshal(raw, &full))
	partial := make(map[string]any, len(keys))
	for _, key := range keys {
		partial[key] = full[key]
	}
	body, err := json.Marshal(partial)
	require.NoError(t, err)
	return string(body)
}
