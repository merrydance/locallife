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
	profitSharingReceiverLifecycleCron       = "*/1 * * * *"
	profitSharingReceiverLifecycleBatchLimit = int32(100)
	profitSharingReceiverLifecycleTaskUnique = 30 * time.Second
)

type profitSharingReceiverLifecycleTaskDistributor interface {
	DistributeTaskProcessProfitSharingReceiverTarget(
		ctx context.Context,
		payload *ProfitSharingReceiverTargetPayload,
		opts ...asynq.Option,
	) error
}

type ProfitSharingReceiverLifecycleScheduler struct {
	cron        *cron.Cron
	wg          sync.WaitGroup
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	runMu       sync.Mutex
	store       db.Store
	distributor profitSharingReceiverLifecycleTaskDistributor
}

func NewProfitSharingReceiverLifecycleScheduler(store db.Store, distributor profitSharingReceiverLifecycleTaskDistributor) *ProfitSharingReceiverLifecycleScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &ProfitSharingReceiverLifecycleScheduler{
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

func (s *ProfitSharingReceiverLifecycleScheduler) Start() error {
	_, err := s.cron.AddFunc(profitSharingReceiverLifecycleCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("profit sharing receiver lifecycle scheduler started (every 1 minute)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *ProfitSharingReceiverLifecycleScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("profit sharing receiver lifecycle scheduler stopped")
}

func (s *ProfitSharingReceiverLifecycleScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *ProfitSharingReceiverLifecycleScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("profit sharing receiver lifecycle already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip profit sharing receiver lifecycle")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	for _, ownerType := range []string{db.ProfitSharingReceiverOwnerTypeOperator, db.ProfitSharingReceiverOwnerTypeRider} {
		targets, err := s.store.ListRetryableProfitSharingReceiverTargetsByOwnerType(ctx, db.ListRetryableProfitSharingReceiverTargetsByOwnerTypeParams{
			OwnerType:  ownerType,
			NowAt:      now,
			LimitCount: profitSharingReceiverLifecycleBatchLimit,
		})
		if err != nil {
			log.Error().Err(err).Str("owner_type", ownerType).Msg("list retryable profit sharing receiver targets failed")
			continue
		}

		for _, target := range targets {
			if target.SyncStatus == db.ProfitSharingReceiverSyncStatusFailed && target.AttemptCount >= 3 {
				log.Error().
					Int64("target_id", target.ID).
					Str("owner_type", target.OwnerType).
					Int64("owner_id", target.OwnerID).
					Int32("attempt_count", target.AttemptCount).
					Str("last_error_code", target.LastErrorCode.String).
					Msg("profit sharing receiver target remains failed after repeated attempts")
			}
			if err := s.distributor.DistributeTaskProcessProfitSharingReceiverTarget(
				ctx,
				&ProfitSharingReceiverTargetPayload{TargetID: target.ID},
				asynq.MaxRetry(3),
				asynq.Queue(QueueCritical),
				asynq.Unique(profitSharingReceiverLifecycleTaskUnique),
			); err != nil {
				log.Error().Err(err).
					Int64("target_id", target.ID).
					Str("owner_type", target.OwnerType).
					Int64("owner_id", target.OwnerID).
					Msg("enqueue profit sharing receiver target task failed")
			}
		}
	}
}
