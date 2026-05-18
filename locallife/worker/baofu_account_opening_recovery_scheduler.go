package worker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/merrydance/locallife/baofu"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/util"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	baofuAccountOpeningRecoveryCron       = "*/5 * * * *"
	baofuAccountOpeningRecoveryBatchLimit = int32(100)
	baofuAccountOpeningRecoveryMinAge     = 2 * time.Minute
)

type BaofuAccountOpeningRecoveryConfig struct {
	VerifyFeeFen                    int64
	IndustryID                      string
	CollectMerchantID               string
	MerchantReportCollectMerchantID string
	MerchantReportCollectTerminalID string
	MerchantReportChannelID         string
	MerchantReportChannelName       string
	MerchantReportBusiness          string
	MiniProgramAppID                string
}

type BaofuAccountOpeningMerchantReportRecoveryConfig struct {
	CollectMerchantID string
	CollectTerminalID string
	ChannelID         string
	ChannelName       string
	Business          string
	MiniProgramAppID  string
}

type BaofuAccountOpeningRecoveryScheduler struct {
	cron                 *cron.Cron
	wg                   sync.WaitGroup
	stopCtx              context.Context
	stopCancel           context.CancelFunc
	runMu                sync.Mutex
	store                db.Store
	client               logic.BaofuAccountClient
	merchantReportClient baofuMerchantReportRecoveryClient
	dataEncryptor        util.DataEncryptor
	config               BaofuAccountOpeningRecoveryConfig
}

func NewBaofuAccountOpeningRecoveryScheduler(store db.Store, client logic.BaofuAccountClient, dataEncryptor util.DataEncryptor, config BaofuAccountOpeningRecoveryConfig) *BaofuAccountOpeningRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &BaofuAccountOpeningRecoveryScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:       stopCtx,
		stopCancel:    stopCancel,
		store:         store,
		client:        client,
		dataEncryptor: dataEncryptor,
		config:        config,
	}
}

func (s *BaofuAccountOpeningRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(baofuAccountOpeningRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}
	s.cron.Start()
	log.Info().Msg("baofu account opening recovery scheduler started")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *BaofuAccountOpeningRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("baofu account opening recovery scheduler stopped")
}

func (s *BaofuAccountOpeningRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *BaofuAccountOpeningRecoveryScheduler) SetMerchantReportClient(client baofuMerchantReportRecoveryClient, config BaofuAccountOpeningMerchantReportRecoveryConfig) {
	s.merchantReportClient = client
	s.config.MerchantReportCollectMerchantID = strings.TrimSpace(config.CollectMerchantID)
	s.config.MerchantReportCollectTerminalID = strings.TrimSpace(config.CollectTerminalID)
	s.config.MerchantReportChannelID = strings.TrimSpace(config.ChannelID)
	s.config.MerchantReportChannelName = strings.TrimSpace(config.ChannelName)
	s.config.MerchantReportBusiness = strings.TrimSpace(config.Business)
	s.config.MiniProgramAppID = strings.TrimSpace(config.MiniProgramAppID)
}

func (s *BaofuAccountOpeningRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("baofu account opening recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.store == nil || s.client == nil {
		log.Warn().
			Bool("missing_store", s.store == nil).
			Bool("missing_account_client", s.client == nil).
			Str("provider_operation", "baofu_account_opening_recovery").
			Msg("baofu account opening recovery dependencies not configured")
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	flows, err := s.store.ListRecoverableBaofuAccountOpeningFlows(ctx, db.ListRecoverableBaofuAccountOpeningFlowsParams{
		BeforeAt:   time.Now().UTC().Add(-baofuAccountOpeningRecoveryMinAge),
		LimitCount: baofuAccountOpeningRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).
			Str("provider_operation", "list_recoverable_baofu_account_opening_flows").
			Msg("list recoverable baofu account opening flows failed")
		return
	}
	service := logic.NewBaofuAccountOnboardingService(s.store, s.client, nil, s.dataEncryptor, logic.BaofuAccountOnboardingConfig{
		VerifyFeeFen:      s.config.VerifyFeeFen,
		IndustryID:        s.config.IndustryID,
		CollectMerchantID: s.config.CollectMerchantID,
	})
	for _, flow := range flows {
		switch strings.TrimSpace(flow.State) {
		case db.BaofuAccountOpeningStateOpeningProcessing, db.BaofuAccountOpeningStateFailed:
			result, err := service.RecoverOpeningFlow(ctx, flow)
			if err != nil {
				logFlow := baofuAccountOpeningRecoveredFlowForLog(flow, result.Flow)
				event := log.Error().Err(err)
				if baofuAccountOpeningRecoveryErrorIsUserActionFailure(logFlow, err) {
					event = log.Warn().Err(err)
				}
				logBaofuAccountOpeningRecoveryFlow(event, logFlow, "baofu_account_query", err).
					Msg("recover baofu account opening flow failed")
			}
		case db.BaofuAccountOpeningStateMerchantReportProcessing, db.BaofuAccountOpeningStateAppletAuthPending:
			if s.merchantReportClient == nil {
				logBaofuAccountOpeningRecoveryFlow(log.Warn().Bool("missing_merchant_report_client", true), flow, "baofu_merchant_report_recover", nil).
					Msg("baofu account opening recovery skipped merchant report flow because client is not configured")
				continue
			}
			if missing := s.missingMerchantReportRecoveryConfigFields(); len(missing) > 0 {
				logBaofuAccountOpeningRecoveryFlow(log.Warn().Strs("missing_config_fields", missing), flow, "baofu_merchant_report_recover", nil).
					Msg("baofu account opening recovery skipped merchant report flow because config is incomplete")
				continue
			}
			reportService := logic.NewBaofuAccountMerchantReportService(s.store, s.merchantReportClient, s.dataEncryptor, logic.BaofuAccountMerchantReportConfig{
				CollectMerchantID: s.config.MerchantReportCollectMerchantID,
				CollectTerminalID: s.config.MerchantReportCollectTerminalID,
				MiniProgramAppID:  s.config.MiniProgramAppID,
				ChannelID:         s.config.MerchantReportChannelID,
				ChannelName:       s.config.MerchantReportChannelName,
				Business:          s.config.MerchantReportBusiness,
			})
			if _, err := reportService.RecoverMerchantReportFlow(ctx, flow); err != nil {
				logBaofuAccountOpeningRecoveryFlow(log.Error().Err(err), flow, "baofu_merchant_report_recover", err).
					Msg("recover baofu merchant report flow failed")
			}
		}
	}
}

func baofuAccountOpeningRecoveredFlowForLog(original db.BaofuAccountOpeningFlow, recovered db.BaofuAccountOpeningFlow) db.BaofuAccountOpeningFlow {
	if recovered.ID == 0 {
		return original
	}
	return recovered
}

func baofuAccountOpeningRecoveryErrorIsUserActionFailure(flow db.BaofuAccountOpeningFlow, err error) bool {
	if strings.TrimSpace(flow.State) != db.BaofuAccountOpeningStateFailed {
		return false
	}
	var providerErr *baofu.ProviderError
	if !errors.As(logic.LoggableError(err), &providerErr) || providerErr == nil {
		return false
	}
	classified := baofu.ClassifyBaofuError(providerErr.UpstreamCode, providerErr.UpstreamMessage)
	return classified.Category == baofu.BaofuErrorCategoryUserActionRequired
}

func logBaofuAccountOpeningRecoveryFlow(event *zerolog.Event, flow db.BaofuAccountOpeningFlow, providerOperation string, err error) *zerolog.Event {
	event = event.
		Int64("flow_id", flow.ID).
		Str("owner_type", strings.TrimSpace(flow.OwnerType)).
		Int64("owner_id", flow.OwnerID).
		Str("open_trans_serial_no", strings.TrimSpace(flow.OpenTransSerialNo.String)).
		Str("current_state", strings.TrimSpace(flow.State)).
		Str("provider_operation", providerOperation)
	var providerErr *baofu.ProviderError
	if errors.As(logic.LoggableError(err), &providerErr) && providerErr != nil {
		event = event.
			Str("provider_method", strings.TrimSpace(providerErr.Operation)).
			Str("provider_capability", strings.TrimSpace(providerErr.Capability)).
			Str("upstream_code", strings.TrimSpace(providerErr.UpstreamCode)).
			Str("frontend_code", strings.TrimSpace(providerErr.Frontend.Code)).
			Bool("retryable", providerErr.Frontend.Retryable)
		if providerErr.StatusCode != 0 {
			event = event.Int("http_status", providerErr.StatusCode)
		}
		if cause := errors.Unwrap(providerErr); cause != nil {
			event = event.Str("provider_error_cause", strings.TrimSpace(cause.Error()))
		}
	}
	return event
}

func (s *BaofuAccountOpeningRecoveryScheduler) missingMerchantReportRecoveryConfigFields() []string {
	var missing []string
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "collect_merchant_id", value: s.config.MerchantReportCollectMerchantID},
		{name: "collect_terminal_id", value: s.config.MerchantReportCollectTerminalID},
		{name: "mini_program_appid", value: s.config.MiniProgramAppID},
		{name: "channel_id", value: s.config.MerchantReportChannelID},
		{name: "channel_name", value: s.config.MerchantReportChannelName},
	} {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}
	return missing
}
