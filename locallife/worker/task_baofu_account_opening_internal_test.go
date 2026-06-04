package worker

import (
	"context"
	"testing"

	"github.com/hibiken/asynq"
)

func TestDistributeTaskProcessBaofuAccountOpeningTreatsDuplicateTaskAsSuccess(t *testing.T) {
	distributor := &RedisTaskDistributor{
		client: stubTaskEnqueueClient{
			enqueueFn: func(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
				return nil, asynq.ErrDuplicateTask
			},
		},
	}

	if err := distributor.DistributeTaskProcessBaofuAccountOpening(context.Background(), &BaofuAccountOpeningPayload{FlowID: 901}); err != nil {
		t.Fatalf("expected duplicate baofu account opening enqueue to be treated as success, got %v", err)
	}
}
