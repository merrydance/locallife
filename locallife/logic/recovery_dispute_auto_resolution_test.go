package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEvaluateAutomaticRecoveryDisputeResolutionUsesRecoveryDisputeContext(t *testing.T) {
	claimID := int64(10)
	orderID := int64(20)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetClaimRecoveryContextByClaimIDAndTarget(gomock.Any(), db.GetClaimRecoveryContextByClaimIDAndTargetParams{
			ClaimID:        claimID,
			RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
		}).
		Times(1).
		Return(db.GetClaimRecoveryContextByClaimIDAndTargetRow{
			ClaimID:        claimID,
			OrderID:        orderID,
			ClaimCreatedAt: time.Now(),
		}, nil)
	store.EXPECT().
		ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: orderID, Valid: true}).
		Times(1).
		Return([]db.BehaviorDecision{{
			ID:               30,
			ClaimID:          pgtype.Int8{Int64: claimID, Valid: true},
			ResponsibleParty: "merchant",
			EffectiveStatus:  db.BehaviorEffectiveStatusEffective,
		}}, nil)

	resolution, err := EvaluateAutomaticRecoveryDisputeResolution(context.Background(), store, db.RecoveryDispute{
		ID:            40,
		ClaimID:       claimID,
		AppellantType: "merchant",
	})

	require.NoError(t, err)
	require.Equal(t, "rejected", resolution.Status)
	require.True(t, resolution.DecisionID.Valid)
	require.Equal(t, int64(30), resolution.DecisionID.Int64)
}

func TestDeriveAutomaticRecoveryDisputeResolutionUsesClaimScopedDecision(t *testing.T) {
	recoveryDispute := db.RecoveryDispute{
		ID:            1,
		ClaimID:       100,
		AppellantType: "merchant",
	}

	resolution := DeriveAutomaticRecoveryDisputeResolution(recoveryDispute, []db.BehaviorDecision{
		{
			ID:                 201,
			ClaimID:            pgtype.Int8{Int64: 999, Valid: true},
			ResponsibleParty:   "rider",
			CompensationSource: "platform",
		},
		{
			ID:                 202,
			ClaimID:            pgtype.Int8{Int64: 100, Valid: true},
			ResponsibleParty:   "merchant",
			CompensationSource: "merchant",
		},
	})

	require.Equal(t, "rejected", resolution.Status)
	require.Equal(t, int64(202), resolution.DecisionID.Int64)
	require.True(t, resolution.DecisionID.Valid)
	require.Equal(t, "系统复核确认最新行为判责仍指向当前申诉方，维持原判。", resolution.ReviewNotes)
}

func TestDeriveAutomaticRecoveryDisputeResolutionApprovesInactiveDecision(t *testing.T) {
	recoveryDispute := db.RecoveryDispute{
		ID:            2,
		ClaimID:       101,
		AppellantType: "rider",
	}

	resolution := DeriveAutomaticRecoveryDisputeResolution(recoveryDispute, []db.BehaviorDecision{
		{
			ID:               301,
			ClaimID:          pgtype.Int8{Int64: 101, Valid: true},
			ResponsibleParty: "rider",
			EffectiveStatus:  "overturned",
		},
	})

	require.Equal(t, "approved", resolution.Status)
	require.Equal(t, int64(301), resolution.DecisionID.Int64)
	require.Equal(t, "系统复核发现相关行为判责已失效，自动撤销原追偿安排。", resolution.ReviewNotes)
}
