package db

const (
	OrderStatusPending   = "pending"
	OrderStatusPaid      = "paid"
	OrderTypeReservation = "reservation"

	FulfillmentStatusScheduled      = "scheduled"
	FulfillmentStatusPendingKitchen = "pending_kitchen"
	FulfillmentStatusPreparing      = "preparing"
	FulfillmentStatusReady          = "ready"
	FulfillmentStatusCompleted      = "completed"
	FulfillmentStatusCancelled      = "cancelled"
)
