package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

// createClaimForMerchant 为指定商户创建索赔记录
func createClaimForMerchant(t *testing.T, userID, merchantID int64, claimType string) Claim {
	// 创建订单
	order := createCompletedOrderForStats(t, userID, merchantID, 5000, "takeout", time.Now())

	arg := CreateClaimParams{
		OrderID:     order.ID,
		UserID:      userID,
		ClaimType:   claimType,
		ClaimAmount: 2000,
		Description: "测试索赔",
		Status:      "approved",
		CreatedAt:   time.Now(), // 必须设置创建时间，否则会使用零值（1970年）导致查询不到
	}

	claim, err := testStore.CreateClaim(context.Background(), arg)
	require.NoError(t, err)

	return claim
}

// ==================== CountMerchantClaimsByType Tests ====================

func TestCountMerchantClaimsByType(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	windowStart := time.Now().Add(-7 * 24 * time.Hour)

	// 创建3个异物索赔
	for i := 0; i < 3; i++ {
		createClaimForMerchant(t, user.ID, merchant.ID, "foreign-object")
	}

	// 创建2个其他类型索赔
	createClaimForMerchant(t, user.ID, merchant.ID, "damage")
	createClaimForMerchant(t, user.ID, merchant.ID, "quality")

	count, err := testStore.CountMerchantClaimsByType(context.Background(), CountMerchantClaimsByTypeParams{
		MerchantID: merchant.ID,
		ClaimType:  "foreign-object",
		CreatedAt:  windowStart,
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), count) // 只有3个异物索赔
}

func TestCountMerchantClaimsByType_NoMatches(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	windowStart := time.Now().Add(-7 * 24 * time.Hour)

	// 只创建非异物索赔
	createClaimForMerchant(t, user.ID, merchant.ID, "damage")
	createClaimForMerchant(t, user.ID, merchant.ID, "quality")

	count, err := testStore.CountMerchantClaimsByType(context.Background(), CountMerchantClaimsByTypeParams{
		MerchantID: merchant.ID,
		ClaimType:  "foreign-object",
		CreatedAt:  windowStart,
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

func TestCountMerchantClaimsByType_TimeWindow(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	// 创建3个异物索赔
	for i := 0; i < 3; i++ {
		createClaimForMerchant(t, user.ID, merchant.ID, "foreign-object")
	}

	// 使用未来时间作为窗口起点，应该查不到
	futureStart := time.Now().Add(1 * time.Hour)

	count, err := testStore.CountMerchantClaimsByType(context.Background(), CountMerchantClaimsByTypeParams{
		MerchantID: merchant.ID,
		ClaimType:  "foreign-object",
		CreatedAt:  futureStart,
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), count) // 时间窗口外，查不到
}

// ==================== ListMerchantClaimsByTypeInPeriod Tests ====================

func TestListMerchantClaimsByTypeInPeriod(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	windowStart := time.Now().Add(-7 * 24 * time.Hour)

	// 创建3个异物索赔
	for i := 0; i < 3; i++ {
		createClaimForMerchant(t, user.ID, merchant.ID, "foreign-object")
	}

	// 创建2个其他类型索赔
	createClaimForMerchant(t, user.ID, merchant.ID, "damage")

	claims, err := testStore.ListMerchantClaimsByTypeInPeriod(context.Background(), ListMerchantClaimsByTypeInPeriodParams{
		MerchantID: merchant.ID,
		ClaimType:  "foreign-object",
		CreatedAt:  windowStart,
	})
	require.NoError(t, err)
	require.Len(t, claims, 3)

	// 验证返回的都是异物索赔
	for _, claim := range claims {
		require.Equal(t, "foreign-object", claim.ClaimType)
	}
}

func TestListMerchantClaimsByTypeInPeriod_OrderByCreatedAt(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	windowStart := time.Now().Add(-7 * 24 * time.Hour)

	// 创建多个索赔
	createClaimForMerchant(t, user.ID, merchant.ID, "foreign-object")
	time.Sleep(10 * time.Millisecond) // 确保时间戳不同
	createClaimForMerchant(t, user.ID, merchant.ID, "foreign-object")
	time.Sleep(10 * time.Millisecond)
	createClaimForMerchant(t, user.ID, merchant.ID, "foreign-object")

	claims, err := testStore.ListMerchantClaimsByTypeInPeriod(context.Background(), ListMerchantClaimsByTypeInPeriodParams{
		MerchantID: merchant.ID,
		ClaimType:  "foreign-object",
		CreatedAt:  windowStart,
	})
	require.NoError(t, err)
	require.Len(t, claims, 3)

	// 验证按创建时间降序排列（最新的在前）
	for i := 0; i < len(claims)-1; i++ {
		require.True(t, claims[i].CreatedAt.After(claims[i+1].CreatedAt) ||
			claims[i].CreatedAt.Equal(claims[i+1].CreatedAt))
	}
}

func TestListMerchantClaimsByTypeInPeriod_Empty(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	windowStart := time.Now().Add(-7 * 24 * time.Hour)

	claims, err := testStore.ListMerchantClaimsByTypeInPeriod(context.Background(), ListMerchantClaimsByTypeInPeriodParams{
		MerchantID: merchant.ID,
		ClaimType:  "foreign-object",
		CreatedAt:  windowStart,
	})
	require.NoError(t, err)
	require.Empty(t, claims)
}

// ==================== 综合场景测试 ====================

func TestMerchantForeignObjectTracking_WarningThreshold(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	windowStart := time.Now().Add(-7 * 24 * time.Hour)

	// 模拟达到警告阈值：7天内3单异物索赔
	for i := 0; i < 3; i++ {
		createClaimForMerchant(t, user.ID, merchant.ID, "foreign-object")
	}

	count, err := testStore.CountMerchantClaimsByType(context.Background(), CountMerchantClaimsByTypeParams{
		MerchantID: merchant.ID,
		ClaimType:  "foreign-object",
		CreatedAt:  windowStart,
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), count)

	// 达到警告阈值
	require.GreaterOrEqual(t, count, int64(3))
}
