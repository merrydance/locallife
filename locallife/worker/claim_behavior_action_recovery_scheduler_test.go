package worker_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"go.uber.org/mock/gomock"
)

type claimBehaviorActionRecoverySchedulerTestDistributor struct {
	worker.NoopTaskDistributor
	actionIDs []int64
}

func (d *claimBehaviorActionRecoverySchedulerTestDistributor) DistributeTaskClaimBehaviorAction(ctx context.Context, payload *worker.ClaimBehaviorActionPayload, opts ...asynq.Option) error {
	d.actionIDs = append(d.actionIDs, payload.ActionID)
	return nil
}

func TestClaimBehaviorActionRecoverySchedulerRunOnceReenqueuesRecoverableActions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &claimBehaviorActionRecoverySchedulerTestDistributor{}

	terminalDetail, err := json.Marshal(map[string]any{"terminal_failure": true})
	if err != nil {
		t.Fatalf("marshal terminal detail: %v", err)
	}
	staleCreatedAt := time.Now().Add(-31 * time.Minute)
	recentCreatedAt := time.Now().Add(-5 * time.Minute)

	store.EXPECT().ListBehaviorActionsByStatusAndType(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.ListBehaviorActionsByStatusAndTypeParams) ([]db.BehaviorAction, error) {
			switch {
			case arg.ActionType == "block" && arg.TargetEntity == "user" && arg.Status == "created":
				return []db.BehaviorAction{{ID: 1001, ActionType: "block", TargetEntity: "user", Status: "created"}}, nil
			case arg.ActionType == "block" && arg.TargetEntity == "user" && arg.Status == "running":
				return []db.BehaviorAction{{ID: 1007, ActionType: "block", TargetEntity: "user", Status: "running", CreatedAt: staleCreatedAt}}, nil
			case arg.ActionType == "block" && arg.TargetEntity == "merchant" && arg.Status == "failed":
				return []db.BehaviorAction{{ID: 1004, ActionType: "block", TargetEntity: "merchant", Status: "failed"}}, nil
			case arg.ActionType == "recovery" && arg.TargetEntity == "merchant" && arg.Status == "created":
				return []db.BehaviorAction{{ID: 1006, ActionType: "recovery", TargetEntity: "merchant", Status: "created"}}, nil
			case arg.ActionType == "release" && arg.TargetEntity == "merchant" && arg.Status == "created":
				return []db.BehaviorAction{{ID: 1005, ActionType: "release", TargetEntity: "merchant", Status: "created"}}, nil
			case arg.ActionType == "release" && arg.TargetEntity == "rider" && arg.Status == "running":
				return []db.BehaviorAction{{ID: 1008, ActionType: "release", TargetEntity: "rider", Status: "running", CreatedAt: staleCreatedAt}}, nil
			case arg.ActionType == "notify" && arg.TargetEntity == "merchant" && arg.Status == "failed":
				return []db.BehaviorAction{{ID: 1002, ActionType: "notify", TargetEntity: "merchant", Status: "failed"}}, nil
			case arg.ActionType == "notify" && arg.TargetEntity == "merchant" && arg.Status == "running":
				return []db.BehaviorAction{{ID: 1009, ActionType: "notify", TargetEntity: "merchant", Status: "running", CreatedAt: recentCreatedAt}}, nil
			case arg.ActionType == "notify" && arg.TargetEntity == "user" && arg.Status == "failed":
				return []db.BehaviorAction{{ID: 1003, ActionType: "notify", TargetEntity: "user", Status: "failed", Detail: terminalDetail}}, nil
			case arg.ActionType == "notify" && arg.TargetEntity == "user" && arg.Status == "running":
				return []db.BehaviorAction{{ID: 1010, ActionType: "notify", TargetEntity: "user", Status: "running", Detail: terminalDetail, CreatedAt: staleCreatedAt}}, nil
			default:
				return []db.BehaviorAction{}, nil
			}
		},
	).Times(30)

	scheduler := worker.NewClaimBehaviorActionRecoveryScheduler(store, distributor)
	scheduler.RunOnce()

	seen := map[int64]bool{}
	for _, actionID := range distributor.actionIDs {
		seen[actionID] = true
	}
	for _, expected := range []int64{1001, 1002, 1004, 1005, 1006, 1007, 1008} {
		if !seen[expected] {
			t.Fatalf("expected action %d to be re-enqueued, got %v", expected, distributor.actionIDs)
		}
	}
	for _, unexpected := range []int64{1003, 1009, 1010} {
		if seen[unexpected] {
			t.Fatalf("expected action %d not to be re-enqueued, got %v", unexpected, distributor.actionIDs)
		}
	}
}
