package contracts

import (
	"fmt"
	"strings"
)

type DirectJSAPIOrderRequestValidationError struct {
	Message string
}

func (e *DirectJSAPIOrderRequestValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "create direct jsapi order: validation failed"
	}
	return e.Message
}

type DirectOrderQueryValidationError struct {
	Message string
}

func (e *DirectOrderQueryValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query direct order: validation failed"
	}
	return e.Message
}

type DirectOrderQueryContractError struct {
	Message string
}

func (e *DirectOrderQueryContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query direct order: upstream contract validation failed"
	}
	return e.Message
}

type DirectPaymentNotificationContractError struct {
	Message string
}

func (e *DirectPaymentNotificationContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "decrypt direct payment notification: upstream contract validation failed"
	}
	return e.Message
}

var allowedDirectOrderTradeStates = map[string]struct{}{
	DirectTradeStateSuccess:    {},
	DirectTradeStateRefund:     {},
	DirectTradeStateNotPay:     {},
	DirectTradeStateClosed:     {},
	DirectTradeStateRevoked:    {},
	DirectTradeStateUserPaying: {},
	DirectTradeStatePayError:   {},
}

var allowedDirectOrderTradeTypes = map[string]struct{}{
	DirectTradeTypeJSAPI:    {},
	DirectTradeTypeNative:   {},
	DirectTradeTypeApp:      {},
	DirectTradeTypeMicropay: {},
	DirectTradeTypeMWEB:     {},
	DirectTradeTypeFacePay:  {},
}

var allowedDirectPromotionScopes = map[string]struct{}{
	DirectPromotionScopeGlobal: {},
	DirectPromotionScopeSingle: {},
}

var allowedDirectPromotionTypes = map[string]struct{}{
	DirectPromotionTypeCash:   {},
	DirectPromotionTypeNoCash: {},
}

func ValidateDirectJSAPIOrderRequest(req *DirectJSAPIOrderRequest) error {
	if req == nil {
		return newDirectJSAPIOrderRequestValidationError("request is nil")
	}
	if strings.TrimSpace(req.Description) == "" || strings.TrimSpace(req.OutTradeNo) == "" {
		return newDirectJSAPIOrderRequestValidationError("description and out_trade_no are required")
	}
	if req.TotalAmount <= 0 {
		return newDirectJSAPIOrderRequestValidationError("total amount must be positive")
	}
	if strings.TrimSpace(req.PayerOpenID) == "" {
		return newDirectJSAPIOrderRequestValidationError("openid is required")
	}
	if strings.TrimSpace(req.NotifyURL) == "" {
		return newDirectJSAPIOrderRequestValidationError("notify_url is required")
	}
	if strings.TrimSpace(req.Currency) != "" && strings.TrimSpace(req.Currency) != DirectPaymentCurrencyCNY {
		return newDirectJSAPIOrderRequestValidationError("currency must be %q", DirectPaymentCurrencyCNY)
	}
	if (strings.TrimSpace(req.DeviceID) != "" || req.StoreInfo != nil) && strings.TrimSpace(req.PayerClientIP) == "" {
		return newDirectJSAPIOrderRequestValidationError("payer_client_ip is required when scene_info is provided")
	}
	if req.StoreInfo != nil && strings.TrimSpace(req.StoreInfo.ID) == "" {
		return newDirectJSAPIOrderRequestValidationError("scene_info.store_info.id is required when store_info is provided")
	}
	return nil
}

func ValidateDirectOrderQueryByTransactionIDInput(transactionID string) (string, error) {
	trimmedTransactionID := strings.TrimSpace(transactionID)
	if trimmedTransactionID == "" {
		return "", NewDirectOrderQueryValidationError("query direct order by transaction_id", "transaction_id is required")
	}
	return trimmedTransactionID, nil
}

func ValidateDirectOrderQueryByOutTradeNoInput(outTradeNo string) (string, error) {
	trimmedOutTradeNo := strings.TrimSpace(outTradeNo)
	if trimmedOutTradeNo == "" {
		return "", NewDirectOrderQueryValidationError("query direct order by out_trade_no", "out_trade_no is required")
	}
	return trimmedOutTradeNo, nil
}

func ValidateDirectOrderQueryResponse(operation string, resp *DirectOrderQueryResponse, requireTransactionFields bool) error {
	if resp == nil {
		return NewDirectOrderQueryContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.AppID) == "" {
		return NewDirectOrderQueryContractError(operation, "wechat response missing appid")
	}
	if strings.TrimSpace(resp.MchID) == "" {
		return NewDirectOrderQueryContractError(operation, "wechat response missing mchid")
	}
	if strings.TrimSpace(resp.OutTradeNo) == "" {
		return NewDirectOrderQueryContractError(operation, "wechat response missing out_trade_no")
	}
	if requireTransactionFields && strings.TrimSpace(resp.TransactionID) == "" {
		return NewDirectOrderQueryContractError(operation, "wechat response missing transaction_id")
	}
	if requireTransactionFields && strings.TrimSpace(resp.TradeType) == "" {
		return NewDirectOrderQueryContractError(operation, "wechat response missing trade_type")
	}
	if strings.TrimSpace(resp.TradeType) != "" {
		if _, ok := allowedDirectOrderTradeTypes[strings.ToUpper(strings.TrimSpace(resp.TradeType))]; !ok {
			return NewDirectOrderQueryContractError(operation, "unsupported trade_type %q", resp.TradeType)
		}
	}
	if strings.TrimSpace(resp.TradeState) == "" {
		return NewDirectOrderQueryContractError(operation, "wechat response missing trade_state")
	}
	if _, ok := allowedDirectOrderTradeStates[strings.ToUpper(strings.TrimSpace(resp.TradeState))]; !ok {
		return NewDirectOrderQueryContractError(operation, "unsupported trade_state %q", resp.TradeState)
	}
	if strings.TrimSpace(resp.TradeStateDesc) == "" {
		return NewDirectOrderQueryContractError(operation, "wechat response missing trade_state_desc")
	}
	if resp.SceneInfo != nil && strings.TrimSpace(resp.SceneInfo.DeviceID) == "" {
		return NewDirectOrderQueryContractError(operation, "scene_info.device_id is required when scene_info is provided")
	}
	if strings.TrimSpace(resp.Amount.Currency) != "" && strings.TrimSpace(resp.Amount.Currency) != DirectPaymentCurrencyCNY {
		return NewDirectOrderQueryContractError(operation, "amount.currency must be %q", DirectPaymentCurrencyCNY)
	}
	if strings.TrimSpace(resp.Amount.PayerCurrency) != "" && strings.TrimSpace(resp.Amount.PayerCurrency) != DirectPaymentCurrencyCNY {
		return NewDirectOrderQueryContractError(operation, "amount.payer_currency must be %q", DirectPaymentCurrencyCNY)
	}
	if err := validateDirectPromotionDetails(resp.PromotionDetail, true, func(format string, args ...any) error {
		return NewDirectOrderQueryContractError(operation, format, args...)
	}, "promotion_detail"); err != nil {
		return err
	}
	return nil
}

func ValidateDirectPaymentNotificationResource(operation string, resource *DirectPaymentNotificationResource) error {
	if resource == nil {
		return NewDirectPaymentNotificationContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resource.AppID) == "" {
		return NewDirectPaymentNotificationContractError(operation, "wechat response missing appid")
	}
	if strings.TrimSpace(resource.MchID) == "" {
		return NewDirectPaymentNotificationContractError(operation, "wechat response missing mchid")
	}
	if strings.TrimSpace(resource.OutTradeNo) == "" {
		return NewDirectPaymentNotificationContractError(operation, "wechat response missing out_trade_no")
	}
	if strings.TrimSpace(resource.TransactionID) == "" {
		return NewDirectPaymentNotificationContractError(operation, "wechat response missing transaction_id")
	}
	if strings.TrimSpace(resource.TradeType) == "" {
		return NewDirectPaymentNotificationContractError(operation, "wechat response missing trade_type")
	}
	if _, ok := allowedDirectOrderTradeTypes[strings.ToUpper(strings.TrimSpace(resource.TradeType))]; !ok {
		return NewDirectPaymentNotificationContractError(operation, "unsupported trade_type %q", resource.TradeType)
	}
	if strings.TrimSpace(resource.TradeState) == "" {
		return NewDirectPaymentNotificationContractError(operation, "wechat response missing trade_state")
	}
	if _, ok := allowedDirectOrderTradeStates[strings.ToUpper(strings.TrimSpace(resource.TradeState))]; !ok {
		return NewDirectPaymentNotificationContractError(operation, "unsupported trade_state %q", resource.TradeState)
	}
	if strings.TrimSpace(resource.TradeStateDesc) == "" {
		return NewDirectPaymentNotificationContractError(operation, "wechat response missing trade_state_desc")
	}
	if strings.TrimSpace(resource.BankType) == "" {
		return NewDirectPaymentNotificationContractError(operation, "wechat response missing bank_type")
	}
	if err := validateRFC3339Timestamp("success_time", resource.SuccessTime, func(format string, args ...any) error {
		return NewDirectPaymentNotificationContractError(operation, format, args...)
	}); err != nil {
		return err
	}
	if strings.TrimSpace(resource.Amount.Currency) == "" {
		return NewDirectPaymentNotificationContractError(operation, "amount.currency is required")
	}
	if strings.TrimSpace(resource.Amount.Currency) != DirectPaymentCurrencyCNY {
		return NewDirectPaymentNotificationContractError(operation, "amount.currency must be %q", DirectPaymentCurrencyCNY)
	}
	if strings.TrimSpace(resource.Amount.PayerCurrency) == "" {
		return NewDirectPaymentNotificationContractError(operation, "amount.payer_currency is required")
	}
	if strings.TrimSpace(resource.Amount.PayerCurrency) != DirectPaymentCurrencyCNY {
		return NewDirectPaymentNotificationContractError(operation, "amount.payer_currency must be %q", DirectPaymentCurrencyCNY)
	}
	if resource.SceneInfo != nil && strings.TrimSpace(resource.SceneInfo.DeviceID) == "" {
		return NewDirectPaymentNotificationContractError(operation, "scene_info.device_id is required when scene_info is provided")
	}
	if err := validateDirectPromotionDetails(resource.PromotionDetail, false, func(format string, args ...any) error {
		return NewDirectPaymentNotificationContractError(operation, format, args...)
	}, "promotion_detail"); err != nil {
		return err
	}
	return nil
}

func NewDirectOrderQueryValidationError(operation string, format string, args ...any) error {
	return &DirectOrderQueryValidationError{Message: formatDirectOrderingValidationMessage("query direct order", operation, format, args...)}
}

func NewDirectOrderQueryContractError(operation string, format string, args ...any) error {
	return &DirectOrderQueryContractError{Message: formatDirectOrderingValidationMessage("query direct order", operation, format, args...)}
}

func NewDirectPaymentNotificationContractError(operation string, format string, args ...any) error {
	return &DirectPaymentNotificationContractError{Message: formatDirectOrderingValidationMessage("decrypt direct payment notification", operation, format, args...)}
}

func newDirectJSAPIOrderRequestValidationError(format string, args ...any) error {
	return &DirectJSAPIOrderRequestValidationError{Message: fmt.Sprintf("create direct jsapi order: "+format, args...)}
}

func formatDirectOrderingValidationMessage(defaultOperation, operation, format string, args ...any) string {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = defaultOperation
	}
	return fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))
}

func validateDirectPromotionDetails(details []DirectPromotionDetail, requireName bool, errorFactory func(string, ...any) error, fieldPrefix string) error {
	for index, detail := range details {
		if strings.TrimSpace(detail.CouponID) == "" {
			return errorFactory("%s[%d].coupon_id is required", fieldPrefix, index)
		}
		if requireName && strings.TrimSpace(detail.Name) == "" {
			return errorFactory("%s[%d].name is required", fieldPrefix, index)
		}
		if detail.Scope != "" {
			if _, ok := allowedDirectPromotionScopes[strings.ToUpper(strings.TrimSpace(detail.Scope))]; !ok {
				return errorFactory("%s[%d].scope has unsupported value %q", fieldPrefix, index, detail.Scope)
			}
		}
		if detail.Type != "" {
			if _, ok := allowedDirectPromotionTypes[strings.ToUpper(strings.TrimSpace(detail.Type))]; !ok {
				return errorFactory("%s[%d].type has unsupported value %q", fieldPrefix, index, detail.Type)
			}
		}
		if detail.Currency != "" && strings.TrimSpace(detail.Currency) != DirectPaymentCurrencyCNY {
			return errorFactory("%s[%d].currency must be %q", fieldPrefix, index, DirectPaymentCurrencyCNY)
		}
		for goodsIndex, goods := range detail.GoodsDetail {
			if strings.TrimSpace(goods.GoodsID) == "" {
				return errorFactory("%s[%d].goods_detail[%d].goods_id is required", fieldPrefix, index, goodsIndex)
			}
		}
	}
	return nil
}
