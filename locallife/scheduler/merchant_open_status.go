package scheduler

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/websocket"
)

const merchantOpenStatusCron = "0 * * * * *"

// MerchantOpenStatusScheduler 收敛商户营业状态的定时变化。
type MerchantOpenStatusScheduler struct {
	cron      *cron.Cron
	store     db.Store
	publisher websocket.MerchantStatusChangePublisher
}

func NewMerchantOpenStatusScheduler(store db.Store, publisher websocket.MerchantStatusChangePublisher) *MerchantOpenStatusScheduler {
	return &MerchantOpenStatusScheduler{
		cron: cron.New(
			cron.WithSeconds(),
			cron.WithChain(
				cron.SkipIfStillRunning(cron.DefaultLogger),
				cron.Recover(cron.DefaultLogger),
			),
		),
		store:     store,
		publisher: publisher,
	}
}

func (s *MerchantOpenStatusScheduler) Start() error {
	_, err := s.cron.AddFunc(merchantOpenStatusCron, s.syncMerchantOpenStatus)
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("merchant open status scheduler started")
	return nil
}

func (s *MerchantOpenStatusScheduler) Stop() {
	s.cron.Stop()
	log.Info().Msg("merchant open status scheduler stopped")
}

func (s *MerchantOpenStatusScheduler) RunOnce() {
	s.syncMerchantOpenStatus()
}

func (s *MerchantOpenStatusScheduler) syncMerchantOpenStatus() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	autoClosedMerchantIDs, err := s.store.AutoCloseMerchants(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to auto close merchants by manual auto_close_at")
	} else {
		s.publishMerchantStatusChanges(ctx, autoClosedMerchantIDs, "auto_close")
	}

	updatedMerchantIDs, err := s.store.SyncMerchantOpenStatusByBusinessHours(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to sync merchant open status by business hours")
		return
	}

	s.publishMerchantStatusChanges(ctx, updatedMerchantIDs, "business_hours")

	clearedManualOverrideCount, err := s.store.ClearExpiredMerchantManualOpenStatusOverrides(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to clear expired merchant manual open status overrides")
	}

	if len(autoClosedMerchantIDs) == 0 && len(updatedMerchantIDs) == 0 && clearedManualOverrideCount == 0 {
		return
	}

	log.Info().
		Int("auto_closed_count", len(autoClosedMerchantIDs)).
		Int("business_hours_updated_count", len(updatedMerchantIDs)).
		Int64("manual_override_cleared_count", clearedManualOverrideCount).
		Msg("synced merchant open status")
}

func (s *MerchantOpenStatusScheduler) publishMerchantStatusChanges(ctx context.Context, merchantIDs []int64, source string) {
	for _, merchantID := range merchantIDs {
		row, err := s.store.GetMerchantIsOpen(ctx, merchantID)
		if err != nil {
			log.Error().Err(err).Int64("merchant_id", merchantID).Str("source", source).Msg("failed to load merchant status after sync")
			continue
		}

		var autoCloseAt *time.Time
		if row.AutoCloseAt.Valid {
			autoCloseAt = &row.AutoCloseAt.Time
		}

		if s.publisher != nil {
			if err := s.publisher.PublishMerchantStatusChange(ctx, merchantID, row.IsOpen, autoCloseAt, source); err != nil {
				log.Error().Err(err).Int64("merchant_id", merchantID).Str("source", source).Msg("failed to publish merchant status change after sync")
			}
		}
	}
}
