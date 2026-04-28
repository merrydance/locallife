package worker

import (
	"context"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	claimRecoveryCron       = "*/5 * * * *"
	claimRecoveryBatchLimit = int32(200)
)

// ClaimRecoveryScheduler scans due claim recoveries and applies overdue actions.
type ClaimRecoveryScheduler struct {
	cron        *cron.Cron
	wg          sync.WaitGroup
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	runMu       sync.Mutex
	store       db.Store
	distributor TaskDistributor
}

// NewClaimRecoveryScheduler creates a new scheduler for claim recoveries.
func NewClaimRecoveryScheduler(store db.Store, distributor TaskDistributor) *ClaimRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &ClaimRecoveryScheduler{
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
func (s *ClaimRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(claimRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("claim recovery scheduler started (every 5 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

// Stop stops the scheduler.
func (s *ClaimRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("claim recovery scheduler stopped")
}

// RunOnce executes a single synchronous recovery scan without starting cron.
func (s *ClaimRecoveryScheduler) RunOnce() {
	s.runOnce(s.stopCtx)
}

func (s *ClaimRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("claim recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	recoveries, err := s.store.ListDueClaimRecoveries(ctx, db.ListDueClaimRecoveriesParams{
		DueAt: time.Now(),
		Limit: claimRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list due claim recoveries failed")
		return
	}

	for _, recovery := range recoveries {
		result, err := s.store.MarkClaimRecoveryOverdueWithActionTx(ctx, db.MarkClaimRecoveryOverdueWithActionTxParams{
			RecoveryID:    recovery.ID,
			SuspendUntil:  time.Now().Add(24 * time.Hour),
			OverdueRemark: overdueClaimRecoveryRemark(recovery),
		})
		if err != nil {
			log.Error().Err(err).Int64("recovery_id", recovery.ID).Msg("mark claim recovery overdue with block action failed")
			continue
		}
		s.enqueueClaimBehaviorAction(ctx, result.Action.ID)
	}
}

func overdueClaimRecoveryRemark(recovery db.ClaimRecovery) string {
	if recovery.RecoveryTarget.Valid {
		switch recovery.RecoveryTarget.String {
		case "merchant":
			return "merchant recovery overdue block action created"
		case "rider":
			return "rider recovery overdue block action created"
		}
	}
	return "claim recovery overdue block action created"
}

func (s *ClaimRecoveryScheduler) enqueueClaimBehaviorAction(ctx context.Context, actionID int64) {
	if s.distributor == nil {
		return
	}
	if err := s.distributor.DistributeTaskClaimBehaviorAction(
		ctx,
		&ClaimBehaviorActionPayload{ActionID: actionID},
		asynq.Queue(QueueCritical),
		asynq.MaxRetry(10),
	); err != nil {
		log.Error().Err(err).Int64("behavior_action_id", actionID).Msg("enqueue claim recovery block action failed")
	}
}
