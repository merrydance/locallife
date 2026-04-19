package wechat

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

// 错误定义
var (
	ErrInvalidSignature           = errors.New("invalid signature")
	nonceRandomReader   io.Reader = rand.Reader
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
	wxPayBaseURL                             = "https://api.mch.weixin.qq.com"
	jsapiOrderURL                            = "/v3/pay/transactions/jsapi"
	queryOrderByOutNoURL                     = "/v3/pay/transactions/out-trade-no"
	closeOrderURL                            = "/v3/pay/transactions/out-trade-no/%s/close"
	refundURL                                = "/v3/refund/domestic/refunds"
	queryRefundURL                           = "/v3/refund/domestic/refunds/%s"
	merchantTransferCreateURL                = "/v3/fund-app/mch-transfer/transfer-bills"
	merchantTransferQueryByOutBillNoURL      = "/v3/fund-app/mch-transfer/transfer-bills/out-bill-no/%s"
	merchantTransferQueryByTransferBillNoURL = "/v3/fund-app/mch-transfer/transfer-bills/transfer-bill-no/%s"
	merchantTransferCancelURL                = "/v3/fund-app/mch-transfer/transfer-bills/out-bill-no/%s/cancel"
)

// DirectPaymentClient 直连支付客户端
type DirectPaymentClient struct {
	mchID                     string          // 商户号
	appID                     string          // 小程序 AppID
	serialNo                  string          // 商户API证书序列号
	apiV3Key                  string          // APIv3 密钥
	privateKey                *rsa.PrivateKey // 商户私钥
	platformPublicKey         *rsa.PublicKey  // 微信支付平台公钥（推荐，用于验签和加密敏感信息）
	platformPublicKeyID       string          // 微信支付平台公钥ID
	notifyURL                 string          // 支付回调 URL
	refundNotifyURL           string          // 退款回调 URL
	merchantTransferNotifyURL string          // 商家转账回调 URL
	httpClient                *http.Client
	httpTimeout               time.Duration // HTTP请求超时时间
}

// DirectPaymentClientConfig 直连支付客户端配置
type DirectPaymentClientConfig struct {
	MchID                     string
	AppID                     string
	SerialNumber              string
	HTTPTimeout               time.Duration // HTTP请求超时时间（默认30秒）
	PrivateKeyPath            string
	APIV3Key                  string
	NotifyURL                 string
	RefundNotifyURL           string
	MerchantTransferNotifyURL string
	PlatformPublicKeyPath     string // 微信支付平台公钥路径（推荐）
	PlatformPublicKeyID       string // 微信支付平台公钥ID（从商户平台获取）
}

// NewDirectPaymentClient 创建直连支付客户端
func NewDirectPaymentClient(cfg DirectPaymentClientConfig) (*DirectPaymentClient, error) {
	// 读取商户私钥
	privateKey, err := loadPrivateKey(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load private key: %w", err)
	}

	if cfg.PlatformPublicKeyPath == "" || cfg.PlatformPublicKeyID == "" {
		return nil, fmt.Errorf("platform public key path and ID are required")
	}

	platformPublicKey, err := loadPublicKey(cfg.PlatformPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load platform public key: %w", err)
	}

	// 设置默认超时时间
	httpTimeout := cfg.HTTPTimeout
	if httpTimeout == 0 {
		httpTimeout = 30 * time.Second
	}

	return &DirectPaymentClient{
		mchID:                     cfg.MchID,
		appID:                     cfg.AppID,
		serialNo:                  cfg.SerialNumber,
		apiV3Key:                  cfg.APIV3Key,
		privateKey:                privateKey,
		platformPublicKey:         platformPublicKey,
		platformPublicKeyID:       cfg.PlatformPublicKeyID,
		notifyURL:                 cfg.NotifyURL,
		refundNotifyURL:           cfg.RefundNotifyURL,
		merchantTransferNotifyURL: cfg.MerchantTransferNotifyURL,
		httpClient: &http.Client{
			Timeout: httpTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		httpTimeout: httpTimeout,
	}, nil
}

// GetMchID 返回当前直连支付商户号，供回调归属校验使用。
func (c *DirectPaymentClient) GetMchID() string {
	return c.mchID
}

// GetAppID 返回当前直连支付 AppID，供回调归属校验使用。
func (c *DirectPaymentClient) GetAppID() string {
	return c.appID
}

func readBoundedConfigFile(path string) ([]byte, error) {
	cleanedPath := filepath.Clean(path)
	rootDir := filepath.Dir(cleanedPath)
	fileName := filepath.Base(cleanedPath)

	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, fmt.Errorf("open config root: %w", err)
	}
	defer root.Close()

	file, err := root.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	return data, nil
}

// loadPublicKey 从 PEM 文件加载 RSA 公钥
func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := readBoundedConfigFile(path)
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
	data, err := readBoundedConfigFile(path)
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

// CreateJSAPIOrder 创建 JSAPI 订单（小程序支付）
func (c *DirectPaymentClient) CreateJSAPIOrder(ctx context.Context, req *wechatcontracts.DirectJSAPIOrderRequest) (*wechatcontracts.DirectJSAPIOrderResponse, *JSAPIPayParams, error) {
	if req == nil {
		return nil, nil, wechatcontracts.ValidateDirectJSAPIOrderRequest(nil)
	}

	effectiveReq := *req
	if strings.TrimSpace(effectiveReq.NotifyURL) == "" {
		effectiveReq.NotifyURL = c.notifyURL
	}
	if strings.TrimSpace(effectiveReq.Currency) == "" {
		effectiveReq.Currency = wechatcontracts.DirectPaymentCurrencyCNY
	}
	if err := wechatcontracts.ValidateDirectJSAPIOrderRequest(&effectiveReq); err != nil {
		return nil, nil, err
	}

	body := wechatcontracts.DirectJSAPIOrderRequestBody{
		AppID:       c.appID,
		MchID:       c.mchID,
		Description: effectiveReq.Description,
		OutTradeNo:  effectiveReq.OutTradeNo,
		Attach:      effectiveReq.Attach,
		NotifyURL:   effectiveReq.NotifyURL,
		GoodsTag:    effectiveReq.GoodsTag,
		Amount: wechatcontracts.DirectOrderAmount{
			Total:    effectiveReq.TotalAmount,
			Currency: effectiveReq.Currency,
		},
		Payer: wechatcontracts.DirectOrderPayer{
			OpenID: effectiveReq.PayerOpenID,
		},
		Detail:        effectiveReq.Detail,
		SupportFapiao: effectiveReq.SupportFapiao,
	}
	if !effectiveReq.ExpireTime.IsZero() {
		body.TimeExpire = effectiveReq.ExpireTime.Format(time.RFC3339)
	}
	if effectiveReq.PayerClientIP != "" || effectiveReq.DeviceID != "" || effectiveReq.StoreInfo != nil {
		body.SceneInfo = &wechatcontracts.DirectOrderSceneInfo{
			PayerClientIP: effectiveReq.PayerClientIP,
			DeviceID:      effectiveReq.DeviceID,
			StoreInfo:     effectiveReq.StoreInfo,
		}
	}
	if effectiveReq.ProfitSharing {
		body.SettleInfo = &wechatcontracts.DirectOrderSettleInfo{ProfitSharing: true}
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, jsapiOrderURL, body)
	if err != nil {
		return nil, nil, fmt.Errorf("create jsapi order: %w", err)
	}

	var resp wechatcontracts.DirectJSAPIOrderResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if strings.TrimSpace(resp.PrepayID) == "" {
		return nil, nil, fmt.Errorf("create direct jsapi order: empty prepay id")
	}

	// 生成小程序调起支付参数
	payParams, err := c.generateJSAPIPayParams(resp.PrepayID)
	if err != nil {
		return nil, nil, fmt.Errorf("generate pay params: %w", err)
	}

	return &resp, payParams, nil
}

// GenerateJSAPIPayParams 生成小程序调起支付的参数（公开方法，实现 DirectPaymentClientInterface）
func (c *DirectPaymentClient) GenerateJSAPIPayParams(prepayID string) (*JSAPIPayParams, error) {
	return c.generateJSAPIPayParams(prepayID)
}

// generateJSAPIPayParams 生成小程序调起支付的参数
func (c *DirectPaymentClient) generateJSAPIPayParams(prepayID string) (*JSAPIPayParams, error) {
	nonceStr, err := generateNonceStr()
	if err != nil {
		return nil, err
	}
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
		SignType:  JSAPIPaySignTypeRSA,
		PaySign:   paySign,
	}, nil
}

// ==================== 查询订单 ====================

// QueryOrderByOutTradeNo 根据商户订单号查询订单
func (c *DirectPaymentClient) QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*wechatcontracts.DirectOrderQueryResponse, error) {
	trimmedOutTradeNo, err := wechatcontracts.ValidateDirectOrderQueryByOutTradeNoInput(outTradeNo)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/%s?mchid=%s", queryOrderByOutNoURL, url.PathEscape(trimmedOutTradeNo), url.QueryEscape(c.mchID))

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("query order: %w", err)
	}

	var resp wechatcontracts.DirectOrderQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if err := wechatcontracts.ValidateDirectOrderQueryResponse("query direct order by out_trade_no", &resp, false); err != nil {
		return nil, err
	}

	return &resp, nil
}

// ==================== 关闭订单 ====================

// CloseOrder 关闭订单
func (c *DirectPaymentClient) CloseOrder(ctx context.Context, outTradeNo string) error {
	trimmedOutTradeNo, err := wechatcontracts.ValidateDirectOrderQueryByOutTradeNoInput(outTradeNo)
	if err != nil {
		return err
	}
	url := fmt.Sprintf(closeOrderURL, trimmedOutTradeNo)
	body := wechatcontracts.DirectCloseOrderRequest{MchID: c.mchID}

	_, err = c.doRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("close order: %w", err)
	}

	return nil
}

// ==================== 申请退款 ====================

// RefundRequest 退款请求
type RefundRequest struct {
	TransactionID string // 微信支付订单号
	OutTradeNo    string // 原商户订单号
	OutRefundNo   string // 商户退款单号
	Reason        string // 退款原因
	NotifyURL     string // 退款回调地址
	FundsAccount  string // 退款资金来源
	RefundAmount  int64  // 退款金额（分）
	TotalAmount   int64  // 原订单金额（分）
	AmountFrom    []wechatcontracts.DirectRefundAmountFrom
	GoodsDetail   []wechatcontracts.DirectRefundGoodsDetail
}

// RefundResponse 退款响应
type RefundResponse struct {
	RefundID            string                                        `json:"refund_id"`
	OutRefundNo         string                                        `json:"out_refund_no"`
	TransactionID       string                                        `json:"transaction_id"`
	OutTradeNo          string                                        `json:"out_trade_no"`
	Channel             string                                        `json:"channel"`
	UserReceivedAccount string                                        `json:"user_received_account"`
	SuccessTime         string                                        `json:"success_time,omitempty"`
	CreateTime          string                                        `json:"create_time"`
	Status              string                                        `json:"status"`
	FundsAccount        string                                        `json:"funds_account"`
	Amount              wechatcontracts.DirectRefundAmount            `json:"amount"`
	PromotionDetail     []wechatcontracts.DirectRefundPromotionDetail `json:"promotion_detail,omitempty"`
}

// RefundStatus 退款状态常量
const (
	RefundStatusSuccess    = "SUCCESS"    // 退款成功
	RefundStatusClosed     = "CLOSED"     // 退款关闭
	RefundStatusProcessing = "PROCESSING" // 退款处理中
	RefundStatusAbnormal   = "ABNORMAL"   // 退款异常
)

// CreateRefund 申请退款
func (c *DirectPaymentClient) CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResponse, error) {
	contractReq := &wechatcontracts.DirectRefundRequest{
		TransactionID: req.TransactionID,
		OutTradeNo:    req.OutTradeNo,
		OutRefundNo:   req.OutRefundNo,
		Reason:        req.Reason,
		NotifyURL:     req.NotifyURL,
		FundsAccount:  req.FundsAccount,
		Amount: &wechatcontracts.DirectRefundRequestAmount{
			Refund:   req.RefundAmount,
			From:     req.AmountFrom,
			Total:    req.TotalAmount,
			Currency: wechatcontracts.DirectRefundCurrencyCNY,
		},
		GoodsDetail: req.GoodsDetail,
	}
	if err := wechatcontracts.ValidateDirectRefundRequest(contractReq); err != nil {
		return nil, err
	}

	notifyURL := strings.TrimSpace(req.NotifyURL)
	if notifyURL == "" {
		notifyURL = c.refundNotifyURL
	}
	body := map[string]interface{}{
		"out_refund_no": req.OutRefundNo,
		"reason":        req.Reason,
		"amount": map[string]interface{}{
			"refund":   req.RefundAmount,
			"total":    req.TotalAmount,
			"currency": wechatcontracts.DirectRefundCurrencyCNY,
		},
	}
	if strings.TrimSpace(req.TransactionID) != "" {
		body["transaction_id"] = req.TransactionID
	}
	if strings.TrimSpace(req.OutTradeNo) != "" {
		body["out_trade_no"] = req.OutTradeNo
	}
	if notifyURL != "" {
		body["notify_url"] = notifyURL
	}
	if strings.TrimSpace(req.FundsAccount) != "" {
		body["funds_account"] = req.FundsAccount
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
	if len(req.GoodsDetail) > 0 {
		goods := make([]map[string]interface{}, 0, len(req.GoodsDetail))
		for _, item := range req.GoodsDetail {
			goods = append(goods, map[string]interface{}{
				"merchant_goods_id":  item.MerchantGoodsID,
				"wechatpay_goods_id": item.WechatpayGoodsID,
				"goods_name":         item.GoodsName,
				"unit_price":         item.UnitPrice,
				"refund_amount":      item.RefundAmount,
				"refund_quantity":    item.RefundQuantity,
			})
		}
		body["goods_detail"] = goods
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, refundURL, body)
	if err != nil {
		return nil, fmt.Errorf("create refund: %w", err)
	}

	var resp RefundResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if err := wechatcontracts.ValidateDirectRefundResponse("create direct refund", toDirectRefundContractResponse(&resp)); err != nil {
		return nil, err
	}

	return &resp, nil
}

// QueryRefund 查询退款
func (c *DirectPaymentClient) QueryRefund(ctx context.Context, outRefundNo string) (*RefundResponse, error) {
	trimmedOutRefundNo, err := wechatcontracts.ValidateDirectQueryRefundByOutRefundNoInput(outRefundNo)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf(queryRefundURL, trimmedOutRefundNo)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("query refund: %w", err)
	}

	var resp RefundResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if err := wechatcontracts.ValidateDirectRefundResponse("query direct refund", toDirectRefundContractResponse(&resp)); err != nil {
		return nil, err
	}

	return &resp, nil
}

// ==================== 商家转账（普通商户单单据） ====================

// CreateTransfer 发起商家转账到零钱。
func (c *DirectPaymentClient) CreateTransfer(ctx context.Context, req *wechatcontracts.DirectMerchantTransferCreateRequest) (*wechatcontracts.DirectMerchantTransferCreateResponse, error) {
	if err := wechatcontracts.ValidateDirectMerchantTransferCreateRequest(req); err != nil {
		return nil, err
	}

	body := wechatcontracts.DirectMerchantTransferCreateRequestBody{
		AppID:                    req.AppID,
		OutBillNo:                req.OutBillNo,
		TransferSceneID:          req.TransferSceneID,
		OpenID:                   req.OpenID,
		TransferAmount:           req.TransferAmount,
		TransferRemark:           req.TransferRemark,
		NotifyURL:                strings.TrimSpace(req.NotifyURL),
		UserRecvPerception:       strings.TrimSpace(req.UserRecvPerception),
		TransferSceneReportInfos: req.TransferSceneReportInfos,
	}
	if body.NotifyURL == "" {
		body.NotifyURL = c.merchantTransferNotifyURL
	}
	if strings.TrimSpace(req.UserName) != "" {
		encryptedUserName, err := c.EncryptSensitiveData(req.UserName)
		if err != nil {
			return nil, fmt.Errorf("encrypt merchant transfer user name: %w", err)
		}
		body.UserName = encryptedUserName
	}

	respBody, err := c.doRequestWithWechatSerial(ctx, http.MethodPost, merchantTransferCreateURL, body)
	if err != nil {
		return nil, fmt.Errorf("create merchant transfer: %w", err)
	}

	var resp wechatcontracts.DirectMerchantTransferCreateResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal merchant transfer response: %w", err)
	}
	if err := wechatcontracts.ValidateDirectMerchantTransferCreateResponse("create direct merchant transfer", &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// QueryTransferByOutBillNo 按商户单号查询商家转账状态。
func (c *DirectPaymentClient) QueryTransferByOutBillNo(ctx context.Context, outBillNo string) (*wechatcontracts.DirectMerchantTransferQueryResponse, error) {
	trimmedOutBillNo, err := wechatcontracts.ValidateDirectMerchantTransferQueryByOutBillNoInput(outBillNo)
	if err != nil {
		return nil, err
	}
	respBody, err := c.doRequestWithWechatSerial(ctx, http.MethodGet, fmt.Sprintf(merchantTransferQueryByOutBillNoURL, url.PathEscape(trimmedOutBillNo)), nil)
	if err != nil {
		return nil, fmt.Errorf("query merchant transfer by out_bill_no: %w", err)
	}

	var resp wechatcontracts.DirectMerchantTransferQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal merchant transfer query response: %w", err)
	}
	if err := wechatcontracts.ValidateDirectMerchantTransferQueryResponse("query direct merchant transfer by out_bill_no", &resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.UserName) != "" {
		decryptedUserName, err := c.DecryptSensitiveResponseData(resp.UserName)
		if err != nil {
			return nil, fmt.Errorf("decrypt merchant transfer query user_name: %w", err)
		}
		resp.UserName = decryptedUserName
	}

	return &resp, nil
}

// QueryTransferByTransferBillNo 按微信转账单号查询商家转账状态。
func (c *DirectPaymentClient) QueryTransferByTransferBillNo(ctx context.Context, transferBillNo string) (*wechatcontracts.DirectMerchantTransferQueryResponse, error) {
	trimmedTransferBillNo, err := wechatcontracts.ValidateDirectMerchantTransferQueryByTransferBillNoInput(transferBillNo)
	if err != nil {
		return nil, err
	}
	respBody, err := c.doRequestWithWechatSerial(ctx, http.MethodGet, fmt.Sprintf(merchantTransferQueryByTransferBillNoURL, url.PathEscape(trimmedTransferBillNo)), nil)
	if err != nil {
		return nil, fmt.Errorf("query merchant transfer by transfer_bill_no: %w", err)
	}

	var resp wechatcontracts.DirectMerchantTransferQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal merchant transfer query response: %w", err)
	}
	if err := wechatcontracts.ValidateDirectMerchantTransferQueryResponse("query direct merchant transfer by transfer_bill_no", &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// CancelTransfer 按商户单号撤销商家转账。
func (c *DirectPaymentClient) CancelTransfer(ctx context.Context, outBillNo string) (*wechatcontracts.DirectMerchantTransferCancelResponse, error) {
	trimmedOutBillNo, err := wechatcontracts.ValidateDirectMerchantTransferQueryByOutBillNoInput(outBillNo)
	if err != nil {
		return nil, err
	}
	respBody, err := c.doRequestWithWechatSerial(ctx, http.MethodPost, fmt.Sprintf(merchantTransferCancelURL, url.PathEscape(trimmedOutBillNo)), nil)
	if err != nil {
		return nil, fmt.Errorf("cancel merchant transfer: %w", err)
	}

	var resp wechatcontracts.DirectMerchantTransferCancelResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal cancel merchant transfer response: %w", err)
	}
	if err := wechatcontracts.ValidateDirectMerchantTransferCancelResponse("cancel direct merchant transfer", &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// EncryptSensitiveData 使用微信支付平台公钥加密敏感数据
// 用于加密身份证号、银行卡号、手机号等敏感信息
// 返回 Base64 编码的加密后字符串
func (c *DirectPaymentClient) EncryptSensitiveData(plaintext string) (string, error) {
	if c.platformPublicKey == nil {
		return "", fmt.Errorf("platform public key not loaded")
	}

	// 微信支付敏感字段加密要求使用 RSAES-OAEP with SHA-1。
	ciphertext, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, c.platformPublicKey, []byte(plaintext), nil)
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSensitiveResponseData 解密微信返回的敏感字段。
// 微信对查询响应中的敏感字段使用商户 API 证书公钥加密，需由商户私钥解密。
func (c *DirectPaymentClient) DecryptSensitiveResponseData(ciphertext string) (string, error) {
	trimmedCiphertext := strings.TrimSpace(ciphertext)
	if trimmedCiphertext == "" {
		return "", nil
	}
	if c.privateKey == nil {
		return "", fmt.Errorf("merchant private key not loaded")
	}

	decodedCiphertext, err := base64.StdEncoding.DecodeString(trimmedCiphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	plaintext, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, c.privateKey, decodedCiphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext: %w", err)
	}

	return string(plaintext), nil
}

// GetPlatformPublicKeyID 返回请求头 Wechatpay-Serial 所需的平台公钥 ID。
func (c *DirectPaymentClient) GetPlatformPublicKeyID() string {
	return c.platformPublicKeyID
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

// DecryptPaymentNotification 解密支付通知
func (c *DirectPaymentClient) DecryptPaymentNotification(notification *PaymentNotification) (*wechatcontracts.DirectPaymentNotificationResource, error) {
	plaintext, err := c.decryptAESGCM(
		notification.Resource.Nonce,
		notification.Resource.Ciphertext,
		notification.Resource.AssociatedData,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt notification: %w", err)
	}

	var resource wechatcontracts.DirectPaymentNotificationResource
	if err := json.Unmarshal(plaintext, &resource); err != nil {
		return nil, fmt.Errorf("unmarshal resource: %w", err)
	}
	if err := wechatcontracts.ValidateDirectPaymentNotificationResource("decrypt direct payment notification", &resource); err != nil {
		return nil, err
	}

	return &resource, nil
}

// DecryptRefundNotification 解密退款通知
func (c *DirectPaymentClient) DecryptRefundNotification(notification *PaymentNotification) (*wechatcontracts.DirectRefundNotificationResource, error) {
	plaintext, err := c.decryptAESGCM(
		notification.Resource.Nonce,
		notification.Resource.Ciphertext,
		notification.Resource.AssociatedData,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt notification: %w", err)
	}

	var resource wechatcontracts.DirectRefundNotificationResource
	if err := json.Unmarshal(plaintext, &resource); err != nil {
		return nil, fmt.Errorf("unmarshal resource: %w", err)
	}
	if err := wechatcontracts.ValidateDirectRefundNotificationResource("decrypt direct refund notification", &resource); err != nil {
		return nil, err
	}

	return &resource, nil
}

// DecryptMerchantTransferNotification 解密商家转账通知。
func (c *DirectPaymentClient) DecryptMerchantTransferNotification(notification *PaymentNotification) (*wechatcontracts.DirectMerchantTransferNotificationResource, error) {
	plaintext, err := c.decryptAESGCM(
		notification.Resource.Nonce,
		notification.Resource.Ciphertext,
		notification.Resource.AssociatedData,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt merchant transfer notification: %w", err)
	}

	var resource wechatcontracts.DirectMerchantTransferNotificationResource
	if err := json.Unmarshal(plaintext, &resource); err != nil {
		return nil, fmt.Errorf("unmarshal merchant transfer resource: %w", err)
	}
	if err := wechatcontracts.ValidateDirectMerchantTransferNotificationResource("decrypt direct merchant transfer notification", &resource); err != nil {
		return nil, err
	}

	return &resource, nil
}

func toDirectRefundContractResponse(resp *RefundResponse) *wechatcontracts.DirectRefundResponse {
	if resp == nil {
		return nil
	}
	return &wechatcontracts.DirectRefundResponse{
		RefundID:            resp.RefundID,
		OutRefundNo:         resp.OutRefundNo,
		TransactionID:       resp.TransactionID,
		OutTradeNo:          resp.OutTradeNo,
		Channel:             resp.Channel,
		UserReceivedAccount: resp.UserReceivedAccount,
		SuccessTime:         resp.SuccessTime,
		CreateTime:          resp.CreateTime,
		Status:              resp.Status,
		FundsAccount:        resp.FundsAccount,
		Amount:              resp.Amount,
		PromotionDetail:     resp.PromotionDetail,
	}
}

// DecryptNotificationRaw 解密通知原始数据（返回 JSON 字节）
// 用于解密非标准的通知类型，如进件状态变更通知
func (c *DirectPaymentClient) DecryptNotificationRaw(notification *PaymentNotification) ([]byte, error) {
	return c.decryptAESGCM(
		notification.Resource.Nonce,
		notification.Resource.Ciphertext,
		notification.Resource.AssociatedData,
	)
}

// notifyTimestampWindow 微信回调时间戳允许偏差（±5 分钟），防重放攻击
const notifyTimestampWindow = 5 * 60 // seconds

// VerifyNotificationSignature 验证回调通知签名
// 参考: https://pay.weixin.qq.com/wiki/doc/apiv3/wechatpay/wechatpay4_1.shtml
func (c *DirectPaymentClient) VerifyNotificationSignature(signature, timestamp, nonce, serial, body string) error {
	expectedPublicKeyID := c.GetPlatformPublicKeyID()
	if expectedPublicKeyID == "" {
		return fmt.Errorf("platform public key ID not configured")
	}
	if serial == "" {
		return fmt.Errorf("missing Wechatpay-Serial header")
	}
	if !strings.EqualFold(serial, expectedPublicKeyID) {
		return fmt.Errorf("unexpected notification serial: got %q want %q", serial, expectedPublicKeyID)
	}

	// 0. 校验时间戳合法性，防止重放攻击（微信官方要求 ±5 分钟内）
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp format: %w", err)
	}
	diff := time.Now().Unix() - ts
	if diff > notifyTimestampWindow || diff < -notifyTimestampWindow {
		return fmt.Errorf("timestamp out of allowed window: diff=%ds", diff)
	}

	if c.platformPublicKey == nil {
		return fmt.Errorf("platform public key not configured")
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
	err = rsa.VerifyPKCS1v15(c.platformPublicKey, crypto.SHA256, hashed[:], signatureBytes)
	if err != nil {
		return ErrInvalidSignature
	}

	return nil
}

func (c *DirectPaymentClient) verifyResponseSignature(signature, timestamp, nonce, serial, body string) error {
	if signature == "" || timestamp == "" || nonce == "" || serial == "" {
		return fmt.Errorf("missing response signature headers")
	}

	expectedPublicKeyID := c.GetPlatformPublicKeyID()
	if expectedPublicKeyID == "" {
		return fmt.Errorf("platform public key ID not configured")
	}
	if !strings.EqualFold(serial, expectedPublicKeyID) {
		return fmt.Errorf("unexpected response serial: got %q want %q", serial, expectedPublicKeyID)
	}

	if c.platformPublicKey == nil {
		return fmt.Errorf("platform public key not configured")
	}

	message := fmt.Sprintf("%s\n%s\n%s\n", timestamp, nonce, body)
	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	hashed := sha256.Sum256([]byte(message))
	if err := rsa.VerifyPKCS1v15(c.platformPublicKey, crypto.SHA256, hashed[:], signatureBytes); err != nil {
		return ErrInvalidSignature
	}

	return nil
}

func (c *DirectPaymentClient) verifyHTTPResponseSignature(resp *http.Response, body []byte) error {
	return c.verifyResponseSignature(
		resp.Header.Get("Wechatpay-Signature"),
		resp.Header.Get("Wechatpay-Timestamp"),
		resp.Header.Get("Wechatpay-Nonce"),
		resp.Header.Get("Wechatpay-Serial"),
		string(body),
	)
}

// decryptAESGCM 使用 AES-GCM 解密（APIv3 回调通知解密）
func (c *DirectPaymentClient) decryptAESGCM(nonceStr, ciphertext, associatedData string) ([]byte, error) {
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
func (c *DirectPaymentClient) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	respBody, _, err := c.doRequestWithRequestID(ctx, method, path, body)
	return respBody, err
}

// doRequestWithWechatSerial 发送带微信支付平台公钥 ID 的请求（用于加密敏感信息）。
func (c *DirectPaymentClient) doRequestWithWechatSerial(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	wechatpaySerial := c.GetPlatformPublicKeyID()
	if wechatpaySerial == "" {
		return nil, fmt.Errorf("platform public key ID not loaded")
	}
	respBody, _, err := c.doRequestWithSerialAndRequestID(ctx, method, path, body, wechatpaySerial)
	return respBody, err
}

func (c *DirectPaymentClient) doRequestWithRequestID(ctx context.Context, method, path string, body interface{}) ([]byte, string, error) {
	return c.doRequestWithSerialAndRequestID(ctx, method, path, body, "")
}

func (c *DirectPaymentClient) doRequestWithSerialAndRequestID(ctx context.Context, method, path string, body interface{}, wechatSerial string) ([]byte, string, error) {
	return c.doRequestWithOptionsAndRequestID(ctx, method, path, body, wechatSerial, true)
}

func (c *DirectPaymentClient) doRequestWithOptionsAndRequestID(ctx context.Context, method, path string, body interface{}, wechatSerial string, verifyResponse bool) ([]byte, string, error) {
	var bodyBytes []byte
	var err error

	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, "", fmt.Errorf("marshal body: %w", err)
		}
	}

	requestURL := wxPayBaseURL + path
	signaturePath := path
	if strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "http://") {
		parsedURL, parseErr := url.Parse(path)
		if parseErr != nil {
			return nil, "", fmt.Errorf("parse request url: %w", parseErr)
		}
		if parsedURL.Scheme == "" || parsedURL.Host == "" {
			return nil, "", fmt.Errorf("invalid absolute request url: %s", path)
		}
		requestURL = parsedURL.String()
		signaturePath = parsedURL.RequestURI()
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}

	// 生成请求ID（用于追踪和问题排查）
	requestID, err := generateNonceStr()
	if err != nil {
		return nil, "", err
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Request-ID", requestID)
	if wechatSerial != "" {
		req.Header.Set("Wechatpay-Serial", wechatSerial)
	}

	// 生成签名
	timestamp := time.Now().Unix()
	nonceStr, err := generateNonceStr()
	if err != nil {
		return nil, requestID, err
	}
	signature, err := c.generateSignature(method, signaturePath, timestamp, nonceStr, bodyBytes)
	if err != nil {
		return nil, requestID, fmt.Errorf("generate signature: %w", err)
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
		return nil, requestID, fmt.Errorf("send request (request_id=%s): %w", requestID, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 4MB上限，防止异常大响应耗尽内存
	if err != nil {
		return nil, requestID, fmt.Errorf("read response: %w", err)
	}
	if verifyResponse {
		if err := c.verifyHTTPResponseSignature(resp, respBody); err != nil {
			return nil, requestID, fmt.Errorf("verify response signature: %w", err)
		}
	}

	// 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// 尝试解析微信支付错误响应
		var wxErr WechatPayError
		wxErr.StatusCode = resp.StatusCode

		if err := json.Unmarshal(respBody, &wxErr); err == nil && wxErr.Code != "" {
			log.Error().
				Str("request_id", requestID).
				Str("method", method).
				Str("path", signaturePath).
				Int("status", wxErr.StatusCode).
				Str("code", wxErr.Code).
				Str("message", wxErr.Message).
				Str("detail", wxErr.Detail).
				Msg("wechat pay api returned error")
			// 成功解析错误响应，添加request_id便于排查
			return nil, requestID, fmt.Errorf("request_id=%s: %w", requestID, &wxErr)
		}

		log.Error().
			Str("request_id", requestID).
			Str("method", method).
			Str("path", signaturePath).
			Int("status", resp.StatusCode).
			Str("body", string(respBody)).
			Msg("wechat pay api returned non-json error body")

		// 解析失败，返回原始错误
		return nil, requestID, fmt.Errorf("wechat pay api error: status=%d, body=%s, request_id=%s", resp.StatusCode, string(respBody), requestID)
	}

	return respBody, requestID, nil
}

// generateSignature 生成请求签名
func (c *DirectPaymentClient) generateSignature(method, path string, timestamp int64, nonceStr string, body []byte) (string, error) {
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
func (c *DirectPaymentClient) signWithRSA(message string) (string, error) {
	hash := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

// generateNonceStr 生成随机字符串。极少数情况下 crypto/rand 不可用时返回 error。
func generateNonceStr() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(nonceRandomReader, b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}
