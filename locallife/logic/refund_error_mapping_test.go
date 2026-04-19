package logic

import (
	"net/http"
	"testing"

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
