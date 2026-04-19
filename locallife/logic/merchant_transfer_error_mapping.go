package logic

import (
	"errors"
	"net/http"
	"strings"

	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

func MapClaimPayoutTransferExecutionError(err error) error {
	if err == nil {
		return nil
	}

	if mapped := mapClaimPayoutLocalStateError(err); mapped != nil {
		return mapped
	}

	if mapped := mapMerchantTransferExecutionError(err); mapped != nil {
		return mapped
	}

	return err
}

func mapClaimPayoutLocalStateError(err error) error {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "transfer client is not configured"):
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("企业赔付服务暂不可用，请联系平台处理"), err)
	case strings.Contains(msg, "missing wechat openid"), strings.Contains(msg, "missing full name"):
		return NewRequestErrorWithCause(http.StatusConflict, errors.New("赔付用户实名信息不完整，当前无法发起企业赔付，请联系平台处理"), err)
	case strings.Contains(msg, "invalid behavior action detail for claim payout"):
		return NewRequestErrorWithCause(http.StatusConflict, errors.New("赔付动作数据不完整，当前无法继续企业赔付，请联系平台处理"), err)
	default:
		return nil
	}
}

func mapMerchantTransferExecutionError(err error) error {
	var createValidationErr *wechatcontracts.DirectMerchantTransferCreateRequestValidationError
	if errors.As(err, &createValidationErr) {
		return NewRequestErrorWithCause(http.StatusBadGateway, errors.New("企业赔付参数准备不完整，请检查赔付金额和收款用户信息后重试"), err)
	}

	var queryValidationErr *wechatcontracts.DirectMerchantTransferQueryValidationError
	if errors.As(err, &queryValidationErr) {
		return NewRequestErrorWithCause(http.StatusBadGateway, errors.New("企业赔付状态查询参数不完整，请稍后刷新赔付状态"), err)
	}

	var contractErr *wechatcontracts.DirectMerchantTransferContractError
	if errors.As(err, &contractErr) {
		if strings.Contains(strings.ToLower(contractErr.Error()), "query direct merchant transfer") {
			return NewRequestErrorWithCause(http.StatusBadGateway, errors.New("微信商户转账状态返回异常，请不要重复赔付，稍后刷新赔付状态"), err)
		}
		return NewRequestErrorWithCause(http.StatusBadGateway, errors.New("微信商户转账受理响应异常，请不要重复赔付，稍后刷新赔付状态"), err)
	}

	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return nil
	}

	switch code := wechaterrorcodes.CanonicalMerchantTransferCode(wxErr.Code); {
	case code == wechaterrorcodes.MerchantTransferCodeParamError,
		code == wechaterrorcodes.MerchantTransferCodeInvalidRequest:
		return NewRequestErrorWithCause(http.StatusBadGateway, errors.New("企业赔付参数不符合微信要求，请检查赔付金额和收款用户信息后重试"), err)
	case code == wechaterrorcodes.MerchantTransferCodeAlreadyExists:
		return NewRequestErrorWithCause(http.StatusConflict, errors.New("该企业赔付请求已受理，请稍后刷新赔付状态，不要重复操作"), err)
	case code == wechaterrorcodes.MerchantTransferCodeNotEnough:
		return NewRequestErrorWithCause(http.StatusConflict, errors.New("商户转账余额不足，当前无法完成企业赔付，请联系平台处理"), err)
	case code == wechaterrorcodes.MerchantTransferCodeNotFound:
		return NewRequestErrorWithCause(http.StatusBadGateway, errors.New("微信侧暂未确认该赔付单，请稍后刷新赔付状态"), err)
	case code == wechaterrorcodes.MerchantTransferCodeNoAuth,
		code == wechaterrorcodes.MerchantTransferCodeSignError:
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("商户转账配置未完成，当前无法发起企业赔付，请联系平台处理"), err)
	case code == wechaterrorcodes.MerchantTransferCodeFrequencyLimited,
		code == wechaterrorcodes.MerchantTransferCodeFrequencyLimit,
		code == wechaterrorcodes.MerchantTransferCodeFrequencyLimitExceed,
		code == wechaterrorcodes.MerchantTransferCodeRateLimitExceeded:
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("微信商户转账请求过于频繁，请稍后刷新赔付状态后再试"), err)
	case code == wechaterrorcodes.MerchantTransferCodeSystemError:
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("微信商户转账服务暂时不可用，请稍后刷新赔付状态"), err)
	default:
		if wxErr.StatusCode == http.StatusNotFound {
			return NewRequestErrorWithCause(http.StatusBadGateway, errors.New("微信侧暂未确认该赔付单，请稍后刷新赔付状态"), err)
		}
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("微信商户转账请求失败，请稍后刷新赔付状态"), err)
	}
}
