package logic

import (
	"strings"

	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
)

func mapBaofuAggregatePaymentWechatOrder(resp *aggregatecontracts.UnifiedOrderResult) *QueryPaymentOrderWechatOrder {
	if resp == nil {
		return nil
	}
	return &QueryPaymentOrderWechatOrder{
		SubMchID:       resp.MerchantID,
		OutTradeNo:     resp.OutTradeNo,
		TransactionID:  resp.TradeNo,
		TradeState:     resp.TxnState,
		TradeStateDesc: baofuPaymentTradeStateDesc(resp.TxnState),
		SuccessTime:    resp.FinishTime,
		Amount: QueryPaymentOrderWechatAmount{
			Total:         resp.SuccessAmountFen,
			PayerTotal:    resp.SuccessAmountFen,
			Currency:      "CNY",
			PayerCurrency: "CNY",
		},
	}
}

func baofuPaymentTradeStateDesc(state string) string {
	switch strings.TrimSpace(state) {
	case aggregatecontracts.PaymentStateSuccess:
		return "支付成功"
	case aggregatecontracts.PaymentStateClosed:
		return "已关闭"
	case aggregatecontracts.PaymentStateWaitPaying:
		return "待支付"
	case aggregatecontracts.PaymentStatePayError:
		return "支付失败"
	case aggregatecontracts.PaymentStateRefund:
		return "已退款"
	case aggregatecontracts.PaymentStateAbnormal:
		return "支付状态异常"
	default:
		return ""
	}
}
