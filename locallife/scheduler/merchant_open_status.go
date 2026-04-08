package scheduler

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	db "github.com/merrydance/locallife/db/sqlc"
)

const merchantOpenStatusCron = "0 * * * * *"

// MerchantOpenStatusScheduler 根据营业时间自动切换显式开启自动模式的商户营业状态。
type MerchantOpenStatusScheduler struct {
	cron  *cron.Cron
	store db.Store
}

func NewMerchantOpenStatusScheduler(store db.Store) *MerchantOpenStatusScheduler {
	return &MerchantOpenStatusScheduler{
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

	updatedMerchantIDs, err := s.store.SyncMerchantOpenStatusByBusinessHours(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to sync merchant open status by business hours")
		return
	}

	if len(updatedMerchantIDs) == 0 {
		return
	}

	log.Info().Int("updated_count", len(updatedMerchantIDs)).Msg("synced merchant open status by business hours")
}
