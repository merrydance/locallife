package logic

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// ConfirmOrderInput describes the request to confirm a takeout order.
type ConfirmOrderInput struct {
	UserID  int64
	OrderID int64
}

// ConfirmOrderResult returns the confirmed order and notification context.
type ConfirmOrderResult struct {
	Order            db.Order
	AlreadyCompleted bool
	RiderID          *int64
}

// ConfirmTakeoutOrder validates and completes a takeout order.
func ConfirmTakeoutOrder(ctx context.Context, store db.Store, input ConfirmOrderInput) (ConfirmOrderResult, error) {
	order, err := store.GetOrder(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ConfirmOrderResult{}, NewRequestError(http.StatusNotFound, errors.New("order not found"))
		}
		return ConfirmOrderResult{}, err
	}

	if order.UserID != input.UserID {
		return ConfirmOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("order does not belong to you"))
	}

	if order.OrderType != "takeout" {
		return ConfirmOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("only takeout orders can be confirmed"))
	}

	if order.Status == "completed" {
		return ConfirmOrderResult{Order: order, AlreadyCompleted: true}, nil
	}

	if order.Status != "rider_delivered" && order.Status != "user_delivered" {
		return ConfirmOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("order is not ready for confirmation"))
	}

	updatedOrder, err := store.CompleteTakeoutOrderByUser(ctx, order.ID)
	if err != nil {
		return ConfirmOrderResult{}, err
	}

	_, _ = store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
		OrderID:      order.ID,
		FromStatus:   pgtype.Text{String: order.Status, Valid: true},
		ToStatus:     "completed",
		OperatorID:   pgtype.Int8{Int64: input.UserID, Valid: true},
		OperatorType: pgtype.Text{String: "user", Valid: true},
		Notes:        pgtype.Text{String: "用户确认收货并完成订单", Valid: true},
	})

	var riderID *int64
	delivery, err := store.GetDeliveryByOrderID(ctx, order.ID)
	if err == nil && delivery.RiderID.Valid {
		value := delivery.RiderID.Int64
		riderID = &value
	}

	return ConfirmOrderResult{
		Order:            updatedOrder,
		AlreadyCompleted: false,
		RiderID:          riderID,
	}, nil
}
