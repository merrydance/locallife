package api

const (
	// WebSocket Event Types
	EventReservationNew        = "reservation_new"
	EventReservationUpdate     = "reservation_update"
	EventTableStatusChange     = "table_status_change"
	EventTableTransfer         = "table_transfer"
	EventMerchantUserRiskAlert = "merchant_user_risk_alert"
	EventSessionClosed         = "session_closed"

	// Reservation Statuses
	ReservationStatusPending   = "pending"
	ReservationStatusPaid      = "paid"
	ReservationStatusConfirmed = "confirmed"
	ReservationStatusCheckedIn = "checked_in"
	ReservationStatusCompleted = "completed"
	ReservationStatusCancelled = "cancelled"
	ReservationStatusExpired   = "expired"
	ReservationStatusNoShow    = "no_show"

	// Table Statuses
	TableStatusAvailable = "available"
	TableStatusOccupied  = "occupied"
	TableStatusReserved  = "reserved"
	TableStatusDisabled  = "disabled"

	// Dining Session Statuses
	DiningSessionStatusOpen   = "open"
	DiningSessionStatusClosed = "closed"
)
