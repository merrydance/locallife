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
	if err := validateClientEndpointRequest(contracts.EndpointApplymentSubmit, req); err != nil {
		return nil, err
	}
	response := &contracts.ApplymentSubmitResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointApplymentSubmit, operationSubmitApplyment, http.MethodPost, "/v3/applyment4sub/applyment/", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryApplymentByID(ctx context.Context, req contracts.ApplymentQueryByIDRequest) (*contracts.ApplymentQueryResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointApplymentQueryByID, req); err != nil {
		return nil, err
	}
	response := &contracts.ApplymentQueryResponse{}
	path := "/v3/applyment4sub/applyment/applyment_id/" + url.PathEscape(strconv.FormatInt(req.ApplymentID, 10))
	if err := c.requestEndpointJSON(ctx, contracts.EndpointApplymentQueryByID, operationQueryApplymentByID, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryApplymentByBusinessCode(ctx context.Context, req contracts.ApplymentQueryByBusinessCodeRequest) (*contracts.ApplymentQueryResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointApplymentQueryByBusinessCode, req); err != nil {
		return nil, err
	}
	response := &contracts.ApplymentQueryResponse{}
	path := "/v3/applyment4sub/applyment/business_code/" + url.PathEscape(req.BusinessCode)
	if err := c.requestEndpointJSON(ctx, contracts.EndpointApplymentQueryByBusinessCode, operationQueryApplymentByBusinessCode, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QuerySettlement(ctx context.Context, req contracts.SettlementQueryRequest) (*contracts.SettlementQueryResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointSettlementQuery, req); err != nil {
		return nil, err
	}
	query := url.Values{}
	if req.AccountNumberRule != "" {
		query.Set("account_number_rule", string(req.AccountNumberRule))
	}
	response := &contracts.SettlementQueryResponse{}
	path := "/v3/apply4sub/sub_merchants/" + url.PathEscape(req.SubMchID) + "/settlement"
	if err := c.requestEndpointJSON(ctx, contracts.EndpointSettlementQuery, operationQuerySettlement, http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) ModifySettlement(ctx context.Context, req contracts.SettlementModifyRequest) (*contracts.SettlementModifyResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointSettlementModify, req); err != nil {
		return nil, err
	}
	response := &contracts.SettlementModifyResponse{}
	path := "/v3/apply4sub/sub_merchants/" + url.PathEscape(req.SubMchID) + "/modify-settlement"
	if err := c.requestEndpointJSON(ctx, contracts.EndpointSettlementModify, operationModifySettlement, http.MethodPost, path, nil, req, response); err != nil {
		return nil, err
	}
	if response.ApplicationNo == "" && response.ApplicationID != "" {
		response.ApplicationNo = response.ApplicationID
	}
	return response, nil
}

func (c *Client) QuerySettlementModification(ctx context.Context, req contracts.SettlementModificationQueryRequest) (*contracts.SettlementModificationQueryResponse, error) {
	applicationNo := strings.TrimSpace(req.ApplicationNo)
	if applicationNo == "" {
		applicationNo = strings.TrimSpace(req.ApplicationID)
	}
	normalizedReq := req
	normalizedReq.ApplicationNo = applicationNo
	if err := validateClientEndpointRequest(contracts.EndpointSettlementModificationQuery, normalizedReq); err != nil {
		return nil, err
	}
	query := url.Values{}
	if req.AccountNumberRule != "" {
		query.Set("account_number_rule", string(req.AccountNumberRule))
	}
	response := &contracts.SettlementModificationQueryResponse{}
	path := "/v3/apply4sub/sub_merchants/" + url.PathEscape(req.SubMchID) + "/application/" + url.PathEscape(applicationNo)
	if err := c.requestEndpointJSON(ctx, contracts.EndpointSettlementModificationQuery, operationQuerySettlementModification, http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	if response.ApplicationNo == "" {
		response.ApplicationNo = applicationNo
	}
	return response, nil
}

func (c *Client) SubmitAccountWillingness(ctx context.Context, req contracts.AccountWillingnessSubmitRequest) (*contracts.AccountWillingnessSubmitResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointAccountWillingnessSubmit, req); err != nil {
		return nil, err
	}
	response := &contracts.AccountWillingnessSubmitResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointAccountWillingnessSubmit, operationSubmitAccountWillingness, http.MethodPost, "/v3/apply4subject/applyment/", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) CancelAccountWillingness(ctx context.Context, req contracts.AccountWillingnessCancelRequest) (*contracts.AccountWillingnessCancelResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointAccountWillingnessCancel, req); err != nil {
		return nil, err
	}
	response := &contracts.AccountWillingnessCancelResponse{}
	path := "/v3/apply4subject/applyment/" + url.PathEscape(req.BusinessCode) + "/cancel"
	if err := c.requestEndpointJSON(ctx, contracts.EndpointAccountWillingnessCancel, operationCancelAccountWillingness, http.MethodPost, path, nil, nil, response); err != nil {
		return nil, err
	}
	if response.BusinessCode == "" {
		response.BusinessCode = req.BusinessCode
	}
	return response, nil
}

func (c *Client) QueryAccountWillingness(ctx context.Context, req contracts.AccountWillingnessQueryRequest) (*contracts.AccountWillingnessQueryResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointAccountWillingnessQuery, req); err != nil {
		return nil, err
	}
	query := url.Values{"business_code": []string{req.BusinessCode}}
	response := &contracts.AccountWillingnessQueryResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointAccountWillingnessQuery, operationQueryAccountWillingness, http.MethodGet, "/v3/apply4subject/applyment", query, nil, response); err != nil {
		return nil, err
	}
	if response.State == "" {
		response.State = response.ApplymentState
	}
	return response, nil
}

func (c *Client) QueryAccountAuthorizeState(ctx context.Context, req contracts.AccountAuthorizeStateRequest) (*contracts.AccountAuthorizeStateResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointAccountAuthorizeState, req); err != nil {
		return nil, err
	}
	response := &contracts.AccountAuthorizeStateResponse{}
	path := "/v3/apply4subject/applyment/merchants/" + url.PathEscape(req.SubMchID) + "/state"
	if err := c.requestEndpointJSON(ctx, contracts.EndpointAccountAuthorizeState, operationQueryAccountAuthorizeState, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	response.SubMchID = req.SubMchID
	response.Authorized = response.AuthorizeState == contracts.AccountAuthorizeStateAuthorized
	return response, nil
}

func (c *Client) QueryMerchantLimitation(ctx context.Context, req contracts.MerchantLimitationQueryRequest) (*contracts.MerchantLimitationQueryResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointMerchantLimitationQuery, req); err != nil {
		return nil, err
	}
	response := &contracts.MerchantLimitationQueryResponse{}
	path := "/v3/mch-operation-manage/merchant-limitations/sub-mchid/" + url.PathEscape(req.SubMchID)
	if err := c.requestEndpointJSON(ctx, contracts.EndpointMerchantLimitationQuery, operationQueryMerchantLimitation, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	response.SubMchID = req.SubMchID
	return response, nil
}

func (c *Client) CreateViolationNotificationConfig(ctx context.Context, req contracts.ViolationNotificationConfigRequest) (*contracts.ViolationNotificationConfigResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointViolationNotificationConfigCreate, req); err != nil {
		return nil, err
	}
	response := &contracts.ViolationNotificationConfigResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointViolationNotificationConfigCreate, operationCreateViolationNotificationConfig, http.MethodPost, "/v3/merchant-risk-manage/violation-notifications", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryViolationNotificationConfig(ctx context.Context) (*contracts.ViolationNotificationConfigResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointViolationNotificationConfigQuery, contracts.NoRequestBody{}); err != nil {
		return nil, err
	}
	response := &contracts.ViolationNotificationConfigResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointViolationNotificationConfigQuery, operationQueryViolationNotificationConfig, http.MethodGet, "/v3/merchant-risk-manage/violation-notifications", nil, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) UpdateViolationNotificationConfig(ctx context.Context, req contracts.ViolationNotificationConfigRequest) (*contracts.ViolationNotificationConfigResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointViolationNotificationConfigUpdate, req); err != nil {
		return nil, err
	}
	response := &contracts.ViolationNotificationConfigResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointViolationNotificationConfigUpdate, operationUpdateViolationNotificationConfig, http.MethodPut, "/v3/merchant-risk-manage/violation-notifications", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) DeleteViolationNotificationConfig(ctx context.Context) error {
	if err := validateClientEndpointRequest(contracts.EndpointViolationNotificationConfigDelete, contracts.NoRequestBody{}); err != nil {
		return err
	}
	return c.requestEndpointNoBody(ctx, contracts.EndpointViolationNotificationConfigDelete, operationDeleteViolationNotificationConfig, http.MethodDelete, "/v3/merchant-risk-manage/violation-notifications", nil)
}

func (c *Client) CreateInactiveMerchantIdentityVerification(ctx context.Context, req contracts.InactiveMerchantIdentityVerificationCreateRequest) (*contracts.InactiveMerchantIdentityVerificationCreateResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointInactiveMerchantVerificationCreate, req); err != nil {
		return nil, err
	}
	response := &contracts.InactiveMerchantIdentityVerificationCreateResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointInactiveMerchantVerificationCreate, operationCreateInactiveMerchantVerification, http.MethodPost, "/v3/compliance/inactive-merchant-identity-verification/merchants", nil, req, response); err != nil {
		return nil, err
	}
	if response.VerifyID == "" {
		response.VerifyID = response.VerificationID
	}
	return response, nil
}

func (c *Client) QueryInactiveMerchantIdentityVerification(ctx context.Context, req contracts.InactiveMerchantIdentityVerificationQueryRequest) (*contracts.InactiveMerchantIdentityVerificationQueryResponse, error) {
	verificationID := strings.TrimSpace(req.VerificationID)
	if verificationID == "" {
		verificationID = strings.TrimSpace(req.VerifyID)
	}
	normalizedReq := req
	normalizedReq.VerificationID = verificationID
	if err := validateClientEndpointRequest(contracts.EndpointInactiveMerchantVerificationQuery, normalizedReq); err != nil {
		return nil, err
	}
	response := &contracts.InactiveMerchantIdentityVerificationQueryResponse{}
	path := "/v3/compliance/inactive-merchant-identity-verification/merchants/" + url.PathEscape(req.SubMchID) + "/verifications/" + url.PathEscape(verificationID)
	if err := c.requestEndpointJSON(ctx, contracts.EndpointInactiveMerchantVerificationQuery, operationQueryInactiveMerchantVerification, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	if response.VerifyID == "" {
		response.VerifyID = response.VerificationID
	}
	return response, nil
}

func (c *Client) CreatePayment(ctx context.Context, req contracts.PaymentPrepayRequest) (*contracts.PaymentPrepayResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointPaymentPrepay, req); err != nil {
		return nil, err
	}
	response := &contracts.PaymentPrepayResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointPaymentPrepay, operationCreatePayment, http.MethodPost, "/v3/pay/partner/transactions/jsapi", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryPayment(ctx context.Context, req contracts.PaymentQueryRequest) (*contracts.PaymentQueryResponse, error) {
	query := url.Values{"sp_mchid": []string{c.config.ServiceProviderMchID}, "sub_mchid": []string{req.SubMchID}}
	response := &contracts.PaymentQueryResponse{}
	if strings.TrimSpace(req.TransactionID) != "" {
		if err := validateClientEndpointRequest(contracts.EndpointPaymentQueryByTransactionID, req); err != nil {
			return nil, err
		}
		path := "/v3/pay/partner/transactions/id/" + url.PathEscape(req.TransactionID)
		if err := c.requestEndpointJSON(ctx, contracts.EndpointPaymentQueryByTransactionID, operationQueryPayment, http.MethodGet, path, query, nil, response); err != nil {
			return nil, err
		}
		return response, nil
	}
	if err := validateClientEndpointRequest(contracts.EndpointPaymentQueryByOutTradeNo, req); err != nil {
		return nil, err
	}
	path := "/v3/pay/partner/transactions/out-trade-no/" + url.PathEscape(req.OutTradeNo)
	if err := c.requestEndpointJSON(ctx, contracts.EndpointPaymentQueryByOutTradeNo, operationQueryPayment, http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) ClosePayment(ctx context.Context, req contracts.PaymentCloseRequest) error {
	if err := validateClientEndpointRequest(contracts.EndpointPaymentClose, req); err != nil {
		return err
	}
	requestBody := struct {
		SpMchID  string `json:"sp_mchid,omitempty"`
		SubMchID string `json:"sub_mchid,omitempty"`
	}{SpMchID: req.SpMchID, SubMchID: req.SubMchID}
	path := "/v3/pay/partner/transactions/out-trade-no/" + url.PathEscape(req.OutTradeNo) + "/close"
	return c.requestEndpointJSON(ctx, contracts.EndpointPaymentClose, operationClosePayment, http.MethodPost, path, nil, requestBody, nil)
}

func (c *Client) CreateCombinePayment(ctx context.Context, req contracts.CombinePrepayRequest) (*contracts.CombinePrepayResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointCombinePrepay, req); err != nil {
		return nil, err
	}
	response := &contracts.CombinePrepayResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointCombinePrepay, operationCreateCombinePayment, http.MethodPost, "/v3/combine-transactions/jsapi", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryCombinePayment(ctx context.Context, req contracts.CombineQueryRequest) (*contracts.CombineQueryResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointCombineQuery, req); err != nil {
		return nil, err
	}
	response := &contracts.CombineQueryResponse{}
	path := "/v3/combine-transactions/out-trade-no/" + url.PathEscape(req.CombineOutTradeNo)
	if err := c.requestEndpointJSON(ctx, contracts.EndpointCombineQuery, operationQueryCombinePayment, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) CloseCombinePayment(ctx context.Context, req contracts.CombineCloseRequest) error {
	if err := validateClientEndpointRequest(contracts.EndpointCombineClose, req); err != nil {
		return err
	}
	requestBody := struct {
		CombineAppID string                           `json:"combine_appid,omitempty"`
		SubOrders    []contracts.CombineCloseSubOrder `json:"sub_orders,omitempty"`
	}{CombineAppID: req.CombineAppID, SubOrders: req.SubOrders}
	path := "/v3/combine-transactions/out-trade-no/" + url.PathEscape(req.CombineOutTradeNo) + "/close"
	return c.requestEndpointJSON(ctx, contracts.EndpointCombineClose, operationCloseCombinePayment, http.MethodPost, path, nil, requestBody, nil)
}

func (c *Client) CreateRefund(ctx context.Context, req contracts.RefundCreateRequest) (*contracts.RefundResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointRefundCreate, req); err != nil {
		return nil, err
	}
	response := &contracts.RefundResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointRefundCreate, operationCreateRefund, http.MethodPost, "/v3/refund/domestic/refunds", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryRefund(ctx context.Context, req contracts.RefundQueryRequest) (*contracts.RefundResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointRefundQuery, req); err != nil {
		return nil, err
	}
	query := url.Values{"sub_mchid": []string{req.SubMchID}}
	response := &contracts.RefundResponse{}
	path := "/v3/refund/domestic/refunds/" + url.PathEscape(req.OutRefundNo)
	if err := c.requestEndpointJSON(ctx, contracts.EndpointRefundQuery, operationQueryRefund, http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) AddProfitSharingReceiver(ctx context.Context, req contracts.ProfitSharingReceiverAddRequest) (*contracts.ProfitSharingReceiverResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointProfitSharingReceiverAdd, req); err != nil {
		return nil, err
	}
	response := &contracts.ProfitSharingReceiverResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointProfitSharingReceiverAdd, operationAddProfitSharingReceiver, http.MethodPost, "/v3/profitsharing/receivers/add", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) DeleteProfitSharingReceiver(ctx context.Context, req contracts.ProfitSharingReceiverDeleteRequest) (*contracts.ProfitSharingReceiverResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointProfitSharingReceiverDelete, req); err != nil {
		return nil, err
	}
	response := &contracts.ProfitSharingReceiverResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointProfitSharingReceiverDelete, operationDeleteProfitSharingReceiver, http.MethodPost, "/v3/profitsharing/receivers/delete", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) CreateProfitSharingOrder(ctx context.Context, req contracts.ProfitSharingOrderRequest) (*contracts.ProfitSharingOrderResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointProfitSharingCreate, req); err != nil {
		return nil, err
	}
	response := &contracts.ProfitSharingOrderResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointProfitSharingCreate, operationCreateProfitSharingOrder, http.MethodPost, "/v3/profitsharing/orders", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryProfitSharingOrder(ctx context.Context, req contracts.ProfitSharingQueryRequest) (*contracts.ProfitSharingOrderResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointProfitSharingQuery, req); err != nil {
		return nil, err
	}
	query := url.Values{"sub_mchid": []string{req.SubMchID}, "transaction_id": []string{req.TransactionID}}
	response := &contracts.ProfitSharingOrderResponse{}
	path := "/v3/profitsharing/orders/" + url.PathEscape(req.OutOrderNo)
	if err := c.requestEndpointJSON(ctx, contracts.EndpointProfitSharingQuery, operationQueryProfitSharingOrder, http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) CreateProfitSharingReturn(ctx context.Context, req contracts.ProfitSharingReturnRequest) (*contracts.ProfitSharingReturnResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointProfitSharingReturnCreate, req); err != nil {
		return nil, err
	}
	response := &contracts.ProfitSharingReturnResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointProfitSharingReturnCreate, operationCreateProfitSharingReturn, http.MethodPost, "/v3/profitsharing/return-orders", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryProfitSharingReturn(ctx context.Context, req contracts.ProfitSharingReturnQueryRequest) (*contracts.ProfitSharingReturnResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointProfitSharingReturnQuery, req); err != nil {
		return nil, err
	}
	query := url.Values{"sub_mchid": []string{req.SubMchID}, "out_order_no": []string{req.OutOrderNo}}
	response := &contracts.ProfitSharingReturnResponse{}
	path := "/v3/profitsharing/return-orders/" + url.PathEscape(req.OutReturnNo)
	if err := c.requestEndpointJSON(ctx, contracts.EndpointProfitSharingReturnQuery, operationQueryProfitSharingReturn, http.MethodGet, path, query, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) UnfreezeProfitSharing(ctx context.Context, req contracts.ProfitSharingUnfreezeRequest) (*contracts.ProfitSharingUnfreezeResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointProfitSharingUnfreeze, req); err != nil {
		return nil, err
	}
	response := &contracts.ProfitSharingUnfreezeResponse{}
	if err := c.requestEndpointJSON(ctx, contracts.EndpointProfitSharingUnfreeze, operationUnfreezeProfitSharing, http.MethodPost, "/v3/profitsharing/orders/unfreeze", nil, req, response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) QueryProfitSharingRemainingAmount(ctx context.Context, req contracts.ProfitSharingRemainingAmountRequest) (*contracts.ProfitSharingRemainingAmountResponse, error) {
	if err := validateClientEndpointRequest(contracts.EndpointProfitSharingRemainingAmount, req); err != nil {
		return nil, err
	}
	response := &contracts.ProfitSharingRemainingAmountResponse{}
	path := "/v3/profitsharing/transactions/" + url.PathEscape(req.TransactionID) + "/amounts"
	if err := c.requestEndpointJSON(ctx, contracts.EndpointProfitSharingRemainingAmount, operationQueryProfitSharingRemainingAmount, http.MethodGet, path, nil, nil, response); err != nil {
		return nil, err
	}
	return response, nil
}

func requiredProviderFieldError(operation, field string) error {
	return validationProviderError(operation, fmt.Errorf("%s is required", field))
}

func validationProviderError(operation string, cause error) error {
	return validationProviderEndpointError(operation, "", cause)
}

func validationProviderEndpointError(operation string, endpointID contracts.EndpointID, cause error) error {
	if cause == nil {
		cause = errors.New("ordinary service provider request validation failed")
	}
	return withEndpointMetadata(&ProviderError{
		Operation:       strings.TrimSpace(operation),
		ProviderCode:    "LOCAL_VALIDATION_ERROR",
		ProviderMessage: cause.Error(),
		Category:        ErrorCategoryValidation,
		Frontend:        frontendGuidanceForCategory(ErrorCategoryValidation),
		cause:           cause,
	}, endpointID)
}

func validateClientEndpointRequest(endpointID contracts.EndpointID, request any) error {
	contract, ok := contracts.EndpointContractByID(endpointID)
	operation := string(endpointID)
	if ok {
		operation = contract.Operation
	}
	if err := contracts.ValidateEndpointRequest(endpointID, request); err != nil {
		return validationProviderEndpointError(operation, endpointID, err)
	}
	return nil
}
