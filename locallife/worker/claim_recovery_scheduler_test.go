package worker

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestClaimRecoverySchedulerRunOnceUsesPersistedDecisionAsAnchor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	scheduler := NewClaimRecoveryScheduler(store)

	recovery := db.ClaimRecovery{
		ID:             801,
		ClaimID:        901,
		OrderID:        701,
		RecoveryTarget: pgtype.Text{String: "merchant", Valid: true},
	}
	decision := db.BehaviorDecision{ID: 601}
	order := db.Order{ID: recovery.OrderID, MerchantID: 501}

	store.EXPECT().
		ListDueClaimRecoveries(gomock.Any(), gomock.Any()).
		Return([]db.ClaimRecovery{recovery}, nil)
	store.EXPECT().
		MarkClaimRecoveryOverdue(gomock.Any(), recovery.ID).
		Return(recovery, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), recovery.OrderID).
		Return(order, nil)
	store.EXPECT().
		SuspendMerchantTakeout(gomock.Any(), gomock.Any()).
		Return(nil)
	store.EXPECT().
		ListBehaviorDecisionsByOrder(gomock.Any(), pgtype.Int8{Int64: recovery.OrderID, Valid: true}).
		Return([]db.BehaviorDecision{decision}, nil)
	store.EXPECT().
		CreateBehaviorAction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateBehaviorActionParams) (db.BehaviorAction, error) {
			require.Equal(t, decision.ID, arg.DecisionID)
			require.Equal(t, "block", arg.ActionType)
			require.Equal(t, "merchant", arg.TargetEntity)
			require.Equal(t, "created", arg.Status)
			return db.BehaviorAction{ID: 1001}, nil
		})

	scheduler.runOnce(context.Background())
}
