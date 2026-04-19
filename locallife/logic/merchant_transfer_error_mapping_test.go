package logic

import (
	"errors"
	"net/http"
	"testing"

	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/stretchr/testify/require"
)

func TestMapClaimPayoutTransferExecutionError_LocalStateErrors(t *testing.T) {
	testCases := []struct {
		name           string
		err            error
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "TransferClientNotConfigured",
			err:            errors.New("transfer client is not configured for claim payout"),
			expectedStatus: http.StatusServiceUnavailable,
			expectedMsg:    "企业赔付服务暂不可用，请联系平台处理",
		},
		{
			name:           "MissingOpenID",
			err:            errors.New("payout user 12 missing wechat openid"),
			expectedStatus: http.StatusConflict,
			expectedMsg:    "赔付用户实名信息不完整，当前无法发起企业赔付，请联系平台处理",
		},
		{
			name:           "InvalidActionDetail",
			err:            errors.New("invalid behavior action detail for claim payout"),
			expectedStatus: http.StatusConflict,
			expectedMsg:    "赔付动作数据不完整，当前无法继续企业赔付，请联系平台处理",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mapped := MapClaimPayoutTransferExecutionError(tc.err)
			reqErr := assertRequestError(t, mapped)
			require.Equal(t, tc.expectedStatus, reqErr.Status)
			require.EqualError(t, reqErr.Err, tc.expectedMsg)
			require.Same(t, tc.err, LoggableError(mapped))
		})
	}
}

func TestMapClaimPayoutTransferExecutionError_ContractErrors(t *testing.T) {
	testCases := []struct {
		name           string
		err            error
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "CreateValidation",
			err:            &wechatcontracts.DirectMerchantTransferCreateRequestValidationError{Message: "openid is required"},
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "企业赔付参数准备不完整，请检查赔付金额和收款用户信息后重试",
		},
		{
			name:           "QueryValidation",
			err:            &wechatcontracts.DirectMerchantTransferQueryValidationError{Message: "out_bill_no is required"},
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "企业赔付状态查询参数不完整，请稍后刷新赔付状态",
		},
		{
			name:           "CreateContractDrift",
			err:            &wechatcontracts.DirectMerchantTransferContractError{Message: "create direct merchant transfer: wechat response missing transfer_bill_no"},
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "微信商户转账受理响应异常，请不要重复赔付，稍后刷新赔付状态",
		},
		{
			name:           "QueryContractDrift",
			err:            &wechatcontracts.DirectMerchantTransferContractError{Message: "query direct merchant transfer by out_bill_no: wechat response missing state"},
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "微信商户转账状态返回异常，请不要重复赔付，稍后刷新赔付状态",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mapped := MapClaimPayoutTransferExecutionError(tc.err)
			reqErr := assertRequestError(t, mapped)
			require.Equal(t, tc.expectedStatus, reqErr.Status)
			require.EqualError(t, reqErr.Err, tc.expectedMsg)
			require.Same(t, tc.err, LoggableError(mapped))

			var unwrapped error
			require.ErrorAs(t, mapped, &unwrapped)
		})
	}
}

func TestMapClaimPayoutTransferExecutionError_WechatPayCodes(t *testing.T) {
	testCases := []struct {
		name           string
		err            *wechat.WechatPayError
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "NoAuth",
			err:            &wechat.WechatPayError{StatusCode: http.StatusForbidden, Code: "NO_AUTH", Message: "权限不足"},
			expectedStatus: http.StatusServiceUnavailable,
			expectedMsg:    "商户转账配置未完成，当前无法发起企业赔付，请联系平台处理",
		},
		{
			name:           "SystemError",
			err:            &wechat.WechatPayError{StatusCode: http.StatusInternalServerError, Code: "SYSTEM_ERROR", Message: "系统错误"},
			expectedStatus: http.StatusServiceUnavailable,
			expectedMsg:    "微信商户转账服务暂时不可用，请稍后刷新赔付状态",
		},
		{
			name:           "FrequencyLimited",
			err:            &wechat.WechatPayError{StatusCode: http.StatusTooManyRequests, Code: "FREQUENCY_LIMITED", Message: "频率限制"},
			expectedStatus: http.StatusServiceUnavailable,
			expectedMsg:    "微信商户转账请求过于频繁，请稍后刷新赔付状态后再试",
		},
		{
			name:           "AlreadyExists",
			err:            &wechat.WechatPayError{StatusCode: http.StatusBadRequest, Code: "ALREADY_EXISTS", Message: "单号重复"},
			expectedStatus: http.StatusConflict,
			expectedMsg:    "该企业赔付请求已受理，请稍后刷新赔付状态，不要重复操作",
		},
		{
			name:           "NotEnough",
			err:            &wechat.WechatPayError{StatusCode: http.StatusForbidden, Code: "NOT_ENOUGH", Message: "余额不足"},
			expectedStatus: http.StatusConflict,
			expectedMsg:    "商户转账余额不足，当前无法完成企业赔付，请联系平台处理",
		},
		{
			name:           "NotFoundByCode",
			err:            &wechat.WechatPayError{StatusCode: http.StatusNotFound, Code: "NOT_FOUND", Message: "单据不存在"},
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "微信侧暂未确认该赔付单，请稍后刷新赔付状态",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mapped := MapClaimPayoutTransferExecutionError(tc.err)
			reqErr := assertRequestError(t, mapped)
			require.Equal(t, tc.expectedStatus, reqErr.Status)
			require.EqualError(t, reqErr.Err, tc.expectedMsg)
			require.Same(t, tc.err, LoggableError(mapped))

			var unwrapped *wechat.WechatPayError
			require.ErrorAs(t, mapped, &unwrapped)
			require.Same(t, tc.err, unwrapped)
		})
	}
}

func TestMapClaimPayoutTransferExecutionError_UnmappedErrorFallsThrough(t *testing.T) {
	original := errors.New("database timeout")
	mapped := MapClaimPayoutTransferExecutionError(original)
	require.Same(t, original, mapped)
}
