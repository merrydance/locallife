package ordinaryserviceprovider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/merrydance/locallife/wechat/ordinaryserviceprovider/contracts"
)

const (
	operationSubmitApplyment                    = "submit ordinary service provider applyment"
	operationQueryApplymentByID                 = "query ordinary service provider applyment by id"
	operationQueryApplymentByBusinessCode       = "query ordinary service provider applyment by business code"
	operationQuerySettlement                    = "query ordinary service provider settlement"
	operationModifySettlement                   = "modify ordinary service provider settlement"
	operationQuerySettlementModification        = "query ordinary service provider settlement modification"
	operationSubmitAccountWillingness           = "submit ordinary service provider account willingness"
	operationCancelAccountWillingness           = "cancel ordinary service provider account willingness"
	operationQueryAccountWillingness            = "query ordinary service provider account willingness"
	operationQueryAccountAuthorizeState         = "query ordinary service provider account authorize state"
	operationQueryMerchantLimitation            = "query ordinary service provider merchant limitation"
	operationCreateViolationNotificationConfig  = "create ordinary service provider violation notification config"
	operationQueryViolationNotificationConfig   = "query ordinary service provider violation notification config"
	operationUpdateViolationNotificationConfig  = "update ordinary service provider violation notification config"
	operationDeleteViolationNotificationConfig  = "delete ordinary service provider violation notification config"
	operationCreateInactiveMerchantVerification = "create ordinary service provider inactive merchant identity verification"
	operationQueryInactiveMerchantVerification  = "query ordinary service provider inactive merchant identity verification"
	operationCreatePayment                      = "create ordinary service provider payment"
	operationQueryPayment                       = "query ordinary service provider payment"
	operationClosePayment                       = "close ordinary service provider payment"
	operationCreateCombinePayment               = "create ordinary service provider combine payment"
	operationQueryCombinePayment                = "query ordinary service provider combine payment"
	operationCloseCombinePayment                = "close ordinary service provider combine payment"
	operationCreateRefund                       = "create ordinary service provider refund"
	operationQueryRefund                        = "query ordinary service provider refund"
	operationAddProfitSharingReceiver           = "add ordinary service provider profit sharing receiver"
	operationDeleteProfitSharingReceiver        = "delete ordinary service provider profit sharing receiver"
	operationCreateProfitSharingOrder           = "create ordinary service provider profit sharing order"
	operationQueryProfitSharingOrder            = "query ordinary service provider profit sharing order"
	operationCreateProfitSharingReturn          = "create ordinary service provider profit sharing return"
	operationQueryProfitSharingReturn           = "query ordinary service provider profit sharing return"
	operationUnfreezeProfitSharing              = "unfreeze ordinary service provider profit sharing order"
	operationQueryProfitSharingRemainingAmount  = "query ordinary service provider profit sharing remaining amount"
)

func (c *Client) SubmitApplyment(ctx context.Context, req contracts.ApplymentSubmitRequest) (*contracts.ApplymentSubmitResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, validationProviderError(operationSubmitApplyment, err)
	}
	response := &contracts.ApplymentSubmitResponse{}
	if err := c.requestJSON(ctx, operationSubmitApplyment, http.MethodPost, "/v3/applyment4sub/applyment/", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryApplymentByID(ctx context.Context, req contracts.ApplymentQueryByIDRequest) (*contracts.ApplymentQueryResponse, error) {
	if req.ApplymentID <= 0 {
		return nil, requiredProviderFieldError(operationQueryApplymentByID, "applyment_id")
	}
	response := &contracts.ApplymentQueryResponse{}
	path := "/v3/applyment4sub/applyment/applyment_id/" + url.PathEscape(strconv.FormatInt(req.ApplymentID, 10))
	if err := c.requestJSON(ctx, operationQueryApplymentByID, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryApplymentByBusinessCode(ctx context.Context, req contracts.ApplymentQueryByBusinessCodeRequest) (*contracts.ApplymentQueryResponse, error) {
	if strings.TrimSpace(req.BusinessCode) == "" {
		return nil, requiredProviderFieldError(operationQueryApplymentByBusinessCode, "business_code")
	}
	response := &contracts.ApplymentQueryResponse{}
	path := "/v3/applyment4sub/applyment/business_code/" + url.PathEscape(req.BusinessCode)
	if err := c.requestJSON(ctx, operationQueryApplymentByBusinessCode, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QuerySettlement(ctx context.Context, req contracts.SettlementQueryRequest) (*contracts.SettlementQueryResponse, error) {
	if strings.TrimSpace(req.SubMchID) == "" {
		return nil, requiredProviderFieldError(operationQuerySettlement, "sub_mchid")
	}
	query := url.Values{}
	if req.AccountNumberRule != "" {
		query.Set("account_number_rule", string(req.AccountNumberRule))
	}
	response := &contracts.SettlementQueryResponse{}
	path := "/v3/apply4sub/sub_merchants/" + url.PathEscape(req.SubMchID) + "/settlement"
	if err := c.requestJSON(ctx, operationQuerySettlement, http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) ModifySettlement(ctx context.Context, req contracts.SettlementModifyRequest) (*contracts.SettlementModifyResponse, error) {
	if strings.TrimSpace(req.SubMchID) == "" {
		return nil, requiredProviderFieldError(operationModifySettlement, "sub_mchid")
	}
	if strings.TrimSpace(string(req.AccountType)) == "" {
		return nil, requiredProviderFieldError(operationModifySettlement, "account_type")
	}
	if strings.TrimSpace(req.AccountBank) == "" {
		return nil, requiredProviderFieldError(operationModifySettlement, "account_bank")
	}
	if strings.TrimSpace(req.AccountNumber) == "" {
		return nil, requiredProviderFieldError(operationModifySettlement, "account_number")
	}
	response := &contracts.SettlementModifyResponse{}
	path := "/v3/apply4sub/sub_merchants/" + url.PathEscape(req.SubMchID) + "/modify-settlement"
	if err := c.requestJSON(ctx, operationModifySettlement, http.MethodPost, path, nil, req, response); err != nil {
		return nil, err
	}
	if response.ApplicationNo == "" && response.ApplicationID != "" {
		response.ApplicationNo = response.ApplicationID
	}
	return response, nil
}

func (c *Client) QuerySettlementModification(ctx context.Context, req contracts.SettlementModificationQueryRequest) (*contracts.SettlementModificationQueryResponse, error) {
	if strings.TrimSpace(req.SubMchID) == "" {
		return nil, requiredProviderFieldError(operationQuerySettlementModification, "sub_mchid")
	}
	applicationNo := strings.TrimSpace(req.ApplicationNo)
	if applicationNo == "" {
		applicationNo = strings.TrimSpace(req.ApplicationID)
	}
	if applicationNo == "" {
		return nil, requiredProviderFieldError(operationQuerySettlementModification, "application_no")
	}
	query := url.Values{}
	if req.AccountNumberRule != "" {
		query.Set("account_number_rule", string(req.AccountNumberRule))
	}
	response := &contracts.SettlementModificationQueryResponse{}
	path := "/v3/apply4sub/sub_merchants/" + url.PathEscape(req.SubMchID) + "/application/" + url.PathEscape(applicationNo)
	if err := c.requestJSON(ctx, operationQuerySettlementModification, http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	if response.ApplicationNo == "" {
		response.ApplicationNo = applicationNo
	}
	return response, nil
}

func (c *Client) SubmitAccountWillingness(ctx context.Context, req contracts.AccountWillingnessSubmitRequest) (*contracts.AccountWillingnessSubmitResponse, error) {
	if strings.TrimSpace(req.BusinessCode) == "" {
		return nil, requiredProviderFieldError(operationSubmitAccountWillingness, "business_code")
	}
	response := &contracts.AccountWillingnessSubmitResponse{}
	if err := c.requestJSON(ctx, operationSubmitAccountWillingness, http.MethodPost, "/v3/apply4subject/applyment/", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) CancelAccountWillingness(ctx context.Context, req contracts.AccountWillingnessCancelRequest) (*contracts.AccountWillingnessCancelResponse, error) {
	if strings.TrimSpace(req.BusinessCode) == "" {
		return nil, requiredProviderFieldError(operationCancelAccountWillingness, "business_code")
	}
	response := &contracts.AccountWillingnessCancelResponse{}
	path := "/v3/apply4subject/applyment/" + url.PathEscape(req.BusinessCode) + "/cancel"
	if err := c.requestJSON(ctx, operationCancelAccountWillingness, http.MethodPost, path, nil, nil, response); err != nil {
		return nil, err
	}
	if response.BusinessCode == "" {
		response.BusinessCode = req.BusinessCode
	}
	return response, nil
}

func (c *Client) QueryAccountWillingness(ctx context.Context, req contracts.AccountWillingnessQueryRequest) (*contracts.AccountWillingnessQueryResponse, error) {
	if strings.TrimSpace(req.BusinessCode) == "" {
		return nil, requiredProviderFieldError(operationQueryAccountWillingness, "business_code")
	}
	query := url.Values{"business_code": []string{req.BusinessCode}}
	response := &contracts.AccountWillingnessQueryResponse{}
	if err := c.requestJSON(ctx, operationQueryAccountWillingness, http.MethodGet, "/v3/apply4subject/applyment", query, nil, response); err != nil {
		return nil, err
	}
	if response.State == "" {
		response.State = response.ApplymentState
	}
	return response, nil
}

func (c *Client) QueryAccountAuthorizeState(ctx context.Context, req contracts.AccountAuthorizeStateRequest) (*contracts.AccountAuthorizeStateResponse, error) {
	if strings.TrimSpace(req.SubMchID) == "" {
		return nil, requiredProviderFieldError(operationQueryAccountAuthorizeState, "sub_mchid")
	}
	response := &contracts.AccountAuthorizeStateResponse{}
	path := "/v3/apply4subject/applyment/merchants/" + url.PathEscape(req.SubMchID) + "/state"
	if err := c.requestJSON(ctx, operationQueryAccountAuthorizeState, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	response.SubMchID = req.SubMchID
	response.Authorized = response.AuthorizeState == contracts.AccountAuthorizeStateAuthorized
	return response, nil
}

func (c *Client) QueryMerchantLimitation(ctx context.Context, req contracts.MerchantLimitationQueryRequest) (*contracts.MerchantLimitationQueryResponse, error) {
	if strings.TrimSpace(req.SubMchID) == "" {
		return nil, requiredProviderFieldError(operationQueryMerchantLimitation, "sub_mchid")
	}
	response := &contracts.MerchantLimitationQueryResponse{}
	path := "/v3/mch-operation-manage/merchant-limitations/sub-mchid/" + url.PathEscape(req.SubMchID)
	if err := c.requestJSON(ctx, operationQueryMerchantLimitation, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	response.SubMchID = req.SubMchID
	return response, nil
}

func (c *Client) CreateViolationNotificationConfig(ctx context.Context, req contracts.ViolationNotificationConfigRequest) (*contracts.ViolationNotificationConfigResponse, error) {
	if err := validateNotificationURL(operationCreateViolationNotificationConfig, req.NotifyURL); err != nil {
		return nil, err
	}
	response := &contracts.ViolationNotificationConfigResponse{}
	if err := c.requestJSON(ctx, operationCreateViolationNotificationConfig, http.MethodPost, "/v3/merchant-risk-manage/violation-notifications", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryViolationNotificationConfig(ctx context.Context) (*contracts.ViolationNotificationConfigResponse, error) {
	response := &contracts.ViolationNotificationConfigResponse{}
	if err := c.requestJSON(ctx, operationQueryViolationNotificationConfig, http.MethodGet, "/v3/merchant-risk-manage/violation-notifications", nil, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) UpdateViolationNotificationConfig(ctx context.Context, req contracts.ViolationNotificationConfigRequest) (*contracts.ViolationNotificationConfigResponse, error) {
	if err := validateNotificationURL(operationUpdateViolationNotificationConfig, req.NotifyURL); err != nil {
		return nil, err
	}
	response := &contracts.ViolationNotificationConfigResponse{}
	if err := c.requestJSON(ctx, operationUpdateViolationNotificationConfig, http.MethodPut, "/v3/merchant-risk-manage/violation-notifications", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) DeleteViolationNotificationConfig(ctx context.Context) error {
	return c.requestNoBody(ctx, operationDeleteViolationNotificationConfig, http.MethodDelete, "/v3/merchant-risk-manage/violation-notifications", nil)
}

func (c *Client) CreateInactiveMerchantIdentityVerification(ctx context.Context, req contracts.InactiveMerchantIdentityVerificationCreateRequest) (*contracts.InactiveMerchantIdentityVerificationCreateResponse, error) {
	if strings.TrimSpace(req.SubMchID) == "" {
		return nil, requiredProviderFieldError(operationCreateInactiveMerchantVerification, "sub_mchid")
	}
	response := &contracts.InactiveMerchantIdentityVerificationCreateResponse{}
	if err := c.requestJSON(ctx, operationCreateInactiveMerchantVerification, http.MethodPost, "/v3/compliance/inactive-merchant-identity-verification/merchants", nil, req, response); err != nil {
		return nil, err
	}
	if response.VerifyID == "" {
		response.VerifyID = response.VerificationID
	}
	return response, nil
}

func (c *Client) QueryInactiveMerchantIdentityVerification(ctx context.Context, req contracts.InactiveMerchantIdentityVerificationQueryRequest) (*contracts.InactiveMerchantIdentityVerificationQueryResponse, error) {
	if strings.TrimSpace(req.SubMchID) == "" {
		return nil, requiredProviderFieldError(operationQueryInactiveMerchantVerification, "sub_mchid")
	}
	verificationID := strings.TrimSpace(req.VerificationID)
	if verificationID == "" {
		verificationID = strings.TrimSpace(req.VerifyID)
	}
	if verificationID == "" {
		return nil, requiredProviderFieldError(operationQueryInactiveMerchantVerification, "verification_id")
	}
	response := &contracts.InactiveMerchantIdentityVerificationQueryResponse{}
	path := "/v3/compliance/inactive-merchant-identity-verification/merchants/" + url.PathEscape(req.SubMchID) + "/verifications/" + url.PathEscape(verificationID)
	if err := c.requestJSON(ctx, operationQueryInactiveMerchantVerification, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	if response.VerifyID == "" {
		response.VerifyID = response.VerificationID
	}
	return response, nil
}

func (c *Client) CreatePayment(ctx context.Context, req contracts.PaymentPrepayRequest) (*contracts.PaymentPrepayResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, validationProviderError(operationCreatePayment, err)
	}
	response := &contracts.PaymentPrepayResponse{}
	if err := c.requestJSON(ctx, operationCreatePayment, http.MethodPost, "/v3/pay/partner/transactions/jsapi", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryPayment(ctx context.Context, req contracts.PaymentQueryRequest) (*contracts.PaymentQueryResponse, error) {
	if strings.TrimSpace(req.SubMchID) == "" {
		return nil, requiredProviderFieldError(operationQueryPayment, "sub_mchid")
	}
	query := url.Values{"sp_mchid": []string{c.config.ServiceProviderMchID}, "sub_mchid": []string{req.SubMchID}}
	response := &contracts.PaymentQueryResponse{}
	if strings.TrimSpace(req.TransactionID) != "" {
		path := "/v3/pay/partner/transactions/id/" + url.PathEscape(req.TransactionID)
		if err := c.requestJSON(ctx, operationQueryPayment, http.MethodGet, path, query, nil, response); err != nil {
			return nil, err
		}
		return response, nil
	}
	if strings.TrimSpace(req.OutTradeNo) == "" {
		return nil, validationProviderError(operationQueryPayment, errors.New("transaction_id or out_trade_no is required"))
	}
	path := "/v3/pay/partner/transactions/out-trade-no/" + url.PathEscape(req.OutTradeNo)
	if err := c.requestJSON(ctx, operationQueryPayment, http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) ClosePayment(ctx context.Context, req contracts.PaymentCloseRequest) error {
	if strings.TrimSpace(req.OutTradeNo) == "" {
		return requiredProviderFieldError(operationClosePayment, "out_trade_no")
	}
	if strings.TrimSpace(req.SpMchID) == "" {
		return requiredProviderFieldError(operationClosePayment, "sp_mchid")
	}
	if strings.TrimSpace(req.SubMchID) == "" {
		return requiredProviderFieldError(operationClosePayment, "sub_mchid")
	}
	requestBody := struct {
		SpMchID  string `json:"sp_mchid,omitempty"`
		SubMchID string `json:"sub_mchid,omitempty"`
	}{SpMchID: req.SpMchID, SubMchID: req.SubMchID}
	path := "/v3/pay/partner/transactions/out-trade-no/" + url.PathEscape(req.OutTradeNo) + "/close"
	return c.requestJSON(ctx, operationClosePayment, http.MethodPost, path, nil, requestBody, nil)
}

func (c *Client) CreateCombinePayment(ctx context.Context, req contracts.CombinePrepayRequest) (*contracts.CombinePrepayResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, validationProviderError(operationCreateCombinePayment, err)
	}
	response := &contracts.CombinePrepayResponse{}
	if err := c.requestJSON(ctx, operationCreateCombinePayment, http.MethodPost, "/v3/combine-transactions/jsapi", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryCombinePayment(ctx context.Context, req contracts.CombineQueryRequest) (*contracts.CombineQueryResponse, error) {
	if strings.TrimSpace(req.CombineOutTradeNo) == "" {
		return nil, requiredProviderFieldError(operationQueryCombinePayment, "combine_out_trade_no")
	}
	response := &contracts.CombineQueryResponse{}
	path := "/v3/combine-transactions/out-trade-no/" + url.PathEscape(req.CombineOutTradeNo)
	if err := c.requestJSON(ctx, operationQueryCombinePayment, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) CloseCombinePayment(ctx context.Context, req contracts.CombineCloseRequest) error {
	if err := validateCombineCloseRequest(req); err != nil {
		return validationProviderError(operationCloseCombinePayment, err)
	}
	requestBody := struct {
		CombineAppID string                           `json:"combine_appid,omitempty"`
		SubOrders    []contracts.CombineCloseSubOrder `json:"sub_orders,omitempty"`
	}{CombineAppID: req.CombineAppID, SubOrders: req.SubOrders}
	path := "/v3/combine-transactions/out-trade-no/" + url.PathEscape(req.CombineOutTradeNo) + "/close"
	return c.requestJSON(ctx, operationCloseCombinePayment, http.MethodPost, path, nil, requestBody, nil)
}

func (c *Client) CreateRefund(ctx context.Context, req contracts.RefundCreateRequest) (*contracts.RefundResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, validationProviderError(operationCreateRefund, err)
	}
	response := &contracts.RefundResponse{}
	if err := c.requestJSON(ctx, operationCreateRefund, http.MethodPost, "/v3/refund/domestic/refunds", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryRefund(ctx context.Context, req contracts.RefundQueryRequest) (*contracts.RefundResponse, error) {
	if strings.TrimSpace(req.SubMchID) == "" {
		return nil, requiredProviderFieldError(operationQueryRefund, "sub_mchid")
	}
	if strings.TrimSpace(req.OutRefundNo) == "" {
		return nil, requiredProviderFieldError(operationQueryRefund, "out_refund_no")
	}
	query := url.Values{"sub_mchid": []string{req.SubMchID}}
	response := &contracts.RefundResponse{}
	path := "/v3/refund/domestic/refunds/" + url.PathEscape(req.OutRefundNo)
	if err := c.requestJSON(ctx, operationQueryRefund, http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) AddProfitSharingReceiver(ctx context.Context, req contracts.ProfitSharingReceiverAddRequest) (*contracts.ProfitSharingReceiverResponse, error) {
	if err := validateProfitSharingReceiver(operationAddProfitSharingReceiver, req.SubMchID, string(req.Type), req.Account); err != nil {
		return nil, err
	}
	response := &contracts.ProfitSharingReceiverResponse{}
	if err := c.requestJSON(ctx, operationAddProfitSharingReceiver, http.MethodPost, "/v3/profitsharing/receivers/add", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) DeleteProfitSharingReceiver(ctx context.Context, req contracts.ProfitSharingReceiverDeleteRequest) (*contracts.ProfitSharingReceiverResponse, error) {
	if err := validateProfitSharingReceiver(operationDeleteProfitSharingReceiver, req.SubMchID, string(req.Type), req.Account); err != nil {
		return nil, err
	}
	response := &contracts.ProfitSharingReceiverResponse{}
	if err := c.requestJSON(ctx, operationDeleteProfitSharingReceiver, http.MethodPost, "/v3/profitsharing/receivers/delete", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) CreateProfitSharingOrder(ctx context.Context, req contracts.ProfitSharingOrderRequest) (*contracts.ProfitSharingOrderResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, validationProviderError(operationCreateProfitSharingOrder, err)
	}
	response := &contracts.ProfitSharingOrderResponse{}
	if err := c.requestJSON(ctx, operationCreateProfitSharingOrder, http.MethodPost, "/v3/profitsharing/orders", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryProfitSharingOrder(ctx context.Context, req contracts.ProfitSharingQueryRequest) (*contracts.ProfitSharingOrderResponse, error) {
	if strings.TrimSpace(req.SubMchID) == "" {
		return nil, requiredProviderFieldError(operationQueryProfitSharingOrder, "sub_mchid")
	}
	if strings.TrimSpace(req.TransactionID) == "" {
		return nil, requiredProviderFieldError(operationQueryProfitSharingOrder, "transaction_id")
	}
	if strings.TrimSpace(req.OutOrderNo) == "" {
		return nil, requiredProviderFieldError(operationQueryProfitSharingOrder, "out_order_no")
	}
	query := url.Values{"sub_mchid": []string{req.SubMchID}, "transaction_id": []string{req.TransactionID}}
	response := &contracts.ProfitSharingOrderResponse{}
	path := "/v3/profitsharing/orders/" + url.PathEscape(req.OutOrderNo)
	if err := c.requestJSON(ctx, operationQueryProfitSharingOrder, http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) CreateProfitSharingReturn(ctx context.Context, req contracts.ProfitSharingReturnRequest) (*contracts.ProfitSharingReturnResponse, error) {
	if err := validateProfitSharingReturnRequest(req); err != nil {
		return nil, validationProviderError(operationCreateProfitSharingReturn, err)
	}
	response := &contracts.ProfitSharingReturnResponse{}
	if err := c.requestJSON(ctx, operationCreateProfitSharingReturn, http.MethodPost, "/v3/profitsharing/return-orders", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryProfitSharingReturn(ctx context.Context, req contracts.ProfitSharingReturnQueryRequest) (*contracts.ProfitSharingReturnResponse, error) {
	if strings.TrimSpace(req.SubMchID) == "" {
		return nil, requiredProviderFieldError(operationQueryProfitSharingReturn, "sub_mchid")
	}
	if strings.TrimSpace(req.OutOrderNo) == "" {
		return nil, requiredProviderFieldError(operationQueryProfitSharingReturn, "out_order_no")
	}
	if strings.TrimSpace(req.OutReturnNo) == "" {
		return nil, requiredProviderFieldError(operationQueryProfitSharingReturn, "out_return_no")
	}
	query := url.Values{"sub_mchid": []string{req.SubMchID}, "out_order_no": []string{req.OutOrderNo}}
	response := &contracts.ProfitSharingReturnResponse{}
	path := "/v3/profitsharing/return-orders/" + url.PathEscape(req.OutReturnNo)
	if err := c.requestJSON(ctx, operationQueryProfitSharingReturn, http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) UnfreezeProfitSharing(ctx context.Context, req contracts.ProfitSharingUnfreezeRequest) (*contracts.ProfitSharingUnfreezeResponse, error) {
	if err := validateProfitSharingUnfreezeRequest(req); err != nil {
		return nil, validationProviderError(operationUnfreezeProfitSharing, err)
	}
	response := &contracts.ProfitSharingUnfreezeResponse{}
	if err := c.requestJSON(ctx, operationUnfreezeProfitSharing, http.MethodPost, "/v3/profitsharing/orders/unfreeze", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryProfitSharingRemainingAmount(ctx context.Context, req contracts.ProfitSharingRemainingAmountRequest) (*contracts.ProfitSharingRemainingAmountResponse, error) {
	if strings.TrimSpace(req.TransactionID) == "" {
		return nil, requiredProviderFieldError(operationQueryProfitSharingRemainingAmount, "transaction_id")
	}
	response := &contracts.ProfitSharingRemainingAmountResponse{}
	path := "/v3/profitsharing/transactions/" + url.PathEscape(req.TransactionID) + "/amounts"
	if err := c.requestJSON(ctx, operationQueryProfitSharingRemainingAmount, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func validateNotificationURL(operation, notifyURL string) error {
	parsedURL, err := url.ParseRequestURI(strings.TrimSpace(notifyURL))
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return validationProviderError(operation, fmt.Errorf("notify_url must be a valid https URL"))
	}
	return nil
}

func validateProfitSharingReceiver(operation, subMchID, receiverType, account string) error {
	if strings.TrimSpace(subMchID) == "" {
		return requiredProviderFieldError(operation, "sub_mchid")
	}
	if strings.TrimSpace(receiverType) == "" {
		return requiredProviderFieldError(operation, "type")
	}
	if strings.TrimSpace(account) == "" {
		return requiredProviderFieldError(operation, "account")
	}
	return nil
}

func validateCombineCloseRequest(req contracts.CombineCloseRequest) error {
	if strings.TrimSpace(req.CombineOutTradeNo) == "" {
		return errors.New("combine_out_trade_no is required")
	}
	if strings.TrimSpace(req.CombineAppID) == "" {
		return errors.New("combine_appid is required")
	}
	if len(req.SubOrders) == 0 {
		return errors.New("sub_orders is required")
	}
	for index, subOrder := range req.SubOrders {
		if strings.TrimSpace(subOrder.MchID) == "" {
			return fmt.Errorf("sub_orders[%d].mchid is required", index)
		}
		if strings.TrimSpace(subOrder.OutTradeNo) == "" {
			return fmt.Errorf("sub_orders[%d].out_trade_no is required", index)
		}
		if strings.TrimSpace(subOrder.SubMchID) == "" {
			return fmt.Errorf("sub_orders[%d].sub_mchid is required", index)
		}
	}
	return nil
}

func validateProfitSharingReturnRequest(req contracts.ProfitSharingReturnRequest) error {
	if strings.TrimSpace(req.SubMchID) == "" {
		return errors.New("sub_mchid is required")
	}
	if strings.TrimSpace(req.OutReturnNo) == "" {
		return errors.New("out_return_no is required")
	}
	if strings.TrimSpace(req.ReturnMchID) == "" {
		return errors.New("return_mchid is required")
	}
	if strings.TrimSpace(req.OutOrderNo) == "" && strings.TrimSpace(req.OrderID) == "" {
		return errors.New("out_order_no or order_id is required")
	}
	if req.Amount <= 0 {
		return errors.New("amount must be positive")
	}
	return nil
}

func validateProfitSharingUnfreezeRequest(req contracts.ProfitSharingUnfreezeRequest) error {
	if strings.TrimSpace(req.SubMchID) == "" {
		return errors.New("sub_mchid is required")
	}
	if strings.TrimSpace(req.TransactionID) == "" {
		return errors.New("transaction_id is required")
	}
	if strings.TrimSpace(req.OutOrderNo) == "" {
		return errors.New("out_order_no is required")
	}
	if strings.TrimSpace(req.Description) == "" {
		return errors.New("description is required")
	}
	return nil
}

func requiredProviderFieldError(operation, field string) error {
	return validationProviderError(operation, fmt.Errorf("%s is required", field))
}

func validationProviderError(operation string, cause error) error {
	if cause == nil {
		cause = errors.New("ordinary service provider request validation failed")
	}
	return &ProviderError{
		Operation:       strings.TrimSpace(operation),
		ProviderCode:    "LOCAL_VALIDATION_ERROR",
		ProviderMessage: cause.Error(),
		Category:        ErrorCategoryValidation,
		Frontend:        frontendGuidanceForCategory(ErrorCategoryValidation),
		cause:           cause,
	}
}
