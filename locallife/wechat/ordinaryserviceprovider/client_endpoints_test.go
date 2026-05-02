package ordinaryserviceprovider

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
	"github.com/stretchr/testify/require"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
)

type capturedSDKRequest struct {
	method      string
	path        string
	query       url.Values
	body        interface{}
	contentType string
}

type captureSDKClient struct {
	responses []string
	calls     []capturedSDKRequest
}

func (c *captureSDKClient) Request(ctx context.Context, method, requestPath string, headerParams http.Header, queryParams url.Values, postBody interface{}, contentType string) (*core.APIResult, error) {
	responseBody := "{}"
	if len(c.responses) > 0 {
		responseBody = c.responses[0]
		c.responses = c.responses[1:]
	}
	c.calls = append(c.calls, capturedSDKRequest{
		method:      method,
		path:        requestPath,
		query:       queryParams,
		body:        postBody,
		contentType: contentType,
	})
	return &core.APIResult{Response: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(responseBody)), Header: http.Header{}}}, nil
}

func (c *captureSDKClient) Upload(ctx context.Context, requestURL, meta, reqBody, formContentType string) (*core.APIResult, error) {
	c.calls = append(c.calls, capturedSDKRequest{
		method:      http.MethodPost,
		path:        requestURL,
		body:        meta,
		contentType: formContentType,
	})
	responseBody := `{"media_id":"media-upload-001"}`
	if len(c.responses) > 0 {
		responseBody = c.responses[0]
		c.responses = c.responses[1:]
	}
	return &core.APIResult{Response: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(responseBody)), Header: http.Header{}}}, nil
}

func TestClientRoutesPaymentRefundAndProfitSharingEndpoints(t *testing.T) {
	fake := &captureSDKClient{responses: []string{
		`{"prepay_id":"wx-prepay-001"}`,
		`{"out_trade_no":"order-001","transaction_id":"tx-001","trade_state":"SUCCESS"}`,
		`{}`,
		`{"refund_id":"refund-id-001","out_refund_no":"refund-001","status":"PROCESSING"}`,
		`{"refund_id":"refund-id-001","out_refund_no":"refund-001","status":"SUCCESS"}`,
		`{"sub_mchid":"1900000109","type":"MERCHANT_ID","account":"1900000200","name":"encrypted-name"}`,
		`{"sub_mchid":"1900000109","type":"MERCHANT_ID","account":"1900000200"}`,
		`{"sub_mchid":"1900000109","transaction_id":"tx-001","out_order_no":"ps-001","order_id":"300845074"}`,
		`{"sub_mchid":"1900000109","transaction_id":"tx-001","out_order_no":"ps-001","order_id":"300845074"}`,
		`{"sub_mchid":"1900000109","out_order_no":"ps-001","out_return_no":"return-001","return_id":"R001"}`,
		`{"sub_mchid":"1900000109","out_order_no":"ps-001","out_return_no":"return-001","return_id":"R001"}`,
		`{"sub_mchid":"1900000109","transaction_id":"tx-001","out_order_no":"ps-unfreeze-001","order_id":"300845075"}`,
		`{"sub_mchid":"1900000109","transaction_id":"tx-001","amount":88}`,
	}}
	client := endpointTestClient(t, fake)
	ctx := context.Background()

	paymentResp, err := client.CreatePayment(ctx, validEndpointPaymentRequest())
	require.NoError(t, err)
	require.Equal(t, "wx-prepay-001", paymentResp.PrepayID)

	_, err = client.QueryPayment(ctx, contracts.PaymentQueryRequest{SubMchID: "1900000109", OutTradeNo: "order-001"})
	require.NoError(t, err)

	err = client.ClosePayment(ctx, contracts.PaymentCloseRequest{SpMchID: "1900000001", SubMchID: "1900000109", OutTradeNo: "order-001"})
	require.NoError(t, err)

	_, err = client.CreateRefund(ctx, validEndpointRefundRequest())
	require.NoError(t, err)

	_, err = client.QueryRefund(ctx, contracts.RefundQueryRequest{SubMchID: "1900000109", OutRefundNo: "refund-001"})
	require.NoError(t, err)

	_, err = client.AddProfitSharingReceiver(ctx, validEndpointReceiverAddRequest())
	require.NoError(t, err)

	_, err = client.DeleteProfitSharingReceiver(ctx, contracts.ProfitSharingReceiverDeleteRequest{SubMchID: "1900000109", AppID: "wx-sp-appid", Type: contracts.ReceiverTypeMerchantID, Account: "1900000200"})
	require.NoError(t, err)

	_, err = client.CreateProfitSharingOrder(ctx, validEndpointProfitSharingOrderRequest())
	require.NoError(t, err)

	_, err = client.QueryProfitSharingOrder(ctx, contracts.ProfitSharingQueryRequest{SubMchID: "1900000109", TransactionID: "tx-001", OutOrderNo: "ps-001"})
	require.NoError(t, err)

	_, err = client.CreateProfitSharingReturn(ctx, contracts.ProfitSharingReturnRequest{SubMchID: "1900000109", OutOrderNo: "ps-001", OutReturnNo: "return-001", ReturnMchID: "1900000200", Amount: 50, Description: "分账回退"})
	require.NoError(t, err)

	_, err = client.QueryProfitSharingReturn(ctx, contracts.ProfitSharingReturnQueryRequest{SubMchID: "1900000109", OutOrderNo: "ps-001", OutReturnNo: "return-001"})
	require.NoError(t, err)

	_, err = client.UnfreezeProfitSharing(ctx, contracts.ProfitSharingUnfreezeRequest{SubMchID: "1900000109", TransactionID: "tx-001", OutOrderNo: "ps-unfreeze-001", Description: "解冻剩余资金"})
	require.NoError(t, err)

	remainingResp, err := client.QueryProfitSharingRemainingAmount(ctx, contracts.ProfitSharingRemainingAmountRequest{SubMchID: "1900000109", TransactionID: "tx-001"})
	require.NoError(t, err)
	require.Equal(t, int64(88), remainingResp.Amount)

	requireEndpointCall(t, fake.calls[0], http.MethodPost, DefaultBaseURL+"/v3/pay/partner/transactions/jsapi")
	requireEndpointCall(t, fake.calls[1], http.MethodGet, DefaultBaseURL+"/v3/pay/partner/transactions/out-trade-no/order-001")
	require.Equal(t, "1900000001", fake.calls[1].query.Get("sp_mchid"))
	require.Equal(t, "1900000109", fake.calls[1].query.Get("sub_mchid"))
	requireEndpointCall(t, fake.calls[2], http.MethodPost, DefaultBaseURL+"/v3/pay/partner/transactions/out-trade-no/order-001/close")
	requireEndpointCall(t, fake.calls[3], http.MethodPost, DefaultBaseURL+"/v3/refund/domestic/refunds")
	requireEndpointCall(t, fake.calls[4], http.MethodGet, DefaultBaseURL+"/v3/refund/domestic/refunds/refund-001")
	requireEndpointCall(t, fake.calls[5], http.MethodPost, DefaultBaseURL+"/v3/profitsharing/receivers/add")
	requireEndpointCall(t, fake.calls[6], http.MethodPost, DefaultBaseURL+"/v3/profitsharing/receivers/delete")
	requireEndpointCall(t, fake.calls[7], http.MethodPost, DefaultBaseURL+"/v3/profitsharing/orders")
	requireEndpointCall(t, fake.calls[8], http.MethodGet, DefaultBaseURL+"/v3/profitsharing/orders/ps-001")
	require.Equal(t, "tx-001", fake.calls[8].query.Get("transaction_id"))
	requireEndpointCall(t, fake.calls[9], http.MethodPost, DefaultBaseURL+"/v3/profitsharing/return-orders")
	requireEndpointCall(t, fake.calls[10], http.MethodGet, DefaultBaseURL+"/v3/profitsharing/return-orders/return-001")
	require.Equal(t, "ps-001", fake.calls[10].query.Get("out_order_no"))
	requireEndpointCall(t, fake.calls[11], http.MethodPost, DefaultBaseURL+"/v3/profitsharing/orders/unfreeze")
	requireEndpointCall(t, fake.calls[12], http.MethodGet, DefaultBaseURL+"/v3/profitsharing/transactions/tx-001/amounts")
}

func TestClientRoutesCombinePaymentEndpoints(t *testing.T) {
	fake := &captureSDKClient{responses: []string{
		`{"prepay_id":"combine-prepay-001"}`,
		`{"combine_out_trade_no":"combine-001","trade_state":"SUCCESS"}`,
		`{}`,
	}}
	client := endpointTestClient(t, fake)
	ctx := context.Background()

	prepayResp, err := client.CreateCombinePayment(ctx, validEndpointCombineRequest())
	require.NoError(t, err)
	require.Equal(t, "combine-prepay-001", prepayResp.PrepayID)

	queryResp, err := client.QueryCombinePayment(ctx, contracts.CombineQueryRequest{CombineOutTradeNo: "combine-001"})
	require.NoError(t, err)
	require.Equal(t, contracts.PaymentTradeStateSuccess, queryResp.TradeState)

	err = client.CloseCombinePayment(ctx, contracts.CombineCloseRequest{
		CombineAppID:      "wx-combine-appid",
		CombineOutTradeNo: "combine-001",
		SubOrders: []contracts.CombineCloseSubOrder{{
			MchID:      "1900000001",
			SubMchID:   "1900000109",
			OutTradeNo: "order-001",
		}},
	})
	require.NoError(t, err)

	requireEndpointCall(t, fake.calls[0], http.MethodPost, DefaultBaseURL+"/v3/combine-transactions/jsapi")
	requireEndpointCall(t, fake.calls[1], http.MethodGet, DefaultBaseURL+"/v3/combine-transactions/out-trade-no/combine-001")
	requireEndpointCall(t, fake.calls[2], http.MethodPost, DefaultBaseURL+"/v3/combine-transactions/out-trade-no/combine-001/close")
}

func TestClientRoutesApplymentSettlementAndMerchantManagementEndpoints(t *testing.T) {
	fake := &captureSDKClient{responses: []string{
		`{"applyment_id":20000000011111}`,
		`{"business_code":"apply-001","applyment_id":20000000011111,"applyment_state":"APPLYMENT_STATE_FINISHED"}`,
		`{"business_code":"apply-001","applyment_id":20000000011111,"applyment_state":"APPLYMENT_STATE_AUDITING"}`,
		`{"account_type":"ACCOUNT_TYPE_BUSINESS","account_bank":"工商银行","account_number":"62********78","verify_result":"VERIFY_SUCCESS"}`,
		`{"application_no":"settle-app-001"}`,
		`{"verify_result":"AUDITING","verify_fail_reason":""}`,
		`{"applyment_id":20000000022222}`,
		`{}`,
		`{"applyment_state":"APPLYMENT_STATE_PASSED","qrcode_data":"base64-qrcode"}`,
		`{"authorize_state":"AUTHORIZE_STATE_AUTHORIZED"}`,
		`{"mchid":"1900000109","limited_functions":["NO_REFUND"],"recovery_specifications":[{"recover_way":"VERIFY_INACTIVE_MERCHANT_IDENTITY"}]}`,
		`{"notify_url":"https://api.example.com/wechat/ordinary/merchant-violation"}`,
		`{"notify_url":"https://api.example.com/wechat/ordinary/merchant-violation"}`,
		`{"notify_url":"https://api.example.com/wechat/ordinary/merchant-violation"}`,
		`{}`,
		`{"verification_id":"verify-001"}`,
		`{"sub_mchid":"1900000109","verification_id":"verify-001","state":"SUCCESS"}`,
	}}
	client := endpointTestClient(t, fake)
	ctx := context.Background()

	applymentResp, err := client.SubmitApplyment(ctx, validEndpointApplymentRequest())
	require.NoError(t, err)
	require.Equal(t, int64(20000000011111), applymentResp.ApplymentID)

	_, err = client.QueryApplymentByID(ctx, contracts.ApplymentQueryByIDRequest{ApplymentID: 20000000011111})
	require.NoError(t, err)

	_, err = client.QueryApplymentByBusinessCode(ctx, contracts.ApplymentQueryByBusinessCodeRequest{BusinessCode: "apply-001"})
	require.NoError(t, err)

	_, err = client.QuerySettlement(ctx, contracts.SettlementQueryRequest{SubMchID: "1900000109"})
	require.NoError(t, err)

	modifyResp, err := client.ModifySettlement(ctx, contracts.SettlementModifyRequest{SubMchID: "1900000109", AccountType: contracts.BankAccountTypeBusiness, AccountBank: "工商银行", AccountNumber: "encrypted-account-number"})
	require.NoError(t, err)
	require.Equal(t, "settle-app-001", modifyResp.ApplicationNo)

	_, err = client.QuerySettlementModification(ctx, contracts.SettlementModificationQueryRequest{SubMchID: "1900000109", ApplicationNo: "settle-app-001"})
	require.NoError(t, err)

	_, err = client.SubmitAccountWillingness(ctx, contracts.AccountWillingnessSubmitRequest{BusinessCode: "will-001", ContactInfo: "encrypted-contact"})
	require.NoError(t, err)

	_, err = client.CancelAccountWillingness(ctx, contracts.AccountWillingnessCancelRequest{BusinessCode: "will-001"})
	require.NoError(t, err)

	_, err = client.QueryAccountWillingness(ctx, contracts.AccountWillingnessQueryRequest{BusinessCode: "will-001"})
	require.NoError(t, err)

	authorizeResp, err := client.QueryAccountAuthorizeState(ctx, contracts.AccountAuthorizeStateRequest{SubMchID: "1900000109"})
	require.NoError(t, err)
	require.True(t, authorizeResp.Authorized)
	require.Equal(t, contracts.AccountAuthorizeStateAuthorized, authorizeResp.AuthorizeState)

	_, err = client.QueryMerchantLimitation(ctx, contracts.MerchantLimitationQueryRequest{SubMchID: "1900000109"})
	require.NoError(t, err)

	_, err = client.CreateViolationNotificationConfig(ctx, contracts.ViolationNotificationConfigRequest{NotifyURL: "https://api.example.com/wechat/ordinary/merchant-violation"})
	require.NoError(t, err)

	_, err = client.QueryViolationNotificationConfig(ctx)
	require.NoError(t, err)

	_, err = client.UpdateViolationNotificationConfig(ctx, contracts.ViolationNotificationConfigRequest{NotifyURL: "https://api.example.com/wechat/ordinary/merchant-violation"})
	require.NoError(t, err)

	err = client.DeleteViolationNotificationConfig(ctx)
	require.NoError(t, err)

	createVerificationResp, err := client.CreateInactiveMerchantIdentityVerification(ctx, contracts.InactiveMerchantIdentityVerificationCreateRequest{SubMchID: "1900000109"})
	require.NoError(t, err)
	require.Equal(t, "verify-001", createVerificationResp.VerificationID)

	_, err = client.QueryInactiveMerchantIdentityVerification(ctx, contracts.InactiveMerchantIdentityVerificationQueryRequest{SubMchID: "1900000109", VerificationID: "verify-001"})
	require.NoError(t, err)

	requireEndpointCall(t, fake.calls[0], http.MethodPost, DefaultBaseURL+"/v3/applyment4sub/applyment/")
	requireEndpointCall(t, fake.calls[1], http.MethodGet, DefaultBaseURL+"/v3/applyment4sub/applyment/applyment_id/20000000011111")
	requireEndpointCall(t, fake.calls[2], http.MethodGet, DefaultBaseURL+"/v3/applyment4sub/applyment/business_code/apply-001")
	requireEndpointCall(t, fake.calls[3], http.MethodGet, DefaultBaseURL+"/v3/apply4sub/sub_merchants/1900000109/settlement")
	requireEndpointCall(t, fake.calls[4], http.MethodPost, DefaultBaseURL+"/v3/apply4sub/sub_merchants/1900000109/modify-settlement")
	requireEndpointCall(t, fake.calls[5], http.MethodGet, DefaultBaseURL+"/v3/apply4sub/sub_merchants/1900000109/application/settle-app-001")
	requireEndpointCall(t, fake.calls[6], http.MethodPost, DefaultBaseURL+"/v3/apply4subject/applyment/")
	requireEndpointCall(t, fake.calls[7], http.MethodPost, DefaultBaseURL+"/v3/apply4subject/applyment/will-001/cancel")
	requireEndpointCall(t, fake.calls[8], http.MethodGet, DefaultBaseURL+"/v3/apply4subject/applyment")
	require.Equal(t, "will-001", fake.calls[8].query.Get("business_code"))
	requireEndpointCall(t, fake.calls[9], http.MethodGet, DefaultBaseURL+"/v3/apply4subject/applyment/merchants/1900000109/state")
	requireEndpointCall(t, fake.calls[10], http.MethodGet, DefaultBaseURL+"/v3/mch-operation-manage/merchant-limitations/sub-mchid/1900000109")
	requireEndpointCall(t, fake.calls[11], http.MethodPost, DefaultBaseURL+"/v3/merchant-risk-manage/violation-notifications")
	requireEndpointCall(t, fake.calls[12], http.MethodGet, DefaultBaseURL+"/v3/merchant-risk-manage/violation-notifications")
	requireEndpointCall(t, fake.calls[13], http.MethodPut, DefaultBaseURL+"/v3/merchant-risk-manage/violation-notifications")
	requireEndpointCall(t, fake.calls[14], http.MethodDelete, DefaultBaseURL+"/v3/merchant-risk-manage/violation-notifications")
	requireEndpointCall(t, fake.calls[15], http.MethodPost, DefaultBaseURL+"/v3/compliance/inactive-merchant-identity-verification/merchants")
	requireEndpointCall(t, fake.calls[16], http.MethodGet, DefaultBaseURL+"/v3/compliance/inactive-merchant-identity-verification/merchants/1900000109/verifications/verify-001")
}

func TestClientValidationFailureReturnsProviderErrorBeforeRequest(t *testing.T) {
	fake := &captureSDKClient{}
	client := endpointTestClient(t, fake)

	_, err := client.CreatePayment(context.Background(), contracts.PaymentPrepayRequest{})

	var providerErr *ProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, ErrorCategoryValidation, providerErr.Category)
	require.Equal(t, "WECHAT_REQUEST_INVALID", providerErr.Frontend.Code)
	require.Empty(t, fake.calls)
}

func endpointTestClient(t *testing.T, sdk sdkClient) *Client {
	t.Helper()
	cfg := validConfig(t).Normalized()
	cfg.ServiceProviderMchID = "1900000001"
	return &Client{config: cfg, sdk: sdk}
}

func requireEndpointCall(t *testing.T, call capturedSDKRequest, method, path string) {
	t.Helper()
	require.Equal(t, method, call.method)
	require.Equal(t, path, call.path)
}

func validEndpointPaymentRequest() contracts.PaymentPrepayRequest {
	return contracts.PaymentPrepayRequest{
		SpAppID:     "wx-sp-appid",
		SpMchID:     "1900000001",
		SubAppID:    "wx-sub-appid",
		SubMchID:    "1900000109",
		Description: "本地生活订单",
		OutTradeNo:  "order-001",
		NotifyURL:   "https://example.test/wechat/pay",
		SettleInfo:  &contracts.PaymentSettleInfo{ProfitSharing: true},
		Amount:      contracts.PaymentAmount{Total: 100, Currency: contracts.CurrencyCNY},
		Payer:       contracts.PaymentPayer{SubOpenID: "sub-openid"},
	}
}

func validEndpointCombineRequest() contracts.CombinePrepayRequest {
	return contracts.CombinePrepayRequest{
		CombineAppID:      "wx-combine-appid",
		CombineMchID:      "1900000001",
		CombineOutTradeNo: "combine-001",
		NotifyURL:         "https://example.test/wechat/combine",
		CombinePayerInfo:  contracts.CombinePayerInfo{OpenID: "sp-openid"},
		SubOrders: []contracts.CombineSubOrder{{
			MchID:       "1900000001",
			SubMchID:    "1900000109",
			OutTradeNo:  "order-001",
			Attach:      "local-life-order-001",
			Description: "本地生活合单子单",
			Amount:      contracts.CombineAmount{TotalAmount: 100, Currency: contracts.CurrencyCNY},
		}},
	}
}

func validEndpointRefundRequest() contracts.RefundCreateRequest {
	return contracts.RefundCreateRequest{
		SubMchID:      "1900000109",
		TransactionID: "tx-001",
		OutRefundNo:   "refund-001",
		NotifyURL:     "https://example.test/wechat/refund",
		Amount:        contracts.RefundAmountRequest{Refund: 100, Total: 100, Currency: contracts.CurrencyCNY},
	}
}

func validEndpointReceiverAddRequest() contracts.ProfitSharingReceiverAddRequest {
	return contracts.ProfitSharingReceiverAddRequest{
		SubMchID:     "1900000109",
		AppID:        "wx-sp-appid",
		Type:         contracts.ReceiverTypeMerchantID,
		Account:      "1900000200",
		Name:         "encrypted-name",
		RelationType: contracts.ProfitSharingRelationServiceProvider,
	}
}

func validEndpointProfitSharingOrderRequest() contracts.ProfitSharingOrderRequest {
	return contracts.ProfitSharingOrderRequest{
		SubMchID:        "1900000109",
		TransactionID:   "tx-001",
		OutOrderNo:      "ps-001",
		UnfreezeUnsplit: true,
		Receivers: []contracts.ProfitSharingReceiver{{
			Type:        contracts.ReceiverTypeMerchantID,
			Account:     "1900000200",
			Name:        "encrypted-name",
			Amount:      100,
			Description: "平台服务费分账",
		}},
	}
}

func validEndpointApplymentRequest() contracts.ApplymentSubmitRequest {
	return contracts.ApplymentSubmitRequest{
		BusinessCode: "apply-001",
		ContactInfo:  contracts.ApplymentContactInfo{ContactType: contracts.ContactTypeLegal, ContactName: "encrypted-name", MobilePhone: "encrypted-mobile", ContactEmail: "contact@example.test"},
		SubjectInfo: contracts.ApplymentSubjectInfo{
			SubjectType:         contracts.SubjectTypeEnterprise,
			BusinessLicenseInfo: &contracts.ApplymentBusinessLicenseInfo{LicenseCopy: "media-license", LicenseNumber: "91310000MA1K000000", MerchantName: "本地生活测试商户", LegalPerson: "张三"},
			IdentityInfo:        contracts.ApplymentIdentityInfo{IDDocType: contracts.IdentificationTypeIDCard, IDCardInfo: &contracts.ApplymentIDCardInfo{IDCardCopy: "media-front", IDCardNational: "media-back"}},
		},
		BusinessInfo:    contracts.ApplymentBusinessInfo{MerchantShortname: "本地生活", ServicePhone: "07550000000", SalesInfo: contracts.ApplymentSalesInfo{SalesScenesType: []contracts.ApplymentSalesSceneType{contracts.SalesSceneMiniProgram}, MiniProgramInfo: &contracts.ApplymentMiniProgramInfo{MiniProgramAppID: "wx-sp-appid"}}},
		SettlementInfo:  contracts.ApplymentSettlementInfo{SettlementID: "719", QualificationType: "餐饮"},
		BankAccountInfo: contracts.ApplymentBankAccountInfo{BankAccountType: contracts.BankAccountTypeCorporate, AccountName: "encrypted-account-name", AccountBank: "工商银行", AccountNumber: "encrypted-account-number"},
	}
}
