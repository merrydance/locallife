package autotag

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// Scheduler 自动标签定时任务调度器
type Scheduler struct {
	cron  *cron.Cron
	store db.Store
}

// NewScheduler 创建自动标签调度器
func NewScheduler(store db.Store) *Scheduler {
	return &Scheduler{
		cron:  cron.New(),
		store: store,
	}
}

// Start 启动调度器（每小时执行一次）
func (s *Scheduler) Start() error {
	// 每小时执行一次
	_, err := s.cron.AddFunc("0 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := s.RefreshDishStats(ctx); err != nil {
			log.Error().Err(err).Msg("failed to refresh auto tags")
		}
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Info().Msg("auto-tag scheduler started (every hour)")

	// 启动时立即执行一次
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		if err := s.RefreshDishStats(ctx); err != nil {
			log.Error().Err(err).Msg("failed to refresh initial auto tags")
		}
	}()

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.cron.Stop()
	log.Info().Msg("auto-tag scheduler stopped")
}

// RefreshDishStats 刷新所有菜品统计数据（销量、复购率）
// 同时也会刷新自动标签（作为辅助展示）
func (s *Scheduler) RefreshDishStats(ctx context.Context) error {
	log.Info().Msg("starting RefreshDishStats...")

	// 1. 获取所有通过软删除检查的菜品ID
	dishIDs, err := s.store.ListAllDishIDs(ctx)
	if err != nil {
		return err
	}
	log.Info().Int("total_dishes", len(dishIDs)).Msg("processing dishes for stats update")

	// 2. 遍历更新每个菜品的统计数据
	// 注意：生产环境应使用批处理或工作池，这里为简单起见使用串行处理
	for _, id := range dishIDs {
		// 计算月销量
		sales, err := s.store.GetDishSales(ctx, pgtype.Int8{Int64: id, Valid: true})
		if err != nil {
			log.Error().Err(err).Int64("dish_id", id).Msg("failed to get dish sales")
			sales = 0
		}

		// 计算复购率
		var repurchaseRate float64
		repurchaseStats, err := s.store.GetDishRepurchaseRate(ctx, pgtype.Int8{Int64: id, Valid: true})
		if err == nil && repurchaseStats.TotalUsers > 0 {
			repurchaseRate = float64(repurchaseStats.RepurchaseUsers) / float64(repurchaseStats.TotalUsers)
		}

		// 更新到 dishes 表
		err = s.store.UpdateDishStats(ctx, db.UpdateDishStatsParams{
			ID:             id,
			MonthlySales:   sales,
			RepurchaseRate: numericFromFloat(repurchaseRate),
		})
		if err != nil {
			log.Error().Err(err).Int64("dish_id", id).Msg("failed to update dish stats")
		}
	}

	log.Info().Msg("dish stats updated successfully")

	log.Info().Msg("RefreshDishStats completed")
	return nil
}

// numericFromFloat converts float64 to pgtype.Numeric with guards for NaN/Inf.
func numericFromFloat(f float64) pgtype.Numeric {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		f = 0
	}
	var n pgtype.Numeric
	if err := n.Scan(f); err != nil {
		_ = n.Scan(0) // fallback to 0 to avoid NULL constraint violations
	}
	return n
}
