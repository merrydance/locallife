package contracts

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	refundIDMaxLength          = 32
	refundOutTradeNoMaxLength  = 32
	refundOutRefundNoMaxLength = 64
	refundReasonMaxLength      = 80
	refundNotifyURLMaxLength   = 256
	refundAppIDMaxLength       = 32
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

var allowedEcommerceRefundAmountAccounts = map[string]struct{}{
	EcommerceRefundAmountAccountAvailable:   {},
	EcommerceRefundAmountAccountUnavailable: {},
}

var allowedEcommerceRefundStatuses = map[string]struct{}{
	EcommerceRefundStatusSuccess:    {},
	EcommerceRefundStatusClosed:     {},
	EcommerceRefundStatusProcessing: {},
	EcommerceRefundStatusAbnormal:   {},
}

var allowedEcommerceRefundNotifyStatuses = map[string]struct{}{
	EcommerceRefundStatusSuccess:  {},
	EcommerceRefundStatusClosed:   {},
	EcommerceRefundStatusAbnormal: {},
}

var allowedEcommerceRefundChannels = map[string]struct{}{
	EcommerceRefundChannelOriginal:      {},
	EcommerceRefundChannelBalance:       {},
	EcommerceRefundChannelOtherBalance:  {},
	EcommerceRefundChannelOtherBankCard: {},
}

var allowedEcommerceRefundPromotionScopes = map[string]struct{}{
	EcommerceRefundPromotionScopeGlobal: {},
	EcommerceRefundPromotionScopeSingle: {},
}

var allowedEcommerceRefundPromotionTypes = map[string]struct{}{
	EcommerceRefundPromotionTypeCoupon:   {},
	EcommerceRefundPromotionTypeDiscount: {},
}

var allowedEcommerceRefundSources = map[string]struct{}{
	EcommerceRefundSourcePartnerAdvance: {},
	EcommerceRefundSourceSubMerchant:    {},
}

var allowedEcommerceRefundRequestFundsAccounts = map[string]struct{}{
	EcommerceRefundFundsAccountAvailable: {},
}

var allowedEcommerceRefundResponseFundsAccounts = map[string]struct{}{
	EcommerceRefundFundsAccountUnsetttled:  {},
	EcommerceRefundFundsAccountAvailable:   {},
	EcommerceRefundFundsAccountUnavailable: {},
	EcommerceRefundFundsAccountOperation:   {},
	EcommerceRefundFundsAccountBasic:       {},
	EcommerceRefundFundsAccountECNYBasic:   {},
}

var allowedEcommerceAbnormalRefundTypes = map[string]struct{}{
	EcommerceAbnormalRefundTypeUserBankCard:     {},
	EcommerceAbnormalRefundTypeMerchantBankCard: {},
}

var allowedEcommerceRefundAdvanceReturnResults = map[string]struct{}{
	EcommerceRefundAdvanceReturnResultSuccess:    {},
	EcommerceRefundAdvanceReturnResultFailed:     {},
	EcommerceRefundAdvanceReturnResultProcessing: {},
}

var allowedDirectRefundRequestFundsAccounts = map[string]struct{}{
	DirectRefundRequestFundsAccountAvailable: {},
	DirectRefundRequestFundsAccountUnsettle:  {},
}

var allowedDirectRefundAmountAccounts = map[string]struct{}{
	DirectRefundFundsAccountAvailable:   {},
	DirectRefundFundsAccountUnavailable: {},
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
	EcommerceRefundPromotionScopeGlobal: {},
	EcommerceRefundPromotionScopeSingle: {},
}

var allowedDirectRefundPromotionTypes = map[string]struct{}{
	"CASH":   {},
	"NOCASH": {},
}

var allowedDirectAbnormalRefundTypes = map[string]struct{}{
	DirectAbnormalRefundTypeUserBankCard:     {},
	DirectAbnormalRefundTypeMerchantBankCard: {},
}

func ValidateEcommerceRefundRequest(req *EcommerceRefundRequest) error {
	if req == nil {
		return newRefundValidationError("create ecommerce refund", "request is nil")
	}
	if err := validateRequiredString("create ecommerce refund", "sub_mchid", req.SubMchID, refundMchIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredString("create ecommerce refund", "sp_appid", req.SpAppID, refundAppIDMaxLength); err != nil {
		return err
	}
	if err := validateOptionalMaxLength("create ecommerce refund", "sub_appid", req.SubAppID, refundAppIDMaxLength); err != nil {
		return err
	}
	if strings.TrimSpace(req.TransactionID) == "" && strings.TrimSpace(req.OutTradeNo) == "" {
		return newRefundValidationError("create ecommerce refund", "transaction_id or out_trade_no is required")
	}
	if err := validateOptionalMaxLength("create ecommerce refund", "transaction_id", req.TransactionID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateOptionalMaxLength("create ecommerce refund", "out_trade_no", req.OutTradeNo, refundOutTradeNoMaxLength); err != nil {
		return err
	}
	if err := validateRequiredString("create ecommerce refund", "out_refund_no", req.OutRefundNo, refundOutRefundNoMaxLength); err != nil {
		return err
	}
	if err := validateOptionalMaxLength("create ecommerce refund", "reason", req.Reason, refundReasonMaxLength); err != nil {
		return err
	}
	if req.Amount == nil {
		return newRefundValidationError("create ecommerce refund", "amount is required")
	}
	if err := validateEcommerceRefundRequestAmount("create ecommerce refund", req.Amount); err != nil {
		return err
	}
	if err := validateRefundNotifyURL("create ecommerce refund", req.NotifyURL, true); err != nil {
		return err
	}
	if err := validateOptionalEnum("create ecommerce refund", "refund_account", req.RefundAccount, allowedEcommerceRefundSources); err != nil {
		return err
	}
	if err := validateOptionalEnum("create ecommerce refund", "funds_account", req.FundsAccount, allowedEcommerceRefundRequestFundsAccounts); err != nil {
		return err
	}
	return nil
}

func ValidateEcommerceRefundCreateResponse(operation string, resp *EcommerceRefundCreateResponse) error {
	if resp == nil {
		return newRefundContractError(operation, "empty wechat response")
	}
	if err := validateRequiredContractString(operation, "refund_id", resp.RefundID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "out_refund_no", resp.OutRefundNo, refundOutRefundNoMaxLength); err != nil {
		return err
	}
	if err := validateRFC3339(operation, "create_time", resp.CreateTime, true); err != nil {
		return err
	}
	if err := validateEcommerceRefundAmountContract(operation, "amount", &resp.Amount, false); err != nil {
		return err
	}
	if err := validateEcommerceRefundPromotionDetails(operation, resp.PromotionDetail); err != nil {
		return err
	}
	if err := validateOptionalContractEnum(operation, "refund_account", resp.RefundAccount, allowedEcommerceRefundSources); err != nil {
		return err
	}
	return nil
}

func ValidateEcommerceRefundQueryByIDInput(refundID, subMchID string) (string, string, error) {
	trimmedRefundID := strings.TrimSpace(refundID)
	if trimmedRefundID == "" {
		return "", "", newRefundValidationError("query ecommerce refund by refund_id", "refund_id is required")
	}
	if len(trimmedRefundID) > refundIDMaxLength {
		return "", "", newRefundValidationError("query ecommerce refund by refund_id", "refund_id must not exceed %d characters", refundIDMaxLength)
	}
	trimmedSubMchID := strings.TrimSpace(subMchID)
	if trimmedSubMchID == "" {
		return "", "", newRefundValidationError("query ecommerce refund by refund_id", "sub_mchid is required")
	}
	if len(trimmedSubMchID) > refundMchIDMaxLength {
		return "", "", newRefundValidationError("query ecommerce refund by refund_id", "sub_mchid must not exceed %d characters", refundMchIDMaxLength)
	}
	return trimmedRefundID, trimmedSubMchID, nil
}

func ValidateEcommerceRefundQueryByOutRefundNoInput(outRefundNo, subMchID string) (string, string, error) {
	trimmedOutRefundNo := strings.TrimSpace(outRefundNo)
	if trimmedOutRefundNo == "" {
		return "", "", newRefundValidationError("query ecommerce refund by out_refund_no", "out_refund_no is required")
	}
	if len(trimmedOutRefundNo) > refundOutRefundNoMaxLength {
		return "", "", newRefundValidationError("query ecommerce refund by out_refund_no", "out_refund_no must not exceed %d characters", refundOutRefundNoMaxLength)
	}
	trimmedSubMchID := strings.TrimSpace(subMchID)
	if trimmedSubMchID == "" {
		return "", "", newRefundValidationError("query ecommerce refund by out_refund_no", "sub_mchid is required")
	}
	if len(trimmedSubMchID) > refundMchIDMaxLength {
		return "", "", newRefundValidationError("query ecommerce refund by out_refund_no", "sub_mchid must not exceed %d characters", refundMchIDMaxLength)
	}
	return trimmedOutRefundNo, trimmedSubMchID, nil
}

func ValidateEcommerceRefundQueryResponse(operation string, resp *EcommerceRefundQueryResponse) error {
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
	if err := validateOptionalContractEnum(operation, "channel", resp.Channel, allowedEcommerceRefundChannels); err != nil {
		return err
	}
	if err := validateRFC3339(operation, "success_time", resp.SuccessTime, false); err != nil {
		return err
	}
	if err := validateRFC3339(operation, "create_time", resp.CreateTime, true); err != nil {
		return err
	}
	if err := validateRequiredContractEnum(operation, "status", resp.Status, allowedEcommerceRefundStatuses); err != nil {
		return err
	}
	if err := validateEcommerceRefundAmountContract(operation, "amount", &resp.Amount, false); err != nil {
		return err
	}
	if err := validateEcommerceRefundPromotionDetails(operation, resp.PromotionDetail); err != nil {
		return err
	}
	if err := validateOptionalContractEnum(operation, "refund_account", resp.RefundAccount, allowedEcommerceRefundSources); err != nil {
		return err
	}
	if err := validateOptionalContractEnum(operation, "funds_account", resp.FundsAccount, allowedEcommerceRefundResponseFundsAccounts); err != nil {
		return err
	}
	return nil
}

func ValidateEcommerceAbnormalRefundRequest(req *EcommerceAbnormalRefundRequest) error {
	if req == nil {
		return newRefundValidationError("apply ecommerce abnormal refund", "request is nil")
	}
	if err := validateRequiredString("apply ecommerce abnormal refund", "refund_id", req.RefundID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredString("apply ecommerce abnormal refund", "sub_mchid", req.SubMchID, refundMchIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredString("apply ecommerce abnormal refund", "out_refund_no", req.OutRefundNo, refundOutRefundNoMaxLength); err != nil {
		return err
	}
	if err := validateRequiredEnum("apply ecommerce abnormal refund", "type", req.Type, allowedEcommerceAbnormalRefundTypes); err != nil {
		return err
	}
	switch strings.TrimSpace(req.Type) {
	case EcommerceAbnormalRefundTypeUserBankCard:
		if strings.TrimSpace(req.BankType) == "" {
			return newRefundValidationError("apply ecommerce abnormal refund", "bank_type is required for type=%s", EcommerceAbnormalRefundTypeUserBankCard)
		}
		if strings.TrimSpace(req.BankAccount) == "" {
			return newRefundValidationError("apply ecommerce abnormal refund", "bank_account is required for type=%s", EcommerceAbnormalRefundTypeUserBankCard)
		}
		if strings.TrimSpace(req.RealName) == "" {
			return newRefundValidationError("apply ecommerce abnormal refund", "real_name is required for type=%s", EcommerceAbnormalRefundTypeUserBankCard)
		}
	case EcommerceAbnormalRefundTypeMerchantBankCard:
		if strings.TrimSpace(req.BankType) != "" || strings.TrimSpace(req.BankAccount) != "" || strings.TrimSpace(req.RealName) != "" {
			return newRefundValidationError("apply ecommerce abnormal refund", "bank_type, bank_account and real_name are not allowed for type=%s", EcommerceAbnormalRefundTypeMerchantBankCard)
		}
	}
	return nil
}

func ValidateEcommerceRefundNotification(operation string, resp *EcommerceRefundNotification) error {
	if resp == nil {
		return newRefundContractError(operation, "empty wechat response")
	}
	if err := validateRequiredContractString(operation, "sp_mchid", resp.SPMchID, refundMchIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "sub_mchid", resp.SubMchID, refundMchIDMaxLength); err != nil {
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
	if err := validateRequiredContractEnum(operation, "refund_status", resp.RefundStatus, allowedEcommerceRefundNotifyStatuses); err != nil {
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
	if err := validateOptionalContractEnum(operation, "refund_account", resp.RefundAccount, allowedEcommerceRefundSources); err != nil {
		return err
	}
	return nil
}

func ValidateEcommerceRefundAdvanceReturnQueryRequest(req *EcommerceRefundAdvanceReturnQueryRequest) error {
	if req == nil {
		return newRefundValidationError("query ecommerce refund advance return", "request is nil")
	}
	if err := validateRequiredString("query ecommerce refund advance return", "refund_id", req.RefundID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredString("query ecommerce refund advance return", "sub_mchid", req.SubMchID, refundMchIDMaxLength); err != nil {
		return err
	}
	return nil
}

func ValidateEcommerceRefundAdvanceReturnRequest(req *EcommerceRefundAdvanceReturnRequest) error {
	if req == nil {
		return newRefundValidationError("create ecommerce refund advance return", "request is nil")
	}
	if err := validateRequiredString("create ecommerce refund advance return", "refund_id", req.RefundID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredString("create ecommerce refund advance return", "sub_mchid", req.SubMchID, refundMchIDMaxLength); err != nil {
		return err
	}
	return nil
}

func ValidateEcommerceRefundAdvanceReturnResponse(operation string, resp *EcommerceRefundAdvanceReturnResponse) error {
	if resp == nil {
		return newRefundContractError(operation, "empty wechat response")
	}
	if err := validateRequiredContractString(operation, "refund_id", resp.RefundID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "advance_return_id", resp.AdvanceReturnID, 0); err != nil {
		return err
	}
	if resp.ReturnAmount <= 0 {
		return newRefundContractError(operation, "return_amount must be positive")
	}
	if err := validateRequiredContractString(operation, "payer_mchid", resp.PayerMchID, refundMchIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "payer_account", resp.PayerAccount, 0); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "payee_mchid", resp.PayeeMchID, refundMchIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredContractString(operation, "payee_account", resp.PayeeAccount, 0); err != nil {
		return err
	}
	if err := validateRequiredContractEnum(operation, "result", resp.Result, allowedEcommerceRefundAdvanceReturnResults); err != nil {
		return err
	}
	if err := validateRFC3339(operation, "success_time", resp.SuccessTime, false); err != nil {
		return err
	}
	return nil
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
		for goodsIndex, goods := range detail.GoodsDetail {
			if strings.TrimSpace(goods.MerchantGoodsID) == "" {
				return newRefundContractError(operation, "promotion_detail[%d].goods_detail[%d].merchant_goods_id is required", index, goodsIndex)
			}
			if goods.UnitPrice <= 0 {
				return newRefundContractError(operation, "promotion_detail[%d].goods_detail[%d].unit_price must be positive", index, goodsIndex)
			}
			if goods.RefundAmount <= 0 {
				return newRefundContractError(operation, "promotion_detail[%d].goods_detail[%d].refund_amount must be positive", index, goodsIndex)
			}
			if goods.RefundQuantity <= 0 {
				return newRefundContractError(operation, "promotion_detail[%d].goods_detail[%d].refund_quantity must be positive", index, goodsIndex)
			}
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
		for goodsIndex, goods := range detail.GoodsDetail {
			if strings.TrimSpace(goods.MerchantGoodsID) == "" {
				return newRefundContractError(operation, "promotion_detail[%d].goods_detail[%d].merchant_goods_id is required", index, goodsIndex)
			}
			if goods.UnitPrice <= 0 {
				return newRefundContractError(operation, "promotion_detail[%d].goods_detail[%d].unit_price must be positive", index, goodsIndex)
			}
			if goods.RefundAmount <= 0 {
				return newRefundContractError(operation, "promotion_detail[%d].goods_detail[%d].refund_amount must be positive", index, goodsIndex)
			}
			if goods.RefundQuantity <= 0 {
				return newRefundContractError(operation, "promotion_detail[%d].goods_detail[%d].refund_quantity must be positive", index, goodsIndex)
			}
		}
	}
	return nil
}

func ValidateDirectAbnormalRefundRequest(req *DirectAbnormalRefundRequest) error {
	if req == nil {
		return newRefundValidationError("apply direct abnormal refund", "request is nil")
	}
	if err := validateRequiredString("apply direct abnormal refund", "refund_id", req.RefundID, refundIDMaxLength); err != nil {
		return err
	}
	if err := validateRequiredString("apply direct abnormal refund", "out_refund_no", req.OutRefundNo, refundOutRefundNoMaxLength); err != nil {
		return err
	}
	if err := validateRequiredEnum("apply direct abnormal refund", "type", req.Type, allowedDirectAbnormalRefundTypes); err != nil {
		return err
	}
	switch strings.TrimSpace(req.Type) {
	case DirectAbnormalRefundTypeUserBankCard:
		if strings.TrimSpace(req.BankType) == "" {
			return newRefundValidationError("apply direct abnormal refund", "bank_type is required for type=%s", DirectAbnormalRefundTypeUserBankCard)
		}
		if strings.TrimSpace(req.BankAccount) == "" {
			return newRefundValidationError("apply direct abnormal refund", "bank_account is required for type=%s", DirectAbnormalRefundTypeUserBankCard)
		}
		if strings.TrimSpace(req.RealName) == "" {
			return newRefundValidationError("apply direct abnormal refund", "real_name is required for type=%s", DirectAbnormalRefundTypeUserBankCard)
		}
	case DirectAbnormalRefundTypeMerchantBankCard:
		if strings.TrimSpace(req.BankType) != "" || strings.TrimSpace(req.BankAccount) != "" || strings.TrimSpace(req.RealName) != "" {
			return newRefundValidationError("apply direct abnormal refund", "bank_type, bank_account and real_name are not allowed for type=%s", DirectAbnormalRefundTypeMerchantBankCard)
		}
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

func validateEcommerceRefundRequestAmount(operation string, amount *EcommerceRefundRequestAmount) error {
	if amount.Refund <= 0 {
		return newRefundValidationError(operation, "amount.refund must be positive")
	}
	if amount.Total <= 0 {
		return newRefundValidationError(operation, "amount.total must be positive")
	}
	if err := validateOptionalEnum(operation, "amount.currency", amount.Currency, map[string]struct{}{EcommerceRefundCurrencyCNY: {}}); err != nil {
		return err
	}
	for index, from := range amount.From {
		if err := validateRequiredEnum(operation, fmt.Sprintf("amount.from[%d].account", index), from.Account, allowedEcommerceRefundAmountAccounts); err != nil {
			return err
		}
		if from.Amount <= 0 {
			return newRefundValidationError(operation, "amount.from[%d].amount must be positive", index)
		}
	}
	return nil
}

func validateEcommerceRefundAmountContract(operation, field string, amount *EcommerceRefundAmount, requireAdvance bool) error {
	if amount == nil {
		return newRefundContractError(operation, "%s is required", field)
	}
	if amount.Refund <= 0 {
		return newRefundContractError(operation, "%s.refund must be positive", field)
	}
	for index, from := range amount.From {
		if err := validateRequiredContractEnum(operation, fmt.Sprintf("%s.from[%d].account", field, index), from.Account, allowedEcommerceRefundAmountAccounts); err != nil {
			return err
		}
		if from.Amount <= 0 {
			return newRefundContractError(operation, "%s.from[%d].amount must be positive", field, index)
		}
	}
	if amount.PayerRefund <= 0 {
		return newRefundContractError(operation, "%s.payer_refund must be positive", field)
	}
	if amount.DiscountRefund < 0 {
		return newRefundContractError(operation, "%s.discount_refund must be non-negative", field)
	}
	if err := validateOptionalContractEnum(operation, field+".currency", amount.Currency, map[string]struct{}{EcommerceRefundCurrencyCNY: {}}); err != nil {
		return err
	}
	if requireAdvance && amount.Advance < 0 {
		return newRefundContractError(operation, "%s.advance must be non-negative", field)
	}
	return nil
}

func validateEcommerceRefundPromotionDetails(operation string, details []EcommerceRefundPromotionDetail) error {
	for index, detail := range details {
		if strings.TrimSpace(detail.PromotionID) == "" {
			return newRefundContractError(operation, "promotion_detail[%d].promotion_id is required", index)
		}
		if err := validateRequiredContractEnum(operation, fmt.Sprintf("promotion_detail[%d].scope", index), detail.Scope, allowedEcommerceRefundPromotionScopes); err != nil {
			return err
		}
		if err := validateRequiredContractEnum(operation, fmt.Sprintf("promotion_detail[%d].type", index), detail.Type, allowedEcommerceRefundPromotionTypes); err != nil {
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
		if err := validateRequiredEnum(operation, fmt.Sprintf("amount.from[%d].account", index), from.Account, allowedDirectRefundAmountAccounts); err != nil {
			return err
		}
		if from.Amount <= 0 {
			return newRefundValidationError(operation, "amount.from[%d].amount must be positive", index)
		}
	}
	return nil
}

func validateDirectRefundAmountContract(operation, field string, amount *DirectRefundAmount) error {
	if amount == nil {
		return newRefundContractError(operation, "%s is required", field)
	}
	if amount.Total <= 0 {
		return newRefundContractError(operation, "%s.total must be positive", field)
	}
	if amount.Refund <= 0 {
		return newRefundContractError(operation, "%s.refund must be positive", field)
	}
	for index, from := range amount.From {
		if err := validateRequiredContractEnum(operation, fmt.Sprintf("%s.from[%d].account", field, index), from.Account, allowedDirectRefundAmountAccounts); err != nil {
			return err
		}
		if from.Amount <= 0 {
			return newRefundContractError(operation, "%s.from[%d].amount must be positive", field, index)
		}
	}
	if amount.PayerTotal <= 0 {
		return newRefundContractError(operation, "%s.payer_total must be positive", field)
	}
	if amount.PayerRefund <= 0 {
		return newRefundContractError(operation, "%s.payer_refund must be positive", field)
	}
	if amount.SettlementRefund <= 0 {
		return newRefundContractError(operation, "%s.settlement_refund must be positive", field)
	}
	if amount.SettlementTotal <= 0 {
		return newRefundContractError(operation, "%s.settlement_total must be positive", field)
	}
	if amount.DiscountRefund <= 0 {
		return newRefundContractError(operation, "%s.discount_refund must be positive", field)
	}
	if strings.TrimSpace(amount.Currency) != DirectRefundCurrencyCNY {
		return newRefundContractError(operation, "%s.currency must be %q", field, DirectRefundCurrencyCNY)
	}
	if amount.RefundFee < 0 {
		return newRefundContractError(operation, "%s.refund_fee must be non-negative", field)
	}
	return nil
}

func validateDirectRefundCreateAmountContract(operation, field string, amount *DirectRefundAmount) error {
	if amount == nil {
		return newRefundContractError(operation, "%s is required", field)
	}
	if amount.Refund <= 0 {
		return newRefundContractError(operation, "%s.refund must be positive", field)
	}
	for index, from := range amount.From {
		if err := validateRequiredContractEnum(operation, fmt.Sprintf("%s.from[%d].account", field, index), from.Account, allowedDirectRefundAmountAccounts); err != nil {
			return err
		}
		if from.Amount <= 0 {
			return newRefundContractError(operation, "%s.from[%d].amount must be positive", field, index)
		}
	}
	if amount.Total < 0 {
		return newRefundContractError(operation, "%s.total must be non-negative", field)
	}
	if amount.PayerTotal < 0 {
		return newRefundContractError(operation, "%s.payer_total must be non-negative", field)
	}
	if amount.PayerRefund < 0 {
		return newRefundContractError(operation, "%s.payer_refund must be non-negative", field)
	}
	if amount.SettlementRefund < 0 {
		return newRefundContractError(operation, "%s.settlement_refund must be non-negative", field)
	}
	if amount.SettlementTotal < 0 {
		return newRefundContractError(operation, "%s.settlement_total must be non-negative", field)
	}
	if amount.DiscountRefund < 0 {
		return newRefundContractError(operation, "%s.discount_refund must be non-negative", field)
	}
	if trimmedCurrency := strings.TrimSpace(amount.Currency); trimmedCurrency != "" && trimmedCurrency != DirectRefundCurrencyCNY {
		return newRefundContractError(operation, "%s.currency must be %q", field, DirectRefundCurrencyCNY)
	}
	if amount.RefundFee < 0 {
		return newRefundContractError(operation, "%s.refund_fee must be non-negative", field)
	}
	return nil
}

func validateRefundNotifyURL(operation, raw string, disallowQuery bool) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) > refundNotifyURLMaxLength {
		return newRefundValidationError(operation, "notify_url must not exceed %d characters", refundNotifyURLMaxLength)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return newRefundValidationError(operation, "notify_url must be a valid absolute URL")
	}
	if disallowQuery && (parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "") {
		return newRefundValidationError(operation, "notify_url must not contain query parameters or fragment")
	}
	return nil
}

func validateRFC3339(operation, field, value string, required bool) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if required {
			return newRefundContractError(operation, "%s is required", field)
		}
		return nil
	}
	if _, err := time.Parse(time.RFC3339, trimmed); err != nil {
		return newRefundContractError(operation, "%s must be RFC3339: %v", field, err)
	}
	return nil
}

func validateRequiredString(operation, field, value string, maxLength int) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return newRefundValidationError(operation, "%s is required", field)
	}
	if maxLength > 0 && len(trimmed) > maxLength {
		return newRefundValidationError(operation, "%s must not exceed %d characters", field, maxLength)
	}
	return nil
}

func validateOptionalMaxLength(operation, field, value string, maxLength int) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if maxLength > 0 && len(trimmed) > maxLength {
		return newRefundValidationError(operation, "%s must not exceed %d characters", field, maxLength)
	}
	return nil
}

func validateRequiredEnum(operation, field, value string, allowed map[string]struct{}) error {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return newRefundValidationError(operation, "%s is required", field)
	}
	if _, ok := allowed[trimmed]; !ok {
		return newRefundValidationError(operation, "%s has unsupported value %q", field, value)
	}
	return nil
}

func validateOptionalEnum(operation, field, value string, allowed map[string]struct{}) error {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return nil
	}
	if _, ok := allowed[trimmed]; !ok {
		return newRefundValidationError(operation, "%s has unsupported value %q", field, value)
	}
	return nil
}

func validateRequiredContractString(operation, field, value string, maxLength int) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return newRefundContractError(operation, "%s is required", field)
	}
	if maxLength > 0 && len(trimmed) > maxLength {
		return newRefundContractError(operation, "%s must not exceed %d characters", field, maxLength)
	}
	return nil
}

func validateOptionalContractString(operation, field, value string, maxLength int) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if maxLength > 0 && len(trimmed) > maxLength {
		return newRefundContractError(operation, "%s must not exceed %d characters", field, maxLength)
	}
	return nil
}

func validateRequiredContractEnum(operation, field, value string, allowed map[string]struct{}) error {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return newRefundContractError(operation, "%s is required", field)
	}
	if _, ok := allowed[trimmed]; !ok {
		return newRefundContractError(operation, "%s has unsupported value %q", field, value)
	}
	return nil
}

func validateOptionalContractEnum(operation, field, value string, allowed map[string]struct{}) error {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return nil
	}
	if _, ok := allowed[trimmed]; !ok {
		return newRefundContractError(operation, "%s has unsupported value %q", field, value)
	}
	return nil
}

func newRefundValidationError(operation, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "refund"
	}
	return &RefundValidationError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}

func newRefundContractError(operation, format string, args ...any) error {
	prefix := strings.TrimSpace(operation)
	if prefix == "" {
		prefix = "refund"
	}
	return &RefundContractError{Message: fmt.Sprintf("%s: %s", prefix, fmt.Sprintf(format, args...))}
}
