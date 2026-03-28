package logic

import (
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// IsOrderStatusAllowedForDeliveryAction checks if an order status permits a delivery action.
func IsOrderStatusAllowedForDeliveryAction(status string, action string) bool {
	switch action {
	case "grab":
		return status == db.OrderStatusReady
	case "start_pickup":
		return status == db.OrderStatusCourierAccepted
	case "confirm_pickup":
		return status == db.OrderStatusCourierAccepted
	case "start_delivery":
		return status == db.OrderStatusPicked
	case "confirm_delivery":
		return status == db.OrderStatusDelivering
	default:
		return false
	}
}

// OrderFreezeAmount determines how much deposit should be frozen for an order.
func OrderFreezeAmount(order db.Order) int64 {
	if order.FinalAmount.Valid && order.FinalAmount.Int64 > 0 {
		return order.FinalAmount.Int64
	}
	if order.TotalAmount > 0 {
		return order.TotalAmount
	}
	return 0
}

func floatFromNumeric(value pgtype.Numeric) (float64, bool) {
	if !value.Valid {
		return 0, false
	}
	floatValue, err := value.Float64Value()
	if err != nil || !floatValue.Valid {
		return 0, false
	}
	return floatValue.Float64, true
}
