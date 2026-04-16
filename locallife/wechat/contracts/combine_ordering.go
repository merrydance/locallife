package contracts

import "time"

// CombineOrderRequest is the stable caller-shaped request contract for combine
// JSAPI creation in the ordering capability group.
type CombineOrderRequest struct {
	CombineOutTradeNo string
	Description       string
	SubOrders         []SubOrder
	PayerOpenID       string
	PayerSubOpenID    string
	ExpireTime        time.Time
	StartTime         *time.Time
	NotifyURL         string
	SceneInfo         *CombineSceneInfo
}

// CombineOrderRequestBody is the official wire request body for
// POST /v3/combine-transactions/jsapi.
type CombineOrderRequestBody struct {
	CombineAppID      string                   `json:"combine_appid"`
	CombineMchID      string                   `json:"combine_mchid"`
	CombineOutTradeNo string                   `json:"combine_out_trade_no"`
	CombinePayerInfo  CombinePayerInfoRequest  `json:"combine_payer_info"`
	SceneInfo         *CombineSceneInfo        `json:"scene_info,omitempty"`
	SubOrders         []CombineSubOrderRequest `json:"sub_orders"`
	TimeStart         string                   `json:"time_start,omitempty"`
	TimeExpire        string                   `json:"time_expire,omitempty"`
	NotifyURL         string                   `json:"notify_url,omitempty"`
}

// CombinePayerInfoRequest is the official combine_payer_info request payload.
type CombinePayerInfoRequest struct {
	OpenID    string `json:"openid,omitempty"`
	SubOpenID string `json:"sub_openid,omitempty"`
}

// CombineSubOrderRequest is the official sub_orders item for combine create.
type CombineSubOrderRequest struct {
	MchID       string                     `json:"mchid"`
	SubMchID    string                     `json:"sub_mchid,omitempty"`
	SubAppID    string                     `json:"sub_appid,omitempty"`
	OutTradeNo  string                     `json:"out_trade_no"`
	Description string                     `json:"description"`
	Attach      string                     `json:"attach"`
	Detail      string                     `json:"detail,omitempty"`
	Amount      CombineSubOrderAmount      `json:"amount"`
	SettleInfo  *CombineSubOrderSettleInfo `json:"settle_info,omitempty"`
	GoodsTag    string                     `json:"goods_tag,omitempty"`
}

// CombineSubOrderAmount is the official amount payload for combine create.
type CombineSubOrderAmount struct {
	TotalAmount int64  `json:"total_amount"`
	Currency    string `json:"currency"`
}

// CombineSubOrderSettleInfo is the official settle_info payload for combine
// create sub-orders.
type CombineSubOrderSettleInfo struct {
	ProfitSharing bool  `json:"profit_sharing"`
	SubsidyAmount int64 `json:"subsidy_amount,omitempty"`
}

// SubOrder is the stable caller-shaped sub-order contract for combine order
// creation and close flows.
type SubOrder struct {
	MchID         string
	SubMchID      string
	SubAppID      string
	OutTradeNo    string
	Description   string
	Amount        int64
	ProfitSharing bool
	SubsidyAmount int64
	Attach        string
	Detail        string
	GoodsTag      string
}

// CombineSceneInfo is the canonical scene projection for combine-order create
// requests.
type CombineSceneInfo struct {
	PayerClientIP string `json:"payer_client_ip,omitempty"`
	DeviceID      string `json:"device_id,omitempty"`
}

// CombineQuerySceneInfo is the canonical scene projection for combine-order
// query responses.
type CombineQuerySceneInfo struct {
	DeviceID string `json:"device_id,omitempty"`
}

// CombinePaymentNotificationSceneInfo is the canonical scene projection for
// decrypted combine payment notifications.
type CombinePaymentNotificationSceneInfo struct {
	DeviceID string `json:"device_id,omitempty"`
}

// CombineOrderResponse is the canonical response contract for combine order
// creation.
type CombineOrderResponse struct {
	PrepayID string `json:"prepay_id"`
}

// CombineQueryResponse is the canonical response contract for combine order
// queries.
type CombineQueryResponse struct {
	CombineAppID      string                 `json:"combine_appid"`
	CombineMchID      string                 `json:"combine_mchid"`
	CombineOutTradeNo string                 `json:"combine_out_trade_no"`
	SubOrders         []CombineQuerySubOrder `json:"sub_orders"`
	CombinePayerInfo  *CombineQueryPayerInfo `json:"combine_payer_info"`
	SceneInfo         *CombineQuerySceneInfo `json:"scene_info"`
}

// CombinePaymentNotification is the canonical decrypted callback resource for
// combine-order payment notifications.
type CombinePaymentNotification struct {
	CombineAppID      string                               `json:"combine_appid"`
	CombineMchID      string                               `json:"combine_mchid"`
	CombineOutTradeNo string                               `json:"combine_out_trade_no"`
	SubOrders         []CombinePaymentNotificationSubOrder `json:"sub_orders"`
	CombinePayerInfo  *CombinePaymentNotificationPayerInfo `json:"combine_payer_info"`
	SceneInfo         *CombinePaymentNotificationSceneInfo `json:"scene_info"`
}

// CombineQueryResponseBody is the official wire response body for
// GET /v3/combine-transactions/out-trade-no/{combine_out_trade_no}.
type CombineQueryResponseBody struct {
	CombineAppID      string                     `json:"combine_appid"`
	CombineMchID      string                     `json:"combine_mchid"`
	CombineOutTradeNo string                     `json:"combine_out_trade_no"`
	SubOrders         []CombineQuerySubOrderBody `json:"sub_orders,omitempty"`
	CombinePayerInfo  *CombineQueryPayerInfoBody `json:"combine_payer_info,omitempty"`
	SceneInfo         *CombineQuerySceneInfo     `json:"scene_info,omitempty"`
}

// CombineQueryPayerInfoBody is the official combine_payer_info payload for
// combine query responses.
type CombineQueryPayerInfoBody struct {
	OpenID string `json:"openid,omitempty"`
}

// CombineQuerySubOrderBody is the official sub_orders item for combine query
// responses.
type CombineQuerySubOrderBody struct {
	MchID           string                   `json:"mchid"`
	SubMchID        string                   `json:"sub_mchid,omitempty"`
	SubAppID        string                   `json:"sub_appid,omitempty"`
	SubOpenID       string                   `json:"sub_openid,omitempty"`
	OutTradeNo      string                   `json:"out_trade_no"`
	TransactionID   string                   `json:"transaction_id,omitempty"`
	TradeType       string                   `json:"trade_type,omitempty"`
	TradeState      string                   `json:"trade_state"`
	BankType        string                   `json:"bank_type,omitempty"`
	Attach          string                   `json:"attach,omitempty"`
	PromotionDetail []PartnerPromotionDetail `json:"promotion_detail,omitempty"`
	Amount          *CombineQueryAmountBody  `json:"amount,omitempty"`
	SuccessTime     string                   `json:"success_time,omitempty"`
}

// CombineQueryAmountBody is the official amount payload for combine query
// sub-orders.
type CombineQueryAmountBody struct {
	TotalAmount    int64  `json:"total_amount"`
	PayerAmount    int64  `json:"payer_amount"`
	Currency       string `json:"currency"`
	PayerCurrency  string `json:"payer_currency"`
	SettlementRate int64  `json:"settlement_rate,omitempty"`
}

// CombineQuerySubOrder is the canonical projection for a single combine query
// response sub-order.
type CombineQuerySubOrder struct {
	MchID           string                   `json:"mchid"`
	SubMchID        string                   `json:"sub_mchid,omitempty"`
	SubAppID        string                   `json:"sub_appid,omitempty"`
	SubOpenID       string                   `json:"sub_openid,omitempty"`
	OutTradeNo      string                   `json:"out_trade_no"`
	TransactionID   string                   `json:"transaction_id"`
	TradeType       string                   `json:"trade_type,omitempty"`
	TradeState      string                   `json:"trade_state"`
	BankType        string                   `json:"bank_type,omitempty"`
	Attach          string                   `json:"attach,omitempty"`
	PromotionDetail []PartnerPromotionDetail `json:"promotion_detail,omitempty"`
	Amount          struct {
		TotalAmount    int64  `json:"total_amount"`
		PayerAmount    int64  `json:"payer_amount"`
		Currency       string `json:"currency"`
		PayerCurrency  string `json:"payer_currency"`
		SettlementRate int64  `json:"settlement_rate"`
	} `json:"amount"`
	SuccessTime string `json:"success_time"`
}

// CombineQueryPayerInfo is the canonical payer projection for combine order
// query responses.
type CombineQueryPayerInfo struct {
	OpenID string `json:"openid"`
}

// CombineSubOrderResult is a compatibility alias for combine query sub-order
// projections.
type CombineSubOrderResult = CombineQuerySubOrder

// CombinePaymentNotificationSubOrder is the canonical projection for a single
// decrypted combine payment notification sub-order.
type CombinePaymentNotificationSubOrder struct {
	MchID           string                   `json:"mchid"`
	SubMchID        string                   `json:"sub_mchid,omitempty"`
	SubAppID        string                   `json:"sub_appid,omitempty"`
	OutTradeNo      string                   `json:"out_trade_no"`
	TransactionID   string                   `json:"transaction_id"`
	TradeType       string                   `json:"trade_type,omitempty"`
	TradeState      string                   `json:"trade_state"`
	BankType        string                   `json:"bank_type,omitempty"`
	Attach          string                   `json:"attach,omitempty"`
	PromotionDetail []PartnerPromotionDetail `json:"promotion_detail,omitempty"`
	Amount          struct {
		TotalAmount    int64  `json:"total_amount"`
		PayerAmount    int64  `json:"payer_amount"`
		Currency       string `json:"currency"`
		PayerCurrency  string `json:"payer_currency"`
		SettlementRate int64  `json:"settlement_rate"`
	} `json:"amount"`
	SuccessTime string `json:"success_time"`
}

// CombinePaymentNotificationPayerInfo is the canonical payer projection for
// decrypted combine payment notifications.
type CombinePaymentNotificationPayerInfo struct {
	OpenID string `json:"openid"`
}

// SubOrderClose is the canonical caller-shaped sub-order contract for combine
// close requests.
type SubOrderClose struct {
	MchID      string
	SubMchID   string
	SubAppID   string
	OutTradeNo string
}

// CombineCloseSubOrderRequest is the canonical transport body item for combine
// close requests.
type CombineCloseSubOrderRequest struct {
	MchID      string `json:"mchid"`
	SubMchID   string `json:"sub_mchid,omitempty"`
	SubAppID   string `json:"sub_appid,omitempty"`
	OutTradeNo string `json:"out_trade_no"`
}

// CombineCloseOrderRequest is the canonical request body for combine close.
type CombineCloseOrderRequest struct {
	CombineAppID string                        `json:"combine_appid"`
	SubOrders    []CombineCloseSubOrderRequest `json:"sub_orders"`
}
