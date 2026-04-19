package contracts

import "time"

// 官方文档：直连支付组
// JSAPI/小程序下单：https://pay.weixin.qq.com/doc/v3/merchant/4012791897.md
// 微信支付订单号查询订单：https://pay.weixin.qq.com/doc/v3/merchant/4012791899.md
// 商户订单号查询订单：https://pay.weixin.qq.com/doc/v3/merchant/4012791900.md
// 关闭订单：https://pay.weixin.qq.com/doc/v3/merchant/4012791901.md
// 支付结果通知：https://pay.weixin.qq.com/doc/v3/merchant/4012791902.md
// 小程序调起支付参数真值见 mini_program_request_payment.go 中的 JSAPIPayParams。

const (
	DirectPaymentCurrencyCNY = "CNY"
)

const (
	DirectTradeTypeJSAPI    = "JSAPI"
	DirectTradeTypeNative   = "NATIVE"
	DirectTradeTypeApp      = "APP"
	DirectTradeTypeMicropay = "MICROPAY"
	DirectTradeTypeMWEB     = "MWEB"
	DirectTradeTypeFacePay  = "FACEPAY"
)

const (
	DirectTradeStateSuccess    = "SUCCESS"
	DirectTradeStateRefund     = "REFUND"
	DirectTradeStateNotPay     = "NOTPAY"
	DirectTradeStateClosed     = "CLOSED"
	DirectTradeStateRevoked    = "REVOKED"
	DirectTradeStateUserPaying = "USERPAYING"
	DirectTradeStatePayError   = "PAYERROR"
)

const (
	DirectPromotionScopeGlobal = "GLOBAL"
	DirectPromotionScopeSingle = "SINGLE"
)

const (
	DirectPromotionTypeCash   = "CASH"
	DirectPromotionTypeNoCash = "NOCASH"
)

const (
	DirectPaymentNotifyEventTypeTransactionSuccess = "TRANSACTION.SUCCESS"
	DirectPaymentNotifyResourceTypeEncryptResource = "encrypt-resource"
	DirectPaymentNotifyOriginalTypeTransaction     = "transaction"
)

type DirectOrderAmount struct {
	Total    int64  `json:"total"`
	Currency string `json:"currency,omitempty"`
}

type DirectOrderPayer struct {
	OpenID string `json:"openid"`
}

type DirectOrderGoodsDetail struct {
	MerchantGoodsID  string `json:"merchant_goods_id"`
	WechatpayGoodsID string `json:"wechatpay_goods_id,omitempty"`
	GoodsName        string `json:"goods_name,omitempty"`
	Quantity         int64  `json:"quantity"`
	UnitPrice        int64  `json:"unit_price"`
}

type DirectOrderDetail struct {
	CostPrice   int64                    `json:"cost_price,omitempty"`
	InvoiceID   string                   `json:"invoice_id,omitempty"`
	GoodsDetail []DirectOrderGoodsDetail `json:"goods_detail,omitempty"`
}

type DirectOrderStoreInfo struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	AreaCode string `json:"area_code,omitempty"`
	Address  string `json:"address,omitempty"`
}

type DirectOrderSceneInfo struct {
	PayerClientIP string                `json:"payer_client_ip,omitempty"`
	DeviceID      string                `json:"device_id,omitempty"`
	StoreInfo     *DirectOrderStoreInfo `json:"store_info,omitempty"`
}

type DirectOrderSettleInfo struct {
	ProfitSharing bool `json:"profit_sharing"`
}

// DirectJSAPIOrderRequest is the stable caller-shaped contract used by the
// LocalLife direct-payment chain for JSAPI create.
//
// Keep this contract as the single caller truth, then derive the official wire
// body from it inside the client implementation.
type DirectJSAPIOrderRequest struct {
	Description   string
	OutTradeNo    string
	ExpireTime    time.Time
	Attach        string
	GoodsTag      string
	TotalAmount   int64
	Currency      string
	NotifyURL     string
	PayerOpenID   string
	PayerClientIP string
	DeviceID      string
	StoreInfo     *DirectOrderStoreInfo
	ProfitSharing bool
	SupportFapiao *bool
	Detail        *DirectOrderDetail
}

// 官方文档：POST /v3/pay/transactions/jsapi
type DirectJSAPIOrderRequestBody struct {
	AppID         string                 `json:"appid"`
	MchID         string                 `json:"mchid"`
	Description   string                 `json:"description"`
	OutTradeNo    string                 `json:"out_trade_no"`
	TimeExpire    string                 `json:"time_expire,omitempty"`
	Attach        string                 `json:"attach,omitempty"`
	NotifyURL     string                 `json:"notify_url"`
	GoodsTag      string                 `json:"goods_tag,omitempty"`
	SupportFapiao *bool                  `json:"support_fapiao,omitempty"`
	Amount        DirectOrderAmount      `json:"amount"`
	Payer         DirectOrderPayer       `json:"payer"`
	Detail        *DirectOrderDetail     `json:"detail,omitempty"`
	SceneInfo     *DirectOrderSceneInfo  `json:"scene_info,omitempty"`
	SettleInfo    *DirectOrderSettleInfo `json:"settle_info,omitempty"`
}

// 官方文档：POST /v3/pay/transactions/jsapi
type DirectJSAPIOrderResponse struct {
	PrepayID string `json:"prepay_id"`
}

// 官方文档：GET /v3/pay/transactions/id/{transaction_id}
type DirectQueryOrderByTransactionIDRequest struct {
	TransactionID string `json:"transaction_id"`
	MchID         string `json:"mchid"`
}

// 官方文档：GET /v3/pay/transactions/out-trade-no/{out_trade_no}
type DirectQueryOrderByOutTradeNoRequest struct {
	OutTradeNo string `json:"out_trade_no"`
	MchID      string `json:"mchid"`
}

type DirectOrderQueryAmount struct {
	Total         int64  `json:"total"`
	PayerTotal    int64  `json:"payer_total"`
	Currency      string `json:"currency,omitempty"`
	PayerCurrency string `json:"payer_currency,omitempty"`
}

type DirectOrderQueryPayer struct {
	OpenID string `json:"openid,omitempty"`
}

type DirectOrderQuerySceneInfo struct {
	DeviceID string `json:"device_id,omitempty"`
}

type DirectPromotionGoodsDetail struct {
	GoodsID        string `json:"goods_id"`
	Quantity       int64  `json:"quantity"`
	UnitPrice      int64  `json:"unit_price"`
	DiscountAmount int64  `json:"discount_amount"`
	GoodsRemark    string `json:"goods_remark,omitempty"`
}

type DirectPromotionDetail struct {
	CouponID            string                       `json:"coupon_id"`
	Name                string                       `json:"name,omitempty"`
	Scope               string                       `json:"scope,omitempty"`
	Type                string                       `json:"type,omitempty"`
	Amount              int64                        `json:"amount"`
	StockID             string                       `json:"stock_id,omitempty"`
	WechatpayContribute int64                        `json:"wechatpay_contribute,omitempty"`
	MerchantContribute  int64                        `json:"merchant_contribute,omitempty"`
	OtherContribute     int64                        `json:"other_contribute,omitempty"`
	Currency            string                       `json:"currency,omitempty"`
	GoodsDetail         []DirectPromotionGoodsDetail `json:"goods_detail,omitempty"`
}

// 官方文档：GET /v3/pay/transactions/id/{transaction_id}
// 官方文档：GET /v3/pay/transactions/out-trade-no/{out_trade_no}
type DirectOrderQueryResponse struct {
	AppID           string                     `json:"appid"`
	MchID           string                     `json:"mchid"`
	OutTradeNo      string                     `json:"out_trade_no"`
	TransactionID   string                     `json:"transaction_id,omitempty"`
	TradeType       string                     `json:"trade_type,omitempty"`
	TradeState      string                     `json:"trade_state"`
	TradeStateDesc  string                     `json:"trade_state_desc"`
	BankType        string                     `json:"bank_type,omitempty"`
	Attach          string                     `json:"attach,omitempty"`
	SuccessTime     string                     `json:"success_time,omitempty"`
	Payer           DirectOrderQueryPayer      `json:"payer,omitempty"`
	Amount          DirectOrderQueryAmount     `json:"amount,omitempty"`
	SceneInfo       *DirectOrderQuerySceneInfo `json:"scene_info,omitempty"`
	PromotionDetail []DirectPromotionDetail    `json:"promotion_detail,omitempty"`
}

type DirectEncryptedNotificationResource struct {
	Algorithm      string `json:"algorithm"`
	Ciphertext     string `json:"ciphertext"`
	OriginalType   string `json:"original_type"`
	AssociatedData string `json:"associated_data,omitempty"`
	Nonce          string `json:"nonce"`
}

// 官方文档：支付结果通知外层 envelope
type DirectPaymentNotification struct {
	ID           string                              `json:"id"`
	CreateTime   string                              `json:"create_time"`
	EventType    string                              `json:"event_type"`
	ResourceType string                              `json:"resource_type"`
	Summary      string                              `json:"summary"`
	Resource     DirectEncryptedNotificationResource `json:"resource"`
}

// 官方文档：支付结果通知 resource 解密后字段
type DirectPaymentNotificationResource struct {
	AppID           string                     `json:"appid"`
	MchID           string                     `json:"mchid"`
	OutTradeNo      string                     `json:"out_trade_no"`
	TransactionID   string                     `json:"transaction_id"`
	TradeType       string                     `json:"trade_type"`
	TradeState      string                     `json:"trade_state"`
	TradeStateDesc  string                     `json:"trade_state_desc"`
	BankType        string                     `json:"bank_type"`
	Attach          string                     `json:"attach,omitempty"`
	SuccessTime     string                     `json:"success_time"`
	Payer           DirectOrderQueryPayer      `json:"payer"`
	Amount          DirectOrderQueryAmount     `json:"amount"`
	SceneInfo       *DirectOrderQuerySceneInfo `json:"scene_info,omitempty"`
	PromotionDetail []DirectPromotionDetail    `json:"promotion_detail,omitempty"`
}

// 官方文档：POST /v3/pay/transactions/out-trade-no/{out_trade_no}/close
type DirectCloseOrderRequest struct {
	MchID string `json:"mchid"`
}
