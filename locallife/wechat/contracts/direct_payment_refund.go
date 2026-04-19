package contracts

// 官方文档：直连支付组退款面
// 申请退款：https://pay.weixin.qq.com/doc/v3/merchant/4012791903.md
// 查询单笔退款：https://pay.weixin.qq.com/doc/v3/merchant/4012791904.md
// 发起异常退款：https://pay.weixin.qq.com/doc/v3/merchant/4012791905.md
// 退款结果通知：https://pay.weixin.qq.com/doc/v3/merchant/4012791906.md

const (
	DirectRefundRequestFundsAccountAvailable = "AVAILABLE"
	DirectRefundRequestFundsAccountUnsettle  = "UNSETTLED"
)

const (
	DirectRefundCurrencyCNY = "CNY"
)

const (
	DirectRefundChannelOriginal      = "ORIGINAL"
	DirectRefundChannelBalance       = "BALANCE"
	DirectRefundChannelOtherBalance  = "OTHER_BALANCE"
	DirectRefundChannelOtherBankCard = "OTHER_BANKCARD"
)

const (
	DirectRefundStatusSuccess    = "SUCCESS"
	DirectRefundStatusClosed     = "CLOSED"
	DirectRefundStatusProcessing = "PROCESSING"
	DirectRefundStatusAbnormal   = "ABNORMAL"
)

const (
	DirectRefundFundsAccountUnsetttled  = "UNSETTLED"
	DirectRefundFundsAccountAvailable   = "AVAILABLE"
	DirectRefundFundsAccountUnavailable = "UNAVAILABLE"
	DirectRefundFundsAccountOperation   = "OPERATION"
	DirectRefundFundsAccountBasic       = "BASIC"
	DirectRefundFundsAccountECNYBasic   = "ECNY_BASIC"
)

const (
	DirectAbnormalRefundTypeUserBankCard     = "USER_BANK_CARD"
	DirectAbnormalRefundTypeMerchantBankCard = "MERCHANT_BANK_CARD"
)

const (
	DirectRefundNotifyEventTypeSuccess  = "REFUND.SUCCESS"
	DirectRefundNotifyEventTypeAbnormal = "REFUND.ABNORMAL"
	DirectRefundNotifyEventTypeClosed   = "REFUND.CLOSED"
	DirectRefundNotifyOriginalType      = "refund"
)

type DirectRefundRequestAmount struct {
	Refund   int64                    `json:"refund"`
	From     []DirectRefundAmountFrom `json:"from,omitempty"`
	Total    int64                    `json:"total"`
	Currency string                   `json:"currency,omitempty"`
}

type DirectRefundGoodsDetail struct {
	MerchantGoodsID  string `json:"merchant_goods_id"`
	WechatpayGoodsID string `json:"wechatpay_goods_id,omitempty"`
	GoodsName        string `json:"goods_name,omitempty"`
	UnitPrice        int64  `json:"unit_price"`
	RefundAmount     int64  `json:"refund_amount"`
	RefundQuantity   int64  `json:"refund_quantity,omitempty"`
}

// 官方文档：POST /v3/refund/domestic/refunds
type DirectRefundRequest struct {
	TransactionID string                     `json:"transaction_id,omitempty"`
	OutTradeNo    string                     `json:"out_trade_no,omitempty"`
	OutRefundNo   string                     `json:"out_refund_no"`
	Reason        string                     `json:"reason,omitempty"`
	NotifyURL     string                     `json:"notify_url,omitempty"`
	FundsAccount  string                     `json:"funds_account,omitempty"`
	Amount        *DirectRefundRequestAmount `json:"amount"`
	GoodsDetail   []DirectRefundGoodsDetail  `json:"goods_detail,omitempty"`
}

type DirectRefundAmountFrom struct {
	Account string `json:"account,omitempty"`
	Amount  int64  `json:"amount,omitempty"`
}

type DirectRefundAmount struct {
	Total            int64                    `json:"total,omitempty"`
	Refund           int64                    `json:"refund"`
	From             []DirectRefundAmountFrom `json:"from,omitempty"`
	PayerTotal       int64                    `json:"payer_total,omitempty"`
	PayerRefund      int64                    `json:"payer_refund,omitempty"`
	SettlementRefund int64                    `json:"settlement_refund,omitempty"`
	SettlementTotal  int64                    `json:"settlement_total,omitempty"`
	DiscountRefund   int64                    `json:"discount_refund,omitempty"`
	RefundFee        int64                    `json:"refund_fee,omitempty"`
	Currency         string                   `json:"currency,omitempty"`
}

type DirectRefundPromotionGoodsDetail struct {
	MerchantGoodsID  string `json:"merchant_goods_id"`
	WechatpayGoodsID string `json:"wechatpay_goods_id,omitempty"`
	GoodsName        string `json:"goods_name,omitempty"`
	UnitPrice        int64  `json:"unit_price"`
	RefundAmount     int64  `json:"refund_amount,omitempty"`
	RefundQuantity   int64  `json:"refund_quantity,omitempty"`
}

type DirectRefundPromotionDetail struct {
	PromotionID  string                             `json:"promotion_id"`
	Scope        string                             `json:"scope,omitempty"`
	Type         string                             `json:"type,omitempty"`
	Amount       int64                              `json:"amount,omitempty"`
	RefundAmount int64                              `json:"refund_amount,omitempty"`
	GoodsDetail  []DirectRefundPromotionGoodsDetail `json:"goods_detail,omitempty"`
}

// 官方文档：POST /v3/refund/domestic/refunds
// 官方文档：GET /v3/refund/domestic/refunds/{out_refund_no}
// 官方文档：POST /v3/refund/domestic/refunds/{refund_id}/apply-abnormal-refund
type DirectRefundResponse struct {
	RefundID            string                        `json:"refund_id"`
	OutRefundNo         string                        `json:"out_refund_no"`
	TransactionID       string                        `json:"transaction_id,omitempty"`
	OutTradeNo          string                        `json:"out_trade_no,omitempty"`
	Channel             string                        `json:"channel,omitempty"`
	UserReceivedAccount string                        `json:"user_received_account,omitempty"`
	SuccessTime         string                        `json:"success_time,omitempty"`
	CreateTime          string                        `json:"create_time,omitempty"`
	Status              string                        `json:"status,omitempty"`
	FundsAccount        string                        `json:"funds_account,omitempty"`
	Amount              DirectRefundAmount            `json:"amount"`
	PromotionDetail     []DirectRefundPromotionDetail `json:"promotion_detail,omitempty"`
}

// 官方文档：GET /v3/refund/domestic/refunds/{out_refund_no}
type DirectQueryRefundByOutRefundNoRequest struct {
	OutRefundNo string `json:"out_refund_no"`
}

// 官方文档：POST /v3/refund/domestic/refunds/{refund_id}/apply-abnormal-refund
type DirectAbnormalRefundRequest struct {
	RefundID    string `json:"refund_id"`
	OutRefundNo string `json:"out_refund_no"`
	Type        string `json:"type"`
	BankType    string `json:"bank_type,omitempty"`
	BankAccount string `json:"bank_account,omitempty"`
	RealName    string `json:"real_name,omitempty"`
}

// 官方文档：退款结果通知外层 envelope
type DirectRefundNotification struct {
	ID           string                              `json:"id"`
	CreateTime   string                              `json:"create_time"`
	EventType    string                              `json:"event_type"`
	ResourceType string                              `json:"resource_type"`
	Summary      string                              `json:"summary"`
	Resource     DirectEncryptedNotificationResource `json:"resource"`
}

type DirectRefundNotificationAmount struct {
	Total       int64 `json:"total"`
	Refund      int64 `json:"refund"`
	PayerTotal  int64 `json:"payer_total"`
	PayerRefund int64 `json:"payer_refund"`
}

// 官方文档：退款结果通知 resource 解密后字段
type DirectRefundNotificationResource struct {
	MchID               string                         `json:"mchid"`
	OutTradeNo          string                         `json:"out_trade_no"`
	TransactionID       string                         `json:"transaction_id"`
	OutRefundNo         string                         `json:"out_refund_no"`
	RefundID            string                         `json:"refund_id"`
	RefundStatus        string                         `json:"refund_status"`
	SuccessTime         string                         `json:"success_time,omitempty"`
	UserReceivedAccount string                         `json:"user_received_account,omitempty"`
	Amount              DirectRefundNotificationAmount `json:"amount"`
}
