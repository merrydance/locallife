package worker

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	wechatNotificationRecoveryCron       = "*/5 * * * *"
	wechatNotificationRecoveryBatchLimit = int32(200)
	wechatNotificationRecoveryMinAge     = 10 * time.Minute
)

// WechatNotificationRecoveryScheduler releases stale, unprocessed notification claims
// so subsequent WeChat retries can re-enter the normal callback path.
type WechatNotificationRecoveryScheduler struct {
	cron       *cron.Cron
	wg         sync.WaitGroup
	stopCtx    context.Context
	stopCancel context.CancelFunc
	runMu      sync.Mutex
	store      db.Store
}

// NewWechatNotificationRecoveryScheduler creates a new stale notification recovery scheduler.
func NewWechatNotificationRecoveryScheduler(store db.Store) *WechatNotificationRecoveryScheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &WechatNotificationRecoveryScheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:    stopCtx,
		stopCancel: stopCancel,
		store:      store,
	}
}

// Start starts the scheduler.
func (s *WechatNotificationRecoveryScheduler) Start() error {
	_, err := s.cron.AddFunc(wechatNotificationRecoveryCron, func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("wechat notification recovery scheduler started (every 5 minutes)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()
	return nil
}

// Stop stops the scheduler.
func (s *WechatNotificationRecoveryScheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("wechat notification recovery scheduler stopped")
}

// RunOnce triggers a single scan.
func (s *WechatNotificationRecoveryScheduler) RunOnce() {
	s.runOnce(context.Background())
}

func (s *WechatNotificationRecoveryScheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("wechat notification recovery already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cutoff := time.Now().Add(-wechatNotificationRecoveryMinAge)
	notifications, err := s.store.ListStaleUnprocessedWechatNotifications(ctx, db.ListStaleUnprocessedWechatNotificationsParams{
		CreatedAt: pgtype.Timestamp{Time: cutoff, Valid: true},
		Limit:     wechatNotificationRecoveryBatchLimit,
	})
	if err != nil {
		log.Error().Err(err).Msg("list stale unprocessed wechat notifications failed")
		return
	}

	for _, notification := range notifications {
		if err := s.store.ReleaseWechatNotificationClaim(ctx, notification.ID); err != nil {
			log.Error().Err(err).
				Str("notification_id", notification.ID).
				Str("event_type", notification.EventType).
				Msg("release stale wechat notification claim failed")
			continue
		}

		log.Warn().
			Str("notification_id", notification.ID).
			Str("event_type", notification.EventType).
			Bool("out_trade_no_valid", notification.OutTradeNo.Valid).
			Bool("transaction_id_valid", notification.TransactionID.Valid).
			Msg("released stale unprocessed wechat notification claim")
	}
}
