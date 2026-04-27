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
	merchantCancelWithdrawRecoveryCron       = "*/3 * * * *"
	merchantCancelWithdrawRecoveryBatchLimit = int32(200)
	merchantCancelWithdrawRecoveryMinAge     = 5 * time.Minute
)

type MerchantCancelWithdrawRecoveryScheduler struct {
	cron        *cron.Cron
	wg          sync.WaitGroup
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	runMu       sync.Mutex
	store       db.Store
	distributor TaskDistributor
}

func NewMerchantCancelWithdrawRecoveryScheduler(store db.Store, distributor TaskDistributor) *MerchantCancelWithdrawRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &MerchantCancelWithdrawRecoveryScheduler{
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

func (s *MerchantCancelWithdrawRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(merchantCancelWithdrawRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("merchant cancel withdraw recovery scheduler started (every 3 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *MerchantCancelWithdrawRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("merchant cancel withdraw recovery scheduler stopped")
}

func (s *MerchantCancelWithdrawRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *MerchantCancelWithdrawRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("merchant cancel withdraw recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip merchant cancel withdraw recovery")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	records, err := s.store.ListPendingMerchantCancelWithdrawApplications(ctx, db.ListPendingMerchantCancelWithdrawApplicationsParams{
		QueryBefore: pgtype.Timestamptz{Time: time.Now().Add(-merchantCancelWithdrawRecoveryMinAge), Valid: true},
		LimitCount:  merchantCancelWithdrawRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list pending merchant cancel withdraw applications failed")
		return
	}

	for _, record := range records {
		err := s.distributor.DistributeTaskProcessMerchantCancelWithdrawResult(
			ctx,
			&MerchantCancelWithdrawResultPayload{ApplicationID: record.ID, RetryCount: 0},
			asynq.Queue(QueueDefault),
		)
		if err != nil {
			log.Error().Err(err).Int64("application_id", record.ID).Msg("enqueue merchant cancel withdraw recovery task failed")
		}
	}
}
