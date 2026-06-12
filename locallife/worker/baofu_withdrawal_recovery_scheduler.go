package worker

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	baofucontracts "github.com/merrydance/locallife/baofu/account/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	baofuWithdrawalRecoveryCron       = "*/5 * * * *"
	baofuWithdrawalRecoveryBatchLimit = int32(100)
	baofuWithdrawalRecoveryMinAge     = 5 * time.Minute
	baofuWithdrawalRecoveryUniqueTTL  = 30 * time.Second
)

type BaofuWithdrawalRecoveryConfig struct {
	PayoutMerchantID string
	PayoutTerminalID string
}

type baofuWithdrawalRecoveryClient interface {
	QueryWithdraw(ctx context.Context, req baofucontracts.WithdrawQueryRequest) (*baofucontracts.WithdrawResult, error)
}

type BaofuWithdrawalRecoveryScheduler struct {
	cron        *cron.Cron
	wg          sync.WaitGroup
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	runMu       sync.Mutex
	store       db.Store
	distributor TaskDistributor
	client      baofuWithdrawalRecoveryClient
	config      BaofuWithdrawalRecoveryConfig
}

func NewBaofuWithdrawalRecoveryScheduler(store db.Store, distributor TaskDistributor, client baofuWithdrawalRecoveryClient, config BaofuWithdrawalRecoveryConfig) *BaofuWithdrawalRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &BaofuWithdrawalRecoveryScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:     stopCtx,
		stopCancel:  stopCancel,
		store:       store,
		distributor: distributor,
		client:      client,
		config:      config.normalized(),
	}
}

func (s *BaofuWithdrawalRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(baofuWithdrawalRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}
	s.cron.Start()
	log.Info().Msg("baofu withdrawal recovery scheduler started")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *BaofuWithdrawalRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("baofu withdrawal recovery scheduler stopped")
}

func (s *BaofuWithdrawalRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *BaofuWithdrawalRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("baofu withdrawal recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.store == nil || s.distributor == nil || s.client == nil {
		log.Warn().Msg("baofu withdrawal recovery dependencies not configured")
		return
	}
	cfg := s.config.normalized()
	if cfg.PayoutMerchantID == "" || cfg.PayoutTerminalID == "" {
		log.Warn().Msg("baofu withdrawal recovery payout merchant config not configured")
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	rows, err := s.store.ListProcessingBaofuWithdrawalOrdersForRecovery(ctx, db.ListProcessingBaofuWithdrawalOrdersForRecoveryParams{
		CreatedBefore: time.Now().UTC().Add(-baofuWithdrawalRecoveryMinAge),
		LimitCount:    baofuWithdrawalRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list processing baofu withdrawal orders for recovery failed")
		return
	}
	s.enqueueSubmittedCommands(ctx)
	for _, order := range rows {
		s.queryAndEnqueue(ctx, cfg, order)
	}
}

func (s *BaofuWithdrawalRecoveryScheduler) enqueueSubmittedCommands(ctx context.Context) {
	rows, err := s.store.ListSubmittedBaofuWithdrawalCommandsForDispatch(ctx, db.ListSubmittedBaofuWithdrawalCommandsForDispatchParams{
		SubmittedBefore: time.Now().UTC(),
		LimitCount:      baofuWithdrawalRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list submitted baofu withdrawal commands for dispatch failed")
		return
	}
	for _, command := range rows {
		if err := s.distributor.DistributeTaskProcessBaofuWithdrawalCommandDispatch(ctx, &BaofuWithdrawalCommandDispatchPayload{
			CommandID: command.ID,
		}, asynq.MaxRetry(5), asynq.Queue(QueueCritical), asynq.Unique(baofuWithdrawalCommandDispatchUniqueTTL)); err != nil {
			log.Error().
				Err(err).
				Int64("external_payment_command_id", command.ID).
				Str("out_request_no", strings.TrimSpace(command.ExternalObjectKey)).
				Msg("enqueue baofu withdrawal command dispatch failed")
		}
	}
}

func (s *BaofuWithdrawalRecoveryScheduler) queryAndEnqueue(ctx context.Context, cfg BaofuWithdrawalRecoveryConfig, order db.BaofuWithdrawalOrder) {
	outRequestNo := strings.TrimSpace(order.OutRequestNo)
	if outRequestNo == "" {
		log.Warn().Int64("baofu_withdrawal_order_id", order.ID).Msg("skip baofu withdrawal recovery because out request no is missing")
		return
	}
	result, err := s.client.QueryWithdraw(ctx, baofucontracts.WithdrawQueryRequest{
		MerchantID:    cfg.PayoutMerchantID,
		TerminalID:    cfg.PayoutTerminalID,
		TransSerialNo: outRequestNo,
		TradeTime:     order.CreatedAt.Format("2006-01-02"),
	})
	if err != nil {
		log.Error().Err(err).Int64("baofu_withdrawal_order_id", order.ID).Str("out_request_no", outRequestNo).Msg("query baofu withdrawal status failed")
		return
	}
	if result == nil {
		log.Error().Int64("baofu_withdrawal_order_id", order.ID).Str("out_request_no", outRequestNo).Msg("query baofu withdrawal returned empty result")
		return
	}
	status := baofucontracts.WithdrawStatusFromUpstream(result.UpstreamState)
	if status == db.BaofuWithdrawalStatusProcessing {
		log.Info().Int64("baofu_withdrawal_order_id", order.ID).Str("out_request_no", outRequestNo).Msg("baofu withdrawal still processing")
		return
	}
	raw := result.Raw
	if len(raw) == 0 || !json.Valid(raw) {
		raw = []byte(`{}`)
	}
	if err := s.distributor.DistributeTaskProcessBaofuWithdrawalFactApplication(ctx, &BaofuWithdrawalFactApplicationPayload{
		WithdrawalOrderID: order.ID,
		UpstreamState:     strings.TrimSpace(result.UpstreamState),
		BaofuWithdrawNo:   strings.TrimSpace(result.BaofuWithdrawNo),
		RawSnapshot:       raw,
	}, asynq.MaxRetry(5), asynq.Queue(QueueCritical), asynq.Unique(baofuWithdrawalRecoveryUniqueTTL)); err != nil {
		log.Error().Err(err).Int64("baofu_withdrawal_order_id", order.ID).Str("out_request_no", outRequestNo).Msg("enqueue baofu withdrawal fact application failed")
		return
	}
}

func (c BaofuWithdrawalRecoveryConfig) normalized() BaofuWithdrawalRecoveryConfig {
	c.PayoutMerchantID = strings.TrimSpace(c.PayoutMerchantID)
	c.PayoutTerminalID = strings.TrimSpace(c.PayoutTerminalID)
	return c
}
