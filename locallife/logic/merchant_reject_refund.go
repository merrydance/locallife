package logic

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	"github.com/rs/zerolog/log"
)

// MerchantRejectRefundInput defines the input for refunding a rejected order.
type MerchantRejectRefundInput struct {
	OrderID int64
	Reason  string
}

// MerchantRejectRefundResult captures refund processing details.
type MerchantRejectRefundResult struct {
	PaymentOrder *db.PaymentOrder
	RefundOrder  *db.RefundOrder
}

// ProcessMerchantRejectRefund handles full refund for a merchant-rejected order.
func ProcessMerchantRejectRefund(
	ctx context.Context,
	store db.Store,
	paymentClient wechat.PaymentClientInterface,
	input MerchantRejectRefundInput,
) (MerchantRejectRefundResult, error) {
	var result MerchantRejectRefundResult

	paymentOrder, err := store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: input.OrderID, Valid: true},
		BusinessType: "order",
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, nil
		}
		return result, err
	}
	if paymentOrder.Status != "paid" {
		paymentOrders, listErr := store.GetPaymentOrdersByOrder(ctx, pgtype.Int8{Int64: input.OrderID, Valid: true})
		if listErr != nil {
			return result, listErr
		}

		foundPaid := false
		for _, candidate := range paymentOrders {
			if candidate.BusinessType == "order" && candidate.Status == "paid" {
				paymentOrder = candidate
				foundPaid = true
				break
			}
		}

		if !foundPaid {
			return result, nil
		}
	}
	result.PaymentOrder = &paymentOrder

	reason := fmt.Sprintf("商户拒单：%s", input.Reason)
	outRefundNo, err := generateOutRefundNo()
	if err != nil {
		return result, fmt.Errorf("generate out refund no: %w", err)
	}
	refundType := paymentOrder.PaymentType
	if refundType == paymentTypeNative {
		refundType = paymentTypeMiniProgram
	}

	txResult, err := store.CreateRefundOrderTx(ctx, db.CreateRefundOrderTxParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     refundType,
		RefundAmount:   paymentOrder.Amount,
		RefundReason:   reason,
		OutRefundNo:    outRefundNo,
	})
	if err != nil {
		if _, ok := db.IsRefundRequestError(err); ok {
			return result, fmt.Errorf("refund validation: %w", err)
		}
		return result, err
	}
	refundOrder := txResult.RefundOrder
	result.RefundOrder = &refundOrder

	if paymentClient == nil {
		return result, nil
	}

	wxRefund, err := paymentClient.CreateRefund(ctx, &wechat.RefundRequest{
		OutTradeNo:   paymentOrder.OutTradeNo,
		OutRefundNo:  outRefundNo,
		Reason:       reason,
		RefundAmount: paymentOrder.Amount,
		TotalAmount:  paymentOrder.Amount,
	})
	if err != nil {
		// R-05 修复：微信API失败时不标记为failed，保持pending状态
		// 由 RefundRecoveryScheduler 每5分钟自动补偿重试
		return result, fmt.Errorf("wechat refund api: %w", err)
	}

	switch wxRefund.Status {
	case wechat.RefundStatusSuccess:
		if _, dbErr := store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as success")
		}
		if _, dbErr := store.UpdatePaymentOrderToRefunded(ctx, paymentOrder.ID); dbErr != nil {
			log.Error().Err(dbErr).Int64("payment_order_id", paymentOrder.ID).Msg("failed to mark payment order as refunded")
		}
	case wechat.RefundStatusProcessing:
		if _, dbErr := store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: true},
		}); dbErr != nil {
			log.Error().Err(dbErr).Int64("refund_order_id", refundOrder.ID).Msg("failed to mark refund order as processing")
		}
	}

	return result, nil
}
