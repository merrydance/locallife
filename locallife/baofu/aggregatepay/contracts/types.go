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
		if strings.TrimSpace(r.SubMchID) == "" {
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
