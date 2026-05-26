package contracts

import (
	"fmt"
	"strings"
	"time"
)

const (
	refundIDMaxLength          = 32
	refundOutTradeNoMaxLength  = 32
	refundOutRefundNoMaxLength = 64
	refundReasonMaxLength      = 80
	refundNotifyURLMaxLength   = 256
	refundMchIDMaxLength       = 32
)

type RefundValidationError struct {
	Message string
}

type RefundContractError struct {
	Message string
}

func (e *RefundValidationError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "refund: validation failed"
	}
	return e.Message
}

func (e *RefundContractError) Error() string {
	if e == nil || strings.TrimSpace(e.Message) == "" {
		return "refund: upstream contract validation failed"
	}
	return e.Message
}

var allowedDirectRefundChannels = map[string]struct{}{
	DirectRefundChannelOriginal:      {},
	DirectRefundChannelBalance:       {},
	DirectRefundChannelOtherBalance:  {},
	DirectRefundChannelOtherBankCard: {},
}

var allowedDirectRefundStatuses = map[string]struct{}{
	DirectRefundStatusSuccess:    {},
	DirectRefundStatusClosed:     {},
	DirectRefundStatusProcessing: {},
	DirectRefundStatusAbnormal:   {},
}

var allowedDirectRefundFundsAccounts = map[string]struct{}{
	DirectRefundFundsAccountUnsetttled:  {},
	DirectRefundFundsAccountAvailable:   {},
	DirectRefundFundsAccountUnavailable: {},
	DirectRefundFundsAccountOperation:   {},
	DirectRefundFundsAccountBasic:       {},
	DirectRefundFundsAccountECNYBasic:   {},
}

var allowedDirectRefundPromotionScopes = map[string]struct{}{
	DirectPromotionScopeGlobal: {},
	DirectPromotionScopeSingle: {},
}

var allowedDirectRefundPromotionTypes = map[string]struct{}{
	DirectPromotionTypeCash:   {},
	DirectPromotionTypeNoCash: {},
}

var allowedDirectRefundRequestFundsAccounts = map[string]struct{}{
	DirectRefundRequestFundsAccountAvailable: {},
	DirectRefundRequestFundsAccountUnsettle:  {},
}

func ValidateDirectRefundRequest(req *DirectRefundRequest) error {
	if req == nil {
		return newRefundValidationError("create direct refund", "request is nil")
	}
	if strings.TrimSpace(req.TransactionID) == "" && strings.TrimSpace(req.OutTradeNo) == "" {
		return newRefundValidationError("create direct refund", "transaction_id or out_trade_no is required")
	}
	if err := validateOptionalMaxLength("create direct refund", "transaction_id", req.TransactionID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateOptionalMaxLength("create direct refund", "out_trade_no", req.OutTradeNo, refundOutTradeNoMaxLength); err != nil {
		return err
	}
	if err := validateRequiredString("create direct refund", "out_refund_no", req.OutRefundNo, refundOutRefundNoMaxLength); err != nil {
		return err
	}
	if err := validateOptionalMaxLength("create direct refund", "reason", req.Reason, refundReasonMaxLength); err != nil {
		return err
	}
	if err := validateRefundNotifyURL("create direct refund", req.NotifyURL, false); err != nil {
		return err
	}
	if err := validateOptionalEnum("create direct refund", "funds_account", req.FundsAccount, allowedDirectRefundRequestFundsAccounts); err != nil {
		return err
	}
	if req.Amount == nil {
		return newRefundValidationError("create direct refund", "amount is required")
	}
	if err := validateDirectRefundRequestAmount("create direct refund", req.Amount); err != nil {
		return err
	}
	for index, goods := range req.GoodsDetail {
		if strings.TrimSpace(goods.MerchantGoodsID) == "" {
			return newRefundValidationError("create direct refund", "goods_detail[%d].merchant_goods_id is required", index)
		}
		if goods.UnitPrice <= 0 {
			return newRefundValidationError("create direct refund", "goods_detail[%d].unit_price must be positive", index)
		}
		if goods.RefundAmount <= 0 {
			return newRefundValidationError("create direct refund", "goods_detail[%d].refund_amount must be positive", index)
		}
		if goods.RefundQuantity <= 0 {
			return newRefundValidationError("create direct refund", "goods_detail[%d].refund_quantity must be positive", index)
		}
	}
	return nil
}

func ValidateDirectQueryRefundByOutRefundNoInput(outRefundNo string) (string, error) {
	trimmedOutRefundNo := strings.TrimSpace(outRefundNo)
	if trimmedOutRefundNo == "" {
		return "", newRefundValidationError("query direct refund", "out_refund_no is required")
	}
	if len(trimmedOutRefundNo) > refundOutRefundNoMaxLength {
		return "", newRefundValidationError("query direct refund", "out_refund_no must not exceed %d characters", refundOutRefundNoMaxLength)
	}
	return trimmedOutRefundNo, nil
}

func ValidateDirectRefundResponse(operation string, resp *DirectRefundResponse) error {
	return ValidateDirectRefundQueryResponse(operation, resp)
}

func ValidateDirectRefundCreateResponse(operation string, resp *DirectRefundResponse) error {
	if resp == nil {
		return newRefundContractError(operation, "empty wechat response")
	}
	if err := validateRequiredContractString(operation, "refund_id", resp.RefundID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "out_refund_no", resp.OutRefundNo, refundOutRefundNoMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "transaction_id", resp.TransactionID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "out_trade_no", resp.OutTradeNo, refundOutTradeNoMaxLength); err != nil {
		return err
	}
	if err := validateOptionalContractEnum(operation, "channel", resp.Channel, allowedDirectRefundChannels); err != nil {
		return err
	}
	if err := validateOptionalContractString(operation, "user_received_account", resp.UserReceivedAccount, 0); err != nil {
		return err
	}
	if err := validateRFC3339(operation, "success_time", resp.SuccessTime, false); err != nil {
		return err
	}
	if err := validateRFC3339(operation, "create_time", resp.CreateTime, true); err != nil {
		return err
	}
	if err := validateRequiredContractEnum(operation, "status", resp.Status, allowedDirectRefundStatuses); err != nil {
		return err
	}
	if err := validateOptionalContractEnum(operation, "funds_account", resp.FundsAccount, allowedDirectRefundFundsAccounts); err != nil {
		return err
	}
	if err := validateDirectRefundCreateAmountContract(operation, "amount", &resp.Amount); err != nil {
		return err
	}
	for index, detail := range resp.PromotionDetail {
		if strings.TrimSpace(detail.PromotionID) == "" {
			return newRefundContractError(operation, "promotion_detail[%d].promotion_id is required", index)
		}
		if err := validateRequiredContractEnum(operation, fmt.Sprintf("promotion_detail[%d].scope", index), detail.Scope, allowedDirectRefundPromotionScopes); err != nil {
			return err
		}
		if err := validateRequiredContractEnum(operation, fmt.Sprintf("promotion_detail[%d].type", index), detail.Type, allowedDirectRefundPromotionTypes); err != nil {
			return err
		}
		if detail.Amount <= 0 {
			return newRefundContractError(operation, "promotion_detail[%d].amount must be positive", index)
		}
		if detail.RefundAmount <= 0 {
			return newRefundContractError(operation, "promotion_detail[%d].refund_amount must be positive", index)
		}
	}
	return nil
}

func ValidateDirectRefundQueryResponse(operation string, resp *DirectRefundResponse) error {
	if resp == nil {
		return newRefundContractError(operation, "empty wechat response")
	}
	if err := validateRequiredContractString(operation, "refund_id", resp.RefundID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "out_refund_no", resp.OutRefundNo, refundOutRefundNoMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "transaction_id", resp.TransactionID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "out_trade_no", resp.OutTradeNo, refundOutTradeNoMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractEnum(operation, "channel", resp.Channel, allowedDirectRefundChannels); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "user_received_account", resp.UserReceivedAccount, 0); err != nil {
		return err
	}
	if err := validateRFC3339(operation, "success_time", resp.SuccessTime, false); err != nil {
		return err
	}
	if err := validateRFC3339(operation, "create_time", resp.CreateTime, true); err != nil {
		return err
	}
	if err := validateRequiredContractEnum(operation, "status", resp.Status, allowedDirectRefundStatuses); err != nil {
		return err
	}
	if err := validateRequiredContractEnum(operation, "funds_account", resp.FundsAccount, allowedDirectRefundFundsAccounts); err != nil {
		return err
	}
	if err := validateDirectRefundAmountContract(operation, "amount", &resp.Amount); err != nil {
		return err
	}
	return nil
}

func ValidateDirectRefundNotificationResource(operation string, resp *DirectRefundNotificationResource) error {
	if resp == nil {
		return newRefundContractError(operation, "empty wechat response")
	}
	if err := validateRequiredContractString(operation, "mchid", resp.MchID, refundMchIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "out_trade_no", resp.OutTradeNo, refundOutTradeNoMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "transaction_id", resp.TransactionID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "out_refund_no", resp.OutRefundNo, refundOutRefundNoMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "refund_id", resp.RefundID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractEnum(operation, "refund_status", resp.RefundStatus, allowedDirectRefundStatuses); err != nil {
		return err
	}
	if err := validateRFC3339(operation, "success_time", resp.SuccessTime, false); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "user_received_account", resp.UserReceivedAccount, 0); err != nil {
		return err
	}
	if resp.Amount.Total <= 0 {
		return newRefundContractError(operation, "amount.total must be positive")
	}
	if resp.Amount.Refund <= 0 {
		return newRefundContractError(operation, "amount.refund must be positive")
	}
	if resp.Amount.PayerTotal <= 0 {
		return newRefundContractError(operation, "amount.payer_total must be positive")
	}
	if resp.Amount.PayerRefund <= 0 {
		return newRefundContractError(operation, "amount.payer_refund must be positive")
	}
	return nil
}

func validateDirectRefundRequestAmount(operation string, amount *DirectRefundRequestAmount) error {
	if amount.Refund <= 0 {
		return newRefundValidationError(operation, "amount.refund must be positive")
	}
	if amount.Total <= 0 {
		return newRefundValidationError(operation, "amount.total must be positive")
	}
	if strings.TrimSpace(amount.Currency) != DirectRefundCurrencyCNY {
		return newRefundValidationError(operation, "amount.currency must be %q", DirectRefundCurrencyCNY)
	}
	for index, from := range amount.From {
		if strings.TrimSpace(from.Account) == "" {
			return newRefundValidationError(operation, "amount.from[%d].account is required", index)
		}
		if from.Amount <= 0 {
			return newRefundValidationError(operation, "amount.from[%d].amount must be positive", index)
		}
	}
	return nil
}

func validateDirectRefundCreateAmountContract(operation string, field string, amount *DirectRefundAmount) error {
	if amount == nil {
		return newRefundContractError(operation, "%s is required", field)
	}
	if amount.Refund <= 0 {
		return newRefundContractError(operation, "%s.refund must be positive", field)
	}
	if strings.TrimSpace(amount.Currency) != "" && strings.TrimSpace(amount.Currency) != DirectRefundCurrencyCNY {
		return newRefundContractError(operation, "%s.currency must be %q", field, DirectRefundCurrencyCNY)
	}
	return nil
}

func validateDirectRefundAmountContract(operation string, field string, amount *DirectRefundAmount) error {
	if err := validateDirectRefundCreateAmountContract(operation, field, amount); err != nil {
		return err
	}
	if amount.Total <= 0 {
		return newRefundContractError(operation, "%s.total must be positive", field)
	}
	return nil
}

func newRefundValidationError(operation string, format string, args ...any) error {
	return &RefundValidationError{Message: fmt.Sprintf("%s: %s", operation, fmt.Sprintf(format, args...))}
}

func newRefundContractError(operation string, format string, args ...any) error {
	return &RefundContractError{Message: fmt.Sprintf("%s: %s", operation, fmt.Sprintf(format, args...))}
}

func validateRequiredString(operation string, field string, value string, maxLength int) error {
	if strings.TrimSpace(value) == "" {
		return newRefundValidationError(operation, "%s is required", field)
	}
	if maxLength > 0 && len(strings.TrimSpace(value)) > maxLength {
		return newRefundValidationError(operation, "%s must not exceed %d characters", field, maxLength)
	}
	return nil
}

func validateOptionalMaxLength(operation string, field string, value string, maxLength int) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if len(strings.TrimSpace(value)) > maxLength {
		return newRefundValidationError(operation, "%s must not exceed %d characters", field, maxLength)
	}
	return nil
}

func validateRefundNotifyURL(operation string, value string, required bool) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if required {
			return newRefundValidationError(operation, "notify_url is required")
		}
		return nil
	}
	if len(trimmed) > refundNotifyURLMaxLength {
		return newRefundValidationError(operation, "notify_url must not exceed %d characters", refundNotifyURLMaxLength)
	}
	return nil
}

func validateOptionalEnum(operation string, field string, value string, allowed map[string]struct{}) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if _, ok := allowed[strings.ToUpper(strings.TrimSpace(value))]; !ok {
		return newRefundValidationError(operation, "%s has unsupported value %q", field, value)
	}
	return nil
}

func validateOptionalContractString(operation string, field string, value string, maxLength int) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if maxLength > 0 && len(strings.TrimSpace(value)) > maxLength {
		return newRefundContractError(operation, "%s must not exceed %d characters", field, maxLength)
	}
	return nil
}

func validateRequiredContractString(operation string, field string, value string, maxLength int) error {
	if strings.TrimSpace(value) == "" {
		return newRefundContractError(operation, "%s is required", field)
	}
	if maxLength > 0 && len(strings.TrimSpace(value)) > maxLength {
		return newRefundContractError(operation, "%s must not exceed %d characters", field, maxLength)
	}
	return nil
}

func validateRequiredContractEnum(operation string, field string, value string, allowed map[string]struct{}) error {
	if strings.TrimSpace(value) == "" {
		return newRefundContractError(operation, "%s is required", field)
	}
	if _, ok := allowed[strings.ToUpper(strings.TrimSpace(value))]; !ok {
		return newRefundContractError(operation, "%s has unsupported value %q", field, value)
	}
	return nil
}

func validateOptionalContractEnum(operation string, field string, value string, allowed map[string]struct{}) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if _, ok := allowed[strings.ToUpper(strings.TrimSpace(value))]; !ok {
		return newRefundContractError(operation, "%s has unsupported value %q", field, value)
	}
	return nil
}

func validateRFC3339(operation string, field string, value string, required bool) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if required {
			return newRefundContractError(operation, "%s is required", field)
		}
		return nil
	}
	if _, err := time.Parse(time.RFC3339Nano, trimmed); err != nil {
		return newRefundContractError(operation, "%s must be RFC3339: %v", field, err)
	}
	return nil
}
