package wechat

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

// 错误定义
var (
	ErrInvalidSignature = errors.New("invalid signature")
)

// WechatPayError 微信支付API错误响应
type WechatPayError struct {
	StatusCode int    `json:"-"`       // HTTP状态码
	Code       string `json:"code"`    // 错误码
	Message    string `json:"message"` // 错误描述
	Detail     string `json:"detail"`  // 详细信息（可选）
}

// Error 实现error接口
func (e *WechatPayError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("wechat pay error: code=%s, message=%s, detail=%s, status=%d",
			e.Code, e.Message, e.Detail, e.StatusCode)
	}
	return fmt.Sprintf("wechat pay error: code=%s, message=%s, status=%d",
		e.Code, e.Message, e.StatusCode)
}

const (
	// 微信支付 API 端点
	wxPayBaseURL         = "https://api.mch.weixin.qq.com"
	jsapiOrderURL        = "/v3/pay/transactions/jsapi"
	queryOrderByOutNoURL = "/v3/pay/transactions/out-trade-no"
	closeOrderURL        = "/v3/pay/transactions/out-trade-no/%s/close"
	refundURL            = "/v3/refund/domestic/refunds"
	queryRefundURL       = "/v3/refund/domestic/refunds/%s"
)

// PaymentClient 微信支付客户端
type PaymentClient struct {
	mchID               string            // 商户号
	appID               string            // 小程序 AppID
	serialNo            string            // 商户API证书序列号
	apiV3Key            string            // APIv3 密钥
	privateKey          *rsa.PrivateKey   // 商户私钥
	platformCertificate *x509.Certificate // 微信支付平台证书（用于验签和加密敏感信息）- 已弃用，建议使用公钥
	platformPublicKey   *rsa.PublicKey    // 微信支付平台公钥（推荐，用于验签和加密敏感信息）
	platformPublicKeyID string            // 微信支付平台公钥ID
	notifyURL           string            // 支付回调 URL
	refundNotifyURL     string            // 退款回调 URL
	httpClient          *http.Client
	httpTimeout         time.Duration // HTTP请求超时时间
}

// PaymentClientConfig 支付客户端配置
type PaymentClientConfig struct {
	MchID                   string
	AppID                   string
	SerialNumber            string
	HTTPTimeout             time.Duration // HTTP请求超时时间（默认30秒）
	PrivateKeyPath          string
	APIV3Key                string
	NotifyURL               string
	RefundNotifyURL         string
	PlatformCertificatePath string // 平台证书路径（已弃用，建议使用公钥）
	PlatformPublicKeyPath   string // 微信支付平台公钥路径（推荐）
	PlatformPublicKeyID     string // 微信支付平台公钥ID（从商户平台获取）
}

// NewPaymentClient 创建微信支付客户端
func NewPaymentClient(cfg PaymentClientConfig) (*PaymentClient, error) {
	// 读取商户私钥
	privateKey, err := loadPrivateKey(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load private key: %w", err)
	}

	// 优先使用平台公钥（推荐），其次使用平台证书（已弃用）
	var platformCert *x509.Certificate
	var platformPublicKey *rsa.PublicKey
	var platformPublicKeyID string

	if cfg.PlatformPublicKeyPath != "" {
		// 使用微信支付平台公钥（推荐方式）
		platformPublicKey, err = loadPublicKey(cfg.PlatformPublicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("load platform public key: %w", err)
		}
		platformPublicKeyID = cfg.PlatformPublicKeyID
		if platformPublicKeyID == "" {
			return nil, fmt.Errorf("platform public key ID is required when using platform public key")
		}
	} else if cfg.PlatformCertificatePath != "" {
		// 使用平台证书（已弃用，建议迁移到公钥模式）
		log.Warn().Msg("使用平台证书模式已弃用，建议迁移到微信支付公钥模式")
		platformCert, err = loadCertificate(cfg.PlatformCertificatePath)
		if err != nil {
			return nil, fmt.Errorf("load platform certificate: %w", err)
		}
	}

	// 设置默认超时时间
	httpTimeout := cfg.HTTPTimeout
	if httpTimeout == 0 {
		httpTimeout = 30 * time.Second
	}

	return &PaymentClient{
		mchID:               cfg.MchID,
		appID:               cfg.AppID,
		serialNo:            cfg.SerialNumber,
		apiV3Key:            cfg.APIV3Key,
		privateKey:          privateKey,
		platformCertificate: platformCert,
		platformPublicKey:   platformPublicKey,
		platformPublicKeyID: platformPublicKeyID,
		notifyURL:           cfg.NotifyURL,
		refundNotifyURL:     cfg.RefundNotifyURL,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
		httpTimeout: httpTimeout,
	}, nil
}

// loadPublicKey 从 PEM 文件加载 RSA 公钥
func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// 尝试解析为 PKIX 格式的公钥
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	rsaKey, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaKey, nil
}

// loadPrivateKey 从 PEM 文件加载 RSA 私钥
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA private key")
	}

	return rsaKey, nil
}

// loadCertificate 从 PEM 文件加载证书
func loadCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read certificate file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	return cert, nil
}

// ==================== JSAPI 下单 ====================

// JSAPIOrderRequest JSAPI 下单请求
type JSAPIOrderRequest struct {
	OutTradeNo    string    // 商户订单号
	Description   string    // 商品描述
	TotalAmount   int64     // 订单金额（分）
	OpenID        string    // 用户 OpenID
	ExpireTime    time.Time // 订单失效时间
	Attach        string    // 商户数据包（选填，建议传递 order_id，支付成功后会原样返回）
	PayerClientIP string    // 用户终端IP（选填但强烈建议，用于风控）
}

// JSAPIOrderResponse JSAPI 下单响应
type JSAPIOrderResponse struct {
	PrepayID string `json:"prepay_id"`
}

// JSAPIPayParams 小程序调起支付所需参数
type JSAPIPayParams struct {
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}

// CreateJSAPIOrder 创建 JSAPI 订单（小程序支付）
func (c *PaymentClient) CreateJSAPIOrder(ctx context.Context, req *JSAPIOrderRequest) (*JSAPIOrderResponse, *JSAPIPayParams, error) {
	body := map[string]interface{}{
		"appid":        c.appID,
		"mchid":        c.mchID,
		"description":  req.Description,
		"out_trade_no": req.OutTradeNo,
		"time_expire":  req.ExpireTime.Format(time.RFC3339),
		"notify_url":   c.notifyURL,
		"amount": map[string]interface{}{
			"total":    req.TotalAmount,
			"currency": "CNY",
		},
		"payer": map[string]interface{}{
			"openid": req.OpenID,
		},
	}

	// 添加商户数据包（用于回调时关联订单）
	if req.Attach != "" {
		body["attach"] = req.Attach
	}

	// 添加场景信息（用户终端IP，用于风控）
	if req.PayerClientIP != "" {
		body["scene_info"] = map[string]interface{}{
			"payer_client_ip": req.PayerClientIP,
		}
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, jsapiOrderURL, body)
	if err != nil {
		return nil, nil, fmt.Errorf("create jsapi order: %w", err)
	}

	var resp JSAPIOrderResponse
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

// generateJSAPIPayParams 生成小程序调起支付的参数
func (c *PaymentClient) generateJSAPIPayParams(prepayID string) (*JSAPIPayParams, error) {
	nonceStr := generateNonceStr()
	timeStamp := fmt.Sprintf("%d", time.Now().Unix())
	packageStr := "prepay_id=" + prepayID

	// 构造签名串
	signStr := fmt.Sprintf("%s\n%s\n%s\n%s\n", c.appID, timeStamp, nonceStr, packageStr)

	// 使用商户私钥签名
	paySign, err := c.signWithRSA(signStr)
	if err != nil {
		return nil, fmt.Errorf("sign pay params: %w", err)
	}

	return &JSAPIPayParams{
		TimeStamp: timeStamp,
		NonceStr:  nonceStr,
		Package:   packageStr,
		SignType:  "RSA",
		PaySign:   paySign,
	}, nil
}

// ==================== 查询订单 ====================

// OrderQueryResponse 订单查询响应
type OrderQueryResponse struct {
	AppID          string `json:"appid"`
	MchID          string `json:"mchid"`
	OutTradeNo     string `json:"out_trade_no"`
	TransactionID  string `json:"transaction_id"`
	TradeType      string `json:"trade_type"`
	TradeState     string `json:"trade_state"`
	TradeStateDesc string `json:"trade_state_desc"`
	BankType       string `json:"bank_type"`
	SuccessTime    string `json:"success_time"`
	Payer          struct {
		OpenID string `json:"openid"`
	} `json:"payer"`
	Amount struct {
		Total         int64  `json:"total"`
		PayerTotal    int64  `json:"payer_total"`
		Currency      string `json:"currency"`
		PayerCurrency string `json:"payer_currency"`
	} `json:"amount"`
}

// TradeState 交易状态常量
const (
	TradeStateSuccess    = "SUCCESS"    // 支付成功
	TradeStateRefund     = "REFUND"     // 转入退款
	TradeStateNotPay     = "NOTPAY"     // 未支付
	TradeStateClosed     = "CLOSED"     // 已关闭
	TradeStateRevoked    = "REVOKED"    // 已撤销（仅付款码支付）
	TradeStateUserPaying = "USERPAYING" // 用户支付中（仅付款码支付）
	TradeStatePayError   = "PAYERROR"   // 支付失败
)

// QueryOrderByOutTradeNo 根据商户订单号查询订单
func (c *PaymentClient) QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*OrderQueryResponse, error) {
	url := fmt.Sprintf("%s/%s?mchid=%s", queryOrderByOutNoURL, outTradeNo, c.mchID)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("query order: %w", err)
	}

	var resp OrderQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// ==================== 关闭订单 ====================

// CloseOrder 关闭订单
func (c *PaymentClient) CloseOrder(ctx context.Context, outTradeNo string) error {
	url := fmt.Sprintf(closeOrderURL, outTradeNo)
	body := map[string]interface{}{
		"mchid": c.mchID,
	}

	_, err := c.doRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("close order: %w", err)
	}

	return nil
}

// ==================== 申请退款 ====================

// RefundRequest 退款请求
type RefundRequest struct {
	OutTradeNo   string // 原商户订单号
	OutRefundNo  string // 商户退款单号
	Reason       string // 退款原因
	RefundAmount int64  // 退款金额（分）
	TotalAmount  int64  // 原订单金额（分）
}

// RefundResponse 退款响应
type RefundResponse struct {
	RefundID            string `json:"refund_id"`
	OutRefundNo         string `json:"out_refund_no"`
	TransactionID       string `json:"transaction_id"`
	OutTradeNo          string `json:"out_trade_no"`
	Channel             string `json:"channel"`
	UserReceivedAccount string `json:"user_received_account"`
	SuccessTime         string `json:"success_time,omitempty"`
	CreateTime          string `json:"create_time"`
	Status              string `json:"status"`
	Amount              struct {
		Total       int64 `json:"total"`
		Refund      int64 `json:"refund"`
		PayerTotal  int64 `json:"payer_total"`
		PayerRefund int64 `json:"payer_refund"`
	} `json:"amount"`
}

// RefundStatus 退款状态常量
const (
	RefundStatusSuccess    = "SUCCESS"    // 退款成功
	RefundStatusClosed     = "CLOSED"     // 退款关闭
	RefundStatusProcessing = "PROCESSING" // 退款处理中
	RefundStatusAbnormal   = "ABNORMAL"   // 退款异常
)

// CreateRefund 申请退款
func (c *PaymentClient) CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResponse, error) {
	body := map[string]interface{}{
		"out_trade_no":  req.OutTradeNo,
		"out_refund_no": req.OutRefundNo,
		"reason":        req.Reason,
		"notify_url":    c.refundNotifyURL,
		"amount": map[string]interface{}{
			"refund":   req.RefundAmount,
			"total":    req.TotalAmount,
			"currency": "CNY",
		},
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, refundURL, body)
	if err != nil {
		return nil, fmt.Errorf("create refund: %w", err)
	}

	var resp RefundResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// QueryRefund 查询退款
func (c *PaymentClient) QueryRefund(ctx context.Context, outRefundNo string) (*RefundResponse, error) {
	url := fmt.Sprintf(queryRefundURL, outRefundNo)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("query refund: %w", err)
	}

	var resp RefundResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// ==================== 提现转账（企业付款到零钱） ====================

const (
	transferURL = "/v3/transfer/batches"
)

// TransferRequest 转账请求
type TransferRequest struct {
	OutBatchNo     string // 商户批次单号
	BatchName      string // 批次名称
	BatchRemark    string // 批次备注
	TransferAmount int64  // 转账金额（分）
	OpenID         string // 收款用户 OpenID
	UserName       string // 收款用户真实姓名（需要加密）
	TransferRemark string // 转账备注
}

// TransferResponse 转账响应
type TransferResponse struct {
	OutBatchNo  string `json:"out_batch_no"`
	BatchID     string `json:"batch_id"`
	CreateTime  string `json:"create_time"`
	BatchStatus string `json:"batch_status"`
}

// CreateTransfer 发起转账（商家转账到零钱）
func (c *PaymentClient) CreateTransfer(ctx context.Context, req *TransferRequest) (*TransferResponse, error) {
	// 生成商户明细单号
	outDetailNo := generateNonceStr()

	// 加密敏感信息（用户真实姓名）
	encryptedUserName, err := c.EncryptSensitiveData(req.UserName)
	if err != nil {
		return nil, fmt.Errorf("encrypt user name: %w", err)
	}

	body := map[string]interface{}{
		"appid":        c.appID,
		"out_batch_no": req.OutBatchNo,
		"batch_name":   req.BatchName,
		"batch_remark": req.BatchRemark,
		"total_amount": req.TransferAmount,
		"total_num":    1,
		"transfer_detail_list": []map[string]interface{}{
			{
				"out_detail_no":   outDetailNo,
				"transfer_amount": req.TransferAmount,
				"transfer_remark": req.TransferRemark,
				"openid":          req.OpenID,
				"user_name":       encryptedUserName,
			},
		},
	}

	respBody, err := c.doRequestWithWechatSerial(ctx, http.MethodPost, transferURL, body)
	if err != nil {
		return nil, fmt.Errorf("create transfer: %w", err)
	}

	var resp TransferResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// EncryptSensitiveData 使用微信支付平台公钥或证书公钥加密敏感数据
// 优先使用平台公钥（推荐方式），其次使用平台证书公钥（已弃用）
// 用于加密身份证号、银行卡号、手机号等敏感信息
// 返回 Base64 编码的加密后字符串
func (c *PaymentClient) EncryptSensitiveData(plaintext string) (string, error) {
	var publicKey *rsa.PublicKey

	// 优先使用平台公钥
	if c.platformPublicKey != nil {
		publicKey = c.platformPublicKey
	} else if c.platformCertificate != nil {
		// 回退到平台证书
		var ok bool
		publicKey, ok = c.platformCertificate.PublicKey.(*rsa.PublicKey)
		if !ok {
			return "", fmt.Errorf("invalid platform certificate public key")
		}
	} else {
		return "", fmt.Errorf("neither platform public key nor platform certificate loaded")
	}

	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, []byte(plaintext), nil)
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// GetPlatformCertificateSerial 获取微信支付平台证书序列号或平台公钥ID
// 用于设置请求头中的 Wechatpay-Serial
// 如果使用平台公钥，返回公钥ID；如果使用平台证书，返回证书序列号
func (c *PaymentClient) GetPlatformCertificateSerial() string {
	// 优先返回平台公钥ID
	if c.platformPublicKeyID != "" {
		return c.platformPublicKeyID
	}
	// 回退到平台证书序列号
	if c.platformCertificate == nil {
		return ""
	}
	return fmt.Sprintf("%X", c.platformCertificate.SerialNumber)
}

// ==================== 回调通知处理 ====================

// PaymentNotification 支付通知结构
type PaymentNotification struct {
	ID           string    `json:"id"`
	CreateTime   time.Time `json:"create_time"`
	EventType    string    `json:"event_type"`
	ResourceType string    `json:"resource_type"`
	Resource     struct {
		Algorithm      string `json:"algorithm"`
		Ciphertext     string `json:"ciphertext"`
		Nonce          string `json:"nonce"`
		AssociatedData string `json:"associated_data"`
		OriginalType   string `json:"original_type"`
	} `json:"resource"`
	Summary string `json:"summary"`
}

// PaymentNotificationResource 支付通知解密后的资源
type PaymentNotificationResource struct {
	TransactionID  string `json:"transaction_id"`
	OutTradeNo     string `json:"out_trade_no"`
	TradeType      string `json:"trade_type"`
	TradeState     string `json:"trade_state"`
	TradeStateDesc string `json:"trade_state_desc"`
	BankType       string `json:"bank_type"`
	SuccessTime    string `json:"success_time"`
	Payer          struct {
		OpenID string `json:"openid"`
	} `json:"payer"`
	Amount struct {
		Total         int64  `json:"total"`
		PayerTotal    int64  `json:"payer_total"`
		Currency      string `json:"currency"`
		PayerCurrency string `json:"payer_currency"`
	} `json:"amount"`
}

// RefundNotificationResource 退款通知解密后的资源
type RefundNotificationResource struct {
	MchID               string `json:"mchid"`
	OutTradeNo          string `json:"out_trade_no"`
	TransactionID       string `json:"transaction_id"`
	OutRefundNo         string `json:"out_refund_no"`
	RefundID            string `json:"refund_id"`
	RefundStatus        string `json:"refund_status"`
	SuccessTime         string `json:"success_time,omitempty"`
	UserReceivedAccount string `json:"user_received_account"`
	Amount              struct {
		Total       int64 `json:"total"`
		Refund      int64 `json:"refund"`
		PayerTotal  int64 `json:"payer_total"`
		PayerRefund int64 `json:"payer_refund"`
	} `json:"amount"`
}

// DecryptPaymentNotification 解密支付通知
func (c *PaymentClient) DecryptPaymentNotification(notification *PaymentNotification) (*PaymentNotificationResource, error) {
	plaintext, err := c.decryptAESGCM(
		notification.Resource.Nonce,
		notification.Resource.Ciphertext,
		notification.Resource.AssociatedData,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt notification: %w", err)
	}

	var resource PaymentNotificationResource
	if err := json.Unmarshal(plaintext, &resource); err != nil {
		return nil, fmt.Errorf("unmarshal resource: %w", err)
	}

	return &resource, nil
}

// DecryptRefundNotification 解密退款通知
func (c *PaymentClient) DecryptRefundNotification(notification *PaymentNotification) (*RefundNotificationResource, error) {
	plaintext, err := c.decryptAESGCM(
		notification.Resource.Nonce,
		notification.Resource.Ciphertext,
		notification.Resource.AssociatedData,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt notification: %w", err)
	}

	var resource RefundNotificationResource
	if err := json.Unmarshal(plaintext, &resource); err != nil {
		return nil, fmt.Errorf("unmarshal resource: %w", err)
	}

	return &resource, nil
}

// DecryptNotificationRaw 解密通知原始数据（返回 JSON 字节）
// 用于解密非标准的通知类型，如进件状态变更通知
func (c *PaymentClient) DecryptNotificationRaw(notification *PaymentNotification) ([]byte, error) {
	return c.decryptAESGCM(
		notification.Resource.Nonce,
		notification.Resource.Ciphertext,
		notification.Resource.AssociatedData,
	)
}

// VerifyNotificationSignature 验证回调通知签名
// 参考: https://pay.weixin.qq.com/wiki/doc/apiv3/wechatpay/wechatpay4_1.shtml
// 优先使用平台公钥（推荐），其次使用平台证书公钥（已弃用）
func (c *PaymentClient) VerifyNotificationSignature(signature, timestamp, nonce, body string) error {
	// 获取用于验签的公钥
	var publicKey *rsa.PublicKey
	if c.platformPublicKey != nil {
		// 优先使用平台公钥（推荐方式）
		publicKey = c.platformPublicKey
	} else if c.platformCertificate != nil {
		// 回退到平台证书公钥（已弃用）
		var ok bool
		publicKey, ok = c.platformCertificate.PublicKey.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("invalid platform certificate public key type")
		}
	} else {
		return fmt.Errorf("neither platform public key nor platform certificate configured")
	}

	// 1. 构造验签名串
	// timestamp\n
	// nonce\n
	// body\n
	message := fmt.Sprintf("%s\n%s\n%s\n", timestamp, nonce, body)

	// 2. Base64解码签名
	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	// 3. 使用SHA256哈希
	hashed := sha256.Sum256([]byte(message))

	// 4. 使用平台公钥验证签名
	err = rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], signatureBytes)
	if err != nil {
		return ErrInvalidSignature
	}

	return nil
}

// decryptAESGCM 使用 AES-GCM 解密（APIv3 回调通知解密）
func (c *PaymentClient) decryptAESGCM(nonceStr, ciphertext, associatedData string) ([]byte, error) {
	nonce := []byte(nonceStr)
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	additionalData := []byte(associatedData)

	block, err := aes.NewCipher([]byte(c.apiV3Key))
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	plaintext, err := aesGCM.Open(nil, nonce, ciphertextBytes, additionalData)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// ==================== HTTP 请求签名 ====================

// doRequest 发送签名请求
func (c *PaymentClient) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	return c.doRequestWithSerial(ctx, method, path, body, "")
}

// doRequestWithWechatSerial 发送带微信支付平台公钥ID/证书序列号的请求（用于加密敏感信息）
// 优先使用平台公钥ID（推荐），其次使用平台证书序列号（已弃用）
func (c *PaymentClient) doRequestWithWechatSerial(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	wechatSerial := c.GetPlatformCertificateSerial()
	if wechatSerial == "" {
		return nil, fmt.Errorf("neither platform public key ID nor platform certificate loaded")
	}
	return c.doRequestWithSerial(ctx, method, path, body, wechatSerial)
}

// doRequestWithSerial 发送请求，支持指定微信支付平台证书序列号
func (c *PaymentClient) doRequestWithSerial(ctx context.Context, method, path string, body interface{}, wechatSerial string) ([]byte, error) {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
	}

	url := wxPayBaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 生成请求ID（用于追踪和问题排查）
	requestID := generateNonceStr()

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Request-ID", requestID)
	if wechatSerial != "" {
		req.Header.Set("Wechatpay-Serial", wechatSerial)
	}

	// 生成签名
	timestamp := time.Now().Unix()
	nonceStr := generateNonceStr()
	signature, err := c.generateSignature(method, path, timestamp, nonceStr, bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("generate signature: %w", err)
	}

	// 设置授权头
	authorization := fmt.Sprintf(
		`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%d",serial_no="%s"`,
		c.mchID, nonceStr, signature, timestamp, c.serialNo,
	)
	req.Header.Set("Authorization", authorization)

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

	// 检查状态码
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

// generateSignature 生成请求签名
func (c *PaymentClient) generateSignature(method, path string, timestamp int64, nonceStr string, body []byte) (string, error) {
	// 构造签名串
	var signStr string
	if len(body) > 0 {
		signStr = fmt.Sprintf("%s\n%s\n%d\n%s\n%s\n", method, path, timestamp, nonceStr, string(body))
	} else {
		signStr = fmt.Sprintf("%s\n%s\n%d\n%s\n\n", method, path, timestamp, nonceStr)
	}

	return c.signWithRSA(signStr)
}

// signWithRSA 使用 RSA-SHA256 签名
func (c *PaymentClient) signWithRSA(message string) (string, error) {
	hash := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

// generateNonceStr 生成随机字符串
func generateNonceStr() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
