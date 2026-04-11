package api

import (
	"errors"
	"fmt"
	"time"
)

// parseDateRange parses start/end date strings and validates the range.
// Date format: YYYY-MM-DD. maxDays <= 0 disables range length check.
func parseDateRange(startDateStr, endDateStr string, maxDays int) (time.Time, time.Time, error) {
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid start_date format, expected YYYY-MM-DD")
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid end_date format, expected YYYY-MM-DD")
	}

	if err := validateDateRange(startDate, endDate, maxDays); err != nil {
		return time.Time{}, time.Time{}, err
	}

	return startDate, endDate, nil
}

// parseISODate parses a YYYY-MM-DD date string and returns a custom error message on failure.
func parseISODate(dateStr, errMsg string) (time.Time, error) {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		if errMsg == "" {
			return time.Time{}, err
		}
		return time.Time{}, errors.New(errMsg)
	}
	return date, nil
}

// validateDateRange validates date range boundaries.
// Returns error if startDate > endDate or range exceeds maxDays.
func validateDateRange(startDate, endDate time.Time, maxDays int) error {
	if startDate.After(endDate) {
		return errors.New("start_date must be before or equal to end_date")
	}
	if maxDays > 0 && endDate.Sub(startDate).Hours()/24 > float64(maxDays) {
		return fmt.Errorf("date range cannot exceed %d days", maxDays)
	}
	return nil
}