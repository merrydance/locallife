package worker_test

import (
	"encoding/json"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"go.uber.org/mock/gomock"
)

func TestClaimBehaviorActionRecoverySchedulerRunOnceReenqueuesRecoverableActions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	terminalDetail, err := json.Marshal(map[string]any{"terminal_failure": true})
	if err != nil {
		t.Fatalf("marshal terminal detail: %v", err)
	}

	store.EXPECT().ListBehaviorActionsByStatusAndType(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.ListBehaviorActionsByStatusAndTypeParams) ([]db.BehaviorAction, error) {
			switch {
			case arg.ActionType == "block" && arg.TargetEntity == "user" && arg.Status == "created":
				return []db.BehaviorAction{{ID: 1001, ActionType: "block", TargetEntity: "user", Status: "created"}}, nil
			case arg.ActionType == "notify" && arg.TargetEntity == "merchant" && arg.Status == "failed":
				return []db.BehaviorAction{{ID: 1002, ActionType: "notify", TargetEntity: "merchant", Status: "failed"}}, nil
			case arg.ActionType == "notify" && arg.TargetEntity == "user" && arg.Status == "failed":
				return []db.BehaviorAction{{ID: 1003, ActionType: "notify", TargetEntity: "user", Status: "failed", Detail: terminalDetail}}, nil
			default:
				return []db.BehaviorAction{}, nil
			}
		},
	).Times(8)

	distributor.EXPECT().DistributeTaskClaimBehaviorAction(gomock.Any(), &worker.ClaimBehaviorActionPayload{ActionID: 1001}, gomock.Any(), gomock.Any()).Return(nil)
	distributor.EXPECT().DistributeTaskClaimBehaviorAction(gomock.Any(), &worker.ClaimBehaviorActionPayload{ActionID: 1002}, gomock.Any(), gomock.Any()).Return(nil)

	scheduler := worker.NewClaimBehaviorActionRecoveryScheduler(store, distributor)
	scheduler.RunOnce()
}
