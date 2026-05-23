package contracts

// ShippingSettlementEventType is the Mini Program shipping settlement callback
// event emitted after user confirmation or automatic confirmation.
const ShippingSettlementEventType = "trade_manage_order_settlement"

// ShippingSettlementNotificationResource is the decrypted Mini Program shipping
// settlement event resource. This callback belongs to retained Mini Program
// shipping compliance helpers, not to retired WeChat platform payment acquisition.
type ShippingSettlementNotificationResource struct {
	AppID                string `json:"appid"`
	TransactionID        string `json:"transaction_id"`
	MerchantTradeNo      string `json:"merchant_trade_no"`
	ConfirmReceiveMethod int    `json:"confirm_receive_method"`
	SettlementTime       string `json:"settlement_time"`
	SuccessTime          string `json:"success_time"`
}
