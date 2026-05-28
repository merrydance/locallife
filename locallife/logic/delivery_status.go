package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/algorithm"
	db "github.com/merrydance/locallife/db/sqlc"
)

// DeliveryStatusInput carries rider and delivery identifiers.
type DeliveryStatusInput struct {
	UserID     int64
	DeliveryID int64
}

// ConfirmDeliveryInput carries confirm delivery parameters.
type ConfirmDeliveryInput struct {
	UserID              int64
	DeliveryID          int64
	ConfirmRadiusMeters int
	LocationMaxAgeSec   int
}

// DeliveryStatusResult returns updated delivery data and related entities.
type DeliveryStatusResult struct {
	Delivery       db.Delivery
	Order          db.Order
	Rider          db.Rider
	PreviousStatus string
}

func mapDeliveryStateTransitionError(err error) error {
	if errors.Is(err, db.ErrDeliveryStateTransitionConflict) {
		return NewRequestError(http.StatusConflict, errors.New("代取状态已变化，请刷新后重试"))
	}
	if errors.Is(err, db.ErrTakeoutOrderPausedByFoodSafety) {
		return NewRequestError(http.StatusForbidden, errors.New("该外卖订单因食安事件已暂停履约，请等待平台处理"))
	}
	return err
}

func validateDeliveryConfirmRadius(rider db.Rider, delivery db.Delivery, confirmRadiusMeters int, locationMaxAgeSec int) error {
	riderLng, riderLngOk := floatFromNumeric(rider.CurrentLongitude)
	riderLat, riderLatOk := floatFromNumeric(rider.CurrentLatitude)
	if !riderLngOk || !riderLatOk {
		return &DeliveryConfirmValidationError{
			Reason:       "rider_location_missing",
			RadiusMeters: confirmRadiusMeters,
			Message:      "骑手定位缺失，无法确认送达，请先刷新定位",
		}
	}

	if locationMaxAgeSec > 0 {
		if !rider.LocationUpdatedAt.Valid {
			return &DeliveryConfirmValidationError{
				Reason:    "rider_location_stale",
				MaxAgeSec: locationMaxAgeSec,
				Message:   "骑手定位已过期，无法确认送达，请刷新定位后重试",
			}
		}

		locationAgeSec := int(time.Since(rider.LocationUpdatedAt.Time).Seconds())
		if locationAgeSec < 0 {
			locationAgeSec = 0
		}
		if locationAgeSec > locationMaxAgeSec {
			return &DeliveryConfirmValidationError{
				Reason:         "rider_location_stale",
				LocationAgeSec: locationAgeSec,
				MaxAgeSec:      locationMaxAgeSec,
				Message:        "骑手定位已过期，无法确认送达，请刷新定位后重试",
			}
		}
	}

	if confirmRadiusMeters <= 0 {
		return nil
	}

	deliveryLng, deliveryLngOk := floatFromNumeric(delivery.DeliveryLongitude)
	deliveryLat, deliveryLatOk := floatFromNumeric(delivery.DeliveryLatitude)
	if !deliveryLngOk || !deliveryLatOk {
		return &DeliveryConfirmValidationError{
			Reason:       "dropoff_location_missing",
			RadiusMeters: confirmRadiusMeters,
			Message:      "收货位置缺失，无法确认送达，请联系平台处理",
		}
	}

	riderLoc := algorithm.Location{Longitude: riderLng, Latitude: riderLat}
	deliveryLoc := algorithm.Location{Longitude: deliveryLng, Latitude: deliveryLat}
	distance := algorithm.HaversineDistance(riderLoc, deliveryLoc)
	if distance > confirmRadiusMeters {
		return &DeliveryConfirmValidationError{
			Reason:         "distance_too_far",
			DistanceMeters: distance,
			RadiusMeters:   confirmRadiusMeters,
			Message: fmt.Sprintf(
				"您距离代取地址%d米，请靠近后确认送达（需在%d米内）",
				distance,
				confirmRadiusMeters,
			),
		}
	}

	return nil
}

// StartPickup advances a delivery into picking status.
func StartPickup(ctx context.Context, store db.Store, input DeliveryStatusInput) (DeliveryStatusResult, error) {
	var result DeliveryStatusResult

	rider, err := store.GetRiderByUserID(ctx, input.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("您还不是骑手"))
		}
		return result, err
	}

	delivery, err := store.GetDelivery(ctx, input.DeliveryID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("代取单不存在"))
		}
		return result, err
	}

	if !delivery.RiderID.Valid || delivery.RiderID.Int64 != rider.ID {
		return result, NewRequestError(http.StatusForbidden, errors.New("无权操作此代取单"))
	}

	if delivery.Status != "assigned" {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("当前状态(%s)不允许开始取餐", delivery.Status))
	}

	order, err := store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("订单不存在"))
		}
		return result, err
	}
	if !IsOrderStatusAllowedForDeliveryAction(order.Status, "start_pickup") {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("当前订单状态(%s)不允许开始取餐", order.Status))
	}

	updated, err := store.UpdateDeliveryToPickupTx(ctx, db.UpdateDeliveryToPickupTxParams{
		DeliveryID: input.DeliveryID,
		RiderID:    rider.ID,
		OrderID:    delivery.OrderID,
	})
	if err != nil {
		return result, mapDeliveryStateTransitionError(err)
	}

	return DeliveryStatusResult{
		Delivery:       updated.Delivery,
		Order:          order,
		Rider:          rider,
		PreviousStatus: order.Status,
	}, nil
}

// ConfirmPickup advances a delivery into picked status and logs order status.
func ConfirmPickup(ctx context.Context, store db.Store, input DeliveryStatusInput) (DeliveryStatusResult, error) {
	var result DeliveryStatusResult

	rider, err := store.GetRiderByUserID(ctx, input.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("您还不是骑手"))
		}
		return result, err
	}

	delivery, err := store.GetDelivery(ctx, input.DeliveryID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("代取单不存在"))
		}
		return result, err
	}

	if !delivery.RiderID.Valid || delivery.RiderID.Int64 != rider.ID {
		return result, NewRequestError(http.StatusForbidden, errors.New("无权操作此代取单"))
	}

	if delivery.Status != "picking" {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("当前状态(%s)不允许确认取餐", delivery.Status))
	}

	order, err := store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("订单不存在"))
		}
		return result, err
	}
	oldStatus := order.Status
	if order.Status == db.OrderStatusCourierAccepted && order.FulfillmentStatus != db.FulfillmentStatusReady {
		blockErr := &DeliveryPickupBlockedError{
			Reason:            DeliveryPickupBlockReasonMerchantNotReady,
			OrderID:           order.ID,
			DeliveryID:        delivery.ID,
			RiderID:           rider.ID,
			OrderStatus:       order.Status,
			FulfillmentStatus: order.FulfillmentStatus,
			Message:           DeliveryPickupBlockedMerchantNotReadyMessage,
		}
		return result, NewRequestError(http.StatusConflict, blockErr)
	}
	if !IsOrderAllowedForDeliveryAction(order, "confirm_pickup") {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("当前订单状态(%s)不允许确认取餐", order.Status))
	}

	updated, err := store.UpdateDeliveryToPickedTx(ctx, db.UpdateDeliveryToPickedTxParams{
		DeliveryID: input.DeliveryID,
		RiderID:    rider.ID,
		OrderID:    delivery.OrderID,
	})
	if err != nil {
		return result, mapDeliveryStateTransitionError(err)
	}

	if oldStatus != "picked" {
		_, _ = store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      order.ID,
			FromStatus:   pgtype.Text{String: oldStatus, Valid: true},
			ToStatus:     "picked",
			OperatorID:   pgtype.Int8{Int64: rider.UserID, Valid: true},
			OperatorType: pgtype.Text{String: "rider", Valid: true},
			Notes:        pgtype.Text{String: "骑手确认取餐", Valid: true},
		})
	}

	order.Status = "picked"

	return DeliveryStatusResult{
		Delivery:       updated.Delivery,
		Order:          order,
		Rider:          rider,
		PreviousStatus: oldStatus,
	}, nil
}

// StartDelivery advances a delivery into delivering status and logs order status.
func StartDelivery(ctx context.Context, store db.Store, input DeliveryStatusInput) (DeliveryStatusResult, error) {
	var result DeliveryStatusResult

	rider, err := store.GetRiderByUserID(ctx, input.UserID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("您还不是骑手"))
		}
		return result, err
	}

	delivery, err := store.GetDelivery(ctx, input.DeliveryID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("代取单不存在"))
		}
		return result, err
	}

	if !delivery.RiderID.Valid || delivery.RiderID.Int64 != rider.ID {
		return result, NewRequestError(http.StatusForbidden, errors.New("无权操作此代取单"))
	}

	if delivery.Status != "picked" {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("当前状态(%s)不允许开始代取", delivery.Status))
	}

	order, err := store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("订单不存在"))
		}
		return result, err
	}
	oldStatus := order.Status

	updated, err := store.UpdateDeliveryToDeliveringTx(ctx, db.UpdateDeliveryToDeliveringTxParams{
		DeliveryID: input.DeliveryID,
		RiderID:    rider.ID,
		OrderID:    delivery.OrderID,
	})
	if err != nil {
		return result, mapDeliveryStateTransitionError(err)
	}

	if oldStatus != "delivering" {
		_, _ = store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      order.ID,
			FromStatus:   pgtype.Text{String: oldStatus, Valid: true},
			ToStatus:     "delivering",
			OperatorID:   pgtype.Int8{Int64: rider.UserID, Valid: true},
			OperatorType: pgtype.Text{String: "rider", Valid: true},
			Notes:        pgtype.Text{String: "骑手开始代取", Valid: true},
		})
	}

	order.Status = "delivering"

	return DeliveryStatusResult{
		Delivery:       updated.Delivery,
		Order:          order,
		Rider:          rider,
		PreviousStatus: oldStatus,
	}, nil
}

// ConfirmDelivery completes a delivery, validates radius, and logs order status.
func ConfirmDelivery(ctx context.Context, store db.Store, input ConfirmDeliveryInput) (DeliveryStatusResult, error) {
	var result DeliveryStatusResult

	rider, err := store.GetRiderByUserID(ctx, input.UserID)
	if err != nil {
		return result, err
	}

	delivery, err := store.GetDelivery(ctx, input.DeliveryID)
	if err != nil {
		return result, err
	}

	if !delivery.RiderID.Valid || delivery.RiderID.Int64 != rider.ID {
		return result, NewRequestError(http.StatusForbidden, errors.New("无权操作此代取单"))
	}

	if delivery.Status != "delivering" {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("当前状态(%s)不允许确认送达", delivery.Status))
	}

	if err := validateDeliveryConfirmRadius(rider, delivery, input.ConfirmRadiusMeters, input.LocationMaxAgeSec); err != nil {
		return result, NewRequestError(http.StatusBadRequest, err)
	}

	order, err := store.GetOrder(ctx, delivery.OrderID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("订单不存在"))
		}
		return result, err
	}
	oldStatus := order.Status

	unfreezeAmount := OrderFreezeAmount(order)
	updated, err := store.CompleteDeliveryTx(ctx, db.CompleteDeliveryTxParams{
		DeliveryID:     input.DeliveryID,
		RiderID:        rider.ID,
		OrderID:        delivery.OrderID,
		UnfreezeAmount: unfreezeAmount,
		DeliveryFee:    delivery.DeliveryFee,
	})
	if err != nil {
		return result, err
	}

	if oldStatus != "rider_delivered" {
		_, _ = store.CreateOrderStatusLog(ctx, db.CreateOrderStatusLogParams{
			OrderID:      order.ID,
			FromStatus:   pgtype.Text{String: oldStatus, Valid: true},
			ToStatus:     "rider_delivered",
			OperatorID:   pgtype.Int8{Int64: rider.UserID, Valid: true},
			OperatorType: pgtype.Text{String: "rider", Valid: true},
			Notes:        pgtype.Text{String: "骑手确认送达", Valid: true},
		})
	}

	order.Status = "rider_delivered"

	return DeliveryStatusResult{
		Delivery:       updated.Delivery,
		Order:          order,
		Rider:          rider,
		PreviousStatus: oldStatus,
	}, nil
}
