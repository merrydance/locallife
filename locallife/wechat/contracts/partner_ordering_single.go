package contracts

import "time"

// PartnerJSAPIOrderRequest is the stable request shape currently used by the
// LocalLife caller chain for partner single-order JSAPI creation.
//
// Note: this is still a caller-shaped contract rather than the raw wire JSON.
// Keep the canonical definition here first, then evolve transport-specific
// structs in the same package when the caller layer is ready to split.
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
	StoreInfo      *PartnerOrderStoreInfo
	ProfitSharing  bool
	SubsidyAmount  int64
	SupportFapiao  *bool
	Detail         *PartnerOrderDetail
}

// PartnerJSAPIOrderRequestBody is the official wire request body for
// POST /v3/pay/partner/transactions/jsapi.
type PartnerJSAPIOrderRequestBody struct {
	SpAppID       string                  `json:"sp_appid"`
	SpMchID       string                  `json:"sp_mchid"`
	SubAppID      string                  `json:"sub_appid,omitempty"`
	SubMchID      string                  `json:"sub_mchid"`
	Description   string                  `json:"description"`
	OutTradeNo    string                  `json:"out_trade_no"`
	TimeExpire    string                  `json:"time_expire,omitempty"`
	Attach        string                  `json:"attach,omitempty"`
	NotifyURL     string                  `json:"notify_url"`
	GoodsTag      string                  `json:"goods_tag,omitempty"`
	SupportFapiao *bool                   `json:"support_fapiao,omitempty"`
	Amount        PartnerJSAPIAmount      `json:"amount"`
	Payer         PartnerJSAPIPayer       `json:"payer"`
	Detail        *PartnerOrderDetail     `json:"detail,omitempty"`
	SceneInfo     *PartnerOrderSceneInfo  `json:"scene_info,omitempty"`
	SettleInfo    *PartnerOrderSettleInfo `json:"settle_info,omitempty"`
}

// PartnerJSAPIAmount is the official amount payload for partner JSAPI create.
type PartnerJSAPIAmount struct {
	Total    int64  `json:"total"`
	Currency string `json:"currency,omitempty"`
}

// PartnerJSAPIPayer is the official payer payload for partner JSAPI create.
type PartnerJSAPIPayer struct {
	SpOpenID  string `json:"sp_openid,omitempty"`
	SubOpenID string `json:"sub_openid,omitempty"`
}

// PartnerOrderSettleInfo is the official settle_info payload for partner
// ordering requests.
type PartnerOrderSettleInfo struct {
	ProfitSharing bool  `json:"profit_sharing"`
	SubsidyAmount int64 `json:"subsidy_amount,omitempty"`
}

// PartnerJSAPIOrderResponse is the canonical response contract for partner
// single-order JSAPI creation.
type PartnerJSAPIOrderResponse struct {
	PrepayID string `json:"prepay_id"`
}

// PartnerOrderPayerInfo is the canonical payer projection shared by query and
// callback payloads in the ordering capability group.
type PartnerOrderPayerInfo struct {
	SpOpenID  string `json:"sp_openid,omitempty"`
	SubOpenID string `json:"sub_openid,omitempty"`
}

// PartnerOrderSceneInfo is the canonical scene projection shared by partner
// single-order request and response contracts.
type PartnerOrderSceneInfo struct {
	PayerClientIP string                 `json:"payer_client_ip,omitempty"`
	DeviceID      string                 `json:"device_id,omitempty"`
	StoreInfo     *PartnerOrderStoreInfo `json:"store_info,omitempty"`
}

// PartnerOrderStoreInfo is the official store_info payload shared by partner
// create scene_info and related caller projections.
type PartnerOrderStoreInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	AreaCode string `json:"area_code,omitempty"`
	Address  string `json:"address,omitempty"`
}

// PartnerOrderDetail is the official detail payload for partner single-order
// create requests.
type PartnerOrderDetail struct {
	CostPrice   int64                     `json:"cost_price,omitempty"`
	InvoiceID   string                    `json:"invoice_id,omitempty"`
	GoodsDetail []PartnerOrderGoodsDetail `json:"goods_detail,omitempty"`
}

// PartnerOrderGoodsDetail is the official goods_detail item for partner
// single-order create requests.
type PartnerOrderGoodsDetail struct {
	MerchantGoodsID  string `json:"merchant_goods_id"`
	WechatpayGoodsID string `json:"wechatpay_goods_id,omitempty"`
	GoodsName        string `json:"goods_name,omitempty"`
	Quantity         int64  `json:"quantity"`
	UnitPrice        int64  `json:"unit_price"`
}

// PartnerOrderQueryAmount is the canonical amount projection for partner order
// query responses.
type PartnerOrderQueryAmount struct {
	Total         int64  `json:"total"`
	PayerTotal    int64  `json:"payer_total"`
	Currency      string `json:"currency"`
	PayerCurrency string `json:"payer_currency"`
}

// PartnerOrderQuerySceneInfo is the canonical scene_info projection for partner
// order query responses.
type PartnerOrderQuerySceneInfo struct {
	DeviceID string `json:"device_id"`
}

// PartnerPromotionGoodsDetail is the canonical goods-detail projection inside
// promotion_detail.
type PartnerPromotionGoodsDetail struct {
	GoodsID        string `json:"goods_id"`
	Quantity       int64  `json:"quantity"`
	UnitPrice      int64  `json:"unit_price"`
	DiscountAmount int64  `json:"discount_amount"`
	GoodsRemark    string `json:"goods_remark,omitempty"`
}

// PartnerPromotionDetail is the canonical promotion_detail projection for
// partner order query responses.
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

// PartnerOrderQueryResponse is the canonical response contract shared by query
// by transaction_id and query by out_trade_no.
type PartnerOrderQueryResponse struct {
	SpAppID         string                      `json:"sp_appid"`
	SpMchID         string                      `json:"sp_mchid"`
	SubAppID        string                      `json:"sub_appid,omitempty"`
	SubMchID        string                      `json:"sub_mchid"`
	OutTradeNo      string                      `json:"out_trade_no"`
	TransactionID   string                      `json:"transaction_id,omitempty"`
	TradeType       string                      `json:"trade_type,omitempty"`
	TradeState      string                      `json:"trade_state"`
	TradeStateDesc  string                      `json:"trade_state_desc"`
	BankType        string                      `json:"bank_type,omitempty"`
	Attach          string                      `json:"attach,omitempty"`
	SuccessTime     string                      `json:"success_time,omitempty"`
	Payer           PartnerOrderPayerInfo       `json:"payer,omitempty"`
	Amount          PartnerOrderQueryAmount     `json:"amount,omitempty"`
	SceneInfo       *PartnerOrderQuerySceneInfo `json:"scene_info,omitempty"`
	PromotionDetail []PartnerPromotionDetail    `json:"promotion_detail,omitempty"`
}

// PartnerPaymentNotificationResource is the canonical decrypted callback
// resource for partner single-order payment notifications.
type PartnerPaymentNotificationResource struct {
	SpAppID         string                      `json:"sp_appid"`
	SpMchID         string                      `json:"sp_mchid"`
	SubAppID        string                      `json:"sub_appid,omitempty"`
	SubMchID        string                      `json:"sub_mchid"`
	OutTradeNo      string                      `json:"out_trade_no"`
	TransactionID   string                      `json:"transaction_id"`
	TradeType       string                      `json:"trade_type"`
	TradeState      string                      `json:"trade_state"`
	TradeStateDesc  string                      `json:"trade_state_desc"`
	BankType        string                      `json:"bank_type"`
	Attach          string                      `json:"attach,omitempty"`
	SuccessTime     string                      `json:"success_time"`
	Payer           PartnerOrderPayerInfo       `json:"payer"`
	Amount          PartnerOrderQueryAmount     `json:"amount"`
	SceneInfo       *PartnerOrderQuerySceneInfo `json:"scene_info,omitempty"`
	PromotionDetail []PartnerPromotionDetail    `json:"promotion_detail,omitempty"`
}

// PartnerCloseOrderRequest is the canonical request body for partner single
// close order.
type PartnerCloseOrderRequest struct {
	SpMchID  string `json:"sp_mchid"`
	SubMchID string `json:"sub_mchid"`
}
