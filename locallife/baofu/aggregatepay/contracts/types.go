package contracts

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	ProductTypeSharing         = "SHARING"
	BaoCaiTongOrderTypeSharing = "7"
	PayCodeWechatJSAPI         = "WECHAT_JSAPI"

	PaymentStateSuccess    = "SUCCESS"
	PaymentStateClosed     = "CLOSED"
	PaymentStateWaitPaying = "WAIT_PAYING"
	PaymentStatePayError   = "PAY_ERROR"
	PaymentStateRefund     = "REFUND"
	PaymentStateAbnormal   = "ABNORMAL"

	ShareStateSuccess    = "SUCCESS"
	ShareStateProcessing = "PROCESSING"
	ShareStateCanceled   = "CANCELED"
	ShareStateAbnormal   = "ABNORMAL"

	RefundStateSuccess  = "SUCCESS"
	RefundStateAccepted = "REFUND"
	RefundStateError    = "REFUND_ERROR"
	RefundStateAbnormal = "ABNORMAL"
)

var (
	ErrUnifiedOrderMerchantIDRequired         = errors.New("baofu unified order merId is required")
	ErrUnifiedOrderTerminalIDRequired         = errors.New("baofu unified order terId is required")
	ErrUnifiedOrderOutTradeNoRequired         = errors.New("baofu unified order outTradeNo is required")
	ErrUnifiedOrderAmountInvalid              = errors.New("baofu unified order txnAmt must be positive")
	ErrUnifiedOrderTotalAmountInvalid         = errors.New("baofu unified order totalAmt must be greater than or equal to txnAmt")
	ErrUnifiedOrderTransactionTimeInvalid     = errors.New("baofu unified order txnTime must use yyyyMMddHHmmss")
	ErrUnifiedOrderProductTypeUnsupported     = errors.New("baofu unified order prodType must be SHARING")
	ErrUnifiedOrderOrderTypeUnsupported       = errors.New("baofu unified order orderType must be 7")
	ErrUnifiedOrderPayCodeUnsupported         = errors.New("baofu unified order payCode is unsupported")
	ErrUnifiedOrderSubMchIDRequired           = errors.New("baofu unified order subMchId is required for wechat/alipay")
	ErrUnifiedOrderPayExtendSubAppIDRequired  = errors.New("baofu unified order payExtend.sub_appid is required")
	ErrUnifiedOrderPayExtendSubOpenIDRequired = errors.New("baofu unified order payExtend.sub_openid is required")
	ErrUnifiedOrderPayExtendBodyRequired      = errors.New("baofu unified order payExtend.body is required")
	ErrUnifiedOrderRiskInfoClientIPRequired   = errors.New("baofu unified order riskInfo.clientIp is required for wechat/alipay")
	ErrUnifiedOrderPageURLInvalid             = errors.New("baofu unified order pageUrl must be https")
	ErrUnifiedOrderForbidCreditUnsupported    = errors.New("baofu unified order forbidCredit must be 0 or 1")

	ErrBaofuRefundMerchantIDRequired           = errors.New("baofu refund merId is required")
	ErrBaofuRefundTerminalIDRequired           = errors.New("baofu refund terId is required")
	ErrBaofuRefundOriginalReferenceRequired    = errors.New("baofu refund originTradeNo or originOutTradeNo is required")
	ErrBaofuRefundOutTradeNoRequired           = errors.New("baofu refund outTradeNo is required")
	ErrBaofuRefundAmountInvalid                = errors.New("baofu refund refundAmt must be positive")
	ErrBaofuRefundTotalAmountInvalid           = errors.New("baofu refund totalAmt must be greater than or equal to refundAmt")
	ErrBaofuRefundTransactionTimeInvalid       = errors.New("baofu refund txnTime must use yyyyMMddHHmmss")
	ErrBaofuRefundReasonRequired               = errors.New("baofu refund refundReason is required")
	ErrBaofuRefundSharingRefundInfoUnsupported = errors.New("baofu refund sharingRefundInfo is unsupported before profit sharing")
	ErrBaofuRefundAdvanceUnsupported           = errors.New("baofu refund advanceAmt is unsupported")
	ErrBaofuPaymentQueryMerchantIDRequired     = errors.New("baofu payment query merId is required")
	ErrBaofuPaymentQueryTerminalIDRequired     = errors.New("baofu payment query terId is required")
	ErrBaofuPaymentQueryReferenceRequired      = errors.New("baofu payment query tradeNo or outTradeNo is required")
	ErrBaofuShareQueryMerchantIDRequired       = errors.New("baofu share query merId is required")
	ErrBaofuShareQueryTerminalIDRequired       = errors.New("baofu share query terId is required")
	ErrBaofuShareQueryReferenceRequired        = errors.New("baofu share query tradeNo or outTradeNo is required")
	ErrBaofuShareTransactionTimeInvalid        = errors.New("baofu share txnTime must use yyyyMMddHHmmss")
	ErrBaofuRefundQueryMerchantIDRequired      = errors.New("baofu refund query merId is required")
	ErrBaofuRefundQueryTerminalIDRequired      = errors.New("baofu refund query terId is required")
	ErrBaofuRefundQueryReferenceRequired       = errors.New("baofu refund query tradeNo or outTradeNo is required")
	ErrBaofuOrderCloseMerchantIDRequired       = errors.New("baofu order close merId is required")
	ErrBaofuOrderCloseTerminalIDRequired       = errors.New("baofu order close terId is required")
	ErrBaofuOrderCloseReferenceRequired        = errors.New("baofu order close tradeNo or outTradeNo is required")
)

type UnifiedOrderInput struct {
	MerchantID string
	TerminalID string
	OutTradeNo string
	AmountFen  int64
	TxnTime    string
	TimeExpire int
	SubMchID   string
	SubAppID   string
	SubOpenID  string
	Body       string
	NotifyURL  string
	PageURL    string
	ClientIP   string
	Attach     string
}

type UnifiedOrderRequest struct {
	AgentMerchantID string         `json:"agentMerId,omitempty"`
	AgentTerminalID string         `json:"agentTerId,omitempty"`
	MerchantID      string         `json:"merId"`
	TerminalID      string         `json:"terId"`
	OutTradeNo      string         `json:"outTradeNo"`
	TransactionAmt  int64          `json:"txnAmt"`
	TransactionTime string         `json:"txnTime"`
	TotalAmt        int64          `json:"totalAmt"`
	TimeExpire      int            `json:"timeExpire,omitempty"`
	ProductType     string         `json:"prodType"`
	OrderType       string         `json:"orderType"`
	PayCode         string         `json:"payCode"`
	PayExtend       PayExtend      `json:"payExtend"`
	SubMchID        string         `json:"subMchId,omitempty"`
	NotifyURL       string         `json:"notifyUrl,omitempty"`
	PageURL         string         `json:"pageUrl,omitempty"`
	ForbidCredit    string         `json:"forbidCredit,omitempty"`
	Attach          string         `json:"attach,omitempty"`
	RequestReserved string         `json:"reqReserved,omitempty"`
	MarketingInfo   *MarketingInfo `json:"mktInfo,omitempty"`
	RiskInfo        *RiskInfo      `json:"riskInfo,omitempty"`
}

type PayExtend struct {
	SubAppID  string `json:"sub_appid,omitempty"`
	SubOpenID string `json:"sub_openid,omitempty"`
	Body      string `json:"body,omitempty"`
}

type RiskInfo struct {
	ClientIP      string `json:"clientIp,omitempty"`
	LocationPoint string `json:"locationPoint,omitempty"`
}

type MarketingInfo struct {
	MerchantID string `json:"mktMerId"`
	AmountFen  int64  `json:"mktAmt"`
}

func NewWechatJSAPISharingUnifiedOrderRequest(input UnifiedOrderInput) UnifiedOrderRequest {
	req := UnifiedOrderRequest{
		MerchantID:      strings.TrimSpace(input.MerchantID),
		TerminalID:      strings.TrimSpace(input.TerminalID),
		OutTradeNo:      strings.TrimSpace(input.OutTradeNo),
		TransactionAmt:  input.AmountFen,
		TransactionTime: strings.TrimSpace(input.TxnTime),
		TotalAmt:        input.AmountFen,
		TimeExpire:      input.TimeExpire,
		ProductType:     ProductTypeSharing,
		OrderType:       BaoCaiTongOrderTypeSharing,
		PayCode:         PayCodeWechatJSAPI,
		PayExtend: PayExtend{
			SubAppID:  strings.TrimSpace(input.SubAppID),
			SubOpenID: strings.TrimSpace(input.SubOpenID),
			Body:      strings.TrimSpace(input.Body),
		},
		SubMchID:  strings.TrimSpace(input.SubMchID),
		NotifyURL: strings.TrimSpace(input.NotifyURL),
		PageURL:   strings.TrimSpace(input.PageURL),
		Attach:    strings.TrimSpace(input.Attach),
	}
	if clientIP := strings.TrimSpace(input.ClientIP); clientIP != "" {
		req.RiskInfo = &RiskInfo{ClientIP: clientIP}
	}
	return req
}

func (r UnifiedOrderRequest) Validate() error {
	return r.ValidateForEnvironment("")
}

func (r UnifiedOrderRequest) ValidateForEnvironment(environment string) error {
	if strings.TrimSpace(r.MerchantID) == "" {
		return ErrUnifiedOrderMerchantIDRequired
	}
	if strings.TrimSpace(r.TerminalID) == "" {
		return ErrUnifiedOrderTerminalIDRequired
	}
	if strings.TrimSpace(r.OutTradeNo) == "" {
		return ErrUnifiedOrderOutTradeNoRequired
	}
	if r.TransactionAmt <= 0 {
		return ErrUnifiedOrderAmountInvalid
	}
	if r.TotalAmt < r.TransactionAmt {
		return ErrUnifiedOrderTotalAmountInvalid
	}
	if !isBaofuTransactionTime(r.TransactionTime) {
		return ErrUnifiedOrderTransactionTimeInvalid
	}
	if strings.TrimSpace(r.ProductType) != ProductTypeSharing {
		return ErrUnifiedOrderProductTypeUnsupported
	}
	if strings.TrimSpace(r.OrderType) != BaoCaiTongOrderTypeSharing {
		return ErrUnifiedOrderOrderTypeUnsupported
	}
	if strings.TrimSpace(r.PayCode) != PayCodeWechatJSAPI {
		return ErrUnifiedOrderPayCodeUnsupported
	}
	if unifiedOrderPayCodeRequiresRiskInfo(r.PayCode) {
		if unifiedOrderRequiresSubMchID(environment) && strings.TrimSpace(r.SubMchID) == "" {
			return ErrUnifiedOrderSubMchIDRequired
		}
		if strings.TrimSpace(r.PayExtend.SubAppID) == "" {
			return ErrUnifiedOrderPayExtendSubAppIDRequired
		}
		if strings.TrimSpace(r.PayExtend.SubOpenID) == "" {
			return ErrUnifiedOrderPayExtendSubOpenIDRequired
		}
		if strings.TrimSpace(r.PayExtend.Body) == "" {
			return ErrUnifiedOrderPayExtendBodyRequired
		}
		if r.RiskInfo == nil || strings.TrimSpace(r.RiskInfo.ClientIP) == "" {
			return ErrUnifiedOrderRiskInfoClientIPRequired
		}
	}
	if strings.TrimSpace(r.PageURL) != "" && !isHTTPSURL(r.PageURL) {
		return ErrUnifiedOrderPageURLInvalid
	}
	if strings.TrimSpace(r.ForbidCredit) != "" && strings.TrimSpace(r.ForbidCredit) != "0" && strings.TrimSpace(r.ForbidCredit) != "1" {
		return ErrUnifiedOrderForbidCreditUnsupported
	}
	return nil
}

func (r UnifiedOrderRequest) WithoutSubMchID() UnifiedOrderRequest {
	r.SubMchID = ""
	return r
}

func unifiedOrderRequiresSubMchID(environment string) bool {
	return !strings.EqualFold(strings.TrimSpace(environment), "sandbox")
}

func isBaofuTransactionTime(value string) bool {
	if len(strings.TrimSpace(value)) != len("20060102150405") {
		return false
	}
	_, err := time.Parse("20060102150405", strings.TrimSpace(value))
	return err == nil
}

func isHTTPSURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.Scheme == "https" && strings.TrimSpace(parsed.Host) != ""
}

func unifiedOrderPayCodeRequiresRiskInfo(payCode string) bool {
	payCode = strings.ToUpper(strings.TrimSpace(payCode))
	return strings.HasPrefix(payCode, "WECHAT_") || strings.HasPrefix(payCode, "ALIPAY_")
}

type UnifiedOrderResult struct {
	AgentMerchantID  string          `json:"agentMerId,omitempty"`
	AgentTerminalID  string          `json:"agentTerId,omitempty"`
	MerchantID       string          `json:"merId,omitempty"`
	TerminalID       string          `json:"terId,omitempty"`
	OutTradeNo       string          `json:"outTradeNo,omitempty"`
	TxnState         string          `json:"txnState,omitempty"`
	TradeNo          string          `json:"tradeNo,omitempty"`
	RequestChannelNo string          `json:"reqChlNo,omitempty"`
	PayCode          string          `json:"payCode,omitempty"`
	ChannelReturn    ChannelReturn   `json:"chlRetParam,omitempty"`
	ResultCode       string          `json:"resultCode,omitempty"`
	ErrorCode        string          `json:"errCode,omitempty"`
	ErrorMessage     string          `json:"errMsg,omitempty"`
	Raw              json.RawMessage `json:"-"`
}

type ChannelReturn struct {
	PrepayID      string          `json:"prepay_id,omitempty"`
	WechatPayData json.RawMessage `json:"wc_pay_data,omitempty"`
	OrderID       string          `json:"order_id,omitempty"`
}

func (r *ChannelReturn) UnmarshalJSON(raw []byte) error {
	var aux struct {
		PrepayID      string          `json:"prepay_id,omitempty"`
		WechatPayData json.RawMessage `json:"wc_pay_data,omitempty"`
		OrderID       json.RawMessage `json:"order_id"`
	}
	if err := json.Unmarshal(raw, &aux); err != nil {
		return err
	}
	result := ChannelReturn{
		PrepayID:      strings.TrimSpace(aux.PrepayID),
		WechatPayData: aux.WechatPayData,
	}
	if len(aux.OrderID) > 0 && string(aux.OrderID) != "null" {
		orderID, err := jsonScalarToString(aux.OrderID)
		if err != nil {
			return err
		}
		result.OrderID = orderID
	}
	*r = result
	return nil
}

func jsonScalarToString(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text), nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return strings.TrimSpace(number.String()), nil
	}
	return "", errors.New("baofu json scalar must be string or number")
}

func (r UnifiedOrderResult) WechatPayData() (json.RawMessage, error) {
	if len(r.ChannelReturn.WechatPayData) == 0 {
		return nil, errors.New("baofu unified order missing wc_pay_data")
	}
	if !json.Valid(r.ChannelReturn.WechatPayData) {
		return nil, errors.New("baofu unified order wc_pay_data is not valid JSON")
	}
	return r.ChannelReturn.WechatPayData, nil
}

type PaymentQueryRequest struct {
	MerchantID string `json:"merId"`
	TerminalID string `json:"terId"`
	TradeNo    string `json:"tradeNo,omitempty"`
	OutTradeNo string `json:"outTradeNo,omitempty"`
}

func (r PaymentQueryRequest) Validate() error {
	if strings.TrimSpace(r.MerchantID) == "" {
		return ErrBaofuPaymentQueryMerchantIDRequired
	}
	if strings.TrimSpace(r.TerminalID) == "" {
		return ErrBaofuPaymentQueryTerminalIDRequired
	}
	if strings.TrimSpace(r.TradeNo) == "" && strings.TrimSpace(r.OutTradeNo) == "" {
		return ErrBaofuPaymentQueryReferenceRequired
	}
	return nil
}

type PaymentFact struct {
	OutTradeNo       string
	TradeNo          string
	TransactionState string
	SuccessAmountFen int64
	FeeAmountFen     int64
	ResultCode       string
	Raw              json.RawMessage
}

func NormalizePaymentTerminalStatus(upstreamState string) string {
	switch strings.TrimSpace(upstreamState) {
	case PaymentStateSuccess:
		return db.ExternalPaymentTerminalStatusSuccess
	case PaymentStateClosed:
		return db.ExternalPaymentTerminalStatusClosed
	case PaymentStateWaitPaying:
		return db.ExternalPaymentTerminalStatusProcessing
	case PaymentStatePayError:
		return db.ExternalPaymentTerminalStatusFailed
	case PaymentStateRefund:
		return db.ExternalPaymentTerminalStatusSuccess
	case PaymentStateAbnormal:
		return db.ExternalPaymentTerminalStatusUnknown
	default:
		return db.ExternalPaymentTerminalStatusUnknown
	}
}

type ShareAfterPayRequest struct {
	AgentMerchantID   string          `json:"agentMerId,omitempty"`
	AgentTerminalID   string          `json:"agentTerId,omitempty"`
	MerchantID        string          `json:"merId"`
	TerminalID        string          `json:"terId"`
	OriginTradeNo     string          `json:"originTradeNo,omitempty"`
	OriginOutTradeNo  string          `json:"originOutTradeNo,omitempty"`
	TxnTime           string          `json:"txnTime"`
	OutTradeNo        string          `json:"outTradeNo"`
	NotifyURL         string          `json:"notifyUrl,omitempty"`
	SharingDetails    []SharingDetail `json:"sharingDetails"`
	RequestReserved   string          `json:"reqReserved,omitempty"`
	BusinessReserved  string          `json:"bizReserved,omitempty"`
	MarketingReserved string          `json:"mktReserved,omitempty"`
}

type SharingDetail struct {
	SharingMerID     string `json:"sharingMerId"`
	SharingAmountFen int64  `json:"sharingAmt"`
}

func (r ShareAfterPayRequest) Validate() error {
	if strings.TrimSpace(r.MerchantID) == "" {
		return errors.New("baofu share merchant id is required")
	}
	if strings.TrimSpace(r.TerminalID) == "" {
		return errors.New("baofu share terminal id is required")
	}
	if strings.TrimSpace(r.OriginTradeNo) == "" && strings.TrimSpace(r.OriginOutTradeNo) == "" {
		return errors.New("baofu share original payment reference is required")
	}
	if strings.TrimSpace(r.OutTradeNo) == "" {
		return errors.New("baofu share out trade no is required")
	}
	if !isBaofuTransactionTime(r.TxnTime) {
		return ErrBaofuShareTransactionTimeInvalid
	}
	if len(r.SharingDetails) == 0 {
		return errors.New("baofu share details are required")
	}
	for _, detail := range r.SharingDetails {
		if strings.TrimSpace(detail.SharingMerID) == "" {
			return errors.New("baofu share receiver id is required")
		}
		if detail.SharingAmountFen <= 0 {
			return errors.New("baofu share amount must be positive")
		}
	}
	return nil
}

type ShareResult struct {
	MerchantID       string          `json:"merId,omitempty"`
	TerminalID       string          `json:"terId,omitempty"`
	ResultCode       string          `json:"resultCode,omitempty"`
	ErrorCode        string          `json:"errCode,omitempty"`
	ErrorMessage     string          `json:"errMsg,omitempty"`
	TradeNo          string          `json:"tradeNo,omitempty"`
	OutTradeNo       string          `json:"outTradeNo,omitempty"`
	TxnState         string          `json:"txnState,omitempty"`
	FinishTime       string          `json:"finishTime,omitempty"`
	SuccessAmountFen int64           `json:"succAmt,omitempty"`
	ClearingDate     string          `json:"clearingDate,omitempty"`
	Raw              json.RawMessage `json:"-"`
}

type ShareFact struct {
	OutTradeNo       string
	TradeNo          string
	TransactionState string
	SuccessAmountFen int64
	ResultCode       string
	Raw              json.RawMessage
}

type ShareQueryRequest struct {
	MerchantID string `json:"merId"`
	TerminalID string `json:"terId"`
	TradeNo    string `json:"tradeNo,omitempty"`
	OutTradeNo string `json:"outTradeNo,omitempty"`
}

func (r ShareQueryRequest) Validate() error {
	if strings.TrimSpace(r.MerchantID) == "" {
		return ErrBaofuShareQueryMerchantIDRequired
	}
	if strings.TrimSpace(r.TerminalID) == "" {
		return ErrBaofuShareQueryTerminalIDRequired
	}
	if strings.TrimSpace(r.TradeNo) == "" && strings.TrimSpace(r.OutTradeNo) == "" {
		return ErrBaofuShareQueryReferenceRequired
	}
	return nil
}

func NormalizeShareTerminalStatus(upstreamState string) string {
	switch strings.TrimSpace(upstreamState) {
	case ShareStateSuccess:
		return db.ExternalPaymentTerminalStatusSuccess
	case ShareStateProcessing:
		return db.ExternalPaymentTerminalStatusProcessing
	case ShareStateCanceled:
		return db.ExternalPaymentTerminalStatusFailed
	case ShareStateAbnormal:
		return db.ExternalPaymentTerminalStatusUnknown
	default:
		return db.ExternalPaymentTerminalStatusUnknown
	}
}

type RefundBeforeShareRequest struct {
	AgentMerchantID   string                `json:"agentMerId,omitempty"`
	AgentTerminalID   string                `json:"agentTerId,omitempty"`
	MerchantID        string                `json:"merId"`
	TerminalID        string                `json:"terId"`
	MerchantName      string                `json:"merchantName,omitempty"`
	OriginTradeNo     string                `json:"originTradeNo,omitempty"`
	OriginOutTradeNo  string                `json:"originOutTradeNo,omitempty"`
	OutTradeNo        string                `json:"outTradeNo"`
	NotifyURL         string                `json:"notifyUrl,omitempty"`
	RefundAmountFen   int64                 `json:"refundAmt"`
	TotalAmountFen    int64                 `json:"totalAmt"`
	TransactionTime   string                `json:"txnTime"`
	Attach            string                `json:"attach,omitempty"`
	RequestReserved   string                `json:"reqReserved,omitempty"`
	SharingRefundInfo []SharingRefundDetail `json:"sharingRefundInfo,omitempty"`
	MarketingInfo     *MarketingRefundInfo  `json:"mktRefundInfo,omitempty"`
	AdvanceAmountFen  int64                 `json:"advanceAmt,omitempty"`
	RefundReason      string                `json:"refundReason"`
}

type SharingRefundDetail struct {
	SharingMerID     string `json:"sharingMerId"`
	SharingAmountFen int64  `json:"sharingAmt"`
}

type MarketingRefundInfo struct {
	MerchantID string `json:"mktMerId"`
	AmountFen  int64  `json:"mktAmt"`
}

func (r RefundBeforeShareRequest) Validate() error {
	if strings.TrimSpace(r.MerchantID) == "" {
		return ErrBaofuRefundMerchantIDRequired
	}
	if strings.TrimSpace(r.TerminalID) == "" {
		return ErrBaofuRefundTerminalIDRequired
	}
	if strings.TrimSpace(r.OriginTradeNo) == "" && strings.TrimSpace(r.OriginOutTradeNo) == "" {
		return ErrBaofuRefundOriginalReferenceRequired
	}
	if strings.TrimSpace(r.OutTradeNo) == "" {
		return ErrBaofuRefundOutTradeNoRequired
	}
	if r.RefundAmountFen <= 0 {
		return ErrBaofuRefundAmountInvalid
	}
	if r.TotalAmountFen < r.RefundAmountFen {
		return ErrBaofuRefundTotalAmountInvalid
	}
	if !isBaofuTransactionTime(r.TransactionTime) {
		return ErrBaofuRefundTransactionTimeInvalid
	}
	if strings.TrimSpace(r.RefundReason) == "" {
		return ErrBaofuRefundReasonRequired
	}
	if len(r.SharingRefundInfo) > 0 {
		return ErrBaofuRefundSharingRefundInfoUnsupported
	}
	if r.AdvanceAmountFen > 0 {
		return ErrBaofuRefundAdvanceUnsupported
	}
	return nil
}

type RefundQueryRequest struct {
	AgentMerchantID string `json:"agentMerId,omitempty"`
	AgentTerminalID string `json:"agentTerId,omitempty"`
	MerchantID      string `json:"merId"`
	TerminalID      string `json:"terId"`
	OutTradeNo      string `json:"outTradeNo,omitempty"`
	TradeNo         string `json:"tradeNo,omitempty"`
}

func (r RefundQueryRequest) Validate() error {
	if strings.TrimSpace(r.MerchantID) == "" {
		return ErrBaofuRefundQueryMerchantIDRequired
	}
	if strings.TrimSpace(r.TerminalID) == "" {
		return ErrBaofuRefundQueryTerminalIDRequired
	}
	if strings.TrimSpace(r.OutTradeNo) == "" && strings.TrimSpace(r.TradeNo) == "" {
		return ErrBaofuRefundQueryReferenceRequired
	}
	return nil
}

type RefundResult struct {
	OriginTradeNo    string          `json:"originTradeNo,omitempty"`
	OriginOutTradeNo string          `json:"originOutTradeNo,omitempty"`
	OutTradeNo       string          `json:"outTradeNo,omitempty"`
	TradeNo          string          `json:"tradeNo,omitempty"`
	RefundAmountFen  int64           `json:"refundAmt,omitempty"`
	TotalAmountFen   int64           `json:"totalAmt,omitempty"`
	ResultCode       string          `json:"resultCode,omitempty"`
	RefundState      string          `json:"refundState,omitempty"`
	FinishTime       string          `json:"finishTime,omitempty"`
	SuccessAmountFen int64           `json:"succAmt,omitempty"`
	ErrorCode        string          `json:"errCode,omitempty"`
	ErrorMessage     string          `json:"errMsg,omitempty"`
	RequestReserved  string          `json:"reqReserved,omitempty"`
	Raw              json.RawMessage `json:"-"`
}

type RefundFact struct {
	OutTradeNo       string
	TradeNo          string
	TransactionState string
	SuccessAmountFen int64
	ResultCode       string
	Raw              json.RawMessage
}

func NormalizeRefundTerminalStatus(upstreamState string) string {
	switch strings.TrimSpace(upstreamState) {
	case RefundStateSuccess:
		return db.ExternalPaymentTerminalStatusSuccess
	case RefundStateAccepted:
		return db.ExternalPaymentTerminalStatusProcessing
	case RefundStateError:
		return db.ExternalPaymentTerminalStatusFailed
	case RefundStateAbnormal:
		return db.ExternalPaymentTerminalStatusUnknown
	default:
		return db.ExternalPaymentTerminalStatusUnknown
	}
}

type OrderCloseRequest struct {
	AgentMerchantID string `json:"agentMerId,omitempty"`
	AgentTerminalID string `json:"agentTerId,omitempty"`
	MerchantID      string `json:"merId"`
	TerminalID      string `json:"terId"`
	TradeNo         string `json:"tradeNo,omitempty"`
	OutTradeNo      string `json:"outTradeNo,omitempty"`
}

func (r OrderCloseRequest) Validate() error {
	if strings.TrimSpace(r.MerchantID) == "" {
		return ErrBaofuOrderCloseMerchantIDRequired
	}
	if strings.TrimSpace(r.TerminalID) == "" {
		return ErrBaofuOrderCloseTerminalIDRequired
	}
	if strings.TrimSpace(r.TradeNo) == "" && strings.TrimSpace(r.OutTradeNo) == "" {
		return ErrBaofuOrderCloseReferenceRequired
	}
	return nil
}

type OrderCloseResult struct {
	AgentMerchantID string          `json:"agentMerId,omitempty"`
	AgentTerminalID string          `json:"agentTerId,omitempty"`
	MerchantID      string          `json:"merId,omitempty"`
	TerminalID      string          `json:"terId,omitempty"`
	TradeNo         string          `json:"tradeNo,omitempty"`
	OutTradeNo      string          `json:"outTradeNo,omitempty"`
	ResultCode      string          `json:"resultCode,omitempty"`
	ErrorCode       string          `json:"errCode,omitempty"`
	ErrorMessage    string          `json:"errMsg,omitempty"`
	Raw             json.RawMessage `json:"-"`
}
