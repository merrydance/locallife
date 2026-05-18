package logic

import (
	"errors"
	"net/http"
	"testing"

	"github.com/merrydance/locallife/baofu"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/stretchr/testify/require"
)

func TestMapDirectRefundCreateErrorPreservesWechatCause(t *testing.T) {
	wxErr := &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "NOT_ENOUGH", Message: "余额不足"}
	err := mapDirectRefundCreateError(wxErr)
	reqErr := assertRequestError(t, err)

	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.EqualError(t, reqErr.Err, "商户退款余额不足，暂时无法原路退款，请联系平台处理")
	require.Same(t, wxErr, LoggableError(err))

	var unwrapped *wechat.WechatPayError
	require.ErrorAs(t, err, &unwrapped)
	require.Same(t, wxErr, unwrapped)
}

func TestMapEcommerceAbnormalRefundErrorPreservesValidationCause(t *testing.T) {
	validationErr := &wechatcontracts.RefundValidationError{Message: "bank_account is required"}
	err := mapEcommerceAbnormalRefundError(validationErr)
	reqErr := assertRequestError(t, err)

	require.Equal(t, http.StatusBadRequest, reqErr.Status)
	require.EqualError(t, reqErr.Err, "异常退款参数不符合微信要求，请检查银行卡信息和退款状态后重试")
	require.Same(t, validationErr, LoggableError(err))

	var unwrapped *wechatcontracts.RefundValidationError
	require.ErrorAs(t, err, &unwrapped)
	require.Same(t, validationErr, unwrapped)
}

func TestMapOrdinaryRefundCreateErrorMapsMissingClientToActionableRequestError(t *testing.T) {
	configErr := errors.New("ordinary service provider client not configured")
	err := mapOrdinaryServiceProviderRefundCreateError(configErr)
	reqErr := assertRequestError(t, err)

	require.Equal(t, http.StatusServiceUnavailable, reqErr.Status)
	require.EqualError(t, reqErr.Err, "微信服务商退款配置未完成，当前无法发起退款，请联系平台处理")
	require.Same(t, configErr, LoggableError(err))
}

func TestMapBaofuRefundCreateErrorUsesSafeChineseProviderMessage(t *testing.T) {
	providerErr := &baofu.ProviderError{
		Operation:       "order_refund",
		UpstreamCode:    "MERCHANT_NOT_REPORTED",
		UpstreamMessage: "raw upstream merchant id detail",
	}

	err := mapBaofuRefundCreateError(providerErr)
	reqErr := assertRequestError(t, err)

	require.Equal(t, http.StatusServiceUnavailable, reqErr.Status)
	require.EqualError(t, reqErr.Err, "商户微信支付通道待开通，请联系平台处理")
	require.NotContains(t, reqErr.Err.Error(), "raw upstream")
	require.Same(t, providerErr, LoggableError(err))
}

func TestMapBaofuPaymentCreateErrorUsesSafeChineseProviderMessage(t *testing.T) {
	providerErr := &baofu.ProviderError{
		Operation:       "unified_order",
		UpstreamCode:    "SYSTEM_BUSY",
		UpstreamMessage: "raw upstream payment detail",
	}

	err := mapBaofuPaymentCreateError(providerErr)
	reqErr := assertRequestError(t, err)

	require.Equal(t, http.StatusServiceUnavailable, reqErr.Status)
	require.EqualError(t, reqErr.Err, "支付通道处理中，请稍后重试")
	require.NotContains(t, reqErr.Err.Error(), "raw upstream")
	require.Same(t, providerErr, LoggableError(err))
}

func TestMapWechatPaymentCreateErrorPreservesWechatCause(t *testing.T) {
	wxErr := &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "ORDER_CLOSED", Message: "订单已关闭"}
	err := mapWechatPaymentCreateError(wxErr)
	reqErr := assertRequestError(t, err)

	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.EqualError(t, reqErr.Err, "支付订单已过期或已关闭，请重新发起支付")
	require.Same(t, wxErr, LoggableError(err))

	var unwrapped *wechat.WechatPayError
	require.ErrorAs(t, err, &unwrapped)
	require.Same(t, wxErr, unwrapped)
}
