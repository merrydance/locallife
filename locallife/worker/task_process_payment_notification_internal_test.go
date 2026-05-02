package worker

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

type sendNotificationFailureDistributor struct {
	NoopTaskDistributor
}

func (sendNotificationFailureDistributor) DistributeTaskSendNotification(context.Context, *SendNotificationPayload, ...asynq.Option) error {
	return errors.New("enqueue notification failed")
}

func TestDistributeTaskSendNotificationWithLogLogsFailure(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := log.Logger
	log.Logger = zerolog.New(&logs)
	t.Cleanup(func() { log.Logger = previousLogger })

	processor := &RedisTaskProcessor{distributor: sendNotificationFailureDistributor{}}
	processor.distributeTaskSendNotificationWithLog(context.Background(), &SendNotificationPayload{
		UserID:      101,
		Type:        "system",
		Title:       "微信支付开户待处理",
		RelatedType: "applyment",
		RelatedID:   202,
	}, "send applyment notification failed", asynq.Queue(QueueDefault))

	require.Contains(t, logs.String(), "send applyment notification failed")
	require.Contains(t, logs.String(), "enqueue notification failed")
	require.Contains(t, logs.String(), "applyment")
}
