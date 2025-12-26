package db

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// numericFromFloat64 将 float64 转换为 pgtype.Numeric
func numericFromFloat64(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	n.Int = big.NewInt(int64(f * 1000000))
	n.Exp = -6
	n.Valid = true
	return n
}

// ==================== TrackBehavior Tests ====================

func TestTrackBehavior_ViewDish(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	behavior, err := testStore.TrackBehavior(context.Background(), TrackBehaviorParams{
		UserID:       user.ID,
		BehaviorType: "view",
		DishID:       pgtype.Int8{Int64: dish.ID, Valid: true},
		Duration:     pgtype.Int4{Int32: 15, Valid: true},
	})

	require.NoError(t, err)
	require.NotZero(t, behavior.ID)
	require.Equal(t, user.ID, behavior.UserID)
	require.Equal(t, "view", behavior.BehaviorType)
	require.True(t, behavior.DishID.Valid)
	require.Equal(t, dish.ID, behavior.DishID.Int64)
	require.False(t, behavior.ComboID.Valid)
	require.False(t, behavior.MerchantID.Valid)
	require.True(t, behavior.Duration.Valid)
	require.Equal(t, int32(15), behavior.Duration.Int32)
}

func TestTrackBehavior_DetailCombo(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	combo := createRandomComboSet(t, merchant.ID)

	behavior, err := testStore.TrackBehavior(context.Background(), TrackBehaviorParams{
		UserID:       user.ID,
		BehaviorType: "detail",
		ComboID:      pgtype.Int8{Int64: combo.ID, Valid: true},
		Duration:     pgtype.Int4{Int32: 60, Valid: true},
	})

	require.NoError(t, err)
	require.NotZero(t, behavior.ID)
	require.Equal(t, "detail", behavior.BehaviorType)
	require.True(t, behavior.ComboID.Valid)
	require.Equal(t, combo.ID, behavior.ComboID.Int64)
	require.False(t, behavior.DishID.Valid)
}

func TestTrackBehavior_CartMerchant(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	behavior, err := testStore.TrackBehavior(context.Background(), TrackBehaviorParams{
		UserID:       user.ID,
		BehaviorType: "cart",
		MerchantID:   pgtype.Int8{Int64: merchant.ID, Valid: true},
	})

	require.NoError(t, err)
	require.NotZero(t, behavior.ID)
	require.Equal(t, "cart", behavior.BehaviorType)
	require.True(t, behavior.MerchantID.Valid)
	require.Equal(t, merchant.ID, behavior.MerchantID.Int64)
	require.False(t, behavior.Duration.Valid)
}

func TestTrackBehavior_Purchase(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	behavior, err := testStore.TrackBehavior(context.Background(), TrackBehaviorParams{
		UserID:       user.ID,
		BehaviorType: "purchase",
		DishID:       pgtype.Int8{Int64: dish.ID, Valid: true},
		MerchantID:   pgtype.Int8{Int64: merchant.ID, Valid: true},
	})

	require.NoError(t, err)
	require.NotZero(t, behavior.ID)
	require.Equal(t, "purchase", behavior.BehaviorType)
	require.True(t, behavior.DishID.Valid)
	require.True(t, behavior.MerchantID.Valid)
}

func TestTrackBehavior_MultipleBehaviors(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	// 同一用户对同一菜品的多次行为
	behavior1, err := testStore.TrackBehavior(context.Background(), TrackBehaviorParams{
		UserID:       user.ID,
		BehaviorType: "view",
		DishID:       pgtype.Int8{Int64: dish.ID, Valid: true},
	})
	require.NoError(t, err)

	behavior2, err := testStore.TrackBehavior(context.Background(), TrackBehaviorParams{
		UserID:       user.ID,
		BehaviorType: "detail",
		DishID:       pgtype.Int8{Int64: dish.ID, Valid: true},
	})
	require.NoError(t, err)

	require.NotEqual(t, behavior1.ID, behavior2.ID)
	require.Equal(t, "view", behavior1.BehaviorType)
	require.Equal(t, "detail", behavior2.BehaviorType)
}

// ==================== GetUserRecentBehaviors Tests ====================

func TestGetUserRecentBehaviors(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	// 创建多个行为
	for i := 0; i < 5; i++ {
		_, err := testStore.TrackBehavior(context.Background(), TrackBehaviorParams{
			UserID:       user.ID,
			BehaviorType: "view",
			DishID:       pgtype.Int8{Int64: dish.ID, Valid: true},
		})
		require.NoError(t, err)
	}

	// 查询最近行为
	since := time.Now().Add(-1 * time.Hour)
	behaviors, err := testStore.GetUserRecentBehaviors(context.Background(), GetUserRecentBehaviorsParams{
		UserID:    user.ID,
		CreatedAt: since,
		Limit:     10,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(behaviors), 5)

	// 验证按创建时间倒序
	for i := 1; i < len(behaviors); i++ {
		require.True(t, behaviors[i-1].CreatedAt.After(behaviors[i].CreatedAt) ||
			behaviors[i-1].CreatedAt.Equal(behaviors[i].CreatedAt))
	}
}

func TestGetUserRecentBehaviors_Empty(t *testing.T) {
	user := createRandomUser(t)

	since := time.Now().Add(-1 * time.Hour)
	behaviors, err := testStore.GetUserRecentBehaviors(context.Background(), GetUserRecentBehaviorsParams{
		UserID:    user.ID,
		CreatedAt: since,
		Limit:     10,
	})
	require.NoError(t, err)
	require.Empty(t, behaviors)
}

func TestGetUserRecentBehaviors_Limit(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	// 创建10个行为
	for i := 0; i < 10; i++ {
		_, err := testStore.TrackBehavior(context.Background(), TrackBehaviorParams{
			UserID:       user.ID,
			BehaviorType: "view",
			DishID:       pgtype.Int8{Int64: dish.ID, Valid: true},
		})
		require.NoError(t, err)
	}

	// 只查询5个
	since := time.Now().Add(-1 * time.Hour)
	behaviors, err := testStore.GetUserRecentBehaviors(context.Background(), GetUserRecentBehaviorsParams{
		UserID:    user.ID,
		CreatedAt: since,
		Limit:     5,
	})
	require.NoError(t, err)
	require.Len(t, behaviors, 5)
}

// ==================== GetUserBehaviorsByType Tests ====================

func TestGetUserBehaviorsByType(t *testing.T) {
	user := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	// 创建不同类型的行为
	types := []string{"view", "view", "detail", "cart", "purchase"}
	for _, bt := range types {
		_, err := testStore.TrackBehavior(context.Background(), TrackBehaviorParams{
			UserID:       user.ID,
			BehaviorType: bt,
			DishID:       pgtype.Int8{Int64: dish.ID, Valid: true},
		})
		require.NoError(t, err)
	}

	// 只查询 view 类型
	since := time.Now().Add(-1 * time.Hour)
	behaviors, err := testStore.GetUserBehaviorsByType(context.Background(), GetUserBehaviorsByTypeParams{
		UserID:       user.ID,
		BehaviorType: "view",
		CreatedAt:    since,
		Limit:        10,
	})
	require.NoError(t, err)
	require.Len(t, behaviors, 2)

	for _, b := range behaviors {
		require.Equal(t, "view", b.BehaviorType)
	}
}

// ==================== UserPreferences Tests ====================

func TestUpsertUserPreferences_Create(t *testing.T) {
	user := createRandomUser(t)

	cuisinePrefs, _ := json.Marshal(map[string]float64{"川菜": 0.8, "粤菜": 0.6})
	topCuisines, _ := json.Marshal(map[string]int{"川菜": 15})

	arg := UpsertUserPreferencesParams{
		UserID:             user.ID,
		CuisinePreferences: cuisinePrefs,
		PriceRangeMin:      pgtype.Int8{Int64: 2000, Valid: true},  // 20元
		PriceRangeMax:      pgtype.Int8{Int64: 10000, Valid: true}, // 100元
		AvgOrderAmount:     pgtype.Int8{Int64: 5000, Valid: true},  // 50元
		FavoriteTimeSlots:  []int32{11, 12, 18, 19},
		PurchaseFrequency:  5,
		LastOrderDate:      pgtype.Date{Time: time.Now(), Valid: true},
		TopCuisines:        topCuisines,
	}

	pref, err := testStore.UpsertUserPreferences(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, pref.ID)
	require.Equal(t, user.ID, pref.UserID)
	require.Equal(t, int64(2000), pref.PriceRangeMin.Int64)
	require.Equal(t, int64(10000), pref.PriceRangeMax.Int64)
	require.Equal(t, int16(5), pref.PurchaseFrequency)
}

func TestUpsertUserPreferences_Update(t *testing.T) {
	user := createRandomUser(t)

	cuisinePrefs1, _ := json.Marshal(map[string]float64{"川菜": 0.8})

	// 初始创建
	arg1 := UpsertUserPreferencesParams{
		UserID:             user.ID,
		CuisinePreferences: cuisinePrefs1,
		PurchaseFrequency:  1,
	}
	pref1, err := testStore.UpsertUserPreferences(context.Background(), arg1)
	require.NoError(t, err)

	cuisinePrefs2, _ := json.Marshal(map[string]float64{"川菜": 0.9, "湘菜": 0.7, "粤菜": 0.5})

	// 更新
	arg2 := UpsertUserPreferencesParams{
		UserID:             user.ID,
		CuisinePreferences: cuisinePrefs2,
		PurchaseFrequency:  10,
		PriceRangeMin:      pgtype.Int8{Int64: 3000, Valid: true},
	}
	pref2, err := testStore.UpsertUserPreferences(context.Background(), arg2)
	require.NoError(t, err)

	require.Equal(t, pref1.ID, pref2.ID) // 同一记录
	require.Equal(t, int16(10), pref2.PurchaseFrequency)
	require.Equal(t, int64(3000), pref2.PriceRangeMin.Int64)
}

func TestGetUserPreferences(t *testing.T) {
	user := createRandomUser(t)

	cuisinePrefs, _ := json.Marshal(map[string]float64{"日料": 0.9, "韩餐": 0.7})

	// 创建偏好
	arg := UpsertUserPreferencesParams{
		UserID:             user.ID,
		CuisinePreferences: cuisinePrefs,
		PurchaseFrequency:  8,
	}
	_, err := testStore.UpsertUserPreferences(context.Background(), arg)
	require.NoError(t, err)

	// 查询
	pref, err := testStore.GetUserPreferences(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, user.ID, pref.UserID)
	require.Equal(t, int16(8), pref.PurchaseFrequency)
}

func TestGetUserPreferences_NotFound(t *testing.T) {
	_, err := testStore.GetUserPreferences(context.Background(), 99999999)
	require.Error(t, err)
}

// ==================== SaveRecommendations Tests ====================

func TestSaveRecommendations(t *testing.T) {
	user := createRandomUser(t)
	expiredAt := time.Now().Add(5 * time.Minute)

	rec, err := testStore.SaveRecommendations(context.Background(), SaveRecommendationsParams{
		UserID:      user.ID,
		DishIds:     []int64{1, 2, 3, 4, 5},
		ComboIds:    []int64{10, 11},
		MerchantIds: []int64{100, 101, 102},
		Algorithm:   "ee-algorithm",
		Score:       pgtype.Numeric{},
		ExpiredAt:   expiredAt,
	})

	require.NoError(t, err)
	require.NotZero(t, rec.ID)
	require.Equal(t, user.ID, rec.UserID)
	require.Equal(t, []int64{1, 2, 3, 4, 5}, rec.DishIds)
	require.Equal(t, []int64{10, 11}, rec.ComboIds)
	require.Equal(t, []int64{100, 101, 102}, rec.MerchantIds)
	require.Equal(t, "ee-algorithm", rec.Algorithm)
}

func TestGetLatestRecommendations(t *testing.T) {
	user := createRandomUser(t)
	expiredAt := time.Now().Add(5 * time.Minute)

	// 保存推荐
	_, err := testStore.SaveRecommendations(context.Background(), SaveRecommendationsParams{
		UserID:    user.ID,
		DishIds:   []int64{1, 2, 3},
		Algorithm: "test-algorithm",
		ExpiredAt: expiredAt,
	})
	require.NoError(t, err)

	// 获取最新推荐
	rec, err := testStore.GetLatestRecommendations(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, user.ID, rec.UserID)
	require.Equal(t, []int64{1, 2, 3}, rec.DishIds)
	require.Equal(t, "test-algorithm", rec.Algorithm)
}

func TestGetLatestRecommendations_Expired(t *testing.T) {
	user := createRandomUser(t)

	// 保存已过期的推荐
	expiredAt := time.Now().Add(-1 * time.Minute)
	_, err := testStore.SaveRecommendations(context.Background(), SaveRecommendationsParams{
		UserID:    user.ID,
		DishIds:   []int64{1, 2, 3},
		Algorithm: "expired-algorithm",
		ExpiredAt: expiredAt,
	})
	require.NoError(t, err)

	// 获取应该失败（已过期）
	_, err = testStore.GetLatestRecommendations(context.Background(), user.ID)
	require.Error(t, err) // no rows
}

func TestGetLatestRecommendations_MultipleVersions(t *testing.T) {
	user := createRandomUser(t)

	// 保存第一个推荐
	expiredAt1 := time.Now().Add(5 * time.Minute)
	_, err := testStore.SaveRecommendations(context.Background(), SaveRecommendationsParams{
		UserID:    user.ID,
		DishIds:   []int64{1, 2, 3},
		Algorithm: "first",
		ExpiredAt: expiredAt1,
	})
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond) // 确保时间戳不同

	// 保存第二个推荐
	expiredAt2 := time.Now().Add(10 * time.Minute)
	_, err = testStore.SaveRecommendations(context.Background(), SaveRecommendationsParams{
		UserID:    user.ID,
		DishIds:   []int64{4, 5, 6},
		Algorithm: "second",
		ExpiredAt: expiredAt2,
	})
	require.NoError(t, err)

	// 获取最新推荐应该是第二个
	rec, err := testStore.GetLatestRecommendations(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, "second", rec.Algorithm)
	require.Equal(t, []int64{4, 5, 6}, rec.DishIds)
}

// ==================== RecommendationConfig Tests ====================

func TestUpsertRecommendationConfig(t *testing.T) {
	// 创建真实的区域
	region := createRandomRegion(t)

	config, err := testStore.UpsertRecommendationConfig(context.Background(), UpsertRecommendationConfigParams{
		RegionID:          region.ID,
		ExploitationRatio: numericFromFloat64(0.60), // 60%
		ExplorationRatio:  numericFromFloat64(0.30), // 30%
		RandomRatio:       numericFromFloat64(0.10), // 10%
		AutoAdjust:        true,
	})
	require.NoError(t, err)
	require.NotZero(t, config.ID)
	require.Equal(t, region.ID, config.RegionID)
}

func TestUpsertRecommendationConfig_Update(t *testing.T) {
	region := createRandomRegion(t)

	// 初始创建
	config1, err := testStore.UpsertRecommendationConfig(context.Background(), UpsertRecommendationConfigParams{
		RegionID:          region.ID,
		ExploitationRatio: numericFromFloat64(0.60),
		ExplorationRatio:  numericFromFloat64(0.30),
		RandomRatio:       numericFromFloat64(0.10),
		AutoAdjust:        false,
	})
	require.NoError(t, err)

	// 更新
	config2, err := testStore.UpsertRecommendationConfig(context.Background(), UpsertRecommendationConfigParams{
		RegionID:          region.ID,
		ExploitationRatio: numericFromFloat64(0.50), // 修改
		ExplorationRatio:  numericFromFloat64(0.40), // 修改
		RandomRatio:       numericFromFloat64(0.10),
		AutoAdjust:        true, // 修改
	})
	require.NoError(t, err)

	require.Equal(t, config1.ID, config2.ID) // 同一记录
	require.True(t, config2.AutoAdjust)
}

func TestGetRecommendationConfig(t *testing.T) {
	region := createRandomRegion(t)

	// 创建配置
	_, err := testStore.UpsertRecommendationConfig(context.Background(), UpsertRecommendationConfigParams{
		RegionID:          region.ID,
		ExploitationRatio: numericFromFloat64(0.70),
		ExplorationRatio:  numericFromFloat64(0.20),
		RandomRatio:       numericFromFloat64(0.10),
	})
	require.NoError(t, err)

	// 查询
	config, err := testStore.GetRecommendationConfig(context.Background(), region.ID)
	require.NoError(t, err)
	require.Equal(t, region.ID, config.RegionID)
}

func TestGetRecommendationConfig_NotFound(t *testing.T) {
	_, err := testStore.GetRecommendationConfig(context.Background(), 99999999)
	require.Error(t, err)
}

func TestGetAllRecommendationConfigs(t *testing.T) {
	// 创建几个配置
	region1 := createRandomRegion(t)
	region2 := createRandomRegion(t)

	for _, rid := range []int64{region1.ID, region2.ID} {
		_, err := testStore.UpsertRecommendationConfig(context.Background(), UpsertRecommendationConfigParams{
			RegionID:          rid,
			ExploitationRatio: numericFromFloat64(0.60),
			ExplorationRatio:  numericFromFloat64(0.30),
			RandomRatio:       numericFromFloat64(0.10),
		})
		require.NoError(t, err)
	}

	configs, err := testStore.GetAllRecommendationConfigs(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(configs), 2)
}
