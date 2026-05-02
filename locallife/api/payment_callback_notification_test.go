package api

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSendPaymentSuccessNotificationLogsTaskDistributorFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	distributor := mockwk.NewMockTaskDistributor(ctrl)
	distributor.EXPECT().
		DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.AssignableToTypeOf(asynq.Queue("")), gomock.AssignableToTypeOf(asynq.MaxRetry(1))).
		Return(errors.New("enqueue notification failed"))

	server := &Server{taskDistributor: distributor}
	server.sendPaymentSuccessNotification(context.Background(), db.PaymentOrder{
		ID:           701,
		UserID:       801,
		OutTradeNo:   "PO701",
		Amount:       1234,
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
	}, "wx_tx_701", "ordinary_payment")

	require.Contains(t, logs.String(), "send payment success notification failed")
	require.Contains(t, logs.String(), "enqueue notification failed")
	require.Contains(t, logs.String(), "ordinary_payment")
}
