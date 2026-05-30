package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type BackfillAbnormalStatsDailyParams struct {
	CompletedAt   pgtype.Timestamptz `json:"completed_at"`
	CompletedAt_2 pgtype.Timestamptz `json:"completed_at_2"`
}

// BackfillAbnormalStatsDaily atomically rebuilds abnormal stats for the requested date range.
func (store *SQLStore) BackfillAbnormalStatsDaily(ctx context.Context, arg BackfillAbnormalStatsDailyParams) error {
	return store.execTx(ctx, func(q *Queries) error {
		if err := q.ClearAbnormalStatsDailyForBackfill(ctx, ClearAbnormalStatsDailyForBackfillParams{
			StartAt: pgtype.Date{Time: arg.CompletedAt.Time, Valid: arg.CompletedAt.Valid},
			EndAt:   pgtype.Date{Time: arg.CompletedAt_2.Time, Valid: arg.CompletedAt_2.Valid},
		}); err != nil {
			return err
		}

		return q.InsertBackfillAbnormalStatsDaily(ctx, InsertBackfillAbnormalStatsDailyParams{
			StartAt: arg.CompletedAt,
			EndAt:   arg.CompletedAt_2,
		})
	})
}
