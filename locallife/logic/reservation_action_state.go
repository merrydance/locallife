package logic

import "time"

type MerchantReservationActionState struct {
	CanEdit          bool
	CanCancel        bool
	CanConfirm       bool
	CanCheckIn       bool
	CanStartCooking  bool
	CanNoShow        bool
	CanComplete      bool
	PrimaryActionKey string
	ShowMoreActions  bool
}

type MerchantReservationActionStateInput struct {
	Status             string
	ReservationDate    *time.Time
	ReservationTime    *time.Time
	CookingStartedAt   *time.Time
	StaffRole          string
	Now                time.Time
	CheckInEarlyMinute int
	CheckInLateMinute  int
}

func ResolveMerchantReservationActionState(input MerchantReservationActionStateInput) MerchantReservationActionState {
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}

	canManageReservation := input.StaffRole == "owner" || input.StaffRole == "manager"
	canEdit := canManageReservation && input.Status != reservationStatusCompleted && input.Status != reservationStatusCancelled && input.Status != reservationStatusExpired
	canCancel := isReservationStatusAllowed(input.Status, reservationActionCancel)
	canConfirm := isReservationStatusAllowed(input.Status, reservationActionConfirm)
	canCheckIn := isReservationStatusAllowed(input.Status, reservationActionCheckIn) && isReservationCheckInWindowOpen(input, now)
	canStartCooking := isReservationStatusAllowed(input.Status, reservationActionStartCooking) && input.CookingStartedAt == nil
	canNoShow := canManageReservation && isReservationStatusAllowed(input.Status, reservationActionNoShow)
	canComplete := isReservationStatusAllowed(input.Status, reservationActionComplete)

	primaryActionKey := ""
	switch {
	case canConfirm:
		primaryActionKey = reservationActionConfirm
	case input.Status == reservationStatusConfirmed && canCheckIn:
		primaryActionKey = reservationActionCheckIn
	case input.Status == reservationStatusCheckedIn && canStartCooking:
		primaryActionKey = reservationActionStartCooking
	case input.Status == reservationStatusCheckedIn && canComplete:
		primaryActionKey = reservationActionComplete
	}

	showMoreActions := canEdit ||
		canCancel ||
		canNoShow ||
		(canConfirm && primaryActionKey != reservationActionConfirm) ||
		(canCheckIn && primaryActionKey != reservationActionCheckIn) ||
		(canStartCooking && primaryActionKey != reservationActionStartCooking) ||
		(canComplete && primaryActionKey != reservationActionComplete)

	return MerchantReservationActionState{
		CanEdit:          canEdit,
		CanCancel:        canCancel,
		CanConfirm:       canConfirm,
		CanCheckIn:       canCheckIn,
		CanStartCooking:  canStartCooking,
		CanNoShow:        canNoShow,
		CanComplete:      canComplete,
		PrimaryActionKey: primaryActionKey,
		ShowMoreActions:  showMoreActions,
	}
}

func isReservationCheckInWindowOpen(input MerchantReservationActionStateInput, now time.Time) bool {
	if input.ReservationDate == nil || input.ReservationTime == nil {
		return true
	}

	scheduledAt := time.Date(
		input.ReservationDate.Year(),
		input.ReservationDate.Month(),
		input.ReservationDate.Day(),
		input.ReservationTime.Hour(),
		input.ReservationTime.Minute(),
		0,
		0,
		time.Local,
	)

	earlyLimit := scheduledAt.Add(-time.Duration(input.CheckInEarlyMinute) * time.Minute)
	lateLimit := scheduledAt.Add(time.Duration(input.CheckInLateMinute) * time.Minute)

	return !now.Before(earlyLimit) && !now.After(lateLimit)
}
