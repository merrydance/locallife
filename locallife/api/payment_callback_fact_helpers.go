package api

import (
	"encoding/json"
	"strings"
	"time"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"

	"github.com/rs/zerolog/log"
)

func paymentFactStringPtr(value string) *string {
	return &value
}

func paymentFactInt64Ptr(value int64) *int64 {
	return &value
}

func parseWechatFactTime(value string) *time.Time {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		log.Warn().Err(err).Str("wechat_time", value).Msg("parse wechat payment fact time failed")
		return nil
	}
	return &parsed
}

func directPaymentFactResource(resource *wechatcontracts.DirectPaymentNotificationResource) []byte {
	if resource == nil {
		return nil
	}
	raw, err := json.Marshal(map[string]any{
		"appid":            resource.AppID,
		"mchid":            resource.MchID,
		"out_trade_no":     resource.OutTradeNo,
		"transaction_id":   resource.TransactionID,
		"trade_type":       resource.TradeType,
		"trade_state":      resource.TradeState,
		"trade_state_desc": resource.TradeStateDesc,
		"amount_total":     resource.Amount.Total,
		"success_time":     resource.SuccessTime,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_trade_no", resource.OutTradeNo).Msg("marshal direct payment fact resource failed")
		return nil
	}
	return raw
}

func directRefundFactResource(resource *wechatcontracts.DirectRefundNotificationResource) []byte {
	if resource == nil {
		return nil
	}
	raw, err := json.Marshal(map[string]any{
		"mchid":          resource.MchID,
		"out_trade_no":   resource.OutTradeNo,
		"transaction_id": resource.TransactionID,
		"out_refund_no":  resource.OutRefundNo,
		"refund_id":      resource.RefundID,
		"refund_status":  resource.RefundStatus,
		"amount_total":   resource.Amount.Total,
		"amount_refund":  resource.Amount.Refund,
		"success_time":   resource.SuccessTime,
	})
	if err != nil {
		log.Warn().Err(err).Str("out_refund_no", resource.OutRefundNo).Msg("marshal direct refund fact resource failed")
		return nil
	}
	return raw
}
