package db

const (
	// Order statuses (SSOT — 所有层引用此处)
	OrderStatusPending         = "pending"
	OrderStatusPaid            = "paid"
	OrderStatusPreparing       = "preparing"
	OrderStatusReady           = "ready"
	OrderStatusCourierAccepted = "courier_accepted"
	OrderStatusPicked          = "picked"
	OrderStatusDelivering      = "delivering"
	OrderStatusRiderDelivered  = "rider_delivered"
	OrderStatusUserDelivered   = "user_delivered"
	OrderStatusCompleted       = "completed"
	OrderStatusCancelled       = "cancelled"

	OrderTypeReservation = "reservation"

	FulfillmentStatusScheduled      = "scheduled"
	FulfillmentStatusPendingKitchen = "pending_kitchen"
	FulfillmentStatusPreparing      = "preparing"
	FulfillmentStatusReady          = "ready"
	FulfillmentStatusCompleted      = "completed"
	FulfillmentStatusCancelled      = "cancelled"
)
