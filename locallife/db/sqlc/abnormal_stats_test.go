package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestBackfillAbnormalStatsDailyRebuildsExistingRowsIdempotently(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	order := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)

	completedOrder, err := testStore.UpdateOrderToCompleted(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", completedOrder.Status)

	statDay := completedOrder.CompletedAt.Time
	start := time.Date(statDay.Year(), statDay.Month(), statDay.Day(), 0, 0, 0, 0, statDay.Location())
	end := start.AddDate(0, 0, 1)
	params := BackfillAbnormalStatsDailyParams{
		CompletedAt:   pgtype.Timestamptz{Time: start, Valid: true},
		CompletedAt_2: pgtype.Timestamptz{Time: end, Valid: true},
	}

	require.NoError(t, testStore.BackfillAbnormalStatsDaily(ctx, params))
	require.NoError(t, testStore.BackfillAbnormalStatsDaily(ctx, params))

	summary, err := testStore.GetAbnormalStatsSummary(ctx, GetAbnormalStatsSummaryParams{
		EntityType: "user",
		EntityID:   user.ID,
		StatDate:   pgtype.Date{Time: start, Valid: true},
		StatDate_2: pgtype.Date{Time: start, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), summary.TotalOrders)
	require.Equal(t, int32(0), summary.AbnormalClaims)
}

func TestInsertBackfillAbnormalStatsDailyHandlesRowsCreatedAfterClear(t *testing.T) {
	ctx := context.Background()
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	order := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)

	completedOrder, err := testStore.UpdateOrderToCompleted(ctx, order.ID)
	require.NoError(t, err)

	statDay := completedOrder.CompletedAt.Time
	start := time.Date(statDay.Year(), statDay.Month(), statDay.Day(), 0, 0, 0, 0, statDay.Location())
	end := start.AddDate(0, 0, 1)

	err = testStore.InsertBackfillAbnormalStatsDaily(ctx, InsertBackfillAbnormalStatsDailyParams{
		StartAt: pgtype.Timestamptz{Time: start, Valid: true},
		EndAt:   pgtype.Timestamptz{Time: end, Valid: true},
	})
	require.NoError(t, err)

	summary, err := testStore.GetAbnormalStatsSummary(ctx, GetAbnormalStatsSummaryParams{
		EntityType: "user",
		EntityID:   user.ID,
		StatDate:   pgtype.Date{Time: start, Valid: true},
		StatDate_2: pgtype.Date{Time: start, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), summary.TotalOrders)
	require.Equal(t, int32(0), summary.AbnormalClaims)
}
