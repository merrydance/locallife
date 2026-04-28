package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func randomProfitSharingReceiverTargetArg(ownerType string, ownerID int64, desiredState string) UpsertProfitSharingReceiverTargetParams {
	return UpsertProfitSharingReceiverTargetParams{
		Provider:        ExternalPaymentProviderWechat,
		Channel:         PaymentChannelEcommerce,
		OwnerType:       ownerType,
		OwnerID:         ownerID,
		ReceiverType:    ProfitSharingReceiverTypePersonalOpenID,
		Appid:           "wx" + util.RandomString(24),
		AccountHash:     "sha256:" + util.RandomString(32),
		DisplayNameHash: pgtype.Text{String: "sha256:" + util.RandomString(32), Valid: true},
		DesiredState:    desiredState,
	}
}

func createRandomProfitSharingReceiverTarget(t *testing.T, desiredState string) ProfitSharingReceiverTarget {
	t.Helper()

	target, err := testStore.UpsertProfitSharingReceiverTarget(context.Background(), randomProfitSharingReceiverTargetArg(
		ProfitSharingReceiverOwnerTypeRider,
		time.Now().UnixNano(),
		desiredState,
	))
	require.NoError(t, err)
	require.NotZero(t, target.ID)
	require.Equal(t, ProfitSharingReceiverSyncStatusPending, target.SyncStatus)
	require.Equal(t, desiredState, target.DesiredState)
	require.False(t, target.NextRetryAt.Valid)

	return target
}

func claimTestProfitSharingReceiverTarget(t *testing.T, targetID int64, now time.Time) ProfitSharingReceiverTarget {
	t.Helper()

	claimed, err := testStore.ClaimProfitSharingReceiverTarget(context.Background(), ClaimProfitSharingReceiverTargetParams{
		NowAt: pgtype.Timestamptz{Time: now, Valid: true},
		ID:    targetID,
	})
	require.NoError(t, err)
	require.Equal(t, ProfitSharingReceiverSyncStatusProcessing, claimed.SyncStatus)

	return claimed
}

func TestUpsertProfitSharingReceiverTarget_DedupesAndResetsPending(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	arg := randomProfitSharingReceiverTargetArg(
		ProfitSharingReceiverOwnerTypeOperator,
		time.Now().UnixNano(),
		ProfitSharingReceiverDesiredStatePresent,
	)

	created, err := testStore.UpsertProfitSharingReceiverTarget(ctx, arg)
	require.NoError(t, err)

	claimTestProfitSharingReceiverTarget(t, created.ID, now)
	synced, err := testStore.MarkProfitSharingReceiverTargetSynced(ctx, MarkProfitSharingReceiverTargetSyncedParams{
		SyncedAt: pgtype.Timestamptz{Time: now, Valid: true},
		ID:       created.ID,
	})
	require.NoError(t, err)
	require.Equal(t, ProfitSharingReceiverSyncStatusSynced, synced.SyncStatus)
	require.True(t, synced.SyncedAt.Valid)

	arg.DesiredState = ProfitSharingReceiverDesiredStateAbsent
	arg.DisplayNameHash = pgtype.Text{String: "sha256:" + util.RandomString(32), Valid: true}
	updated, err := testStore.UpsertProfitSharingReceiverTarget(ctx, arg)
	require.NoError(t, err)

	require.Equal(t, created.ID, updated.ID)
	require.Equal(t, ProfitSharingReceiverDesiredStateAbsent, updated.DesiredState)
	require.Equal(t, ProfitSharingReceiverSyncStatusPending, updated.SyncStatus)
	require.Equal(t, int32(1), updated.AttemptCount)
	require.False(t, updated.NextRetryAt.Valid)
	require.False(t, updated.LastErrorCode.Valid)
	require.False(t, updated.LastErrorMessage.Valid)
	require.False(t, updated.SyncedAt.Valid)
	require.False(t, updated.SkippedAt.Valid)
}

func TestClaimPendingProfitSharingReceiverTargets_RespectsRetryTime(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	pendingTarget := createRandomProfitSharingReceiverTarget(t, ProfitSharingReceiverDesiredStatePresent)
	dueTarget := createRandomProfitSharingReceiverTarget(t, ProfitSharingReceiverDesiredStateAbsent)
	futureTarget := createRandomProfitSharingReceiverTarget(t, ProfitSharingReceiverDesiredStatePresent)

	claimTestProfitSharingReceiverTarget(t, dueTarget.ID, now)
	dueRetryAt := now.Add(-time.Minute)
	_, err := testStore.MarkProfitSharingReceiverTargetFailed(ctx, MarkProfitSharingReceiverTargetFailedParams{
		LastErrorCode:    pgtype.Text{String: "TIMEOUT", Valid: true},
		LastErrorMessage: pgtype.Text{String: "wechat request timeout", Valid: true},
		NextRetryAt:      pgtype.Timestamptz{Time: dueRetryAt, Valid: true},
		ID:               dueTarget.ID,
	})
	require.NoError(t, err)

	claimTestProfitSharingReceiverTarget(t, futureTarget.ID, now)
	futureRetryAt := now.Add(time.Hour)
	_, err = testStore.MarkProfitSharingReceiverTargetFailed(ctx, MarkProfitSharingReceiverTargetFailedParams{
		LastErrorCode:    pgtype.Text{String: "RATE_LIMIT", Valid: true},
		LastErrorMessage: pgtype.Text{String: "wechat rate limit", Valid: true},
		NextRetryAt:      pgtype.Timestamptz{Time: futureRetryAt, Valid: true},
		ID:               futureTarget.ID,
	})
	require.NoError(t, err)

	claimed, err := testStore.ClaimPendingProfitSharingReceiverTargets(ctx, ClaimPendingProfitSharingReceiverTargetsParams{
		NowAt:      pgtype.Timestamptz{Time: now, Valid: true},
		LimitCount: 1000,
	})
	require.NoError(t, err)

	claimedIDs := make(map[int64]ClaimPendingProfitSharingReceiverTargetsRow, len(claimed))
	for _, row := range claimed {
		claimedIDs[row.ID] = row
	}
	require.Contains(t, claimedIDs, pendingTarget.ID)
	require.Contains(t, claimedIDs, dueTarget.ID)
	require.NotContains(t, claimedIDs, futureTarget.ID)
	require.Equal(t, ProfitSharingReceiverSyncStatusProcessing, claimedIDs[pendingTarget.ID].SyncStatus)
	require.Equal(t, ProfitSharingReceiverSyncStatusProcessing, claimedIDs[dueTarget.ID].SyncStatus)

	_, err = testStore.ClaimProfitSharingReceiverTarget(ctx, ClaimProfitSharingReceiverTargetParams{
		NowAt: pgtype.Timestamptz{Time: now, Valid: true},
		ID:    futureTarget.ID,
	})
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestListRetryableProfitSharingReceiverTargetsByOwnerType_FiltersOwnerAndRetryTime(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	operatorTarget, err := testStore.UpsertProfitSharingReceiverTarget(ctx, randomProfitSharingReceiverTargetArg(
		ProfitSharingReceiverOwnerTypeOperator,
		time.Now().UnixNano(),
		ProfitSharingReceiverDesiredStatePresent,
	))
	require.NoError(t, err)

	riderTarget := createRandomProfitSharingReceiverTarget(t, ProfitSharingReceiverDesiredStatePresent)
	futureOperatorTarget, err := testStore.UpsertProfitSharingReceiverTarget(ctx, randomProfitSharingReceiverTargetArg(
		ProfitSharingReceiverOwnerTypeOperator,
		time.Now().UnixNano()+1,
		ProfitSharingReceiverDesiredStateAbsent,
	))
	require.NoError(t, err)
	claimTestProfitSharingReceiverTarget(t, futureOperatorTarget.ID, now)
	_, err = testStore.MarkProfitSharingReceiverTargetFailed(ctx, MarkProfitSharingReceiverTargetFailedParams{
		LastErrorCode:    pgtype.Text{String: "RATE_LIMIT", Valid: true},
		LastErrorMessage: pgtype.Text{String: "wechat rate limit", Valid: true},
		NextRetryAt:      pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true},
		ID:               futureOperatorTarget.ID,
	})
	require.NoError(t, err)

	retryable, err := testStore.ListRetryableProfitSharingReceiverTargetsByOwnerType(ctx, ListRetryableProfitSharingReceiverTargetsByOwnerTypeParams{
		OwnerType:  ProfitSharingReceiverOwnerTypeOperator,
		NowAt:      pgtype.Timestamptz{Time: now, Valid: true},
		LimitCount: 1000,
	})
	require.NoError(t, err)

	retryableIDs := make(map[int64]struct{}, len(retryable))
	for _, row := range retryable {
		retryableIDs[row.ID] = struct{}{}
		require.Equal(t, ProfitSharingReceiverOwnerTypeOperator, row.OwnerType)
	}
	require.Contains(t, retryableIDs, operatorTarget.ID)
	require.NotContains(t, retryableIDs, riderTarget.ID)
	require.NotContains(t, retryableIDs, futureOperatorTarget.ID)
}

func TestListProfitSharingReceiverTargets_FiltersAndCounts(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := time.Now().UnixNano()

	operatorTarget, err := testStore.UpsertProfitSharingReceiverTarget(ctx, randomProfitSharingReceiverTargetArg(
		ProfitSharingReceiverOwnerTypeOperator,
		ownerID,
		ProfitSharingReceiverDesiredStatePresent,
	))
	require.NoError(t, err)
	claimTestProfitSharingReceiverTarget(t, operatorTarget.ID, now)
	failedTarget, err := testStore.MarkProfitSharingReceiverTargetFailed(ctx, MarkProfitSharingReceiverTargetFailedParams{
		LastErrorCode:    pgtype.Text{String: "PARAM_ERROR", Valid: true},
		LastErrorMessage: pgtype.Text{String: "sanitized invalid receiver", Valid: true},
		NextRetryAt:      pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
		ID:               operatorTarget.ID,
	})
	require.NoError(t, err)

	_, err = testStore.UpsertProfitSharingReceiverTarget(ctx, randomProfitSharingReceiverTargetArg(
		ProfitSharingReceiverOwnerTypeRider,
		ownerID,
		ProfitSharingReceiverDesiredStatePresent,
	))
	require.NoError(t, err)

	params := ListProfitSharingReceiverTargetsParams{
		OwnerType:   pgtype.Text{String: ProfitSharingReceiverOwnerTypeOperator, Valid: true},
		OwnerID:     pgtype.Int8{Int64: ownerID, Valid: true},
		SyncStatus:  pgtype.Text{String: ProfitSharingReceiverSyncStatusFailed, Valid: true},
		LimitCount:  10,
		OffsetCount: 0,
	}
	targets, err := testStore.ListProfitSharingReceiverTargets(ctx, params)
	require.NoError(t, err)
	require.Len(t, targets, 1)
	require.Equal(t, failedTarget.ID, targets[0].ID)
	require.Equal(t, ProfitSharingReceiverSyncStatusFailed, targets[0].SyncStatus)

	total, err := testStore.CountProfitSharingReceiverTargets(ctx, CountProfitSharingReceiverTargetsParams{
		OwnerType:  params.OwnerType,
		OwnerID:    params.OwnerID,
		SyncStatus: params.SyncStatus,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
}

func TestProfitSharingReceiverAttempt_StateTransitions(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	target := createRandomProfitSharingReceiverTarget(t, ProfitSharingReceiverDesiredStatePresent)

	attempt, err := testStore.CreateProfitSharingReceiverAttempt(ctx, CreateProfitSharingReceiverAttemptParams{
		TargetID:  target.ID,
		Action:    ProfitSharingReceiverAttemptActionEnsure,
		Status:    ProfitSharingReceiverAttemptStatusProcessing,
		StartedAt: now,
	})
	require.NoError(t, err)
	require.Equal(t, ProfitSharingReceiverAttemptStatusProcessing, attempt.Status)
	require.False(t, attempt.IdempotentSuccess)

	succeeded, err := testStore.MarkProfitSharingReceiverAttemptSucceeded(ctx, MarkProfitSharingReceiverAttemptSucceededParams{
		IdempotentSuccess: true,
		FinishedAt:        pgtype.Timestamptz{Time: now.Add(time.Second), Valid: true},
		ID:                attempt.ID,
	})
	require.NoError(t, err)
	require.Equal(t, ProfitSharingReceiverAttemptStatusSucceeded, succeeded.Status)
	require.True(t, succeeded.IdempotentSuccess)
	require.False(t, succeeded.ErrorCode.Valid)
	require.True(t, succeeded.FinishedAt.Valid)

	attempts, err := testStore.ListProfitSharingReceiverAttemptsByTarget(ctx, target.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	require.Equal(t, attempt.ID, attempts[0].ID)
}

func TestListProfitSharingReceiverAttemptsByTargetPaginated_Counts(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	target := createRandomProfitSharingReceiverTarget(t, ProfitSharingReceiverDesiredStatePresent)

	firstAttempt, err := testStore.CreateProfitSharingReceiverAttempt(ctx, CreateProfitSharingReceiverAttemptParams{
		TargetID:  target.ID,
		Action:    ProfitSharingReceiverAttemptActionEnsure,
		Status:    ProfitSharingReceiverAttemptStatusProcessing,
		StartedAt: now,
	})
	require.NoError(t, err)
	secondAttempt, err := testStore.CreateProfitSharingReceiverAttempt(ctx, CreateProfitSharingReceiverAttemptParams{
		TargetID:  target.ID,
		Action:    ProfitSharingReceiverAttemptActionEnsure,
		Status:    ProfitSharingReceiverAttemptStatusProcessing,
		StartedAt: now.Add(time.Second),
	})
	require.NoError(t, err)

	total, err := testStore.CountProfitSharingReceiverAttemptsByTarget(ctx, target.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)

	latest, err := testStore.ListProfitSharingReceiverAttemptsByTargetPaginated(ctx, ListProfitSharingReceiverAttemptsByTargetPaginatedParams{
		TargetID:    target.ID,
		LimitCount:  1,
		OffsetCount: 0,
	})
	require.NoError(t, err)
	require.Len(t, latest, 1)
	require.Equal(t, secondAttempt.ID, latest[0].ID)

	older, err := testStore.ListProfitSharingReceiverAttemptsByTargetPaginated(ctx, ListProfitSharingReceiverAttemptsByTargetPaginatedParams{
		TargetID:    target.ID,
		LimitCount:  1,
		OffsetCount: 1,
	})
	require.NoError(t, err)
	require.Len(t, older, 1)
	require.Equal(t, firstAttempt.ID, older[0].ID)
}
