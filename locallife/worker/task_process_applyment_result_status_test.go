package worker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskApplymentResult_UsesNormalizedStatusFromRecoveryPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	store.EXPECT().
		GetMerchant(gomock.Any(), int64(100)).
		Return(db.Merchant{ID: 100, OwnerUserID: 1001, Name: "恢复商户"}, nil)

	distributor.EXPECT().
		DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{})).
		DoAndReturn(func(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(1001), payload.UserID)
			require.Equal(t, "微信支付开户待处理", payload.Title)
			require.Contains(t, payload.Content, "需要确认")
			return nil
		})

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)

	payloadBytes, err := json.Marshal(worker.ApplymentResultPayload{
		ApplymentID:     88,
		OutRequestNo:    "APPLY_RECOVERY_004",
		ApplymentStatus: "to_be_confirmed",
		SubjectType:     "merchant",
		SubjectID:       100,
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessApplymentResult, payloadBytes)
	err = processor.ProcessTaskApplymentResult(context.Background(), task)
	require.NoError(t, err)
}
