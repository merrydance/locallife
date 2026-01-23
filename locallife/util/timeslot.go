package util

import (
	"time"
)

// DiningTimeSlot represents a dining time slot
type DiningTimeSlot string

const (
	TimeSlotLunch  DiningTimeSlot = "lunch"
	TimeSlotDinner DiningTimeSlot = "dinner"
	TimeSlotOther  DiningTimeSlot = "other"
)

// Default time slots (according to user request)
const (
	LunchStartHour = 11
	LunchStartMin  = 0
	LunchEndHour   = 14
	LunchEndMin    = 30

	DinnerStartHour = 17
	DinnerStartMin  = 0
	DinnerEndHour   = 23
	DinnerEndMin    = 0
)

// GetDiningTimeSlot returns the time slot for a given time
func GetDiningTimeSlot(t time.Time) DiningTimeSlot {
	h := t.Hour()
	m := t.Minute()
	timeVal := h*100 + m

	if timeVal >= LunchStartHour*100+LunchStartMin && timeVal <= LunchEndHour*100+LunchEndMin {
		return TimeSlotLunch
	}
	if timeVal >= DinnerStartHour*100+DinnerStartMin && timeVal <= DinnerEndHour*100+DinnerEndMin {
		return TimeSlotDinner
	}

	return TimeSlotOther
}

// CombineDateAndTime combines a date and microseconds from midnight into a time.Time
func CombineDateAndTime(date time.Time, microseconds int64) time.Time {
	// 1 hour = 3600 * 10^6 microseconds
	h := microseconds / 3600000000
	m := (microseconds % 3600000000) / 60000000
	return time.Date(date.Year(), date.Month(), date.Day(), int(h), int(m), 0, 0, date.Location())
}

// IsSameDiningTimeSlot checks if two times are in the same time slot on the same day
func IsSameDiningTimeSlot(t1, t2 time.Time) bool {
	if t1.Year() != t2.Year() || t1.Month() != t2.Month() || t1.Day() != t2.Day() {
		return false
	}
	slot1 := GetDiningTimeSlot(t1)
	slot2 := GetDiningTimeSlot(t2)

	// If both are "other", we treat them as different unless they are the exact same time
	// but basically, the user wants Lunch vs Dinner separation.
	if slot1 == TimeSlotOther || slot2 == TimeSlotOther {
		return false
	}

	return slot1 == slot2
}

// IsConflictWithReservation checks if a dining session starting at 'now' would conflict with a reservation at 'resTime'
func IsConflictWithReservation(now, resTime time.Time) bool {
	// If same slot, it's a conflict
	if IsSameDiningTimeSlot(now, resTime) {
		return true
	}

	// Also check for a Proximity window: if reservation is within 1 hour, it's a conflict
	// even if slots don't match (e.g. if someone stays too long or reservation is early)
	diff := resTime.Sub(now)
	if diff > 0 && diff < 60*time.Minute {
		return true
	}

	return false
}

// AreReservationsConflicting checks if two reservation times on the same day conflict
func AreReservationsConflicting(t1, t2 time.Time) bool {
	if t1.Year() != t2.Year() || t1.Month() != t2.Month() || t1.Day() != t2.Day() {
		return false
	}
	slot1 := GetDiningTimeSlot(t1)
	slot2 := GetDiningTimeSlot(t2)

	// If they are in different well-defined slots (one lunch, one dinner), no conflict
	if (slot1 == TimeSlotLunch && slot2 == TimeSlotDinner) || (slot1 == TimeSlotDinner && slot2 == TimeSlotLunch) {
		return false
	}

	// If they are in the same slot (both lunch or both dinner), they conflict
	if slot1 != TimeSlotOther && slot1 == slot2 {
		return true
	}

	// Fallback: If any is "Other" or they are both "Other", use a 4-hour window
	diff := t1.Sub(t2)
	if diff < 0 {
		diff = -diff
	}
	return diff < 4*time.Hour
}
