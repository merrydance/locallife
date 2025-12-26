package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

// recordBrowseHistoryHelper 记录一条浏览历史
func recordBrowseHistoryHelper(t *testing.T, userID int64, targetType string, targetID int64) BrowseHistory {
	history, err := testStore.RecordBrowseHistory(context.Background(), RecordBrowseHistoryParams{
		UserID:     userID,
		TargetType: targetType,
		TargetID:   targetID,
	})
	require.NoError(t, err)
	require.NotZero(t, history.ID)

	return history
}

// ==================== RecordBrowseHistory Tests ====================

func TestRecordBrowseHistory(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	history := recordBrowseHistoryHelper(t, user.ID, "merchant", merchant.ID)

	require.Equal(t, user.ID, history.UserID)
	require.Equal(t, "merchant", history.TargetType)
	require.Equal(t, merchant.ID, history.TargetID)
	require.Equal(t, int32(1), history.ViewCount)
	require.NotZero(t, history.LastViewedAt)
}

func TestRecordBrowseHistory_DishTarget(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	user := createRandomUser(t)

	history := recordBrowseHistoryHelper(t, user.ID, "dish", dish.ID)

	require.Equal(t, "dish", history.TargetType)
	require.Equal(t, dish.ID, history.TargetID)
	require.Equal(t, int32(1), history.ViewCount)
}

func TestRecordBrowseHistory_IncrementViewCount(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	// 第一次浏览
	history1 := recordBrowseHistoryHelper(t, user.ID, "merchant", merchant.ID)
	require.Equal(t, int32(1), history1.ViewCount)

	// 第二次浏览（同一目标）
	history2 := recordBrowseHistoryHelper(t, user.ID, "merchant", merchant.ID)
	require.Equal(t, int32(2), history2.ViewCount)
	require.Equal(t, history1.ID, history2.ID) // 应该是同一条记录

	// 第三次浏览
	history3 := recordBrowseHistoryHelper(t, user.ID, "merchant", merchant.ID)
	require.Equal(t, int32(3), history3.ViewCount)
}

func TestRecordBrowseHistory_DifferentTargets(t *testing.T) {
	owner1 := createRandomUser(t)
	owner2 := createRandomUser(t)
	merchant1 := createRandomMerchantWithOwner(t, owner1.ID)
	merchant2 := createRandomMerchantWithOwner(t, owner2.ID)
	user := createRandomUser(t)

	// 浏览不同商户
	history1 := recordBrowseHistoryHelper(t, user.ID, "merchant", merchant1.ID)
	history2 := recordBrowseHistoryHelper(t, user.ID, "merchant", merchant2.ID)

	require.NotEqual(t, history1.ID, history2.ID)
	require.Equal(t, int32(1), history1.ViewCount)
	require.Equal(t, int32(1), history2.ViewCount)
}

// ==================== ListBrowseHistory Tests ====================

func TestListBrowseHistory(t *testing.T) {
	owner1 := createRandomUser(t)
	owner2 := createRandomUser(t)
	merchant1 := createRandomMerchantWithOwner(t, owner1.ID)
	merchant2 := createRandomMerchantWithOwner(t, owner2.ID)
	user := createRandomUser(t)

	_ = recordBrowseHistoryHelper(t, user.ID, "merchant", merchant1.ID)
	_ = recordBrowseHistoryHelper(t, user.ID, "merchant", merchant2.ID)

	histories, err := testStore.ListBrowseHistory(context.Background(), ListBrowseHistoryParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(histories), 2)

	// 按最后浏览时间倒序排列
	for i := 1; i < len(histories); i++ {
		require.True(t, histories[i-1].LastViewedAt.After(histories[i].LastViewedAt) ||
			histories[i-1].LastViewedAt.Equal(histories[i].LastViewedAt))
	}
}

func TestListBrowseHistory_Pagination(t *testing.T) {
	user := createRandomUser(t)

	// 创建多个浏览记录
	for i := 0; i < 5; i++ {
		owner := createRandomUser(t)
		merchant := createRandomMerchantWithOwner(t, owner.ID)
		_ = recordBrowseHistoryHelper(t, user.ID, "merchant", merchant.ID)
	}

	// 第一页
	page1, err := testStore.ListBrowseHistory(context.Background(), ListBrowseHistoryParams{
		UserID: user.ID,
		Limit:  2,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, page1, 2)

	// 第二页
	page2, err := testStore.ListBrowseHistory(context.Background(), ListBrowseHistoryParams{
		UserID: user.ID,
		Limit:  2,
		Offset: 2,
	})
	require.NoError(t, err)
	require.Len(t, page2, 2)

	// 不应重复
	require.NotEqual(t, page1[0].ID, page2[0].ID)
}

func TestListBrowseHistory_Empty(t *testing.T) {
	user := createRandomUser(t)

	histories, err := testStore.ListBrowseHistory(context.Background(), ListBrowseHistoryParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Empty(t, histories)
}

// ==================== ListBrowseHistoryByType Tests ====================

func TestListBrowseHistoryByType(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)
	user := createRandomUser(t)

	// 浏览商户和菜品
	_ = recordBrowseHistoryHelper(t, user.ID, "merchant", merchant.ID)
	_ = recordBrowseHistoryHelper(t, user.ID, "dish", dish1.ID)
	_ = recordBrowseHistoryHelper(t, user.ID, "dish", dish2.ID)

	// 只查询菜品浏览历史
	dishHistories, err := testStore.ListBrowseHistoryByType(context.Background(), ListBrowseHistoryByTypeParams{
		UserID:     user.ID,
		TargetType: "dish",
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, dishHistories, 2)

	for _, h := range dishHistories {
		require.Equal(t, "dish", h.TargetType)
	}

	// 只查询商户浏览历史
	merchantHistories, err := testStore.ListBrowseHistoryByType(context.Background(), ListBrowseHistoryByTypeParams{
		UserID:     user.ID,
		TargetType: "merchant",
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, merchantHistories, 1)
	require.Equal(t, "merchant", merchantHistories[0].TargetType)
}

// ==================== CountBrowseHistory Tests ====================

func TestCountBrowseHistory(t *testing.T) {
	user := createRandomUser(t)

	// 初始计数
	count, err := testStore.CountBrowseHistory(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)

	// 添加浏览记录
	for i := 0; i < 3; i++ {
		owner := createRandomUser(t)
		merchant := createRandomMerchantWithOwner(t, owner.ID)
		_ = recordBrowseHistoryHelper(t, user.ID, "merchant", merchant.ID)
	}

	count, err = testStore.CountBrowseHistory(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

// ==================== CountBrowseHistoryByType Tests ====================

func TestCountBrowseHistoryByType(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)
	user := createRandomUser(t)

	_ = recordBrowseHistoryHelper(t, user.ID, "merchant", merchant.ID)
	_ = recordBrowseHistoryHelper(t, user.ID, "dish", dish1.ID)
	_ = recordBrowseHistoryHelper(t, user.ID, "dish", dish2.ID)

	// 商户计数
	merchantCount, err := testStore.CountBrowseHistoryByType(context.Background(), CountBrowseHistoryByTypeParams{
		UserID:     user.ID,
		TargetType: "merchant",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), merchantCount)

	// 菜品计数
	dishCount, err := testStore.CountBrowseHistoryByType(context.Background(), CountBrowseHistoryByTypeParams{
		UserID:     user.ID,
		TargetType: "dish",
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), dishCount)
}

// ==================== DeleteBrowseHistory Tests ====================

func TestDeleteBrowseHistory(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	_ = recordBrowseHistoryHelper(t, user.ID, "merchant", merchant.ID)

	// 验证存在
	count, err := testStore.CountBrowseHistory(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	// 删除
	err = testStore.DeleteBrowseHistory(context.Background(), DeleteBrowseHistoryParams{
		UserID:     user.ID,
		TargetType: "merchant",
		TargetID:   merchant.ID,
	})
	require.NoError(t, err)

	// 验证已删除
	count, err = testStore.CountBrowseHistory(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

func TestDeleteBrowseHistory_NotExist(t *testing.T) {
	user := createRandomUser(t)

	// 删除不存在的记录（不应报错）
	err := testStore.DeleteBrowseHistory(context.Background(), DeleteBrowseHistoryParams{
		UserID:     user.ID,
		TargetType: "merchant",
		TargetID:   99999999,
	})
	require.NoError(t, err)
}

// ==================== ClearBrowseHistory Tests ====================

func TestClearBrowseHistory(t *testing.T) {
	user := createRandomUser(t)

	// 添加多个浏览记录
	for i := 0; i < 5; i++ {
		owner := createRandomUser(t)
		merchant := createRandomMerchantWithOwner(t, owner.ID)
		_ = recordBrowseHistoryHelper(t, user.ID, "merchant", merchant.ID)
	}

	// 验证有记录
	count, err := testStore.CountBrowseHistory(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(5), count)

	// 清空
	err = testStore.ClearBrowseHistory(context.Background(), user.ID)
	require.NoError(t, err)

	// 验证已清空
	count, err = testStore.CountBrowseHistory(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

func TestClearBrowseHistory_DoesNotAffectOtherUsers(t *testing.T) {
	user1 := createRandomUser(t)
	user2 := createRandomUser(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 两个用户都有浏览记录
	_ = recordBrowseHistoryHelper(t, user1.ID, "merchant", merchant.ID)
	_ = recordBrowseHistoryHelper(t, user2.ID, "merchant", merchant.ID)

	// 只清空 user1
	err := testStore.ClearBrowseHistory(context.Background(), user1.ID)
	require.NoError(t, err)

	// user1 无记录
	count1, err := testStore.CountBrowseHistory(context.Background(), user1.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), count1)

	// user2 仍有记录
	count2, err := testStore.CountBrowseHistory(context.Background(), user2.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count2)
}

// ==================== Edge Cases Tests ====================

func TestBrowseHistory_MultipleViewsSameTarget(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	// 多次浏览同一目标
	for i := 0; i < 10; i++ {
		_ = recordBrowseHistoryHelper(t, user.ID, "merchant", merchant.ID)
	}

	// 只有一条记录，但浏览次数为10
	histories, err := testStore.ListBrowseHistory(context.Background(), ListBrowseHistoryParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, histories, 1)
	require.Equal(t, int32(10), histories[0].ViewCount)
}

func TestBrowseHistory_DifferentTypes(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	user := createRandomUser(t)

	// 浏览商户
	_ = recordBrowseHistoryHelper(t, user.ID, "merchant", merchant.ID)

	// 浏览菜品
	_ = recordBrowseHistoryHelper(t, user.ID, "dish", dish.ID)

	count, err := testStore.CountBrowseHistory(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}
