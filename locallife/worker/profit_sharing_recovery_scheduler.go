package worker

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	profitSharingRecoveryCron       = "*/10 * * * *"
	profitSharingRecoveryBatchLimit = int32(200)
	profitSharingRecoveryMinAge     = 10 * time.Minute
)

// ProfitSharingRecoveryScheduler scans failed/stale profit sharing orders and re-enqueues processing.
type ProfitSharingRecoveryScheduler struct {
	cron        *cron.Cron
	store       db.Store
	distributor TaskDistributor
}

// NewProfitSharingRecoveryScheduler creates a new scheduler for profit sharing recovery.
func NewProfitSharingRecoveryScheduler(store db.Store, distributor TaskDistributor) *ProfitSharingRecoveryScheduler {
	return &ProfitSharingRecoveryScheduler{
		cron:        cron.New(),
		store:       store,
		distributor: distributor,
	}
}

// Start starts the recovery scheduler.
func (s *ProfitSharingRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(profitSharingRecoveryCron, func() {
		s.runOnce()
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("profit sharing recovery scheduler started (every 10 minutes)")

	go s.runOnce()
	return nil
}

// Stop stops the scheduler.
func (s *ProfitSharingRecoveryScheduler) Stop() {
	s.cron.Stop()
	log.Info().Msg("profit sharing recovery scheduler stopped")
}

// RunOnce triggers a single scan cycle.
// Useful for integration tests and manual runs.
func (s *ProfitSharingRecoveryScheduler) RunOnce() {
	s.runOnce()
}

func (s *ProfitSharingRecoveryScheduler) runOnce() {
	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip profit sharing recovery")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cutoff := time.Now().Add(-profitSharingRecoveryMinAge)
	orders, err := s.store.ListProfitSharingOrdersForRetry(ctx, db.ListProfitSharingOrdersForRetryParams{
		CreatedAt: cutoff,
		Limit:     profitSharingRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list profit sharing orders for retry failed")
		return
	}

	for _, order := range orders {
		paymentOrder, err := s.store.GetPaymentOrder(ctx, order.PaymentOrderID)
		if err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_order_id", order.ID).
				Int64("payment_order_id", order.PaymentOrderID).
				Msg("get payment order for profit sharing retry failed")
			continue
		}

		if !paymentOrder.OrderID.Valid {
			log.Warn().
				Int64("profit_sharing_order_id", order.ID).
				Int64("payment_order_id", order.PaymentOrderID).
				Msg("payment order missing order_id, skip profit sharing retry")
			continue
		}

		err = s.distributor.DistributeTaskProcessProfitSharing(
			ctx,
			&ProfitSharingPayload{
				PaymentOrderID: order.PaymentOrderID,
				OrderID:        paymentOrder.OrderID.Int64,
			},
			asynq.MaxRetry(5),
			asynq.Queue(QueueCritical),
		)
		if err != nil {
			log.Error().Err(err).
				Int64("profit_sharing_order_id", order.ID).
				Int64("payment_order_id", order.PaymentOrderID).
				Msg("enqueue profit sharing recovery task failed")
		}
	}
}
