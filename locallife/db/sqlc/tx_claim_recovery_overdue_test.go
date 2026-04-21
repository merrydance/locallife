package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestMarkClaimRecoveryOverdueWithActionTx_RollsBackWhenDecisionAnchorMissing(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	order := createCompletedOrderForStats(t, user.ID, merchant.ID, 10000, "takeout", time.Now())
	claim := createRandomClaim(t, user.ID, order.ID)
	recovery, err := testStore.CreateClaimRecovery(context.Background(), CreateClaimRecoveryParams{
		ClaimID:          claim.ID,
		OrderID:          order.ID,
		DecisionID:       pgtype.Int8{},
		ResponsibleParty: "merchant",
		RecoveryTarget:   pgtype.Text{String: "merchant", Valid: true},
		RecoveryAmount:   3000,
		Status:           "pending",
		DueAt:            time.Now().Add(-time.Minute),
		DecisionSnapshot: []byte(`{"source":"test"}`),
		RecoveryBasis:    pgtype.Text{String: ClaimRecoveryBasisMerchantRecovery, Valid: true},
	})
	require.NoError(t, err)

	_, err = testStore.MarkClaimRecoveryOverdueWithActionTx(context.Background(), MarkClaimRecoveryOverdueWithActionTxParams{
		RecoveryID:    recovery.ID,
		SuspendUntil:  time.Now().Add(24 * time.Hour),
		OverdueRemark: "merchant recovery overdue block action created",
	})
	require.ErrorContains(t, err, "no behavior decision found")

	persistedRecovery, err := testStore.GetClaimRecoveryByClaimID(context.Background(), claim.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", persistedRecovery.Status)

	events, err := testStore.ListClaimRecoveryEventsByRecovery(context.Background(), recovery.ID)
	require.NoError(t, err)
	require.Len(t, events, 0)
	actions, err := testStore.ListBehaviorActionsByDecision(context.Background(), 0)
	require.NoError(t, err)
	require.Len(t, actions, 0)
}
