package wechat

import (
	"context"
	"mime/multipart"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

const (
	SecCheckMediaTypeVoice = 1
	SecCheckMediaTypeImage = 2
)

type MediaCheckAsyncRequest struct {
	MediaURL  string
	MediaType int
	Version   int
	OpenID    string
	Scene     int
}

type MediaCheckAsyncResponse struct {
	TraceID string `json:"trace_id"`
}

// WechatClient 微信客户端接口，便于测试mock
type WechatClient interface {
	// Code2Session 使用code换取openid和session_key
	Code2Session(ctx context.Context, code string) (*Code2SessionResponse, error)

	// ImgSecCheck 图片内容安全识别（用于头像/证照/证据等上传前审核）
	ImgSecCheck(ctx context.Context, imgFile multipart.File) error

	// MsgSecCheck 文本内容安全检测（msg_sec_check v2）
	// 返回 nil 表示通过；返回 ErrRiskyTextContent 表示不通过；其他 error 表示调用失败。
	MsgSecCheck(ctx context.Context, openid string, scene int, content string) error

	// MediaCheckAsync 异步检测图片/音频内容安全。
	MediaCheckAsync(ctx context.Context, req MediaCheckAsyncRequest) (*MediaCheckAsyncResponse, error)

	// OCRBusinessLicense 识别营业执照
	OCRBusinessLicense(ctx context.Context, imgFile multipart.File) (*BusinessLicenseOCRResponse, error)

	// OCRIDCard 识别身份证
	OCRIDCard(ctx context.Context, imgFile multipart.File, cardSide string) (*IDCardOCRResponse, error)

	// OCRPrintedText 通用印刷体识别（用于食品经营许可证等）
	OCRPrintedText(ctx context.Context, imgFile multipart.File) (*PrintedTextOCRResponse, error)

	// GetWXACodeUnlimited 获取小程序码（不限量版本）
	// scene: 场景参数，page: 跳转页面路径
	// 返回PNG图片数据
	GetWXACodeUnlimited(ctx context.Context, req *WXACodeRequest) ([]byte, error)

	// UploadShippingInfo 上传单商户支付发货信息（同城配送）
	// 在骑手取货后调用，logistics_type=2（同城配送）
	UploadShippingInfo(ctx context.Context, req *UploadShippingInfoRequest) error

	// UploadCombinedShippingInfo 上传合单支付发货信息（同城配送）
	UploadCombinedShippingInfo(ctx context.Context, req *UploadCombinedShippingInfoRequest) error
}

// DirectPaymentClientInterface 直连支付客户端接口
// 用于骑手押金、追偿付款及其配套退款/通知场景
type DirectPaymentClientInterface interface {
	// GetMchID 获取直连支付商户号
	GetMchID() string

	// GetAppID 获取直连支付 AppID
	GetAppID() string

	// CreateJSAPIOrder 创建 JSAPI 订单（小程序支付）
	CreateJSAPIOrder(ctx context.Context, req *wechatcontracts.DirectJSAPIOrderRequest) (*wechatcontracts.DirectJSAPIOrderResponse, *JSAPIPayParams, error)

	// QueryOrderByOutTradeNo 根据商户订单号查询订单
	QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*wechatcontracts.DirectOrderQueryResponse, error)

	// CloseOrder 关闭订单
	CloseOrder(ctx context.Context, outTradeNo string) error

	// CreateRefund 申请退款
	CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResponse, error)

	// QueryRefund 查询退款
	QueryRefund(ctx context.Context, outRefundNo string) (*RefundResponse, error)

	// DecryptPaymentNotification 解密支付通知
	DecryptPaymentNotification(notification *PaymentNotification) (*wechatcontracts.DirectPaymentNotificationResource, error)

	// DecryptRefundNotification 解密退款通知
	DecryptRefundNotification(notification *PaymentNotification) (*wechatcontracts.DirectRefundNotificationResource, error)

	// DecryptNotificationRaw 解密通知原始数据（返回 JSON 字节）
	DecryptNotificationRaw(notification *PaymentNotification) ([]byte, error)

	// VerifyNotificationSignature 验证微信支付回调签名
	VerifyNotificationSignature(signature, timestamp, nonce, serial, body string) error

	// GenerateJSAPIPayParams 根据 prepay_id 重新生成小程序调起支付所需参数（用于幂等返回旧 pending 记录时重新签名）
	GenerateJSAPIPayParams(prepayID string) (*JSAPIPayParams, error)
}

// TransferClientInterface 商家转账客户端接口。
// 用于索赔赔付等转账到微信零钱场景，不属于直连 JSAPI 支付主链路。
type TransferClientInterface interface {
	// GetMchID 获取商家转账商户号。
	GetMchID() string

	// GetAppID 获取商家转账 AppID。
	GetAppID() string

	// CreateTransfer 发起转账（商家转账到零钱）。
	CreateTransfer(ctx context.Context, req *wechatcontracts.DirectMerchantTransferCreateRequest) (*wechatcontracts.DirectMerchantTransferCreateResponse, error)

	// QueryTransferByOutBillNo 按商户单号查询转账单状态。
	QueryTransferByOutBillNo(ctx context.Context, outBillNo string) (*wechatcontracts.DirectMerchantTransferQueryResponse, error)

	// VerifyNotificationSignature 验证商家转账回调签名。
	VerifyNotificationSignature(signature, timestamp, nonce, serial, body string) error

	// DecryptMerchantTransferNotification 解密商家转账回调通知资源。
	DecryptMerchantTransferNotification(notification *PaymentNotification) (*wechatcontracts.DirectMerchantTransferNotificationResource, error)
}

// 确保 *Client 实现了 WechatClient 接口
var _ WechatClient = (*Client)(nil)

// 确保 *DirectPaymentClient 实现了 DirectPaymentClientInterface 接口
var _ DirectPaymentClientInterface = (*DirectPaymentClient)(nil)
