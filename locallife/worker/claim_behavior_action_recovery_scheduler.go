package worker

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	claimBehaviorActionRecoveryCron       = "*/5 * * * *"
	claimBehaviorActionRecoveryBatchLimit = int32(200)
)

type ClaimBehaviorActionRecoveryScheduler struct {
	cron        *cron.Cron
	wg          sync.WaitGroup
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	runMu       sync.Mutex
	store       db.Store
	distributor TaskDistributor
}

func NewClaimBehaviorActionRecoveryScheduler(store db.Store, distributor TaskDistributor) *ClaimBehaviorActionRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &ClaimBehaviorActionRecoveryScheduler{
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

func (s *ClaimBehaviorActionRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(claimBehaviorActionRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("claim behavior action recovery scheduler started (every 5 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *ClaimBehaviorActionRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("claim behavior action recovery scheduler stopped")
}

func (s *ClaimBehaviorActionRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *ClaimBehaviorActionRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("claim behavior action recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.distributor == nil {
		log.Warn().Msg("claim behavior action recovery skipped: task distributor not configured")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	recoverActionType := func(actionType string, targetEntity string, queue string, retries int) {
		statuses := []string{"created", "failed"}
		for _, status := range statuses {
			actions, err := s.store.ListBehaviorActionsByStatusAndType(ctx, db.ListBehaviorActionsByStatusAndTypeParams{
				Status:       status,
				ActionType:   actionType,
				TargetEntity: targetEntity,
				Limit:        claimBehaviorActionRecoveryBatchLimit,
			})
			if err != nil {
				log.Error().Err(err).Str("status", status).Str("action_type", actionType).Str("target_entity", targetEntity).Msg("list claim behavior actions failed")
				continue
			}
			for _, action := range actions {
				if isClaimBehaviorActionTerminalFailure(action.Detail, action.ActionType) {
					continue
				}
				if err := s.distributor.DistributeTaskClaimBehaviorAction(
					ctx,
					&ClaimBehaviorActionPayload{ActionID: action.ID},
					asynq.Queue(queue),
					asynq.MaxRetry(retries),
				); err != nil {
					log.Error().Err(err).Int64("behavior_action_id", action.ID).Msg("re-enqueue claim behavior action failed")
				}
			}
		}
	}

	recoverActionType("block", "user", QueueCritical, 10)
	recoverActionType("notify", "user", QueueDefault, 5)
	recoverActionType("notify", "merchant", QueueDefault, 5)
	recoverActionType("notify", "rider", QueueDefault, 5)
}

func isClaimBehaviorActionTerminalFailure(detailBytes []byte, actionType string) bool {
	if len(detailBytes) == 0 {
		return false
	}
	switch actionType {
	case "block":
		var detail claimRestrictionActionDetail
		if err := json.Unmarshal(detailBytes, &detail); err != nil {
			return false
		}
		return detail.TerminalFailure
	case "notify":
		var detail claimNotifyActionDetail
		if err := json.Unmarshal(detailBytes, &detail); err != nil {
			return false
		}
		return detail.TerminalFailure
	default:
		return false
	}
}
