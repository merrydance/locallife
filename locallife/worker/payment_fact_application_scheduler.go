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
	paymentFactApplicationCron              = "*/1 * * * *"
	paymentFactApplicationBatchLimit        = int32(200)
	paymentFactApplicationProcessingStale   = 15 * time.Minute
	paymentFactApplicationTaskUnique        = 30 * time.Second
	claimRecoveryPaymentFactConsumer        = "claim_recovery_domain"
	baofuVerifyFeePaymentFactConsumerDomain = "baofu_account_verify_fee_domain"
	paymentFactApplicationStaleError        = "stale processing payment fact application reclaimed by scheduler"
)

var paymentFactApplicationSchedulerTargets = []struct {
	consumer           string
	businessObjectType string
}{
	{consumer: profitSharingFactConsumerDomain, businessObjectType: profitSharingFactBusinessObjectOrder},
	{consumer: profitSharingFactConsumerDomain, businessObjectType: profitSharingReturnFactBusinessObject},
	{consumer: claimRecoveryPaymentFactConsumer, businessObjectType: orderPaymentFactBusinessObjectOrder},
	{consumer: riderDepositPaymentFactConsumerDomain, businessObjectType: riderDepositPaymentFactBusinessObjectOrder},
	{consumer: baofuVerifyFeePaymentFactConsumerDomain, businessObjectType: orderPaymentFactBusinessObjectOrder},
	{consumer: orderPaymentFactConsumerDomain, businessObjectType: orderPaymentFactBusinessObjectOrder},
	{consumer: reservationPaymentFactConsumerDomain, businessObjectType: reservationPaymentFactBusinessObjectOrder},
	{consumer: orderRefundFactConsumerDomain, businessObjectType: orderRefundFactBusinessObjectOrder},
	{consumer: reservationRefundFactConsumerDomain, businessObjectType: reservationRefundFactBusinessObjectOrder},
	{consumer: riderDepositRefundFactConsumerDomain, businessObjectType: riderDepositRefundFactBusinessObjectOrder},
}

type PaymentFactApplicationScheduler struct {
	cron        *cron.Cron
	wg          sync.WaitGroup
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	runMu       sync.Mutex
	store       db.Store
	distributor PaymentFactApplicationTaskDistributor
}

func NewPaymentFactApplicationScheduler(store db.Store, distributor PaymentFactApplicationTaskDistributor) *PaymentFactApplicationScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &PaymentFactApplicationScheduler{
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

func (s *PaymentFactApplicationScheduler) Start() error {
	_, err := s.cron.AddFunc(paymentFactApplicationCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("payment fact application scheduler started (every 1 minute)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *PaymentFactApplicationScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("payment fact application scheduler stopped")
}

func (s *PaymentFactApplicationScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *PaymentFactApplicationScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("payment fact application scheduler already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip payment fact application scheduler")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	now := time.Now()
	nowAt := pgtype.Timestamptz{Time: now, Valid: true}
	for _, target := range paymentFactApplicationSchedulerTargets {
		s.reclaimStaleTarget(ctx, target.consumer, target.businessObjectType, now)

		applications, err := s.store.ListRetryableExternalPaymentFactApplicationsByTarget(ctx, db.ListRetryableExternalPaymentFactApplicationsByTargetParams{
			Consumer:           target.consumer,
			BusinessObjectType: target.businessObjectType,
			NowAt:              nowAt,
			LimitCount:         paymentFactApplicationBatchLimit,
		})
		if err != nil {
			log.Error().Err(err).
				Str("consumer", target.consumer).
				Str("business_object_type", target.businessObjectType).
				Msg("list retryable payment fact applications failed")
			continue
		}

		for _, application := range applications {
			if err := s.distributor.DistributeTaskProcessPaymentFactApplication(
				ctx,
				&PaymentFactApplicationPayload{ApplicationID: application.ID},
				asynq.MaxRetry(5),
				asynq.Queue(QueueCritical),
				asynq.Unique(paymentFactApplicationTaskUnique),
			); err != nil {
				log.Error().Err(err).
					Int64("payment_fact_application_id", application.ID).
					Int64("payment_fact_id", application.FactID).
					Str("consumer", application.Consumer).
					Str("business_object_type", application.BusinessObjectType).
					Int64("business_object_id", application.BusinessObjectID).
					Msg("enqueue payment fact application task failed")
			}
		}
	}
}

func (s *PaymentFactApplicationScheduler) reclaimStaleTarget(ctx context.Context, consumer, businessObjectType string, now time.Time) {
	reclaimed, err := s.store.ReclaimStaleExternalPaymentFactApplicationsByTarget(ctx, db.ReclaimStaleExternalPaymentFactApplicationsByTargetParams{
		Consumer:           consumer,
		BusinessObjectType: businessObjectType,
		StaleBefore:        now.Add(-paymentFactApplicationProcessingStale),
		LastError:          pgtype.Text{String: paymentFactApplicationStaleError, Valid: true},
		NextRetryAt:        pgtype.Timestamptz{Time: now, Valid: true},
		LimitCount:         paymentFactApplicationBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).
			Str("consumer", consumer).
			Str("business_object_type", businessObjectType).
			Msg("reclaim stale payment fact applications failed")
		return
	}
	if len(reclaimed) == 0 {
		return
	}
	log.Warn().
		Int("reclaimed_count", len(reclaimed)).
		Str("consumer", consumer).
		Str("business_object_type", businessObjectType).
		Msg("reclaimed stale payment fact applications for retry")
}
