
// WebSocket Event Types
export const EventReservationNew = "reservation_new";
export const EventReservationUpdate = "reservation_update";
export const EventTableStatusChange = "table_status_change";
export const EventTableTransfer = "table_transfer";
export const EventMerchantUserRiskAlert = "merchant_user_risk_alert";
export const EventSessionClosed = "session_closed";

// Reservation Statuses
export const ReservationStatusPending = "pending";
export const ReservationStatusPaid = "paid";
export const ReservationStatusConfirmed = "confirmed";
export const ReservationStatusCheckedIn = "checked_in";
export const ReservationStatusCompleted = "completed";
export const ReservationStatusCancelled = "cancelled";
export const ReservationStatusExpired = "expired";
export const ReservationStatusNoShow = "no_show";

// Table Statuses
export const TableStatusAvailable = "available";
export const TableStatusOccupied = "occupied";
export const TableStatusReserved = "reserved";
export const TableStatusDisabled = "disabled";
