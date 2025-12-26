package algorithm

import (
	"context"
	"math/rand"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

// PersonalizedRecommender 个性化推荐引擎（EE算法：Exploitation & Exploration）
type PersonalizedRecommender struct {
	store db.Store
}

// NewPersonalizedRecommender 创建推荐引擎实例
func NewPersonalizedRecommender(store db.Store) *PersonalizedRecommender {
	return &PersonalizedRecommender{
		store: store,
	}
}

// PersonalizedRecommendConfig EE算法配置（个性化推荐专用）
type PersonalizedRecommendConfig struct {
	ExploitationRatio float64 // 喜好推荐比例（基于用户偏好）
	ExplorationRatio  float64 // 探索推荐比例（新品/未购买）
	RandomRatio       float64 // 随机推荐比例
}

// DefaultPersonalizedConfig 默认EE算法配置：60%喜好 + 30%探索 + 10%随机
func DefaultPersonalizedConfig() PersonalizedRecommendConfig {
	return PersonalizedRecommendConfig{
		ExploitationRatio: 0.60,
		ExplorationRatio:  0.30,
		RandomRatio:       0.10,
	}
}

// NewUserPersonalizedConfig 新用户配置：30%喜好 + 50%探索 + 20%随机
// 新用户订单<3笔，提高探索比例帮助发现口味
func NewUserPersonalizedConfig() PersonalizedRecommendConfig {
	return PersonalizedRecommendConfig{
		ExploitationRatio: 0.30,
		ExplorationRatio:  0.50,
		RandomRatio:       0.20,
	}
}

// RecommendDishes 推荐菜品
func (r *PersonalizedRecommender) RecommendDishes(
	ctx context.Context,
	userID int64,
	config PersonalizedRecommendConfig,
	limit int,
) ([]int64, error) {
	// 获取用户偏好
	preferences, err := r.store.GetUserPreferences(ctx, userID)
	if err != nil {
		// 用户无偏好，返回热门菜品
		return r.getPopularDishes(ctx, limit)
	}

	// 判断是否为新用户（订单<3笔）
	isNewUser := preferences.PurchaseFrequency < 3

	// 计算各部分推荐数量
	exploitationCount := int(float64(limit) * config.ExploitationRatio)
	explorationCount := int(float64(limit) * config.ExplorationRatio)
	randomCount := limit - exploitationCount - explorationCount

	var dishIDs []int64

	// 1. Exploitation: 基于用户偏好推荐（60%）
	if exploitationCount > 0 && !isNewUser {
		exploitDishes, err := r.getExploitationDishes(ctx, preferences, exploitationCount)
		if err == nil {
			dishIDs = append(dishIDs, exploitDishes...)
		}
	}

	// 2. Exploration: 探索推荐（30%）
	if explorationCount > 0 {
		exploreDishes, err := r.getExplorationDishes(ctx, userID, preferences, explorationCount)
		if err == nil {
			dishIDs = append(dishIDs, exploreDishes...)
		}
	}

	// 3. Random: 随机推荐（10%）
	if randomCount > 0 {
		randomDishes, err := r.getRandomDishes(ctx, randomCount)
		if err == nil {
			dishIDs = append(dishIDs, randomDishes...)
		}
	}

	// 如果推荐数不足，用热门菜品补齐
	if len(dishIDs) < limit {
		popularDishes, _ := r.getPopularDishes(ctx, limit-len(dishIDs))
		dishIDs = append(dishIDs, popularDishes...)
	}

	// 去重并打乱顺序
	dishIDs = uniqueAndShuffle(dishIDs)
	if len(dishIDs) > limit {
		dishIDs = dishIDs[:limit]
	}

	return dishIDs, nil
}

// getExploitationDishes 基于用户偏好推荐菜品
// 使用用户的价格区间偏好推荐菜品
func (r *PersonalizedRecommender) getExploitationDishes(
	ctx context.Context,
	pref db.UserPreference,
	limit int,
) ([]int64, error) {
	// 获取价格区间
	priceMin := int64(0)
	priceMax := int64(100000) // 默认最大价格 1000元
	if pref.PriceRangeMin.Valid {
		priceMin = pref.PriceRangeMin.Int64
	}
	if pref.PriceRangeMax.Valid && pref.PriceRangeMax.Int64 > 0 {
		priceMax = pref.PriceRangeMax.Int64
	}

	rows, err := r.store.GetDishIDsByCuisines(ctx, db.GetDishIDsByCuisinesParams{
		Price:   priceMin,
		Price_2: priceMax,
		Limit:   int32(limit),
	})
	if err != nil {
		return []int64{}, err
	}

	return rows, nil
}

// getExplorationDishes 探索推荐（用户未购买过的热门菜品）
func (r *PersonalizedRecommender) getExplorationDishes(
	ctx context.Context,
	userID int64,
	_ db.UserPreference,
	limit int,
) ([]int64, error) {
	// 获取用户未购买过的热门菜品
	rows, err := r.store.GetExploreDishes(ctx, db.GetExploreDishesParams{
		UserID: userID,
		Limit:  int32(limit),
	})
	if err != nil {
		return []int64{}, err
	}

	dishIDs := make([]int64, len(rows))
	for i, row := range rows {
		dishIDs[i] = row.ID
	}
	return dishIDs, nil
}

// getRandomDishes 随机推荐菜品
func (r *PersonalizedRecommender) getRandomDishes(
	ctx context.Context,
	limit int,
) ([]int64, error) {
	// 从所有在线菜品中随机选择
	rows, err := r.store.GetRandomDishes(ctx, int32(limit))
	if err != nil {
		return []int64{}, err
	}
	return rows, nil
}

// getPopularDishes 获取热门菜品（备选方案）
func (r *PersonalizedRecommender) getPopularDishes(
	ctx context.Context,
	limit int,
) ([]int64, error) {
	// 按销量排序获取热门菜品
	rows, err := r.store.GetPopularDishes(ctx, int32(limit))
	if err != nil {
		return []int64{}, err
	}

	dishIDs := make([]int64, len(rows))
	for i, row := range rows {
		dishIDs[i] = row.ID
	}
	return dishIDs, nil
}

// RecommendCombos 推荐套餐（基于热门套餐）
func (r *PersonalizedRecommender) RecommendCombos(
	ctx context.Context,
	_ int64,
	_ PersonalizedRecommendConfig,
	limit int,
) ([]int64, error) {
	// 获取热门套餐
	rows, err := r.store.GetPopularCombos(ctx, int32(limit))
	if err != nil {
		return []int64{}, err
	}

	comboIDs := make([]int64, len(rows))
	for i, row := range rows {
		comboIDs[i] = row.ID
	}
	return comboIDs, nil
}

// RecommendMerchants 推荐商户（基于订单量等经营数据）
func (r *PersonalizedRecommender) RecommendMerchants(
	ctx context.Context,
	_ int64,
	_ PersonalizedRecommendConfig,
	limit int,
) ([]int64, error) {
	// 获取热门商户
	rows, err := r.store.GetPopularMerchants(ctx, int32(limit))
	if err != nil {
		return []int64{}, err
	}

	merchantIDs := make([]int64, len(rows))
	for i, row := range rows {
		merchantIDs[i] = row.ID
	}
	return merchantIDs, nil
}

// uniqueAndShuffle 去重并打乱数组
func uniqueAndShuffle(ids []int64) []int64 {
	// 去重
	seen := make(map[int64]bool)
	unique := []int64{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}

	// 打乱顺序
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(unique), func(i, j int) {
		unique[i], unique[j] = unique[j], unique[i]
	})

	return unique
}

// CheckRecommendationFatigue 检查推荐疲劳
// 同一菜品/商户推荐间隔至少1天
// 注意：此功能需要 recommendation_history 表支持，当前未实现
// 建议：如需启用，请先创建推荐历史表记录每次推荐，然后查询是否在24小时内已推荐过
func (r *PersonalizedRecommender) CheckRecommendationFatigue(
	ctx context.Context,
	userID int64,
	itemID int64,
	itemType string, // "dish", "combo", "merchant"
) (bool, error) {
	// 此功能需要：
	// 1. 创建 recommendation_history 表（user_id, item_id, item_type, recommended_at）
	// 2. 每次推荐时记录
	// 3. 查询是否在24小时内已推荐过
	// 当前返回 false 表示不疲劳（可以推荐）
	return false, nil
}
