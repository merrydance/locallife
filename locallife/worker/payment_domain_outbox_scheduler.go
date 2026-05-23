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
	paymentDomainOutboxCron       = "*/1 * * * *"
	paymentDomainOutboxBatchLimit = int32(200)
	paymentDomainOutboxTaskUnique = 30 * time.Second
)

var defaultPaymentDomainOutboxSchedulerEventTypes = []string{
	PaymentDomainOutboxEventDispatcherProbe,
	db.PaymentDomainOutboxEventOrderPaymentSucceeded,
	db.PaymentDomainOutboxEventReservationPaymentSucceeded,
	db.PaymentDomainOutboxEventProfitSharingResultReady,
	db.PaymentDomainOutboxEventOrderRefundSucceeded,
	db.PaymentDomainOutboxEventOrderRefundAbnormal,
	db.PaymentDomainOutboxEventReservationRefundAbnormal,
	db.PaymentDomainOutboxEventRiderDepositRefundAbnormal,
}

type PaymentDomainOutboxScheduler struct {
	cron        *cron.Cron
	wg          sync.WaitGroup
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	runMu       sync.Mutex
	store       db.Store
	distributor PaymentDomainOutboxTaskDistributor
	eventTypes  []string
}

func NewPaymentDomainOutboxScheduler(store db.Store, distributor PaymentDomainOutboxTaskDistributor) *PaymentDomainOutboxScheduler {
	return NewPaymentDomainOutboxSchedulerWithEventTypes(store, distributor, defaultPaymentDomainOutboxSchedulerEventTypes)
}

func NewPaymentDomainOutboxSchedulerWithEventTypes(store db.Store, distributor PaymentDomainOutboxTaskDistributor, eventTypes []string) *PaymentDomainOutboxScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &PaymentDomainOutboxScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:     stopCtx,
		stopCancel:  stopCancel,
		store:       store,
		distributor: distributor,
		eventTypes:  normalizePaymentDomainOutboxSchedulerEventTypes(eventTypes),
	}
}

func normalizePaymentDomainOutboxSchedulerEventTypes(eventTypes []string) []string {
	if len(eventTypes) == 0 {
		return append([]string(nil), defaultPaymentDomainOutboxSchedulerEventTypes...)
	}

	normalized := make([]string, 0, len(eventTypes))
	seen := make(map[string]struct{}, len(eventTypes))
	for _, eventType := range eventTypes {
		if eventType == "" {
			continue
		}
		if _, ok := seen[eventType]; ok {
			continue
		}
		seen[eventType] = struct{}{}
		normalized = append(normalized, eventType)
	}
	if len(normalized) == 0 {
		return append([]string(nil), defaultPaymentDomainOutboxSchedulerEventTypes...)
	}
	return normalized
}

func (s *PaymentDomainOutboxScheduler) Start() error {
	_, err := s.cron.AddFunc(paymentDomainOutboxCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Strs("event_types", s.eventTypes).Msg("payment domain outbox scheduler started (every 1 minute)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

func (s *PaymentDomainOutboxScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("payment domain outbox scheduler stopped")
}

func (s *PaymentDomainOutboxScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *PaymentDomainOutboxScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("payment domain outbox scheduler already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip payment domain outbox scheduler")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	now := time.Now()
	for _, eventType := range s.eventTypes {
		s.enqueuePendingEventType(ctx, eventType, now)
	}
}

func (s *PaymentDomainOutboxScheduler) enqueuePendingEventType(ctx context.Context, eventType string, now time.Time) {
	entries, err := s.store.ListPendingPaymentDomainOutboxByEventType(ctx, db.ListPendingPaymentDomainOutboxByEventTypeParams{
		EventType:  eventType,
		NowAt:      pgtype.Timestamptz{Time: now, Valid: true},
		LimitCount: paymentDomainOutboxBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Str("event_type", eventType).Msg("list pending payment domain outbox entries failed")
		return
	}

	for _, entry := range entries {
		if err := s.distributor.DistributeTaskProcessPaymentDomainOutbox(
			ctx,
			&PaymentDomainOutboxPayload{OutboxID: entry.ID},
			asynq.MaxRetry(0),
			asynq.Queue(QueueCritical),
			asynq.Unique(paymentDomainOutboxTaskUnique),
		); err != nil {
			log.Error().Err(err).
				Int64("payment_domain_outbox_id", entry.ID).
				Str("event_type", entry.EventType).
				Str("aggregate_type", entry.AggregateType).
				Int64("aggregate_id", entry.AggregateID).
				Msg("enqueue payment domain outbox task failed")
		}
	}
}
