package logic

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// AutoStatusResult captures updates from geofence-triggered transitions.
type AutoStatusResult struct {
	Delivery       db.Delivery
	Order          db.Order
	OrderLoaded    bool
	Updated        bool
	PreviousStatus string
	LoadOrderErr   error
}

// AutoAdvancePickup moves an assigned delivery into picking when the rider dwells at pickup.
func AutoAdvancePickup(ctx context.Context, store db.Store, delivery db.Delivery, rider db.Rider) (AutoStatusResult, error) {
	result := AutoStatusResult{Delivery: delivery}
	if delivery.Status != "assigned" {
		return result, nil
	}

	order, err := store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		if !errors.Is(err, db.ErrRecordNotFound) {
			result.LoadOrderErr = err
		}
	} else {
		result.OrderLoaded = true
		result.Order = order
		result.PreviousStatus = order.Status
		if !IsOrderStatusAllowedForDeliveryAction(order.Status, "start_pickup") {
			return result, nil
		}
	}

	updated, err := store.UpdateDeliveryToPickupTx(ctx, db.UpdateDeliveryToPickupTxParams{
		DeliveryID: delivery.ID,
		RiderID:    rider.ID,
		OrderID:    delivery.OrderID,
	})
	if err != nil {
		return result, err
	}

	result.Delivery = updated.Delivery
	result.Updated = true

	if result.OrderLoaded && result.PreviousStatus != "" && result.PreviousStatus != "courier_accepted" {
		_, _ = store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      order.ID,
			FromStatus:   pgtype.Text{String: result.PreviousStatus, Valid: true},
			ToStatus:     "courier_accepted",
			OperatorID:   pgtype.Int8{Int64: rider.UserID, Valid: true},
			OperatorType: pgtype.Text{String: "rider", Valid: true},
			Notes:        pgtype.Text{String: "围栏驻留自动触发开始取餐", Valid: true},
		})
		result.Order.Status = "courier_accepted"
	}

	return result, nil
}

// AutoConfirmPickup moves a picking delivery into picked when the rider dwells at pickup.
func AutoConfirmPickup(ctx context.Context, store db.Store, delivery db.Delivery, rider db.Rider) (AutoStatusResult, error) {
	result := AutoStatusResult{Delivery: delivery}
	if delivery.Status != "picking" {
		return result, nil
	}

	order, err := store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, nil
		}
		return result, err
	}
	if !IsOrderStatusAllowedForDeliveryAction(order.Status, "confirm_pickup") {
		return result, nil
	}

	updated, err := store.UpdateDeliveryToPickedTx(ctx, db.UpdateDeliveryToPickedTxParams{
		DeliveryID: delivery.ID,
		RiderID:    rider.ID,
		OrderID:    delivery.OrderID,
	})
	if err != nil {
		return result, err
	}

	result.Delivery = updated.Delivery
	result.OrderLoaded = true
	result.Order = order
	result.PreviousStatus = order.Status
	result.Updated = true

	if order.Status != "picked" {
		_, _ = store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      order.ID,
			FromStatus:   pgtype.Text{String: order.Status, Valid: true},
			ToStatus:     "picked",
			OperatorID:   pgtype.Int8{Int64: rider.UserID, Valid: true},
			OperatorType: pgtype.Text{String: "rider", Valid: true},
			Notes:        pgtype.Text{String: "围栏驻留自动确认取餐", Valid: true},
		})
		result.Order.Status = "picked"
	}

	return result, nil
}

// AutoConfirmDelivery completes a delivering delivery when the rider dwells at dropoff.
func AutoConfirmDelivery(ctx context.Context, store db.Store, delivery db.Delivery, rider db.Rider, unfreezeAmount int64) (AutoStatusResult, error) {
	result := AutoStatusResult{Delivery: delivery}
	if delivery.Status != "delivering" {
		return result, nil
	}

	order, err := store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, nil
		}
		return result, err
	}
	if !IsOrderStatusAllowedForDeliveryAction(order.Status, "confirm_delivery") {
		return result, nil
	}

	updated, err := store.CompleteDeliveryTx(ctx, db.CompleteDeliveryTxParams{
		DeliveryID:     delivery.ID,
		RiderID:        rider.ID,
		OrderID:        delivery.OrderID,
		UnfreezeAmount: unfreezeAmount,
		DeliveryFee:    delivery.DeliveryFee,
	})
	if err != nil {
		return result, err
	}

	result.Delivery = updated.Delivery
	result.OrderLoaded = true
	result.Order = order
	result.PreviousStatus = order.Status
	result.Updated = true

	if order.Status != "rider_delivered" {
		_, _ = store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      order.ID,
			FromStatus:   pgtype.Text{String: order.Status, Valid: true},
			ToStatus:     "rider_delivered",
			OperatorID:   pgtype.Int8{Int64: rider.UserID, Valid: true},
			OperatorType: pgtype.Text{String: "rider", Valid: true},
			Notes:        pgtype.Text{String: "围栏驻留自动确认送达", Valid: true},
		})
		result.Order.Status = "rider_delivered"
	}

	return result, nil
}
