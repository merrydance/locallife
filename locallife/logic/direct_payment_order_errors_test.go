package logic

import (
	"net/http"
	"testing"

	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/stretchr/testify/require"
)

func TestMapDirectJSAPIOrderCreateErrorPreservesWechatCause(t *testing.T) {
	wxErr := &wechat.WechatPayError{StatusCode: http.StatusForbidden, Code: "OUT_TRADE_NO_USED", Message: "订单号重复"}
	err := mapDirectJSAPIOrderCreateError(wxErr)
	reqErr := assertRequestError(t, err)

	require.Equal(t, http.StatusConflict, reqErr.Status)
	require.EqualError(t, reqErr.Err, "支付订单正在处理中，请在支付结果页刷新确认后再决定是否重试")
	require.Same(t, wxErr, LoggableError(err))
}

func TestMapDirectJSAPIOrderCreateErrorPreservesValidationCause(t *testing.T) {
	validationErr := &wechatcontracts.DirectJSAPIOrderRequestValidationError{Message: "notify_url is required"}
	err := mapDirectJSAPIOrderCreateError(validationErr)
	reqErr := assertRequestError(t, err)

	require.Equal(t, http.StatusBadGateway, reqErr.Status)
	require.EqualError(t, reqErr.Err, "支付请求参数准备不完整，请返回订单页重新发起支付")
	require.Same(t, validationErr, LoggableError(err))
}
