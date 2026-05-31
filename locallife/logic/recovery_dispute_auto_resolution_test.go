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
