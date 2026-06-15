package scheduler

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	// DineInCheckoutRecoveryDelay is a short buffer after payment before backend recovery closes the session.
	DineInCheckoutRecoveryDelay = 2 * time.Minute

	dineInCheckoutRecoveryBatchLimit int32 = 100
)

var (
	dineInCheckoutRecoveryScansTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dine_in_checkout_recovery_scans_total",
			Help: "Total number of dine-in checkout recovery scans by result",
		},
		[]string{"result"},
	)

	dineInCheckoutRecoverySessionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dine_in_checkout_recovery_sessions_total",
			Help: "Total number of dine-in checkout recovery sessions observed by result",
		},
		[]string{"result"},
	)
)

// DineInCheckoutRecoveryScheduler closes paid dine-in sessions when the client checkout callback path did not.
type DineInCheckoutRecoveryScheduler struct {
	cron  *cron.Cron
	store db.Store
}

func NewDineInCheckoutRecoveryScheduler(store db.Store) *DineInCheckoutRecoveryScheduler {
	return &DineInCheckoutRecoveryScheduler{
		cron: cron.New(
			cron.WithSeconds(),
			cron.WithChain(
				cron.SkipIfStillRunning(cron.DefaultLogger),
				cron.Recover(cron.DefaultLogger),
			),
		),
		store: store,
	}
}

func (s *DineInCheckoutRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc("0 */5 * * * *", s.recoverPaidOpenDineInSessions)
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("dine-in checkout recovery scheduler started")
	return nil
}

func (s *DineInCheckoutRecoveryScheduler) Stop() {
	s.cron.Stop()
	log.Info().Msg("dine-in checkout recovery scheduler stopped")
}

func (s *DineInCheckoutRecoveryScheduler) RunOnce() {
	s.recoverPaidOpenDineInSessions()
}

func (s *DineInCheckoutRecoveryScheduler) recoverPaidOpenDineInSessions() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	openedBefore := time.Now().Add(-DineInCheckoutRecoveryDelay)
	sessions, err := s.store.ListPaidOpenDineInSessionsForCheckoutRecovery(ctx, db.ListPaidOpenDineInSessionsForCheckoutRecoveryParams{
		OpenedBefore: openedBefore,
		Limit:        dineInCheckoutRecoveryBatchLimit,
	})
	if err != nil {
		dineInCheckoutRecoveryScansTotal.WithLabelValues("list_error").Inc()
		log.Error().Err(err).Msg("failed to list paid open dine-in sessions for checkout recovery")
		return
	}
	dineInCheckoutRecoveryScansTotal.WithLabelValues("success").Inc()
	if len(sessions) == 0 {
		return
	}
	dineInCheckoutRecoverySessionsTotal.WithLabelValues("listed").Add(float64(len(sessions)))

	closedCount := 0
	for _, session := range sessions {
		_, err := s.store.CloseDiningSessionTx(ctx, db.CloseDiningSessionTxParams{
			ID:         session.ID,
			MerchantID: session.MerchantID,
		})
		if err != nil {
			dineInCheckoutRecoverySessionsTotal.WithLabelValues("close_failed").Inc()
			log.Warn().Err(err).Int64("session_id", session.ID).Int64("merchant_id", session.MerchantID).Msg("failed to recover paid dine-in checkout")
			continue
		}
		dineInCheckoutRecoverySessionsTotal.WithLabelValues("closed").Inc()
		closedCount++
	}

	log.Info().Int("closed", closedCount).Int("total", len(sessions)).Msg("dine-in checkout recovery scan finished")
}
