package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// UrgeOrderInput describes the request to urge an order.
type UrgeOrderInput struct {
	UserID          int64
	OrderID         int64
	RateLimitWindow time.Duration
	RateLimitMax    int64
	Now             time.Time
}

// UrgeOrderResult returns details used for notifications.
type UrgeOrderResult struct {
	Order          db.Order
	NotifyMerchant bool
	RiderID        *int64
}

// UrgeOrder validates and records an urge request.
func UrgeOrder(ctx context.Context, store db.Store, input UrgeOrderInput) (UrgeOrderResult, error) {
	order, err := store.GetOrder(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return UrgeOrderResult{}, NewRequestError(http.StatusNotFound, errors.New("order not found"))
		}
		return UrgeOrderResult{}, err
	}

	if order.UserID != input.UserID {
		return UrgeOrderResult{}, NewRequestError(http.StatusForbidden, errors.New("order does not belong to you"))
	}

	recentUrgeCount, err := store.CountRecentOrderStatusLogs(ctx, db.CountRecentOrderStatusLogsParams{
		OrderID:   order.ID,
		Notes:     pgtype.Text{String: "用户催单", Valid: true},
		CreatedAt: input.Now.Add(-input.RateLimitWindow),
	})
	if err == nil && recentUrgeCount >= input.RateLimitMax {
		return UrgeOrderResult{}, NewRequestError(http.StatusTooManyRequests, fmt.Errorf(
			"催单过于频繁，请%d分钟后再试",
			int(input.RateLimitWindow.Minutes()),
		))
	}

	allowedStatuses := map[string]bool{
		"paid":             true,
		"preparing":        true,
		"ready":            true,
		"courier_accepted": true,
		"picked":           true,
		"delivering":       true,
		"rider_delivered":  true,
	}
	if !allowedStatuses[order.Status] {
		return UrgeOrderResult{}, NewRequestError(http.StatusBadRequest, errors.New("order cannot be urged in current status"))
	}

	notifyMerchant := order.Status == "paid" || order.Status == "preparing"
	var riderID *int64
	if order.Status == "delivering" || order.Status == "courier_accepted" || order.Status == "picked" || order.Status == "rider_delivered" {
		delivery, err := store.GetDeliveryByOrderID(ctx, order.ID)
		if err == nil && delivery.RiderID.Valid {
			value := delivery.RiderID.Int64
			riderID = &value
		}
	}

	_, _ = store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
		OrderID:      order.ID,
		FromStatus:   pgtype.Text{String: order.Status, Valid: true},
		ToStatus:     order.Status,
		OperatorID:   pgtype.Int8{Int64: input.UserID, Valid: true},
		OperatorType: pgtype.Text{String: "user", Valid: true},
		Notes:        pgtype.Text{String: "用户催单", Valid: true},
	})

	return UrgeOrderResult{
		Order:          order,
		NotifyMerchant: notifyMerchant,
		RiderID:        riderID,
	}, nil
}
