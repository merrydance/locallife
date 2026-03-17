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
	merchantWithdrawRecoveryCron       = "*/3 * * * *"
	merchantWithdrawRecoveryBatchLimit = int32(200)
)

// MerchantWithdrawRecoveryScheduler 扫描pending商户提现并触发结果轮询
type MerchantWithdrawRecoveryScheduler struct {
	cron        *cron.Cron
	wg          sync.WaitGroup
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	runMu       sync.Mutex
	store       db.Store
	distributor TaskDistributor
}

// NewMerchantWithdrawRecoveryScheduler 创建商户提现轮询调度器
func NewMerchantWithdrawRecoveryScheduler(store db.Store, distributor TaskDistributor) *MerchantWithdrawRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &MerchantWithdrawRecoveryScheduler{
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

// Start 启动调度器
func (s *MerchantWithdrawRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(merchantWithdrawRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("merchant withdraw recovery scheduler started (every 3 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

// Stop 停止调度器
func (s *MerchantWithdrawRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("merchant withdraw recovery scheduler stopped")
}

// RunOnce 用于测试或手动触发
func (s *MerchantWithdrawRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *MerchantWithdrawRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("merchant withdraw recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip merchant withdraw recovery")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	channels := []string{merchantWithdrawChannel, operatorWithdrawChannel}
	for _, channel := range channels {
		records, err := s.store.ListPendingWithdrawalRecordsByChannel(ctx, db.ListPendingWithdrawalRecordsByChannelParams{
			Channel: channel,
			Limit:   merchantWithdrawRecoveryBatchLimit,
		})
		if err != nil {
			log.Error().Err(err).Str("channel", channel).Msg("list pending withdrawal records failed")
			continue
		}

		for _, record := range records {
			err := s.distributor.DistributeTaskProcessMerchantWithdrawResult(
				ctx,
				&MerchantWithdrawResultPayload{WithdrawalRecordID: record.ID, RetryCount: 0},
				asynq.Queue(QueueDefault),
			)
			if err != nil {
				log.Error().Err(err).Int64("withdrawal_record_id", record.ID).Str("channel", channel).Msg("enqueue withdraw recovery task failed")
			}
		}
	}
}
