package logic

import (
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	DeliveryPickupBlockReasonMerchantNotReady    = "merchant_not_ready"
	DeliveryPickupBlockedMerchantNotReadyMessage = "商户未出餐，暂不可确认取餐"
	DeliveryPickupWaitMerchantActionLabel        = "等待商户出餐"
	DeliveryPickupConfirmActionLabel             = "确认取餐"
	DeliveryPickupUnavailableActionLabel         = "暂不可取餐"
	DeliveryPickupUnavailableMessage             = "当前订单状态不允许确认取餐，请刷新任务后重试"
	DeliveryPickupStateUnavailableMessage        = "当前任务状态暂不可用，请刷新后重试"
)

type DeliveryPickupActionState struct {
	CanConfirmPickup  bool
	PickupBlockReason string
	PickupActionLabel string
}

// IsOrderStatusAllowedForDeliveryAction checks if an order status permits a delivery action.
func IsOrderStatusAllowedForDeliveryAction(status string, action string) bool {
	switch action {
	case "grab":
		return status == db.OrderStatusPreparing || status == db.OrderStatusReady
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

func IsOrderAllowedForDeliveryAction(order db.Order, action string) bool {
	if action == "confirm_pickup" {
		return order.Status == db.OrderStatusCourierAccepted && order.FulfillmentStatus == db.FulfillmentStatusReady
	}
	return IsOrderStatusAllowedForDeliveryAction(order.Status, action)
}

func GetDeliveryPickupActionState(delivery db.Delivery, order db.Order) DeliveryPickupActionState {
	if delivery.Status != db.DeliveryStatusPicking {
		return DeliveryPickupActionState{}
	}
	if order.Status == db.OrderStatusCourierAccepted && order.FulfillmentStatus == db.FulfillmentStatusReady {
		return DeliveryPickupActionState{
			CanConfirmPickup:  true,
			PickupActionLabel: DeliveryPickupConfirmActionLabel,
		}
	}
	if order.Status == db.OrderStatusCourierAccepted && order.FulfillmentStatus != db.FulfillmentStatusReady {
		return DeliveryPickupActionState{
			PickupBlockReason: DeliveryPickupBlockedMerchantNotReadyMessage,
			PickupActionLabel: DeliveryPickupWaitMerchantActionLabel,
		}
	}
	return DeliveryPickupActionState{
		PickupBlockReason: DeliveryPickupUnavailableMessage,
		PickupActionLabel: DeliveryPickupUnavailableActionLabel,
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
