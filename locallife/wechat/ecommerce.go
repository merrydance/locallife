package wechat

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"
)

// 微信支付平台收付通 API 端点
const (
	// 图片上传
	merchantMediaUploadURL = "/v3/merchant/media/upload"

	// 二级商户进件
	ecommerceApplymentsURL      = "/v3/ecommerce/applyments/"
	ecommerceApplymentQueryURL  = "/v3/ecommerce/applyments/%d"
	ecommerceApplymentQueryByNo = "/v3/ecommerce/applyments/out-request-no/%s"

	// 合单支付（收付通）
	ecommerceCombineOrderURL = "/v3/combine-transactions/jsapi"
	ecommerceQueryCombineURL = "/v3/combine-transactions/out-trade-no/%s"
	ecommerceCloseCombineURL = "/v3/combine-transactions/out-trade-no/%s/close"

	// 分账
	profitSharingURL            = "/v3/ecommerce/profitsharing/orders"
	profitSharingFinishURL      = "/v3/ecommerce/profitsharing/finish-order"
	profitSharingReturnURL      = "/v3/ecommerce/profitsharing/returnorders"
	profitSharingReturnQueryURL = "/v3/ecommerce/profitsharing/returnorders/%s"

	// 分账接收方
	profitSharingReceiverAddURL    = "/v3/ecommerce/profitsharing/receivers/add"
	profitSharingReceiverDeleteURL = "/v3/ecommerce/profitsharing/receivers/delete"

	// 退款（平台收付通）
	ecommerceRefundURL      = "/v3/ecommerce/refunds/apply"
	ecommerceRefundQueryURL = "/v3/ecommerce/refunds/out-refund-no/%s"
)

// EcommerceClient 平台收付通客户端
// 用于多商户场景，支持分账功能
type EcommerceClient struct {
	*PaymentClient        // 复用基础支付客户端
	spMchID        string // 服务商商户号
	spAppID        string // 服务商 AppID
}

// EcommerceClientConfig 平台收付通客户端配置
type EcommerceClientConfig struct {
	PaymentClientConfig        // 嵌入基础配置
	SpMchID             string // 服务商商户号（如与 MchID 相同可不填）
	SpAppID             string // 服务商 AppID（如与 AppID 相同可不填）
}

// NewEcommerceClient 创建平台收付通客户端
func NewEcommerceClient(cfg EcommerceClientConfig) (*EcommerceClient, error) {
	baseClient, err := NewPaymentClient(cfg.PaymentClientConfig)
	if err != nil {
		return nil, fmt.Errorf("create base payment client: %w", err)
	}

	spMchID := cfg.SpMchID
	if spMchID == "" {
		spMchID = cfg.MchID
	}

	spAppID := cfg.SpAppID
	if spAppID == "" {
		spAppID = cfg.AppID
	}

	return &EcommerceClient{
		PaymentClient: baseClient,
		spMchID:       spMchID,
		spAppID:       spAppID,
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

// ==================== 二级商户进件 ====================

// EcommerceApplymentRequest 二级商户进件申请请求
type EcommerceApplymentRequest struct {
	OutRequestNo         string                    `json:"out_request_no"`                   // 业务申请编号
	OrganizationType     string                    `json:"organization_type"`                // 主体类型: 2401-小微, 2500-个体户, 2600-企业
	BusinessLicense      *BusinessLicenseInfo      `json:"business_license_info,omitempty"`  // 营业执照信息（个体户/企业必填）
	IDCardInfo           *ApplymentIDCardInfo      `json:"id_card_info"`                     // 法人身份证信息
	NeedAccountInfo      bool                      `json:"need_account_info"`                // 是否填写结算账户（true必填）
	AccountInfo          *ApplymentBankAccountInfo `json:"account_info,omitempty"`           // 结算银行账户
	ContactInfo          *ApplymentContactInfo     `json:"contact_info"`                     // 联系人信息
	SalesSceneInfo       *ApplymentSalesSceneInfo  `json:"sales_scene_info"`                 // 经营场景信息
	MerchantShortname    string                    `json:"merchant_shortname"`               // 商户简称
	Qualifications       []string                  `json:"qualifications,omitempty"`         // 特殊资质
	BusinessAdditionPics []string                  `json:"business_addition_pics,omitempty"` // 补充材料
	BusinessAdditionDesc string                    `json:"business_addition_desc,omitempty"` // 补充说明
}

// BusinessLicenseInfo 营业执照信息
type BusinessLicenseInfo struct {
	BusinessLicenseCopy   string `json:"business_license_copy"`   // 营业执照照片MediaID
	BusinessLicenseNumber string `json:"business_license_number"` // 营业执照注册号
	MerchantName          string `json:"merchant_name"`           // 商户名称
	LegalPerson           string `json:"legal_person"`            // 法人姓名
}

// ApplymentIDCardInfo 进件身份证信息
type ApplymentIDCardInfo struct {
	IDCardCopy      string `json:"id_card_copy"`       // 身份证正面照片MediaID
	IDCardNational  string `json:"id_card_national"`   // 身份证背面照片MediaID
	IDCardName      string `json:"id_card_name"`       // 身份证姓名（需加密）
	IDCardNumber    string `json:"id_card_number"`     // 身份证号码（需加密）
	IDCardValidTime string `json:"id_card_valid_time"` // 身份证有效期：YYYY-MM-DD 或 长期
}

// ApplymentBankAccountInfo 进件银行账户信息
type ApplymentBankAccountInfo struct {
	BankAccountType string `json:"bank_account_type"`   // ACCOUNT_TYPE_BUSINESS-对公, ACCOUNT_TYPE_PRIVATE-对私
	AccountBank     string `json:"account_bank"`        // 开户银行
	AccountName     string `json:"account_name"`        // 开户名称（需加密）
	BankAddressCode string `json:"bank_address_code"`   // 开户银行省市编码
	BankName        string `json:"bank_name,omitempty"` // 开户银行全称（支行）
	AccountNumber   string `json:"account_number"`      // 银行账号（需加密）
}

// ApplymentContactInfo 联系人信息
type ApplymentContactInfo struct {
	ContactType         string `json:"contact_type,omitempty"`           // 联系人类型: LEGAL-法人
	ContactName         string `json:"contact_name"`                     // 联系人姓名（需加密）
	ContactIDCardNumber string `json:"contact_id_card_number,omitempty"` // 联系人身份证号（需加密）
	MobilePhone         string `json:"mobile_phone"`                     // 联系手机号（需加密）
	ContactEmail        string `json:"contact_email,omitempty"`          // 联系邮箱（需加密）
}

// ApplymentSalesSceneInfo 经营场景信息
type ApplymentSalesSceneInfo struct {
	StoreName           string `json:"store_name"`                       // 店铺名称
	StoreURL            string `json:"store_url,omitempty"`              // 店铺链接
	StoreQRCode         string `json:"store_qr_code,omitempty"`          // 店铺二维码MediaID
	MiniProgramSubAppID string `json:"mini_program_sub_appid,omitempty"` // 小程序AppID
}

// EcommerceApplymentResponse 二级商户进件响应
type EcommerceApplymentResponse struct {
	ApplymentID  int64  `json:"applyment_id"`   // 微信支付申请单号
	OutRequestNo string `json:"out_request_no"` // 业务申请编号
}

// EcommerceApplymentQueryResponse 二级商户进件查询响应
type EcommerceApplymentQueryResponse struct {
	ApplymentID        int64                  `json:"applyment_id"`           // 微信支付申请单号
	OutRequestNo       string                 `json:"out_request_no"`         // 业务申请编号
	ApplymentState     string                 `json:"applyment_state"`        // 申请状态
	ApplymentStateDesc string                 `json:"applyment_state_desc"`   // 申请状态描述
	SignURL            string                 `json:"sign_url,omitempty"`     // 签约链接
	SignState          string                 `json:"sign_state,omitempty"`   // 签约状态
	SubMchID           string                 `json:"sub_mchid,omitempty"`    // 特约商户号
	AuditDetail        []ApplymentAuditDetail `json:"audit_detail,omitempty"` // 驳回详情
}

// ApplymentAuditDetail 进件审核详情
type ApplymentAuditDetail struct {
	ParamName    string `json:"param_name"`    // 参数名称
	RejectReason string `json:"reject_reason"` // 驳回原因
}

// CreateEcommerceApplyment 提交二级商户进件申请
// 注意：敏感信息需要使用微信支付平台公钥加密
func (c *EcommerceClient) CreateEcommerceApplyment(ctx context.Context, req *EcommerceApplymentRequest) (*EcommerceApplymentResponse, error) {
	body := map[string]interface{}{
		"out_request_no":     req.OutRequestNo,
		"organization_type":  req.OrganizationType,
		"merchant_shortname": req.MerchantShortname,
		"need_account_info":  req.NeedAccountInfo,
	}

	// 营业执照信息（个体户/企业必填）
	if req.BusinessLicense != nil {
		body["business_license_info"] = map[string]interface{}{
			"business_license_copy":   req.BusinessLicense.BusinessLicenseCopy,
			"business_license_number": req.BusinessLicense.BusinessLicenseNumber,
			"merchant_name":           req.BusinessLicense.MerchantName,
			"legal_person":            req.BusinessLicense.LegalPerson,
		}
	}

	// 身份证信息
	body["id_card_info"] = map[string]interface{}{
		"id_card_copy":       req.IDCardInfo.IDCardCopy,
		"id_card_national":   req.IDCardInfo.IDCardNational,
		"id_card_name":       req.IDCardInfo.IDCardName,   // 需加密
		"id_card_number":     req.IDCardInfo.IDCardNumber, // 需加密
		"id_card_valid_time": req.IDCardInfo.IDCardValidTime,
	}

	// 银行账户信息
	if req.AccountInfo != nil {
		accountInfo := map[string]interface{}{
			"bank_account_type": req.AccountInfo.BankAccountType,
			"account_bank":      req.AccountInfo.AccountBank,
			"account_name":      req.AccountInfo.AccountName, // 需加密
			"bank_address_code": req.AccountInfo.BankAddressCode,
			"account_number":    req.AccountInfo.AccountNumber, // 需加密
		}
		if req.AccountInfo.BankName != "" {
			accountInfo["bank_name"] = req.AccountInfo.BankName
		}
		body["account_info"] = accountInfo
	}

	// 联系人信息
	contactInfo := map[string]interface{}{
		"contact_name": req.ContactInfo.ContactName, // 需加密
		"mobile_phone": req.ContactInfo.MobilePhone, // 需加密
	}
	if req.ContactInfo.ContactType != "" {
		contactInfo["contact_type"] = req.ContactInfo.ContactType
	}
	if req.ContactInfo.ContactIDCardNumber != "" {
		contactInfo["contact_id_card_number"] = req.ContactInfo.ContactIDCardNumber // 需加密
	}
	if req.ContactInfo.ContactEmail != "" {
		contactInfo["contact_email"] = req.ContactInfo.ContactEmail // 需加密
	}
	body["contact_info"] = contactInfo

	// 经营场景信息
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

	// 特殊资质
	if len(req.Qualifications) > 0 {
		body["qualifications"] = req.Qualifications
	}

	// 补充材料
	if len(req.BusinessAdditionPics) > 0 {
		body["business_addition_pics"] = req.BusinessAdditionPics
	}
	if req.BusinessAdditionDesc != "" {
		body["business_addition_desc"] = req.BusinessAdditionDesc
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, ecommerceApplymentsURL, body)
	if err != nil {
		return nil, fmt.Errorf("create ecommerce applyment: %w", err)
	}

	var resp EcommerceApplymentResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// QueryEcommerceApplymentByID 通过申请单号查询进件状态
func (c *EcommerceClient) QueryEcommerceApplymentByID(ctx context.Context, applymentID int64) (*EcommerceApplymentQueryResponse, error) {
	url := fmt.Sprintf(ecommerceApplymentQueryURL, applymentID)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("query ecommerce applyment: %w", err)
	}

	var resp EcommerceApplymentQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// QueryEcommerceApplymentByOutRequestNo 通过业务申请编号查询进件状态
func (c *EcommerceClient) QueryEcommerceApplymentByOutRequestNo(ctx context.Context, outRequestNo string) (*EcommerceApplymentQueryResponse, error) {
	url := fmt.Sprintf(ecommerceApplymentQueryByNo, outRequestNo)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("query ecommerce applyment: %w", err)
	}

	var resp EcommerceApplymentQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
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
	ExpireTime        time.Time         // 交易结束时间
	SceneInfo         *CombineSceneInfo // 场景信息（可选）
}

// SubOrder 子订单
type SubOrder struct {
	MchID         string // 子单商户号（二级商户）
	OutTradeNo    string // 子单商户订单号
	Description   string // 商品描述
	Amount        int64  // 订单金额（分）
	ProfitSharing bool   // 是否分账，true 表示需要分账
	Attach        string // 附加数据（选填，用于子订单关联信息）
}

// CombineSceneInfo 场景信息
type CombineSceneInfo struct {
	PayerClientIP string // 用户终端 IP
	DeviceID      string // 商户端设备号
}

// CombineOrderResponse 合单下单响应
type CombineOrderResponse struct {
	PrepayID string `json:"prepay_id"`
}

// CreateCombineOrder 创建合单订单（平台收付通）
// 用于商户交易，资金进入二级商户账户
func (c *EcommerceClient) CreateCombineOrder(ctx context.Context, req *CombineOrderRequest) (*CombineOrderResponse, *JSAPIPayParams, error) {
	// 构建子订单列表
	subOrders := make([]map[string]interface{}, len(req.SubOrders))
	for i, sub := range req.SubOrders {
		subOrder := map[string]interface{}{
			"mchid":        sub.MchID,
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
		// 添加附加数据（如果提供）
		if sub.Attach != "" {
			subOrder["attach"] = sub.Attach
		}
		subOrders[i] = subOrder
	}

	body := map[string]interface{}{
		"combine_appid":        c.spAppID,
		"combine_mchid":        c.spMchID,
		"combine_out_trade_no": req.CombineOutTradeNo,
		"sub_orders":           subOrders,
		"combine_payer_info": map[string]interface{}{
			"openid": req.PayerOpenID,
		},
		"notify_url":  c.notifyURL,
		"time_expire": req.ExpireTime.Format(time.RFC3339),
	}

	if req.SceneInfo != nil {
		body["scene_info"] = map[string]interface{}{
			"payer_client_ip": req.SceneInfo.PayerClientIP,
			"device_id":       req.SceneInfo.DeviceID,
		}
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, ecommerceCombineOrderURL, body)
	if err != nil {
		return nil, nil, fmt.Errorf("create combine order: %w", err)
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

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("query combine order: %w", err)
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
}

// CombineSubOrderResult 合单子订单结果
type CombineSubOrderResult struct {
	MchID          string `json:"mchid"`
	OutTradeNo     string `json:"out_trade_no"`
	TransactionID  string `json:"transaction_id"`
	TradeState     string `json:"trade_state"` // SUCCESS/REFUND/NOTPAY/CLOSED/PAYERROR
	TradeStateDesc string `json:"trade_state_desc"`
	Amount         struct {
		TotalAmount int64  `json:"total_amount"`
		PayerAmount int64  `json:"payer_amount"`
		Currency    string `json:"currency"`
	} `json:"amount"`
	SuccessTime string `json:"success_time"`
}

// CombinePayerInfo 合单支付者信息
type CombinePayerInfo struct {
	OpenID string `json:"openid"`
}

// CloseCombineOrder 关闭合单订单
func (c *EcommerceClient) CloseCombineOrder(ctx context.Context, combineOutTradeNo string, subOrders []SubOrderClose) error {
	url := fmt.Sprintf(ecommerceCloseCombineURL, combineOutTradeNo)

	subs := make([]map[string]string, len(subOrders))
	for i, sub := range subOrders {
		subs[i] = map[string]string{
			"mchid":        sub.MchID,
			"out_trade_no": sub.OutTradeNo,
		}
	}

	body := map[string]interface{}{
		"combine_appid": c.spAppID,
		"sub_orders":    subs,
	}

	_, err := c.doRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("close combine order: %w", err)
	}

	return nil
}

// SubOrderClose 关闭子订单参数
type SubOrderClose struct {
	MchID      string
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
	Type            string // 分账接收方类型：MERCHANT_ID/PERSONAL_OPENID
	ReceiverAccount string // 分账接收方账号
	Amount          int64  // 分账金额（分）
	Description     string // 分账描述
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
	receivers := make([]map[string]interface{}, len(req.Receivers))
	for i, r := range req.Receivers {
		receivers[i] = map[string]interface{}{
			"type":             r.Type,
			"receiver_account": r.ReceiverAccount,
			"amount":           r.Amount,
			"description":      r.Description,
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

	respBody, err := c.doRequest(ctx, http.MethodPost, profitSharingURL, body)
	if err != nil {
		return nil, fmt.Errorf("create profit sharing: %w", err)
	}

	var resp ProfitSharingResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// QueryProfitSharing 查询分账结果
func (c *EcommerceClient) QueryProfitSharing(ctx context.Context, subMchID, transactionID, outOrderNo string) (*ProfitSharingQueryResponse, error) {
	url := fmt.Sprintf("%s?sub_mchid=%s&transaction_id=%s&out_order_no=%s",
		profitSharingURL, subMchID, transactionID, outOrderNo)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
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
	ReceiverTypeMerchant = "MERCHANT_ID"      // 商户号
	ReceiverTypePersonal = "PERSONAL_OPENID"  // 个人openid
)

// RelationType 分账关系类型
const (
	RelationServiceProvider = "SERVICE_PROVIDER" // 服务商
	RelationStore           = "STORE"            // 门店
	RelationStaff           = "STAFF"            // 员工
	RelationDistributor     = "DISTRIBUTOR"      // 分销商
	RelationSupplier        = "SUPPLIER"         // 供应商
	RelationPartner         = "PARTNER"          // 合作伙伴
)

// AddReceiverRequest 添加分账接收方请求
type AddReceiverRequest struct {
	AppID           string `json:"appid"`                      // 应用ID
	Type            string `json:"type"`                       // 接收方类型：MERCHANT_ID/PERSONAL_OPENID
	Account         string `json:"account"`                    // 接收方账号（商户号或openid）
	Name            string `json:"name,omitempty"`             // 接收方名称（需要加密）
	EncryptedName   string `json:"encrypted_name,omitempty"`   // 加密后的接收方名称
	RelationType    string `json:"relation_type"`              // 与分账方的关系类型
}

// AddReceiverResponse 添加分账接收方响应
type AddReceiverResponse struct {
	Type         string `json:"type"`
	Account      string `json:"account"`
	RelationType string `json:"relation_type"`
}

// AddProfitSharingReceiver 添加分账接收方
// 在二级商户进件成功后，需要将商户添加为分账接收方才能分账
func (c *EcommerceClient) AddProfitSharingReceiver(ctx context.Context, req *AddReceiverRequest) (*AddReceiverResponse, error) {
	body := map[string]interface{}{
		"appid":         req.AppID,
		"type":          req.Type,
		"account":       req.Account,
		"relation_type": req.RelationType,
	}

	// 如果有名称（个人类型需要）
	if req.EncryptedName != "" {
		body["encrypted_name"] = req.EncryptedName
	} else if req.Name != "" {
		// TODO: 需要使用微信平台证书加密
		body["name"] = req.Name
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, profitSharingReceiverAddURL, body)
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
	SubMchID    string // 二级商户号
	OrderID     string // 微信分账单号
	OutOrderNo  string // 商户分账单号
	OutReturnNo string // 商户回退单号
	ReturnMchID string // 回退商户号
	Amount      int64  // 回退金额（分）
	Description string // 回退描述
}

// ProfitSharingReturnResponse 分账回退响应
type ProfitSharingReturnResponse struct {
	SubMchID    string `json:"sub_mchid"`
	OrderID     string `json:"order_id"`
	OutOrderNo  string `json:"out_order_no"`
	OutReturnNo string `json:"out_return_no"`
	ReturnID    string `json:"return_id"`
	ReturnMchID string `json:"return_mchid"`
	Amount      int64  `json:"amount"`
	Result      string `json:"result"` // PROCESSING/SUCCESS/FAILED
	FinishTime  string `json:"finish_time"`
	FailReason  string `json:"fail_reason"`
}

// CreateProfitSharingReturn 请求分账回退
// 退款时需要先从各分账方回退资金
func (c *EcommerceClient) CreateProfitSharingReturn(ctx context.Context, req *ProfitSharingReturnRequest) (*ProfitSharingReturnResponse, error) {
	body := map[string]interface{}{
		"sub_mchid":     req.SubMchID,
		"order_id":      req.OrderID,
		"out_order_no":  req.OutOrderNo,
		"out_return_no": req.OutReturnNo,
		"return_mchid":  req.ReturnMchID,
		"amount":        req.Amount,
		"description":   req.Description,
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
	url := fmt.Sprintf("%s?sub_mchid=%s&out_return_no=%s&out_order_no=%s",
		profitSharingReturnQueryURL, subMchID, outReturnNo, outOrderNo)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("query profit sharing return: %w", err)
	}

	var resp ProfitSharingReturnResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// ==================== 电商退款 ====================

// EcommerceRefundRequest 电商退款请求
type EcommerceRefundRequest struct {
	SubMchID      string // 二级商户号
	TransactionID string // 微信支付订单号
	OutTradeNo    string // 商户订单号（二选一）
	OutRefundNo   string // 商户退款单号
	Reason        string // 退款原因
	RefundAmount  int64  // 退款金额（分）
	TotalAmount   int64  // 原订单金额（分）
	// 退款资金来源
	// AVAILABLE: 可用余额账户（默认）
	// UNSETTLED: 未结算资金
	FundsAccount string
}

// EcommerceRefundResponse 电商退款响应
type EcommerceRefundResponse struct {
	RefundID    string `json:"refund_id"`
	OutRefundNo string `json:"out_refund_no"`
	CreateTime  string `json:"create_time"`
	Amount      struct {
		Refund      int64  `json:"refund"`
		PayerRefund int64  `json:"payer_refund"`
		Total       int64  `json:"total"`
		PayerTotal  int64  `json:"payer_total"`
		Currency    string `json:"currency"`
	} `json:"amount"`
	Status      string `json:"status"` // PROCESSING/SUCCESS/CLOSED/ABNORMAL
	SuccessTime string `json:"success_time"`
}

// CreateEcommerceRefund 申请电商退款
// 退款前需要先调用分账回退
func (c *EcommerceClient) CreateEcommerceRefund(ctx context.Context, req *EcommerceRefundRequest) (*EcommerceRefundResponse, error) {
	body := map[string]interface{}{
		"sub_mchid":     req.SubMchID,
		"sp_appid":      c.spAppID,
		"out_refund_no": req.OutRefundNo,
		"reason":        req.Reason,
		"notify_url":    c.refundNotifyURL,
		"amount": map[string]interface{}{
			"refund":   req.RefundAmount,
			"total":    req.TotalAmount,
			"currency": "CNY",
		},
	}

	// 微信支付订单号或商户订单号二选一
	if req.TransactionID != "" {
		body["transaction_id"] = req.TransactionID
	} else if req.OutTradeNo != "" {
		body["out_trade_no"] = req.OutTradeNo
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

// QueryEcommerceRefund 查询电商退款
func (c *EcommerceClient) QueryEcommerceRefund(ctx context.Context, subMchID, outRefundNo string) (*EcommerceRefundResponse, error) {
	url := fmt.Sprintf(ecommerceRefundQueryURL+"?sub_mchid=%s", outRefundNo, subMchID)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("query ecommerce refund: %w", err)
	}

	var resp EcommerceRefundResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
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
	SpMchID       string `json:"sp_mchid"`
	SubMchID      string `json:"sub_mchid"`
	TransactionID string `json:"transaction_id"`
	OutTradeNo    string `json:"out_trade_no"`
	RefundID      string `json:"refund_id"`
	OutRefundNo   string `json:"out_refund_no"`
	RefundStatus  string `json:"refund_status"` // SUCCESS/CLOSED/ABNORMAL
	Amount        struct {
		Total       int64 `json:"total"`
		Refund      int64 `json:"refund"`
		PayerTotal  int64 `json:"payer_total"`
		PayerRefund int64 `json:"payer_refund"`
	} `json:"amount"`
	SuccessTime string `json:"success_time"`
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
	requestID := generateNonceStr()

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Request-ID", requestID)

	// 生成签名
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonceStr := generateNonceStr()
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

// UploadImage 上传图片到微信支付
// filename: 文件名（需要包含扩展名如 .jpg, .png）
// fileData: 文件二进制数据
// 返回 MediaID 用于进件申请
func (c *EcommerceClient) UploadImage(ctx context.Context, filename string, fileData []byte) (*ImageUploadResponse, error) {
	// 计算文件 SHA256 哈希
	fileHash := sha256.Sum256(fileData)
	sha256Hex := fmt.Sprintf("%x", fileHash)

	// 构造 meta 信息
	meta := map[string]string{
		"filename": filename,
		"sha256":   sha256Hex,
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
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
		return nil, fmt.Errorf("create meta part: %w", err)
	}
	metaPart.Write(metaBytes)

	// 添加 file 字段
	contentType := getImageContentType(filename)
	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	fileHeader.Set("Content-Type", contentType)
	filePart, err := writer.CreatePart(fileHeader)
	if err != nil {
		return nil, fmt.Errorf("create file part: %w", err)
	}
	filePart.Write(fileData)

	writer.Close()

	// 发送请求
	url := wxPayBaseURL + merchantMediaUploadURL
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	// 生成签名（对于文件上传，body 使用 meta JSON）
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonceStr := generateNonceStr()
	signature, err := c.generateSignature(http.MethodPost, merchantMediaUploadURL, timestamp, nonceStr, metaBytes)
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
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 检查响应状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var wxErr WechatPayError
		wxErr.StatusCode = resp.StatusCode
		if err := json.Unmarshal(respBody, &wxErr); err == nil && wxErr.Code != "" {
			return nil, &wxErr
		}
		return nil, fmt.Errorf("upload image failed: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	var result ImageUploadResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

// getImageContentType 根据文件名获取 Content-Type
func getImageContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".bmp":
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}
