package worker

import (
	"context"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

func (s *RefundRecoveryScheduler) recoverPendingOrderRefunds(ctx context.Context) {
	pendingOrderRefundOrders, err := s.store.ListPendingOrderRefundOrdersForRecovery(ctx, db.ListPendingOrderRefundOrdersForRecoveryParams{
		CreatedBefore: time.Now().Add(-1 * time.Minute),
		Limit:         refundRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list pending order refund orders for recovery failed")
		return
	}

	for _, refundOrder := range pendingOrderRefundOrders {
		if !refundOrder.OrderID.Valid {
			continue
		}

		reason := "系统自动退款补偿（订单退款）"
		if refundOrder.RefundReason.Valid && refundOrder.RefundReason.String != "" {
			reason = refundOrder.RefundReason.String
		}

		err = s.distributor.DistributeTaskProcessRefund(ctx, &PayloadProcessRefund{
			PaymentOrderID: refundOrder.PaymentOrderID,
			OrderID:        refundOrder.OrderID.Int64,
			RefundAmount:   refundOrder.RefundAmount,
			Reason:         reason,
			OutRefundNo:    refundOrder.OutRefundNo,
		})
		if err != nil {
			log.Error().Err(err).
				Int64("refund_order_id", refundOrder.ID).
				Int64("payment_order_id", refundOrder.PaymentOrderID).
				Str("business_type", refundOrder.BusinessType).
				Msg("enqueue pending order refund recovery task failed")
			continue
		}

		log.Info().
			Int64("refund_order_id", refundOrder.ID).
			Int64("payment_order_id", refundOrder.PaymentOrderID).
			Str("business_type", refundOrder.BusinessType).
			Msg("pending order refund recovery task enqueued")
	}
}
