package worker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskPaymentFactApplication_SkipsUnclaimableApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().ClaimExternalPaymentFactApplication(gomock.Any(), int64(901)).Return(db.ExternalPaymentFactApplication{}, db.ErrRecordNotFound)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payload, err := json.Marshal(worker.PaymentFactApplicationPayload{ApplicationID: 901})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessPaymentFactApplication, payload))
	require.NoError(t, err)
}

func TestProcessTaskPaymentFactApplication_RejectsMissingApplicationID(t *testing.T) {
	processor := worker.NewTestTaskProcessor(nil, nil, nil, nil)
	payload, err := json.Marshal(worker.PaymentFactApplicationPayload{})
	require.NoError(t, err)

	err = processor.ProcessTaskPaymentFactApplication(context.Background(), asynq.NewTask(worker.TaskProcessPaymentFactApplication, payload))
	require.Error(t, err)
	require.Contains(t, err.Error(), "application id is required")
}
