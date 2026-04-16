package contracts

import (
	"fmt"
	"strings"
	"time"
)

type PartnerJSAPIOrderRequestValidationError struct {
	Message string
}

func (e *PartnerJSAPIOrderRequestValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "create partner jsapi order: validation failed"
	}
	return e.Message
}

type CombineOrderRequestValidationError struct {
	Message string
}

func (e *CombineOrderRequestValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "create combine order: validation failed"
	}
	return e.Message
}

type PartnerOrderQueryValidationError struct {
	Message string
}

func (e *PartnerOrderQueryValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query partner order: validation failed"
	}
	return e.Message
}

type PartnerOrderQueryContractError struct {
	Message string
}

func (e *PartnerOrderQueryContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query partner order: upstream contract validation failed"
	}
	return e.Message
}

type PartnerPaymentNotificationContractError struct {
	Message string
}

func (e *PartnerPaymentNotificationContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "decrypt partner payment notification: upstream contract validation failed"
	}
	return e.Message
}

type CombineOrderQueryValidationError struct {
	Message string
}

func (e *CombineOrderQueryValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query combine order: validation failed"
	}
	return e.Message
}

type CombineOrderQueryContractError struct {
	Message string
}

func (e *CombineOrderQueryContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "query combine order: upstream contract validation failed"
	}
	return e.Message
}

type CombinePaymentNotificationContractError struct {
	Message string
}

func (e *CombinePaymentNotificationContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "decrypt combine payment notification: upstream contract validation failed"
	}
	return e.Message
}

var allowedPartnerOrderTradeStates = map[string]struct{}{
	"SUCCESS":    {},
	"REFUND":     {},
	"NOTPAY":     {},
	"CLOSED":     {},
	"REVOKED":    {},
	"USERPAYING": {},
	"PAYERROR":   {},
}

var allowedPartnerOrderTradeTypes = map[string]struct{}{
	"JSAPI":    {},
	"NATIVE":   {},
	"APP":      {},
	"MICROPAY": {},
	"MWEB":     {},
	"FACEPAY":  {},
}

var allowedCombineOrderTradeStates = map[string]struct{}{
	"SUCCESS":  {},
	"REFUND":   {},
	"NOTPAY":   {},
	"CLOSED":   {},
	"PAYERROR": {},
}

var allowedCombineOrderTradeTypes = map[string]struct{}{
	"NATIVE": {},
	"JSAPI":  {},
	"APP":    {},
	"MWEB":   {},
}

var allowedPartnerPromotionScopes = map[string]struct{}{
	"GLOBAL": {},
	"SINGLE": {},
}

var allowedPartnerPromotionTypes = map[string]struct{}{
	"CASH":   {},
	"NOCASH": {},
}

func ValidatePartnerJSAPIOrderRequest(req *PartnerJSAPIOrderRequest) error {
	if req == nil {
		return newPartnerJSAPIOrderRequestValidationError("request is nil")
	}
	if strings.TrimSpace(req.PayerSubOpenID) != "" || strings.TrimSpace(req.SubAppID) != "" {
		return newPartnerJSAPIOrderRequestValidationError("sub_openid and sub_appid are not supported in the single-appid project flow")
	}
	if strings.TrimSpace(req.SubMchID) == "" {
		return newPartnerJSAPIOrderRequestValidationError("sub_mchid is required")
	}
	if strings.TrimSpace(req.Description) == "" || strings.TrimSpace(req.OutTradeNo) == "" {
		return newPartnerJSAPIOrderRequestValidationError("description and out_trade_no are required")
	}
	if req.TotalAmount <= 0 {
		return newPartnerJSAPIOrderRequestValidationError("total amount must be positive")
	}
	if strings.TrimSpace(req.PayerOpenID) == "" && strings.TrimSpace(req.PayerSubOpenID) == "" {
		return newPartnerJSAPIOrderRequestValidationError("sp_openid or sub_openid is required")
	}
	if (strings.TrimSpace(req.DeviceID) != "" || req.StoreInfo != nil) && strings.TrimSpace(req.PayerClientIP) == "" {
		return newPartnerJSAPIOrderRequestValidationError("payer_client_ip is required when scene_info is provided")
	}
	if req.StoreInfo != nil && strings.TrimSpace(req.StoreInfo.ID) == "" {
		return newPartnerJSAPIOrderRequestValidationError("scene_info.store_info.id is required when store_info is provided")
	}
	return nil
}

func ValidateCombineOrderRequest(req *CombineOrderRequest) error {
	if req == nil {
		return newCombineOrderRequestValidationError("request is nil")
	}
	if strings.TrimSpace(req.PayerSubOpenID) != "" {
		return newCombineOrderRequestValidationError("sub_openid is not supported in the single-appid project flow")
	}
	if strings.TrimSpace(req.CombineOutTradeNo) == "" {
		return newCombineOrderRequestValidationError("combine_out_trade_no is required")
	}
	if len(req.SubOrders) == 0 {
		return newCombineOrderRequestValidationError("sub_orders is required")
	}
	if len(req.SubOrders) > 50 {
		return newCombineOrderRequestValidationError("sub_orders exceeds the maximum of 50")
	}
	if strings.TrimSpace(req.PayerOpenID) == "" && strings.TrimSpace(req.PayerSubOpenID) == "" {
		return newCombineOrderRequestValidationError("openid or sub_openid is required")
	}
	if req.SceneInfo != nil && strings.TrimSpace(req.SceneInfo.PayerClientIP) == "" {
		return newCombineOrderRequestValidationError("scene_info.payer_client_ip is required when scene_info is provided")
	}
	for index, sub := range req.SubOrders {
		if strings.TrimSpace(sub.SubAppID) != "" {
			return newCombineOrderRequestValidationError("sub_orders[%d].sub_appid is not supported in the single-appid project flow", index)
		}
		if strings.TrimSpace(sub.OutTradeNo) == "" {
			return newCombineOrderRequestValidationError("sub_orders[%d].out_trade_no is required", index)
		}
		if strings.TrimSpace(sub.Attach) == "" {
			return newCombineOrderRequestValidationError("sub_orders[%d].attach is required", index)
		}
		if strings.TrimSpace(sub.Description) == "" {
			return newCombineOrderRequestValidationError("sub_orders[%d].description is required", index)
		}
		if sub.Amount <= 0 {
			return newCombineOrderRequestValidationError("sub_orders[%d].amount.total_amount must be positive", index)
		}
	}
	return nil
}

func ValidatePartnerOrderQueryByTransactionIDInput(transactionID, subMchID string) (string, string, error) {
	trimmedTransactionID := strings.TrimSpace(transactionID)
	if trimmedTransactionID == "" {
		return "", "", NewPartnerOrderQueryValidationError("query partner order by transaction_id", "transaction_id is required")
	}
	trimmedSubMchID := strings.TrimSpace(subMchID)
	if trimmedSubMchID == "" {
		return "", "", NewPartnerOrderQueryValidationError("query partner order by transaction_id", "sub_mchid is required")
	}
	return trimmedTransactionID, trimmedSubMchID, nil
}

func ValidatePartnerOrderQueryByOutTradeNoInput(outTradeNo, subMchID string) (string, string, error) {
	trimmedOutTradeNo := strings.TrimSpace(outTradeNo)
	if trimmedOutTradeNo == "" {
		return "", "", NewPartnerOrderQueryValidationError("query partner order by out_trade_no", "out_trade_no is required")
	}
	trimmedSubMchID := strings.TrimSpace(subMchID)
	if trimmedSubMchID == "" {
		return "", "", NewPartnerOrderQueryValidationError("query partner order by out_trade_no", "sub_mchid is required")
	}
	return trimmedOutTradeNo, trimmedSubMchID, nil
}

func ValidateCombineOrderQueryInput(combineOutTradeNo string) (string, error) {
	trimmed := strings.TrimSpace(combineOutTradeNo)
	if trimmed == "" {
		return "", NewCombineOrderQueryValidationError("query combine order", "combine_out_trade_no is required")
	}
	return trimmed, nil
}

func ValidatePartnerOrderQueryResponse(operation string, resp *PartnerOrderQueryResponse, requireTransactionFields bool) error {
	if resp == nil {
		return NewPartnerOrderQueryContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.SpAppID) == "" {
		return NewPartnerOrderQueryContractError(operation, "wechat response missing sp_appid")
	}
	if strings.TrimSpace(resp.SpMchID) == "" {
		return NewPartnerOrderQueryContractError(operation, "wechat response missing sp_mchid")
	}
	if strings.TrimSpace(resp.SubMchID) == "" {
		return NewPartnerOrderQueryContractError(operation, "wechat response missing sub_mchid")
	}
	if strings.TrimSpace(resp.OutTradeNo) == "" {
		return NewPartnerOrderQueryContractError(operation, "wechat response missing out_trade_no")
	}
	if requireTransactionFields && strings.TrimSpace(resp.TransactionID) == "" {
		return NewPartnerOrderQueryContractError(operation, "wechat response missing transaction_id")
	}
	if requireTransactionFields && strings.TrimSpace(resp.TradeType) == "" {
		return NewPartnerOrderQueryContractError(operation, "wechat response missing trade_type")
	}
	if strings.TrimSpace(resp.TradeType) != "" {
		if _, ok := allowedPartnerOrderTradeTypes[strings.ToUpper(strings.TrimSpace(resp.TradeType))]; !ok {
			return NewPartnerOrderQueryContractError(operation, "unsupported trade_type %q", resp.TradeType)
		}
	}
	if strings.TrimSpace(resp.TradeState) == "" {
		return NewPartnerOrderQueryContractError(operation, "wechat response missing trade_state")
	}
	if _, ok := allowedPartnerOrderTradeStates[strings.ToUpper(strings.TrimSpace(resp.TradeState))]; !ok {
		return NewPartnerOrderQueryContractError(operation, "unsupported trade_state %q", resp.TradeState)
	}
	if strings.TrimSpace(resp.TradeStateDesc) == "" {
		return NewPartnerOrderQueryContractError(operation, "wechat response missing trade_state_desc")
	}
	if resp.SceneInfo != nil && strings.TrimSpace(resp.SceneInfo.DeviceID) == "" {
		return NewPartnerOrderQueryContractError(operation, "scene_info.device_id is required when scene_info is provided")
	}
	if err := validatePartnerPromotionDetails(resp.PromotionDetail, true, func(format string, args ...any) error {
		return NewPartnerOrderQueryContractError(operation, format, args...)
	}, "promotion_detail"); err != nil {
		return err
	}
	return nil
}

func ValidatePartnerPaymentNotification(operation string, resource *PartnerPaymentNotificationResource) error {
	if resource == nil {
		return NewPartnerPaymentNotificationContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resource.SpAppID) == "" {
		return NewPartnerPaymentNotificationContractError(operation, "wechat response missing sp_appid")
	}
	if strings.TrimSpace(resource.SpMchID) == "" {
		return NewPartnerPaymentNotificationContractError(operation, "wechat response missing sp_mchid")
	}
	if strings.TrimSpace(resource.SubMchID) == "" {
		return NewPartnerPaymentNotificationContractError(operation, "wechat response missing sub_mchid")
	}
	if strings.TrimSpace(resource.OutTradeNo) == "" {
		return NewPartnerPaymentNotificationContractError(operation, "wechat response missing out_trade_no")
	}
	if strings.TrimSpace(resource.TransactionID) == "" {
		return NewPartnerPaymentNotificationContractError(operation, "wechat response missing transaction_id")
	}
	if strings.TrimSpace(resource.TradeType) == "" {
		return NewPartnerPaymentNotificationContractError(operation, "wechat response missing trade_type")
	}
	if _, ok := allowedPartnerOrderTradeTypes[strings.ToUpper(strings.TrimSpace(resource.TradeType))]; !ok {
		return NewPartnerPaymentNotificationContractError(operation, "unsupported trade_type %q", resource.TradeType)
	}
	if strings.TrimSpace(resource.TradeState) == "" {
		return NewPartnerPaymentNotificationContractError(operation, "wechat response missing trade_state")
	}
	if _, ok := allowedPartnerOrderTradeStates[strings.ToUpper(strings.TrimSpace(resource.TradeState))]; !ok {
		return NewPartnerPaymentNotificationContractError(operation, "unsupported trade_state %q", resource.TradeState)
	}
	if strings.TrimSpace(resource.TradeStateDesc) == "" {
		return NewPartnerPaymentNotificationContractError(operation, "wechat response missing trade_state_desc")
	}
	if strings.TrimSpace(resource.BankType) == "" {
		return NewPartnerPaymentNotificationContractError(operation, "wechat response missing bank_type")
	}
	if err := validateRFC3339Timestamp("success_time", resource.SuccessTime, func(format string, args ...any) error {
		return NewPartnerPaymentNotificationContractError(operation, format, args...)
	}); err != nil {
		return err
	}
	if strings.TrimSpace(resource.Amount.Currency) == "" {
		return NewPartnerPaymentNotificationContractError(operation, "amount.currency is required")
	}
	if strings.TrimSpace(resource.Amount.PayerCurrency) == "" {
		return NewPartnerPaymentNotificationContractError(operation, "amount.payer_currency is required")
	}
	if resource.SceneInfo != nil && strings.TrimSpace(resource.SceneInfo.DeviceID) == "" {
		return NewPartnerPaymentNotificationContractError(operation, "scene_info.device_id is required when scene_info is provided")
	}
	if err := validatePartnerPromotionDetails(resource.PromotionDetail, false, func(format string, args ...any) error {
		return NewPartnerPaymentNotificationContractError(operation, format, args...)
	}, "promotion_detail"); err != nil {
		return err
	}
	return nil

}

func ValidateCombineOrderQueryResponse(operation string, resp *CombineQueryResponseBody) error {
	if resp == nil {
		return NewCombineOrderQueryContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resp.CombineAppID) == "" {
		return NewCombineOrderQueryContractError(operation, "wechat response missing combine_appid")
	}
	if strings.TrimSpace(resp.CombineMchID) == "" {
		return NewCombineOrderQueryContractError(operation, "wechat response missing combine_mchid")
	}
	if strings.TrimSpace(resp.CombineOutTradeNo) == "" {
		return NewCombineOrderQueryContractError(operation, "wechat response missing combine_out_trade_no")
	}
	if resp.CombinePayerInfo != nil && strings.TrimSpace(resp.CombinePayerInfo.OpenID) == "" {
		return NewCombineOrderQueryContractError(operation, "combine_payer_info.openid is required when combine_payer_info is present")
	}
	if resp.SceneInfo != nil && strings.TrimSpace(resp.SceneInfo.DeviceID) == "" {
		return NewCombineOrderQueryContractError(operation, "scene_info.device_id is required when scene_info is present")
	}
	for index, subOrder := range resp.SubOrders {
		if strings.TrimSpace(subOrder.MchID) == "" {
			return NewCombineOrderQueryContractError(operation, "sub_orders[%d].mchid is required", index)
		}
		if strings.TrimSpace(subOrder.OutTradeNo) == "" {
			return NewCombineOrderQueryContractError(operation, "sub_orders[%d].out_trade_no is required", index)
		}
		if strings.TrimSpace(subOrder.TradeState) == "" {
			return NewCombineOrderQueryContractError(operation, "sub_orders[%d].trade_state is required", index)
		}
		if _, ok := allowedCombineOrderTradeStates[strings.ToUpper(strings.TrimSpace(subOrder.TradeState))]; !ok {
			return NewCombineOrderQueryContractError(operation, "sub_orders[%d].trade_state has unsupported value %q", index, subOrder.TradeState)
		}
		if strings.TrimSpace(subOrder.TradeType) != "" {
			if _, ok := allowedCombineOrderTradeTypes[strings.ToUpper(strings.TrimSpace(subOrder.TradeType))]; !ok {
				return NewCombineOrderQueryContractError(operation, "sub_orders[%d].trade_type has unsupported value %q", index, subOrder.TradeType)
			}
		}
		if subOrder.Amount == nil {
			return NewCombineOrderQueryContractError(operation, "sub_orders[%d].amount is required", index)
		}
		if strings.TrimSpace(subOrder.Amount.Currency) == "" {
			return NewCombineOrderQueryContractError(operation, "sub_orders[%d].amount.currency is required", index)
		}
		if strings.TrimSpace(subOrder.Amount.PayerCurrency) == "" {
			return NewCombineOrderQueryContractError(operation, "sub_orders[%d].amount.payer_currency is required", index)
		}
		if err := validatePartnerPromotionDetails(subOrder.PromotionDetail, false, func(format string, args ...any) error {
			indexedArgs := append([]any{index}, args...)
			return NewCombineOrderQueryContractError(operation, "sub_orders[%d]."+format, indexedArgs...)
		}, "promotion_detail"); err != nil {
			return err
		}
	}
	return nil
}

func ValidateCombinePaymentNotification(operation string, resource *CombinePaymentNotification) error {
	if resource == nil {
		return NewCombinePaymentNotificationContractError(operation, "empty wechat response")
	}
	if strings.TrimSpace(resource.CombineAppID) == "" {
		return NewCombinePaymentNotificationContractError(operation, "wechat response missing combine_appid")
	}
	if strings.TrimSpace(resource.CombineMchID) == "" {
		return NewCombinePaymentNotificationContractError(operation, "wechat response missing combine_mchid")
	}
	if strings.TrimSpace(resource.CombineOutTradeNo) == "" {
		return NewCombinePaymentNotificationContractError(operation, "wechat response missing combine_out_trade_no")
	}
	if resource.CombinePayerInfo == nil {
		return NewCombinePaymentNotificationContractError(operation, "combine_payer_info is required")
	}
	if strings.TrimSpace(resource.CombinePayerInfo.OpenID) == "" {
		return NewCombinePaymentNotificationContractError(operation, "combine_payer_info.openid is required")
	}
	if resource.SceneInfo != nil && strings.TrimSpace(resource.SceneInfo.DeviceID) == "" {
		return NewCombinePaymentNotificationContractError(operation, "scene_info.device_id is required when scene_info is present")
	}
	if len(resource.SubOrders) == 0 {
		return NewCombinePaymentNotificationContractError(operation, "sub_orders is required")
	}
	for index, subOrder := range resource.SubOrders {
		if strings.TrimSpace(subOrder.MchID) == "" {
			return NewCombinePaymentNotificationContractError(operation, "sub_orders[%d].mchid is required", index)
		}
		if strings.TrimSpace(subOrder.OutTradeNo) == "" {
			return NewCombinePaymentNotificationContractError(operation, "sub_orders[%d].out_trade_no is required", index)
		}
		if strings.TrimSpace(subOrder.TransactionID) == "" {
			return NewCombinePaymentNotificationContractError(operation, "sub_orders[%d].transaction_id is required", index)
		}
		if strings.TrimSpace(subOrder.TradeType) == "" {
			return NewCombinePaymentNotificationContractError(operation, "sub_orders[%d].trade_type is required", index)
		}
		if _, ok := allowedCombineOrderTradeTypes[strings.ToUpper(strings.TrimSpace(subOrder.TradeType))]; !ok {
			return NewCombinePaymentNotificationContractError(operation, "sub_orders[%d].trade_type has unsupported value %q", index, subOrder.TradeType)
		}
		if strings.TrimSpace(subOrder.TradeState) == "" {
			return NewCombinePaymentNotificationContractError(operation, "sub_orders[%d].trade_state is required", index)
		}
		if _, ok := allowedCombineOrderTradeStates[strings.ToUpper(strings.TrimSpace(subOrder.TradeState))]; !ok {
			return NewCombinePaymentNotificationContractError(operation, "sub_orders[%d].trade_state has unsupported value %q", index, subOrder.TradeState)
		}
		if strings.TrimSpace(subOrder.BankType) == "" {
			return NewCombinePaymentNotificationContractError(operation, "sub_orders[%d].bank_type is required", index)
		}
		if err := validateRFC3339Timestamp(fmt.Sprintf("sub_orders[%d].success_time", index), subOrder.SuccessTime, func(format string, args ...any) error {
			return NewCombinePaymentNotificationContractError(operation, format, args...)
		}); err != nil {
			return err
		}
		if strings.TrimSpace(subOrder.Amount.Currency) == "" {
			return NewCombinePaymentNotificationContractError(operation, "sub_orders[%d].amount.currency is required", index)
		}
		if strings.TrimSpace(subOrder.Amount.PayerCurrency) == "" {
			return NewCombinePaymentNotificationContractError(operation, "sub_orders[%d].amount.payer_currency is required", index)
		}
		if err := validatePartnerPromotionDetails(subOrder.PromotionDetail, false, func(format string, args ...any) error {
			indexedArgs := append([]any{index}, args...)
			return NewCombinePaymentNotificationContractError(operation, "sub_orders[%d]."+format, indexedArgs...)
		}, "promotion_detail"); err != nil {
			return err
		}
	}
	return nil
}

func NewPartnerOrderQueryValidationError(operation string, format string, args ...any) error {
	return &PartnerOrderQueryValidationError{Message: formatOrderingValidationMessage("query partner order", operation, format, args...)}
}

func NewPartnerOrderQueryContractError(operation string, format string, args ...any) error {
	return &PartnerOrderQueryContractError{Message: formatOrderingValidationMessage("query partner order", operation, format, args...)}
}

func NewPartnerPaymentNotificationContractError(operation string, format string, args ...any) error {
	return &PartnerPaymentNotificationContractError{Message: formatOrderingValidationMessage("decrypt partner payment notification", operation, format, args...)}
}

func NewCombineOrderQueryValidationError(operation string, format string, args ...any) error {
	return &CombineOrderQueryValidationError{Message: formatOrderingValidationMessage("query combine order", operation, format, args...)}
}

func NewCombineOrderQueryContractError(operation string, format string, args ...any) error {
	return &CombineOrderQueryContractError{Message: formatOrderingValidationMessage("query combine order", operation, format, args...)}
}

func NewCombinePaymentNotificationContractError(operation string, format string, args ...any) error {
	return &CombinePaymentNotificationContractError{Message: formatOrderingValidationMessage("decrypt combine payment notification", operation, format, args...)}
}

func newPartnerJSAPIOrderRequestValidationError(format string, args ...any) error {
	return &PartnerJSAPIOrderRequestValidationError{Message: fmt.Sprintf("create partner jsapi order: "+format, args...)}
}

func newCombineOrderRequestValidationError(format string, args ...any) error {
	return &CombineOrderRequestValidationError{Message: fmt.Sprintf("create combine order: "+format, args...)}
}

func formatOrderingValidationMessage(defaultOperation, operation, format string, args ...any) string {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = defaultOperation
	}
	return fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))
}

func validateRFC3339Timestamp(field, value string, errorFactory func(string, ...any) error) error {
	if strings.TrimSpace(value) == "" {
		return errorFactory("%s is required", field)
	}
	if _, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value)); err != nil {
		return errorFactory("%s must be RFC3339: %v", field, err)
	}
	return nil
}

func validatePartnerPromotionDetails(details []PartnerPromotionDetail, requireName bool, errorFactory func(string, ...any) error, fieldPrefix string) error {
	for index, detail := range details {
		if strings.TrimSpace(detail.CouponID) == "" {
			return errorFactory("%s[%d].coupon_id is required", fieldPrefix, index)
		}
		if requireName && strings.TrimSpace(detail.Name) == "" {
			return errorFactory("%s[%d].name is required", fieldPrefix, index)
		}
		if detail.Scope != "" {
			if _, ok := allowedPartnerPromotionScopes[strings.ToUpper(strings.TrimSpace(detail.Scope))]; !ok {
				return errorFactory("%s[%d].scope has unsupported value %q", fieldPrefix, index, detail.Scope)
			}
		}
		if detail.Type != "" {
			if _, ok := allowedPartnerPromotionTypes[strings.ToUpper(strings.TrimSpace(detail.Type))]; !ok {
				return errorFactory("%s[%d].type has unsupported value %q", fieldPrefix, index, detail.Type)
			}
		}
		for goodsIndex, goods := range detail.GoodsDetail {
			if strings.TrimSpace(goods.GoodsID) == "" {
				return errorFactory("%s[%d].goods_detail[%d].goods_id is required", fieldPrefix, index, goodsIndex)
			}
		}
	}
	return nil
}
