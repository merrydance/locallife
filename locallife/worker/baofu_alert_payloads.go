package worker

import (
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	AlertTypeBaofuPaymentCallbackMissing  AlertType = "BAOFU_PAYMENT_CALLBACK_MISSING"
	AlertTypeBaofuShareProcessingSLA      AlertType = "BAOFU_SHARE_PROCESSING_SLA"
	AlertTypeBaofuWithdrawalProcessingSLA AlertType = "BAOFU_WITHDRAWAL_PROCESSING_SLA"
	AlertTypeBaofuFactApplicationFailed   AlertType = "BAOFU_FACT_APPLICATION_FAILED"
	AlertTypeBaofuFeeLedgerMismatch       AlertType = "BAOFU_FEE_LEDGER_MISMATCH"
)

func newBaofuPaymentCallbackMissingAlert(paymentOrder db.PaymentOrder, sla time.Duration) AlertData {
	return AlertData{
		AlertType:   AlertTypeBaofuPaymentCallbackMissing,
		Level:       AlertLevelWarning,
		Title:       "宝付支付回调超时",
		Message:     "宝付支付订单超过预期时间未收到终态回调，请查询上游支付状态并推进事实应用。",
		RelatedID:   paymentOrder.ID,
		RelatedType: "payment_order",
		Extra: baofuReconciliationAlertExtra(map[string]interface{}{
			"payment_order_id": paymentOrder.ID,
			"status":           paymentOrder.Status,
			"created_at":       paymentOrder.CreatedAt.Format(time.RFC3339),
			"sla_seconds":      int64(sla.Seconds()),
		}),
	}
}

func newBaofuProfitSharingProcessingSLAAlert(order db.ProfitSharingOrder, sla time.Duration) AlertData {
	return AlertData{
		AlertType:   AlertTypeBaofuShareProcessingSLA,
		Level:       AlertLevelWarning,
		Title:       "宝付分账处理中超时",
		Message:     "宝付分账订单长时间处于处理中，请查询分账结果并推进事实应用。",
		RelatedID:   order.ID,
		RelatedType: "profit_sharing_order",
		Extra: baofuReconciliationAlertExtra(map[string]interface{}{
			"profit_sharing_order_id": order.ID,
			"payment_order_id":        order.PaymentOrderID,
			"status":                  order.Status,
			"created_at":              order.CreatedAt.Format(time.RFC3339),
			"sla_seconds":             int64(sla.Seconds()),
		}),
	}
}

func newBaofuWithdrawalProcessingSLAAlert(order db.BaofuWithdrawalOrder, sla time.Duration) AlertData {
	return AlertData{
		AlertType:   AlertTypeBaofuWithdrawalProcessingSLA,
		Level:       AlertLevelWarning,
		Title:       "宝付提现处理中超时",
		Message:     "宝付提现订单长时间处于处理中，请查询提现结果并推进事实应用。",
		RelatedID:   order.ID,
		RelatedType: "baofu_withdrawal_order",
		Extra: baofuReconciliationAlertExtra(map[string]interface{}{
			"withdrawal_order_id": order.ID,
			"owner_type":          order.OwnerType,
			"owner_id":            order.OwnerID,
			"status":              order.Status,
			"amount":              order.Amount,
			"created_at":          order.CreatedAt.Format(time.RFC3339),
			"sla_seconds":         int64(sla.Seconds()),
		}),
	}
}

func newBaofuFailedFactAlert(fact db.ExternalPaymentFact) AlertData {
	return AlertData{
		AlertType:   AlertTypeBaofuFactApplicationFailed,
		Level:       AlertLevelWarning,
		Title:       "宝付支付事实应用失败",
		Message:     "宝付支付事实进入失败状态，请检查对应业务对象并重新推进应用。",
		RelatedID:   fact.ID,
		RelatedType: "external_payment_fact",
		Extra: baofuReconciliationAlertExtra(map[string]interface{}{
			"fact_id":              fact.ID,
			"capability":           fact.Capability,
			"external_object_type": fact.ExternalObjectType,
			"terminal_status":      fact.TerminalStatus,
			"processing_status":    fact.ProcessingStatus,
		}),
	}
}

func newBaofuFeeLedgerMismatchAlert(profitSharingOrderID int64, paymentOrderID int64, expectedFee int64, ledgerFee int64) AlertData {
	return AlertData{
		AlertType:   AlertTypeBaofuFeeLedgerMismatch,
		Level:       AlertLevelWarning,
		Title:       "宝付手续费台账不一致",
		Message:     "宝付手续费台账金额与分账订单快照不一致，请核对费用台账后再进行日终确认。",
		RelatedID:   profitSharingOrderID,
		RelatedType: "profit_sharing_order",
		Extra: baofuReconciliationAlertExtra(map[string]interface{}{
			"profit_sharing_order_id": profitSharingOrderID,
			"payment_order_id":        paymentOrderID,
			"expected_payment_fee":    expectedFee,
			"ledger_payment_fee":      ledgerFee,
		}),
	}
}
