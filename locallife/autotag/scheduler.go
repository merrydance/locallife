package autotag

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

const (
	// 热卖标签阈值：近7天销量 >= 10 单
	HotSellingThreshold = 10
	// 推荐标签阈值：总销量 >= 5 单
	QualitySalesThreshold = 5
	// 复购率阈值：>= 30%
	RepurchaseRateThreshold = 0.30
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

		if err := s.RefreshAutoTags(ctx); err != nil {
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

		if err := s.RefreshAutoTags(ctx); err != nil {
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

// RefreshAutoTags 刷新所有自动标签
func (s *Scheduler) RefreshAutoTags(ctx context.Context) error {
	log.Info().Msg("refreshing auto tags...")

	// 刷新热卖标签
	if err := s.refreshHotSellingTag(ctx); err != nil {
		log.Error().Err(err).Msg("failed to refresh hot-selling tag")
	}

	// 刷新推荐标签
	if err := s.refreshRecommendedTag(ctx); err != nil {
		log.Error().Err(err).Msg("failed to refresh recommended tag")
	}

	log.Info().Msg("auto tags refresh completed")
	return nil
}

// refreshHotSellingTag 刷新热卖标签
func (s *Scheduler) refreshHotSellingTag(ctx context.Context) error {
	// 获取热卖标签
	tag, err := s.store.GetSystemTagByName(ctx, "热卖")
	if err != nil {
		return err
	}

	// 获取符合热卖条件的菜品
	dishIDs, err := s.store.GetHotSellingDishIDs(ctx, HotSellingThreshold)
	if err != nil {
		return err
	}

	log.Info().
		Int64("tag_id", tag.ID).
		Int("dish_count", len(dishIDs)).
		Msg("hot-selling dishes found")

	// 更新标签关联
	return s.syncDishTags(ctx, tag.ID, dishIDs)
}

// refreshRecommendedTag 刷新推荐标签
func (s *Scheduler) refreshRecommendedTag(ctx context.Context) error {
	// 获取推荐标签
	tag, err := s.store.GetSystemTagByName(ctx, "推荐")
	if err != nil {
		return err
	}

	// 获取符合质量条件的菜品（销量+无投诉）
	qualityDishIDs, err := s.store.GetQualityDishIDs(ctx, QualitySalesThreshold)
	if err != nil {
		return err
	}

	// 进一步过滤：只保留复购率 >= 30% 的菜品
	var recommendedIDs []int64
	for _, dishID := range qualityDishIDs {
		stats, err := s.store.GetDishRepurchaseRate(ctx, pgtype.Int8{Int64: dishID, Valid: true})
		if err != nil {
			continue
		}

		if stats.TotalUsers > 0 {
			rate := float64(stats.RepurchaseUsers) / float64(stats.TotalUsers)
			if rate >= RepurchaseRateThreshold {
				recommendedIDs = append(recommendedIDs, dishID)
			}
		}
	}

	log.Info().
		Int64("tag_id", tag.ID).
		Int("quality_dish_count", len(qualityDishIDs)).
		Int("recommended_dish_count", len(recommendedIDs)).
		Msg("recommended dishes found")

	// 更新标签关联
	return s.syncDishTags(ctx, tag.ID, recommendedIDs)
}

// syncDishTags 同步菜品标签（删除旧的，添加新的）
func (s *Scheduler) syncDishTags(ctx context.Context, tagID int64, newDishIDs []int64) error {
	// 获取当前有此标签的菜品
	currentDishIDs, err := s.store.GetDishIDsWithTag(ctx, tagID)
	if err != nil {
		return err
	}

	// 构建新旧菜品ID集合
	newSet := make(map[int64]bool)
	for _, id := range newDishIDs {
		newSet[id] = true
	}

	currentSet := make(map[int64]bool)
	for _, id := range currentDishIDs {
		currentSet[id] = true
	}

	// 删除不再符合条件的菜品标签
	for _, dishID := range currentDishIDs {
		if !newSet[dishID] {
			// 需要删除单个标签关联
			// 注意：这里用 DeleteDishTagByTagID 会删除所有，改用逐个删除
			// 暂时先用重建策略
		}
	}

	// 简单策略：先删除所有该标签的关联，再重新添加
	if err := s.store.DeleteDishTagByTagID(ctx, tagID); err != nil {
		return err
	}

	// 添加新的标签关联
	for _, dishID := range newDishIDs {
		if err := s.store.UpsertDishTag(ctx, db.UpsertDishTagParams{
			DishID: dishID,
			TagID:  tagID,
		}); err != nil {
			log.Warn().Err(err).Int64("dish_id", dishID).Msg("failed to add dish tag")
		}
	}

	return nil
}
