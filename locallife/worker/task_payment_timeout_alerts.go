package worker

import (
	"context"
	"fmt"

	db "github.com/merrydance/locallife/db/sqlc"
)

func (p *RedisTaskProcessor) publishPaymentTimeoutRemoteAmountMismatchAlert(ctx context.Context, paymentOrder db.PaymentOrder, remoteAmount int64, remoteState string) {
	p.publishAlert(ctx, AlertData{
		AlertType:   AlertTypePaymentTimeout,
		Level:       AlertLevelCritical,
		Title:       "支付超时扫描发现远端支付金额不一致",
		Message:     fmt.Sprintf("支付单 %s 超时扫描发现微信侧状态为 %s，但远端金额 %d 与本地金额 %d 不一致，系统已停止自动关单。", paymentOrder.OutTradeNo, remoteState, remoteAmount, paymentOrder.Amount),
		RelatedID:   paymentOrder.ID,
		RelatedType: "payment_order",
		Extra: map[string]interface{}{
			"out_trade_no":    paymentOrder.OutTradeNo,
			"remote_state":    remoteState,
			"expected_amount": paymentOrder.Amount,
			"actual_amount":   remoteAmount,
		},
	})
}

func (p *RedisTaskProcessor) publishPaymentTimeoutUnexpectedRemoteStateAlert(ctx context.Context, paymentOrder db.PaymentOrder, remoteState string) {
	p.publishAlert(ctx, AlertData{
		AlertType:   AlertTypePaymentTimeout,
		Level:       AlertLevelCritical,
		Title:       "支付超时扫描遇到异常远端状态",
		Message:     fmt.Sprintf("支付单 %s 超时扫描发现微信侧状态为 %s，系统已停止自动关单，请人工核对。", paymentOrder.OutTradeNo, remoteState),
		RelatedID:   paymentOrder.ID,
		RelatedType: "payment_order",
		Extra: map[string]interface{}{
			"out_trade_no": paymentOrder.OutTradeNo,
			"remote_state": remoteState,
		},
	})
}
