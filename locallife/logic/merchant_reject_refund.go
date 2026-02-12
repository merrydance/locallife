package logic

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
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
		return result, nil
	}
	result.PaymentOrder = &paymentOrder

	reason := fmt.Sprintf("商户拒单：%s", input.Reason)
	outRefundNo := generateOutRefundNo()

	refundOrder, err := store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
		PaymentOrderID: paymentOrder.ID,
		RefundType:     "full",
		RefundAmount:   paymentOrder.Amount,
		RefundReason:   pgtype.Text{String: reason, Valid: true},
		OutRefundNo:    outRefundNo,
		Status:         "pending",
	})
	if err != nil {
		return result, err
	}
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
		_, _ = store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
		return result, err
	}

	switch wxRefund.Status {
	case wechat.RefundStatusSuccess:
		_, _ = store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
		_, _ = store.UpdatePaymentOrderToRefunded(ctx, paymentOrder.ID)
	case wechat.RefundStatusProcessing:
		_, _ = store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
			ID:       refundOrder.ID,
			RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: true},
		})
	}

	return result, nil
}
