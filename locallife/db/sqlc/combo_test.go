package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// 辅助函数：创建随机套餐
func createRandomComboSet(t *testing.T, merchantID int64) ComboSet {
	arg := CreateComboSetParams{
		MerchantID:        merchantID,
		Name:              util.RandomString(10),
		Description:       pgtype.Text{String: util.RandomString(30), Valid: true},
		ImageMediaAssetID: pgtype.Int8{},
		OriginalPrice:     util.RandomMoney(),
		ComboPrice:        util.RandomMoney(),
		IsOnline:          true,
	}

	combo, err := testStore.CreateComboSet(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, combo)

	require.Equal(t, arg.MerchantID, combo.MerchantID)
	require.Equal(t, arg.Name, combo.Name)
	require.Equal(t, arg.OriginalPrice, combo.OriginalPrice)
	require.Equal(t, arg.ComboPrice, combo.ComboPrice)
	require.True(t, combo.IsOnline)
	require.NotZero(t, combo.ID)
	require.NotZero(t, combo.CreatedAt)

	return combo
}

func newComboDishParams(comboID int64, dish Dish, quantity int16) AddComboDishParams {
	return AddComboDishParams{
		ComboID:               comboID,
		DishID:                dish.ID,
		Quantity:              quantity,
		DishBasePriceSnapshot: dish.Price,
	}
}

func comboIDsFromListOnline(rows []ListOnlineCombosByMerchantRow) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func comboIDsFromSearch(rows []SearchCombosGlobalRow) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func comboIDsFromMerchantOnline(rows []GetMerchantOnlineCombosRow) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

// ============================================
// 套餐测试
// ============================================

func TestCreateComboSet(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	createRandomComboSet(t, merchant.ID)
}

func TestGetComboSet(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	combo1 := createRandomComboSet(t, merchant.ID)

	combo2, err := testStore.GetComboSet(context.Background(), combo1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, combo2)

	require.Equal(t, combo1.ID, combo2.ID)
	require.Equal(t, combo1.Name, combo2.Name)
	require.Equal(t, combo1.MerchantID, combo2.MerchantID)
}

func TestListComboSetsByMerchant(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	tag := createRandomTag(t, "combo")

	// 创建多个套餐
	for i := 0; i < 3; i++ {
		combo := createRandomComboSet(t, merchant.ID)
		dish := createRandomDish(t, merchant.ID, category.ID)
		_, err := testStore.AddComboDish(context.Background(), newComboDishParams(combo.ID, dish, 2))
		require.NoError(t, err)
		_, err = testStore.AddComboTag(context.Background(), AddComboTagParams{
			ComboID: combo.ID,
			TagID:   tag.ID,
		})
		require.NoError(t, err)
	}

	arg := ListComboSetsByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      10,
		Offset:     0,
	}

	combos, err := testStore.ListComboSetsByMerchant(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(combos), 3)

	for _, combo := range combos {
		require.NotZero(t, combo.ID)
		require.NotEmpty(t, combo.Name)
		require.GreaterOrEqual(t, combo.DishCount, int64(1))
		require.GreaterOrEqual(t, combo.DishTotalQuantity, int64(2))
		require.NotNil(t, combo.Tags)
	}
}

func TestListComboSetsByMerchantExcludesDeletedDishes(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	combo := createRandomComboSet(t, merchant.ID)

	activeDish := createRandomDish(t, merchant.ID, category.ID)
	deletedDish := createRandomDish(t, merchant.ID, category.ID)

	_, err := testStore.AddComboDish(context.Background(), newComboDishParams(combo.ID, activeDish, 2))
	require.NoError(t, err)

	_, err = testStore.AddComboDish(context.Background(), newComboDishParams(combo.ID, deletedDish, 3))
	require.NoError(t, err)

	err = testStore.DeleteDish(context.Background(), deletedDish.ID)
	require.NoError(t, err)

	rows, err := testStore.ListComboSetsByMerchant(context.Background(), ListComboSetsByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      10,
		Offset:     0,
	})
	require.NoError(t, err)

	var target *ListComboSetsByMerchantRow
	for i := range rows {
		if rows[i].ID == combo.ID {
			target = &rows[i]
			break
		}
	}
	require.NotNil(t, target)
	require.Equal(t, int64(1), target.DishCount)
	require.Equal(t, int64(2), target.DishTotalQuantity)
}

func TestListComboSetsByMerchantWithFilter(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	createRandomComboSet(t, merchant.ID)

	arg := ListComboSetsByMerchantParams{
		MerchantID: merchant.ID,
		IsOnline:   pgtype.Bool{Bool: true, Valid: true},
		Limit:      10,
		Offset:     0,
	}

	combos, err := testStore.ListComboSetsByMerchant(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, combos)

	for _, combo := range combos {
		require.True(t, combo.IsOnline)
		require.NotZero(t, combo.ID)
	}
}

func TestListComboSetsByMerchantUsesIDTieBreaker(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	combo1 := createRandomComboSet(t, merchant.ID)
	combo2 := createRandomComboSet(t, merchant.ID)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)

	store, ok := testStore.(*SQLStore)
	require.True(t, ok)

	_, err := store.connPool.Exec(context.Background(),
		"UPDATE combo_sets SET created_at = $1 WHERE id = ANY($2)",
		tiedCreatedAt,
		[]int64{combo1.ID, combo2.ID},
	)
	require.NoError(t, err)

	rows, err := testStore.ListComboSetsByMerchant(context.Background(), ListComboSetsByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, combo2.ID, rows[0].ID)
	require.Equal(t, combo1.ID, rows[1].ID)
}

func TestUpdateComboSet(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	combo := createRandomComboSet(t, merchant.ID)

	newName := util.RandomString(10)
	newComboPrice := util.RandomMoney()

	arg := UpdateComboSetParams{
		ID:         combo.ID,
		Name:       pgtype.Text{String: newName, Valid: true},
		ComboPrice: pgtype.Int8{Int64: newComboPrice, Valid: true},
	}

	updatedCombo, err := testStore.UpdateComboSet(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedCombo)

	require.Equal(t, combo.ID, updatedCombo.ID)
	require.Equal(t, newName, updatedCombo.Name)
	require.Equal(t, newComboPrice, updatedCombo.ComboPrice)
}

func TestUpdateComboSetOnlineStatus(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	combo := createRandomComboSet(t, merchant.ID)

	arg := UpdateComboSetOnlineStatusParams{
		ID:       combo.ID,
		IsOnline: false,
	}

	err := testStore.UpdateComboSetOnlineStatus(context.Background(), arg)
	require.NoError(t, err)

	// 验证更新
	updatedCombo, err := testStore.GetComboSet(context.Background(), combo.ID)
	require.NoError(t, err)
	require.False(t, updatedCombo.IsOnline)
}

func TestDeleteComboSet(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	combo := createRandomComboSet(t, merchant.ID)
	dish := createRandomDish(t, merchant.ID, category.ID)
	tag := createRandomTag(t, "combo")

	_, err := testStore.AddComboDish(context.Background(), newComboDishParams(combo.ID, dish, 2))
	require.NoError(t, err)
	_, err = testStore.AddComboTag(context.Background(), AddComboTagParams{
		ComboID: combo.ID,
		TagID:   tag.ID,
	})
	require.NoError(t, err)

	dishes, err := testStore.ListComboDishes(context.Background(), combo.ID)
	require.NoError(t, err)
	require.Len(t, dishes, 1)
	require.Equal(t, dish.ID, dishes[0].ID)

	tags, err := testStore.ListComboTags(context.Background(), combo.ID)
	require.NoError(t, err)
	require.Len(t, tags, 1)
	require.Equal(t, tag.ID, tags[0].ID)

	err = testStore.DeleteComboSet(context.Background(), combo.ID)
	require.NoError(t, err)

	// 验证软删除后主套餐对普通读取不可见。
	combo2, err := testStore.GetComboSet(context.Background(), combo.ID)
	require.Error(t, err)
	require.Empty(t, combo2)

	// 关联行被刻意保留，用于历史订单、审计和后台排查。
	dishes, err = testStore.ListComboDishes(context.Background(), combo.ID)
	require.NoError(t, err)
	require.Len(t, dishes, 1)
	require.Equal(t, dish.ID, dishes[0].ID)

	tags, err = testStore.ListComboTags(context.Background(), combo.ID)
	require.NoError(t, err)
	require.Len(t, tags, 1)
	require.Equal(t, tag.ID, tags[0].ID)
}

func TestCountComboSetsByMerchant(t *testing.T) {
	merchant := createRandomMerchantForDish(t)

	// 创建一些套餐
	for i := 0; i < 3; i++ {
		createRandomComboSet(t, merchant.ID)
	}

	arg := CountComboSetsByMerchantParams{
		MerchantID: merchant.ID,
	}

	count, err := testStore.CountComboSetsByMerchant(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(3))
}

// ============================================
// 套餐-菜品关联测试
// ============================================

func TestAddComboDish(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	combo := createRandomComboSet(t, merchant.ID)
	customizations := []byte(`{"12":34,"meta_specs":"大杯 / 少冰"}`)

	arg := AddComboDishParams{
		ComboID:                 combo.ID,
		DishID:                  dish.ID,
		Quantity:                int16(util.RandomInt(1, 5)),
		DishBasePriceSnapshot:   dish.Price,
		Customizations:          customizations,
		CustomizationExtraPrice: 300,
	}

	comboDish, err := testStore.AddComboDish(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, comboDish)
	require.Equal(t, combo.ID, comboDish.ComboID)
	require.Equal(t, dish.ID, comboDish.DishID)
	require.Equal(t, dish.Price, comboDish.DishBasePriceSnapshot)
	require.JSONEq(t, string(customizations), string(comboDish.Customizations))
	require.Equal(t, int64(300), comboDish.CustomizationExtraPrice)

	// 验证关联
	dishes, err := testStore.ListComboDishes(context.Background(), combo.ID)
	require.NoError(t, err)
	require.NotEmpty(t, dishes)

	found := false
	for _, d := range dishes {
		if d.ID == dish.ID {
			found = true
			require.Equal(t, dish.Price, d.DishBasePriceSnapshot)
			require.JSONEq(t, string(customizations), string(d.Customizations))
			require.Equal(t, int64(300), d.CustomizationExtraPrice)
			break
		}
	}
	require.True(t, found)
}

func TestListComboDishes(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	combo := createRandomComboSet(t, merchant.ID)

	// 添加多个菜品到套餐
	for i := 0; i < 3; i++ {
		dish := createRandomDish(t, merchant.ID, category.ID)
		_, err := testStore.AddComboDish(context.Background(), newComboDishParams(combo.ID, dish, 1))
		require.NoError(t, err)
	}

	dishes, err := testStore.ListComboDishes(context.Background(), combo.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(dishes), 3)
}

func TestRemoveComboDish(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	combo := createRandomComboSet(t, merchant.ID)

	// 先添加
	_, err := testStore.AddComboDish(context.Background(), newComboDishParams(combo.ID, dish, 1))
	require.NoError(t, err)

	// 再移除
	arg := RemoveComboDishParams{
		ComboID: combo.ID,
		DishID:  dish.ID,
	}
	err = testStore.RemoveComboDish(context.Background(), arg)
	require.NoError(t, err)

	// 验证已移除
	dishes, err := testStore.ListComboDishes(context.Background(), combo.ID)
	require.NoError(t, err)

	for _, d := range dishes {
		require.NotEqual(t, dish.ID, d.ID)
	}
}

func TestRemoveAllComboDishes(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	combo := createRandomComboSet(t, merchant.ID)

	// 添加多个菜品
	for i := 0; i < 3; i++ {
		dish := createRandomDish(t, merchant.ID, category.ID)
		_, err := testStore.AddComboDish(context.Background(), newComboDishParams(combo.ID, dish, 1))
		require.NoError(t, err)
	}

	// 移除所有菜品
	err := testStore.RemoveAllComboDishes(context.Background(), combo.ID)
	require.NoError(t, err)

	// 验证全部移除
	dishes, err := testStore.ListComboDishes(context.Background(), combo.ID)
	require.NoError(t, err)
	require.Empty(t, dishes)
}

// ============================================
// 套餐-标签关联测试
// ============================================

func TestAddComboTag(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	combo := createRandomComboSet(t, merchant.ID)

	// 创建一个combo类型的tag
	tag, err := testStore.CreateTag(context.Background(), CreateTagParams{
		Name: util.RandomString(5),
		Type: "combo",
	})
	require.NoError(t, err)

	// 添加tag到combo
	arg := AddComboTagParams{
		ComboID: combo.ID,
		TagID:   tag.ID,
	}

	comboTag, err := testStore.AddComboTag(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, comboTag)

	// 验证关联
	tags, err := testStore.ListComboTags(context.Background(), combo.ID)
	require.NoError(t, err)
	require.NotEmpty(t, tags)

	found := false
	for _, t := range tags {
		if t.ID == tag.ID {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestRemoveComboTag(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	combo := createRandomComboSet(t, merchant.ID)

	// 创建并添加tag
	tag, err := testStore.CreateTag(context.Background(), CreateTagParams{
		Name: util.RandomString(5),
		Type: "combo",
	})
	require.NoError(t, err)

	_, err = testStore.AddComboTag(context.Background(), AddComboTagParams{
		ComboID: combo.ID,
		TagID:   tag.ID,
	})
	require.NoError(t, err)

	// 移除tag
	arg := RemoveComboTagParams{
		ComboID: combo.ID,
		TagID:   tag.ID,
	}
	err = testStore.RemoveComboTag(context.Background(), arg)
	require.NoError(t, err)

	// 验证已移除
	tags, err := testStore.ListComboTags(context.Background(), combo.ID)
	require.NoError(t, err)

	for _, tg := range tags {
		require.NotEqual(t, tag.ID, tg.ID)
	}
}

// ============================================
// 复杂查询测试 - GetComboSetWithDetails
// ============================================

func TestGetComboSetWithDetails(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	combo := createRandomComboSet(t, merchant.ID)

	// 添加菜品
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)
	arg1 := newComboDishParams(combo.ID, dish1, 1)
	arg1.DishBasePriceSnapshot = dish1.Price + 500
	arg1.Customizations = []byte(`{"12":34,"meta_specs":"大杯"}`)
	arg1.CustomizationExtraPrice = 300
	_, err := testStore.AddComboDish(context.Background(), arg1)
	require.NoError(t, err)
	_, err = testStore.AddComboDish(context.Background(), newComboDishParams(combo.ID, dish2, 2))
	require.NoError(t, err)

	// 添加标签
	tag, err := testStore.CreateTag(context.Background(), CreateTagParams{
		Name: util.RandomString(5),
		Type: "combo",
	})
	require.NoError(t, err)
	_, err = testStore.AddComboTag(context.Background(), AddComboTagParams{
		ComboID: combo.ID,
		TagID:   tag.ID,
	})
	require.NoError(t, err)

	// 获取完整信息
	comboDetails, err := testStore.GetComboSetWithDetails(context.Background(), combo.ID)
	require.NoError(t, err)
	require.NotEmpty(t, comboDetails)

	// 验证基本信息
	require.Equal(t, combo.ID, comboDetails.ID)
	require.Equal(t, combo.Name, comboDetails.Name)

	// 验证JSON字段不为空（具体解析在API层）
	require.NotEmpty(t, comboDetails.Dishes)
	require.NotEmpty(t, comboDetails.Tags)

	payload, err := json.Marshal(comboDetails.Dishes)
	require.NoError(t, err)

	var dishes []struct {
		DishID    int64 `json:"dish_id"`
		DishPrice int64 `json:"dish_price"`
	}
	err = json.Unmarshal(payload, &dishes)
	require.NoError(t, err)
	require.Len(t, dishes, 2)
	dishPriceByID := make(map[int64]int64, len(dishes))
	for _, dish := range dishes {
		dishPriceByID[dish.DishID] = dish.DishPrice
	}
	require.Equal(t, arg1.DishBasePriceSnapshot+arg1.CustomizationExtraPrice, dishPriceByID[dish1.ID])
}

func TestGetComboSetWithDetailsExcludesDeletedDishes(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	combo := createRandomComboSet(t, merchant.ID)

	activeDish := createRandomDish(t, merchant.ID, category.ID)
	deletedDish := createRandomDish(t, merchant.ID, category.ID)

	_, err := testStore.AddComboDish(context.Background(), newComboDishParams(combo.ID, activeDish, 1))
	require.NoError(t, err)

	_, err = testStore.AddComboDish(context.Background(), newComboDishParams(combo.ID, deletedDish, 2))
	require.NoError(t, err)

	err = testStore.DeleteDish(context.Background(), deletedDish.ID)
	require.NoError(t, err)

	comboDetails, err := testStore.GetComboSetWithDetails(context.Background(), combo.ID)
	require.NoError(t, err)

	payload, err := json.Marshal(comboDetails.Dishes)
	require.NoError(t, err)

	var dishes []struct {
		DishID int64 `json:"dish_id"`
	}
	err = json.Unmarshal(payload, &dishes)
	require.NoError(t, err)
	require.Len(t, dishes, 1)
	require.Equal(t, activeDish.ID, dishes[0].DishID)
}

// ============================================
// GetCombosWithMerchantByIDs 测试（推荐流用）
// ============================================

func TestGetCombosWithMerchantByIDs(t *testing.T) {
	// 创建商户和套餐
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	combo1 := createRandomComboSet(t, merchant.ID)
	combo2 := createRandomComboSet(t, merchant.ID)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)

	_, err := testStore.AddComboDish(context.Background(), newComboDishParams(combo1.ID, dish1, 1))
	require.NoError(t, err)
	_, err = testStore.AddComboDish(context.Background(), newComboDishParams(combo2.ID, dish2, 1))
	require.NoError(t, err)

	// 批量查询
	comboIDs := []int64{combo1.ID, combo2.ID}
	results, err := testStore.GetCombosWithMerchantByIDs(context.Background(), comboIDs)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// 验证返回数据结构
	for _, r := range results {
		require.NotZero(t, r.ID)
		require.NotEmpty(t, r.Name)
		require.NotZero(t, r.ComboPrice)
		require.NotZero(t, r.OriginalPrice)
		require.NotZero(t, r.MerchantID)
		require.NotEmpty(t, r.MerchantName)
		require.NotZero(t, r.MerchantRegionID)
		// MonthlySales 新套餐应该是0
		require.GreaterOrEqual(t, r.MonthlySales, int32(0))
	}
}

func TestGetCombosWithMerchantByIDs_VerifyPrices(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	// 创建套餐，明确设置价格
	originalPrice := int64(10000) // 100元
	comboPrice := int64(7500)     // 75元
	arg := CreateComboSetParams{
		MerchantID:    merchant.ID,
		Name:          "价格测试套餐_" + util.RandomString(6),
		OriginalPrice: originalPrice,
		ComboPrice:    comboPrice,
		IsOnline:      true,
	}
	combo, err := testStore.CreateComboSet(context.Background(), arg)
	require.NoError(t, err)
	dish := createRandomDish(t, merchant.ID, category.ID)
	_, err = testStore.AddComboDish(context.Background(), newComboDishParams(combo.ID, dish, 1))
	require.NoError(t, err)

	// 查询
	results, err := testStore.GetCombosWithMerchantByIDs(context.Background(), []int64{combo.ID})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// 验证价格
	require.Equal(t, originalPrice, results[0].OriginalPrice)
	require.Equal(t, comboPrice, results[0].ComboPrice)
	// 验证折扣率计算所需的数据完整
	require.Greater(t, results[0].OriginalPrice, results[0].ComboPrice, "原价应大于套餐价")
}

func TestGetCombosWithMerchantByIDs_EmptyIDs(t *testing.T) {
	results, err := testStore.GetCombosWithMerchantByIDs(context.Background(), []int64{})
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestGetCombosWithMerchantByIDs_NonExistentIDs(t *testing.T) {
	results, err := testStore.GetCombosWithMerchantByIDs(context.Background(), []int64{999999999})
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestGetCombosWithMerchantByIDs_FilterOffline(t *testing.T) {
	merchant := createRandomMerchantForDish(t)

	// 创建一个下架套餐
	arg := CreateComboSetParams{
		MerchantID:    merchant.ID,
		Name:          "下架套餐_" + util.RandomString(6),
		OriginalPrice: 5000,
		ComboPrice:    4000,
		IsOnline:      false, // 下架
	}
	offlineCombo, err := testStore.CreateComboSet(context.Background(), arg)
	require.NoError(t, err)

	// 查询应该过滤掉下架套餐
	results, err := testStore.GetCombosWithMerchantByIDs(context.Background(), []int64{offlineCombo.ID})
	require.NoError(t, err)
	require.Empty(t, results, "下架套餐不应被返回")
}

func TestPublicComboQueriesExcludeUnavailableChildDishes(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	availableCombo := createRandomComboSet(t, merchant.ID)
	unavailableCombo := createRandomComboSet(t, merchant.ID)
	searchName := "ComboAvailability_" + util.RandomString(8)

	_, err := testStore.UpdateComboSet(context.Background(), UpdateComboSetParams{
		ID:   availableCombo.ID,
		Name: pgtype.Text{String: searchName + "_available", Valid: true},
	})
	require.NoError(t, err)
	_, err = testStore.UpdateComboSet(context.Background(), UpdateComboSetParams{
		ID:   unavailableCombo.ID,
		Name: pgtype.Text{String: searchName + "_unavailable", Valid: true},
	})
	require.NoError(t, err)

	availableDish := createRandomDish(t, merchant.ID, category.ID)
	unavailableDish := createRandomDish(t, merchant.ID, category.ID)
	err = testStore.UpdateDishAvailability(context.Background(), UpdateDishAvailabilityParams{
		ID:          unavailableDish.ID,
		IsAvailable: false,
	})
	require.NoError(t, err)

	_, err = testStore.AddComboDish(context.Background(), newComboDishParams(availableCombo.ID, availableDish, 1))
	require.NoError(t, err)
	_, err = testStore.AddComboDish(context.Background(), newComboDishParams(unavailableCombo.ID, unavailableDish, 1))
	require.NoError(t, err)

	combosByID, err := testStore.GetCombosByIDs(context.Background(), []int64{availableCombo.ID, unavailableCombo.ID})
	require.NoError(t, err)
	require.Len(t, combosByID, 1)
	require.Equal(t, availableCombo.ID, combosByID[0].ID)

	combosWithMerchant, err := testStore.GetCombosWithMerchantByIDs(context.Background(), []int64{availableCombo.ID, unavailableCombo.ID})
	require.NoError(t, err)
	require.Len(t, combosWithMerchant, 1)
	require.Equal(t, availableCombo.ID, combosWithMerchant[0].ID)

	menuCombos, err := testStore.ListOnlineCombosByMerchant(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Contains(t, comboIDsFromListOnline(menuCombos), availableCombo.ID)
	require.NotContains(t, comboIDsFromListOnline(menuCombos), unavailableCombo.ID)

	publicMerchantCombos, err := testStore.GetMerchantOnlineCombos(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.Contains(t, comboIDsFromMerchantOnline(publicMerchantCombos), availableCombo.ID)
	require.NotContains(t, comboIDsFromMerchantOnline(publicMerchantCombos), unavailableCombo.ID)

	searchRows, err := testStore.SearchCombosGlobal(context.Background(), SearchCombosGlobalParams{
		Column1: searchName,
		Limit:   20,
		Offset:  0,
		Column4: 39.9282,
		Column5: 116.4507,
		RegionID: pgtype.Int8{
			Int64: merchant.RegionID,
			Valid: true,
		},
	})
	require.NoError(t, err)
	require.Contains(t, comboIDsFromSearch(searchRows), availableCombo.ID)
	require.NotContains(t, comboIDsFromSearch(searchRows), unavailableCombo.ID)

	searchIDs, err := testStore.SearchComboIDsGlobal(context.Background(), pgtype.Text{String: searchName, Valid: true})
	require.NoError(t, err)
	require.Contains(t, searchIDs, availableCombo.ID)
	require.NotContains(t, searchIDs, unavailableCombo.ID)

	popularRows, err := testStore.GetPopularCombos(context.Background(), GetPopularCombosParams{
		Limit:   20,
		Column2: 39.9282,
		Column3: 116.4507,
	})
	require.NoError(t, err)
	popularIDs := make([]int64, 0, len(popularRows))
	for _, row := range popularRows {
		popularIDs = append(popularIDs, row.ID)
	}
	require.Contains(t, popularIDs, availableCombo.ID)
	require.NotContains(t, popularIDs, unavailableCombo.ID)

	searchCount, err := testStore.CountSearchCombosGlobal(context.Background(), CountSearchCombosGlobalParams{
		Column1: searchName,
		RegionID: pgtype.Int8{
			Int64: merchant.RegionID,
			Valid: true,
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), searchCount)
}

func TestSearchCombosGlobal_ExcludesTakeoutSuspendedMerchants(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	uniqueName := "SuspendedCombo_" + util.RandomString(8)

	combo, err := testStore.CreateComboSet(context.Background(), CreateComboSetParams{
		MerchantID:    merchant.ID,
		Name:          uniqueName,
		OriginalPrice: util.RandomMoney(),
		ComboPrice:    util.RandomMoney(),
		IsOnline:      true,
	})
	require.NoError(t, err)
	dish := createRandomDish(t, merchant.ID, category.ID)
	_, err = testStore.AddComboDish(context.Background(), newComboDishParams(combo.ID, dish, 1))
	require.NoError(t, err)

	_, err = testStore.CreateMerchantProfile(context.Background(), merchant.ID)
	require.NoError(t, err)

	err = testStore.SuspendMerchantTakeout(context.Background(), SuspendMerchantTakeoutParams{
		MerchantID:           merchant.ID,
		TakeoutSuspendReason: pgtype.Text{String: "claim recovery overdue", Valid: true},
	})
	require.NoError(t, err)

	combos, err := testStore.SearchCombosGlobal(context.Background(), SearchCombosGlobalParams{
		Column1: uniqueName,
		Limit:   10,
		Offset:  0,
		Column4: 39.9282,
		Column5: 116.4507,
		RegionID: pgtype.Int8{
			Int64: merchant.RegionID,
			Valid: true,
		},
	})
	require.NoError(t, err)
	require.Empty(t, combos)

	count, err := testStore.CountSearchCombosGlobal(context.Background(), CountSearchCombosGlobalParams{
		Column1: uniqueName,
		RegionID: pgtype.Int8{
			Int64: merchant.RegionID,
			Valid: true,
		},
	})
	require.NoError(t, err)
	require.Zero(t, count)
}
