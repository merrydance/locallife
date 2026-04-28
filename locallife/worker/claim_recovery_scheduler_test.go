package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type claimRecoverySchedulerTestDistributor struct {
	NoopTaskDistributor
	called  bool
	payload *ClaimBehaviorActionPayload
}

func (d *claimRecoverySchedulerTestDistributor) DistributeTaskClaimBehaviorAction(ctx context.Context, payload *ClaimBehaviorActionPayload, opts ...asynq.Option) error {
	d.called = true
	d.payload = payload
	return nil
}

func TestClaimRecoverySchedulerRunOnceUsesPersistedDecisionAsAnchor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &claimRecoverySchedulerTestDistributor{}
	scheduler := NewClaimRecoveryScheduler(store, distributor)

	recovery := db.ClaimRecovery{
		ID:             801,
		ClaimID:        901,
		OrderID:        701,
		DecisionID:     pgtype.Int8{Int64: 601, Valid: true},
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}
	action := db.BehaviorAction{ID: 1001, DecisionID: recovery.DecisionID.Int64, ActionType: "block", TargetEntity: "merchant", Status: "created"}

	store.EXPECT().
		ListDueClaimRecoveries(gomock.Any(), gomock.Any()).
		Return([]db.ClaimRecovery{recovery}, nil)
	store.EXPECT().
		MarkClaimRecoveryOverdueWithActionTx(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.MarkClaimRecoveryOverdueWithActionTxParams) (db.MarkClaimRecoveryOverdueWithActionTxResult, error) {
			require.Equal(t, recovery.ID, arg.RecoveryID)
			require.False(t, arg.SuspendUntil.IsZero())
			return db.MarkClaimRecoveryOverdueWithActionTxResult{Recovery: recovery, Action: action}, nil
		})

	scheduler.runOnce(context.Background())
	require.True(t, distributor.called)
	require.NotNil(t, distributor.payload)
	require.Equal(t, int64(1001), distributor.payload.ActionID)
}

func TestClaimRecoverySchedulerRunOnceLeavesRecoveryRetryableWhenAtomicTransitionFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &claimRecoverySchedulerTestDistributor{}
	scheduler := NewClaimRecoveryScheduler(store, distributor)

	recovery := db.ClaimRecovery{ID: 901, ClaimID: 902, OrderID: 903, RecoveryTarget: pgtype.Text{String: "merchant", Valid: true}}

	store.EXPECT().
		ListDueClaimRecoveries(gomock.Any(), gomock.Any()).
		Return([]db.ClaimRecovery{recovery}, nil)
	store.EXPECT().
		MarkClaimRecoveryOverdueWithActionTx(gomock.Any(), gomock.Any()).
		Return(db.MarkClaimRecoveryOverdueWithActionTxResult{}, errors.New("decision anchor unavailable"))

	scheduler.runOnce(context.Background())
	require.False(t, distributor.called)
}
