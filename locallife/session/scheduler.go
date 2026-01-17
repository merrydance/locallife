package session

import (
	"context"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// Scheduler session cleanup scheduler
// 定期清理已过期的会话
// 通过 cron 执行，避免在登录/刷新链路中做清理
// 默认每小时执行一次
// 启动时立即执行一次
// 使用 context 超时控制，避免长时间占用连接
type Scheduler struct {
	cron  *cron.Cron
	store db.Store
}

// NewScheduler 创建会话清理调度器
func NewScheduler(store db.Store) *Scheduler {
	return &Scheduler{
		cron:  cron.New(),
		store: store,
	}
}

// Start 启动调度器（每小时执行一次）
func (s *Scheduler) Start() error {
	_, err := s.cron.AddFunc("0 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if err := s.store.DeleteExpiredSessions(ctx); err != nil {
			log.Error().Err(err).Msg("failed to delete expired sessions")
		}
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("session cleanup scheduler started (every hour)")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if err := s.store.DeleteExpiredSessions(ctx); err != nil {
			log.Error().Err(err).Msg("failed to delete expired sessions on startup")
		}
	}()

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.cron.Stop()
	log.Info().Msg("session cleanup scheduler stopped")
}
