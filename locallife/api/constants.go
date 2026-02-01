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

	// Operator Rules Defaults
	DefaultMerchantDeposit = "5000"
	DefaultWeatherExtreme  = "2.0"
	DefaultWeatherHeavy    = "1.8"
	DefaultWeatherModerate = "1.3"
	DefaultWeatherLight    = "1.1"

	// Stats Constants
	StatsStartYear = 2020
	StatsEndYear   = 2099
)
