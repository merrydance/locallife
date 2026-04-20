package worker

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	claimRefundRecoveryCron       = "*/5 * * * *"
	claimRefundRecoveryBatchLimit = int32(200)
)

// ClaimPayoutRecoveryScheduler 扫描未完成的赔付动作并重试执行。
type ClaimPayoutRecoveryScheduler struct {
	cron           *cron.Cron
	wg             sync.WaitGroup
	stopCtx        context.Context
	stopCancel     context.CancelFunc
	runMu          sync.Mutex
	store          db.Store
	transferClient wechat.TransferClientInterface
}

func NewClaimPayoutRecoveryScheduler(store db.Store, transferClient wechat.TransferClientInterface) *ClaimPayoutRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &ClaimPayoutRecoveryScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:        stopCtx,
		stopCancel:     stopCancel,
		store:          store,
		transferClient: transferClient,
	}
}

func (s *ClaimPayoutRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(claimRefundRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("claim payout recovery scheduler started (every 5 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *ClaimPayoutRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("claim payout recovery scheduler stopped")
}

func (s *ClaimPayoutRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("claim payout recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	statuses := []string{"created", "failed", "running"}
	for _, status := range statuses {
		actions, err := s.store.ListBehaviorActionsByStatusAndType(ctx, db.ListBehaviorActionsByStatusAndTypeParams{
			Status:       status,
			ActionType:   "payout",
			TargetEntity: "user",
			Limit:        claimRefundRecoveryBatchLimit,
		})
		if err != nil {
			log.Error().Err(err).Str("status", status).Msg("list claim payout actions failed")
			continue
		}

		for _, action := range actions {
			if action.Status == "failed" && isClaimPayoutTerminalFailure(action.Detail) {
				continue
			}
			if err := ExecuteClaimPayoutAction(ctx, s.store, nil, s.transferClient, action.ID); err != nil {
				log.Error().Err(err).Int64("behavior_action_id", action.ID).Msg("recover claim payout action failed")
			}
		}
	}
}

func isClaimPayoutTerminalFailure(detailBytes []byte) bool {
	if len(detailBytes) == 0 {
		return false
	}
	var detail claimPayoutActionDetail
	if err := json.Unmarshal(detailBytes, &detail); err != nil {
		return false
	}
	return detail.TerminalFailure
}
