package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// SetBusinessHoursTxParams contains input parameters for setting merchant business hours
type SetBusinessHoursTxParams struct {
	MerchantID int64
	Hours      []BusinessHourInput
}

// BusinessHourInput represents a business hour input
type BusinessHourInput struct {
	DayOfWeek int32
	OpenTime  pgtype.Time
	CloseTime pgtype.Time
	IsClosed  bool
}

// SetBusinessHoursTxResult contains the result of setting business hours
type SetBusinessHoursTxResult struct {
	Hours []MerchantBusinessHour
}

// SetBusinessHoursTx replaces all business hours for a merchant in a single transaction.
// This ensures atomicity: either all hours are replaced or none are.
func (store *SQLStore) SetBusinessHoursTx(ctx context.Context, arg SetBusinessHoursTxParams) (SetBusinessHoursTxResult, error) {
	var result SetBusinessHoursTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		// Step 1: Delete all existing business hours
		err := q.DeleteMerchantBusinessHours(ctx, arg.MerchantID)
		if err != nil {
			return fmt.Errorf("delete business hours: %w", err)
		}

		// Step 2: Create new business hours
		result.Hours = make([]MerchantBusinessHour, 0, len(arg.Hours))
		for _, h := range arg.Hours {
			bh, err := q.CreateBusinessHour(ctx, CreateBusinessHourParams{
				MerchantID: arg.MerchantID,
				DayOfWeek:  h.DayOfWeek,
				OpenTime:   h.OpenTime,
				CloseTime:  h.CloseTime,
				IsClosed:   h.IsClosed,
			})
			if err != nil {
				return fmt.Errorf("create business hour for day %d: %w", h.DayOfWeek, err)
			}
			result.Hours = append(result.Hours, bh)
		}

		return nil
	})

	return result, err
}
