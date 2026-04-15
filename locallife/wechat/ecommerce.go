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

type EcommerceCancelWithdrawAccountInfo struct {
	OutAccountType string `json:"out_account_type"`
	Amount         int64  `json:"amount"`
}

type EcommerceCancelWithdrawBlockReason struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type EcommerceCancelWithdrawEligibilityResponse struct {
	SubMchID       string                               `json:"sub_mchid"`
	MerchantState  string                               `json:"merchant_state"`
	ValidateResult string                               `json:"validate_result"`
	AccountInfo    []EcommerceCancelWithdrawAccountInfo `json:"account_info,omitempty"`
	BlockReasons   []EcommerceCancelWithdrawBlockReason `json:"block_reasons,omitempty"`
}

type EcommerceCancelWithdrawIdentityInfo struct {
	IDDocType          string `json:"id_doc_type,omitempty"`
	IdentificationName string `json:"identification_name,omitempty"`
	IdentificationNo   string `json:"identification_no,omitempty"`
}

type EcommerceCancelWithdrawBankAccountInfo struct {
	AccountName    string `json:"account_name,omitempty"`
	AccountBank    string `json:"account_bank,omitempty"`
	BankBranchID   string `json:"bank_branch_id,omitempty"`
	BankBranchName string `json:"bank_branch_name,omitempty"`
	AccountNumber  string `json:"account_number,omitempty"`
}

type EcommerceCancelWithdrawPayeeInfo struct {
	AccountType     string                                  `json:"account_type,omitempty"`
	BankAccountInfo *EcommerceCancelWithdrawBankAccountInfo `json:"bank_account_info,omitempty"`
	IdentityInfo    *EcommerceCancelWithdrawIdentityInfo    `json:"identity_info,omitempty"`
}

type EcommerceCancelWithdrawProofMedia struct {
	ProofMediaType string `json:"proof_media_type"`
	ProofMedia     string `json:"proof_media"`
}

type EcommerceCancelWithdrawRequest struct {
	SubMchID            string                              `json:"sub_mchid"`
	OutRequestNo        string                              `json:"out_request_no"`
	Withdraw            string                              `json:"withdraw,omitempty"`
	PayeeInfo           *EcommerceCancelWithdrawPayeeInfo   `json:"payee_info,omitempty"`
	ProofMedias         []EcommerceCancelWithdrawProofMedia `json:"proof_medias,omitempty"`
	AdditionalMaterials []string                            `json:"additional_materials,omitempty"`
	Remark              string                              `json:"remark,omitempty"`
}

type EcommerceCancelWithdrawCreateResponse struct {
	ApplymentID  string `json:"applyment_id"`
	OutRequestNo string `json:"out_request_no"`
}

type EcommerceCancelWithdrawAccountWithdrawResult struct {
	OutAccountType   string `json:"out_account_type"`
	PayState         string `json:"pay_state"`
	StateDescription string `json:"state_description"`
}

type EcommerceCancelWithdrawConfirmCancel struct {
	ConfirmCancelURL string `json:"confirm_cancel_url,omitempty"`
}

type EcommerceCancelWithdrawQueryResponse struct {
	ApplymentID              string                                         `json:"applyment_id"`
	OutRequestNo             string                                         `json:"out_request_no"`
	CancelState              string                                         `json:"cancel_state"`
	CancelStateDescription   string                                         `json:"cancel_state_description"`
	Withdraw                 string                                         `json:"withdraw,omitempty"`
	WithdrawState            string                                         `json:"withdraw_state,omitempty"`
	WithdrawStateDescription string                                         `json:"withdraw_state_description,omitempty"`
	AccountWithdrawResult    []EcommerceCancelWithdrawAccountWithdrawResult `json:"account_withdraw_result,omitempty"`
	ModifyTime               string                                         `json:"modify_time,omitempty"`
	SubMchID                 string                                         `json:"sub_mchid"`
	AccountInfo              []EcommerceCancelWithdrawAccountInfo           `json:"account_info,omitempty"`
	ConfirmCancel            *EcommerceCancelWithdrawConfirmCancel          `json:"confirm_cancel,omitempty"`
}

// ValidateEcommerceCancelWithdraw 校验二级商户是否满足注销提现条件
func (c *EcommerceClient) ValidateEcommerceCancelWithdraw(ctx context.Context, subMchID string) (*EcommerceCancelWithdrawEligibilityResponse, error) {
	trimmedSubMchID, err := validateMerchantCancelWithdrawIdentifier("validate merchant cancel withdraw", "sub_mchid", subMchID)
	if err != nil {
		return nil, err
	}

	requestURL := fmt.Sprintf(ecommerceCancelWithdrawValidateURL, url.PathEscape(trimmedSubMchID))
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("validate ecommerce cancel withdraw: %w", err)
	}

	var resp EcommerceCancelWithdrawEligibilityResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.SubMchID == "" {
		resp.SubMchID = trimmedSubMchID
	}
	if err := validateMerchantCancelWithdrawEligibilityResponse(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateEcommerceCancelWithdraw 提交商户注销提现申请
func (c *EcommerceClient) CreateEcommerceCancelWithdraw(ctx context.Context, req *EcommerceCancelWithdrawRequest) (*EcommerceCancelWithdrawCreateResponse, error) {
	if err := validateMerchantCancelWithdrawCreateRequest(req); err != nil {
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

	var resp EcommerceCancelWithdrawCreateResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.OutRequestNo == "" {
		resp.OutRequestNo = strings.TrimSpace(req.OutRequestNo)
	}
	return &resp, nil
}

// QueryEcommerceCancelWithdrawByOutRequestNo 按平台申请单号查询注销提现申请状态
func (c *EcommerceClient) QueryEcommerceCancelWithdrawByOutRequestNo(ctx context.Context, outRequestNo string) (*EcommerceCancelWithdrawQueryResponse, error) {
	trimmedOutRequestNo, err := validateMerchantCancelWithdrawIdentifier("query merchant cancel withdraw by out_request_no", "out_request_no", outRequestNo)
	if err != nil {
		return nil, err
	}

	requestURL := fmt.Sprintf(ecommerceCancelWithdrawQueryByNoURL, url.PathEscape(trimmedOutRequestNo))
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query ecommerce cancel withdraw by out request no: %w", err)
	}

	var resp EcommerceCancelWithdrawQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.OutRequestNo == "" {
		resp.OutRequestNo = trimmedOutRequestNo
	}
	if err := validateMerchantCancelWithdrawQueryResponse("query merchant cancel withdraw by out_request_no", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// QueryEcommerceCancelWithdrawByApplymentID 按微信申请单号查询注销提现申请状态
func (c *EcommerceClient) QueryEcommerceCancelWithdrawByApplymentID(ctx context.Context, applymentID string) (*EcommerceCancelWithdrawQueryResponse, error) {
	trimmedApplymentID, err := validateMerchantCancelWithdrawIdentifier("query merchant cancel withdraw by applyment_id", "applyment_id", applymentID)
	if err != nil {
		return nil, err
	}

	requestURL := fmt.Sprintf(ecommerceCancelWithdrawQueryByIDURL, url.PathEscape(trimmedApplymentID))
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("query ecommerce cancel withdraw by applyment id: %w", err)
	}

	var resp EcommerceCancelWithdrawQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.ApplymentID == "" {
		resp.ApplymentID = trimmedApplymentID
	}
	if err := validateMerchantCancelWithdrawQueryResponse("query merchant cancel withdraw by applyment_id", &resp); err != nil {
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

// PartnerJSAPIOrderRequest 服务商模式单笔 JSAPI 下单请求。
type PartnerJSAPIOrderRequest struct {
	SubMchID       string
	SubAppID       string
	Description    string
	OutTradeNo     string
	ExpireTime     time.Time
	Attach         string
	GoodsTag       string
	TotalAmount    int64
	Currency       string
	NotifyURL      string
	PayerOpenID    string
	PayerSubOpenID string
	PayerClientIP  string
	DeviceID       string
	ProfitSharing  bool
	SupportFapiao  *bool
}

// PartnerJSAPIOrderResponse 服务商模式单笔 JSAPI 下单响应。
type PartnerJSAPIOrderResponse struct {
	PrepayID string `json:"prepay_id"`
}

// PartnerOrderPayerInfo 服务商模式支付者信息。
type PartnerOrderPayerInfo struct {
	SpOpenID  string `json:"sp_openid,omitempty"`
	SubOpenID string `json:"sub_openid,omitempty"`
}

// PartnerOrderSceneInfo 服务商模式场景信息。
type PartnerOrderSceneInfo struct {
	PayerClientIP string `json:"payer_client_ip,omitempty"`
	DeviceID      string `json:"device_id,omitempty"`
}

// PartnerOrderQueryResponse 服务商模式单笔支付查询响应。
type PartnerOrderQueryResponse struct {
	SpAppID        string                `json:"sp_appid"`
	SpMchID        string                `json:"sp_mchid"`
	SubAppID       string                `json:"sub_appid,omitempty"`
	SubMchID       string                `json:"sub_mchid"`
	OutTradeNo     string                `json:"out_trade_no"`
	TransactionID  string                `json:"transaction_id,omitempty"`
	TradeType      string                `json:"trade_type,omitempty"`
	TradeState     string                `json:"trade_state"`
	TradeStateDesc string                `json:"trade_state_desc"`
	BankType       string                `json:"bank_type,omitempty"`
	Attach         string                `json:"attach,omitempty"`
	SuccessTime    string                `json:"success_time,omitempty"`
	Payer          PartnerOrderPayerInfo `json:"payer,omitempty"`
	Amount         struct {
		Total         int64  `json:"total"`
		PayerTotal    int64  `json:"payer_total"`
		Currency      string `json:"currency"`
		PayerCurrency string `json:"payer_currency"`
	} `json:"amount,omitempty"`
	SceneInfo *struct {
		DeviceID string `json:"device_id"`
	} `json:"scene_info,omitempty"`
	PromotionDetail []PartnerPromotionDetail `json:"promotion_detail,omitempty"`
}

type PartnerPromotionDetail struct {
	CouponID            string                        `json:"coupon_id"`
	Name                string                        `json:"name,omitempty"`
	Scope               string                        `json:"scope,omitempty"`
	Type                string                        `json:"type,omitempty"`
	Amount              int64                         `json:"amount"`
	StockID             string                        `json:"stock_id,omitempty"`
	WechatpayContribute int64                         `json:"wechatpay_contribute,omitempty"`
	MerchantContribute  int64                         `json:"merchant_contribute,omitempty"`
	OtherContribute     int64                         `json:"other_contribute,omitempty"`
	Currency            string                        `json:"currency,omitempty"`
	GoodsDetail         []PartnerPromotionGoodsDetail `json:"goods_detail,omitempty"`
}

type PartnerPromotionGoodsDetail struct {
	GoodsID        string `json:"goods_id"`
	Quantity       int64  `json:"quantity"`
	UnitPrice      int64  `json:"unit_price"`
	DiscountAmount int64  `json:"discount_amount"`
	GoodsRemark    string `json:"goods_remark,omitempty"`
}

// PartnerPaymentNotificationResource 服务商模式单笔支付成功回调资源。
type PartnerPaymentNotificationResource struct {
	SpAppID        string                `json:"sp_appid"`
	SpMchID        string                `json:"sp_mchid"`
	SubAppID       string                `json:"sub_appid,omitempty"`
	SubMchID       string                `json:"sub_mchid"`
	OutTradeNo     string                `json:"out_trade_no"`
	TransactionID  string                `json:"transaction_id"`
	TradeType      string                `json:"trade_type"`
	TradeState     string                `json:"trade_state"`
	TradeStateDesc string                `json:"trade_state_desc"`
	BankType       string                `json:"bank_type"`
	Attach         string                `json:"attach,omitempty"`
	SuccessTime    string                `json:"success_time"`
	Payer          PartnerOrderPayerInfo `json:"payer"`
	Amount         struct {
		Total         int64  `json:"total"`
		PayerTotal    int64  `json:"payer_total"`
		Currency      string `json:"currency"`
		PayerCurrency string `json:"payer_currency"`
	} `json:"amount"`
	SceneInfo *struct {
		DeviceID string `json:"device_id"`
	} `json:"scene_info,omitempty"`
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
	if strings.TrimSpace(req.DeviceID) != "" && strings.TrimSpace(req.PayerClientIP) == "" {
		return nil, nil, fmt.Errorf("create partner jsapi order: payer_client_ip is required when scene_info.device_id is provided")
	}

	currency := req.Currency
	if currency == "" {
		currency = "CNY"
	}
	notifyURL := req.NotifyURL
	if notifyURL == "" {
		notifyURL = c.partnerNotifyURL
	}
	body := map[string]interface{}{
		"sp_appid":     c.spAppID,
		"sp_mchid":     c.spMchID,
		"sub_mchid":    req.SubMchID,
		"description":  req.Description,
		"out_trade_no": req.OutTradeNo,
		"notify_url":   notifyURL,
		"amount": map[string]interface{}{
			"total":    req.TotalAmount,
			"currency": currency,
		},
		"payer": map[string]interface{}{},
		"settle_info": map[string]interface{}{
			"profit_sharing": req.ProfitSharing,
		},
	}
	if !req.ExpireTime.IsZero() {
		body["time_expire"] = req.ExpireTime.Format(time.RFC3339)
	}
	if req.Attach != "" {
		body["attach"] = req.Attach
	}
	if req.GoodsTag != "" {
		body["goods_tag"] = req.GoodsTag
	}
	if req.SupportFapiao != nil {
		body["support_fapiao"] = *req.SupportFapiao
	}
	payer := body["payer"].(map[string]interface{})
	if req.PayerOpenID != "" {
		payer["sp_openid"] = req.PayerOpenID
	}
	if req.PayerClientIP != "" || req.DeviceID != "" {
		sceneInfo := map[string]interface{}{}
		if req.PayerClientIP != "" {
			sceneInfo["payer_client_ip"] = req.PayerClientIP
		}
		if req.DeviceID != "" {
			sceneInfo["device_id"] = req.DeviceID
		}
		body["scene_info"] = sceneInfo
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
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// QueryPartnerOrderByOutTradeNo 通过商户订单号查询服务商模式单笔订单。
func (c *EcommerceClient) QueryPartnerOrderByOutTradeNo(ctx context.Context, outTradeNo, subMchID string) (*PartnerOrderQueryResponse, error) {
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
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// ClosePartnerOrder 关闭服务商模式单笔订单。
func (c *EcommerceClient) ClosePartnerOrder(ctx context.Context, outTradeNo, subMchID string) error {
	body := map[string]interface{}{
		"sp_mchid":  c.spMchID,
		"sub_mchid": subMchID,
	}
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
	OutRequestNo         string                    `json:"out_request_no"`                   // 业务申请编号
	OrganizationType     string                    `json:"organization_type"`                // 主体类型: 2401-小微, 2500-个人卖家, 4-个体工商户, 2-企业, 3-事业单位, 2502-政府机关, 1708-社会组织
	FinanceInstitution   bool                      `json:"finance_institution"`              // 是否金融机构
	BusinessLicense      *BusinessLicenseInfo      `json:"business_license_info,omitempty"`  // 营业执照信息（个体户/企业必填）
	IDCardInfo           *ApplymentIDCardInfo      `json:"id_card_info"`                     // 法人身份证信息
	AccountInfo          *ApplymentBankAccountInfo `json:"account_info,omitempty"`           // 结算银行账户
	ContactInfo          *ApplymentContactInfo     `json:"contact_info"`                     // 联系人信息
	SalesSceneInfo       *ApplymentSalesSceneInfo  `json:"sales_scene_info"`                 // 经营场景信息
	SettlementInfo       *ApplymentSettlementInfo  `json:"settlement_info,omitempty"`        // 结算规则
	MerchantShortname    string                    `json:"merchant_shortname"`               // 商户简称
	Qualifications       []string                  `json:"qualifications,omitempty"`         // 特殊资质
	BusinessAdditionPics []string                  `json:"business_addition_pics,omitempty"` // 补充材料
	BusinessAdditionDesc string                    `json:"business_addition_desc,omitempty"` // 补充说明
}

// BusinessLicenseInfo 营业执照信息
type BusinessLicenseInfo struct {
	CertType              string `json:"cert_type,omitempty"`       // 证书类型（政府/事业单位/社会组织）
	BusinessLicenseCopy   string `json:"business_license_copy"`     // 营业执照照片MediaID
	BusinessLicenseNumber string `json:"business_license_number"`   // 营业执照注册号
	MerchantName          string `json:"merchant_name"`             // 商户名称
	LegalPerson           string `json:"legal_person"`              // 法人姓名
	CompanyAddress        string `json:"company_address,omitempty"` // 注册地址
	BusinessTime          string `json:"business_time,omitempty"`   // 营业期限
}

// ApplymentIDCardInfo 进件身份证信息
type ApplymentIDCardInfo struct {
	IDCardCopy           string `json:"id_card_copy"`             // 身份证正面照片MediaID
	IDCardNational       string `json:"id_card_national"`         // 身份证背面照片MediaID
	IDCardName           string `json:"id_card_name"`             // 身份证姓名（需加密）
	IDCardNumber         string `json:"id_card_number"`           // 身份证号码（需加密）
	IDCardValidTimeBegin string `json:"id_card_valid_time_begin"` // 身份证有效期开始时间
	IDCardValidTime      string `json:"id_card_valid_time"`       // 身份证有效期结束时间：YYYY-MM-DD 或 长期
}

// ApplymentBankAccountInfo 进件银行账户信息
type ApplymentBankAccountInfo struct {
	BankAccountType string `json:"bank_account_type"`           // ACCOUNT_TYPE_BUSINESS-对公, ACCOUNT_TYPE_PRIVATE-对私
	AccountBank     string `json:"account_bank"`                // 开户银行
	AccountName     string `json:"account_name"`                // 开户名称（需加密）
	BankAddressCode string `json:"bank_address_code,omitempty"` // 开户银行省市编码
	BankBranchID    string `json:"bank_branch_id,omitempty"`    // 开户银行联行号
	BankName        string `json:"bank_name,omitempty"`         // 开户银行全称（支行）
	AccountNumber   string `json:"account_number"`              // 银行账号（需加密）
}

// CapitalBank 开户银行选项
type CapitalBank struct {
	BankAlias       string `json:"bank_alias"`
	BankAliasCode   string `json:"bank_alias_code"`
	AccountBank     string `json:"account_bank"`
	AccountBankCode int64  `json:"account_bank_code"`
	NeedBankBranch  bool   `json:"need_bank_branch"`
}

// CapitalBankListLinks 分页链接
type CapitalBankListLinks struct {
	Next string `json:"next,omitempty"`
	Prev string `json:"prev,omitempty"`
	Self string `json:"self,omitempty"`
}

// CapitalBankListResponse 银行列表响应
type CapitalBankListResponse struct {
	TotalCount int                  `json:"total_count"`
	Count      int                  `json:"count"`
	Data       []CapitalBank        `json:"data,omitempty"`
	Offset     int                  `json:"offset"`
	Links      CapitalBankListLinks `json:"links"`
}

// CapitalBankAccountSearchResponse 银行卡开户银行识别响应
type CapitalBankAccountSearchResponse struct {
	TotalCount int           `json:"total_count"`
	Data       []CapitalBank `json:"data,omitempty"`
}

// CapitalProvince 省份选项
type CapitalProvince struct {
	ProvinceName string `json:"province_name"`
	ProvinceCode int    `json:"province_code"`
}

// CapitalProvinceListResponse 省份列表响应
type CapitalProvinceListResponse struct {
	Data       []CapitalProvince `json:"data,omitempty"`
	TotalCount int               `json:"total_count"`
}

// CapitalCity 城市选项
type CapitalCity struct {
	CityName string `json:"city_name"`
	CityCode int    `json:"city_code"`
}

// CapitalCityListResponse 城市列表响应
type CapitalCityListResponse struct {
	Data       []CapitalCity `json:"data,omitempty"`
	TotalCount int           `json:"total_count"`
}

// CapitalBranch 开户支行选项
type CapitalBranch struct {
	BankBranchName string `json:"bank_branch_name"`
	BankBranchID   string `json:"bank_branch_id"`
}

// CapitalBranchListResponse 支行列表响应
type CapitalBranchListResponse struct {
	TotalCount      int                  `json:"total_count"`
	Count           int                  `json:"count"`
	Data            []CapitalBranch      `json:"data,omitempty"`
	Offset          int                  `json:"offset"`
	Links           CapitalBankListLinks `json:"links"`
	AccountBank     string               `json:"account_bank"`
	AccountBankCode int64                `json:"account_bank_code"`
	BankAlias       string               `json:"bank_alias"`
	BankAliasCode   string               `json:"bank_alias_code"`
}

// ApplymentContactInfo 联系人信息
type ApplymentContactInfo struct {
	ContactType             string `json:"contact_type,omitempty"`                // 联系人类型: 65-法人, 66-经办人
	ContactName             string `json:"contact_name"`                          // 联系人姓名（需加密）
	ContactIDDocType        string `json:"contact_id_doc_type,omitempty"`         // 联系人证件类型
	ContactIDCardNumber     string `json:"contact_id_card_number,omitempty"`      // 联系人身份证号（需加密）
	ContactIDDocCopy        string `json:"contact_id_doc_copy,omitempty"`         // 联系人证件正面照片
	ContactIDDocCopyBack    string `json:"contact_id_doc_copy_back,omitempty"`    // 联系人证件反面照片
	ContactIDDocPeriodBegin string `json:"contact_id_doc_period_begin,omitempty"` // 联系人证件有效期开始时间
	ContactIDDocPeriodEnd   string `json:"contact_id_doc_period_end,omitempty"`   // 联系人证件有效期结束时间
	MobilePhone             string `json:"mobile_phone"`                          // 联系手机号（需加密）
}

// ApplymentSalesSceneInfo 经营场景信息
type ApplymentSalesSceneInfo struct {
	StoreName           string `json:"store_name"`                       // 店铺名称
	StoreURL            string `json:"store_url,omitempty"`              // 店铺链接
	StoreQRCode         string `json:"store_qr_code,omitempty"`          // 店铺二维码MediaID
	MiniProgramSubAppID string `json:"mini_program_sub_appid,omitempty"` // 小程序AppID
}

// ApplymentSettlementInfo 结算规则信息
type ApplymentSettlementInfo struct {
	SettlementID      int64  `json:"settlement_id,omitempty"`
	QualificationType string `json:"qualification_type,omitempty"`
}

// EcommerceApplymentResponse 二级商户进件响应
type EcommerceApplymentResponse struct {
	ApplymentID  int64  `json:"applyment_id"`   // 微信支付申请单号
	OutRequestNo string `json:"out_request_no"` // 业务申请编号
}

// EcommerceApplymentAccountValidation 汇款账户验证信息。
type EcommerceApplymentAccountValidation struct {
	AccountName              string `json:"account_name,omitempty"`
	AccountNo                string `json:"account_no,omitempty"`
	PayAmount                int64  `json:"pay_amount,omitempty"`
	DestinationAccountNumber string `json:"destination_account_number,omitempty"`
	DestinationAccountName   string `json:"destination_account_name,omitempty"`
	DestinationAccountBank   string `json:"destination_account_bank,omitempty"`
	City                     string `json:"city,omitempty"`
	Remark                   string `json:"remark,omitempty"`
	Deadline                 string `json:"deadline,omitempty"`
	RawAccountName           string `json:"-"`
	RawAccountNo             string `json:"-"`
}

func MarshalEcommerceApplymentAccountValidation(validation *EcommerceApplymentAccountValidation) []byte {
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

func UnmarshalEcommerceApplymentAccountValidation(raw []byte) (*EcommerceApplymentAccountValidation, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var validation EcommerceApplymentAccountValidation
	if err := json.Unmarshal(raw, &validation); err != nil {
		return nil, err
	}

	return &validation, nil
}

// EcommerceApplymentQueryResponse 二级商户进件查询响应
type EcommerceApplymentQueryResponse struct {
	ApplymentID        int64                                `json:"applyment_id"`                   // 微信支付申请单号
	OutRequestNo       string                               `json:"out_request_no"`                 // 业务申请编号
	ApplymentState     string                               `json:"applyment_state"`                // 申请状态
	ApplymentStateDesc string                               `json:"applyment_state_desc"`           // 申请状态描述
	SignURL            string                               `json:"sign_url,omitempty"`             // 签约链接
	SignState          string                               `json:"sign_state,omitempty"`           // 签约状态
	SubMchID           string                               `json:"sub_mchid,omitempty"`            // 特约商户号
	AccountValidation  *EcommerceApplymentAccountValidation `json:"account_validation,omitempty"`   // 汇款账户验证信息
	LegalValidationURL string                               `json:"legal_validation_url,omitempty"` // 法人扫码验证链接
	AuditDetail        []ApplymentAuditDetail               `json:"audit_detail,omitempty"`         // 驳回详情
}

// ApplymentAuditDetail 进件审核详情
type ApplymentAuditDetail struct {
	ParamName    string `json:"param_name"`    // 参数名称
	RejectReason string `json:"reject_reason"` // 驳回原因
}

const (
	ecommerceApplymentOutRequestNoMaxLength      = 124
	subMerchantSettlementMchIDLength             = 10
	subMerchantSettlementApplicationNoMaxLength  = 64
	subMerchantSettlementFieldMaxLength          = 128
	subMerchantSettlementAccountNameMaxLength    = 1024
	subMerchantSettlementFailReasonMaxLength     = 1024
	subMerchantSettlementAccountNumberRuleV1     = "ACCOUNT_NUMBER_RULE_MASK_V1"
	subMerchantSettlementAccountNumberRuleV2     = "ACCOUNT_NUMBER_RULE_MASK_V2"
	subMerchantSettlementVerifyResultSuccess     = "VERIFY_SUCCESS"
	subMerchantSettlementVerifyResultFail        = "VERIFY_FAIL"
	subMerchantSettlementVerifyResultVerifying   = "VERIFYING"
	subMerchantSettlementApplicationAuditSuccess = "AUDIT_SUCCESS"
	subMerchantSettlementApplicationAuditing     = "AUDITING"
	subMerchantSettlementApplicationAuditFail    = "AUDIT_FAIL"
)

type EcommerceApplymentQueryValidationError struct {
	Message string
}

func (e *EcommerceApplymentQueryValidationError) Error() string {
	return e.Message
}

type SubMerchantSettlementQueryValidationError struct {
	Message string
}

func (e *SubMerchantSettlementQueryValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query sub merchant settlement: validation failed"
	}
	return e.Message
}

type SubMerchantSettlementContractError struct {
	Message string
}

func (e *SubMerchantSettlementContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query sub merchant settlement: upstream contract validation failed"
	}
	return e.Message
}

type SubMerchantSettlementApplicationQueryValidationError struct {
	Message string
}

func (e *SubMerchantSettlementApplicationQueryValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query sub merchant settlement application: validation failed"
	}
	return e.Message
}

type SubMerchantSettlementApplicationContractError struct {
	Message string
}

func (e *SubMerchantSettlementApplicationContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query sub merchant settlement application: upstream contract validation failed"
	}
	return e.Message
}

type MerchantCancelWithdrawValidationError struct {
	Message string
}

func (e *MerchantCancelWithdrawValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "merchant cancel withdraw: validation failed"
	}
	return e.Message
}

type MerchantCancelWithdrawContractError struct {
	Message string
}

func (e *MerchantCancelWithdrawContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "merchant cancel withdraw: upstream contract validation failed"
	}
	return e.Message
}

type ecommerceApplymentQueryKind string

const (
	ecommerceApplymentQueryByIDKind           ecommerceApplymentQueryKind = "applyment_id"
	ecommerceApplymentQueryByOutRequestNoKind ecommerceApplymentQueryKind = "out_request_no"
)

var allowedEcommerceApplymentStates = map[string]struct{}{
	"CHECKING":            {},
	"ACCOUNT_NEED_VERIFY": {},
	"AUDITING":            {},
	"REJECTED":            {},
	"NEED_SIGN":           {},
	"FINISH":              {},
	"FROZEN":              {},
	"CANCELED":            {},
}

var allowedEcommerceApplymentSignStates = map[string]struct{}{
	"UNSIGNED":     {},
	"SIGNED":       {},
	"NOT_SIGNABLE": {},
}

var allowedSubMerchantSettlementAccountNumberRules = map[string]struct{}{
	subMerchantSettlementAccountNumberRuleV1: {},
	subMerchantSettlementAccountNumberRuleV2: {},
}

var allowedSubMerchantSettlementAccountTypes = map[string]struct{}{
	"ACCOUNT_TYPE_BUSINESS": {},
	"ACCOUNT_TYPE_PRIVATE":  {},
}

var allowedSubMerchantSettlementVerifyResults = map[string]struct{}{
	subMerchantSettlementVerifyResultSuccess:   {},
	subMerchantSettlementVerifyResultFail:      {},
	subMerchantSettlementVerifyResultVerifying: {},
}

var allowedSubMerchantSettlementApplicationVerifyResults = map[string]struct{}{
	subMerchantSettlementApplicationAuditSuccess: {},
	subMerchantSettlementApplicationAuditing:     {},
	subMerchantSettlementApplicationAuditFail:    {},
}

var allowedMerchantCancelWithdrawMerchantStates = map[string]struct{}{
	"NORMAL":             {},
	"HAS_BEEN_CANCELLED": {},
}

var allowedMerchantCancelWithdrawValidateResults = map[string]struct{}{
	"ALLOW_CANCEL_WITHDRAW":     {},
	"NOT_ALLOW_CANCEL_WITHDRAW": {},
}

var allowedMerchantCancelWithdrawBlockReasonTypes = map[string]struct{}{
	"CONSUMER_COMPLAINT_UNPROCESSED": {},
	"HAS_BLOCKING_CONTROL":           {},
	"FUNDS_PENDING_PROCESSING":       {},
	"OTHER_REASON":                   {},
}

var allowedMerchantCancelWithdrawModes = map[string]struct{}{
	"NOT_APPLY_WITHDRAW": {},
	"APPLY_WITHDRAW":     {},
}

var allowedMerchantCancelWithdrawAccountTypes = map[string]struct{}{
	"ACCOUNT_TYPE_CORPORATE": {},
	"ACCOUNT_TYPE_PERSONAL":  {},
}

var allowedMerchantCancelWithdrawIDDocTypes = map[string]struct{}{
	"IDENTIFICATION_TYPE_ID_CARD":                 {},
	"IDENTIFICATION_TYPE_OVERSEA_PASSPORT":        {},
	"IDENTIFICATION_TYPE_HONGKONG_PASSPORT":       {},
	"IDENTIFICATION_TYPE_MACAO_PASSPORT":          {},
	"IDENTIFICATION_TYPE_TAIWAN_PASSPORT":         {},
	"IDENTIFICATION_TYPE_FOREIGN_RESIDENT":        {},
	"IDENTIFICATION_TYPE_HONGKONG_MACAO_RESIDENT": {},
	"IDENTIFICATION_TYPE_TAIWAN_RESIDENT":         {},
}

var allowedMerchantCancelWithdrawStates = map[string]struct{}{
	"ACCEPTED":                 {},
	"REVIEWING":                {},
	"REJECTED":                 {},
	"WAITING_MERCHANT_CONFIRM": {},
	"REVOKED":                  {},
	"SYSTEM_PROCESSING":        {},
	"CANCELED":                 {},
	"FUND_PROCESSING":          {},
	"FINISH":                   {},
}

var allowedMerchantCancelWithdrawStatesWithWithdrawProgress = map[string]struct{}{
	"FUND_PROCESSING": {},
	"FINISH":          {},
}

var allowedMerchantCancelWithdrawWithdrawStates = map[string]struct{}{
	"WITHDRAW_PROCESSING": {},
	"WITHDRAW_EXCEPTION":  {},
	"WITHDRAW_SUCCEED":    {},
}

var allowedMerchantCancelWithdrawOutAccountTypes = map[string]struct{}{
	"BASIC_ACCOUNT":     {},
	"OPERATE_ACCOUNT":   {},
	"MARGIN_ACCOUNT":    {},
	"TRADE_FEE_ACCOUNT": {},
}

var allowedMerchantCancelWithdrawPayStates = map[string]struct{}{
	"PAY_PROCESSING": {},
	"PAY_SUCCEED":    {},
	"PAY_FAIL":       {},
	"BANK_REFUNDED":  {},
}

func newEcommerceApplymentQueryValidationError(format string, args ...any) error {
	return &EcommerceApplymentQueryValidationError{Message: fmt.Sprintf("query ecommerce applyment: "+format, args...)}
}

func newSubMerchantSettlementQueryValidationError(format string, args ...any) error {
	return &SubMerchantSettlementQueryValidationError{Message: fmt.Sprintf("query sub merchant settlement: "+format, args...)}
}

func newSubMerchantSettlementContractError(format string, args ...any) error {
	return &SubMerchantSettlementContractError{Message: fmt.Sprintf("query sub merchant settlement: "+format, args...)}
}

func newSubMerchantSettlementApplicationQueryValidationError(format string, args ...any) error {
	return &SubMerchantSettlementApplicationQueryValidationError{Message: fmt.Sprintf("query sub merchant settlement application: "+format, args...)}
}

func newSubMerchantSettlementApplicationContractError(format string, args ...any) error {
	return &SubMerchantSettlementApplicationContractError{Message: fmt.Sprintf("query sub merchant settlement application: "+format, args...)}
}

func newMerchantCancelWithdrawValidationError(operation string, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "merchant cancel withdraw"
	}
	return &MerchantCancelWithdrawValidationError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func newMerchantCancelWithdrawContractError(operation string, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "merchant cancel withdraw"
	}
	return &MerchantCancelWithdrawContractError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func validateMerchantCancelWithdrawIdentifier(operation string, fieldName string, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", newMerchantCancelWithdrawValidationError(operation, "%s is required", fieldName)
	}
	if utf8.RuneCountInString(trimmed) > 32 {
		return "", newMerchantCancelWithdrawValidationError(operation, "%s must not exceed 32 characters", fieldName)
	}
	return trimmed, nil
}

func validateMerchantCancelWithdrawCreateRequest(req *EcommerceCancelWithdrawRequest) error {
	if req == nil {
		return newMerchantCancelWithdrawValidationError("create merchant cancel withdraw", "request is nil")
	}
	trimmedSubMchID, err := validateMerchantCancelWithdrawIdentifier("create merchant cancel withdraw", "sub_mchid", req.SubMchID)
	if err != nil {
		return err
	}
	req.SubMchID = trimmedSubMchID
	trimmedOutRequestNo, err := validateMerchantCancelWithdrawIdentifier("create merchant cancel withdraw", "out_request_no", req.OutRequestNo)
	if err != nil {
		return err
	}
	for _, r := range trimmedOutRequestNo {
		if (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return newMerchantCancelWithdrawValidationError("create merchant cancel withdraw", "out_request_no must contain only letters and digits")
		}
	}
	req.OutRequestNo = trimmedOutRequestNo
	if req.Withdraw != "" {
		trimmedWithdraw := strings.TrimSpace(req.Withdraw)
		if _, ok := allowedMerchantCancelWithdrawModes[trimmedWithdraw]; !ok {
			return newMerchantCancelWithdrawValidationError("create merchant cancel withdraw", "unsupported withdraw %q", req.Withdraw)
		}
		req.Withdraw = trimmedWithdraw
	}
	if req.PayeeInfo != nil {
		trimmedAccountType := strings.TrimSpace(req.PayeeInfo.AccountType)
		if trimmedAccountType == "" {
			return newMerchantCancelWithdrawValidationError("create merchant cancel withdraw", "payee_info.account_type is required when payee_info is provided")
		}
		if _, ok := allowedMerchantCancelWithdrawAccountTypes[trimmedAccountType]; !ok {
			return newMerchantCancelWithdrawValidationError("create merchant cancel withdraw", "unsupported payee_info.account_type %q", req.PayeeInfo.AccountType)
		}
		req.PayeeInfo.AccountType = trimmedAccountType
		if req.PayeeInfo.IdentityInfo != nil {
			trimmedDocType := strings.TrimSpace(req.PayeeInfo.IdentityInfo.IDDocType)
			if trimmedDocType != "" {
				if _, ok := allowedMerchantCancelWithdrawIDDocTypes[trimmedDocType]; !ok {
					return newMerchantCancelWithdrawValidationError("create merchant cancel withdraw", "unsupported payee_info.identity_info.id_doc_type %q", req.PayeeInfo.IdentityInfo.IDDocType)
				}
				req.PayeeInfo.IdentityInfo.IDDocType = trimmedDocType
			}
		}
	}
	if len(req.AdditionalMaterials) > 10 {
		return newMerchantCancelWithdrawValidationError("create merchant cancel withdraw", "additional_materials must not exceed 10 items")
	}
	if utf8.RuneCountInString(strings.TrimSpace(req.Remark)) > 32 {
		return newMerchantCancelWithdrawValidationError("create merchant cancel withdraw", "remark must not exceed 32 characters")
	}
	req.Remark = strings.TrimSpace(req.Remark)
	return nil
}

func validateMerchantCancelWithdrawEligibilityResponse(resp *EcommerceCancelWithdrawEligibilityResponse) error {
	if resp == nil {
		return newMerchantCancelWithdrawContractError("validate merchant cancel withdraw", "empty wechat response")
	}
	if strings.TrimSpace(resp.SubMchID) == "" {
		return newMerchantCancelWithdrawContractError("validate merchant cancel withdraw", "wechat response missing sub_mchid")
	}
	if _, ok := allowedMerchantCancelWithdrawMerchantStates[strings.TrimSpace(resp.MerchantState)]; !ok {
		return newMerchantCancelWithdrawContractError("validate merchant cancel withdraw", "unsupported merchant_state %q", resp.MerchantState)
	}
	if _, ok := allowedMerchantCancelWithdrawValidateResults[strings.TrimSpace(resp.ValidateResult)]; !ok {
		return newMerchantCancelWithdrawContractError("validate merchant cancel withdraw", "unsupported validate_result %q", resp.ValidateResult)
	}
	for index, account := range resp.AccountInfo {
		if _, ok := allowedMerchantCancelWithdrawOutAccountTypes[strings.TrimSpace(account.OutAccountType)]; !ok {
			return newMerchantCancelWithdrawContractError("validate merchant cancel withdraw", "account_info[%d].out_account_type has unsupported value %q", index, account.OutAccountType)
		}
	}
	for index, reason := range resp.BlockReasons {
		trimmedType := strings.TrimSpace(reason.Type)
		if trimmedType == "" {
			continue
		}
		if _, ok := allowedMerchantCancelWithdrawBlockReasonTypes[trimmedType]; !ok {
			return newMerchantCancelWithdrawContractError("validate merchant cancel withdraw", "block_reasons[%d].type has unsupported value %q", index, reason.Type)
		}
	}
	return nil
}

func validateMerchantCancelWithdrawQueryResponse(operation string, resp *EcommerceCancelWithdrawQueryResponse) error {
	if resp == nil {
		return newMerchantCancelWithdrawContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.ApplymentID) == "" {
		return newMerchantCancelWithdrawContractError(operation, "wechat response missing applyment_id")
	}
	if strings.TrimSpace(resp.OutRequestNo) == "" {
		return newMerchantCancelWithdrawContractError(operation, "wechat response missing out_request_no")
	}
	if _, ok := allowedMerchantCancelWithdrawStates[strings.TrimSpace(resp.CancelState)]; !ok {
		return newMerchantCancelWithdrawContractError(operation, "unsupported cancel_state %q", resp.CancelState)
	}
	if strings.TrimSpace(resp.CancelStateDescription) == "" {
		return newMerchantCancelWithdrawContractError(operation, "wechat response missing cancel_state_description")
	}
	if strings.TrimSpace(resp.SubMchID) == "" {
		return newMerchantCancelWithdrawContractError(operation, "wechat response missing sub_mchid")
	}
	if trimmedWithdraw := strings.TrimSpace(resp.Withdraw); trimmedWithdraw != "" {
		if _, ok := allowedMerchantCancelWithdrawModes[trimmedWithdraw]; !ok {
			return newMerchantCancelWithdrawContractError(operation, "unsupported withdraw %q", resp.Withdraw)
		}
	}
	if trimmedWithdrawState := strings.TrimSpace(resp.WithdrawState); trimmedWithdrawState != "" {
		if _, ok := allowedMerchantCancelWithdrawWithdrawStates[trimmedWithdrawState]; !ok {
			return newMerchantCancelWithdrawContractError(operation, "unsupported withdraw_state %q", resp.WithdrawState)
		}
		if _, ok := allowedMerchantCancelWithdrawStatesWithWithdrawProgress[strings.TrimSpace(resp.CancelState)]; !ok {
			return newMerchantCancelWithdrawContractError(operation, "withdraw_state is only allowed after the request reaches a withdraw-processing state")
		}
	}
	if strings.TrimSpace(resp.ModifyTime) != "" {
		if _, err := time.Parse(time.RFC3339, resp.ModifyTime); err != nil {
			return newMerchantCancelWithdrawContractError(operation, "modify_time must be RFC3339: %v", err)
		}
	}
	for index, account := range resp.AccountInfo {
		if _, ok := allowedMerchantCancelWithdrawOutAccountTypes[strings.TrimSpace(account.OutAccountType)]; !ok {
			return newMerchantCancelWithdrawContractError(operation, "account_info[%d].out_account_type has unsupported value %q", index, account.OutAccountType)
		}
	}
	for index, result := range resp.AccountWithdrawResult {
		if _, ok := allowedMerchantCancelWithdrawOutAccountTypes[strings.TrimSpace(result.OutAccountType)]; !ok {
			return newMerchantCancelWithdrawContractError(operation, "account_withdraw_result[%d].out_account_type has unsupported value %q", index, result.OutAccountType)
		}
		if _, ok := allowedMerchantCancelWithdrawPayStates[strings.TrimSpace(result.PayState)]; !ok {
			return newMerchantCancelWithdrawContractError(operation, "account_withdraw_result[%d].pay_state has unsupported value %q", index, result.PayState)
		}
		if strings.TrimSpace(result.StateDescription) == "" {
			return newMerchantCancelWithdrawContractError(operation, "account_withdraw_result[%d].state_description is required", index)
		}
	}
	confirmCancelURL := ""
	if resp.ConfirmCancel != nil {
		confirmCancelURL = strings.TrimSpace(resp.ConfirmCancel.ConfirmCancelURL)
	}
	if confirmCancelURL != "" && strings.TrimSpace(resp.CancelState) != "WAITING_MERCHANT_CONFIRM" {
		return newMerchantCancelWithdrawContractError(operation, "confirm_cancel.confirm_cancel_url is only allowed when cancel_state=WAITING_MERCHANT_CONFIRM")
	}
	return nil
}

func normalizeEcommerceApplymentQueryState(state string) string {
	return strings.ToUpper(strings.TrimSpace(state))
}

func classifyEcommerceApplymentQueryState(state string) string {
	switch normalizeEcommerceApplymentQueryState(state) {
	case "AUDITING":
		return "AUDITING"
	case "REJECTED":
		return "REJECTED"
	case "NEED_SIGN":
		return "NEED_SIGN"
	case "FINISH":
		return "FINISH"
	case "FROZEN":
		return "FROZEN"
	case "CANCELED":
		return "CANCELED"
	case "ACCOUNT_NEED_VERIFY":
		return "ACCOUNT_NEED_VERIFY"
	case "CHECKING":
		return "CHECKING"
	default:
		return normalizeEcommerceApplymentQueryState(state)
	}
}

func ecommerceApplymentStateAllowsAccountValidation(stateClass string) bool {
	return stateClass == "ACCOUNT_NEED_VERIFY"
}

func ecommerceApplymentStateAllowsLegalValidationURL(stateClass string) bool {
	return stateClass == "ACCOUNT_NEED_VERIFY"
}

func ecommerceApplymentStateAllowsAuditDetail(stateClass string) bool {
	return stateClass == "REJECTED" || stateClass == "FROZEN"
}

func ecommerceApplymentStateAllowsSubMchID(stateClass string) bool {
	return stateClass == "NEED_SIGN" || stateClass == "FINISH"
}

func ecommerceApplymentStateAllowsSignURL(stateClass, signState string) bool {
	if stateClass == "NEED_SIGN" {
		return true
	}
	return signState == "UNSIGNED"
}

func normalizeEcommerceApplymentQuerySignState(signState string) string {
	return strings.ToUpper(strings.TrimSpace(signState))
}

func validateEcommerceApplymentID(applymentID int64) error {
	if applymentID <= 0 {
		return newEcommerceApplymentQueryValidationError("applyment_id must be a positive integer")
	}
	return nil
}

func validateEcommerceApplymentCreateResponse(resp *EcommerceApplymentResponse) error {
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
		return "", newEcommerceApplymentQueryValidationError("out_request_no is required")
	}
	if len(normalized) > ecommerceApplymentOutRequestNoMaxLength {
		return "", newEcommerceApplymentQueryValidationError("out_request_no must not exceed %d characters", ecommerceApplymentOutRequestNoMaxLength)
	}
	return normalized, nil
}

func validateSubMerchantSettlementSubMchID(subMchID string) (string, error) {
	normalized := strings.TrimSpace(subMchID)
	if normalized == "" {
		return "", newSubMerchantSettlementQueryValidationError("sub_mchid is required")
	}
	if len(normalized) != subMerchantSettlementMchIDLength {
		return "", newSubMerchantSettlementQueryValidationError("sub_mchid must be exactly %d digits", subMerchantSettlementMchIDLength)
	}
	for _, ch := range normalized {
		if ch < '0' || ch > '9' {
			return "", newSubMerchantSettlementQueryValidationError("sub_mchid must contain only digits")
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
		return "", newSubMerchantSettlementQueryValidationError("account_number_rule must be one of %s or %s", subMerchantSettlementAccountNumberRuleV1, subMerchantSettlementAccountNumberRuleV2)
	}
	return normalized, nil
}

func validateSubMerchantSettlementApplicationNo(applicationNo string) (string, error) {
	normalized := strings.TrimSpace(applicationNo)
	if normalized == "" {
		return "", newSubMerchantSettlementApplicationQueryValidationError("application_no is required")
	}
	if utf8.RuneCountInString(normalized) > subMerchantSettlementApplicationNoMaxLength {
		return "", newSubMerchantSettlementApplicationQueryValidationError("application_no must not exceed %d characters", subMerchantSettlementApplicationNoMaxLength)
	}
	return normalized, nil
}

func validateSubMerchantSettlementFieldLength(fieldName, value string, maxRunes int) error {
	if utf8.RuneCountInString(value) > maxRunes {
		return newSubMerchantSettlementContractError("wechat response %s exceeds %d characters", fieldName, maxRunes)
	}
	return nil
}

func validateSubMerchantSettlementResponse(resp *SubMerchantSettlementResponse) error {
	if resp == nil {
		return newSubMerchantSettlementContractError("empty wechat response")
	}

	resp.AccountType = strings.TrimSpace(resp.AccountType)
	resp.AccountBank = strings.TrimSpace(resp.AccountBank)
	resp.BankName = strings.TrimSpace(resp.BankName)
	resp.BankBranchID = strings.TrimSpace(resp.BankBranchID)
	resp.AccountNumber = strings.TrimSpace(resp.AccountNumber)
	resp.VerifyResult = strings.TrimSpace(resp.VerifyResult)
	resp.VerifyFailReason = strings.TrimSpace(resp.VerifyFailReason)

	if resp.AccountType == "" {
		return newSubMerchantSettlementContractError("wechat response missing account_type")
	}
	if _, ok := allowedSubMerchantSettlementAccountTypes[resp.AccountType]; !ok {
		return newSubMerchantSettlementContractError("unsupported account_type %q", resp.AccountType)
	}
	if resp.AccountBank == "" {
		return newSubMerchantSettlementContractError("wechat response missing account_bank")
	}
	if resp.AccountNumber == "" {
		return newSubMerchantSettlementContractError("wechat response missing account_number")
	}
	if resp.VerifyResult == "" {
		return newSubMerchantSettlementContractError("wechat response missing verify_result")
	}
	if _, ok := allowedSubMerchantSettlementVerifyResults[resp.VerifyResult]; !ok {
		return newSubMerchantSettlementContractError("unsupported verify_result %q", resp.VerifyResult)
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

	if resp.VerifyResult == subMerchantSettlementVerifyResultFail {
		if resp.VerifyFailReason == "" {
			return newSubMerchantSettlementContractError("verify_fail_reason is required when verify_result=%s", subMerchantSettlementVerifyResultFail)
		}
		return nil
	}
	if resp.VerifyFailReason != "" {
		return newSubMerchantSettlementContractError("verify_fail_reason is only allowed when verify_result=%s", subMerchantSettlementVerifyResultFail)
	}

	return nil
}

func validateSubMerchantSettlementApplicationResponse(resp *QuerySubMerchantSettlementApplicationResponse) error {
	if resp == nil {
		return newSubMerchantSettlementApplicationContractError("empty wechat response")
	}

	resp.AccountName = strings.TrimSpace(resp.AccountName)
	resp.AccountType = strings.TrimSpace(resp.AccountType)
	resp.AccountBank = strings.TrimSpace(resp.AccountBank)
	resp.BankName = strings.TrimSpace(resp.BankName)
	resp.BankBranchID = strings.TrimSpace(resp.BankBranchID)
	resp.AccountNumber = strings.TrimSpace(resp.AccountNumber)
	resp.VerifyResult = strings.TrimSpace(resp.VerifyResult)
	resp.VerifyFailReason = strings.TrimSpace(resp.VerifyFailReason)
	resp.VerifyFinishTime = strings.TrimSpace(resp.VerifyFinishTime)

	if resp.AccountName == "" {
		return newSubMerchantSettlementApplicationContractError("wechat response missing account_name")
	}
	if utf8.RuneCountInString(resp.AccountName) > subMerchantSettlementAccountNameMaxLength {
		return newSubMerchantSettlementApplicationContractError("wechat response account_name exceeds %d characters", subMerchantSettlementAccountNameMaxLength)
	}
	if resp.AccountType == "" {
		return newSubMerchantSettlementApplicationContractError("wechat response missing account_type")
	}
	if _, ok := allowedSubMerchantSettlementAccountTypes[resp.AccountType]; !ok {
		return newSubMerchantSettlementApplicationContractError("unsupported account_type %q", resp.AccountType)
	}
	if resp.AccountBank == "" {
		return newSubMerchantSettlementApplicationContractError("wechat response missing account_bank")
	}
	if err := validateSubMerchantSettlementFieldLength("account_bank", resp.AccountBank, subMerchantSettlementFieldMaxLength); err != nil {
		return newSubMerchantSettlementApplicationContractError("%s", strings.TrimPrefix(err.Error(), "query sub merchant settlement: "))
	}
	if err := validateSubMerchantSettlementFieldLength("bank_name", resp.BankName, subMerchantSettlementFieldMaxLength); err != nil {
		return newSubMerchantSettlementApplicationContractError("%s", strings.TrimPrefix(err.Error(), "query sub merchant settlement: "))
	}
	if err := validateSubMerchantSettlementFieldLength("bank_branch_id", resp.BankBranchID, subMerchantSettlementFieldMaxLength); err != nil {
		return newSubMerchantSettlementApplicationContractError("%s", strings.TrimPrefix(err.Error(), "query sub merchant settlement: "))
	}
	if resp.AccountNumber == "" {
		return newSubMerchantSettlementApplicationContractError("wechat response missing account_number")
	}
	if err := validateSubMerchantSettlementFieldLength("account_number", resp.AccountNumber, subMerchantSettlementFieldMaxLength); err != nil {
		return newSubMerchantSettlementApplicationContractError("%s", strings.TrimPrefix(err.Error(), "query sub merchant settlement: "))
	}
	if resp.VerifyResult == "" {
		return newSubMerchantSettlementApplicationContractError("wechat response missing verify_result")
	}
	if _, ok := allowedSubMerchantSettlementApplicationVerifyResults[resp.VerifyResult]; !ok {
		return newSubMerchantSettlementApplicationContractError("unsupported verify_result %q", resp.VerifyResult)
	}
	if err := validateSubMerchantSettlementFieldLength("verify_fail_reason", resp.VerifyFailReason, subMerchantSettlementFailReasonMaxLength); err != nil {
		return newSubMerchantSettlementApplicationContractError("%s", strings.TrimPrefix(err.Error(), "query sub merchant settlement: "))
	}
	if resp.VerifyResult == subMerchantSettlementApplicationAuditFail {
		if resp.VerifyFailReason == "" {
			return newSubMerchantSettlementApplicationContractError("verify_fail_reason is required when verify_result=%s", subMerchantSettlementApplicationAuditFail)
		}
	} else if resp.VerifyFailReason != "" {
		return newSubMerchantSettlementApplicationContractError("verify_fail_reason is only allowed when verify_result=%s", subMerchantSettlementApplicationAuditFail)
	}
	if resp.VerifyFinishTime != "" {
		if _, err := time.Parse(time.RFC3339, resp.VerifyFinishTime); err != nil {
			return newSubMerchantSettlementApplicationContractError("verify_finish_time must be RFC3339: %w", err)
		}
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

func validateEcommerceApplymentAccountValidation(validation *EcommerceApplymentAccountValidation) error {
	if validation == nil {
		return errors.New("wechat response missing account_validation")
	}
	if strings.TrimSpace(validation.AccountName) == "" {
		return errors.New("wechat response missing account_validation.account_name")
	}
	if validation.PayAmount == 0 {
		return errors.New("wechat response missing account_validation.pay_amount")
	}
	if strings.TrimSpace(validation.DestinationAccountNumber) == "" {
		return errors.New("wechat response missing account_validation.destination_account_number")
	}
	if strings.TrimSpace(validation.DestinationAccountName) == "" {
		return errors.New("wechat response missing account_validation.destination_account_name")
	}
	if strings.TrimSpace(validation.DestinationAccountBank) == "" {
		return errors.New("wechat response missing account_validation.destination_account_bank")
	}
	if strings.TrimSpace(validation.City) == "" {
		return errors.New("wechat response missing account_validation.city")
	}
	if strings.TrimSpace(validation.Remark) == "" {
		return errors.New("wechat response missing account_validation.remark")
	}
	if strings.TrimSpace(validation.Deadline) == "" {
		return errors.New("wechat response missing account_validation.deadline")
	}
	return nil
}

func validateEcommerceApplymentAuditDetail(auditDetail []ApplymentAuditDetail) error {
	if len(auditDetail) == 0 {
		return errors.New("wechat response missing audit_detail")
	}
	for idx, detail := range auditDetail {
		if strings.TrimSpace(detail.ParamName) == "" {
			return fmt.Errorf("wechat response missing audit_detail[%d].param_name", idx)
		}
		if strings.TrimSpace(detail.RejectReason) == "" {
			return fmt.Errorf("wechat response missing audit_detail[%d].reject_reason", idx)
		}
	}
	return nil
}

func validateEcommerceApplymentQueryResponse(resp *EcommerceApplymentQueryResponse, kind ecommerceApplymentQueryKind) error {
	if resp == nil {
		return errors.New("query ecommerce applyment: empty wechat response")
	}
	outRequestNo := strings.TrimSpace(resp.OutRequestNo)
	applymentState := normalizeEcommerceApplymentQueryState(resp.ApplymentState)
	applymentStateClass := classifyEcommerceApplymentQueryState(resp.ApplymentState)
	applymentStateDesc := strings.TrimSpace(resp.ApplymentStateDesc)
	signURL := strings.TrimSpace(resp.SignURL)
	signState := normalizeEcommerceApplymentQuerySignState(resp.SignState)
	subMchID := strings.TrimSpace(resp.SubMchID)
	legalValidationURL := strings.TrimSpace(resp.LegalValidationURL)

	if resp.ApplymentID <= 0 {
		return errors.New("query ecommerce applyment: wechat response missing applyment_id")
	}
	if outRequestNo == "" {
		return errors.New("query ecommerce applyment: wechat response missing out_request_no")
	}
	if applymentState == "" {
		return errors.New("query ecommerce applyment: wechat response missing applyment_state")
	}
	if applymentStateDesc == "" {
		return errors.New("query ecommerce applyment: wechat response missing applyment_state_desc")
	}
	if _, ok := allowedEcommerceApplymentStates[applymentState]; !ok {
		return fmt.Errorf("query ecommerce applyment: unsupported applyment_state %q", resp.ApplymentState)
	}
	if signState != "" {
		if _, ok := allowedEcommerceApplymentSignStates[signState]; !ok {
			return fmt.Errorf("query ecommerce applyment: unsupported sign_state %q", resp.SignState)
		}
	}
	if signURL != "" {
		if !ecommerceApplymentStateAllowsSignURL(applymentStateClass, signState) {
			if kind == ecommerceApplymentQueryByOutRequestNoKind {
				return fmt.Errorf("query ecommerce applyment: sign_url is only allowed when applyment_state=NEED_SIGN for out_request_no query, got applyment_state=%s", resp.ApplymentState)
			}
			return fmt.Errorf("query ecommerce applyment: sign_url is only allowed when applyment_state=NEED_SIGN or sign_state=UNSIGNED for applyment_id query, got applyment_state=%s sign_state=%s", resp.ApplymentState, resp.SignState)
		}
	}
	if resp.AccountValidation != nil && !ecommerceApplymentStateAllowsAccountValidation(applymentStateClass) {
		return fmt.Errorf("query ecommerce applyment: account_validation is only allowed when applyment_state=ACCOUNT_NEED_VERIFY, got %s", resp.ApplymentState)
	}
	if resp.AccountValidation != nil {
		if err := validateEcommerceApplymentAccountValidation(resp.AccountValidation); err != nil {
			return fmt.Errorf("query ecommerce applyment: %w", err)
		}
	}
	if len(resp.AuditDetail) > 0 && !ecommerceApplymentStateAllowsAuditDetail(applymentStateClass) {
		return fmt.Errorf("query ecommerce applyment: audit_detail is only allowed when applyment_state is REJECTED or FROZEN, got %s", resp.ApplymentState)
	}
	if len(resp.AuditDetail) > 0 {
		if err := validateEcommerceApplymentAuditDetail(resp.AuditDetail); err != nil {
			return fmt.Errorf("query ecommerce applyment: %w", err)
		}
	}
	if legalValidationURL != "" && !ecommerceApplymentStateAllowsLegalValidationURL(applymentStateClass) {
		return fmt.Errorf("query ecommerce applyment: legal_validation_url is only allowed when applyment_state=ACCOUNT_NEED_VERIFY, got %s", resp.ApplymentState)
	}
	if subMchID != "" && !ecommerceApplymentStateAllowsSubMchID(applymentStateClass) {
		return fmt.Errorf("query ecommerce applyment: sub_mchid is only allowed when applyment_state is NEED_SIGN or FINISH, got %s", resp.ApplymentState)
	}

	if applymentStateClass == "ACCOUNT_NEED_VERIFY" {
		if err := validateEcommerceApplymentAccountValidation(resp.AccountValidation); err != nil {
			return fmt.Errorf("query ecommerce applyment: account_validation is required when applyment_state=ACCOUNT_NEED_VERIFY: %w", err)
		}
	}
	if applymentStateClass == "REJECTED" || applymentStateClass == "FROZEN" {
		if err := validateEcommerceApplymentAuditDetail(resp.AuditDetail); err != nil {
			return fmt.Errorf("query ecommerce applyment: audit_detail is required when applyment_state=%s: %w", applymentStateClass, err)
		}
	}
	if applymentStateClass == "NEED_SIGN" {
		if signURL == "" {
			return errors.New("query ecommerce applyment: sign_url is required when applyment_state=NEED_SIGN")
		}
		if subMchID == "" {
			return errors.New("query ecommerce applyment: sub_mchid is required when applyment_state=NEED_SIGN")
		}
	}
	if applymentStateClass == "FINISH" && subMchID == "" {
		return errors.New("query ecommerce applyment: sub_mchid is required when applyment_state=FINISH")
	}
	if signState == "UNSIGNED" && signURL == "" {
		return fmt.Errorf("query ecommerce applyment: sign_url is required when sign_state=UNSIGNED for %s query", kind)
	}

	return nil
}

func (c *EcommerceClient) decryptEcommerceApplymentAccountValidation(validation *EcommerceApplymentAccountValidation) error {
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

func (c *EcommerceClient) queryEcommerceApplyment(ctx context.Context, kind ecommerceApplymentQueryKind, requestURL string, applymentID int64, outRequestNo string) (*EcommerceApplymentQueryResponse, error) {
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

	var resp EcommerceApplymentQueryResponse
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
func (c *EcommerceClient) CreateEcommerceApplyment(ctx context.Context, req *EcommerceApplymentRequest) (*EcommerceApplymentResponse, error) {
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

	var resp EcommerceApplymentResponse
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
func (c *EcommerceClient) QueryEcommerceApplymentByID(ctx context.Context, applymentID int64) (*EcommerceApplymentQueryResponse, error) {
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
func (c *EcommerceClient) QueryEcommerceApplymentByOutRequestNo(ctx context.Context, outRequestNo string) (*EcommerceApplymentQueryResponse, error) {
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
func (c *EcommerceClient) ListPersonalBankingBanks(ctx context.Context, offset, limit int) (*CapitalBankListResponse, error) {
	requestURL := fmt.Sprintf("%s?offset=%d&limit=%d", capitalPersonalBanksURL, offset, limit)
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("list personal banking banks: %w", err)
	}

	var resp CapitalBankListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal personal banking banks: %w", err)
	}

	return &resp, nil
}

// ListCorporateBankingBanks 查询支持对公业务的银行列表
func (c *EcommerceClient) ListCorporateBankingBanks(ctx context.Context, offset, limit int) (*CapitalBankListResponse, error) {
	requestURL := fmt.Sprintf("%s?offset=%d&limit=%d", capitalCorporateBanksURL, offset, limit)
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("list corporate banking banks: %w", err)
	}

	var resp CapitalBankListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal corporate banking banks: %w", err)
	}

	return &resp, nil
}

// SearchBanksByBankAccount 根据个人银行卡号识别开户银行候选
func (c *EcommerceClient) SearchBanksByBankAccount(ctx context.Context, accountNumber string) (*CapitalBankAccountSearchResponse, error) {
	encryptedAccountNumber, err := c.EncryptSensitiveData(accountNumber)
	if err != nil {
		return nil, fmt.Errorf("encrypt account number: %w", err)
	}

	requestURL := fmt.Sprintf("%s?account_number=%s", capitalSearchBanksByAccountURL, url.QueryEscape(encryptedAccountNumber))
	respBody, err := c.doRequestWithWechatSerial(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("search banks by account number: %w", err)
	}

	var resp CapitalBankAccountSearchResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal banks by account number: %w", err)
	}

	return &resp, nil
}

// ListProvinceAreas 查询省份列表
func (c *EcommerceClient) ListProvinceAreas(ctx context.Context) (*CapitalProvinceListResponse, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, capitalProvincesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("list province areas: %w", err)
	}

	var resp CapitalProvinceListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal province areas: %w", err)
	}

	return &resp, nil
}

// ListCityAreas 查询省份下城市列表
func (c *EcommerceClient) ListCityAreas(ctx context.Context, provinceCode int) (*CapitalCityListResponse, error) {
	requestURL := fmt.Sprintf(capitalProvinceCitiesURL, provinceCode)
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("list city areas: %w", err)
	}

	var resp CapitalCityListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal city areas: %w", err)
	}

	return &resp, nil
}

// ListBankBranches 查询支行列表
func (c *EcommerceClient) ListBankBranches(ctx context.Context, bankAliasCode string, cityCode, offset, limit int) (*CapitalBranchListResponse, error) {
	requestURL := fmt.Sprintf("%s?city_code=%d&offset=%d&limit=%d", fmt.Sprintf(capitalBankBranchesURL, bankAliasCode), cityCode, offset, limit)
	respBody, err := c.doRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("list bank branches: %w", err)
	}

	var resp CapitalBranchListResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal bank branches: %w", err)
	}

	return &resp, nil
}

// ==================== 合单支付 ====================

// CombineOrderRequest 合单下单请求
type CombineOrderRequest struct {
	CombineOutTradeNo string            // 合单商户订单号
	Description       string            // 商品描述
	SubOrders         []SubOrder        // 子订单列表
	PayerOpenID       string            // 支付者 OpenID
	PayerSubOpenID    string            // 支付者子商户 OpenID（可选）
	ExpireTime        time.Time         // 交易结束时间
	StartTime         *time.Time        // 交易开始时间（可选）
	NotifyURL         string            // 支付通知地址（可选，默认使用客户端配置）
	SceneInfo         *CombineSceneInfo // 场景信息（可选）
}

// SubOrder 子订单
type SubOrder struct {
	MchID         string // 商品单商户号；为空时默认使用服务商商户号
	SubMchID      string // 特约商户号（二级商户号）
	SubAppID      string // 子商户绑定的 AppID（可选）
	OutTradeNo    string // 子单商户订单号
	Description   string // 商品描述
	Amount        int64  // 订单金额（分）
	ProfitSharing bool   // 是否分账，true 表示需要分账
	Attach        string // 附加数据
	GoodsTag      string // 订单优惠标记（可选）
}

// CombineSceneInfo 场景信息
type CombineSceneInfo struct {
	PayerClientIP string `json:"payer_client_ip,omitempty"` // 用户终端 IP
	DeviceID      string `json:"device_id,omitempty"`       // 商户端设备号
}

// CombineOrderResponse 合单下单响应
type CombineOrderResponse struct {
	PrepayID string `json:"prepay_id"`
}

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
	subOrders := make([]map[string]interface{}, len(req.SubOrders))
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
		subOrder := map[string]interface{}{
			"mchid":        mchID,
			"attach":       sub.Attach,
			"out_trade_no": sub.OutTradeNo,
			"description":  sub.Description,
			"amount": map[string]interface{}{
				"total_amount": sub.Amount,
				"currency":     "CNY",
			},
			"settle_info": map[string]interface{}{
				"profit_sharing": sub.ProfitSharing,
			},
		}
		if sub.SubMchID != "" {
			subOrder["sub_mchid"] = sub.SubMchID
		}
		if sub.GoodsTag != "" {
			subOrder["goods_tag"] = sub.GoodsTag
		}
		subOrders[i] = subOrder
	}

	combinePayerInfo := map[string]interface{}{}
	if req.PayerOpenID != "" {
		combinePayerInfo["openid"] = req.PayerOpenID
	}

	notifyURL := c.combineNotifyURL
	if req.NotifyURL != "" {
		notifyURL = req.NotifyURL
	}

	body := map[string]interface{}{
		"combine_appid":        c.spAppID,
		"combine_mchid":        c.spMchID,
		"combine_out_trade_no": req.CombineOutTradeNo,
		"sub_orders":           subOrders,
		"combine_payer_info":   combinePayerInfo,
	}
	if notifyURL != "" {
		body["notify_url"] = notifyURL
	}
	if !req.ExpireTime.IsZero() {
		body["time_expire"] = req.ExpireTime.Format(time.RFC3339)
	}
	if req.StartTime != nil {
		body["time_start"] = req.StartTime.Format(time.RFC3339)
	}

	if req.SceneInfo != nil {
		body["scene_info"] = map[string]interface{}{
			"payer_client_ip": req.SceneInfo.PayerClientIP,
			"device_id":       req.SceneInfo.DeviceID,
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

	var resp CombineQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// CombineQueryResponse 合单查询响应
type CombineQueryResponse struct {
	CombineAppID      string                  `json:"combine_appid"`
	CombineMchID      string                  `json:"combine_mchid"`
	CombineOutTradeNo string                  `json:"combine_out_trade_no"`
	SubOrders         []CombineSubOrderResult `json:"sub_orders"`
	CombinePayerInfo  *CombinePayerInfo       `json:"combine_payer_info"`
	SceneInfo         *CombineSceneInfo       `json:"scene_info"`
}

// CombineSubOrderResult 合单子订单结果
type CombineSubOrderResult struct {
	MchID          string `json:"mchid"`
	SubMchID       string `json:"sub_mchid,omitempty"`
	SubAppID       string `json:"sub_appid,omitempty"`
	SubOpenID      string `json:"sub_openid,omitempty"`
	OutTradeNo     string `json:"out_trade_no"`
	TransactionID  string `json:"transaction_id"`
	TradeType      string `json:"trade_type,omitempty"`
	TradeState     string `json:"trade_state"` // SUCCESS/REFUND/NOTPAY/CLOSED/PAYERROR
	TradeStateDesc string `json:"trade_state_desc"`
	BankType       string `json:"bank_type,omitempty"`
	Attach         string `json:"attach,omitempty"`
	Amount         struct {
		TotalAmount    int64  `json:"total_amount"`
		PayerAmount    int64  `json:"payer_amount"`
		Currency       string `json:"currency"`
		PayerCurrency  string `json:"payer_currency"`
		SettlementRate int64  `json:"settlement_rate"`
	} `json:"amount"`
	SuccessTime string `json:"success_time"`
}

// CombinePayerInfo 合单支付者信息
type CombinePayerInfo struct {
	OpenID    string `json:"openid"`
	SubOpenID string `json:"sub_openid,omitempty"`
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

	subs := make([]map[string]string, len(subOrders))
	for i, sub := range subOrders {
		if strings.TrimSpace(sub.OutTradeNo) == "" {
			return fmt.Errorf("close combine order: sub_orders[%d].out_trade_no is required", i)
		}
		mchID := strings.TrimSpace(sub.MchID)
		if mchID == "" {
			mchID = c.spMchID
		}
		subOrder := map[string]string{
			"mchid":        mchID,
			"out_trade_no": sub.OutTradeNo,
		}
		if sub.SubMchID != "" {
			subOrder["sub_mchid"] = sub.SubMchID
		}
		if sub.SubAppID != "" {
			subOrder["sub_appid"] = sub.SubAppID
		}
		subs[i] = subOrder
	}

	body := map[string]interface{}{
		"combine_appid": c.spAppID,
		"sub_orders":    subs,
	}

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

func normalizePaymentOrderingWechatCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func partnerJSAPIOrderCreateWechatErrorGuide(wxErr *WechatPayError) string {
	if wxErr == nil {
		return "wechat partner jsapi order creation failed"
	}
	switch normalizePaymentOrderingWechatCode(wxErr.Code) {
	case "PARAM_ERROR", "INVALID_REQUEST":
		return "verify the partner jsapi request fields, especially payer, amount, sub_mchid, and scene_info.payer_client_ip, then retry"
	case "APPID_MCHID_NOT_MATCH", "OPENID_MISMATCH", "MCH_NOT_EXISTS":
		return "verify the configured appid, mchid, openid binding, and merchant configuration before retrying"
	case "NOAUTH", "NO_AUTH":
		return "verify the service provider merchant has permission to create partner jsapi orders before retrying"
	case "SIGN_ERROR":
		return "verify the merchant certificate, private key, and authorization signature before retrying"
	case "ORDER_CLOSED":
		return "wechat reported the payment order is already closed; recreate the payment with a new out_trade_no"
	case "OUT_TRADE_NO_USED":
		return "wechat reported the out_trade_no already exists; verify idempotency and reuse the existing pending payment when possible"
	case "ACCOUNTERROR", "ACCOUNT_ERROR":
		return "wechat rejected the payer account; ask the user to switch account or retry later"
	case "RULELIMIT", "RULE_LIMIT", "FREQUENCY_LIMITED", "RATELIMIT_EXCEEDED":
		return "wechat rate limited the payment request; retry later with backoff"
	case "TRADE_ERROR":
		return "wechat rejected the payment due to business rules; inspect the upstream detail and request_id before retrying"
	case "BANKERROR", "BANK_ERROR", "SYSTEMERROR", "SYSTEM_ERROR":
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
	switch normalizePaymentOrderingWechatCode(wxErr.Code) {
	case "ORDER_NOT_EXIST", "ORDERNOTEXIST":
		return "wechat could not find the partner payment order; verify out_trade_no, transaction_id, and sub_mchid before retrying"
	case "PARAM_ERROR", "INVALID_REQUEST":
		return "verify the partner order query parameter format and request path before retrying"
	case "NOAUTH", "NO_AUTH":
		return "verify the merchant has permission to query this partner payment order before retrying"
	case "SIGN_ERROR":
		return "verify the merchant certificate, private key, and authorization signature before retrying"
	case "RULELIMIT", "RULE_LIMIT", "FREQUENCY_LIMITED", "RATELIMIT_EXCEEDED":
		return "wechat rate limited the partner order query; retry later with backoff"
	case "SYSTEMERROR", "SYSTEM_ERROR", "BANKERROR", "BANK_ERROR":
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
	switch normalizePaymentOrderingWechatCode(wxErr.Code) {
	case "ORDER_CLOSED":
		return "wechat reported the partner payment order is already closed"
	case "ORDER_NOT_EXIST", "ORDERNOTEXIST":
		return "wechat could not find the partner payment order to close; verify out_trade_no and sub_mchid before retrying"
	case "INVALID_REQUEST":
		return "verify the partner order close payload and merchant identifiers before retrying"
	case "NOAUTH", "NO_AUTH":
		return "verify the merchant has permission to close this partner payment order before retrying"
	case "SIGN_ERROR":
		return "verify the merchant certificate, private key, and authorization signature before retrying"
	case "RULELIMIT", "RULE_LIMIT", "FREQUENCY_LIMITED", "RATELIMIT_EXCEEDED":
		return "wechat rate limited the partner order close request; retry later with backoff"
	case "SYSTEMERROR", "SYSTEM_ERROR", "BANKERROR", "BANK_ERROR":
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
	switch normalizePaymentOrderingWechatCode(wxErr.Code) {
	case "PARAM_ERROR", "INVALID_REQUEST":
		return "verify combine_out_trade_no, sub_orders, payer info, and scene_info.payer_client_ip before retrying"
	case "APPID_MCHID_NOT_MATCH", "OPENID_MISMATCH", "MCH_NOT_EXISTS":
		return "verify the configured combine_appid, combine_mchid, and payer openid binding before retrying"
	case "NOAUTH", "NO_AUTH":
		return "verify the service provider merchant has permission to create combine orders before retrying"
	case "SIGN_ERROR":
		return "verify the merchant certificate, private key, and authorization signature before retrying"
	case "ORDER_CLOSED":
		return "wechat reported the combine order is already closed; recreate the payment with a new combine_out_trade_no"
	case "OUT_TRADE_NO_USED":
		return "wechat reported one of the trade numbers already exists; verify idempotency and reuse the existing pending payment when possible"
	case "ACCOUNTERROR", "ACCOUNT_ERROR":
		return "wechat rejected the payer account; ask the user to switch account or retry later"
	case "RULELIMIT", "RULE_LIMIT", "FREQUENCY_LIMITED", "RATELIMIT_EXCEEDED":
		return "wechat rate limited the combine order request; retry later with backoff"
	case "TRADE_ERROR":
		return "wechat rejected the combine payment due to business rules; inspect the upstream detail and request_id before retrying"
	case "BANKERROR", "BANK_ERROR", "SYSTEMERROR", "SYSTEM_ERROR":
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
	switch normalizePaymentOrderingWechatCode(wxErr.Code) {
	case "ORDER_NOT_EXIST", "ORDERNOTEXIST":
		return "wechat could not find the combine payment order; verify combine_out_trade_no before retrying"
	case "PARAM_ERROR", "INVALID_REQUEST":
		return "verify the combine order query parameter format and request path before retrying"
	case "NOAUTH", "NO_AUTH":
		return "verify the merchant has permission to query this combine payment order before retrying"
	case "SIGN_ERROR":
		return "verify the merchant certificate, private key, and authorization signature before retrying"
	case "RULELIMIT", "RULE_LIMIT", "FREQUENCY_LIMITED", "RATELIMIT_EXCEEDED":
		return "wechat rate limited the combine order query; retry later with backoff"
	case "SYSTEMERROR", "SYSTEM_ERROR", "BANKERROR", "BANK_ERROR":
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
	switch normalizePaymentOrderingWechatCode(wxErr.Code) {
	case "ORDER_CLOSED":
		return "wechat reported the combine payment order is already closed"
	case "ORDER_NOT_EXIST", "ORDERNOTEXIST":
		return "wechat could not find the combine payment order to close; verify combine_out_trade_no before retrying"
	case "INVALID_REQUEST":
		return "verify the close payload matches the original combine order sub-orders exactly before retrying"
	case "NOAUTH", "NO_AUTH":
		return "verify the merchant has permission to close this combine payment order before retrying"
	case "SIGN_ERROR":
		return "verify the merchant certificate, private key, and authorization signature before retrying"
	case "USERPAYING":
		return "wechat reports the payer is still completing payment; query the order result before retrying close"
	case "RULELIMIT", "RULE_LIMIT", "FREQUENCY_LIMITED", "RATELIMIT_EXCEEDED":
		return "wechat rate limited the combine order close request; retry later with backoff"
	case "SYSTEMERROR", "SYSTEM_ERROR", "BANKERROR", "BANK_ERROR":
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

// SubOrderClose 关闭子订单参数
type SubOrderClose struct {
	MchID      string
	SubMchID   string
	SubAppID   string
	OutTradeNo string
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

// SubMerchantSettlementResponse 结算账户查询应答
type SubMerchantSettlementResponse struct {
	AccountType      string `json:"account_type"`                 // 账户类型：ACCOUNT_TYPE_BUSINESS / ACCOUNT_TYPE_PRIVATE
	AccountBank      string `json:"account_bank"`                 // 开户银行
	BankName         string `json:"bank_name,omitempty"`          // 开户银行全称（含支行）
	BankBranchID     string `json:"bank_branch_id,omitempty"`     // 开户银行联行号
	AccountNumber    string `json:"account_number"`               // 银行账号（掩码展示）
	VerifyResult     string `json:"verify_result"`                // 验证结果：VERIFY_SUCCESS / VERIFY_FAIL / VERIFYING
	VerifyFailReason string `json:"verify_fail_reason,omitempty"` // 验证失败原因
}

// QuerySubMerchantSettlement 查询特约商户/二级商户结算账户信息
//
// subMchID: 特约商户号；accountNumberRule: 账号展示规则（空字符串使用微信默认 ACCOUNT_NUMBER_RULE_MASK_V1）
func (c *EcommerceClient) QuerySubMerchantSettlement(ctx context.Context, subMchID string, accountNumberRule string) (*SubMerchantSettlementResponse, error) {
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

	var resp SubMerchantSettlementResponse
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

// ModifySubMerchantSettlementRequest 修改结算账户请求
type ModifySubMerchantSettlementRequest struct {
	AccountType   string `json:"account_type"`             // 账户类型：ACCOUNT_TYPE_BUSINESS / ACCOUNT_TYPE_PRIVATE
	AccountBank   string `json:"account_bank"`             // 开户银行
	BankName      string `json:"bank_name,omitempty"`      // 开户银行全称（含支行）
	BankBranchID  string `json:"bank_branch_id,omitempty"` // 开户银行联行号
	AccountNumber string `json:"account_number"`           // 银行账号（微信支付公钥加密）
	AccountName   string `json:"account_name,omitempty"`   // 开户名称（微信支付公钥加密，可选）
}

// ModifySubMerchantSettlementResponse 修改结算账户应答
type ModifySubMerchantSettlementResponse struct {
	ApplicationNo string `json:"application_no"` // 修改结算账户申请单号
}

// ModifySubMerchantSettlement 修改特约商户/二级商户结算账户
func (c *EcommerceClient) ModifySubMerchantSettlement(ctx context.Context, subMchID string, req *ModifySubMerchantSettlementRequest) (*ModifySubMerchantSettlementResponse, error) {
	requestURL := fmt.Sprintf(apply4subModifySettlementURL, subMchID)

	respBody, err := c.doRequestWithWechatSerial(ctx, http.MethodPost, requestURL, req)
	if err != nil {
		return nil, fmt.Errorf("modify sub merchant settlement: %w", err)
	}

	var resp ModifySubMerchantSettlementResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal modify sub merchant settlement: %w", err)
	}

	return &resp, nil
}

// ==================== 结算账户修改申请查询 ====================

// QuerySubMerchantSettlementApplicationResponse 查询结算账户修改申请状态应答
type QuerySubMerchantSettlementApplicationResponse struct {
	AccountName      string `json:"account_name"`        // 开户名称（掩码）
	AccountType      string `json:"account_type"`        // 账户类型
	AccountBank      string `json:"account_bank"`        // 开户银行
	BankName         string `json:"bank_name,omitempty"` // 开户银行全称（含支行）
	BankBranchID     string `json:"bank_branch_id,omitempty"`
	AccountNumber    string `json:"account_number"` // 银行账号（掩码）
	VerifyResult     string `json:"verify_result"`  // 审核状态：AUDIT_SUCCESS / AUDITING / AUDIT_FAIL
	VerifyFailReason string `json:"verify_fail_reason,omitempty"`
	VerifyFinishTime string `json:"verify_finish_time,omitempty"`
}

// QuerySubMerchantSettlementApplication 查询结算账户修改申请状态
//
// subMchID: 特约商户号；applicationNo: 申请单号；accountNumberRule: 账号展示规则（空字符串使用微信默认）
func (c *EcommerceClient) QuerySubMerchantSettlementApplication(ctx context.Context, subMchID, applicationNo, accountNumberRule string) (*QuerySubMerchantSettlementApplicationResponse, error) {
	normalizedSubMchID, err := validateSubMerchantSettlementSubMchID(subMchID)
	if err != nil {
		wrappedErr := newSubMerchantSettlementApplicationQueryValidationError("%s", strings.TrimPrefix(err.Error(), "query sub merchant settlement: "))
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
		wrappedErr := newSubMerchantSettlementApplicationQueryValidationError("%s", strings.TrimPrefix(err.Error(), "query sub merchant settlement: "))
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

	var resp QuerySubMerchantSettlementApplicationResponse
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

// CombinePaymentNotification 合单支付通知
type CombinePaymentNotification struct {
	CombineAppID      string                  `json:"combine_appid"`
	CombineMchID      string                  `json:"combine_mchid"`
	CombineOutTradeNo string                  `json:"combine_out_trade_no"`
	SubOrders         []CombineSubOrderResult `json:"sub_orders"`
	CombinePayerInfo  *CombinePayerInfo       `json:"combine_payer_info"`
	SceneInfo         *CombineSceneInfo       `json:"scene_info"`
}

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
