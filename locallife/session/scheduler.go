package session

import (
	"context"
	"sync"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// Scheduler session cleanup scheduler
// 定期清理已过期的会话
type Scheduler struct {
	cron       *cron.Cron
	wg         sync.WaitGroup
	stopCtx    context.Context
	stopCancel context.CancelFunc
	runMu      sync.Mutex
	store      db.Store
}

// NewScheduler 创建会话清理调度器
func NewScheduler(store db.Store) *Scheduler {
	stopCtx, stopCancel := context.WithCancel(context.Background())
	return &Scheduler{
		cron: cron.New(cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		)),
		stopCtx:    stopCtx,
		stopCancel: stopCancel,
		store:      store,
	}
}

// Start 启动调度器（每小时执行一次）
func (s *Scheduler) Start() error {
	_, err := s.cron.AddFunc("0 * * * *", func() {
		s.runOnce(s.stopCtx)
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("session cleanup scheduler started (every hour)")

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runOnce(s.stopCtx)
	}()

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.stopCancel()
	s.cron.Stop()
	s.wg.Wait()
	log.Info().Msg("session cleanup scheduler stopped")
}

func (s *Scheduler) runOnce(ctx context.Context) {
	if !s.runMu.TryLock() {
		log.Warn().Msg("session cleanup already running, skipping concurrent execution")
		return
	}
	defer s.runMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := s.store.DeleteExpiredSessions(ctx); err != nil {
		log.Error().Err(err).Msg("failed to delete expired sessions")
	}
}
