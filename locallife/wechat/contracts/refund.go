package contracts

// 官方文档：交易退款组
// 申请退款：https://pay.weixin.qq.com/doc/v3/partner/4012476892.md
// 查询单笔退款（按微信支付退款单号）：https://pay.weixin.qq.com/doc/v3/partner/4012476908.md
// 查询单笔退款（按商户退款单号）：https://pay.weixin.qq.com/doc/v3/partner/4012476911.md
// 退款结果通知：https://pay.weixin.qq.com/doc/v3/partner/4012124635.md
// 查询垫付回补通知：https://pay.weixin.qq.com/doc/v3/partner/4012476916.md
// 垫付退款回补：https://pay.weixin.qq.com/doc/v3/partner/4012476927.md
// 发起异常退款：https://pay.weixin.qq.com/doc/v3/partner/4015181616.md

const (
	EcommerceRefundAmountAccountAvailable   = "AVAILABLE"
	EcommerceRefundAmountAccountUnavailable = "UNAVAILABLE"
)

const (
	EcommerceRefundCurrencyCNY = "CNY"
)

const (
	EcommerceRefundChannelOriginal      = "ORIGINAL"
	EcommerceRefundChannelBalance       = "BALANCE"
	EcommerceRefundChannelOtherBalance  = "OTHER_BALANCE"
	EcommerceRefundChannelOtherBankCard = "OTHER_BANKCARD"
)

const (
	EcommerceRefundStatusSuccess    = "SUCCESS"
	EcommerceRefundStatusClosed     = "CLOSED"
	EcommerceRefundStatusProcessing = "PROCESSING"
	EcommerceRefundStatusAbnormal   = "ABNORMAL"
)

const (
	EcommerceRefundPromotionScopeGlobal = "GLOBAL"
	EcommerceRefundPromotionScopeSingle = "SINGLE"
)

const (
	EcommerceRefundPromotionTypeCoupon   = "COUPON"
	EcommerceRefundPromotionTypeDiscount = "DISCOUNT"
)

const (
	EcommerceRefundSourcePartnerAdvance = "REFUND_SOURCE_PARTNER_ADVANCE"
	EcommerceRefundSourceSubMerchant    = "REFUND_SOURCE_SUB_MERCHANT"
)

const (
	EcommerceRefundFundsAccountUnsetttled  = "UNSETTLED"
	EcommerceRefundFundsAccountAvailable   = "AVAILABLE"
	EcommerceRefundFundsAccountUnavailable = "UNAVAILABLE"
	EcommerceRefundFundsAccountOperation   = "OPERATION"
	EcommerceRefundFundsAccountBasic       = "BASIC"
	EcommerceRefundFundsAccountECNYBasic   = "ECNY_BASIC"
)

const (
	EcommerceAbnormalRefundTypeUserBankCard     = "USER_BANK_CARD"
	EcommerceAbnormalRefundTypeMerchantBankCard = "MERCHANT_BANK_CARD"
)

const (
	EcommerceRefundAdvanceAccountBasic     = "BASIC"
	EcommerceRefundAdvanceAccountOperation = "OPERATION"
)

const (
	EcommerceRefundAdvanceReturnResultSuccess    = "SUCCESS"
	EcommerceRefundAdvanceReturnResultFailed     = "FAILED"
	EcommerceRefundAdvanceReturnResultProcessing = "PROCESSING"
)

type EcommerceRefundAmountFrom struct {
	Account string `json:"account"`
	Amount  int64  `json:"amount"`
}

type EcommerceRefundRequestAmount struct {
	Refund   int64                       `json:"refund"`
	From     []EcommerceRefundAmountFrom `json:"from,omitempty"`
	Total    int64                       `json:"total"`
	Currency string                      `json:"currency,omitempty"`
}

type EcommerceRefundAmount struct {
	Refund         int64                       `json:"refund"`
	From           []EcommerceRefundAmountFrom `json:"from,omitempty"`
	PayerRefund    int64                       `json:"payer_refund"`
	DiscountRefund int64                       `json:"discount_refund,omitempty"`
	Currency       string                      `json:"currency,omitempty"`
	Advance        int64                       `json:"advance,omitempty"`
}

type EcommerceRefundPromotionDetail struct {
	PromotionID  string `json:"promotion_id"`
	Scope        string `json:"scope"`
	Type         string `json:"type"`
	Amount       int64  `json:"amount"`
	RefundAmount int64  `json:"refund_amount"`
}

// 官方文档：POST /v3/ecommerce/refunds/apply
type EcommerceRefundRequest struct {
	SubMchID      string                        `json:"sub_mchid"`
	SpAppID       string                        `json:"sp_appid"`
	SubAppID      string                        `json:"sub_appid,omitempty"`
	TransactionID string                        `json:"transaction_id,omitempty"`
	OutTradeNo    string                        `json:"out_trade_no,omitempty"`
	OutRefundNo   string                        `json:"out_refund_no"`
	Reason        string                        `json:"reason,omitempty"`
	Amount        *EcommerceRefundRequestAmount `json:"amount"`
	NotifyURL     string                        `json:"notify_url,omitempty"`
	RefundAccount string                        `json:"refund_account,omitempty"`
	FundsAccount  string                        `json:"funds_account,omitempty"`
}

// 官方文档：POST /v3/ecommerce/refunds/apply
type EcommerceRefundCreateResponse struct {
	RefundID        string                           `json:"refund_id"`
	OutRefundNo     string                           `json:"out_refund_no"`
	CreateTime      string                           `json:"create_time"`
	Amount          EcommerceRefundAmount            `json:"amount"`
	PromotionDetail []EcommerceRefundPromotionDetail `json:"promotion_detail,omitempty"`
	RefundAccount   string                           `json:"refund_account,omitempty"`
}

// 官方文档：GET /v3/ecommerce/refunds/id/{refund_id}
type EcommerceRefundQueryByIDRequest struct {
	RefundID string `json:"refund_id"`
	SubMchID string `json:"sub_mchid"`
}

// 官方文档：GET /v3/ecommerce/refunds/out-refund-no/{out_refund_no}
type EcommerceRefundQueryByOutRefundNoRequest struct {
	OutRefundNo string `json:"out_refund_no"`
	SubMchID    string `json:"sub_mchid"`
}

// 官方文档：GET /v3/ecommerce/refunds/id/{refund_id}
// 官方文档：GET /v3/ecommerce/refunds/out-refund-no/{out_refund_no}
type EcommerceRefundQueryResponse struct {
	RefundID            string                           `json:"refund_id"`
	OutRefundNo         string                           `json:"out_refund_no"`
	TransactionID       string                           `json:"transaction_id"`
	OutTradeNo          string                           `json:"out_trade_no"`
	Channel             string                           `json:"channel,omitempty"`
	UserReceivedAccount string                           `json:"user_received_account,omitempty"`
	SuccessTime         string                           `json:"success_time,omitempty"`
	CreateTime          string                           `json:"create_time"`
	Status              string                           `json:"status"`
	Amount              EcommerceRefundAmount            `json:"amount"`
	PromotionDetail     []EcommerceRefundPromotionDetail `json:"promotion_detail,omitempty"`
	RefundAccount       string                           `json:"refund_account,omitempty"`
	FundsAccount        string                           `json:"funds_account,omitempty"`
}

// 官方文档：POST /v3/ecommerce/refunds/{refund_id}/apply-abnormal-refund
type EcommerceAbnormalRefundRequest struct {
	RefundID    string `json:"refund_id"`
	SubMchID    string `json:"sub_mchid"`
	OutRefundNo string `json:"out_refund_no"`
	Type        string `json:"type"`
	BankType    string `json:"bank_type,omitempty"`
	BankAccount string `json:"bank_account,omitempty"`
	RealName    string `json:"real_name,omitempty"`
}

type EcommerceRefundNotificationAmount struct {
	Total       int64 `json:"total"`
	Refund      int64 `json:"refund"`
	PayerTotal  int64 `json:"payer_total"`
	PayerRefund int64 `json:"payer_refund"`
}

// 官方文档：退款结果通知 resource 解密后字段
type EcommerceRefundNotification struct {
	SPMchID             string                            `json:"sp_mchid"`
	SubMchID            string                            `json:"sub_mchid"`
	OutTradeNo          string                            `json:"out_trade_no"`
	TransactionID       string                            `json:"transaction_id"`
	OutRefundNo         string                            `json:"out_refund_no"`
	RefundID            string                            `json:"refund_id"`
	RefundStatus        string                            `json:"refund_status"`
	SuccessTime         string                            `json:"success_time,omitempty"`
	UserReceivedAccount string                            `json:"user_received_account"`
	Amount              EcommerceRefundNotificationAmount `json:"amount"`
	RefundAccount       string                            `json:"refund_account,omitempty"`
}

// 官方文档：GET /v3/ecommerce/refunds/{refund_id}/return-advance
type EcommerceRefundAdvanceReturnQueryRequest struct {
	RefundID string `json:"refund_id"`
	SubMchID string `json:"sub_mchid"`
}

// 官方文档：POST /v3/ecommerce/refunds/{refund_id}/return-advance
type EcommerceRefundAdvanceReturnRequest struct {
	RefundID string `json:"refund_id"`
	SubMchID string `json:"sub_mchid"`
}

// 官方文档：GET/POST /v3/ecommerce/refunds/{refund_id}/return-advance
type EcommerceRefundAdvanceReturnResponse struct {
	RefundID        string `json:"refund_id"`
	AdvanceReturnID string `json:"advance_return_id"`
	ReturnAmount    int64  `json:"return_amount"`
	PayerMchID      string `json:"payer_mchid"`
	PayerAccount    string `json:"payer_account"`
	PayeeMchID      string `json:"payee_mchid"`
	PayeeAccount    string `json:"payee_account"`
	Result          string `json:"result"`
	SuccessTime     string `json:"success_time,omitempty"`
}
