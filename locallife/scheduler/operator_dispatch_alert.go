package scheduler

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/rs/zerolog/log"
)

const (
	operatorPendingDispatchAlertThreshold = 3 * time.Minute
	operatorPendingDispatchAlertKey       = "pending_dispatch_3m"
	operatorPendingDispatchBatchLimit     = int32(100)
)

func (s *DataCleanupScheduler) enqueueOperatorPendingDispatchAlerts() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if s.taskDistributor == nil {
		log.Warn().Msg("skip operator pending dispatch alerts: task distributor not configured")
		return
	}

	cutoff := time.Now().Add(-operatorPendingDispatchAlertThreshold)
	deliveries, err := s.store.ListPendingDeliveriesBeforeWithoutAlert(ctx, db.ListPendingDeliveriesBeforeWithoutAlertParams{
		Status:    "pending",
		CreatedAt: cutoff,
		AlertKey:  operatorPendingDispatchAlertKey,
		Limit:     operatorPendingDispatchBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to list pending deliveries for operator dispatch alert")
		return
	}

	queuedCount := 0
	for _, delivery := range deliveries {
		if delivery.OrderID == 0 {
			continue
		}

		_, err := s.store.CreateDeliveryTimeoutAlert(ctx, db.CreateDeliveryTimeoutAlertParams{
			DeliveryID: delivery.ID,
			AlertKey:   operatorPendingDispatchAlertKey,
		})
		if err != nil {
			if db.ErrorCode(err) == db.UniqueViolation {
				continue
			}
			log.Error().Err(err).Int64("delivery_id", delivery.ID).Msg("failed to create delivery timeout alert ledger")
			continue
		}

		err = s.taskDistributor.DistributeTaskOperatorPendingDispatchAlert(ctx, &worker.OperatorPendingDispatchAlertPayload{
			DeliveryID:       delivery.ID,
			AlertKey:         operatorPendingDispatchAlertKey,
			ThresholdMinutes: int32(operatorPendingDispatchAlertThreshold / time.Minute),
		}, asynq.MaxRetry(10), asynq.Queue(worker.QueueCritical))
		if err != nil {
			log.Error().Err(err).Int64("delivery_id", delivery.ID).Msg("failed to enqueue operator pending dispatch alert task")
			if cleanupErr := s.store.DeleteDeliveryTimeoutAlert(ctx, db.DeleteDeliveryTimeoutAlertParams{
				DeliveryID: delivery.ID,
				AlertKey:   operatorPendingDispatchAlertKey,
			}); cleanupErr != nil {
				log.Error().Err(cleanupErr).Int64("delivery_id", delivery.ID).Msg("failed to rollback delivery timeout alert ledger after enqueue failure")
			}
			continue
		}

		queuedCount++
	}

	if queuedCount > 0 {
		log.Info().Int("queued_count", queuedCount).Msg("queued operator pending dispatch alerts")
	}
}
