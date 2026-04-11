package logic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolveMerchantReservationActionState(t *testing.T) {
	reservationDate := time.Date(2026, 4, 9, 0, 0, 0, 0, time.Local)
	checkInTime := time.Date(0, time.January, 1, 18, 30, 0, 0, time.Local)

	testCases := []struct {
		name  string
		input MerchantReservationActionStateInput
		check func(t *testing.T, state MerchantReservationActionState)
	}{
		{
			name: "OwnerConfirmedWithinCheckInWindow",
			input: MerchantReservationActionStateInput{
				Status:             reservationStatusConfirmed,
				ReservationDate:    &reservationDate,
				ReservationTime:    &checkInTime,
				StaffRole:          "owner",
				Now:                time.Date(2026, 4, 9, 18, 10, 0, 0, time.Local),
				CheckInEarlyMinute: 30,
				CheckInLateMinute:  30,
			},
			check: func(t *testing.T, state MerchantReservationActionState) {
				require.True(t, state.CanEdit)
				require.True(t, state.CanCancel)
				require.False(t, state.CanConfirm)
				require.True(t, state.CanCheckIn)
				require.True(t, state.CanStartCooking)
				require.True(t, state.CanNoShow)
				require.True(t, state.CanComplete)
				require.Equal(t, reservationActionCheckIn, state.PrimaryActionKey)
				require.True(t, state.ShowMoreActions)
			},
		},
		{
			name: "CashierConfirmedWithinCheckInWindow",
			input: MerchantReservationActionStateInput{
				Status:             reservationStatusConfirmed,
				ReservationDate:    &reservationDate,
				ReservationTime:    &checkInTime,
				StaffRole:          "cashier",
				Now:                time.Date(2026, 4, 9, 18, 10, 0, 0, time.Local),
				CheckInEarlyMinute: 30,
				CheckInLateMinute:  30,
			},
			check: func(t *testing.T, state MerchantReservationActionState) {
				require.False(t, state.CanEdit)
				require.True(t, state.CanCancel)
				require.True(t, state.CanCheckIn)
				require.True(t, state.CanStartCooking)
				require.False(t, state.CanNoShow)
				require.True(t, state.CanComplete)
				require.Equal(t, reservationActionCheckIn, state.PrimaryActionKey)
			},
		},
		{
			name: "OwnerConfirmedOutsideCheckInWindow",
			input: MerchantReservationActionStateInput{
				Status:             reservationStatusConfirmed,
				ReservationDate:    &reservationDate,
				ReservationTime:    &checkInTime,
				StaffRole:          "owner",
				Now:                time.Date(2026, 4, 9, 19, 5, 0, 0, time.Local),
				CheckInEarlyMinute: 30,
				CheckInLateMinute:  30,
			},
			check: func(t *testing.T, state MerchantReservationActionState) {
				require.False(t, state.CanCheckIn)
				require.True(t, state.CanStartCooking)
				require.Equal(t, "", state.PrimaryActionKey)
				require.True(t, state.ShowMoreActions)
			},
		},
		{
			name: "CheckedInAfterCookingStarted",
			input: MerchantReservationActionStateInput{
				Status:             reservationStatusCheckedIn,
				ReservationDate:    &reservationDate,
				ReservationTime:    &checkInTime,
				CookingStartedAt:   ptrTime(time.Date(2026, 4, 9, 18, 40, 0, 0, time.Local)),
				StaffRole:          "owner",
				Now:                time.Date(2026, 4, 9, 18, 45, 0, 0, time.Local),
				CheckInEarlyMinute: 30,
				CheckInLateMinute:  30,
			},
			check: func(t *testing.T, state MerchantReservationActionState) {
				require.False(t, state.CanStartCooking)
				require.True(t, state.CanComplete)
				require.Equal(t, reservationActionComplete, state.PrimaryActionKey)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := ResolveMerchantReservationActionState(tc.input)
			tc.check(t, state)
		})
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
