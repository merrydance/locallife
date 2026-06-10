package api

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const businessHourOverrideLookaheadDays = 8

func (server *Server) manualOpenStatusUntil(ctx context.Context, merchant db.Merchant) (pgtype.Timestamptz, error) {
	if !merchant.AutoOpenByBusinessHours {
		return pgtype.Timestamptz{}, nil
	}

	clock, err := server.store.GetDatabaseLocalClock(ctx)
	if err != nil {
		return pgtype.Timestamptz{}, err
	}

	hours, err := server.store.ListMerchantBusinessHoursAll(ctx, merchant.ID)
	if err != nil {
		return pgtype.Timestamptz{}, err
	}

	if next, ok := nextBusinessHoursSwitchAt(databaseLocalNow(clock), hours); ok {
		return pgtype.Timestamptz{Time: next, Valid: true}, nil
	}
	return pgtype.Timestamptz{Valid: true, InfinityModifier: pgtype.Infinity}, nil
}

func databaseLocalNow(clock db.GetDatabaseLocalClockRow) time.Time {
	location := time.Local
	if clock.TimeZone != "" {
		if loaded, err := time.LoadLocation(clock.TimeZone); err == nil {
			location = loaded
		}
	}

	return time.Date(int(clock.CurrentYear), time.Month(clock.CurrentMonth), int(clock.CurrentDay), 0, 0, 0, 0, location).
		Add(time.Duration(clock.LocalTimeMicros) * time.Microsecond)
}

func nextBusinessHoursSwitchAt(now time.Time, hours []db.MerchantBusinessHour) (time.Time, bool) {
	current := businessHoursShouldOpenAt(now, hours)
	candidates := businessHoursSwitchCandidates(now, hours)

	for _, candidate := range candidates {
		if !candidate.After(now) {
			continue
		}
		if businessHoursShouldOpenAt(candidate, hours) != current {
			return candidate, true
		}
	}

	return time.Time{}, false
}

func businessHoursSwitchCandidates(now time.Time, hours []db.MerchantBusinessHour) []time.Time {
	dateSet := make(map[time.Time]struct{})
	start := startOfLocalDay(now)
	for offset := 0; offset <= businessHourOverrideLookaheadDays; offset++ {
		dateSet[start.AddDate(0, 0, offset)] = struct{}{}
	}
	for _, hour := range hours {
		if !hour.SpecialDate.Valid {
			continue
		}
		date := businessDateInLocation(hour.SpecialDate.Time, now.Location())
		if date.Before(start) {
			continue
		}
		dateSet[date] = struct{}{}
		dateSet[date.AddDate(0, 0, 1)] = struct{}{}
	}

	candidateSet := make(map[time.Time]struct{})
	for date := range dateSet {
		candidateSet[date] = struct{}{}
		for _, row := range effectiveBusinessHoursForDate(date, hours) {
			if row.IsClosed || !row.OpenTime.Valid || !row.CloseTime.Valid {
				continue
			}
			candidateSet[date.Add(time.Duration(row.OpenTime.Microseconds)*time.Microsecond)] = struct{}{}
			candidateSet[date.Add(time.Duration(row.CloseTime.Microseconds)*time.Microsecond)] = struct{}{}
		}
	}

	candidates := make([]time.Time, 0, len(candidateSet))
	for candidate := range candidateSet {
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Before(candidates[j])
	})
	return candidates
}

func businessHoursShouldOpenAt(at time.Time, hours []db.MerchantBusinessHour) bool {
	rows := effectiveBusinessHoursForDate(startOfLocalDay(at), hours)
	if len(rows) == 0 {
		return false
	}

	micros := localTimeMicros(at)
	for _, row := range rows {
		if row.IsClosed {
			return false
		}
	}
	for _, row := range rows {
		if !row.OpenTime.Valid || !row.CloseTime.Valid {
			continue
		}
		if micros >= row.OpenTime.Microseconds && micros < row.CloseTime.Microseconds {
			return true
		}
	}
	return false
}

func effectiveBusinessHoursForDate(date time.Time, hours []db.MerchantBusinessHour) []db.MerchantBusinessHour {
	specialRows := make([]db.MerchantBusinessHour, 0)
	weeklyRows := make([]db.MerchantBusinessHour, 0)
	dayOfWeek := int32(date.Weekday())

	for _, row := range hours {
		if row.SpecialDate.Valid {
			rowDate := businessDateInLocation(row.SpecialDate.Time, date.Location())
			if rowDate.Equal(date) {
				specialRows = append(specialRows, row)
			}
			continue
		}
		if row.DayOfWeek == dayOfWeek {
			weeklyRows = append(weeklyRows, row)
		}
	}
	if len(specialRows) > 0 {
		return specialRows
	}
	return weeklyRows
}

func startOfLocalDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func businessDateInLocation(t time.Time, location *time.Location) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, location)
}

func localTimeMicros(t time.Time) int64 {
	return int64(t.Hour())*3600*1000000 +
		int64(t.Minute())*60*1000000 +
		int64(t.Second())*1000000 +
		int64(t.Nanosecond()/1000)
}
