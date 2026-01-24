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

// TimeSlotConfig defines the boundaries for lunch and dinner
type TimeSlotConfig struct {
	LunchStart  int // HHMM
	LunchEnd    int // HHMM
	DinnerStart int // HHMM
	DinnerEnd   int // HHMM
}

var DefaultConfig = TimeSlotConfig{
	LunchStart:  LunchStartHour*100 + LunchStartMin,
	LunchEnd:    LunchEndHour*100 + LunchEndMin,
	DinnerStart: DinnerStartHour*100 + DinnerStartMin,
	DinnerEnd:   DinnerEndHour*100 + DinnerEndMin,
}

// GetDiningTimeSlot returns the time slot for a given time using default config
func GetDiningTimeSlot(t time.Time) DiningTimeSlot {
	return GetDiningTimeSlotWithConfig(t, DefaultConfig)
}

// GetDiningTimeSlotWithConfig returns the time slot for a given time using provided config
func GetDiningTimeSlotWithConfig(t time.Time, config TimeSlotConfig) DiningTimeSlot {
	h := t.Hour()
	m := t.Minute()
	timeVal := h*100 + m

	// If dinner start is 0 and lunch end is late, split at 15:00
	if config.DinnerStart == 0 && config.LunchEnd >= 1500 {
		if timeVal >= config.LunchStart && timeVal < 1500 {
			return TimeSlotLunch
		}
		if timeVal >= 1500 && timeVal <= config.LunchEnd {
			return TimeSlotDinner
		}
	}

	if timeVal >= config.LunchStart && timeVal <= config.LunchEnd {
		return TimeSlotLunch
	}
	if timeVal >= config.DinnerStart && timeVal <= config.DinnerEnd {
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

// IsSameDiningTimeSlot checks if two times are in the same time slot on the same day using default config
func IsSameDiningTimeSlot(t1, t2 time.Time) bool {
	return IsSameDiningTimeSlotWithConfig(t1, t2, DefaultConfig)
}

// IsSameDiningTimeSlotWithConfig checks if two times are in the same time slot on the same day using provided config
func IsSameDiningTimeSlotWithConfig(t1, t2 time.Time, config TimeSlotConfig) bool {
	if t1.Year() != t2.Year() || t1.Month() != t2.Month() || t1.Day() != t2.Day() {
		return false
	}
	slot1 := GetDiningTimeSlotWithConfig(t1, config)
	slot2 := GetDiningTimeSlotWithConfig(t2, config)

	// If both are "other", we treat them as different unless they are the exact same time
	// but basically, the user wants Lunch vs Dinner separation.
	if slot1 == TimeSlotOther || slot2 == TimeSlotOther {
		return false
	}

	return slot1 == slot2
}

// IsConflictWithReservation checks if a dining session starting at 'now' would conflict with a reservation at 'resTime' using default config
func IsConflictWithReservation(now, resTime time.Time) bool {
	return IsConflictWithReservationWithConfig(now, resTime, DefaultConfig)
}

// IsConflictWithReservationWithConfig checks if a dining session starting at 'now' would conflict with a reservation at 'resTime' using provided config
func IsConflictWithReservationWithConfig(now, resTime time.Time, config TimeSlotConfig) bool {
	// If same slot, it's a conflict
	if IsSameDiningTimeSlotWithConfig(now, resTime, config) {
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

// AreReservationsConflicting checks if two reservation times on the same day conflict using default config
func AreReservationsConflicting(t1, t2 time.Time) bool {
	return AreReservationsConflictingWithConfig(t1, t2, DefaultConfig)
}

// AreReservationsConflictingWithConfig checks if two reservation times on the same day conflict using provided config
func AreReservationsConflictingWithConfig(t1, t2 time.Time, config TimeSlotConfig) bool {
	if t1.Year() != t2.Year() || t1.Month() != t2.Month() || t1.Day() != t2.Day() {
		return false
	}
	slot1 := GetDiningTimeSlotWithConfig(t1, config)
	slot2 := GetDiningTimeSlotWithConfig(t2, config)

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
