package worker

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	paymentRecoveryCron       = "*/5 * * * *"
	paymentRecoveryBatchLimit = int32(200)
	paymentRecoveryMinAge     = 2 * time.Minute
)

// PaymentRecoveryScheduler scans paid but unprocessed payment orders and re-enqueues processing.
type PaymentRecoveryScheduler struct {
	cron        *cron.Cron
	wg          sync.WaitGroup
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	runMu       sync.Mutex
	store       db.Store
	distributor any
}

// NewPaymentRecoveryScheduler creates a new scheduler for payment recovery.
func NewPaymentRecoveryScheduler(store db.Store, distributor any) *PaymentRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &PaymentRecoveryScheduler{
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

// Start starts the recovery scheduler.
func (s *PaymentRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(paymentRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("payment recovery scheduler started (every 5 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

// RunOnce triggers a single recovery scan.
func (s *PaymentRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
}

// Stop stops the scheduler.
func (s *PaymentRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("payment recovery scheduler stopped")
}

func (s *PaymentRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("payment recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	if s.distributor == nil {
		log.Warn().Msg("task distributor not configured, skip payment recovery")
		return
	}
	if _, ok := s.distributor.(PaymentFactApplicationTaskDistributor); !ok {
		log.Warn().Msg("payment fact application distributor not configured, skip payment recovery")
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cutoff := time.Now().Add(-paymentRecoveryMinAge)
	orders, err := s.store.ListPaidUnprocessedPaymentOrders(ctx, db.ListPaidUnprocessedPaymentOrdersParams{
		PaidAt: pgtype.Timestamptz{
			Time:  cutoff,
			Valid: true,
		},
		Limit: paymentRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list paid unprocessed payment orders failed")
		return
	}

	for _, order := range orders {
		refundOrders, err := s.store.ListRefundOrdersByPaymentOrder(ctx, order.ID)
		if err != nil {
			log.Error().Err(err).
				Int64("payment_order_id", order.ID).
				Str("out_trade_no", order.OutTradeNo).
				Msg("list refund orders for payment recovery failed")
			continue
		}
		if len(refundOrders) > 0 {
			log.Warn().
				Int64("payment_order_id", order.ID).
				Str("out_trade_no", order.OutTradeNo).
				Int("refund_order_count", len(refundOrders)).
				Msg("skip payment recovery because refund activity exists")
			continue
		}

		if shouldRecordOrderPaymentRecoveryFact(order) {
			application, factErr := recordRecoveredOrderPaymentFact(ctx, s.store, order)
			if factErr != nil {
				log.Error().Err(factErr).
					Int64("payment_order_id", order.ID).
					Str("out_trade_no", order.OutTradeNo).
					Msg("record recovered order payment fact failed")
				continue
			}
			enqueueErr := enqueueOrderPaymentFactApplication(ctx, s.distributor, application)
			if enqueueErr != nil {
				log.Error().Err(enqueueErr).
					Int64("payment_order_id", order.ID).
					Str("out_trade_no", order.OutTradeNo).
					Msg("enqueue recovered order payment fact application failed")
			}
			continue
		}
		if shouldRecordReservationPaymentRecoveryFact(order) {
			application, factErr := recordRecoveredReservationPaymentFact(ctx, s.store, order)
			if factErr != nil {
				log.Error().Err(factErr).
					Int64("payment_order_id", order.ID).
					Str("out_trade_no", order.OutTradeNo).
					Msg("record recovered reservation payment fact failed")
				continue
			}
			enqueueErr := enqueueReservationPaymentFactApplication(ctx, s.distributor, application)
			if enqueueErr != nil {
				log.Error().Err(enqueueErr).
					Int64("payment_order_id", order.ID).
					Str("out_trade_no", order.OutTradeNo).
					Msg("enqueue recovered reservation payment fact application failed")
			}
			continue
		}
		if shouldRecordDirectPaymentRecoveryFact(order) {
			application, factErr := recordRecoveredDirectPaymentFact(ctx, s.store, order)
			if factErr != nil {
				log.Error().Err(factErr).
					Int64("payment_order_id", order.ID).
					Str("out_trade_no", order.OutTradeNo).
					Str("business_type", order.BusinessType).
					Msg("record recovered direct payment fact failed")
				continue
			}
			enqueueErr := enqueueOrderPaymentFactApplication(ctx, s.distributor, application)
			if enqueueErr != nil {
				log.Error().Err(enqueueErr).
					Int64("payment_order_id", order.ID).
					Str("out_trade_no", order.OutTradeNo).
					Str("business_type", order.BusinessType).
					Msg("enqueue recovered direct payment fact application failed")
			}
			continue
		}

		log.Warn().
			Int64("payment_order_id", order.ID).
			Str("out_trade_no", order.OutTradeNo).
			Str("payment_channel", order.PaymentChannel).
			Str("business_type", order.BusinessType).
			Msg("skip paid unprocessed payment recovery because no fact application target is configured")
	}
}

func shouldRecordOrderPaymentRecoveryFact(order db.PaymentOrder) bool {
	return order.BusinessType == db.ExternalPaymentBusinessOwnerOrder && db.PaymentOrderUsesEcommerceChannel(order)
}

func recordRecoveredOrderPaymentFact(ctx context.Context, store db.Store, order db.PaymentOrder) (*db.ExternalPaymentFactApplication, error) {
	capability := db.ExternalPaymentCapabilityPartnerJSAPIPayment
	if order.CombinedPaymentID.Valid {
		capability = db.ExternalPaymentCapabilityCombinePayment
	}

	service := logic.NewPaymentFactService(store)
	result, err := service.RecordExternalPaymentFact(ctx, logic.RecordExternalPaymentFactInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelEcommerce,
		Capability:           capability,
		FactSource:           db.ExternalPaymentFactSourceManualReconciliation,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    order.OutTradeNo,
		ExternalSecondaryKey: orderPaymentOptionalStringPtr(order.TransactionID),
		BusinessOwner:        orderPaymentStringPtr(db.ExternalPaymentBusinessOwnerOrder),
		BusinessObjectType:   orderPaymentStringPtr(orderPaymentFactBusinessObjectOrder),
		BusinessObjectID:     orderPaymentInt64Ptr(order.ID),
		UpstreamState:        "SUCCESS",
		TerminalStatus:       db.ExternalPaymentTerminalStatusSuccess,
		Amount:               orderPaymentInt64Ptr(order.Amount),
		Currency:             "CNY",
		RawResource:          recoveredOrderPaymentFactResource(order),
		DedupeKey:            recoveredOrderPaymentFactDedupeKey(order, capability),
		Application: &logic.ExternalPaymentFactApplicationTarget{
			Consumer:           orderPaymentFactConsumerDomain,
			BusinessObjectType: orderPaymentFactBusinessObjectOrder,
			BusinessObjectID:   order.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	return result.Application, nil
}
