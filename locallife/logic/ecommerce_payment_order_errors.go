package logic

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/merrydance/locallife/wechat"
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
		return NewRequestError(http.StatusServiceUnavailable, errors.New("wechat payment did not return a valid prepay session, please retry later"))
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
		return NewRequestError(http.StatusServiceUnavailable, errors.New("wechat payment did not return a valid prepay session, please retry later"))
	}
	return fmt.Errorf("create combine order: %w", err)
}

func mapCombineOrderQueryError(err error) error {
	if err == nil {
		return nil
	}
	if mapped := mapWechatPaymentQueryError(err); mapped != nil {
		return mapped
	}
	return fmt.Errorf("query combine order: %w", err)
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

	switch normalizeWechatPaymentCode(wxErr.Code) {
	case "ORDER_CLOSED":
		return NewRequestError(http.StatusConflict, errors.New("payment order has expired or been closed, please recreate the payment"))
	case "OUT_TRADE_NO_USED":
		return NewRequestError(http.StatusConflict, errors.New("payment order is already being processed, please retry"))
	case "ACCOUNTERROR", "ACCOUNT_ERROR":
		return NewRequestError(http.StatusConflict, errors.New("current wechat account cannot complete payment, please switch account and retry"))
	case "TRADE_ERROR":
		return NewRequestError(http.StatusConflict, errors.New("wechat could not create the payment order, please retry or use another payment method"))
	case "RULELIMIT", "RULE_LIMIT", "FREQUENCY_LIMITED", "RATELIMIT_EXCEEDED", "SYSTEMERROR", "SYSTEM_ERROR", "BANKERROR", "BANK_ERROR":
		return NewRequestError(http.StatusServiceUnavailable, errors.New("payment service is temporarily unavailable, please retry later"))
	case "PARAM_ERROR", "INVALID_REQUEST", "APPID_MCHID_NOT_MATCH", "OPENID_MISMATCH", "MCH_NOT_EXISTS", "NOAUTH", "NO_AUTH", "SIGN_ERROR":
		return NewRequestError(http.StatusServiceUnavailable, errors.New("payment service configuration is temporarily unavailable, please retry later"))
	default:
		return NewRequestError(http.StatusServiceUnavailable, errors.New("wechat payment request failed, please retry later"))
	}
}

func mapWechatPaymentQueryError(err error) error {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return nil
	}

	switch normalizeWechatPaymentCode(wxErr.Code) {
	case "ORDER_NOT_EXIST", "ORDERNOTEXIST":
		return NewRequestError(http.StatusServiceUnavailable, errors.New("payment status is still being synchronized, please retry later"))
	case "RULELIMIT", "RULE_LIMIT", "FREQUENCY_LIMITED", "RATELIMIT_EXCEEDED", "SYSTEMERROR", "SYSTEM_ERROR", "BANKERROR", "BANK_ERROR":
		return NewRequestError(http.StatusServiceUnavailable, errors.New("payment status query is temporarily unavailable, please retry later"))
	case "PARAM_ERROR", "INVALID_REQUEST", "APPID_MCHID_NOT_MATCH", "OPENID_MISMATCH", "MCH_NOT_EXISTS", "NOAUTH", "NO_AUTH", "SIGN_ERROR":
		return NewRequestError(http.StatusServiceUnavailable, errors.New("payment status query configuration is temporarily unavailable, please retry later"))
	default:
		return NewRequestError(http.StatusServiceUnavailable, errors.New("payment status query failed, please retry later"))
	}
}

func mapWechatPaymentCloseError(err error) error {
	var wxErr *wechat.WechatPayError
	if !errors.As(err, &wxErr) {
		return nil
	}

	switch normalizeWechatPaymentCode(wxErr.Code) {
	case "ORDER_CLOSED":
		return NewRequestError(http.StatusConflict, errors.New("payment order is already closed"))
	case "USERPAYING":
		return NewRequestError(http.StatusConflict, errors.New("payment is being processed, please retry after confirming the latest status"))
	case "ORDER_NOT_EXIST", "ORDERNOTEXIST":
		return NewRequestError(http.StatusServiceUnavailable, errors.New("payment close status is still being synchronized, please retry later"))
	case "RULELIMIT", "RULE_LIMIT", "FREQUENCY_LIMITED", "RATELIMIT_EXCEEDED", "SYSTEMERROR", "SYSTEM_ERROR", "BANKERROR", "BANK_ERROR":
		return NewRequestError(http.StatusServiceUnavailable, errors.New("payment close is temporarily unavailable, please retry later"))
	case "PARAM_ERROR", "INVALID_REQUEST", "APPID_MCHID_NOT_MATCH", "OPENID_MISMATCH", "MCH_NOT_EXISTS", "NOAUTH", "NO_AUTH", "SIGN_ERROR":
		return NewRequestError(http.StatusServiceUnavailable, errors.New("payment close configuration is temporarily unavailable, please retry later"))
	default:
		return NewRequestError(http.StatusServiceUnavailable, errors.New("payment close failed, please retry later"))
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
		"sub_mchid is required",
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
	return NewRequestError(http.StatusServiceUnavailable, errors.New("payment request preparation failed, please retry later"))
}

func normalizeWechatPaymentCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}
