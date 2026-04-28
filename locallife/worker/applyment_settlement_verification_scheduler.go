package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	applymentSettlementVerificationCron       = "15 * * * *"
	applymentSettlementVerificationBatchLimit = int32(100)
	settlementVerificationFactConsumerDomain  = "settlement_domain"
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

func (s *ApplymentSettlementVerificationScheduler) SetNowFuncForTest(now func() time.Time) {
	if now == nil {
		s.now = time.Now
		return
	}
	s.now = now
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
		if reason, ok := settlementVerificationTerminalFailureReason(err); ok {
			s.markSettlementVerificationInternalFailure(ctx, runAt, item, subMchID, reason, err)
			return
		}

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
	application, err := s.recordSettlementVerificationQueryFact(ctx, runAt, item, resp)
	if err != nil {
		log.Error().Err(err).
			Int64("applyment_id", item.ID).
			Str("verify_result", resp.VerifyResult).
			Msg("record settlement verification fact failed")
		return
	}
	if application != nil {
		if _, err := logic.NewPaymentFactService(s.store).ApplyExternalPaymentFactApplication(ctx, application.ID); err != nil {
			log.Error().Err(err).
				Int64("applyment_id", item.ID).
				Int64("payment_fact_application_id", application.ID).
				Str("verify_result", resp.VerifyResult).
				Msg("apply settlement verification fact failed")
			return
		}
	}

	logger := log.Info()
	if status == "fail" {
		logger = log.Warn()
	}
	logger.
		Int64("applyment_id", item.ID).
		Int64("merchant_id", item.SubjectID).
		Str("sub_mch_id", subMchID).
		Time("first_trade_at", firstTradeAt).
		Time("checked_at", runAt).
		Int32("check_count", item.SettlementVerifyCheckCount+1).
		Str("verify_result", strings.TrimSpace(resp.VerifyResult)).
		Str("settlement_verify_status", status).
		Bool("has_verify_fail_reason", failReason != "").
		Msg("settlement verification fact applied")

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

func (s *ApplymentSettlementVerificationScheduler) recordSettlementVerificationQueryFact(ctx context.Context, runAt time.Time, item db.ListMerchantApplymentsPendingSettlementVerificationRow, resp *wechatcontracts.SubMerchantSettlementResponse) (*db.ExternalPaymentFactApplication, error) {
	service := logic.NewPaymentFactService(s.store)
	subMchID := strings.TrimSpace(textValue(item.SubMchID))
	checkCount := item.SettlementVerifyCheckCount + 1
	firstTradeAt := item.FirstPaidAt
	if item.SettlementVerifyFirstTradeAt.Valid {
		firstTradeAt = item.SettlementVerifyFirstTradeAt.Time
	}
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:                    db.ExternalPaymentProviderWechat,
		Channel:                     db.PaymentChannelEcommerce,
		Capability:                  db.ExternalPaymentCapabilitySettlement,
		FactSource:                  db.ExternalPaymentFactSourceQuery,
		ExternalObjectType:          db.ExternalPaymentObjectSettlement,
		ExternalObjectKey:           subMchID,
		BusinessOwner:               settlementVerificationStringPtr(db.ExternalPaymentBusinessOwnerMerchantFunds),
		BusinessObjectType:          settlementVerificationStringPtr("ecommerce_applyment"),
		BusinessObjectID:            settlementVerificationInt64Ptr(item.ID),
		UpstreamState:               strings.TrimSpace(resp.VerifyResult),
		TerminalStatus:              settlementVerificationTerminalStatus(resp.VerifyResult),
		Currency:                    "CNY",
		OccurredAt:                  settlementVerificationTimePtr(runAt.UTC()),
		UpstreamUpdatedAt:           settlementVerificationTimePtr(runAt.UTC()),
		RawResource:                 settlementVerificationQueryFactResource(runAt, item, firstTradeAt, checkCount, resp),
		DedupeKey:                   fmt.Sprintf("wechat:query:ecommerce:settlement_verification:%d:%d:%s", item.ID, checkCount, strings.TrimSpace(resp.VerifyResult)),
		AllowNonTerminalApplication: true,
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           settlementVerificationFactConsumerDomain,
			BusinessObjectType: "ecommerce_applyment",
			BusinessObjectID:   item.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}

func (s *ApplymentSettlementVerificationScheduler) markSettlementVerificationInternalFailure(ctx context.Context, runAt time.Time, item db.ListMerchantApplymentsPendingSettlementVerificationRow, subMchID, failReason string, cause error) {
	firstTradeAt := item.FirstPaidAt
	if item.SettlementVerifyFirstTradeAt.Valid {
		firstTradeAt = item.SettlementVerifyFirstTradeAt.Time
	}

	_, err := s.store.UpdateEcommerceApplymentSettlementVerification(ctx, db.UpdateEcommerceApplymentSettlementVerificationParams{
		ID:                            item.ID,
		SettlementVerifyFirstTradeAt:  pgtype.Timestamptz{Time: firstTradeAt, Valid: true},
		SettlementVerifyLastCheckedAt: pgtype.Timestamptz{Time: runAt, Valid: true},
		SettlementVerifyCheckCount:    pgtype.Int4{Int32: item.SettlementVerifyCheckCount + 1, Valid: true},
		SettlementVerifyStatus:        pgtype.Text{String: "fail", Valid: true},
		SettlementVerifyFailReason:    pgtype.Text{String: failReason, Valid: failReason != ""},
	})
	if err != nil {
		log.Error().Err(err).
			Int64("applyment_id", item.ID).
			Int64("merchant_id", item.SubjectID).
			Str("sub_mch_id", subMchID).
			Msg("mark settlement verification internal failure failed")
		return
	}

	log.Error().Err(cause).
		Int64("applyment_id", item.ID).
		Int64("merchant_id", item.SubjectID).
		Str("sub_mch_id", subMchID).
		Time("first_trade_at", firstTradeAt).
		Time("checked_at", runAt).
		Int32("check_count", item.SettlementVerifyCheckCount+1).
		Str("settlement_verify_status", "fail").
		Str("settlement_verify_fail_reason", failReason).
		Msg("settlement verification stopped on non-retryable query failure")
}

func settlementVerificationTerminalFailureReason(err error) (string, bool) {
	var validationErr *wechatcontracts.SubMerchantSettlementQueryValidationError
	if errors.As(err, &validationErr) {
		return "结算卡验卡巡检请求无效，请联系平台处理微信二级商户号数据", true
	}

	var contractErr *wechatcontracts.SubMerchantSettlementContractError
	if errors.As(err, &contractErr) {
		return "微信结算卡查询响应不符合预期，请联系平台处理", true
	}

	return "", false
}

func settlementVerificationQueryFactResource(runAt time.Time, item db.ListMerchantApplymentsPendingSettlementVerificationRow, firstTradeAt time.Time, checkCount int32, resp *wechatcontracts.SubMerchantSettlementResponse) []byte {
	raw, err := json.Marshal(map[string]any{
		"applyment_id":                      item.ID,
		"merchant_id":                       item.SubjectID,
		"sub_mch_id":                        strings.TrimSpace(textValue(item.SubMchID)),
		"verify_result":                     strings.TrimSpace(resp.VerifyResult),
		"verify_fail_reason":                strings.TrimSpace(resp.VerifyFailReason),
		"settlement_verify_first_trade_at":  firstTradeAt.UTC().Format(time.RFC3339Nano),
		"settlement_verify_last_checked_at": runAt.UTC().Format(time.RFC3339Nano),
		"settlement_verify_check_count":     checkCount,
	})
	if err != nil {
		return nil
	}
	return raw
}

func settlementVerificationTerminalStatus(verifyResult string) string {
	switch strings.TrimSpace(verifyResult) {
	case wechatcontracts.SubMerchantSettlementVerifyResultSuccess:
		return db.ExternalPaymentTerminalStatusSuccess
	case wechatcontracts.SubMerchantSettlementVerifyResultFail:
		return db.ExternalPaymentTerminalStatusFailed
	case wechatcontracts.SubMerchantSettlementVerifyResultVerifying:
		return db.ExternalPaymentTerminalStatusProcessing
	default:
		return db.ExternalPaymentTerminalStatusUnknown
	}
}

func settlementVerificationStringPtr(value string) *string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return nil
	}
	return &trimmedValue
}

func settlementVerificationInt64Ptr(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

func settlementVerificationTimePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func resolveSettlementVerificationState(resp *wechatcontracts.SubMerchantSettlementResponse, checkCount int) (string, string) {
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
