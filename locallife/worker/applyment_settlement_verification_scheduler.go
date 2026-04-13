package worker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	applymentSettlementVerificationCron       = "15 * * * *"
	applymentSettlementVerificationBatchLimit = int32(100)
)

type ApplymentSettlementVerificationScheduler struct {
	cron            *cron.Cron
	wg              sync.WaitGroup
	stopCtx         context.Context
	stopCancel      context.CancelFunc
	runMu           sync.Mutex
	store           db.Store
	distributor     TaskDistributor
	ecommerceClient wechat.EcommerceClientInterface
	now             func() time.Time
}

func NewApplymentSettlementVerificationScheduler(store db.Store, distributor TaskDistributor, ecommerceClient wechat.EcommerceClientInterface) *ApplymentSettlementVerificationScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &ApplymentSettlementVerificationScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:         stopCtx,
		stopCancel:      stopCancel,
		store:           store,
		distributor:     distributor,
		ecommerceClient: ecommerceClient,
		now:             time.Now,
	}
}

func (s *ApplymentSettlementVerificationScheduler) Start() error {
	_, err := s.cron.AddFunc(applymentSettlementVerificationCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("applyment settlement verification scheduler started (hourly, max once per merchant per day)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *ApplymentSettlementVerificationScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("applyment settlement verification scheduler stopped")
}

func (s *ApplymentSettlementVerificationScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *ApplymentSettlementVerificationScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("applyment settlement verification already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.ecommerceClient == nil {
		log.Warn().Msg("ecommerce client not configured, skip applyment settlement verification")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	runAt := s.now()
	items, err := s.store.ListMerchantApplymentsPendingSettlementVerification(ctx, db.ListMerchantApplymentsPendingSettlementVerificationParams{
		RunAt: runAt,
		Limit: applymentSettlementVerificationBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list merchant applyments pending settlement verification failed")
		return
	}

	for _, item := range items {
		s.verifyApplymentSettlement(ctx, runAt, item)
	}
}

func (s *ApplymentSettlementVerificationScheduler) verifyApplymentSettlement(ctx context.Context, runAt time.Time, item db.ListMerchantApplymentsPendingSettlementVerificationRow) {
	subMchID := strings.TrimSpace(textValue(item.SubMchID))
	if subMchID == "" {
		return
	}

	resp, err := s.ecommerceClient.QuerySubMerchantSettlement(ctx, subMchID, "")
	if err != nil {
		log.Error().Err(err).
			Int64("applyment_id", item.ID).
			Int64("merchant_id", item.SubjectID).
			Str("sub_mch_id", subMchID).
			Msg("query merchant settlement verification failed")
		return
	}

	firstTradeAt := item.FirstPaidAt
	if item.SettlementVerifyFirstTradeAt.Valid {
		firstTradeAt = item.SettlementVerifyFirstTradeAt.Time
	}

	status, failReason := resolveSettlementVerificationState(resp, int(item.SettlementVerifyCheckCount)+1)
	_, err = s.store.UpdateEcommerceApplymentSettlementVerification(ctx, db.UpdateEcommerceApplymentSettlementVerificationParams{
		ID:                            item.ID,
		SettlementVerifyFirstTradeAt:  pgtype.Timestamptz{Time: firstTradeAt, Valid: true},
		SettlementVerifyLastCheckedAt: pgtype.Timestamptz{Time: runAt, Valid: true},
		SettlementVerifyCheckCount:    pgtype.Int4{Int32: item.SettlementVerifyCheckCount + 1, Valid: true},
		SettlementVerifyStatus:        pgtype.Text{String: status, Valid: status != ""},
		SettlementVerifyFailReason:    pgtype.Text{String: failReason, Valid: failReason != ""},
	})
	if err != nil {
		log.Error().Err(err).
			Int64("applyment_id", item.ID).
			Str("verify_result", resp.VerifyResult).
			Msg("update settlement verification tracking failed")
		return
	}

	if status != "fail" || item.SettlementVerifyFailedNotifiedAt.Valid {
		return
	}

	merchant, err := s.store.GetMerchant(ctx, item.SubjectID)
	if err != nil {
		log.Error().Err(err).Int64("merchant_id", item.SubjectID).Msg("get merchant for settlement verification failure notification")
		return
	}

	operator, err := s.store.GetActiveOperatorByRegion(ctx, merchant.RegionID)
	if err != nil {
		log.Error().Err(err).Int64("region_id", merchant.RegionID).Int64("merchant_id", merchant.ID).Msg("get active operator for settlement verification failure notification")
		return
	}

	if s.distributor == nil || operator.UserID == 0 {
		return
	}

	expiresAt := runAt.Add(7 * 24 * time.Hour)
	err = s.distributor.DistributeTaskSendNotification(ctx, &SendNotificationPayload{
		UserID:            operator.UserID,
		Type:              "system",
		Title:             "商户结算卡校验失败",
		Content:           fmt.Sprintf("商户「%s」的微信结算银行卡校验失败，请尽快联系商户更换银行卡。失败原因：%s", merchant.Name, failReason),
		RelatedType:       "merchant",
		RelatedID:         merchant.ID,
		ExpiresAt:         &expiresAt,
		IgnorePreferences: true,
	}, asynq.Queue(QueueDefault), asynq.MaxRetry(3))
	if err != nil {
		log.Error().Err(err).Int64("operator_user_id", operator.UserID).Int64("merchant_id", merchant.ID).Msg("enqueue settlement verification failure notification failed")
		return
	}

	if _, err := s.store.MarkEcommerceApplymentSettlementVerifyFailedNotified(ctx, item.ID); err != nil {
		log.Error().Err(err).Int64("applyment_id", item.ID).Msg("mark settlement verification failure notified failed")
	}
}

func resolveSettlementVerificationState(resp *wechat.SubMerchantSettlementResponse, checkCount int) (string, string) {
	if resp == nil {
		return "", ""
	}

	switch strings.TrimSpace(resp.VerifyResult) {
	case "VERIFY_FAIL":
		return "fail", strings.TrimSpace(resp.VerifyFailReason)
	case "VERIFY_SUCCESS":
		return "success", ""
	case "VERIFYING":
		if checkCount >= 3 {
			return "success", ""
		}
		return "verifying", ""
	default:
		if checkCount >= 3 {
			return "success", ""
		}
		return "verifying", ""
	}
}
