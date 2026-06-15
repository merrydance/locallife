package scheduler

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	db "github.com/merrydance/locallife/db/sqlc"
)

const (
	// DineInCheckoutRecoveryDelay is a short buffer after payment before backend recovery closes the session.
	DineInCheckoutRecoveryDelay = 2 * time.Minute

	dineInCheckoutRecoveryBatchLimit int32 = 100
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
		log.Error().Err(err).Msg("failed to list paid open dine-in sessions for checkout recovery")
		return
	}
	if len(sessions) == 0 {
		return
	}

	closedCount := 0
	for _, session := range sessions {
		_, err := s.store.CloseDiningSessionTx(ctx, db.CloseDiningSessionTxParams{
			ID:         session.ID,
			MerchantID: session.MerchantID,
		})
		if err != nil {
			log.Warn().Err(err).Int64("session_id", session.ID).Int64("merchant_id", session.MerchantID).Msg("failed to recover paid dine-in checkout")
			continue
		}
		closedCount++
	}

	log.Info().Int("closed", closedCount).Int("total", len(sessions)).Msg("dine-in checkout recovery scan finished")
}
