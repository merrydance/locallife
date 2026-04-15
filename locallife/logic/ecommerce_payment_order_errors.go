package logic

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/merrydance/locallife/wechat"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

func mapPartnerJSAPIOrderCreateError(err error) error {
	if err == nil {
		return nil
	}
	if mapped := mapEcommercePaymentClientPreparationError(err); mapped != nil {
		return mapped
	}
	if mapped := mapWechatPaymentCreateError(err); mapped != nil {
		return mapped
	}
	if strings.Contains(err.Error(), "empty prepay id") {
		return NewRequestError(http.StatusBadGateway, errors.New("微信支付未返回可用预支付会话，请返回订单页重新发起支付"))
	}
	return fmt.Errorf("create partner jsapi order: %w", err)
}

func mapCombineOrderCreateError(err error) error {
	if err == nil {
		return nil
	}
	if mapped := mapEcommercePaymentClientPreparationError(err); mapped != nil {
		return mapped
	}
	if mapped := mapWechatPaymentCreateError(err); mapped != nil {
		return mapped
	}
	if strings.Contains(err.Error(), "empty prepay id") {
		return NewRequestError(http.StatusBadGateway, errors.New("微信支付未返回可用预支付会话，请返回订单页重新发起支付"))
	}
	return fmt.Errorf("create combine order: %w", err)
}

func mapCombineOrderQueryError(err error) error {
	if err == nil {
		return nil
	}
	var validationErr *wechat.CombineOrderQueryValidationError
	if errors.As(err, &validationErr) {
		return NewRequestError(http.StatusBadGateway, errors.New("合单支付查询参数不完整，请返回支付结果页重新进入"))
	}
	var contractErr *wechat.CombineOrderQueryContractError
	if errors.As(err, &contractErr) {
		return NewRequestError(http.StatusBadGateway, errors.New("微信支付状态返回异常，请不要重复支付，返回订单页后重新查询"))
	}
	if mapped := mapWechatPaymentQueryError(err); mapped != nil {
		return mapped
	}
	return fmt.Errorf("query combine order: %w", err)
}

func mapPartnerOrderQueryError(err error) error {
	if err == nil {
		return nil
	}

	var validationErr *wechat.PartnerOrderQueryValidationError
	if errors.As(err, &validationErr) {
		return NewRequestError(http.StatusBadGateway, errors.New("支付查询参数不完整，请返回订单页重新进入"))
	}

	var contractErr *wechat.PartnerOrderQueryContractError
	if errors.As(err, &contractErr) {
		return NewRequestError(http.StatusBadGateway, errors.New("微信支付状态返回异常，请不要重复支付，返回订单页后重新查询"))
	}

	if mapped := mapWechatPaymentQueryError(err); mapped != nil {
		return mapped
	}

	return fmt.Errorf("query partner order: %w", err)
}

func mapPartnerOrderCloseError(err error) error {
	if err == nil {
		return nil
	}
	if mapped := mapEcommercePaymentClientPreparationError(err); mapped != nil {
		return mapped
	}
	if mapped := mapWechatPaymentCloseError(err); mapped != nil {
		return mapped
	}
	return fmt.Errorf("close partner order: %w", err)
}

func mapCombineOrderCloseError(err error) error {
	if err == nil {
		return nil
	}
	if mapped := mapEcommercePaymentClientPreparationError(err); mapped != nil {
		return mapped
	}
	if mapped := mapWechatPaymentCloseError(err); mapped != nil {
		return mapped
	}
	return fmt.Errorf("close combine order: %w", err)
}

func mapWechatPaymentCreateError(err error) error {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return nil
	}

	switch {
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOrderClosed):
		return NewRequestError(http.StatusConflict, errors.New("支付订单已过期或已关闭，请重新发起支付"))
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOutTradeNoUsed):
		return NewRequestError(http.StatusConflict, errors.New("支付订单正在处理中，请在支付结果页刷新确认后再决定是否重试"))
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeAccountError):
		return NewRequestError(http.StatusConflict, errors.New("当前微信支付账户暂时无法完成支付，请更换账户后重试"))
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeTradeError):
		return NewRequestError(http.StatusConflict, errors.New("微信支付下单失败，请返回订单页重新发起，必要时更换支付方式"))
	case wechaterrorcodes.OrderingInfrastructureCodes.Has(wxErr.Code):
		return NewRequestError(http.StatusServiceUnavailable, errors.New("微信支付服务响应异常，请不要重复扣款，返回订单页后重新查询"))
	case wechaterrorcodes.OrderingConfigurationCodes.Has(wxErr.Code):
		return NewRequestError(http.StatusServiceUnavailable, errors.New("商户支付配置未完成，请联系平台处理"))
	default:
		return NewRequestError(http.StatusServiceUnavailable, errors.New("微信支付请求失败，请返回订单页重新查询支付状态"))
	}
}

func mapWechatPaymentQueryError(err error) error {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return nil
	}

	switch {
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOrderNotExist):
		return NewRequestError(http.StatusBadGateway, errors.New("微信侧暂未确认该支付单，请保留当前订单并稍后刷新结果"))
	case wechaterrorcodes.OrderingInfrastructureCodes.Has(wxErr.Code):
		return NewRequestError(http.StatusServiceUnavailable, errors.New("微信支付状态查询异常，请不要重复支付，返回订单页后重新查询"))
	case wechaterrorcodes.OrderingConfigurationCodes.Has(wxErr.Code):
		return NewRequestError(http.StatusServiceUnavailable, errors.New("商户支付配置未完成，当前无法确认支付状态，请联系平台处理"))
	default:
		return NewRequestError(http.StatusServiceUnavailable, errors.New("支付状态查询失败，请返回订单页后重新查询"))
	}
}

func mapWechatPaymentCloseError(err error) error {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return nil
	}

	switch {
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOrderClosed):
		return NewRequestError(http.StatusConflict, errors.New("支付订单已关闭"))
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeUserPaying):
		return NewRequestError(http.StatusConflict, errors.New("支付处理中，请先刷新支付结果确认后再决定是否关闭"))
	case wechaterrorcodes.OrderingCodeEquals(wxErr.Code, wechaterrorcodes.OrderingCodeOrderNotExist):
		return NewRequestError(http.StatusBadGateway, errors.New("微信侧暂未确认该支付单的关闭状态，请返回订单页重新查询"))
	case wechaterrorcodes.OrderingInfrastructureCodes.Has(wxErr.Code):
		return NewRequestError(http.StatusServiceUnavailable, errors.New("微信支付关闭请求异常，请返回订单页确认最新状态"))
	case wechaterrorcodes.OrderingConfigurationCodes.Has(wxErr.Code):
		return NewRequestError(http.StatusServiceUnavailable, errors.New("商户支付配置未完成，当前无法关闭支付单，请联系平台处理"))
	default:
		return NewRequestError(http.StatusServiceUnavailable, errors.New("支付关闭失败，请返回订单页确认最新状态"))
	}
}

func mapEcommercePaymentClientPreparationError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	if !containsAny(message, []string{
		"request is nil",
		"not supported in the single-appid project flow",
		"notify_url is required",
		"sub_mchid is required",
		"transaction_id is required",
		"out_trade_no is required",
		"description and out_trade_no are required",
		"total amount must be positive",
		"sp_openid or sub_openid is required",
		"sub_appid is required",
		"payer_client_ip is required",
		"combine_out_trade_no is required",
		"sub_orders is required",
		"attach is required",
		"amount.total_amount must be positive",
		"openid or sub_openid is required",
	}) {
		return nil
	}
	return NewRequestError(http.StatusBadGateway, errors.New("支付请求参数准备不完整，请返回订单页重新发起支付"))
}

func hasWechatPaymentCode(err error, codes ...string) bool {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return false
	}
	return wechaterrorcodes.OrderingCodeIn(wxErr.Code, codes...)
}
