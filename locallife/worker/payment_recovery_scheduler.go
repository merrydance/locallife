package worker

import (
	"context"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	paymentRecoveryCron       = "*/5 * * * *"
	paymentRecoveryBatchLimit = int32(200)
	paymentRecoveryMinAge     = 2 * time.Minute
)

// PaymentRecoveryScheduler scans paid but unprocessed payment orders and re-enqueues processing.
type PaymentRecoveryScheduler struct {
	cron        *cron.Cron
	wg          sync.WaitGroup
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	runMu       sync.Mutex
	store       db.Store
	distributor TaskDistributor
}

// NewPaymentRecoveryScheduler creates a new scheduler for payment recovery.
func NewPaymentRecoveryScheduler(store db.Store, distributor TaskDistributor) *PaymentRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &PaymentRecoveryScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:     stopCtx,
		stopCancel:  stopCancel,
		store:       store,
		distributor: distributor,
	}
}

// Start starts the recovery scheduler.
func (s *PaymentRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(paymentRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("payment recovery scheduler started (every 5 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

// RunOnce triggers a single recovery scan.
func (s *PaymentRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
}

// Stop stops the scheduler.
func (s *PaymentRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("payment recovery scheduler stopped")
}

func (s *PaymentRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("payment recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip payment recovery")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cutoff := time.Now().Add(-paymentRecoveryMinAge)
	orders, err := s.store.ListPaidUnprocessedPaymentOrders(ctx, db.ListPaidUnprocessedPaymentOrdersParams{
		PaidAt: pgtype.Timestamptz{
			Time:  cutoff,
			Valid: true,
		},
		Limit: paymentRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list paid unprocessed payment orders failed")
		return
	}

	for _, order := range orders {
		transactionID := ""
		if order.TransactionID.Valid {
			transactionID = order.TransactionID.String
		}

		err := s.distributor.DistributeTaskProcessPaymentSuccess(
			ctx,
			&PaymentSuccessPayload{
				PaymentOrderID: order.ID,
				TransactionID:  transactionID,
				BusinessType:   order.BusinessType,
			},
			asynq.MaxRetry(5),
			asynq.Queue(QueueCritical),
		)
		if err != nil {
			log.Error().Err(err).
				Int64("payment_order_id", order.ID).
				Str("out_trade_no", order.OutTradeNo).
				Msg("enqueue payment recovery task failed")
		}
	}
}
