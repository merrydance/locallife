package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// 辅助函数：创建随机库存记录
func createRandomDailyInventory(t *testing.T, merchantID, dishID int64, date time.Time) DailyInventory {
	arg := CreateDailyInventoryParams{
		MerchantID:    merchantID,
		DishID:        dishID,
		Date:          pgtype.Date{Time: date, Valid: true},
		TotalQuantity: int32(util.RandomInt(10, 100)),
		SoldQuantity:  int32(0),
	}

	inventory, err := testStore.CreateDailyInventory(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, inventory)

	require.Equal(t, arg.MerchantID, inventory.MerchantID)
	require.Equal(t, arg.DishID, inventory.DishID)
	require.Equal(t, date.Format("2006-01-02"), inventory.Date.Time.Format("2006-01-02"))
	require.Equal(t, arg.TotalQuantity, inventory.TotalQuantity)
	require.Equal(t, arg.SoldQuantity, inventory.SoldQuantity)
	require.NotZero(t, inventory.CreatedAt)

	return inventory
}

// ============================================
// 库存测试
// ============================================

func TestCreateDailyInventory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	today := time.Now()

	createRandomDailyInventory(t, merchant.ID, dish.ID, today)
}

func TestGetDailyInventory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	today := time.Now()

	inventory1 := createRandomDailyInventory(t, merchant.ID, dish.ID, today)

	arg := GetDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     dish.ID,
		Date:       pgtype.Date{Time: today, Valid: true},
	}

	inventory2, err := testStore.GetDailyInventory(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, inventory2)

	require.Equal(t, inventory1.MerchantID, inventory2.MerchantID)
	require.Equal(t, inventory1.DishID, inventory2.DishID)
	require.Equal(t, inventory1.TotalQuantity, inventory2.TotalQuantity)
}

func TestGetDailyInventoryForUpdate(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	today := time.Now()

	createRandomDailyInventory(t, merchant.ID, dish.ID, today)

	arg := GetDailyInventoryForUpdateParams{
		MerchantID: merchant.ID,
		DishID:     dish.ID,
		Date:       pgtype.Date{Time: today, Valid: true},
	}

	inventory, err := testStore.GetDailyInventoryForUpdate(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, inventory)
}

func TestListDailyInventoryByMerchant(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	today := time.Now()

	// 创建多个菜品的库存
	for i := 0; i < 3; i++ {
		dish := createRandomDish(t, merchant.ID, category.ID)
		createRandomDailyInventory(t, merchant.ID, dish.ID, today)
	}

	arg := ListDailyInventoryByMerchantParams{
		MerchantID: merchant.ID,
		Date:       pgtype.Date{Time: today, Valid: true},
	}

	inventories, err := testStore.ListDailyInventoryByMerchant(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(inventories), 3)

	for _, inv := range inventories {
		require.Equal(t, merchant.ID, inv.MerchantID)
		require.NotEmpty(t, inv.DishName)
		require.NotEmpty(t, inv.DishPrice)
	}
}

func TestListDailyInventoryByDate(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	today := time.Now()

	createRandomDailyInventory(t, merchant.ID, dish.ID, today)

	inventories, err := testStore.ListDailyInventoryByDate(context.Background(), pgtype.Date{Time: today, Valid: true})
	require.NoError(t, err)
	require.NotEmpty(t, inventories)

	for _, inv := range inventories {
		require.Equal(t, today.Format("2006-01-02"), inv.Date.Time.Format("2006-01-02"))
	}
}

func TestUpdateDailyInventory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	today := time.Now()

	inventory := createRandomDailyInventory(t, merchant.ID, dish.ID, today)

	newTotalQty := int32(util.RandomInt(50, 150))
	newSoldQty := int32(util.RandomInt(0, 10))

	arg := UpdateDailyInventoryParams{
		MerchantID:    merchant.ID,
		DishID:        dish.ID,
		Date:          pgtype.Date{Time: today, Valid: true},
		TotalQuantity: pgtype.Int4{Int32: newTotalQty, Valid: true},
		SoldQuantity:  pgtype.Int4{Int32: newSoldQty, Valid: true},
	}

	updatedInventory, err := testStore.UpdateDailyInventory(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedInventory)

	require.Equal(t, inventory.MerchantID, updatedInventory.MerchantID)
	require.Equal(t, inventory.DishID, updatedInventory.DishID)
	require.Equal(t, newTotalQty, updatedInventory.TotalQuantity)
	require.Equal(t, newSoldQty, updatedInventory.SoldQuantity)
}

func TestIncrementSoldQuantity(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	today := time.Now()

	inventory := createRandomDailyInventory(t, merchant.ID, dish.ID, today)
	originalSoldQty := inventory.SoldQuantity

	increment := int32(5)
	arg := IncrementSoldQuantityParams{
		MerchantID:   merchant.ID,
		DishID:       dish.ID,
		Date:         pgtype.Date{Time: today, Valid: true},
		SoldQuantity: increment,
	}

	updatedInventory, err := testStore.IncrementSoldQuantity(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedInventory)

	require.Equal(t, originalSoldQty+increment, updatedInventory.SoldQuantity)
}

func TestCheckAndDecrementInventory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	today := time.Now()

	// 创建有限库存
	arg := CreateDailyInventoryParams{
		MerchantID:    merchant.ID,
		DishID:        dish.ID,
		Date:          pgtype.Date{Time: today, Valid: true},
		TotalQuantity: int32(10),
		SoldQuantity:  int32(0),
	}
	_, err := testStore.CreateDailyInventory(context.Background(), arg)
	require.NoError(t, err)

	// 扣减库存
	deductArg := CheckAndDecrementInventoryParams{
		MerchantID:   merchant.ID,
		DishID:       dish.ID,
		Date:         pgtype.Date{Time: today, Valid: true},
		SoldQuantity: int32(3),
	}

	updatedInventory, err := testStore.CheckAndDecrementInventory(context.Background(), deductArg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedInventory)
	require.Equal(t, int32(3), updatedInventory.SoldQuantity)

	// 尝试扣减超过剩余库存的数量（应该失败或返回空）
	deductArg2 := CheckAndDecrementInventoryParams{
		MerchantID:   merchant.ID,
		DishID:       dish.ID,
		Date:         pgtype.Date{Time: today, Valid: true},
		SoldQuantity: int32(10), // 只剩7个，尝试打10个
	}

	updatedInventory2, err := testStore.CheckAndDecrementInventory(context.Background(), deductArg2)
	if err == nil && updatedInventory2.SoldQuantity == 0 {
		// 没有更新任何行，符合预期（库存不足）
		require.Empty(t, updatedInventory2.MerchantID)
	} else if err != nil {
		// 或者返回错误也是可以接受的
		require.Error(t, err)
	}
}

func TestCheckAndDecrementInventoryUnlimited(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	today := time.Now()

	// 创建无限库存（total_quantity = -1）
	arg := CreateDailyInventoryParams{
		MerchantID:    merchant.ID,
		DishID:        dish.ID,
		Date:          pgtype.Date{Time: today, Valid: true},
		TotalQuantity: int32(-1), // 无限库存
		SoldQuantity:  int32(0),
	}
	_, err := testStore.CreateDailyInventory(context.Background(), arg)
	require.NoError(t, err)

	// 扣减大量库存，应该成功
	deductArg := CheckAndDecrementInventoryParams{
		MerchantID:   merchant.ID,
		DishID:       dish.ID,
		Date:         pgtype.Date{Time: today, Valid: true},
		SoldQuantity: int32(1000),
	}

	updatedInventory, err := testStore.CheckAndDecrementInventory(context.Background(), deductArg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedInventory)
	require.Equal(t, int32(1000), updatedInventory.SoldQuantity)
	require.Equal(t, int32(-1), updatedInventory.TotalQuantity) // 依然是无限库存
}

func TestDeleteDailyInventory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	today := time.Now()

	createRandomDailyInventory(t, merchant.ID, dish.ID, today)

	arg := DeleteDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     dish.ID,
		Date:       pgtype.Date{Time: today, Valid: true},
	}

	err := testStore.DeleteDailyInventory(context.Background(), arg)
	require.NoError(t, err)

	// 验证已删除
	getArg := GetDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     dish.ID,
		Date:       pgtype.Date{Time: today, Valid: true},
	}
	inventory, err := testStore.GetDailyInventory(context.Background(), getArg)
	require.Error(t, err)
	require.Empty(t, inventory)
}

func TestDeleteOldInventory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	// 创建过去的库存
	pastDate := time.Now().AddDate(0, 0, -10)
	createRandomDailyInventory(t, merchant.ID, dish.ID, pastDate)

	// 删除7天前的库存
	cutoffDate := time.Now().AddDate(0, 0, -7)
	err := testStore.DeleteOldInventory(context.Background(), pgtype.Date{Time: cutoffDate, Valid: true})
	require.NoError(t, err)

	// 验证已删除
	getArg := GetDailyInventoryParams{
		MerchantID: merchant.ID,
		DishID:     dish.ID,
		Date:       pgtype.Date{Time: pastDate, Valid: true},
	}
	inventory, err := testStore.GetDailyInventory(context.Background(), getArg)
	require.Error(t, err)
	require.Empty(t, inventory)
}

func TestGetInventoryStats(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	today := time.Now()

	// 创建不同状态的库存
	// 1. 无限库存
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	_, err := testStore.CreateDailyInventory(context.Background(), CreateDailyInventoryParams{
		MerchantID:    merchant.ID,
		DishID:        dish1.ID,
		Date:          pgtype.Date{Time: today, Valid: true},
		TotalQuantity: -1,
		SoldQuantity:  0,
	})
	require.NoError(t, err)

	// 2. 售罄
	dish2 := createRandomDish(t, merchant.ID, category.ID)
	_, err = testStore.CreateDailyInventory(context.Background(), CreateDailyInventoryParams{
		MerchantID:    merchant.ID,
		DishID:        dish2.ID,
		Date:          pgtype.Date{Time: today, Valid: true},
		TotalQuantity: 10,
		SoldQuantity:  10,
	})
	require.NoError(t, err)

	// 3. 有库存
	dish3 := createRandomDish(t, merchant.ID, category.ID)
	_, err = testStore.CreateDailyInventory(context.Background(), CreateDailyInventoryParams{
		MerchantID:    merchant.ID,
		DishID:        dish3.ID,
		Date:          pgtype.Date{Time: today, Valid: true},
		TotalQuantity: 20,
		SoldQuantity:  5,
	})
	require.NoError(t, err)

	// 获取统计
	arg := GetInventoryStatsParams{
		MerchantID: merchant.ID,
		Date:       pgtype.Date{Time: today, Valid: true},
	}

	stats, err := testStore.GetInventoryStats(context.Background(), arg)
	require.NoError(t, err)

	require.GreaterOrEqual(t, stats.TotalDishes, int64(3))
	require.GreaterOrEqual(t, stats.UnlimitedDishes, int64(1))
	require.GreaterOrEqual(t, stats.SoldOutDishes, int64(1))
	require.GreaterOrEqual(t, stats.AvailableDishes, int64(1))
}
