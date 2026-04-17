package logic

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

func MapDirectJSAPIOrderCreateError(err error) error {
	return mapDirectJSAPIOrderCreateError(err)
}

func mapDirectJSAPIOrderCreateError(err error) error {
	if err == nil {
		return nil
	}

	var validationErr *wechatcontracts.DirectJSAPIOrderRequestValidationError
	if errors.As(err, &validationErr) {
		return NewRequestErrorWithCause(http.StatusBadGateway, errors.New("支付请求参数准备不完整，请返回订单页重新发起支付"), err)
	}

	if mapped := mapDirectWechatPaymentCreateError(err); mapped != nil {
		return mapped
	}

	if strings.Contains(err.Error(), "empty prepay id") {
		return NewRequestErrorWithCause(http.StatusBadGateway, errors.New("微信支付未返回可用预支付会话，请返回订单页重新发起支付"), err)
	}

	return fmt.Errorf("create direct jsapi order: %w", err)
}

func mapDirectWechatPaymentCreateError(err error) error {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return nil
	}

	switch code := wechaterrorcodes.CanonicalDirectPaymentCode(wxErr.Code); {
	case code == wechaterrorcodes.DirectPaymentCodeOutTradeNoUsed:
		return NewRequestErrorWithCause(http.StatusConflict, errors.New("支付订单正在处理中，请在支付结果页刷新确认后再决定是否重试"), err)
	case code == wechaterrorcodes.DirectPaymentCodeParamError || code == wechaterrorcodes.DirectPaymentCodeInvalidRequest:
		return NewRequestErrorWithCause(http.StatusBadGateway, errors.New("支付请求参数准备不完整，请返回订单页重新发起支付"), err)
	case code == wechaterrorcodes.DirectPaymentCodeAppIDMchIDNotMatch,
		code == wechaterrorcodes.DirectPaymentCodeMchNotExists,
		code == wechaterrorcodes.DirectPaymentCodeNoAuth,
		code == wechaterrorcodes.DirectPaymentCodeSignError:
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("商户支付配置未完成，请联系平台处理"), err)
	case code == wechaterrorcodes.DirectPaymentCodeFrequencyLimited,
		code == wechaterrorcodes.DirectPaymentCodeSystemError:
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("微信支付服务响应异常，请不要重复扣款，返回订单页后重新查询"), err)
	default:
		return NewRequestErrorWithCause(http.StatusServiceUnavailable, errors.New("微信支付请求失败，请返回订单页重新查询支付状态"), err)
	}
}
