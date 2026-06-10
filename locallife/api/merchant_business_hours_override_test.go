package api

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestNextBusinessHoursSwitchAt(t *testing.T) {
	now := time.Date(2026, time.June, 10, 10, 30, 0, 0, time.Local)
	dayOfWeek := int32(now.Weekday())
	hours := []db.MerchantBusinessHour{
		businessHourForOverrideTest(dayOfWeek, 9, 0, 11, 0),
		businessHourForOverrideTest(dayOfWeek, 13, 0, 15, 0),
	}

	next, ok := nextBusinessHoursSwitchAt(now, hours)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, time.June, 10, 11, 0, 0, 0, time.Local), next)

	next, ok = nextBusinessHoursSwitchAt(time.Date(2026, time.June, 10, 12, 0, 0, 0, time.Local), hours)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, time.June, 10, 13, 0, 0, 0, time.Local), next)
}

func TestNextBusinessHoursSwitchAtSpecialDateOverridesWeekly(t *testing.T) {
	now := time.Date(2026, time.June, 10, 10, 30, 0, 0, time.Local)
	tomorrow := now.AddDate(0, 0, 1)
	hours := []db.MerchantBusinessHour{
		businessHourForOverrideTest(int32(now.Weekday()), 9, 0, 21, 0),
		closedSpecialBusinessHourForOverrideTest(now),
		businessHourForOverrideTest(int32(tomorrow.Weekday()), 9, 0, 21, 0),
	}

	next, ok := nextBusinessHoursSwitchAt(now, hours)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, time.June, 11, 9, 0, 0, 0, time.Local), next)
}

func TestNextBusinessHoursSwitchAtSpecialDateClosedAllDayStartsAtMidnight(t *testing.T) {
	now := time.Date(2026, time.June, 10, 10, 30, 0, 0, time.Local)
	tomorrow := now.AddDate(0, 0, 1)
	hours := []db.MerchantBusinessHour{
		businessHourForOverrideTest(int32(now.Weekday()), 0, 0, 24, 0),
		businessHourForOverrideTest(int32(tomorrow.Weekday()), 0, 0, 24, 0),
		closedSpecialBusinessHourForOverrideTest(tomorrow),
	}

	next, ok := nextBusinessHoursSwitchAt(now, hours)
	require.True(t, ok)
	require.Equal(t, startOfLocalDay(now).AddDate(0, 0, 1), next)
}

func TestNextBusinessHoursSwitchAtTreatsSpecialDateAsCalendarDate(t *testing.T) {
	location := time.FixedZone("UTC-8", -8*3600)
	now := time.Date(2026, time.June, 10, 10, 30, 0, 0, location)
	tomorrowDate := time.Date(2026, time.June, 11, 0, 0, 0, 0, time.UTC)
	hours := []db.MerchantBusinessHour{
		businessHourForOverrideTest(int32(now.Weekday()), 9, 0, 21, 0),
		businessHourForOverrideTest(int32(now.AddDate(0, 0, 1).Weekday()), 9, 0, 21, 0),
		{
			DayOfWeek: int32(now.AddDate(0, 0, 1).Weekday()),
			OpenTime:  timeOfDayForOverrideTest(0, 0),
			CloseTime: timeOfDayForOverrideTest(23, 59),
			IsClosed:  true,
			SpecialDate: pgtype.Date{
				Time:  tomorrowDate,
				Valid: true,
			},
		},
	}

	next, ok := nextBusinessHoursSwitchAt(now, hours)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, time.June, 10, 21, 0, 0, 0, location), next)
}

func TestNextBusinessHoursSwitchAtNoFutureSwitch(t *testing.T) {
	now := time.Date(2026, time.June, 10, 10, 30, 0, 0, time.Local)

	next, ok := nextBusinessHoursSwitchAt(now, nil)
	require.False(t, ok)
	require.True(t, next.IsZero())
}

func businessHourForOverrideTest(dayOfWeek int32, openHour, openMinute, closeHour, closeMinute int) db.MerchantBusinessHour {
	return db.MerchantBusinessHour{
		DayOfWeek: dayOfWeek,
		OpenTime:  timeOfDayForOverrideTest(openHour, openMinute),
		CloseTime: timeOfDayForOverrideTest(closeHour, closeMinute),
		IsClosed:  false,
	}
}

func closedSpecialBusinessHourForOverrideTest(date time.Time) db.MerchantBusinessHour {
	return db.MerchantBusinessHour{
		DayOfWeek: int32(date.Weekday()),
		OpenTime:  timeOfDayForOverrideTest(0, 0),
		CloseTime: timeOfDayForOverrideTest(23, 59),
		IsClosed:  true,
		SpecialDate: pgtype.Date{
			Time:  date,
			Valid: true,
		},
	}
}

func timeOfDayForOverrideTest(hour, minute int) pgtype.Time {
	return pgtype.Time{
		Microseconds: int64(hour*3600+minute*60) * 1000000,
		Valid:        true,
	}
}
