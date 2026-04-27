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
	paymentFactApplicationCron         = "*/1 * * * *"
	paymentFactApplicationBatchLimit   = int32(200)
	paymentFactApplicationTaskUnique   = 30 * time.Second
	claimRecoveryPaymentFactConsumer   = "claim_recovery_domain"
	settlementFactBusinessObjectConfig = "merchant_payment_config"
)

var paymentFactApplicationSchedulerTargets = []struct {
	consumer           string
	businessObjectType string
}{
	{consumer: profitSharingFactConsumerDomain, businessObjectType: profitSharingFactBusinessObjectOrder},
	{consumer: profitSharingFactConsumerDomain, businessObjectType: profitSharingReturnFactBusinessObject},
	{consumer: applymentFactConsumerDomain, businessObjectType: applymentFactBusinessObjectApplyment},
	{consumer: settlementVerificationFactConsumerDomain, businessObjectType: applymentFactBusinessObjectApplyment},
	{consumer: settlementVerificationFactConsumerDomain, businessObjectType: settlementFactBusinessObjectConfig},
	{consumer: merchantWithdrawFactConsumerDomain, businessObjectType: merchantWithdrawFactBusinessType},
	{consumer: merchantWithdrawFactConsumerDomain, businessObjectType: merchantCancelWithdrawFactBusinessType},
	{consumer: claimRecoveryPaymentFactConsumer, businessObjectType: orderPaymentFactBusinessObjectOrder},
	{consumer: riderDepositPaymentFactConsumerDomain, businessObjectType: riderDepositPaymentFactBusinessObjectOrder},
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

	nowAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	for _, target := range paymentFactApplicationSchedulerTargets {
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
