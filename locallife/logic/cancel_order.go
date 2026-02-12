package logic

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// CancelOrderInput defines the input for canceling an order.
type CancelOrderInput struct {
	UserID  int64
	OrderID int64
	Reason  string
}

// RefundTask describes a refund task to be dispatched.
type RefundTask struct {
	PaymentOrderID int64
	Amount         int64
	Reason         string
}

// CancelOrderResult holds the cancel result.
type CancelOrderResult struct {
	Order  db.Order
	Refund *RefundTask
}

// CancelOrder cancels a pending/paid order and prepares refund info when needed.
func CancelOrder(ctx context.Context, store db.Store, input CancelOrderInput) (CancelOrderResult, error) {
	order, err := store.GetOrderForUpdate(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return CancelOrderResult{}, NewRequestError(http.StatusNotFound, errors.New("order not found"))
		}
		return CancelOrderResult{}, err
	}

	if order.UserID != input.UserID {
		return CancelOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("order does not belong to you"))
	}

	lateStatuses := map[string]bool{
		"preparing":        true,
		"ready":            true,
		"courier_accepted": true,
		"picked":           true,
		"delivering":       true,
		"rider_delivered":  true,
	}
	if lateStatuses[order.Status] {
		_, _ = store.UpdateOrderExceptionState(ctx, db.UpdateOrderExceptionStateParams{
			ID:             order.ID,
			ExceptionState: pgtype.Text{String: "cancel_requested", Valid: true},
			ClaimChannel:   pgtype.Text{String: "user", Valid: true},
		})
		_, _ = store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      order.ID,
			FromStatus:   pgtype.Text{String: order.Status, Valid: true},
			ToStatus:     order.Status,
			OperatorID:   pgtype.Int8{Int64: input.UserID, Valid: true},
			OperatorType: pgtype.Text{String: "user", Valid: true},
			Notes:        pgtype.Text{String: "用户申请取消，进入售后通道", Valid: true},
		})
		return CancelOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("订单已制作/配送，已记录取消诉求，请联系商户或客服处理"))
	}

	if order.Status != "pending" && order.Status != "paid" {
		return CancelOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("订单当前状态无法取消，商户已接单后请联系商户处理"))
	}

	cancelReason := "用户取消"
	if input.Reason != "" {
		cancelReason = input.Reason
	}

	result, err := store.CancelOrderTx(ctx, db.CancelOrderTxParams{
		OrderID:      input.OrderID,
		OldStatus:    order.Status,
		CancelReason: cancelReason,
		OperatorID:   input.UserID,
		OperatorType: "user",
	})
	if err != nil {
		return CancelOrderResult{}, err
	}

	finalResult := CancelOrderResult{Order: result.Order}

	if order.Status == "paid" {
		paymentOrders, err := store.GetPaymentOrdersByOrder(ctx, pgtype.Int8{Int64: order.ID, Valid: true})
		if err == nil {
			for _, paymentOrder := range paymentOrders {
				if paymentOrder.Status == "paid" {
					finalResult.Refund = &RefundTask{
						PaymentOrderID: paymentOrder.ID,
						Amount:         paymentOrder.Amount,
						Reason:         cancelReason,
					}
					break
				}
			}
		}
	}

	return finalResult, nil
}
