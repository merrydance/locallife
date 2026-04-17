package logic

import (
	"context"
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

// DeliveryOrderViewerInput defines viewer access parameters by order.
type DeliveryOrderViewerInput struct {
	UserID           int64
	OrderID          int64
	ForbiddenMessage string
}

// DeliveryViewerInput defines access parameters by delivery.
type DeliveryViewerInput struct {
	UserID           int64
	DeliveryID       int64
	ForbiddenMessage string
}

// DeliveryViewerResult returns delivery and order context for access checks.
type DeliveryViewerResult struct {
	Delivery     db.Delivery
	Order        db.Order
	IsOrderOwner bool
	IsRider      bool
}

// GetDeliveryForViewerByOrder loads delivery info for an order owner or the assigned rider.
func GetDeliveryForViewerByOrder(ctx context.Context, store db.Store, input DeliveryOrderViewerInput) (db.Delivery, error) {
	order, err := store.GetOrder(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.Delivery{}, NewRequestError(http.StatusNotFound, errors.New("订单不存在"))
		}
		return db.Delivery{}, err
	}

	delivery, err := store.GetDeliveryByOrderID(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.Delivery{}, NewRequestError(http.StatusNotFound, errors.New("配送单不存在"))
		}
		return db.Delivery{}, err
	}

	if order.UserID == input.UserID {
		return delivery, nil
	}

	isRider, err := isDeliveryAssignedToUserRider(ctx, store, input.UserID, delivery)
	if err != nil {
		return db.Delivery{}, err
	}
	if isRider {
		return delivery, nil
	}

	msg := input.ForbiddenMessage
	if msg == "" {
		msg = "无权查看此订单配送信息"
	}
	return db.Delivery{}, NewRequestError(http.StatusForbidden, errors.New(msg))
}

// ValidateDeliveryViewer validates that the user can view delivery data.
func ValidateDeliveryViewer(ctx context.Context, store db.Store, input DeliveryViewerInput) (DeliveryViewerResult, error) {
	var result DeliveryViewerResult

	delivery, err := store.GetDelivery(ctx, input.DeliveryID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("配送单不存在"))
		}
		return result, err
	}

	order, err := store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("订单不存在"))
		}
		return result, err
	}

	isOwner := order.UserID == input.UserID
	isRider := false
	if !isOwner {
		isRider, err = isDeliveryAssignedToUserRider(ctx, store, input.UserID, delivery)
		if err != nil {
			return result, err
		}
	}
	if !isOwner && !isRider {
		msg := input.ForbiddenMessage
		if msg == "" {
			msg = "无权查看此配送单"
		}
		return result, NewRequestError(http.StatusForbidden, errors.New(msg))
	}

	result.Delivery = delivery
	result.Order = order
	result.IsOrderOwner = isOwner
	result.IsRider = isRider

	return result, nil
}

func isDeliveryAssignedToUserRider(ctx context.Context, store db.Store, userID int64, delivery db.Delivery) (bool, error) {
	if !delivery.RiderID.Valid {
		return false, nil
	}

	rider, err := store.GetRiderByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	return rider.ID == delivery.RiderID.Int64, nil
}
