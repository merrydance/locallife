package logic

import (
	"errors"
	"net/http"
	"strings"

	"github.com/merrydance/locallife/baofu"
	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

func isDirectRefundAlreadyFullyRefundedError(err error) bool {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return false
	}

	if wechaterrorcodes.CanonicalDirectPaymentCode(wxErr.Code) != wechaterrorcodes.DirectPaymentCodeInvalidRequest {
		return false
	}

	message := strings.TrimSpace(wxErr.Message + " " + wxErr.Detail)
	return strings.Contains(message, "订单已全额退款") || strings.Contains(message, "已全额退款")
}

func mapDirectRefundCreateError(err error) error {
	if err == nil {
		return nil
	}

	var validationErr *wechatcontracts.RefundValidationError
	if errors.As(err, &validationErr) {
		return NewRequestErrorWithCause(http.StatusBadRequest, errors.New("退款请求参数不符合微信支付要求，请检查退款原因和金额后重试"), err)
	}

	var contractErr *wechatcontracts.RefundContractError
	if errors.As(err, &contractErr) {
		return NewRequestErrorWithCause(http.StatusBadGateway, errors.New("微信退款受理响应异常，请不要重复退款，稍后刷新退款状态"), err)
	}

	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return err
	}

	switch {
	case wechaterrorcodes.DirectRefundCreateDocumentedCodes.Has(wxErr.Code) && wechaterrorcodes.CanonicalDirectPaymentCode(wxErr.Code) == wechaterrorcodes.DirectPaymentCodeNotEnough:
		return NewRequestErrorWithCause(http.StatusConflict, errors.New("商户退款余额不足，暂时无法原路退款，请联系平台处理"), err)
	case wechaterrorcodes.DirectRefundCreateDocumentedCodes.Has(wxErr.Code) && wechaterrorcodes.CanonicalDirectPaymentCode(wxErr.Code) == wechaterrorcodes.DirectPaymentCodeUserAccountAbnormal:
		return NewRequestErrorWithCause(http.StatusConflict, errors.New("当前微信支付账户状态异常，暂时无法完成退款，请稍后重试或联系平台处理"), err)
	case wechaterrorcodes.DirectRefundCreateDocumentedCodes.Has(wxErr.Code) && wechaterrorcodes.CanonicalDirectPaymentCode(wxErr.Code) == wechaterrorcodes.DirectPaymentCodeResourceNotExists:
		return NewRequestErrorWithCause(http.StatusConflict, errors.New("微信侧未找到可退款的原支付单，请刷新订单状态后确认是否仍需退款"), err)
	case wechaterrorcodes.DirectRefundCreateDocumentedCodes.Has(wxErr.Code) && wechaterrorcodes.CanonicalDirectPaymentCode(wxErr.Code) == wechaterrorcodes.DirectPaymentCodeFrequencyLimited:
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("微信退款请求过于频繁，请稍后刷新退款状态后再试"), err)
	case wechaterrorcodes.DirectPaymentCommonCodes.Has(wxErr.Code):
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("微信退款服务暂时不可用，请稍后刷新退款状态"), err)
	case wechaterrorcodes.CanonicalDirectPaymentCode(wxErr.Code) == wechaterrorcodes.DirectPaymentCodeMchNotExists,
		wechaterrorcodes.CanonicalDirectPaymentCode(wxErr.Code) == wechaterrorcodes.DirectPaymentCodeNoAuth:
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("商户退款配置未完成，当前无法发起退款，请联系平台处理"), err)
	default:
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("微信退款请求失败，请稍后刷新退款状态"), err)
	}
}

type BaofuRefundCreateFailureDisposition struct {
	CommandStatus  string
	MarkFailed     bool
	MarkProcessing bool
	RetryCreate    bool
}

func ClassifyBaofuRefundCreateFailure(err error) BaofuRefundCreateFailureDisposition {
	disposition := BaofuRefundCreateFailureDisposition{
		CommandStatus: db.ExternalPaymentCommandStatusRejected,
		MarkFailed:    true,
	}
	if err == nil {
		return disposition
	}

	var providerErr *baofu.ProviderError
	if !errors.As(err, &providerErr) {
		return disposition
	}

	code := strings.ToUpper(strings.TrimSpace(providerErr.UpstreamCode))
	if baofuRefundCreateFailureShouldQuery(code) {
		return BaofuRefundCreateFailureDisposition{
			CommandStatus:  db.ExternalPaymentCommandStatusUnknown,
			MarkProcessing: true,
		}
	}

	classified := baofu.ClassifyBaofuError(providerErr.UpstreamCode, providerErr.UpstreamMessage)
	if classified.Category == baofu.BaofuErrorCategoryRetryable || baofuRefundCreateRequestFailureIsRetryable(providerErr) {
		return BaofuRefundCreateFailureDisposition{
			CommandStatus: db.ExternalPaymentCommandStatusUnknown,
			RetryCreate:   true,
		}
	}

	return disposition
}

func BaofuRefundCreateProviderResultError(refundResp *aggregatecontracts.RefundResult) error {
	if refundResp == nil {
		return errors.New("baofu refund returned empty result")
	}
	upstreamCode := strings.TrimSpace(refundResp.ErrorCode)
	if upstreamCode == "" {
		upstreamCode = strings.TrimSpace(refundResp.RefundState)
	}
	if upstreamCode == "" {
		upstreamCode = strings.TrimSpace(refundResp.ResultCode)
	}
	return &baofu.ProviderError{
		Operation:       "order_refund",
		Capability:      db.ExternalPaymentCapabilityBaofuRefund,
		UpstreamCode:    upstreamCode,
		UpstreamMessage: strings.TrimSpace(refundResp.ErrorMessage),
		Frontend:        baofu.ClassifyBaofuError(upstreamCode, refundResp.ErrorMessage).FrontendGuidance(),
	}
}

func baofuRefundCreateFailureShouldQuery(code string) bool {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "ORDER_EXIST", "REPEATED_REQUEST", "BF0013", "TRADE_UNCONFIRMED", "PROCESSING", "ABNORMAL":
		return true
	default:
		return false
	}
}

func baofuRefundCreateRequestFailureIsRetryable(providerErr *baofu.ProviderError) bool {
	if providerErr == nil {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(providerErr.UpstreamCode)) {
	case "REQUEST_FAILED":
		return true
	case "HTTP_STATUS":
		return providerErr.StatusCode == 0 || providerErr.StatusCode >= http.StatusInternalServerError
	default:
		return false
	}
}
