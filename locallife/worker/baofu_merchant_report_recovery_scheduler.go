package worker

import (
	"context"
	"strings"
	"sync"
	"time"

	merchantcontracts "github.com/merrydance/locallife/baofu/merchantreport/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	baofuMerchantReportRecoveryCron       = "*/5 * * * *"
	baofuMerchantReportRecoveryBatchLimit = int32(100)
	baofuMerchantReportRecoveryMinAge     = 5 * time.Minute
)

type BaofuMerchantReportRecoveryConfig struct {
	CollectMerchantID string
	CollectTerminalID string
	MiniProgramAppID  string
}

type baofuMerchantReportRecoveryClient interface {
	SubmitWechatReport(ctx context.Context, req merchantcontracts.WechatMerchantReportRequest) (*merchantcontracts.MerchantReportResult, error)
	QueryReport(ctx context.Context, req merchantcontracts.MerchantReportQueryRequest) (*merchantcontracts.MerchantReportResult, error)
	BindSubConfig(ctx context.Context, req merchantcontracts.BindSubConfigRequest) (*merchantcontracts.BindSubConfigResult, error)
}

type BaofuMerchantReportRecoveryScheduler struct {
	cron       *cron.Cron
	wg         sync.WaitGroup
	stopCtx    context.Context
	stopCancel context.CancelFunc
	runMu      sync.Mutex
	store      db.Store
	client     baofuMerchantReportRecoveryClient
	config     BaofuMerchantReportRecoveryConfig
}

func NewBaofuMerchantReportRecoveryScheduler(store db.Store, client baofuMerchantReportRecoveryClient, config BaofuMerchantReportRecoveryConfig) *BaofuMerchantReportRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &BaofuMerchantReportRecoveryScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:    stopCtx,
		stopCancel: stopCancel,
		store:      store,
		client:     client,
		config:     config.normalized(),
	}
}

func (s *BaofuMerchantReportRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(baofuMerchantReportRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}
	s.cron.Start()
	log.Info().Msg("baofu merchant report recovery scheduler started")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *BaofuMerchantReportRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("baofu merchant report recovery scheduler stopped")
}

func (s *BaofuMerchantReportRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *BaofuMerchantReportRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("baofu merchant report recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.store == nil || s.client == nil {
		log.Warn().Msg("baofu merchant report recovery dependencies not configured")
		return
	}
	cfg := s.config.normalized()
	if cfg.CollectMerchantID == "" || cfg.CollectTerminalID == "" || cfg.MiniProgramAppID == "" {
		log.Warn().Msg("baofu merchant report recovery config not configured")
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	reports, err := s.store.ListRecoverableBaofuMerchantReports(ctx, db.ListRecoverableBaofuMerchantReportsParams{
		UpdatedBefore: time.Now().UTC().Add(-baofuMerchantReportRecoveryMinAge),
		LimitCount:    baofuMerchantReportRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list recoverable baofu merchant reports failed")
		return
	}
	service := logic.NewBaofuMerchantReportService(s.store, s.client, logic.BaofuMerchantReportConfig{
		CollectMerchantID: cfg.CollectMerchantID,
		CollectTerminalID: cfg.CollectTerminalID,
		MiniProgramAppID:  cfg.MiniProgramAppID,
	})
	for _, report := range reports {
		if _, err := service.RecoverWechatMerchantReport(ctx, report); err != nil {
			log.Error().
				Err(err).
				Int64("baofu_merchant_report_id", report.ID).
				Str("report_no", strings.TrimSpace(report.ReportNo)).
				Msg("recover baofu merchant report failed")
			continue
		}
	}
}

func (c BaofuMerchantReportRecoveryConfig) normalized() BaofuMerchantReportRecoveryConfig {
	c.CollectMerchantID = strings.TrimSpace(c.CollectMerchantID)
	c.CollectTerminalID = strings.TrimSpace(c.CollectTerminalID)
	c.MiniProgramAppID = strings.TrimSpace(c.MiniProgramAppID)
	return c
}
