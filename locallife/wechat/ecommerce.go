package wechat

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// 微信支付平台收付通 API 端点
const (
	// 图片上传
	merchantMediaUploadURL = "/v3/merchant/media/upload"
	// 微信支付该接口的业务错误表明确限制图片不能超过 2MB。
	merchantMediaUploadMaxBytes = 2 * 1024 * 1024

	// 二级商户进件
	ecommerceApplymentsURL      = "/v3/ecommerce/applyments/"
	ecommerceApplymentQueryURL  = "/v3/ecommerce/applyments/%d"
	ecommerceApplymentQueryByNo = "/v3/ecommerce/applyments/out-request-no/%s"

	// 收付通进件辅助资料
	capitalPersonalBanksURL        = "/v3/capital/capitallhh/banks/personal-banking"
	capitalCorporateBanksURL       = "/v3/capital/capitallhh/banks/corporate-banking"
	capitalSearchBanksByAccountURL = "/v3/capital/capitallhh/banks/search-banks-by-bank-account"
	capitalProvincesURL            = "/v3/capital/capitallhh/areas/provinces"
	capitalProvinceCitiesURL       = "/v3/capital/capitallhh/areas/provinces/%d/cities"
	capitalBankBranchesURL         = "/v3/capital/capitallhh/banks/%s/branches"

	// 合单支付（收付通）
	ecommercePartnerJSAPIOrderURL        = "/v3/pay/partner/transactions/jsapi"
	ecommercePartnerQueryByIDURL         = "/v3/pay/partner/transactions/id/%s?sp_mchid=%s&sub_mchid=%s"
	ecommercePartnerQueryByOutTradeNoURL = "/v3/pay/partner/transactions/out-trade-no/%s?sp_mchid=%s&sub_mchid=%s"
	ecommercePartnerCloseOrderURL        = "/v3/pay/partner/transactions/out-trade-no/%s/close"
	ecommerceCombineOrderURL             = "/v3/combine-transactions/jsapi"
	ecommerceQueryCombineURL             = "/v3/combine-transactions/out-trade-no/%s"
	ecommerceCloseCombineURL             = "/v3/combine-transactions/out-trade-no/%s/close"

	// 分账
	profitSharingURL            = "/v3/ecommerce/profitsharing/orders"
	profitSharingAmountsURL     = "/v3/ecommerce/profitsharing/orders/%s/amounts"
	profitSharingFinishURL      = "/v3/ecommerce/profitsharing/finish-order"
	profitSharingReturnURL      = "/v3/ecommerce/profitsharing/returnorders"
	profitSharingReturnQueryURL = "/v3/ecommerce/profitsharing/returnorders"

	// 分账接收方
	profitSharingReceiverAddURL    = "/v3/ecommerce/profitsharing/receivers/add"
	profitSharingReceiverDeleteURL = "/v3/ecommerce/profitsharing/receivers/delete"

	// 退款（平台收付通）
	ecommerceRefundURL                 = "/v3/ecommerce/refunds/apply"
	ecommerceAbnormalRefundURL         = "/v3/ecommerce/refunds/%s/apply-abnormal-refund"
	ecommerceRefundQueryByIDURL        = "/v3/ecommerce/refunds/id/%s"
	ecommerceRefundQueryByOutRefundURL = "/v3/ecommerce/refunds/out-refund-no/%s"

	// 账户资金管理（平台收付通）
	ecommerceCancelWithdrawValidateURL  = "/v3/ecommerce/account/apply-cancel-withdraw/validate-cancel/%s"
	ecommerceCancelWithdrawApplyURL     = "/v3/ecommerce/account/apply-cancel-withdraw"
	ecommerceCancelWithdrawQueryByNoURL = "/v3/ecommerce/account/apply-cancel-withdraw/out-request-no/%s"
	ecommerceCancelWithdrawQueryByIDURL = "/v3/ecommerce/account/apply-cancel-withdraw/applyment-id/%s"
	ecommerceFundBalanceURL             = "/v3/ecommerce/fund/balance/%s"
	ecommerceFundDayEndBalanceURL       = "/v3/ecommerce/fund/enddaybalance/%s"
	platformFundBalanceURL              = "/v3/merchant/fund/balance/%s"
	platformFundDayEndBalanceURL        = "/v3/merchant/fund/dayendbalance/%s"
	ecommerceFundWithdrawURL            = "/v3/ecommerce/fund/withdraw"
	ecommerceFundWithdrawQueryByNo      = "/v3/ecommerce/fund/withdraw/out-request-no/%s"

	// 结算账户查询/修改/申请查询（apply4sub）
	apply4subSettlementURL            = "/v3/apply4sub/sub_merchants/%s/settlement"
	apply4subModifySettlementURL      = "/v3/apply4sub/sub_merchants/%s/modify-settlement"
	apply4subModifySettlementQueryURL = "/v3/apply4sub/sub_merchants/%s/application/%s"
)

// EcommerceClient 平台收付通客户端
// 用于多商户场景，支持分账功能
type EcommerceClient struct {
	*PaymentClient            // 复用基础支付客户端
	spMchID            string // 服务商商户号
	explicitSpMchID    bool
	spAppID            string // 服务商 AppID
	spMchName          string // 服务商名称（可选）
	partnerNotifyURL   string
	combineNotifyURL   string
	withdrawNotifyURL  string
	violationNotifyURL string
}

// ValidateEcommerceCancelWithdraw 校验二级商户是否满足注销提现条件
func (c *EcommerceClient) ValidateEcommerceCancelWithdraw(ctx context.Context, subMchID string) (*wechatcontracts.CancelWithdrawEligibilityResponse, error) {
	trimmedSubMchID, err := wechatcontracts.ValidateCancelWithdrawIdentifier("validate merchant cancel withdraw", "sub_mchid", subMchID)
	if err != nil {
		return nil, err
	}

	requestURL := fmt.Sprintf(ecommerceCancelWithdrawValidateURL, url.PathEscape(trimmedSubMchID))
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("validate ecommerce cancel withdraw: %w", err)
	}

	var resp wechatcontracts.CancelWithdrawEligibilityResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.SubMchID == "" {
		resp.SubMchID = trimmedSubMchID
	}
	if err := wechatcontracts.ValidateCancelWithdrawEligibilityResponse(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateEcommerceCancelWithdraw 提交商户注销提现申请
func (c *EcommerceClient) CreateEcommerceCancelWithdraw(ctx context.Context, req *wechatcontracts.CancelWithdrawRequest) (*wechatcontracts.CancelWithdrawCreateResponse, error) {
	if err := wechatcontracts.ValidateCancelWithdrawCreateRequest(req); err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"sub_mchid":      strings.TrimSpace(req.SubMchID),
		"out_request_no": strings.TrimSpace(req.OutRequestNo),
	}
	if req.Withdraw != "" {
		body["withdraw"] = req.Withdraw
	}
	if req.PayeeInfo != nil {
		body["payee_info"] = req.PayeeInfo
	}
	if len(req.ProofMedias) > 0 {
		body["proof_medias"] = req.ProofMedias
	}
	if len(req.AdditionalMaterials) > 0 {
		body["additional_materials"] = req.AdditionalMaterials
	}
	if strings.TrimSpace(req.Remark) != "" {
		body["remark"] = strings.TrimSpace(req.Remark)
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, ecommerceCancelWithdrawApplyURL, body)
	if err != nil {
		return nil, fmt.Errorf("create ecommerce cancel withdraw: %w", err)
	}

	var resp wechatcontracts.CancelWithdrawCreateResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.OutRequestNo == "" {
		resp.OutRequestNo = strings.TrimSpace(req.OutRequestNo)
	}
	return &resp, nil
}

// QueryEcommerceCancelWithdrawByOutRequestNo 按平台申请单号查询注销提现申请状态
func (c *EcommerceClient) QueryEcommerceCancelWithdrawByOutRequestNo(ctx context.Context, outRequestNo string) (*wechatcontracts.CancelWithdrawQueryResponse, error) {
	trimmedOutRequestNo, err := wechatcontracts.ValidateCancelWithdrawIdentifier("query merchant cancel withdraw by out_request_no", "out_request_no", outRequestNo)
	if err != nil {
		return nil, err
	}

	requestURL := fmt.Sprintf(ecommerceCancelWithdrawQueryByNoURL, url.PathEscape(trimmedOutRequestNo))
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query ecommerce cancel withdraw by out request no: %w", err)
	}

	var resp wechatcontracts.CancelWithdrawQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.OutRequestNo == "" {
		resp.OutRequestNo = trimmedOutRequestNo
	}
	if err := wechatcontracts.ValidateCancelWithdrawQueryResponse("query merchant cancel withdraw by out_request_no", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// QueryEcommerceCancelWithdrawByApplymentID 按微信申请单号查询注销提现申请状态
func (c *EcommerceClient) QueryEcommerceCancelWithdrawByApplymentID(ctx context.Context, applymentID string) (*wechatcontracts.CancelWithdrawQueryResponse, error) {
	trimmedApplymentID, err := wechatcontracts.ValidateCancelWithdrawIdentifier("query merchant cancel withdraw by applyment_id", "applyment_id", applymentID)
	if err != nil {
		return nil, err
	}

	requestURL := fmt.Sprintf(ecommerceCancelWithdrawQueryByIDURL, url.PathEscape(trimmedApplymentID))
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query ecommerce cancel withdraw by applyment id: %w", err)
	}

	var resp wechatcontracts.CancelWithdrawQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.ApplymentID == "" {
		resp.ApplymentID = trimmedApplymentID
	}
	if err := wechatcontracts.ValidateCancelWithdrawQueryResponse("query merchant cancel withdraw by applyment_id", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// EcommerceClientConfig 平台收付通客户端配置
type EcommerceClientConfig struct {
	PaymentClientConfig        // 嵌入基础配置
	SpMchID             string // 服务商商户号（UploadImage 要求显式配置；即使与 MchID 相同也应填写）
	SpAppID             string // 服务商 AppID（如与 AppID 相同可不填）
	SpMchName           string // 服务商名称（可选）
	PartnerNotifyURL    string // 收付通普通支付回调地址（空则回退到 PaymentClientConfig.NotifyURL）
	CombineNotifyURL    string // 收付通合单支付回调地址（空则回退到 PartnerNotifyURL / PaymentClientConfig.NotifyURL）
	WithdrawNotifyURL   string // 收付通提现回调地址（空则不为提现请求上送 notify_url）
	ViolationNotifyURL  string // 收付通商户违规通知回调地址（空则回退到 PartnerNotifyURL / PaymentClientConfig.NotifyURL）
}

// NewEcommerceClient 创建平台收付通客户端
func NewEcommerceClient(cfg EcommerceClientConfig) (*EcommerceClient, error) {
	baseClient, err := NewPaymentClient(cfg.PaymentClientConfig)
	if err != nil {
		return nil, fmt.Errorf("create base payment client: %w", err)
	}

	spMchID := strings.TrimSpace(cfg.SpMchID)
	explicitSpMchID := spMchID != ""
	if spMchID == "" {
		spMchID = strings.TrimSpace(cfg.MchID)
	}

	spAppID := strings.TrimSpace(cfg.SpAppID)
	if spAppID == "" {
		spAppID = strings.TrimSpace(cfg.AppID)
	}

	spMchName := strings.TrimSpace(cfg.SpMchName)
	partnerNotifyURL := strings.TrimSpace(cfg.PartnerNotifyURL)
	if partnerNotifyURL == "" {
		partnerNotifyURL = strings.TrimSpace(cfg.NotifyURL)
	}
	combineNotifyURL := strings.TrimSpace(cfg.CombineNotifyURL)
	if combineNotifyURL == "" {
		combineNotifyURL = partnerNotifyURL
	}
	withdrawNotifyURL := strings.TrimSpace(cfg.WithdrawNotifyURL)
	violationNotifyURL := strings.TrimSpace(cfg.ViolationNotifyURL)
	if violationNotifyURL == "" {
		violationNotifyURL = partnerNotifyURL
	}

	return &EcommerceClient{
		PaymentClient:      baseClient,
		spMchID:            spMchID,
		explicitSpMchID:    explicitSpMchID,
		spAppID:            spAppID,
		spMchName:          spMchName,
		partnerNotifyURL:   partnerNotifyURL,
		combineNotifyURL:   combineNotifyURL,
		withdrawNotifyURL:  withdrawNotifyURL,
		violationNotifyURL: violationNotifyURL,
	}, nil
}

// GetSpMchID 获取服务商商户号
func (c *EcommerceClient) GetSpMchID() string {
	return c.spMchID
}

// GetSpAppID 获取服务商AppID
func (c *EcommerceClient) GetSpAppID() string {
	return c.spAppID
}

// GetSpMchName 获取服务商名称
func (c *EcommerceClient) GetSpMchName() string {
	return c.spMchName
}

// CreatePartnerJSAPIOrder 创建服务商模式单笔 JSAPI 订单。
func (c *EcommerceClient) CreatePartnerJSAPIOrder(ctx context.Context, req *PartnerJSAPIOrderRequest) (*PartnerJSAPIOrderResponse, *JSAPIPayParams, error) {
	if req == nil {
		return nil, nil, fmt.Errorf("create partner jsapi order: request is nil")
	}
	if strings.TrimSpace(req.PayerSubOpenID) != "" || strings.TrimSpace(req.SubAppID) != "" {
		return nil, nil, fmt.Errorf("create partner jsapi order: sub_openid and sub_appid are not supported in the single-appid project flow")
	}
	if strings.TrimSpace(req.SubMchID) == "" {
		return nil, nil, fmt.Errorf("create partner jsapi order: sub_mchid is required")
	}
	if strings.TrimSpace(req.Description) == "" || strings.TrimSpace(req.OutTradeNo) == "" {
		return nil, nil, fmt.Errorf("create partner jsapi order: description and out_trade_no are required")
	}
	if req.TotalAmount <= 0 {
		return nil, nil, fmt.Errorf("create partner jsapi order: total amount must be positive")
	}
	if strings.TrimSpace(req.PayerOpenID) == "" && strings.TrimSpace(req.PayerSubOpenID) == "" {
		return nil, nil, fmt.Errorf("create partner jsapi order: sp_openid or sub_openid is required")
	}
	if (strings.TrimSpace(req.DeviceID) != "" || req.StoreInfo != nil) && strings.TrimSpace(req.PayerClientIP) == "" {
		return nil, nil, fmt.Errorf("create partner jsapi order: payer_client_ip is required when scene_info is provided")
	}
	if req.StoreInfo != nil && strings.TrimSpace(req.StoreInfo.ID) == "" {
		return nil, nil, fmt.Errorf("create partner jsapi order: scene_info.store_info.id is required when store_info is provided")
	}

	currency := req.Currency
	if currency == "" {
		currency = "CNY"
	}
	notifyURL := req.NotifyURL
	if notifyURL == "" {
		notifyURL = c.partnerNotifyURL
	}
	if strings.TrimSpace(notifyURL) == "" {
		return nil, nil, fmt.Errorf("create partner jsapi order: notify_url is required")
	}
	body := PartnerJSAPIOrderRequestBody{
		SpAppID:     c.spAppID,
		SpMchID:     c.spMchID,
		SubMchID:    req.SubMchID,
		Description: req.Description,
		OutTradeNo:  req.OutTradeNo,
		NotifyURL:   notifyURL,
		Amount: PartnerJSAPIAmount{
			Total:    req.TotalAmount,
			Currency: currency,
		},
		Payer: PartnerJSAPIPayer{
			SpOpenID: req.PayerOpenID,
		},
		SettleInfo: &PartnerOrderSettleInfo{ProfitSharing: req.ProfitSharing, SubsidyAmount: req.SubsidyAmount},
	}
	if req.Detail != nil {
		body.Detail = req.Detail
	}
	if !req.ExpireTime.IsZero() {
		body.TimeExpire = req.ExpireTime.Format(time.RFC3339)
	}
	if req.Attach != "" {
		body.Attach = req.Attach
	}
	if req.GoodsTag != "" {
		body.GoodsTag = req.GoodsTag
	}
	if req.SupportFapiao != nil {
		body.SupportFapiao = req.SupportFapiao
	}
	if req.PayerClientIP != "" || req.DeviceID != "" || req.StoreInfo != nil {
		body.SceneInfo = &PartnerOrderSceneInfo{}
		if req.PayerClientIP != "" {
			body.SceneInfo.PayerClientIP = req.PayerClientIP
		}
		if req.DeviceID != "" {
			body.SceneInfo.DeviceID = req.DeviceID
		}
		if req.StoreInfo != nil {
			body.SceneInfo.StoreInfo = req.StoreInfo
		}
	}

	respBody, requestID, err := c.doRequestWithRequestID(ctx, http.MethodPost, ecommercePartnerJSAPIOrderURL, body)
	if err != nil {
		wrappedErr := wrapPartnerJSAPIOrderCreateError(err)
		ecommercePaymentOrderLogEvent(requestID, "create_partner_jsapi_order").
			Str("sub_mchid", strings.TrimSpace(req.SubMchID)).
			Str("out_trade_no", strings.TrimSpace(req.OutTradeNo)).
			Int64("total_amount", req.TotalAmount).
			Err(wrappedErr).
			Msg("wechat partner jsapi order failed")
		return nil, nil, wrappedErr
	}

	var resp PartnerJSAPIOrderResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, nil, fmt.Errorf("unmarshal response: %w", err)
	}

	payParams, err := c.generateJSAPIPayParams(resp.PrepayID)
	if err != nil {
		return nil, nil, fmt.Errorf("generate pay params: %w", err)
	}

	return &resp, payParams, nil
}

// QueryPartnerOrderByTransactionID 通过微信支付订单号查询服务商模式单笔订单。
func (c *EcommerceClient) QueryPartnerOrderByTransactionID(ctx context.Context, transactionID, subMchID string) (*PartnerOrderQueryResponse, error) {
	if strings.TrimSpace(transactionID) == "" {
		return nil, newPartnerOrderQueryValidationError("query partner order by transaction_id", "transaction_id is required")
	}
	if strings.TrimSpace(subMchID) == "" {
		return nil, newPartnerOrderQueryValidationError("query partner order by transaction_id", "sub_mchid is required")
	}
	respBody, requestID, err := c.doRequestWithRequestID(ctx, http.MethodGet, fmt.Sprintf(ecommercePartnerQueryByIDURL, transactionID, c.spMchID, subMchID), nil)
	if err != nil {
		wrappedErr := wrapPartnerOrderQueryError(err)
		ecommercePaymentOrderLogEvent(requestID, "query_partner_order_by_transaction_id").
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("transaction_id", strings.TrimSpace(transactionID)).
			Err(wrappedErr).
			Msg("wechat partner order query by transaction id failed")
		return nil, wrappedErr
	}

	var resp PartnerOrderQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		contractErr := newPartnerOrderQueryContractError("query partner order by transaction_id", "unmarshal response: %v", err)
		ecommercePaymentOrderLogEvent(requestID, "query_partner_order_by_transaction_id").
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("transaction_id", strings.TrimSpace(transactionID)).
			Err(contractErr).
			Msg("wechat partner order query by transaction id response contract invalid")
		return nil, contractErr
	}
	if err := validatePartnerOrderQueryResponse("query partner order by transaction_id", &resp, true); err != nil {
		ecommercePaymentOrderLogEvent(requestID, "query_partner_order_by_transaction_id").
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("transaction_id", strings.TrimSpace(transactionID)).
			Err(err).
			Msg("wechat partner order query by transaction id response contract invalid")
		return nil, err
	}

	return &resp, nil
}

// QueryPartnerOrderByOutTradeNo 通过商户订单号查询服务商模式单笔订单。
func (c *EcommerceClient) QueryPartnerOrderByOutTradeNo(ctx context.Context, outTradeNo, subMchID string) (*PartnerOrderQueryResponse, error) {
	if strings.TrimSpace(outTradeNo) == "" {
		return nil, newPartnerOrderQueryValidationError("query partner order by out_trade_no", "out_trade_no is required")
	}
	if strings.TrimSpace(subMchID) == "" {
		return nil, newPartnerOrderQueryValidationError("query partner order by out_trade_no", "sub_mchid is required")
	}
	respBody, requestID, err := c.doRequestWithRequestID(ctx, http.MethodGet, fmt.Sprintf(ecommercePartnerQueryByOutTradeNoURL, outTradeNo, c.spMchID, subMchID), nil)
	if err != nil {
		wrappedErr := wrapPartnerOrderQueryError(err)
		ecommercePaymentOrderLogEvent(requestID, "query_partner_order_by_out_trade_no").
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("out_trade_no", strings.TrimSpace(outTradeNo)).
			Err(wrappedErr).
			Msg("wechat partner order query by out trade no failed")
		return nil, wrappedErr
	}

	var resp PartnerOrderQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		contractErr := newPartnerOrderQueryContractError("query partner order by out_trade_no", "unmarshal response: %v", err)
		ecommercePaymentOrderLogEvent(requestID, "query_partner_order_by_out_trade_no").
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("out_trade_no", strings.TrimSpace(outTradeNo)).
			Err(contractErr).
			Msg("wechat partner order query by out trade no response contract invalid")
		return nil, contractErr
	}
	if err := validatePartnerOrderQueryResponse("query partner order by out_trade_no", &resp, false); err != nil {
		ecommercePaymentOrderLogEvent(requestID, "query_partner_order_by_out_trade_no").
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("out_trade_no", strings.TrimSpace(outTradeNo)).
			Err(err).
			Msg("wechat partner order query by out trade no response contract invalid")
		return nil, err
	}

	return &resp, nil
}

// ClosePartnerOrder 关闭服务商模式单笔订单。
func (c *EcommerceClient) ClosePartnerOrder(ctx context.Context, outTradeNo, subMchID string) error {
	if strings.TrimSpace(outTradeNo) == "" {
		return fmt.Errorf("close partner order: out_trade_no is required")
	}
	if strings.TrimSpace(subMchID) == "" {
		return fmt.Errorf("close partner order: sub_mchid is required")
	}
	body := PartnerCloseOrderRequest{SpMchID: c.spMchID, SubMchID: subMchID}
	if _, requestID, err := c.doRequestWithRequestID(ctx, http.MethodPost, fmt.Sprintf(ecommercePartnerCloseOrderURL, outTradeNo), body); err != nil {
		wrappedErr := wrapPartnerOrderCloseError(err)
		ecommercePaymentOrderLogEvent(requestID, "close_partner_order").
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("out_trade_no", strings.TrimSpace(outTradeNo)).
			Err(wrappedErr).
			Msg("wechat partner order close failed")
		return wrappedErr
	}
	return nil
}

// ==================== 二级商户进件 ====================

// EcommerceApplymentRequest 二级商户进件申请请求
type EcommerceApplymentRequest struct {
	OutRequestNo         string                                        `json:"out_request_no"`                   // 业务申请编号
	OrganizationType     string                                        `json:"organization_type"`                // 主体类型: 2401-小微, 2500-个人卖家, 4-个体工商户, 2-企业, 3-事业单位, 2502-政府机关, 1708-社会组织
	FinanceInstitution   bool                                          `json:"finance_institution"`              // 是否金融机构
	BusinessLicense      *wechatcontracts.ApplymentBusinessLicenseInfo `json:"business_license_info,omitempty"`  // 营业执照信息（个体户/企业必填）
	IDCardInfo           *wechatcontracts.ApplymentIDCardInfo          `json:"id_card_info"`                     // 法人身份证信息
	AccountInfo          *wechatcontracts.ApplymentBankAccountInfo     `json:"account_info,omitempty"`           // 结算银行账户
	ContactInfo          *wechatcontracts.ApplymentContactInfo         `json:"contact_info"`                     // 联系人信息
	SalesSceneInfo       *wechatcontracts.ApplymentSalesSceneInfo      `json:"sales_scene_info"`                 // 经营场景信息
	SettlementInfo       *wechatcontracts.ApplymentSettlementInfo      `json:"settlement_info,omitempty"`        // 结算规则
	MerchantShortname    string                                        `json:"merchant_shortname"`               // 商户简称
	Qualifications       []string                                      `json:"qualifications,omitempty"`         // 特殊资质
	BusinessAdditionPics []string                                      `json:"business_addition_pics,omitempty"` // 补充材料
	BusinessAdditionDesc string                                        `json:"business_addition_desc,omitempty"` // 补充说明
}

// 公开 applyment/query/settlement 合同类型已迁移到 wechat/contracts 包。

func MarshalEcommerceApplymentAccountValidation(validation *wechatcontracts.EcommerceApplymentAccountValidation) []byte {
	if validation == nil {
		return nil
	}

	normalized := *validation
	if trimmedRawAccountName := strings.TrimSpace(normalized.RawAccountName); trimmedRawAccountName != "" {
		normalized.AccountName = trimmedRawAccountName
	} else {
		normalized.AccountName = strings.TrimSpace(normalized.AccountName)
	}
	if trimmedRawAccountNo := strings.TrimSpace(normalized.RawAccountNo); trimmedRawAccountNo != "" {
		normalized.AccountNo = trimmedRawAccountNo
	} else {
		normalized.AccountNo = strings.TrimSpace(normalized.AccountNo)
	}
	normalized.DestinationAccountNumber = strings.TrimSpace(normalized.DestinationAccountNumber)
	normalized.DestinationAccountName = strings.TrimSpace(normalized.DestinationAccountName)
	normalized.DestinationAccountBank = strings.TrimSpace(normalized.DestinationAccountBank)
	normalized.City = strings.TrimSpace(normalized.City)
	normalized.Remark = strings.TrimSpace(normalized.Remark)
	normalized.Deadline = strings.TrimSpace(normalized.Deadline)

	payload, err := json.Marshal(&normalized)
	if err != nil {
		return nil
	}

	return payload
}

func UnmarshalEcommerceApplymentAccountValidation(raw []byte) (*wechatcontracts.EcommerceApplymentAccountValidation, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var validation wechatcontracts.EcommerceApplymentAccountValidation
	if err := json.Unmarshal(raw, &validation); err != nil {
		return nil, err
	}

	return &validation, nil
}

const (
	ecommerceApplymentOutRequestNoMaxLength     = 124
	subMerchantSettlementMchIDLength            = 10
	subMerchantSettlementApplicationNoMaxLength = 64
	subMerchantSettlementFieldMaxLength         = 128
	subMerchantSettlementAccountNameMaxLength   = 1024
	subMerchantSettlementFailReasonMaxLength    = 1024
)

type PartnerOrderQueryValidationError struct {
	Message string
}

func (e *PartnerOrderQueryValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query partner order: validation failed"
	}
	return e.Message
}

type PartnerOrderQueryContractError struct {
	Message string
}

func (e *PartnerOrderQueryContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query partner order: upstream contract validation failed"
	}
	return e.Message
}

type CombineOrderQueryValidationError struct {
	Message string
}

func (e *CombineOrderQueryValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query combine order: validation failed"
	}
	return e.Message
}

type CombineOrderQueryContractError struct {
	Message string
}

func (e *CombineOrderQueryContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query combine order: upstream contract validation failed"
	}
	return e.Message
}

type ecommerceApplymentQueryKind string

const (
	ecommerceApplymentQueryByIDKind           ecommerceApplymentQueryKind = "applyment_id"
	ecommerceApplymentQueryByOutRequestNoKind ecommerceApplymentQueryKind = "out_request_no"
)

var allowedPartnerOrderTradeStates = map[string]struct{}{
	"SUCCESS":    {},
	"REFUND":     {},
	"NOTPAY":     {},
	"CLOSED":     {},
	"REVOKED":    {},
	"USERPAYING": {},
	"PAYERROR":   {},
}

var allowedCombineOrderTradeStates = map[string]struct{}{
	"SUCCESS":  {},
	"REFUND":   {},
	"NOTPAY":   {},
	"CLOSED":   {},
	"PAYERROR": {},
}

var allowedCombineOrderTradeTypes = map[string]struct{}{
	"NATIVE": {},
	"JSAPI":  {},
	"APP":    {},
	"MWEB":   {},
}

var allowedPartnerPromotionScopes = map[string]struct{}{
	"GLOBAL": {},
	"SINGLE": {},
}

var allowedPartnerPromotionTypes = map[string]struct{}{
	"CASH":   {},
	"NOCASH": {},
}

var allowedSubMerchantSettlementAccountNumberRules = map[string]struct{}{
	wechatcontracts.SubMerchantSettlementAccountNumberRuleMaskV1: {},
	wechatcontracts.SubMerchantSettlementAccountNumberRuleMaskV2: {},
}

func newPartnerOrderQueryValidationError(operation string, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "query partner order"
	}
	return &PartnerOrderQueryValidationError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func newPartnerOrderQueryContractError(operation string, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "query partner order"
	}
	return &PartnerOrderQueryContractError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func newCombineOrderQueryValidationError(operation string, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "query combine order"
	}
	return &CombineOrderQueryValidationError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func newCombineOrderQueryContractError(operation string, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "query combine order"
	}
	return &CombineOrderQueryContractError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func validatePartnerOrderQueryResponse(operation string, resp *PartnerOrderQueryResponse, requireTransactionFields bool) error {
	if resp == nil {
		return newPartnerOrderQueryContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.SpAppID) == "" {
		return newPartnerOrderQueryContractError(operation, "wechat response missing sp_appid")
	}
	if strings.TrimSpace(resp.SpMchID) == "" {
		return newPartnerOrderQueryContractError(operation, "wechat response missing sp_mchid")
	}
	if strings.TrimSpace(resp.SubMchID) == "" {
		return newPartnerOrderQueryContractError(operation, "wechat response missing sub_mchid")
	}
	if strings.TrimSpace(resp.OutTradeNo) == "" {
		return newPartnerOrderQueryContractError(operation, "wechat response missing out_trade_no")
	}
	if requireTransactionFields && strings.TrimSpace(resp.TransactionID) == "" {
		return newPartnerOrderQueryContractError(operation, "wechat response missing transaction_id")
	}
	if requireTransactionFields && strings.TrimSpace(resp.TradeType) == "" {
		return newPartnerOrderQueryContractError(operation, "wechat response missing trade_type")
	}
	if strings.TrimSpace(resp.TradeState) == "" {
		return newPartnerOrderQueryContractError(operation, "wechat response missing trade_state")
	}
	if _, ok := allowedPartnerOrderTradeStates[strings.ToUpper(strings.TrimSpace(resp.TradeState))]; !ok {
		return newPartnerOrderQueryContractError(operation, "unsupported trade_state %q", resp.TradeState)
	}
	if strings.TrimSpace(resp.TradeStateDesc) == "" {
		return newPartnerOrderQueryContractError(operation, "wechat response missing trade_state_desc")
	}
	if resp.SceneInfo != nil && strings.TrimSpace(resp.SceneInfo.DeviceID) == "" {
		return newPartnerOrderQueryContractError(operation, "scene_info.device_id is required when scene_info is provided")
	}
	for index, detail := range resp.PromotionDetail {
		if strings.TrimSpace(detail.CouponID) == "" {
			return newPartnerOrderQueryContractError(operation, "promotion_detail[%d].coupon_id is required", index)
		}
		if strings.TrimSpace(detail.Name) == "" {
			return newPartnerOrderQueryContractError(operation, "promotion_detail[%d].name is required", index)
		}
		if detail.Scope != "" {
			if _, ok := allowedPartnerPromotionScopes[strings.ToUpper(strings.TrimSpace(detail.Scope))]; !ok {
				return newPartnerOrderQueryContractError(operation, "promotion_detail[%d].scope has unsupported value %q", index, detail.Scope)
			}
		}
		if detail.Type != "" {
			if _, ok := allowedPartnerPromotionTypes[strings.ToUpper(strings.TrimSpace(detail.Type))]; !ok {
				return newPartnerOrderQueryContractError(operation, "promotion_detail[%d].type has unsupported value %q", index, detail.Type)
			}
		}
		for goodsIndex, goods := range detail.GoodsDetail {
			if strings.TrimSpace(goods.GoodsID) == "" {
				return newPartnerOrderQueryContractError(operation, "promotion_detail[%d].goods_detail[%d].goods_id is required", index, goodsIndex)
			}
		}
	}
	return nil
}

func validateCombineOrderQueryResponse(operation string, resp *CombineQueryResponseBody) error {
	if resp == nil {
		return newCombineOrderQueryContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.CombineAppID) == "" {
		return newCombineOrderQueryContractError(operation, "wechat response missing combine_appid")
	}
	if strings.TrimSpace(resp.CombineMchID) == "" {
		return newCombineOrderQueryContractError(operation, "wechat response missing combine_mchid")
	}
	if strings.TrimSpace(resp.CombineOutTradeNo) == "" {
		return newCombineOrderQueryContractError(operation, "wechat response missing combine_out_trade_no")
	}
	for index, subOrder := range resp.SubOrders {
		if strings.TrimSpace(subOrder.MchID) == "" {
			return newCombineOrderQueryContractError(operation, "sub_orders[%d].mchid is required", index)
		}
		if strings.TrimSpace(subOrder.OutTradeNo) == "" {
			return newCombineOrderQueryContractError(operation, "sub_orders[%d].out_trade_no is required", index)
		}
		if strings.TrimSpace(subOrder.TradeState) == "" {
			return newCombineOrderQueryContractError(operation, "sub_orders[%d].trade_state is required", index)
		}
		if _, ok := allowedCombineOrderTradeStates[strings.ToUpper(strings.TrimSpace(subOrder.TradeState))]; !ok {
			return newCombineOrderQueryContractError(operation, "sub_orders[%d].trade_state has unsupported value %q", index, subOrder.TradeState)
		}
		if strings.TrimSpace(subOrder.TradeType) != "" {
			if _, ok := allowedCombineOrderTradeTypes[strings.ToUpper(strings.TrimSpace(subOrder.TradeType))]; !ok {
				return newCombineOrderQueryContractError(operation, "sub_orders[%d].trade_type has unsupported value %q", index, subOrder.TradeType)
			}
		}
		if subOrder.Amount == nil {
			return newCombineOrderQueryContractError(operation, "sub_orders[%d].amount is required", index)
		}
		if strings.TrimSpace(subOrder.Amount.Currency) == "" {
			return newCombineOrderQueryContractError(operation, "sub_orders[%d].amount.currency is required", index)
		}
		if strings.TrimSpace(subOrder.Amount.PayerCurrency) == "" {
			return newCombineOrderQueryContractError(operation, "sub_orders[%d].amount.payer_currency is required", index)
		}
		for detailIndex, detail := range subOrder.PromotionDetail {
			if strings.TrimSpace(detail.CouponID) == "" {
				return newCombineOrderQueryContractError(operation, "sub_orders[%d].promotion_detail[%d].coupon_id is required", index, detailIndex)
			}
			if detail.Scope != "" {
				if _, ok := allowedPartnerPromotionScopes[strings.ToUpper(strings.TrimSpace(detail.Scope))]; !ok {
					return newCombineOrderQueryContractError(operation, "sub_orders[%d].promotion_detail[%d].scope has unsupported value %q", index, detailIndex, detail.Scope)
				}
			}
			if detail.Type != "" {
				if _, ok := allowedPartnerPromotionTypes[strings.ToUpper(strings.TrimSpace(detail.Type))]; !ok {
					return newCombineOrderQueryContractError(operation, "sub_orders[%d].promotion_detail[%d].type has unsupported value %q", index, detailIndex, detail.Type)
				}
			}
			for goodsIndex, goods := range detail.GoodsDetail {
				if strings.TrimSpace(goods.GoodsID) == "" {
					return newCombineOrderQueryContractError(operation, "sub_orders[%d].promotion_detail[%d].goods_detail[%d].goods_id is required", index, detailIndex, goodsIndex)
				}
			}
		}
	}
	return nil
}

func combineQueryResponseFromBody(resp *CombineQueryResponseBody) *CombineQueryResponse {
	if resp == nil {
		return nil
	}

	result := &CombineQueryResponse{
		CombineAppID:      resp.CombineAppID,
		CombineMchID:      resp.CombineMchID,
		CombineOutTradeNo: resp.CombineOutTradeNo,
		SceneInfo:         resp.SceneInfo,
	}
	if resp.CombinePayerInfo != nil {
		result.CombinePayerInfo = &CombinePayerInfo{
			OpenID:    resp.CombinePayerInfo.OpenID,
			SubOpenID: resp.CombinePayerInfo.SubOpenID,
		}
	}
	if len(resp.SubOrders) == 0 {
		return result
	}

	result.SubOrders = make([]CombineSubOrderResult, 0, len(resp.SubOrders))
	for _, subOrder := range resp.SubOrders {
		mapped := CombineSubOrderResult{
			MchID:           subOrder.MchID,
			SubMchID:        subOrder.SubMchID,
			SubAppID:        subOrder.SubAppID,
			SubOpenID:       subOrder.SubOpenID,
			OutTradeNo:      subOrder.OutTradeNo,
			TransactionID:   subOrder.TransactionID,
			TradeType:       subOrder.TradeType,
			TradeState:      subOrder.TradeState,
			TradeStateDesc:  subOrder.TradeStateDesc,
			BankType:        subOrder.BankType,
			Attach:          subOrder.Attach,
			PromotionDetail: subOrder.PromotionDetail,
			SuccessTime:     subOrder.SuccessTime,
		}
		if subOrder.Amount != nil {
			mapped.Amount = struct {
				TotalAmount    int64  `json:"total_amount"`
				PayerAmount    int64  `json:"payer_amount"`
				Currency       string `json:"currency"`
				PayerCurrency  string `json:"payer_currency"`
				SettlementRate int64  `json:"settlement_rate"`
			}{
				TotalAmount:    subOrder.Amount.TotalAmount,
				PayerAmount:    subOrder.Amount.PayerAmount,
				Currency:       subOrder.Amount.Currency,
				PayerCurrency:  subOrder.Amount.PayerCurrency,
				SettlementRate: subOrder.Amount.SettlementRate,
			}
		}
		result.SubOrders = append(result.SubOrders, mapped)
	}

	return result
}

func validateEcommerceApplymentID(applymentID int64) error {
	if applymentID <= 0 {
		return wechatcontracts.NewEcommerceApplymentQueryValidationError("applyment_id must be a positive integer")
	}
	return nil
}

func validateEcommerceApplymentCreateResponse(resp *wechatcontracts.EcommerceApplymentResponse) error {
	if resp == nil {
		return errors.New("create ecommerce applyment: response is nil")
	}
	if resp.ApplymentID <= 0 {
		return errors.New("create ecommerce applyment: applyment_id must be a positive integer")
	}
	return nil
}

func validateEcommerceApplymentOutRequestNo(outRequestNo string) (string, error) {
	normalized := strings.TrimSpace(outRequestNo)
	if normalized == "" {
		return "", wechatcontracts.NewEcommerceApplymentQueryValidationError("out_request_no is required")
	}
	if len(normalized) > ecommerceApplymentOutRequestNoMaxLength {
		return "", wechatcontracts.NewEcommerceApplymentQueryValidationError("out_request_no must not exceed %d characters", ecommerceApplymentOutRequestNoMaxLength)
	}
	return normalized, nil
}

func validateSubMerchantSettlementSubMchID(subMchID string) (string, error) {
	normalized := strings.TrimSpace(subMchID)
	if normalized == "" {
		return "", wechatcontracts.NewSubMerchantSettlementQueryValidationError("sub_mchid is required")
	}
	if len(normalized) != subMerchantSettlementMchIDLength {
		return "", wechatcontracts.NewSubMerchantSettlementQueryValidationError("sub_mchid must be exactly %d digits", subMerchantSettlementMchIDLength)
	}
	for _, ch := range normalized {
		if ch < '0' || ch > '9' {
			return "", wechatcontracts.NewSubMerchantSettlementQueryValidationError("sub_mchid must contain only digits")
		}
	}
	return normalized, nil
}

func validateSubMerchantSettlementAccountNumberRule(accountNumberRule string) (string, error) {
	normalized := strings.TrimSpace(accountNumberRule)
	if normalized == "" {
		return "", nil
	}
	if _, ok := allowedSubMerchantSettlementAccountNumberRules[normalized]; !ok {
		return "", wechatcontracts.NewSubMerchantSettlementQueryValidationError("account_number_rule must be one of %s or %s", wechatcontracts.SubMerchantSettlementAccountNumberRuleMaskV1, wechatcontracts.SubMerchantSettlementAccountNumberRuleMaskV2)
	}
	return normalized, nil
}

func validateSubMerchantSettlementApplicationNo(applicationNo string) (string, error) {
	normalized := strings.TrimSpace(applicationNo)
	if normalized == "" {
		return "", wechatcontracts.NewSubMerchantSettlementApplicationQueryValidationError("application_no is required")
	}
	if utf8.RuneCountInString(normalized) > subMerchantSettlementApplicationNoMaxLength {
		return "", wechatcontracts.NewSubMerchantSettlementApplicationQueryValidationError("application_no must not exceed %d characters", subMerchantSettlementApplicationNoMaxLength)
	}
	return normalized, nil
}

func validateSubMerchantSettlementFieldLength(fieldName, value string, maxRunes int) error {
	if utf8.RuneCountInString(value) > maxRunes {
		return wechatcontracts.NewSubMerchantSettlementContractError("response %s exceeds %d characters", fieldName, maxRunes)
	}
	return nil
}

func validateSubMerchantSettlementResponse(resp *wechatcontracts.SubMerchantSettlementResponse) error {
	resp.AccountType = strings.TrimSpace(resp.AccountType)
	resp.AccountBank = strings.TrimSpace(resp.AccountBank)
	resp.BankName = strings.TrimSpace(resp.BankName)
	resp.BankBranchID = strings.TrimSpace(resp.BankBranchID)
	resp.AccountNumber = strings.TrimSpace(resp.AccountNumber)
	resp.VerifyResult = strings.TrimSpace(resp.VerifyResult)
	resp.VerifyFailReason = strings.TrimSpace(resp.VerifyFailReason)

	if err := wechatcontracts.ValidateSubMerchantSettlementResponse(resp); err != nil {
		return err
	}

	if err := validateSubMerchantSettlementFieldLength("account_bank", resp.AccountBank, subMerchantSettlementFieldMaxLength); err != nil {
		return err
	}
	if err := validateSubMerchantSettlementFieldLength("bank_name", resp.BankName, subMerchantSettlementFieldMaxLength); err != nil {
		return err
	}
	if err := validateSubMerchantSettlementFieldLength("bank_branch_id", resp.BankBranchID, subMerchantSettlementFieldMaxLength); err != nil {
		return err
	}
	if err := validateSubMerchantSettlementFieldLength("account_number", resp.AccountNumber, subMerchantSettlementFieldMaxLength); err != nil {
		return err
	}
	if err := validateSubMerchantSettlementFieldLength("verify_fail_reason", resp.VerifyFailReason, subMerchantSettlementFailReasonMaxLength); err != nil {
		return err
	}

	return nil
}

func validateSubMerchantSettlementApplicationResponse(resp *wechatcontracts.QuerySubMerchantSettlementApplicationResponse) error {
	resp.AccountName = strings.TrimSpace(resp.AccountName)
	resp.AccountType = strings.TrimSpace(resp.AccountType)
	resp.AccountBank = strings.TrimSpace(resp.AccountBank)
	resp.BankName = strings.TrimSpace(resp.BankName)
	resp.BankBranchID = strings.TrimSpace(resp.BankBranchID)
	resp.AccountNumber = strings.TrimSpace(resp.AccountNumber)
	resp.VerifyResult = strings.TrimSpace(resp.VerifyResult)
	resp.VerifyFailReason = strings.TrimSpace(resp.VerifyFailReason)
	resp.VerifyFinishTime = strings.TrimSpace(resp.VerifyFinishTime)

	if err := wechatcontracts.ValidateSubMerchantSettlementApplicationResponse(resp); err != nil {
		return err
	}

	if utf8.RuneCountInString(resp.AccountName) > subMerchantSettlementAccountNameMaxLength {
		return wechatcontracts.NewSubMerchantSettlementApplicationContractError("response account_name exceeds %d characters", subMerchantSettlementAccountNameMaxLength)
	}
	if err := validateSubMerchantSettlementFieldLength("account_bank", resp.AccountBank, subMerchantSettlementFieldMaxLength); err != nil {
		return wechatcontracts.NewSubMerchantSettlementApplicationContractError("response account_bank exceeds %d characters", subMerchantSettlementFieldMaxLength)
	}
	if err := validateSubMerchantSettlementFieldLength("bank_name", resp.BankName, subMerchantSettlementFieldMaxLength); err != nil {
		return wechatcontracts.NewSubMerchantSettlementApplicationContractError("response bank_name exceeds %d characters", subMerchantSettlementFieldMaxLength)
	}
	if err := validateSubMerchantSettlementFieldLength("bank_branch_id", resp.BankBranchID, subMerchantSettlementFieldMaxLength); err != nil {
		return wechatcontracts.NewSubMerchantSettlementApplicationContractError("response bank_branch_id exceeds %d characters", subMerchantSettlementFieldMaxLength)
	}
	if err := validateSubMerchantSettlementFieldLength("account_number", resp.AccountNumber, subMerchantSettlementFieldMaxLength); err != nil {
		return wechatcontracts.NewSubMerchantSettlementApplicationContractError("response account_number exceeds %d characters", subMerchantSettlementFieldMaxLength)
	}
	if err := validateSubMerchantSettlementFieldLength("verify_fail_reason", resp.VerifyFailReason, subMerchantSettlementFailReasonMaxLength); err != nil {
		return wechatcontracts.NewSubMerchantSettlementApplicationContractError("response verify_fail_reason exceeds %d characters", subMerchantSettlementFailReasonMaxLength)
	}

	return nil
}

func subMerchantSettlementQueryWechatErrorGuide(wxErr *WechatPayError) string {
	if wxErr == nil {
		return "wechat sub merchant settlement query failed"
	}
	switch wxErr.Code {
	case "PARAM_ERROR":
		return "verify sub_mchid and account_number_rule, then retry"
	case "INVALID_REQUEST":
		return "verify the request URL, merchant configuration, and signing input, then retry"
	case "SIGN_ERROR":
		return "verify the merchant certificate, private key, and authorization signature, then retry"
	case "SYSTEM_ERROR":
		return "wechat settlement query failed due to an upstream system error; retry with backoff and escalate if it persists"
	case "RATELIMIT_EXCEEDED":
		return "wechat rate limited the settlement query; keep the query rate below 100 requests per second and retry later"
	default:
		return fmt.Sprintf("wechat sub merchant settlement query failed with upstream code %s; inspect the upstream message and request_id", wxErr.Code)
	}
}

func wrapSubMerchantSettlementQueryError(err error) error {
	var wxErr *WechatPayError
	if errors.As(err, &wxErr) {
		return fmt.Errorf("query sub merchant settlement: %s: %w", subMerchantSettlementQueryWechatErrorGuide(wxErr), err)
	}
	return fmt.Errorf("query sub merchant settlement: %w", err)
}

func subMerchantSettlementApplicationQueryWechatErrorGuide(wxErr *WechatPayError) string {
	if wxErr == nil {
		return "wechat settlement application query failed"
	}
	switch wxErr.Code {
	case "PARAM_ERROR":
		return "verify sub_mchid, application_no, and account_number_rule, then retry"
	case "INVALID_REQUEST":
		return "verify the request URL, merchant configuration, and signing input, then retry"
	case "NO_AUTH":
		return "verify that the sub-merchant belongs to the current service provider before retrying"
	case "SIGN_ERROR":
		return "verify the merchant certificate, private key, and authorization signature, then retry"
	case "ORDER_NOT_EXIST":
		return "verify application_no and sub_mchid, then retry"
	case "FREQENCY_LIMIT", "RATELIMIT_EXCEEDED":
		return "wechat rate limited the settlement application query; keep the query rate below 100 requests per second and retry later"
	case "SYSTEM_ERROR":
		return "wechat settlement application query failed due to an upstream system error; retry with backoff and escalate if it persists"
	default:
		return fmt.Sprintf("wechat settlement application query failed with upstream code %s; inspect the upstream message and request_id", wxErr.Code)
	}
}

func wrapSubMerchantSettlementApplicationQueryError(err error) error {
	var wxErr *WechatPayError
	if errors.As(err, &wxErr) {
		return fmt.Errorf("query sub merchant settlement application: %s: %w", subMerchantSettlementApplicationQueryWechatErrorGuide(wxErr), err)
	}
	return fmt.Errorf("query sub merchant settlement application: %w", err)
}

func querySubMerchantSettlementLogEvent(requestID, subMchID, accountNumberRule string) *zerolog.Event {
	return log.Error().
		Str("request_id", strings.TrimSpace(requestID)).
		Str("sub_mchid", strings.TrimSpace(subMchID)).
		Str("account_number_rule", strings.TrimSpace(accountNumberRule))
}

func querySubMerchantSettlementApplicationLogEvent(requestID, subMchID, applicationNo, accountNumberRule string) *zerolog.Event {
	return log.Error().
		Str("request_id", strings.TrimSpace(requestID)).
		Str("sub_mchid", strings.TrimSpace(subMchID)).
		Str("application_no", strings.TrimSpace(applicationNo)).
		Str("account_number_rule", strings.TrimSpace(accountNumberRule))
}

func validateEcommerceApplymentQueryResponse(resp *wechatcontracts.EcommerceApplymentQueryResponse, kind ecommerceApplymentQueryKind) error {
	switch kind {
	case ecommerceApplymentQueryByOutRequestNoKind:
		return wechatcontracts.ValidateEcommerceApplymentQueryByOutRequestNoResponse(resp)
	case ecommerceApplymentQueryByIDKind:
		return wechatcontracts.ValidateEcommerceApplymentQueryByIDResponse(resp)
	default:
		return wechatcontracts.NewApplymentQueryContractError("unsupported query kind %q", kind)
	}
}

func (c *EcommerceClient) decryptEcommerceApplymentAccountValidation(validation *wechatcontracts.EcommerceApplymentAccountValidation) error {
	if validation == nil {
		return nil
	}
	rawAccountName := strings.TrimSpace(validation.AccountName)
	if rawAccountName == "" {
		return nil
	}
	plaintextAccountName, err := c.DecryptSensitiveResponseData(rawAccountName)
	if err != nil {
		return fmt.Errorf("query ecommerce applyment: decrypt account_validation.account_name: %w", err)
	}
	validation.RawAccountName = rawAccountName
	validation.AccountName = strings.TrimSpace(plaintextAccountName)
	if validation.AccountName == "" {
		return errors.New("query ecommerce applyment: decrypted account_validation.account_name is empty")
	}

	rawAccountNo := strings.TrimSpace(validation.AccountNo)
	if rawAccountNo == "" {
		return nil
	}
	plaintextAccountNo, err := c.DecryptSensitiveResponseData(rawAccountNo)
	if err != nil {
		return fmt.Errorf("query ecommerce applyment: decrypt account_validation.account_no: %w", err)
	}
	validation.RawAccountNo = rawAccountNo
	validation.AccountNo = strings.TrimSpace(plaintextAccountNo)
	return nil
}

func ecommerceApplymentQueryWechatErrorGuide(wxErr *WechatPayError) string {
	if wxErr == nil {
		return "wechat ecommerce applyment query failed"
	}
	switch wxErr.Code {
	case "PARAM_ERROR":
		return "verify the query parameter format, especially out_request_no length and applyment_id semantics, then retry"
	case "INVALID_REQUEST":
		return "verify the request URL and merchant configuration, then retry"
	case "SIGN_ERROR":
		return "verify the merchant certificate, private key, and authorization signature, then retry"
	case "SYSTEM_ERROR":
		return "wechat query failed due to an upstream system error; retry with backoff and escalate if it persists"
	case "RESOURCE_ALREADY_EXISTS":
		return "wechat reported a conflicting resource state; verify the current applyment record before retrying"
	case "NO_AUTH":
		return "verify the merchant has permission to query this applyment and that the configured mchid/appid pair is correct"
	case "RESOURCE_NOT_EXISTS":
		return "wechat could not find the applyment; verify out_request_no or applyment_id before retrying"
	case "RATELIMIT_EXCEEDED":
		return "wechat rate limited the query; reduce polling frequency and retry later"
	default:
		return fmt.Sprintf("wechat ecommerce applyment query failed with upstream code %s; inspect the upstream message and request_id", wxErr.Code)
	}
}

func wrapEcommerceApplymentQueryError(err error) error {
	var wxErr *WechatPayError
	if errors.As(err, &wxErr) {
		return fmt.Errorf("query ecommerce applyment: %s: %w", ecommerceApplymentQueryWechatErrorGuide(wxErr), err)
	}
	return fmt.Errorf("query ecommerce applyment: %w", err)
}

func queryEcommerceApplymentLogEvent(requestID string, applymentID int64, outRequestNo string) *zerolog.Event {
	evt := log.Error().Str("request_id", requestID)
	if applymentID > 0 {
		evt = evt.Int64("applyment_id", applymentID)
	}
	if outRequestNo != "" {
		evt = evt.Str("out_request_no", outRequestNo)
	}
	return evt
}

func (c *EcommerceClient) queryEcommerceApplyment(ctx context.Context, kind ecommerceApplymentQueryKind, requestURL string, applymentID int64, outRequestNo string) (*wechatcontracts.EcommerceApplymentQueryResponse, error) {
	respBody, requestID, err := c.doRequestWithRequestID(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		wrappedErr := wrapEcommerceApplymentQueryError(err)
		evt := queryEcommerceApplymentLogEvent(requestID, applymentID, outRequestNo)
		var wxErr *WechatPayError
		if errors.As(err, &wxErr) {
			evt = evt.
				Int("status_code", wxErr.StatusCode).
				Str("wechat_code", wxErr.Code).
				Str("wechat_message", wxErr.Message).
				Str("wechat_detail", wxErr.Detail)
		}
		evt.Err(wrappedErr).Msg("wechat ecommerce applyment query failed")
		return nil, wrappedErr
	}

	var resp wechatcontracts.EcommerceApplymentQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		wrappedErr := fmt.Errorf("query ecommerce applyment: request_id=%s: decode response: %w", requestID, err)
		queryEcommerceApplymentLogEvent(requestID, applymentID, outRequestNo).
			Err(wrappedErr).
			Msg("wechat ecommerce applyment query response decode failed")
		return nil, wrappedErr
	}
	if err := validateEcommerceApplymentQueryResponse(&resp, kind); err != nil {
		wrappedErr := fmt.Errorf("query ecommerce applyment: request_id=%s: %w", requestID, err)
		queryEcommerceApplymentLogEvent(requestID, applymentID, outRequestNo).
			Str("applyment_state", resp.ApplymentState).
			Str("sign_state", resp.SignState).
			Str("sub_mchid", resp.SubMchID).
			Err(wrappedErr).
			Msg("wechat ecommerce applyment query response contract validation failed")
		return nil, wrappedErr
	}
	if err := c.decryptEcommerceApplymentAccountValidation(resp.AccountValidation); err != nil {
		wrappedErr := fmt.Errorf("query ecommerce applyment: request_id=%s: %w", requestID, err)
		queryEcommerceApplymentLogEvent(requestID, applymentID, outRequestNo).
			Err(wrappedErr).
			Msg("wechat ecommerce applyment query response sensitive field decryption failed")
		return nil, wrappedErr
	}

	return &resp, nil
}

// CreateEcommerceApplyment 提交二级商户进件申请
// 注意：敏感信息需要使用微信支付平台公钥加密
func (c *EcommerceClient) CreateEcommerceApplyment(ctx context.Context, req *EcommerceApplymentRequest) (*wechatcontracts.EcommerceApplymentResponse, error) {
	body := map[string]interface{}{
		"out_request_no":      req.OutRequestNo,
		"organization_type":   req.OrganizationType,
		"finance_institution": req.FinanceInstitution,
		"merchant_shortname":  req.MerchantShortname,
	}

	if req.BusinessLicense != nil {
		businessLicenseInfo := map[string]interface{}{
			"business_license_copy":   req.BusinessLicense.BusinessLicenseCopy,
			"business_license_number": req.BusinessLicense.BusinessLicenseNumber,
			"merchant_name":           req.BusinessLicense.MerchantName,
			"legal_person":            req.BusinessLicense.LegalPerson,
		}
		if req.BusinessLicense.CertType != "" {
			businessLicenseInfo["cert_type"] = req.BusinessLicense.CertType
		}
		if req.BusinessLicense.CompanyAddress != "" {
			businessLicenseInfo["company_address"] = req.BusinessLicense.CompanyAddress
		}
		if req.BusinessLicense.BusinessTime != "" {
			businessLicenseInfo["business_time"] = req.BusinessLicense.BusinessTime
		}
		body["business_license_info"] = businessLicenseInfo
	}

	body["id_card_info"] = map[string]interface{}{
		"id_card_copy":             req.IDCardInfo.IDCardCopy,
		"id_card_national":         req.IDCardInfo.IDCardNational,
		"id_card_name":             req.IDCardInfo.IDCardName,
		"id_card_number":           req.IDCardInfo.IDCardNumber,
		"id_card_valid_time_begin": req.IDCardInfo.IDCardValidTimeBegin,
		"id_card_valid_time":       req.IDCardInfo.IDCardValidTime,
	}

	if req.AccountInfo != nil {
		accountInfo := map[string]interface{}{
			"bank_account_type": normalizeEcommerceBankAccountType(req.AccountInfo.BankAccountType),
			"account_bank":      req.AccountInfo.AccountBank,
			"account_name":      req.AccountInfo.AccountName,
			"account_number":    req.AccountInfo.AccountNumber,
		}
		if req.AccountInfo.BankAddressCode != "" {
			accountInfo["bank_address_code"] = req.AccountInfo.BankAddressCode
		}
		if req.AccountInfo.BankBranchID != "" {
			accountInfo["bank_branch_id"] = req.AccountInfo.BankBranchID
		}
		if req.AccountInfo.BankName != "" {
			accountInfo["bank_name"] = req.AccountInfo.BankName
		}
		body["account_info"] = accountInfo
	}

	contactInfo := map[string]interface{}{
		"contact_type": normalizeEcommerceContactType(req.ContactInfo.ContactType),
		"contact_name": req.ContactInfo.ContactName,
		"mobile_phone": req.ContactInfo.MobilePhone,
	}
	if req.ContactInfo.ContactIDDocType != "" {
		contactInfo["contact_id_doc_type"] = req.ContactInfo.ContactIDDocType
	}
	if req.ContactInfo.ContactIDCardNumber != "" {
		contactInfo["contact_id_card_number"] = req.ContactInfo.ContactIDCardNumber
	}
	if req.ContactInfo.ContactIDDocCopy != "" {
		contactInfo["contact_id_doc_copy"] = req.ContactInfo.ContactIDDocCopy
	}
	if req.ContactInfo.ContactIDDocCopyBack != "" {
		contactInfo["contact_id_doc_copy_back"] = req.ContactInfo.ContactIDDocCopyBack
	}
	if req.ContactInfo.ContactIDDocPeriodBegin != "" {
		contactInfo["contact_id_doc_period_begin"] = req.ContactInfo.ContactIDDocPeriodBegin
	}
	if req.ContactInfo.ContactIDDocPeriodEnd != "" {
		contactInfo["contact_id_doc_period_end"] = req.ContactInfo.ContactIDDocPeriodEnd
	}
	body["contact_info"] = contactInfo

	salesScene := map[string]interface{}{
		"store_name": req.SalesSceneInfo.StoreName,
	}
	if req.SalesSceneInfo.StoreURL != "" {
		salesScene["store_url"] = req.SalesSceneInfo.StoreURL
	}
	if req.SalesSceneInfo.StoreQRCode != "" {
		salesScene["store_qr_code"] = req.SalesSceneInfo.StoreQRCode
	}
	if req.SalesSceneInfo.MiniProgramSubAppID != "" {
		salesScene["mini_program_sub_appid"] = req.SalesSceneInfo.MiniProgramSubAppID
	}
	body["sales_scene_info"] = salesScene

	if req.SettlementInfo != nil {
		settlementInfo := map[string]interface{}{}
		if req.SettlementInfo.SettlementID != 0 {
			settlementInfo["settlement_id"] = req.SettlementInfo.SettlementID
		}
		if req.SettlementInfo.QualificationType != "" {
			settlementInfo["qualification_type"] = req.SettlementInfo.QualificationType
		}
		if len(settlementInfo) > 0 {
			body["settlement_info"] = settlementInfo
		}
	}

	if len(req.Qualifications) > 0 {
		body["qualifications"] = req.Qualifications
	}
	if len(req.BusinessAdditionPics) > 0 {
		body["business_addition_pics"] = req.BusinessAdditionPics
	}
	if req.BusinessAdditionDesc != "" {
		body["business_addition_desc"] = req.BusinessAdditionDesc
	}

	// 包含敏感加密字段时必须携带 Wechatpay-Serial 头，以告知微信使用哪把公钥/证书解密。
	respBody, err := c.doRequestWithWechatSerial(ctx, http.MethodPost, ecommerceApplymentsURL, body)
	if err != nil {
		return nil, fmt.Errorf("create ecommerce applyment: %w", err)
	}

	var resp wechatcontracts.EcommerceApplymentResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if err := validateEcommerceApplymentCreateResponse(&resp); err != nil {
		log.Error().
			Err(err).
			Str("endpoint", ecommerceApplymentsURL).
			Int64("applyment_id", resp.ApplymentID).
			Msg("validate create ecommerce applyment response failed")
		return nil, err
	}
	return &resp, nil
}

func normalizeEcommerceBankAccountType(accountType string) string {
	switch strings.TrimSpace(accountType) {
	case "ACCOUNT_TYPE_BUSINESS", "74":
		return "74"
	case "ACCOUNT_TYPE_PRIVATE", "75":
		return "75"
	default:
		return accountType
	}
}

func normalizeEcommerceContactType(contactType string) string {
	switch strings.TrimSpace(contactType) {
	case "", "LEGAL", "65":
		return "65"
	case "SUPER", "66":
		return "66"
	default:
		return contactType
	}
}

// QueryEcommerceApplymentByID 通过申请单号查询进件状态
func (c *EcommerceClient) QueryEcommerceApplymentByID(ctx context.Context, applymentID int64) (*wechatcontracts.EcommerceApplymentQueryResponse, error) {
	if err := validateEcommerceApplymentID(applymentID); err != nil {
		log.Error().
			Int64("applyment_id", applymentID).
			Err(err).
			Msg("wechat ecommerce applyment query rejected invalid applyment_id")
		return nil, err
	}

	requestURL := fmt.Sprintf(ecommerceApplymentQueryURL, applymentID)
	return c.queryEcommerceApplyment(ctx, ecommerceApplymentQueryByIDKind, requestURL, applymentID, "")
}

// QueryEcommerceApplymentByOutRequestNo 通过业务申请编号查询进件状态
func (c *EcommerceClient) QueryEcommerceApplymentByOutRequestNo(ctx context.Context, outRequestNo string) (*wechatcontracts.EcommerceApplymentQueryResponse, error) {
	normalizedOutRequestNo, err := validateEcommerceApplymentOutRequestNo(outRequestNo)
	if err != nil {
		log.Error().
			Str("out_request_no", strings.TrimSpace(outRequestNo)).
			Err(err).
			Msg("wechat ecommerce applyment query rejected invalid out_request_no")
		return nil, err
	}

	requestURL := fmt.Sprintf(ecommerceApplymentQueryByNo, url.PathEscape(normalizedOutRequestNo))
	return c.queryEcommerceApplyment(ctx, ecommerceApplymentQueryByOutRequestNoKind, requestURL, 0, normalizedOutRequestNo)
}

// ListPersonalBankingBanks 查询支持个人业务的银行列表
func (c *EcommerceClient) ListPersonalBankingBanks(ctx context.Context, offset, limit int) (*wechatcontracts.CapitalBankListResponse, error) {
	requestURL := fmt.Sprintf("%s?offset=%d&limit=%d", capitalPersonalBanksURL, offset, limit)
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("list personal banking banks: %w", err)
	}

	var resp wechatcontracts.CapitalBankListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal personal banking banks: %w", err)
	}

	return &resp, nil
}

// ListCorporateBankingBanks 查询支持对公业务的银行列表
func (c *EcommerceClient) ListCorporateBankingBanks(ctx context.Context, offset, limit int) (*wechatcontracts.CapitalBankListResponse, error) {
	requestURL := fmt.Sprintf("%s?offset=%d&limit=%d", capitalCorporateBanksURL, offset, limit)
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("list corporate banking banks: %w", err)
	}

	var resp wechatcontracts.CapitalBankListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal corporate banking banks: %w", err)
	}

	return &resp, nil
}

// SearchBanksByBankAccount 根据个人银行卡号识别开户银行候选
func (c *EcommerceClient) SearchBanksByBankAccount(ctx context.Context, accountNumber string) (*wechatcontracts.CapitalBankAccountSearchResponse, error) {
	encryptedAccountNumber, err := c.EncryptSensitiveData(accountNumber)
	if err != nil {
		return nil, fmt.Errorf("encrypt account number: %w", err)
	}

	requestURL := fmt.Sprintf("%s?account_number=%s", capitalSearchBanksByAccountURL, url.QueryEscape(encryptedAccountNumber))
	respBody, err := c.doRequestWithWechatSerial(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("search banks by account number: %w", err)
	}

	var resp wechatcontracts.CapitalBankAccountSearchResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal banks by account number: %w", err)
	}

	return &resp, nil
}

// ListProvinceAreas 查询省份列表
func (c *EcommerceClient) ListProvinceAreas(ctx context.Context) (*wechatcontracts.CapitalProvinceListResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, capitalProvincesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("list province areas: %w", err)
	}

	var resp wechatcontracts.CapitalProvinceListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal province areas: %w", err)
	}

	return &resp, nil
}

// ListCityAreas 查询省份下城市列表
func (c *EcommerceClient) ListCityAreas(ctx context.Context, provinceCode int) (*wechatcontracts.CapitalCityListResponse, error) {
	requestURL := fmt.Sprintf(capitalProvinceCitiesURL, provinceCode)
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("list city areas: %w", err)
	}

	var resp wechatcontracts.CapitalCityListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal city areas: %w", err)
	}

	return &resp, nil
}

// ListBankBranches 查询支行列表
func (c *EcommerceClient) ListBankBranches(ctx context.Context, bankAliasCode string, cityCode, offset, limit int) (*wechatcontracts.CapitalBranchListResponse, error) {
	requestURL := fmt.Sprintf("%s?city_code=%d&offset=%d&limit=%d", fmt.Sprintf(capitalBankBranchesURL, bankAliasCode), cityCode, offset, limit)
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("list bank branches: %w", err)
	}

	var resp wechatcontracts.CapitalBranchListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal bank branches: %w", err)
	}

	return &resp, nil
}

// ==================== 合单支付 ====================

// CreateCombineOrder 创建合单订单（平台收付通）
// 用于商户交易，资金进入二级商户账户
func (c *EcommerceClient) CreateCombineOrder(ctx context.Context, req *CombineOrderRequest) (*CombineOrderResponse, *JSAPIPayParams, error) {
	if req == nil {
		return nil, nil, fmt.Errorf("create combine order: request is nil")
	}
	if strings.TrimSpace(req.PayerSubOpenID) != "" {
		return nil, nil, fmt.Errorf("create combine order: sub_openid is not supported in the single-appid project flow")
	}
	if strings.TrimSpace(req.CombineOutTradeNo) == "" {
		return nil, nil, fmt.Errorf("create combine order: combine_out_trade_no is required")
	}
	if len(req.SubOrders) == 0 {
		return nil, nil, fmt.Errorf("create combine order: sub_orders is required")
	}
	if len(req.SubOrders) > 50 {
		return nil, nil, fmt.Errorf("create combine order: sub_orders exceeds the maximum of 50")
	}
	if strings.TrimSpace(req.PayerOpenID) == "" && strings.TrimSpace(req.PayerSubOpenID) == "" {
		return nil, nil, fmt.Errorf("create combine order: openid or sub_openid is required")
	}
	if req.SceneInfo != nil && strings.TrimSpace(req.SceneInfo.PayerClientIP) == "" {
		return nil, nil, fmt.Errorf("create combine order: scene_info.payer_client_ip is required when scene_info is provided")
	}

	// 构建子订单列表
	subOrders := make([]CombineSubOrderRequest, len(req.SubOrders))
	for i, sub := range req.SubOrders {
		if strings.TrimSpace(sub.SubAppID) != "" {
			return nil, nil, fmt.Errorf("create combine order: sub_orders[%d].sub_appid is not supported in the single-appid project flow", i)
		}
		if strings.TrimSpace(sub.OutTradeNo) == "" {
			return nil, nil, fmt.Errorf("create combine order: sub_orders[%d].out_trade_no is required", i)
		}
		if strings.TrimSpace(sub.Attach) == "" {
			return nil, nil, fmt.Errorf("create combine order: sub_orders[%d].attach is required", i)
		}
		if strings.TrimSpace(sub.Description) == "" {
			return nil, nil, fmt.Errorf("create combine order: sub_orders[%d].description is required", i)
		}
		if sub.Amount <= 0 {
			return nil, nil, fmt.Errorf("create combine order: sub_orders[%d].amount.total_amount must be positive", i)
		}
		mchID := strings.TrimSpace(sub.MchID)
		if mchID == "" {
			mchID = c.spMchID
		}
		subOrders[i] = CombineSubOrderRequest{
			MchID:       mchID,
			SubMchID:    sub.SubMchID,
			SubAppID:    sub.SubAppID,
			OutTradeNo:  sub.OutTradeNo,
			Description: sub.Description,
			Attach:      sub.Attach,
			Detail:      sub.Detail,
			Amount: CombineSubOrderAmount{
				TotalAmount: sub.Amount,
				Currency:    "CNY",
			},
			SettleInfo: &CombineSubOrderSettleInfo{ProfitSharing: sub.ProfitSharing, SubsidyAmount: sub.SubsidyAmount},
			GoodsTag:   sub.GoodsTag,
		}
	}

	notifyURL := c.combineNotifyURL
	if req.NotifyURL != "" {
		notifyURL = req.NotifyURL
	}
	if strings.TrimSpace(notifyURL) == "" {
		return nil, nil, fmt.Errorf("create combine order: notify_url is required")
	}

	body := CombineOrderRequestBody{
		CombineAppID:      c.spAppID,
		CombineMchID:      c.spMchID,
		CombineOutTradeNo: req.CombineOutTradeNo,
		CombinePayerInfo: CombinePayerInfoRequest{
			OpenID: req.PayerOpenID,
		},
		SubOrders: subOrders,
		NotifyURL: notifyURL,
	}
	if !req.ExpireTime.IsZero() {
		body.TimeExpire = req.ExpireTime.Format(time.RFC3339)
	}
	if req.StartTime != nil {
		body.TimeStart = req.StartTime.Format(time.RFC3339)
	}

	if req.SceneInfo != nil {
		body.SceneInfo = &CombineSceneInfo{
			PayerClientIP: req.SceneInfo.PayerClientIP,
			DeviceID:      req.SceneInfo.DeviceID,
		}
	}

	respBody, requestID, err := c.doRequestWithRequestID(ctx, http.MethodPost, ecommerceCombineOrderURL, body)
	if err != nil {
		wrappedErr := wrapCombineOrderCreateError(err)
		ecommercePaymentOrderLogEvent(requestID, "create_combine_order").
			Str("combine_out_trade_no", strings.TrimSpace(req.CombineOutTradeNo)).
			Int("sub_order_count", len(req.SubOrders)).
			Err(wrappedErr).
			Msg("wechat combine order creation failed")
		return nil, nil, wrappedErr
	}

	var resp CombineOrderResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 生成小程序调起支付参数
	payParams, err := c.generateJSAPIPayParams(resp.PrepayID)
	if err != nil {
		return nil, nil, fmt.Errorf("generate pay params: %w", err)
	}

	return &resp, payParams, nil
}

// QueryCombineOrder 查询合单订单
func (c *EcommerceClient) QueryCombineOrder(ctx context.Context, combineOutTradeNo string) (*CombineQueryResponse, error) {
	if strings.TrimSpace(combineOutTradeNo) == "" {
		return nil, newCombineOrderQueryValidationError("query combine order", "combine_out_trade_no is required")
	}

	url := fmt.Sprintf(ecommerceQueryCombineURL, combineOutTradeNo)

	respBody, requestID, err := c.doRequestWithRequestID(ctx, http.MethodGet, url, nil)
	if err != nil {
		wrappedErr := wrapCombineOrderQueryError(err)
		ecommercePaymentOrderLogEvent(requestID, "query_combine_order").
			Str("combine_out_trade_no", strings.TrimSpace(combineOutTradeNo)).
			Err(wrappedErr).
			Msg("wechat combine order query failed")
		return nil, wrappedErr
	}

	var resp CombineQueryResponseBody
	if err := json.Unmarshal(respBody, &resp); err != nil {
		contractErr := newCombineOrderQueryContractError("query combine order", "unmarshal response: %v", err)
		ecommercePaymentOrderLogEvent(requestID, "query_combine_order").
			Str("combine_out_trade_no", strings.TrimSpace(combineOutTradeNo)).
			Err(contractErr).
			Msg("wechat combine order query response contract invalid")
		return nil, contractErr
	}
	if err := validateCombineOrderQueryResponse("query combine order", &resp); err != nil {
		ecommercePaymentOrderLogEvent(requestID, "query_combine_order").
			Str("combine_out_trade_no", strings.TrimSpace(combineOutTradeNo)).
			Err(err).
			Msg("wechat combine order query response contract invalid")
		return nil, err
	}

	return combineQueryResponseFromBody(&resp), nil
}

// CloseCombineOrder 关闭合单订单
func (c *EcommerceClient) CloseCombineOrder(ctx context.Context, combineOutTradeNo string, subOrders []SubOrderClose) error {
	if strings.TrimSpace(combineOutTradeNo) == "" {
		return fmt.Errorf("close combine order: combine_out_trade_no is required")
	}
	if len(subOrders) == 0 {
		return fmt.Errorf("close combine order: sub_orders is required")
	}
	url := fmt.Sprintf(ecommerceCloseCombineURL, combineOutTradeNo)

	subs := make([]CombineCloseSubOrderRequest, len(subOrders))
	for i, sub := range subOrders {
		if strings.TrimSpace(sub.OutTradeNo) == "" {
			return fmt.Errorf("close combine order: sub_orders[%d].out_trade_no is required", i)
		}
		mchID := strings.TrimSpace(sub.MchID)
		if mchID == "" {
			mchID = c.spMchID
		}
		subs[i] = CombineCloseSubOrderRequest{
			MchID:      mchID,
			SubMchID:   sub.SubMchID,
			SubAppID:   sub.SubAppID,
			OutTradeNo: sub.OutTradeNo,
		}
	}

	body := CombineCloseOrderRequest{CombineAppID: c.spAppID, SubOrders: subs}

	_, requestID, err := c.doRequestWithRequestID(ctx, http.MethodPost, url, body)
	if err != nil {
		wrappedErr := wrapCombineOrderCloseError(err)
		ecommercePaymentOrderLogEvent(requestID, "close_combine_order").
			Str("combine_out_trade_no", strings.TrimSpace(combineOutTradeNo)).
			Int("sub_order_count", len(subOrders)).
			Err(wrappedErr).
			Msg("wechat combine order close failed")
		return wrappedErr
	}

	return nil
}

func ecommercePaymentOrderLogEvent(requestID string, operation string) *zerolog.Event {
	evt := log.Error().Str("wechat_operation", operation)
	if strings.TrimSpace(requestID) != "" {
		evt = evt.Str("request_id", strings.TrimSpace(requestID))
	}
	return evt
}

func partnerJSAPIOrderCreateWechatErrorGuide(wxErr *WechatPayError) string {
	if wxErr == nil {
		return "wechat partner jsapi order creation failed"
	}
	switch {
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeParamError, wechaterrorcodes.OrderingCodeInvalidRequest):
		return "verify the partner jsapi request fields, especially payer, amount, sub_mchid, and scene_info.payer_client_ip, then retry"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeAppIDMchIDNotMatch, wechaterrorcodes.OrderingCodeOpenIDMismatch, wechaterrorcodes.OrderingCodeMchNotExists):
		return "verify the configured appid, mchid, openid binding, and merchant configuration before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeNoAuth):
		return "verify the service provider merchant has permission to create partner jsapi orders before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeSignError):
		return "verify the merchant certificate, private key, and authorization signature before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOrderClosed):
		return "wechat reported the payment order is already closed; recreate the payment with a new out_trade_no"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOutTradeNoUsed):
		return "wechat reported the out_trade_no already exists; verify idempotency and reuse the existing pending payment when possible"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeAccountError):
		return "wechat rejected the payer account; ask the user to switch account or retry later"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeRuleLimit, wechaterrorcodes.OrderingCodeFrequencyLimited, wechaterrorcodes.OrderingCompatCodeRateLimit):
		return "wechat rate limited the payment request; retry later with backoff"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeTradeError):
		return "wechat rejected the payment due to business rules; inspect the upstream detail and request_id before retrying"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeBankError, wechaterrorcodes.OrderingCodeSystemError):
		return "wechat payment creation failed due to an upstream system error; retry later with backoff"
	default:
		return fmt.Sprintf("wechat partner jsapi order creation failed with upstream code %s; inspect the upstream message and request_id", wxErr.Code)
	}
}

func wrapPartnerJSAPIOrderCreateError(err error) error {
	var wxErr *WechatPayError
	if errors.As(err, &wxErr) {
		return fmt.Errorf("create partner jsapi order: %s: %w", partnerJSAPIOrderCreateWechatErrorGuide(wxErr), err)
	}
	return fmt.Errorf("create partner jsapi order: %w", err)
}

func partnerOrderQueryWechatErrorGuide(wxErr *WechatPayError) string {
	if wxErr == nil {
		return "wechat partner order query failed"
	}
	switch {
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOrderNotExist):
		return "wechat could not find the partner payment order; verify out_trade_no, transaction_id, and sub_mchid before retrying"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeParamError, wechaterrorcodes.OrderingCodeInvalidRequest):
		return "verify the partner order query parameter format and request path before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeNoAuth):
		return "verify the merchant has permission to query this partner payment order before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeSignError):
		return "verify the merchant certificate, private key, and authorization signature before retrying"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeRuleLimit, wechaterrorcodes.OrderingCodeFrequencyLimited, wechaterrorcodes.OrderingCompatCodeRateLimit):
		return "wechat rate limited the partner order query; retry later with backoff"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeSystemError, wechaterrorcodes.OrderingCodeBankError):
		return "wechat partner order query failed due to an upstream system error; retry later with backoff"
	default:
		return fmt.Sprintf("wechat partner order query failed with upstream code %s; inspect the upstream message and request_id", wxErr.Code)
	}
}

func wrapPartnerOrderQueryError(err error) error {
	var wxErr *WechatPayError
	if errors.As(err, &wxErr) {
		return fmt.Errorf("query partner order: %s: %w", partnerOrderQueryWechatErrorGuide(wxErr), err)
	}
	return fmt.Errorf("query partner order: %w", err)
}

func partnerOrderCloseWechatErrorGuide(wxErr *WechatPayError) string {
	if wxErr == nil {
		return "wechat partner order close failed"
	}
	switch {
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOrderClosed):
		return "wechat reported the partner payment order is already closed"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOrderNotExist):
		return "wechat could not find the partner payment order to close; verify out_trade_no and sub_mchid before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeInvalidRequest):
		return "verify the partner order close payload and merchant identifiers before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeNoAuth):
		return "verify the merchant has permission to close this partner payment order before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeSignError):
		return "verify the merchant certificate, private key, and authorization signature before retrying"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeRuleLimit, wechaterrorcodes.OrderingCodeFrequencyLimited, wechaterrorcodes.OrderingCompatCodeRateLimit):
		return "wechat rate limited the partner order close request; retry later with backoff"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeSystemError, wechaterrorcodes.OrderingCodeBankError):
		return "wechat partner order close failed due to an upstream system error; retry later with backoff"
	default:
		return fmt.Sprintf("wechat partner order close failed with upstream code %s; inspect the upstream message and request_id", wxErr.Code)
	}
}

func wrapPartnerOrderCloseError(err error) error {
	var wxErr *WechatPayError
	if errors.As(err, &wxErr) {
		return fmt.Errorf("close partner order: %s: %w", partnerOrderCloseWechatErrorGuide(wxErr), err)
	}
	return fmt.Errorf("close partner order: %w", err)
}

func combineOrderCreateWechatErrorGuide(wxErr *WechatPayError) string {
	if wxErr == nil {
		return "wechat combine order creation failed"
	}
	switch {
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeParamError, wechaterrorcodes.OrderingCodeInvalidRequest):
		return "verify combine_out_trade_no, sub_orders, payer info, and scene_info.payer_client_ip before retrying"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeAppIDMchIDNotMatch, wechaterrorcodes.OrderingCodeOpenIDMismatch, wechaterrorcodes.OrderingCodeMchNotExists):
		return "verify the configured combine_appid, combine_mchid, and payer openid binding before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeNoAuth):
		return "verify the service provider merchant has permission to create combine orders before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeSignError):
		return "verify the merchant certificate, private key, and authorization signature before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOrderClosed):
		return "wechat reported the combine order is already closed; recreate the payment with a new combine_out_trade_no"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOutTradeNoUsed):
		return "wechat reported one of the trade numbers already exists; verify idempotency and reuse the existing pending payment when possible"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeAccountError):
		return "wechat rejected the payer account; ask the user to switch account or retry later"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeRuleLimit, wechaterrorcodes.OrderingCodeFrequencyLimited, wechaterrorcodes.OrderingCompatCodeRateLimit):
		return "wechat rate limited the combine order request; retry later with backoff"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeTradeError):
		return "wechat rejected the combine payment due to business rules; inspect the upstream detail and request_id before retrying"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeBankError, wechaterrorcodes.OrderingCodeSystemError):
		return "wechat combine order creation failed due to an upstream system error; retry later with backoff"
	default:
		return fmt.Sprintf("wechat combine order creation failed with upstream code %s; inspect the upstream message and request_id", wxErr.Code)
	}
}

func wrapCombineOrderCreateError(err error) error {
	var wxErr *WechatPayError
	if errors.As(err, &wxErr) {
		return fmt.Errorf("create combine order: %s: %w", combineOrderCreateWechatErrorGuide(wxErr), err)
	}
	return fmt.Errorf("create combine order: %w", err)
}

func combineOrderQueryWechatErrorGuide(wxErr *WechatPayError) string {
	if wxErr == nil {
		return "wechat combine order query failed"
	}
	switch {
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOrderNotExist):
		return "wechat could not find the combine payment order; verify combine_out_trade_no before retrying"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeParamError, wechaterrorcodes.OrderingCodeInvalidRequest):
		return "verify the combine order query parameter format and request path before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeNoAuth):
		return "verify the merchant has permission to query this combine payment order before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeSignError):
		return "verify the merchant certificate, private key, and authorization signature before retrying"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeRuleLimit, wechaterrorcodes.OrderingCodeFrequencyLimited, wechaterrorcodes.OrderingCompatCodeRateLimit):
		return "wechat rate limited the combine order query; retry later with backoff"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeSystemError, wechaterrorcodes.OrderingCodeBankError):
		return "wechat combine order query failed due to an upstream system error; retry later with backoff"
	default:
		return fmt.Sprintf("wechat combine order query failed with upstream code %s; inspect the upstream message and request_id", wxErr.Code)
	}
}

func wrapCombineOrderQueryError(err error) error {
	var wxErr *WechatPayError
	if errors.As(err, &wxErr) {
		return fmt.Errorf("query combine order: %s: %w", combineOrderQueryWechatErrorGuide(wxErr), err)
	}
	return fmt.Errorf("query combine order: %w", err)
}

func combineOrderCloseWechatErrorGuide(wxErr *WechatPayError) string {
	if wxErr == nil {
		return "wechat combine order close failed"
	}
	switch {
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOrderClosed):
		return "wechat reported the combine payment order is already closed"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOrderNotExist):
		return "wechat could not find the combine payment order to close; verify combine_out_trade_no before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeInvalidRequest):
		return "verify the close payload matches the original combine order sub-orders exactly before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeNoAuth):
		return "verify the merchant has permission to close this combine payment order before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeSignError):
		return "verify the merchant certificate, private key, and authorization signature before retrying"
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeUserPaying):
		return "wechat reports the payer is still completing payment; query the order result before retrying close"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeRuleLimit, wechaterrorcodes.OrderingCodeFrequencyLimited, wechaterrorcodes.OrderingCompatCodeRateLimit):
		return "wechat rate limited the combine order close request; retry later with backoff"
	case wechaterrorcodes.OrderingCodeIn(wxErr.Code, wechaterrorcodes.OrderingCodeSystemError, wechaterrorcodes.OrderingCodeBankError):
		return "wechat combine order close failed due to an upstream system error; retry later with backoff"
	default:
		return fmt.Sprintf("wechat combine order close failed with upstream code %s; inspect the upstream message and request_id", wxErr.Code)
	}
}

func wrapCombineOrderCloseError(err error) error {
	var wxErr *WechatPayError
	if errors.As(err, &wxErr) {
		return fmt.Errorf("close combine order: %s: %w", combineOrderCloseWechatErrorGuide(wxErr), err)
	}
	return fmt.Errorf("close combine order: %w", err)
}

// ==================== 分账 ====================

// ProfitSharingRequest 分账请求
type ProfitSharingRequest struct {
	SubMchID      string                  // 二级商户号
	TransactionID string                  // 微信订单号
	OutOrderNo    string                  // 商户分账单号
	Receivers     []ProfitSharingReceiver // 分账接收方列表
	Finish        bool                    // 是否分账完成
}

// ProfitSharingReceiver 分账接收方
type ProfitSharingReceiver struct {
	Type                  string // 分账接收方类型：MERCHANT_ID/PERSONAL_OPENID
	ReceiverAccount       string // 分账接收方账号
	ReceiverName          string // 分账接收方名称（明文，发送时会加密）
	EncryptedReceiverName string // 已加密的分账接收方名称
	Amount                int64  // 分账金额（分）
	Description           string // 分账描述
}

// ProfitSharingResponse 分账响应
type ProfitSharingResponse struct {
	SubMchID      string `json:"sub_mchid"`
	TransactionID string `json:"transaction_id"`
	OutOrderNo    string `json:"out_order_no"`
	OrderID       string `json:"order_id"` // 微信分账单号
	Status        string `json:"status"`   // PROCESSING/FINISHED
}

// CreateProfitSharing 请求分账
// 订单支付成功后，调用此接口将资金分给各方
func (c *EcommerceClient) CreateProfitSharing(ctx context.Context, req *ProfitSharingRequest) (*ProfitSharingResponse, error) {
	if err := c.validateProfitSharingRequest(req); err != nil {
		return nil, err
	}

	receivers := make([]map[string]interface{}, len(req.Receivers))
	hasSensitiveReceiverName := false
	for i, r := range req.Receivers {
		receivers[i] = map[string]interface{}{
			"type":             r.Type,
			"receiver_account": r.ReceiverAccount,
			"amount":           r.Amount,
			"description":      r.Description,
		}

		receiverName, err := c.resolveEncryptedReceiverName(r.ReceiverName, r.EncryptedReceiverName)
		if err != nil {
			return nil, fmt.Errorf("resolve receiver name: %w", err)
		}
		if receiverName != "" {
			receivers[i]["receiver_name"] = receiverName
			hasSensitiveReceiverName = true
		}
	}

	body := map[string]interface{}{
		"appid":          c.spAppID,
		"sub_mchid":      req.SubMchID,
		"transaction_id": req.TransactionID,
		"out_order_no":   req.OutOrderNo,
		"receivers":      receivers,
		"finish":         req.Finish,
	}

	requestFn := c.doRequest
	if hasSensitiveReceiverName {
		requestFn = c.doRequestWithWechatSerial
	}

	respBody, err := requestFn(ctx, http.MethodPost, profitSharingURL, body)
	if err != nil {
		return nil, fmt.Errorf("create profit sharing: %w", err)
	}

	var resp ProfitSharingResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// ProfitSharingAmountsResponse 查询订单剩余待分账金额响应。
type ProfitSharingAmountsResponse struct {
	TransactionID string `json:"transaction_id"`
	UnsplitAmount int64  `json:"unsplit_amount"`
}

// QueryProfitSharingAmounts 查询订单剩余待分账金额。
func (c *EcommerceClient) QueryProfitSharingAmounts(ctx context.Context, transactionID string) (*ProfitSharingAmountsResponse, error) {
	if strings.TrimSpace(transactionID) == "" {
		return nil, fmt.Errorf("query profit sharing amounts: transaction_id is required")
	}

	respBody, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf(profitSharingAmountsURL, url.PathEscape(transactionID)), nil)
	if err != nil {
		return nil, fmt.Errorf("query profit sharing amounts: %w", err)
	}

	var resp ProfitSharingAmountsResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.TransactionID == "" {
		resp.TransactionID = transactionID
	}

	return &resp, nil
}

// QueryProfitSharing 查询分账结果
func (c *EcommerceClient) QueryProfitSharing(ctx context.Context, subMchID, transactionID, outOrderNo string) (*ProfitSharingQueryResponse, error) {
	query := url.Values{}
	query.Set("sub_mchid", subMchID)
	query.Set("transaction_id", transactionID)
	query.Set("out_order_no", outOrderNo)
	requestURL := profitSharingURL + "?" + query.Encode()

	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query profit sharing: %w", err)
	}

	var resp ProfitSharingQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// ProfitSharingQueryResponse 分账查询响应
type ProfitSharingQueryResponse struct {
	SubMchID          string                        `json:"sub_mchid"`
	TransactionID     string                        `json:"transaction_id"`
	OutOrderNo        string                        `json:"out_order_no"`
	OrderID           string                        `json:"order_id"`
	Status            string                        `json:"status"`
	Receivers         []ProfitSharingReceiverResult `json:"receivers"`
	FinishAmount      int64                         `json:"finish_amount"`
	FinishDescription string                        `json:"finish_description"`
}

// ProfitSharingReceiverResult 分账接收方结果
type ProfitSharingReceiverResult struct {
	Type            string `json:"type"`
	ReceiverAccount string `json:"receiver_account"`
	Amount          int64  `json:"amount"`
	Description     string `json:"description"`
	Result          string `json:"result"` // PENDING/SUCCESS/CLOSED
	FinishTime      string `json:"finish_time"`
	FailReason      string `json:"fail_reason"`
	DetailID        string `json:"detail_id"`
}

// FinishProfitSharing 完结分账
// 分账完成后，剩余资金解冻给二级商户
func (c *EcommerceClient) FinishProfitSharing(ctx context.Context, subMchID, transactionID, outOrderNo, description string) (*ProfitSharingResponse, error) {
	body := map[string]interface{}{
		"sub_mchid":      subMchID,
		"transaction_id": transactionID,
		"out_order_no":   outOrderNo,
		"description":    description,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, profitSharingFinishURL, body)
	if err != nil {
		return nil, fmt.Errorf("finish profit sharing: %w", err)
	}

	var resp ProfitSharingResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// ==================== 分账接收方管理 ====================

// ReceiverType 分账接收方类型
const (
	ReceiverTypeMerchant = "MERCHANT_ID"     // 商户号
	ReceiverTypePersonal = "PERSONAL_OPENID" // 个人openid
)

// RelationType 分账关系类型
const (
	RelationServiceProvider = "SERVICE_PROVIDER" // 服务商
	RelationDistributor     = "DISTRIBUTOR"      // 分销商
	RelationSupplier        = "SUPPLIER"         // 供应商
	RelationPlatform        = "PLATFORM"         // 平台
	RelationOthers          = "OTHERS"           // 其他
)

// AddReceiverRequest 添加分账接收方请求
type AddReceiverRequest struct {
	AppID         string `json:"appid"`          // 应用ID
	Type          string `json:"type"`           // 接收方类型：MERCHANT_ID/PERSONAL_OPENID
	Account       string `json:"account"`        // 接收方账号（商户号或openid）
	Name          string `json:"name,omitempty"` // 接收方名称（明文，发送时会加密）
	EncryptedName string `json:"-"`              // 已加密的接收方名称
	RelationType  string `json:"relation_type"`  // 与分账方的关系类型
}

// AddReceiverResponse 添加分账接收方响应
type AddReceiverResponse struct {
	Type         string `json:"type"`
	Account      string `json:"account"`
	RelationType string `json:"relation_type"`
}

// AddProfitSharingReceiver 添加分账接收方。
// 该接口用于为后续分账建立接收方关系，不同接收方类型是否需要预先添加应以当前官方规则和业务主链为准。
func (c *EcommerceClient) AddProfitSharingReceiver(ctx context.Context, req *AddReceiverRequest) (*AddReceiverResponse, error) {
	if err := c.validateAddReceiverRequest(req); err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"appid":         req.AppID,
		"type":          req.Type,
		"account":       req.Account,
		"relation_type": req.RelationType,
	}

	receiverName, err := c.resolveEncryptedReceiverName(req.Name, req.EncryptedName)
	if err != nil {
		return nil, fmt.Errorf("resolve receiver name: %w", err)
	}
	if receiverName != "" {
		body["name"] = receiverName
	}

	// 包含敏感加密字段时必须携带 Wechatpay-Serial 头，以告知微信使用哪把公钥/证书解密
	respBody, err := c.doRequestWithWechatSerial(ctx, http.MethodPost, profitSharingReceiverAddURL, body)
	if err != nil {
		return nil, fmt.Errorf("add profit sharing receiver: %w", err)
	}

	var resp AddReceiverResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// DeleteReceiverRequest 删除分账接收方请求
type DeleteReceiverRequest struct {
	AppID   string `json:"appid"`   // 应用ID
	Type    string `json:"type"`    // 接收方类型
	Account string `json:"account"` // 接收方账号
}

// DeleteReceiverResponse 删除分账接收方响应
type DeleteReceiverResponse struct {
	Type    string `json:"type"`
	Account string `json:"account"`
}

// DeleteProfitSharingReceiver 删除分账接收方
func (c *EcommerceClient) DeleteProfitSharingReceiver(ctx context.Context, req *DeleteReceiverRequest) (*DeleteReceiverResponse, error) {
	body := map[string]interface{}{
		"appid":   req.AppID,
		"type":    req.Type,
		"account": req.Account,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, profitSharingReceiverDeleteURL, body)
	if err != nil {
		return nil, fmt.Errorf("delete profit sharing receiver: %w", err)
	}

	var resp DeleteReceiverResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// ==================== 分账回退 ====================

// ProfitSharingReturnRequest 分账回退请求
type ProfitSharingReturnRequest struct {
	SubMchID      string // 二级商户号
	OrderID       string // 微信分账单号
	OutOrderNo    string // 商户分账单号
	OutReturnNo   string // 商户回退单号
	TransactionID string // 微信订单号（大于 6 个月订单必填）
	// 回退接收方（兼容两种写法）
	// 推荐：ReturnAccountType + ReturnAccount
	// 兼容：ReturnMchID（历史商户号字段）
	ReturnAccountType string // MERCHANT_ID / PERSONAL_OPENID
	ReturnAccount     string // 商户号或openid
	ReturnMchID       string // 回退商户号（兼容旧逻辑）
	Amount            int64  // 回退金额（分）
	Description       string // 回退描述
}

// ProfitSharingReturnResponse 分账回退响应
type ProfitSharingReturnResponse struct {
	SubMchID          string `json:"sub_mchid"`
	OrderID           string `json:"order_id"`
	OutOrderNo        string `json:"out_order_no"`
	OutReturnNo       string `json:"out_return_no"`
	ReturnID          string `json:"return_no"`
	ReturnMchID       string `json:"return_mchid"`
	ReturnAccountType string `json:"return_account_type"`
	ReturnAccount     string `json:"return_account"`
	Amount            int64  `json:"amount"`
	Result            string `json:"result"` // PROCESSING/SUCCESS/FAILED
	FinishTime        string `json:"finish_time"`
	FailReason        string `json:"fail_reason"`
	TransactionID     string `json:"transaction_id"`
}

// CreateProfitSharingReturn 请求分账回退
// 退款时需要先从各分账方回退资金
func (c *EcommerceClient) CreateProfitSharingReturn(ctx context.Context, req *ProfitSharingReturnRequest) (*ProfitSharingReturnResponse, error) {
	if err := c.validateProfitSharingReturnRequest(req); err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"sub_mchid":     req.SubMchID,
		"out_order_no":  req.OutOrderNo,
		"out_return_no": req.OutReturnNo,
		"amount":        req.Amount,
		"description":   req.Description,
	}
	if req.OrderID != "" {
		body["order_id"] = req.OrderID
	}
	if req.TransactionID != "" {
		body["transaction_id"] = req.TransactionID
	}

	if req.ReturnMchID != "" {
		body["return_mchid"] = req.ReturnMchID
	} else if req.ReturnAccountType == ReceiverTypeMerchant && req.ReturnAccount != "" {
		body["return_mchid"] = req.ReturnAccount
	} else if req.ReturnAccountType != "" && req.ReturnAccount != "" {
		body["return_account_type"] = req.ReturnAccountType
		body["return_account"] = req.ReturnAccount
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, profitSharingReturnURL, body)
	if err != nil {
		return nil, fmt.Errorf("create profit sharing return: %w", err)
	}

	var resp ProfitSharingReturnResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// QueryProfitSharingReturn 查询分账回退结果
func (c *EcommerceClient) QueryProfitSharingReturn(ctx context.Context, subMchID, outReturnNo, outOrderNo string) (*ProfitSharingReturnResponse, error) {
	if strings.TrimSpace(subMchID) == "" || strings.TrimSpace(outReturnNo) == "" {
		return nil, fmt.Errorf("query profit sharing return: sub_mchid and out_return_no are required")
	}

	query := url.Values{}
	query.Set("sub_mchid", subMchID)
	query.Set("out_return_no", outReturnNo)
	if strings.TrimSpace(outOrderNo) != "" {
		query.Set("out_order_no", outOrderNo)
	}
	requestURL := profitSharingReturnQueryURL + "?" + query.Encode()

	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query profit sharing return: %w", err)
	}

	var resp ProfitSharingReturnResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

func (c *EcommerceClient) resolveEncryptedReceiverName(name, encryptedName string) (string, error) {
	if encryptedName != "" {
		return encryptedName, nil
	}
	if name == "" {
		return "", nil
	}

	resolved, err := c.EncryptSensitiveData(name)
	if err != nil {
		return "", fmt.Errorf("encrypt receiver name: %w", err)
	}
	return resolved, nil
}

func (c *EcommerceClient) validateProfitSharingRequest(req *ProfitSharingRequest) error {
	if req == nil {
		return fmt.Errorf("create profit sharing: request is nil")
	}
	if strings.TrimSpace(req.SubMchID) == "" || strings.TrimSpace(req.TransactionID) == "" || strings.TrimSpace(req.OutOrderNo) == "" {
		return fmt.Errorf("create profit sharing: sub_mchid, transaction_id and out_order_no are required")
	}
	if len(req.Receivers) == 0 {
		return fmt.Errorf("create profit sharing: receivers are required")
	}

	seenReceivers := make(map[string]struct{}, len(req.Receivers))
	for _, receiver := range req.Receivers {
		receiverType := strings.TrimSpace(receiver.Type)
		receiverAccount := strings.TrimSpace(receiver.ReceiverAccount)
		if receiverType == "" || receiverAccount == "" {
			return fmt.Errorf("create profit sharing: receiver type and account are required")
		}
		if receiver.Amount <= 0 {
			return fmt.Errorf("create profit sharing: receiver amount must be positive")
		}
		if strings.TrimSpace(receiver.Description) == "" {
			return fmt.Errorf("create profit sharing: receiver description is required")
		}
		if receiverType == ReceiverTypePersonal && strings.TrimSpace(c.spAppID) == "" {
			return fmt.Errorf("create profit sharing: appid is required for personal receivers")
		}
		if req.Finish && receiverType == ReceiverTypeMerchant && receiverAccount == req.SubMchID {
			return fmt.Errorf("create profit sharing: finish=true does not allow sub_mchid as receiver")
		}

		receiverKey := receiverType + ":" + receiverAccount
		if _, exists := seenReceivers[receiverKey]; exists {
			return fmt.Errorf("create profit sharing: duplicate receiver %s", receiverKey)
		}
		seenReceivers[receiverKey] = struct{}{}
	}

	return nil
}

func (c *EcommerceClient) validateAddReceiverRequest(req *AddReceiverRequest) error {
	if req == nil {
		return fmt.Errorf("add profit sharing receiver: request is nil")
	}
	if strings.TrimSpace(req.Type) == "" || strings.TrimSpace(req.Account) == "" {
		return fmt.Errorf("add profit sharing receiver: type and account are required")
	}
	if req.Type == ReceiverTypePersonal && strings.TrimSpace(req.AppID) == "" {
		return fmt.Errorf("add profit sharing receiver: appid is required for personal receivers")
	}
	if !isSupportedRelationType(req.RelationType) {
		return fmt.Errorf("add profit sharing receiver: unsupported relation_type %q", req.RelationType)
	}
	return nil
}

func (c *EcommerceClient) validateProfitSharingReturnRequest(req *ProfitSharingReturnRequest) error {
	if req == nil {
		return fmt.Errorf("create profit sharing return: request is nil")
	}
	if strings.TrimSpace(req.SubMchID) == "" || strings.TrimSpace(req.OutReturnNo) == "" {
		return fmt.Errorf("create profit sharing return: sub_mchid and out_return_no are required")
	}
	if strings.TrimSpace(req.OrderID) == "" && strings.TrimSpace(req.OutOrderNo) == "" {
		return fmt.Errorf("create profit sharing return: order_id or out_order_no is required")
	}
	if req.Amount <= 0 {
		return fmt.Errorf("create profit sharing return: amount must be positive")
	}
	if strings.TrimSpace(req.Description) == "" {
		return fmt.Errorf("create profit sharing return: description is required")
	}
	if strings.TrimSpace(req.ReturnMchID) == "" && strings.TrimSpace(req.ReturnAccount) == "" {
		return fmt.Errorf("create profit sharing return: return target is required")
	}
	return nil
}

func isSupportedRelationType(relationType string) bool {
	switch strings.TrimSpace(relationType) {
	case RelationServiceProvider, RelationDistributor, RelationSupplier, RelationPlatform, RelationOthers:
		return true
	default:
		return false
	}
}

// IsProfitSharingReturnProcessingError 判断分账回退请求错误是否仍需进入结果轮询。
func IsProfitSharingReturnProcessingError(err error) bool {
	if err == nil {
		return false
	}

	var wxErr *WechatPayError
	if !errors.As(err, &wxErr) {
		return false
	}

	switch strings.ToUpper(strings.TrimSpace(wxErr.Code)) {
	case "NOT_ENOUGH", "PAYER_ACCOUNT_ABNORMAL":
		return true
	default:
		return false
	}
}

// ==================== 电商退款 ====================

const (
	EcommerceAbnormalRefundTypeUserBankCard     = "USER_BANK_CARD"
	EcommerceAbnormalRefundTypeMerchantBankCard = "MERCHANT_BANK_CARD"
)

// EcommerceRefundAmountFrom 指定退款出资账户及金额
type EcommerceRefundAmountFrom struct {
	Account string `json:"account"`
	Amount  int64  `json:"amount"`
}

// EcommerceRefundPromotionDetail 电商退款营销明细
type EcommerceRefundPromotionDetail struct {
	PromotionID  string `json:"promotion_id"`
	Scope        string `json:"scope"`
	Type         string `json:"type"`
	Amount       int64  `json:"amount"`
	RefundAmount int64  `json:"refund_amount"`
}

// EcommerceRefundAmount 电商退款金额信息
type EcommerceRefundAmount struct {
	Refund         int64                       `json:"refund"`
	From           []EcommerceRefundAmountFrom `json:"from,omitempty"`
	PayerRefund    int64                       `json:"payer_refund"`
	DiscountRefund int64                       `json:"discount_refund,omitempty"`
	Total          int64                       `json:"total,omitempty"`
	PayerTotal     int64                       `json:"payer_total,omitempty"`
	Currency       string                      `json:"currency,omitempty"`
	Advance        int64                       `json:"advance,omitempty"`
	RefundAccount  string                      `json:"refund_account,omitempty"`
}

// EcommerceRefundRequest 电商退款请求
type EcommerceRefundRequest struct {
	SubMchID      string                      // 二级商户号
	SubAppID      string                      // 二级商户 AppID（选填）
	TransactionID string                      // 微信支付订单号
	OutTradeNo    string                      // 商户订单号（二选一）
	OutRefundNo   string                      // 商户退款单号
	Reason        string                      // 退款原因
	RefundAmount  int64                       // 退款金额（分）
	TotalAmount   int64                       // 原订单金额（分）
	AmountFrom    []EcommerceRefundAmountFrom // 指定退款出资账户及金额
	NotifyURL     string                      // 本次退款回调地址（选填）
	RefundAccount string                      // 退款出资商户（选填）
	// 退款资金来源
	// AVAILABLE: 可用余额账户（默认）
	// UNSETTLED: 未结算资金
	FundsAccount string
}

// EcommerceRefundResponse 电商退款响应
type EcommerceRefundResponse struct {
	RefundID            string                           `json:"refund_id"`
	OutRefundNo         string                           `json:"out_refund_no"`
	TransactionID       string                           `json:"transaction_id"`
	OutTradeNo          string                           `json:"out_trade_no"`
	Channel             string                           `json:"channel"`
	UserReceivedAccount string                           `json:"user_received_account"`
	CreateTime          string                           `json:"create_time"`
	Status              string                           `json:"status"` // PROCESSING/SUCCESS/CLOSED/ABNORMAL
	SuccessTime         string                           `json:"success_time"`
	Amount              EcommerceRefundAmount            `json:"amount"`
	PromotionDetail     []EcommerceRefundPromotionDetail `json:"promotion_detail,omitempty"`
	RefundAccount       string                           `json:"refund_account"`
	FundsAccount        string                           `json:"funds_account"`
}

// EcommerceAbnormalRefundRequest 电商异常退款处理请求
type EcommerceAbnormalRefundRequest struct {
	RefundID    string // 微信退款单号
	SubMchID    string // 二级商户号
	OutRefundNo string // 商户退款单号
	Type        string // USER_BANK_CARD 或 MERCHANT_BANK_CARD
	BankType    string // 用户银行卡开户行，仅 USER_BANK_CARD 必填
	BankAccount string // 用户银行卡号，仅 USER_BANK_CARD 必填
	RealName    string // 收款用户姓名，仅 USER_BANK_CARD 必填
}

// CreateEcommerceRefund 申请电商退款
// 退款前需要先调用分账回退
func (c *EcommerceClient) CreateEcommerceRefund(ctx context.Context, req *EcommerceRefundRequest) (*EcommerceRefundResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("create ecommerce refund: request is nil")
	}
	if req.SubMchID == "" {
		return nil, fmt.Errorf("create ecommerce refund: sub_mchid is required")
	}
	if req.OutRefundNo == "" {
		return nil, fmt.Errorf("create ecommerce refund: out_refund_no is required")
	}
	if req.TransactionID == "" && req.OutTradeNo == "" {
		return nil, fmt.Errorf("create ecommerce refund: transaction_id or out_trade_no is required")
	}
	if req.RefundAmount <= 0 || req.TotalAmount <= 0 {
		return nil, fmt.Errorf("create ecommerce refund: refund and total amount must be positive")
	}
	if len(req.AmountFrom) > 0 && req.FundsAccount != "" {
		return nil, fmt.Errorf("create ecommerce refund: amount.from and funds_account are mutually exclusive")
	}

	body := map[string]interface{}{
		"sub_mchid":     req.SubMchID,
		"sp_appid":      c.spAppID,
		"out_refund_no": req.OutRefundNo,
		"amount": map[string]interface{}{
			"refund":   req.RefundAmount,
			"total":    req.TotalAmount,
			"currency": "CNY",
		},
	}
	if req.SubAppID != "" {
		body["sub_appid"] = req.SubAppID
	}
	if req.Reason != "" {
		body["reason"] = req.Reason
	}
	notifyURL := req.NotifyURL
	if notifyURL == "" {
		notifyURL = c.refundNotifyURL
	}
	if notifyURL != "" {
		body["notify_url"] = notifyURL
	}

	// 微信支付订单号或商户订单号二选一
	if req.TransactionID != "" {
		body["transaction_id"] = req.TransactionID
	} else if req.OutTradeNo != "" {
		body["out_trade_no"] = req.OutTradeNo
	}
	if len(req.AmountFrom) > 0 {
		from := make([]map[string]interface{}, 0, len(req.AmountFrom))
		for _, entry := range req.AmountFrom {
			from = append(from, map[string]interface{}{
				"account": entry.Account,
				"amount":  entry.Amount,
			})
		}
		body["amount"].(map[string]interface{})["from"] = from
	}
	if req.RefundAccount != "" {
		body["refund_account"] = req.RefundAccount
	}

	if req.FundsAccount != "" {
		body["funds_account"] = req.FundsAccount
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, ecommerceRefundURL, body)
	if err != nil {
		return nil, fmt.Errorf("create ecommerce refund: %w", err)
	}

	var resp EcommerceRefundResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// ApplyEcommerceAbnormalRefund 发起电商异常退款处理
func (c *EcommerceClient) ApplyEcommerceAbnormalRefund(ctx context.Context, req *EcommerceAbnormalRefundRequest) (*EcommerceRefundResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("apply ecommerce abnormal refund: request is nil")
	}
	if req.RefundID == "" {
		return nil, fmt.Errorf("apply ecommerce abnormal refund: refund_id is required")
	}
	if req.SubMchID == "" {
		return nil, fmt.Errorf("apply ecommerce abnormal refund: sub_mchid is required")
	}
	if req.OutRefundNo == "" {
		return nil, fmt.Errorf("apply ecommerce abnormal refund: out_refund_no is required")
	}

	body := map[string]interface{}{
		"sub_mchid":     req.SubMchID,
		"out_refund_no": req.OutRefundNo,
		"type":          req.Type,
	}

	switch req.Type {
	case EcommerceAbnormalRefundTypeUserBankCard:
		if req.BankType == "" {
			return nil, fmt.Errorf("apply ecommerce abnormal refund: bank_type is required for USER_BANK_CARD")
		}
		if req.BankAccount == "" {
			return nil, fmt.Errorf("apply ecommerce abnormal refund: bank_account is required for USER_BANK_CARD")
		}
		if req.RealName == "" {
			return nil, fmt.Errorf("apply ecommerce abnormal refund: real_name is required for USER_BANK_CARD")
		}
		encryptedBankAccount, err := c.EncryptSensitiveData(req.BankAccount)
		if err != nil {
			return nil, fmt.Errorf("apply ecommerce abnormal refund: encrypt bank_account: %w", err)
		}
		encryptedRealName, err := c.EncryptSensitiveData(req.RealName)
		if err != nil {
			return nil, fmt.Errorf("apply ecommerce abnormal refund: encrypt real_name: %w", err)
		}
		body["bank_type"] = req.BankType
		body["bank_account"] = encryptedBankAccount
		body["real_name"] = encryptedRealName
	case EcommerceAbnormalRefundTypeMerchantBankCard:
		// 退款至交易商户银行账户无需额外字段。
	default:
		return nil, fmt.Errorf("apply ecommerce abnormal refund: unsupported type %q", req.Type)
	}

	requestURL := fmt.Sprintf(ecommerceAbnormalRefundURL, url.PathEscape(req.RefundID))
	respBody, err := c.doRequestWithWechatSerial(ctx, http.MethodPost, requestURL, body)
	if err != nil {
		return nil, fmt.Errorf("apply ecommerce abnormal refund: %w", err)
	}

	var resp EcommerceRefundResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// QueryEcommerceRefund 查询电商退款
func (c *EcommerceClient) QueryEcommerceRefund(ctx context.Context, subMchID, outRefundNo string) (*EcommerceRefundResponse, error) {
	return c.QueryEcommerceRefundByOutRefundNo(ctx, subMchID, outRefundNo)
}

// QueryEcommerceRefundByOutRefundNo 按商户退款单号查询电商退款
func (c *EcommerceClient) QueryEcommerceRefundByOutRefundNo(ctx context.Context, subMchID, outRefundNo string) (*EcommerceRefundResponse, error) {
	if subMchID == "" {
		return nil, fmt.Errorf("query ecommerce refund by out_refund_no: sub_mchid is required")
	}
	if outRefundNo == "" {
		return nil, fmt.Errorf("query ecommerce refund by out_refund_no: out_refund_no is required")
	}
	requestURL := fmt.Sprintf(ecommerceRefundQueryByOutRefundURL, url.PathEscape(outRefundNo)) + "?sub_mchid=" + url.QueryEscape(subMchID)

	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query ecommerce refund by out_refund_no: %w", err)
	}

	var resp EcommerceRefundResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// QueryEcommerceRefundByID 按微信退款单号查询电商退款
func (c *EcommerceClient) QueryEcommerceRefundByID(ctx context.Context, subMchID, refundID string) (*EcommerceRefundResponse, error) {
	if subMchID == "" {
		return nil, fmt.Errorf("query ecommerce refund by refund_id: sub_mchid is required")
	}
	if refundID == "" {
		return nil, fmt.Errorf("query ecommerce refund by refund_id: refund_id is required")
	}
	requestURL := fmt.Sprintf(ecommerceRefundQueryByIDURL, url.PathEscape(refundID)) + "?sub_mchid=" + url.QueryEscape(subMchID)

	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query ecommerce refund by refund_id: %w", err)
	}

	var resp EcommerceRefundResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// ==================== 账户资金管理 ====================

// EcommerceFundBalanceResponse 二级商户资金账户余额
type EcommerceFundBalanceResponse struct {
	SubMchID           string `json:"sub_mchid"`
	AvailableAmount    int64  `json:"available_amount"`
	PendingAmount      int64  `json:"pending_amount"`
	AccountType        string `json:"account_type"`
	WithdrawableAmount int64  `json:"withdrawable_amount"`
}

// PlatformFundBalanceResponse 平台商户资金账户余额
type PlatformFundBalanceResponse struct {
	AvailableAmount int64  `json:"available_amount"`
	PendingAmount   int64  `json:"pending_amount"`
	AccountType     string `json:"account_type"`
}

// QueryEcommerceFundBalance 查询二级商户可用余额
func (c *EcommerceClient) QueryEcommerceFundBalance(ctx context.Context, subMchID string) (*EcommerceFundBalanceResponse, error) {
	return c.QueryEcommerceFundBalanceByAccountType(ctx, subMchID, "BASIC")
}

// QueryEcommerceFundBalanceByAccountType 按账户类型查询二级商户实时余额
func (c *EcommerceClient) QueryEcommerceFundBalanceByAccountType(ctx context.Context, subMchID, accountType string) (*EcommerceFundBalanceResponse, error) {
	if subMchID == "" {
		return nil, fmt.Errorf("query ecommerce fund balance: sub_mchid is required")
	}
	if accountType == "" {
		accountType = "BASIC"
	}

	query := url.Values{}
	query.Set("account_type", accountType)
	requestURL := fmt.Sprintf(ecommerceFundBalanceURL, url.PathEscape(subMchID)) + "?" + query.Encode()

	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query ecommerce fund balance: %w", err)
	}

	var resp EcommerceFundBalanceResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.SubMchID == "" {
		resp.SubMchID = subMchID
	}
	if resp.AccountType == "" {
		resp.AccountType = accountType
	}
	if resp.WithdrawableAmount == 0 && resp.AvailableAmount > 0 {
		resp.WithdrawableAmount = resp.AvailableAmount
	}

	return &resp, nil
}

// QueryEcommerceFundDayEndBalance 查询二级商户指定日期日终余额
func (c *EcommerceClient) QueryEcommerceFundDayEndBalance(ctx context.Context, subMchID, date, accountType string) (*EcommerceFundBalanceResponse, error) {
	if subMchID == "" {
		return nil, fmt.Errorf("query ecommerce fund day end balance: sub_mchid is required")
	}
	if date == "" {
		return nil, fmt.Errorf("query ecommerce fund day end balance: date is required")
	}
	if accountType == "" {
		accountType = "BASIC"
	}

	query := url.Values{}
	query.Set("date", date)
	query.Set("account_type", accountType)
	requestURL := fmt.Sprintf(ecommerceFundDayEndBalanceURL, url.PathEscape(subMchID)) + "?" + query.Encode()

	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query ecommerce fund day end balance: %w", err)
	}

	var resp EcommerceFundBalanceResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.SubMchID == "" {
		resp.SubMchID = subMchID
	}
	if resp.AccountType == "" {
		resp.AccountType = accountType
	}
	if resp.WithdrawableAmount == 0 && resp.AvailableAmount > 0 {
		resp.WithdrawableAmount = resp.AvailableAmount
	}

	return &resp, nil
}

// QueryPlatformFundBalance 查询平台商户实时余额
func (c *EcommerceClient) QueryPlatformFundBalance(ctx context.Context, accountType string) (*PlatformFundBalanceResponse, error) {
	if accountType == "" {
		accountType = "BASIC"
	}

	requestURL := fmt.Sprintf(platformFundBalanceURL, url.PathEscape(accountType))
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query platform fund balance: %w", err)
	}

	var resp PlatformFundBalanceResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.AccountType == "" {
		resp.AccountType = accountType
	}

	return &resp, nil
}

// QueryPlatformFundDayEndBalance 查询平台商户指定日期日终余额
func (c *EcommerceClient) QueryPlatformFundDayEndBalance(ctx context.Context, accountType, date string) (*PlatformFundBalanceResponse, error) {
	if date == "" {
		return nil, fmt.Errorf("query platform fund day end balance: date is required")
	}
	if accountType == "" {
		accountType = "BASIC"
	}

	query := url.Values{}
	query.Set("date", date)
	requestURL := fmt.Sprintf(platformFundDayEndBalanceURL, url.PathEscape(accountType)) + "?" + query.Encode()

	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query platform fund day end balance: %w", err)
	}

	var resp PlatformFundBalanceResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.AccountType == "" {
		resp.AccountType = accountType
	}

	return &resp, nil
}

// EcommerceWithdrawRequest 二级商户提现请求
type EcommerceWithdrawRequest struct {
	SubMchID     string // 二级商户号
	OutRequestNo string // 商户提现申请单号
	Amount       int64  // 提现金额（分）
	Remark       string // 提现备注
}

// EcommerceWithdrawResponse 二级商户提现响应
type EcommerceWithdrawResponse struct {
	SubMchID     string `json:"sub_mchid"`
	WithdrawID   string `json:"withdraw_id"`
	OutRequestNo string `json:"out_request_no"`
	Amount       int64  `json:"amount"`
	Status       string `json:"status"`
	CreateTime   string `json:"create_time"`
	UpdateTime   string `json:"update_time"`
	SuccessTime  string `json:"success_time"`
	Reason       string `json:"reason"`
}

// CreateEcommerceWithdraw 发起二级商户提现
func (c *EcommerceClient) CreateEcommerceWithdraw(ctx context.Context, req *EcommerceWithdrawRequest) (*EcommerceWithdrawResponse, error) {
	body := map[string]interface{}{
		"sub_mchid":      req.SubMchID,
		"out_request_no": req.OutRequestNo,
		"amount":         req.Amount,
		"remark":         req.Remark,
	}
	if c.withdrawNotifyURL != "" {
		body["notify_url"] = c.withdrawNotifyURL
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, ecommerceFundWithdrawURL, body)
	if err != nil {
		return nil, fmt.Errorf("create ecommerce withdraw: %w", err)
	}

	var resp EcommerceWithdrawResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.SubMchID == "" {
		resp.SubMchID = req.SubMchID
	}
	if resp.OutRequestNo == "" {
		resp.OutRequestNo = req.OutRequestNo
	}
	if resp.Amount == 0 {
		resp.Amount = req.Amount
	}

	return &resp, nil
}

// QueryEcommerceWithdrawByOutRequestNo 通过外部申请单号查询提现状态
func (c *EcommerceClient) QueryEcommerceWithdrawByOutRequestNo(ctx context.Context, subMchID, outRequestNo string) (*EcommerceWithdrawResponse, error) {
	url := fmt.Sprintf(ecommerceFundWithdrawQueryByNo+"?sub_mchid=%s", outRequestNo, subMchID)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("query ecommerce withdraw: %w", err)
	}

	var resp EcommerceWithdrawResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.SubMchID == "" {
		resp.SubMchID = subMchID
	}
	if resp.OutRequestNo == "" {
		resp.OutRequestNo = outRequestNo
	}

	return &resp, nil
}

// ==================== 结算账户查询 ====================

// QuerySubMerchantSettlement 查询特约商户/二级商户结算账户信息
//
// subMchID: 特约商户号；accountNumberRule: 账号展示规则（空字符串使用微信默认 ACCOUNT_NUMBER_RULE_MASK_V1）
func (c *EcommerceClient) QuerySubMerchantSettlement(ctx context.Context, subMchID string, accountNumberRule string) (*wechatcontracts.SubMerchantSettlementResponse, error) {
	normalizedSubMchID, err := validateSubMerchantSettlementSubMchID(subMchID)
	if err != nil {
		log.Error().
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("account_number_rule", strings.TrimSpace(accountNumberRule)).
			Err(err).
			Msg("wechat sub merchant settlement query rejected invalid input")
		return nil, err
	}
	normalizedAccountNumberRule, err := validateSubMerchantSettlementAccountNumberRule(accountNumberRule)
	if err != nil {
		log.Error().
			Str("sub_mchid", normalizedSubMchID).
			Str("account_number_rule", strings.TrimSpace(accountNumberRule)).
			Err(err).
			Msg("wechat sub merchant settlement query rejected invalid input")
		return nil, err
	}

	requestURL := fmt.Sprintf(apply4subSettlementURL, url.PathEscape(normalizedSubMchID))
	if normalizedAccountNumberRule != "" {
		requestURL += "?account_number_rule=" + url.QueryEscape(normalizedAccountNumberRule)
	}

	respBody, requestID, err := c.doRequestWithRequestID(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		wrappedErr := wrapSubMerchantSettlementQueryError(err)
		evt := querySubMerchantSettlementLogEvent(requestID, normalizedSubMchID, normalizedAccountNumberRule)
		var wxErr *WechatPayError
		if errors.As(err, &wxErr) {
			evt = evt.
				Int("status_code", wxErr.StatusCode).
				Str("wechat_code", wxErr.Code).
				Str("wechat_message", wxErr.Message).
				Str("wechat_detail", wxErr.Detail)
		}
		evt.Err(wrappedErr).Msg("wechat sub merchant settlement query failed")
		return nil, wrappedErr
	}

	var resp wechatcontracts.SubMerchantSettlementResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		wrappedErr := fmt.Errorf("query sub merchant settlement: request_id=%s: decode response: %w", requestID, err)
		querySubMerchantSettlementLogEvent(requestID, normalizedSubMchID, normalizedAccountNumberRule).
			Err(wrappedErr).
			Msg("wechat sub merchant settlement response decode failed")
		return nil, wrappedErr
	}
	if err := validateSubMerchantSettlementResponse(&resp); err != nil {
		wrappedErr := fmt.Errorf("query sub merchant settlement: request_id=%s: %w", requestID, err)
		querySubMerchantSettlementLogEvent(requestID, normalizedSubMchID, normalizedAccountNumberRule).
			Str("account_type", resp.AccountType).
			Str("verify_result", resp.VerifyResult).
			Bool("has_verify_fail_reason", strings.TrimSpace(resp.VerifyFailReason) != "").
			Err(wrappedErr).
			Msg("wechat sub merchant settlement response contract validation failed")
		return nil, wrappedErr
	}

	return &resp, nil
}

// ==================== 结算账户修改 ====================

// ModifySubMerchantSettlement 修改特约商户/二级商户结算账户
func (c *EcommerceClient) ModifySubMerchantSettlement(ctx context.Context, subMchID string, req *wechatcontracts.ModifySubMerchantSettlementRequest) (*wechatcontracts.ModifySubMerchantSettlementResponse, error) {
	requestURL := fmt.Sprintf(apply4subModifySettlementURL, subMchID)

	respBody, err := c.doRequestWithWechatSerial(ctx, http.MethodPost, requestURL, req)
	if err != nil {
		return nil, fmt.Errorf("modify sub merchant settlement: %w", err)
	}

	var resp wechatcontracts.ModifySubMerchantSettlementResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal modify sub merchant settlement: %w", err)
	}

	return &resp, nil
}

// ==================== 结算账户修改申请查询 ====================

// QuerySubMerchantSettlementApplication 查询结算账户修改申请状态
//
// subMchID: 特约商户号；applicationNo: 申请单号；accountNumberRule: 账号展示规则（空字符串使用微信默认）
func (c *EcommerceClient) QuerySubMerchantSettlementApplication(ctx context.Context, subMchID, applicationNo, accountNumberRule string) (*wechatcontracts.QuerySubMerchantSettlementApplicationResponse, error) {
	normalizedSubMchID, err := validateSubMerchantSettlementSubMchID(subMchID)
	if err != nil {
		wrappedErr := wechatcontracts.NewSubMerchantSettlementApplicationQueryValidationError("%s", strings.TrimPrefix(err.Error(), "validate sub merchant settlement query request: "))
		log.Error().
			Str("sub_mchid", strings.TrimSpace(subMchID)).
			Str("application_no", strings.TrimSpace(applicationNo)).
			Str("account_number_rule", strings.TrimSpace(accountNumberRule)).
			Err(wrappedErr).
			Msg("wechat sub merchant settlement application query rejected invalid input")
		return nil, wrappedErr
	}
	normalizedApplicationNo, err := validateSubMerchantSettlementApplicationNo(applicationNo)
	if err != nil {
		log.Error().
			Str("sub_mchid", normalizedSubMchID).
			Str("application_no", strings.TrimSpace(applicationNo)).
			Str("account_number_rule", strings.TrimSpace(accountNumberRule)).
			Err(err).
			Msg("wechat sub merchant settlement application query rejected invalid input")
		return nil, err
	}
	normalizedAccountNumberRule, err := validateSubMerchantSettlementAccountNumberRule(accountNumberRule)
	if err != nil {
		wrappedErr := wechatcontracts.NewSubMerchantSettlementApplicationQueryValidationError("%s", strings.TrimPrefix(err.Error(), "validate sub merchant settlement query request: "))
		log.Error().
			Str("sub_mchid", normalizedSubMchID).
			Str("application_no", normalizedApplicationNo).
			Str("account_number_rule", strings.TrimSpace(accountNumberRule)).
			Err(wrappedErr).
			Msg("wechat sub merchant settlement application query rejected invalid input")
		return nil, wrappedErr
	}

	requestURL := fmt.Sprintf(apply4subModifySettlementQueryURL, url.PathEscape(normalizedSubMchID), url.PathEscape(normalizedApplicationNo))
	if normalizedAccountNumberRule != "" {
		requestURL += "?account_number_rule=" + url.QueryEscape(normalizedAccountNumberRule)
	}

	respBody, requestID, err := c.doRequestWithRequestID(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		wrappedErr := wrapSubMerchantSettlementApplicationQueryError(err)
		evt := querySubMerchantSettlementApplicationLogEvent(requestID, normalizedSubMchID, normalizedApplicationNo, normalizedAccountNumberRule)
		var wxErr *WechatPayError
		if errors.As(err, &wxErr) {
			evt = evt.
				Int("status_code", wxErr.StatusCode).
				Str("wechat_code", wxErr.Code).
				Str("wechat_message", wxErr.Message).
				Str("wechat_detail", wxErr.Detail)
		}
		evt.Err(wrappedErr).Msg("wechat sub merchant settlement application query failed")
		return nil, wrappedErr
	}

	var resp wechatcontracts.QuerySubMerchantSettlementApplicationResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		wrappedErr := fmt.Errorf("query sub merchant settlement application: request_id=%s: decode response: %w", requestID, err)
		querySubMerchantSettlementApplicationLogEvent(requestID, normalizedSubMchID, normalizedApplicationNo, normalizedAccountNumberRule).
			Err(wrappedErr).
			Msg("wechat sub merchant settlement application response decode failed")
		return nil, wrappedErr
	}
	if err := validateSubMerchantSettlementApplicationResponse(&resp); err != nil {
		wrappedErr := fmt.Errorf("query sub merchant settlement application: request_id=%s: %w", requestID, err)
		querySubMerchantSettlementApplicationLogEvent(requestID, normalizedSubMchID, normalizedApplicationNo, normalizedAccountNumberRule).
			Str("account_type", resp.AccountType).
			Str("verify_result", resp.VerifyResult).
			Bool("has_verify_fail_reason", strings.TrimSpace(resp.VerifyFailReason) != "").
			Bool("has_verify_finish_time", strings.TrimSpace(resp.VerifyFinishTime) != "").
			Err(wrappedErr).
			Msg("wechat sub merchant settlement application response contract validation failed")
		return nil, wrappedErr
	}

	return &resp, nil
}

// ==================== 回调通知解密 ====================

// DecryptCombinePaymentNotification 解密合单支付通知
func (c *EcommerceClient) DecryptCombinePaymentNotification(notification *PaymentNotification) (*CombinePaymentNotification, error) {
	plaintext, err := c.decryptAESGCM(
		notification.Resource.Nonce,
		notification.Resource.Ciphertext,
		notification.Resource.AssociatedData,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt notification: %w", err)
	}

	var result CombinePaymentNotification
	if err := json.Unmarshal(plaintext, &result); err != nil {
		return nil, fmt.Errorf("unmarshal notification: %w", err)
	}

	return &result, nil
}

// DecryptPartnerPaymentNotification 解密服务商模式单笔支付通知。
func (c *EcommerceClient) DecryptPartnerPaymentNotification(notification *PaymentNotification) (*PartnerPaymentNotificationResource, error) {
	plaintext, err := c.decryptAESGCM(
		notification.Resource.Nonce,
		notification.Resource.Ciphertext,
		notification.Resource.AssociatedData,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt notification: %w", err)
	}

	var result PartnerPaymentNotificationResource
	if err := json.Unmarshal(plaintext, &result); err != nil {
		return nil, fmt.Errorf("unmarshal notification: %w", err)
	}

	return &result, nil
}

// ProfitSharingNotification 分账通知
type ProfitSharingNotification struct {
	MchID         string `json:"mchid"`
	SubMchID      string `json:"sub_mchid"`
	TransactionID string `json:"transaction_id"`
	OrderID       string `json:"order_id"`
	OutOrderNo    string `json:"out_order_no"`
	Receiver      struct {
		Type            string `json:"type"`
		ReceiverAccount string `json:"receiver_account"`
		Amount          int64  `json:"amount"`
		Description     string `json:"description"`
		Result          string `json:"result"`
		DetailID        string `json:"detail_id"`
		FinishTime      string `json:"finish_time"`
		FailReason      string `json:"fail_reason"`
	} `json:"receiver"`
	SuccessTime string `json:"success_time"`
}

// DecryptProfitSharingNotification 解密分账通知
func (c *EcommerceClient) DecryptProfitSharingNotification(notification *PaymentNotification) (*ProfitSharingNotification, error) {
	plaintext, err := c.decryptAESGCM(
		notification.Resource.Nonce,
		notification.Resource.Ciphertext,
		notification.Resource.AssociatedData,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt notification: %w", err)
	}

	var result ProfitSharingNotification
	if err := json.Unmarshal(plaintext, &result); err != nil {
		return nil, fmt.Errorf("unmarshal notification: %w", err)
	}

	return &result, nil
}

// EcommerceRefundNotification 电商退款通知
type EcommerceRefundNotification struct {
	SpMchID             string                `json:"sp_mchid"`
	SubMchID            string                `json:"sub_mchid"`
	TransactionID       string                `json:"transaction_id"`
	OutTradeNo          string                `json:"out_trade_no"`
	RefundID            string                `json:"refund_id"`
	OutRefundNo         string                `json:"out_refund_no"`
	RefundStatus        string                `json:"refund_status"` // SUCCESS/CLOSED/ABNORMAL
	SuccessTime         string                `json:"success_time"`
	UserReceivedAccount string                `json:"user_received_account"`
	Amount              EcommerceRefundAmount `json:"amount"`
	RefundAccount       string                `json:"refund_account"`
}

// DecryptEcommerceRefundNotification 解密电商退款通知
func (c *EcommerceClient) DecryptEcommerceRefundNotification(notification *PaymentNotification) (*EcommerceRefundNotification, error) {
	plaintext, err := c.decryptAESGCM(
		notification.Resource.Nonce,
		notification.Resource.Ciphertext,
		notification.Resource.AssociatedData,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt notification: %w", err)
	}

	var result EcommerceRefundNotification
	if err := json.Unmarshal(plaintext, &result); err != nil {
		return nil, fmt.Errorf("unmarshal notification: %w", err)
	}

	return &result, nil
}

// SettlementNotificationResource 微信结算事件通知解密后的资源数据
// 事件类型：trade_manage_order_settlement
// 该事件由平台在用户确认收货或 T+2 自动确认后推送，settlement_time 非空代表资金已实际结算。
type SettlementNotificationResource struct {
	AppID                string `json:"appid"`
	TransactionID        string `json:"transaction_id"`         // 微信支付订单号（子单）
	MerchantTradeNo      string `json:"merchant_trade_no"`      // 商户子单号（我方 out_trade_no）
	ConfirmReceiveMethod int    `json:"confirm_receive_method"` // 1=用户手动确收, 2=T+2自动确收
	SettlementTime       string `json:"settlement_time"`        // 非空表示资金已结算
	SuccessTime          string `json:"success_time"`
}

// DecryptSettlementNotification 解密结算事件通知（trade_manage_order_settlement）
func (c *EcommerceClient) DecryptSettlementNotification(notification *PaymentNotification) (*SettlementNotificationResource, error) {
	plaintext, err := c.decryptAESGCM(
		notification.Resource.Nonce,
		notification.Resource.Ciphertext,
		notification.Resource.AssociatedData,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt notification: %w", err)
	}

	var result SettlementNotificationResource
	if err := json.Unmarshal(plaintext, &result); err != nil {
		return nil, fmt.Errorf("unmarshal notification: %w", err)
	}

	return &result, nil
}

// ==================== 工具方法 ====================

// doRequest 执行 HTTP 请求（复用基础客户端）
func (c *EcommerceClient) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	var bodyBytes []byte

	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	url := wxPayBaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 生成请求ID（用于追踪和问题排查）
	requestID, err := generateNonceStr()
	if err != nil {
		return nil, err
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Request-ID", requestID)

	// 生成签名
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonceStr, err := generateNonceStr()
	if err != nil {
		return nil, err
	}
	signature, err := c.generateSignature(method, path, timestamp, nonceStr, bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("generate signature: %w", err)
	}

	// 设置 Authorization 头
	authHeader := fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",timestamp="%s",serial_no="%s",signature="%s"`,
		c.mchID, nonceStr, timestamp, c.serialNo, signature)
	req.Header.Set("Authorization", authHeader)

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request (request_id=%s): %w", requestID, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 检查响应状态码
	if err := c.verifyHTTPResponseSignature(resp, respBody); err != nil {
		return nil, fmt.Errorf("verify response signature: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 尝试解析微信支付错误响应
		var wxErr WechatPayError
		wxErr.StatusCode = resp.StatusCode

		if err := json.Unmarshal(respBody, &wxErr); err == nil && wxErr.Code != "" {
			// 成功解析错误响应，添加request_id便于排查
			return nil, fmt.Errorf("request_id=%s: %w", requestID, &wxErr)
		}

		// 解析失败，返回原始错误
		return nil, fmt.Errorf("wechat pay api error: status=%d, body=%s, request_id=%s", resp.StatusCode, string(respBody), requestID)
	}

	return respBody, nil
}

// generateSignature 生成签名（复用基础客户端的私钥）
func (c *EcommerceClient) generateSignature(method, path, timestamp, nonceStr string, body []byte) (string, error) {
	message := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n",
		method, path, timestamp, nonceStr, string(body))

	hash := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(nil, c.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign message: %w", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// ==================== 图片上传 ====================

// ImageUploadResponse 图片上传响应
type ImageUploadResponse struct {
	MediaID string `json:"media_id"` // 媒体文件标识ID
}

// UploadImageValidationError represents a caller-fixable local validation failure
// before the WeChat merchant media upload request is sent.
type UploadImageValidationError struct {
	Message string
}

func (e *UploadImageValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "upload image validation failed"
	}
	return e.Message
}

func IsUploadImageValidationError(err error) bool {
	var target *UploadImageValidationError
	return errors.As(err, &target)
}

// UploadImage 上传图片到微信支付
// filename: 文件名（需要包含扩展名如 .jpg, .png）
// fileData: 文件二进制数据
// 返回 MediaID 用于进件申请
func (c *EcommerceClient) UploadImage(ctx context.Context, filename string, fileData []byte) (*ImageUploadResponse, error) {
	requestID, err := generateNonceStr()
	if err != nil {
		requestID = fallbackMerchantMediaUploadRequestID()
		wrappedErr := fmt.Errorf("upload image: failed to generate request id: %w", err)
		log.Error().
			Str("request_id", requestID).
			Str("filename", filename).
			Int("file_size", len(fileData)).
			Err(wrappedErr).
			Msg("wechat merchant media upload request id generation failed")
		return nil, fmt.Errorf("request_id=%s: %w", requestID, wrappedErr)
	}

	if !c.explicitSpMchID {
		err := errors.New("upload image: service provider merchant id must be configured explicitly for /v3/merchant/media/upload")
		log.Error().
			Str("request_id", requestID).
			Str("filename", filename).
			Int("file_size", len(fileData)).
			Bool("sp_mchid_explicit", c.explicitSpMchID).
			Err(err).
			Msg("wechat merchant media upload missing explicit service provider merchant id")
		return nil, fmt.Errorf("request_id=%s: %w", requestID, err)
	}

	normalizedFilename, contentType, err := validateMerchantMediaUploadImage(filename, fileData)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("filename", filename).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload validation failed")
		return nil, err
	}

	// 计算文件 SHA256 哈希
	fileHash := sha256.Sum256(fileData)
	sha256Hex := fmt.Sprintf("%x", fileHash)

	// 构造 meta 信息
	meta := map[string]string{
		"filename": normalizedFilename,
		"sha256":   sha256Hex,
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload meta marshal failed")
		return nil, fmt.Errorf("marshal meta: %w", err)
	}

	// 构造 multipart/form-data 请求体
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 添加 meta 字段
	metaHeader := make(textproto.MIMEHeader)
	metaHeader.Set("Content-Disposition", `form-data; name="meta"`)
	metaHeader.Set("Content-Type", "application/json")
	metaPart, err := writer.CreatePart(metaHeader)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload meta part creation failed")
		return nil, fmt.Errorf("create meta part: %w", err)
	}
	if _, err := metaPart.Write(metaBytes); err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload meta part write failed")
		return nil, fmt.Errorf("write meta part: %w", err)
	}

	// 添加 file 字段
	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, normalizedFilename))
	fileHeader.Set("Content-Type", contentType)
	filePart, err := writer.CreatePart(fileHeader)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Str("content_type", contentType).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload file part creation failed")
		return nil, fmt.Errorf("create file part: %w", err)
	}
	if _, err := io.Copy(filePart, bytes.NewReader(fileData)); err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Str("content_type", contentType).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload file part write failed")
		return nil, fmt.Errorf("write file part: %w", err)
	}

	if err := writer.Close(); err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Str("content_type", contentType).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload multipart close failed")
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	// 发送请求
	url := wxPayBaseURL + merchantMediaUploadURL
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Str("content_type", contentType).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload request creation failed")
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Request-ID", requestID)

	// 生成签名（对于文件上传，body 使用 meta JSON）
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonceStr, err := generateNonceStr()
	if err != nil {
		wrappedErr := fmt.Errorf("upload image: failed to generate signing nonce: %w", err)
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Str("content_type", contentType).
			Int("file_size", len(fileData)).
			Err(wrappedErr).
			Msg("wechat merchant media upload signing nonce generation failed")
		return nil, fmt.Errorf("request_id=%s: %w", requestID, wrappedErr)
	}
	signature, err := c.generateSignature(http.MethodPost, merchantMediaUploadURL, timestamp, nonceStr, metaBytes)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Str("content_type", contentType).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload signature generation failed")
		return nil, fmt.Errorf("generate signature: %w", err)
	}

	// 设置 Authorization 头
	authHeader := fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",timestamp="%s",serial_no="%s",signature="%s"`,
		c.spMchID, nonceStr, timestamp, c.serialNo, signature)
	req.Header.Set("Authorization", authHeader)

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Str("content_type", contentType).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload request failed")
		return nil, fmt.Errorf("send request (request_id=%s): %w", requestID, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Str("content_type", contentType).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload response read failed")
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 检查响应状态码
	if err := c.verifyHTTPResponseSignature(resp, respBody); err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Str("content_type", contentType).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload response signature verification failed")
		return nil, fmt.Errorf("verify response signature (request_id=%s): %w", requestID, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var wxErr WechatPayError
		wxErr.StatusCode = resp.StatusCode
		if err := json.Unmarshal(respBody, &wxErr); err == nil && wxErr.Code != "" {
			log.Error().
				Str("request_id", requestID).
				Str("filename", normalizedFilename).
				Str("content_type", contentType).
				Int("file_size", len(fileData)).
				Int("status_code", resp.StatusCode).
				Str("wechat_code", wxErr.Code).
				Str("wechat_message", wxErr.Message).
				Str("wechat_detail", wxErr.Detail).
				Msg("wechat merchant media upload rejected by upstream")
			return nil, fmt.Errorf("request_id=%s: %s: %w", requestID, uploadImageWechatErrorGuide(&wxErr), &wxErr)
		}
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Str("content_type", contentType).
			Int("file_size", len(fileData)).
			Int("status_code", resp.StatusCode).
			Bytes("response_body", respBody).
			Msg("wechat merchant media upload failed with unparseable upstream error")
		return nil, fmt.Errorf("wechat pay api error: status=%d, body=%s, request_id=%s", resp.StatusCode, string(respBody), requestID)
	}

	var result ImageUploadResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Str("content_type", contentType).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload response decode failed")
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if strings.TrimSpace(result.MediaID) == "" {
		err := errors.New("upload image: wechat response missing media_id")
		log.Error().
			Str("request_id", requestID).
			Str("filename", normalizedFilename).
			Str("content_type", contentType).
			Int("file_size", len(fileData)).
			Err(err).
			Msg("wechat merchant media upload response missing media_id")
		return nil, fmt.Errorf("request_id=%s: %w", requestID, err)
	}

	return &result, nil
}

func validateMerchantMediaUploadImage(filename string, fileData []byte) (string, string, error) {
	normalizedFilename := strings.TrimSpace(filepath.Base(filename))
	if normalizedFilename == "" || normalizedFilename == "." {
		return "", "", newUploadImageValidationError("filename is required and must end with .jpg, .jpeg, .png, or .bmp")
	}
	if len(fileData) == 0 {
		return "", "", newUploadImageValidationError("file is empty; provide a non-empty JPG, JPEG, PNG, or BMP image")
	}
	if len(fileData) > merchantMediaUploadMaxBytes {
		return "", "", newUploadImageValidationError("file size %d exceeds the 2MB WeChat merchant media upload limit; compress the image and retry", len(fileData))
	}

	ext := strings.ToLower(filepath.Ext(normalizedFilename))
	switch ext {
	case ".jpg", ".jpeg":
		if err := validateDecodedImageFormat(fileData, "jpeg"); err != nil {
			return "", "", newUploadImageValidationError("file content does not match %s; provide a real JPEG image", ext)
		}
		return normalizedFilename, "image/jpeg", nil
	case ".png":
		if err := validateDecodedImageFormat(fileData, "png"); err != nil {
			return "", "", newUploadImageValidationError("file content does not match .png; provide a real PNG image")
		}
		return normalizedFilename, "image/png", nil
	case ".bmp":
		if err := validateBMPImage(fileData); err != nil {
			return "", "", newUploadImageValidationError("file content does not match .bmp; provide a real BMP image")
		}
		return normalizedFilename, "image/bmp", nil
	default:
		return "", "", newUploadImageValidationError("unsupported file extension %q; only .jpg, .jpeg, .png, and .bmp are allowed", ext)
	}
}

func validateDecodedImageFormat(fileData []byte, expectedFormat string) error {
	config, format, err := image.DecodeConfig(bytes.NewReader(fileData))
	if err != nil {
		return err
	}
	if config.Width <= 0 || config.Height <= 0 {
		return errors.New("image has invalid dimensions")
	}
	if format != expectedFormat {
		return fmt.Errorf("decoded image format %q does not match expected %q", format, expectedFormat)
	}
	return nil
}

func validateBMPImage(fileData []byte) error {
	if len(fileData) < 54 {
		return errors.New("bmp payload is too small")
	}
	if fileData[0] != 'B' || fileData[1] != 'M' {
		return errors.New("bmp signature is missing")
	}
	declaredSize := binary.LittleEndian.Uint32(fileData[2:6])
	if declaredSize < 54 || declaredSize > uint32(len(fileData)) {
		return errors.New("bmp file size header is invalid")
	}
	pixelOffset := binary.LittleEndian.Uint32(fileData[10:14])
	if pixelOffset < 54 || pixelOffset > uint32(len(fileData)) {
		return errors.New("bmp pixel offset is invalid")
	}
	dibHeaderSize := binary.LittleEndian.Uint32(fileData[14:18])
	if dibHeaderSize < 40 {
		return errors.New("bmp dib header is invalid")
	}
	width := int32(binary.LittleEndian.Uint32(fileData[18:22]))
	height := int32(binary.LittleEndian.Uint32(fileData[22:26]))
	if width <= 0 || height == 0 {
		return errors.New("bmp dimensions are invalid")
	}
	if binary.LittleEndian.Uint16(fileData[28:30]) == 0 {
		return errors.New("bmp bits per pixel is invalid")
	}
	return nil
}

func newUploadImageValidationError(format string, args ...any) error {
	return &UploadImageValidationError{Message: fmt.Sprintf("upload image: "+format, args...)}
}

func fallbackMerchantMediaUploadRequestID() string {
	return fmt.Sprintf("merchant-media-upload-%d", time.Now().UnixNano())
}

func uploadImageWechatErrorGuide(wxErr *WechatPayError) string {
	if wxErr == nil {
		return "upload image failed"
	}
	switch wxErr.Code {
	case "PARAM_ERROR":
		return "upload image rejected by wechat; verify filename suffix, image content, sha256, and file size before retrying"
	case "NO_AUTH":
		return "upload image rejected by wechat; confirm the service provider merchant has media upload permission"
	case "FREQUENCY_LIMIT_EXCEED":
		return "upload image rejected by wechat due to frequency limits; retry later with a lower request rate"
	case "SIGN_ERROR":
		return "upload image rejected by wechat because signature verification failed; verify merchant credentials and signing inputs"
	case "SYSTEM_ERROR":
		return "upload image failed because wechat reported a system error; retry later with the same parameters"
	default:
		return fmt.Sprintf("upload image failed with wechat error code %s", wxErr.Code)
	}
}
