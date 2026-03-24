package worker

import (
	"context"

	db "github.com/merrydance/locallife/db/sqlc"
)

func mergeAlertExtra(base map[string]interface{}, extras map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(base)+len(extras))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extras {
		merged[key] = value
	}
	return merged
}

func paymentOrderAlertExtra(paymentOrder db.PaymentOrder, merchantID int64) map[string]interface{} {
	extra := map[string]interface{}{
		"payment_order_id": paymentOrder.ID,
		"out_trade_no":     paymentOrder.OutTradeNo,
		"payment_type":     paymentOrder.PaymentType,
		"business_type":    paymentOrder.BusinessType,
	}

	if paymentOrder.TransactionID.Valid {
		extra["transaction_id"] = paymentOrder.TransactionID.String
	}
	if paymentOrder.OrderID.Valid {
		extra["order_id"] = paymentOrder.OrderID.Int64
	}
	if paymentOrder.ReservationID.Valid {
		extra["reservation_id"] = paymentOrder.ReservationID.Int64
	}
	if merchantID > 0 {
		extra["merchant_id"] = merchantID
	}

	return extra
}

func refundOrderAlertExtra(paymentOrder db.PaymentOrder, refundOrder db.RefundOrder, merchantID int64, extras map[string]interface{}) map[string]interface{} {
	base := paymentOrderAlertExtra(paymentOrder, merchantID)
	base["refund_order_id"] = refundOrder.ID
	base["out_refund_no"] = refundOrder.OutRefundNo
	base["refund_amount"] = refundOrder.RefundAmount
	base["refund_type"] = refundOrder.RefundType
	if refundOrder.RefundID.Valid {
		base["refund_id"] = refundOrder.RefundID.String
	}

	return mergeAlertExtra(base, extras)
}

func profitSharingOrderAlertExtra(order db.ProfitSharingOrder, extras map[string]interface{}) map[string]interface{} {
	base := map[string]interface{}{
		"profit_sharing_order_id": order.ID,
		"payment_order_id":        order.PaymentOrderID,
		"merchant_id":             order.MerchantID,
		"out_order_no":            order.OutOrderNo,
		"total_amount":            order.TotalAmount,
		"merchant_amount":         order.MerchantAmount,
		"platform_commission":     order.PlatformCommission,
		"operator_commission":     order.OperatorCommission,
		"rider_amount":            order.RiderAmount,
	}
	if order.OperatorID.Valid {
		base["operator_id"] = order.OperatorID.Int64
	}
	if order.RiderID.Valid {
		base["rider_id"] = order.RiderID.Int64
	}

	return mergeAlertExtra(base, extras)
}

func withdrawalAlertExtra(record db.WithdrawalRecord, accountInfo merchantWithdrawAccountInfo, extras map[string]interface{}) map[string]interface{} {
	base := map[string]interface{}{
		"withdrawal_record_id": record.ID,
		"user_id":              record.UserID,
		"amount":               record.Amount,
		"channel":              record.Channel,
		"out_request_no":       accountInfo.OutRequestNo,
		"sub_mch_id":           accountInfo.SubMchID,
	}
	if accountInfo.MerchantID > 0 {
		base["merchant_id"] = accountInfo.MerchantID
	}
	if accountInfo.OperatorID > 0 {
		base["operator_id"] = accountInfo.OperatorID
	}
	if accountInfo.WithdrawID != "" {
		base["withdraw_id"] = accountInfo.WithdrawID
	}
	if accountInfo.Remark != "" {
		base["remark"] = accountInfo.Remark
	}

	return mergeAlertExtra(base, extras)
}

func (processor *RedisTaskProcessor) resolveMerchantIDByPaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder) (int64, error) {
	if paymentOrder.OrderID.Valid {
		order, err := processor.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
		if err != nil {
			return 0, err
		}
		return order.MerchantID, nil
	}
	if paymentOrder.ReservationID.Valid {
		reservation, err := processor.store.GetTableReservation(ctx, paymentOrder.ReservationID.Int64)
		if err != nil {
			return 0, err
		}
		return reservation.MerchantID, nil
	}

	return 0, nil
}
