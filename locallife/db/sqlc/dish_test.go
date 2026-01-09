package db

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

// 辅助函数：创建随机商户（依赖merchant相关代码）
func createRandomMerchantForDish(t *testing.T) Merchant {
	user := createRandomUser(t)
	region := createRandomRegion(t)

	arg := CreateMerchantParams{
		OwnerUserID: user.ID,
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(20), Valid: true},
		Phone:       util.RandomString(11),
		Address:     util.RandomString(30),
		Status:      "approved", // 必须是approved才能被推荐流查询到
		RegionID:    region.ID,
	}

	merchant, err := testStore.CreateMerchant(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, merchant)

	return merchant
}

// 辅助函数：创建随机菜品分类
func createRandomDishCategory(t *testing.T) DishCategory {
	name := util.RandomString(8)
	category, err := testStore.CreateDishCategory(context.Background(), name)
	require.NoError(t, err)
	require.NotEmpty(t, category)

	require.Equal(t, name, category.Name)
	require.NotZero(t, category.ID)
	require.NotZero(t, category.CreatedAt)

	return category
}

func createAndLinkRandomDishCategory(t *testing.T, merchantID int64) (DishCategory, MerchantDishCategory) {
	category := createRandomDishCategory(t)
	sortOrder := int16(util.RandomInt(1, 100))

	arg := LinkMerchantDishCategoryParams{
		MerchantID: merchantID,
		CategoryID: category.ID,
		SortOrder:  sortOrder,
	}

	mdc, err := testStore.LinkMerchantDishCategory(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, mdc)
	require.Equal(t, merchantID, mdc.MerchantID)
	require.Equal(t, category.ID, mdc.CategoryID)
	require.Equal(t, sortOrder, mdc.SortOrder)

	return category, mdc
}

// 辅助函数：创建随机菜品
func createRandomDish(t *testing.T, merchantID, categoryID int64) Dish {
	arg := CreateDishParams{
		MerchantID:  merchantID,
		CategoryID:  pgtype.Int8{Int64: categoryID, Valid: true},
		Name:        util.RandomString(10),
		Description: pgtype.Text{String: util.RandomString(50), Valid: true},
		ImageUrl:    pgtype.Text{String: "https://example.com/dish.jpg", Valid: true},
		Price:       util.RandomMoney(),
		MemberPrice: pgtype.Int8{Int64: util.RandomMoney(), Valid: true},
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   int16(util.RandomInt(1, 100)),
	}

	dish, err := testStore.CreateDish(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, dish)

	require.Equal(t, arg.MerchantID, dish.MerchantID)
	require.Equal(t, arg.Name, dish.Name)
	require.Equal(t, arg.Price, dish.Price)
	require.True(t, dish.IsAvailable)
	require.True(t, dish.IsOnline)
	require.NotZero(t, dish.ID)
	require.NotZero(t, dish.CreatedAt)

	return dish
}

// ============================================
// 菜品分类测试
// ============================================

func TestCreateDishCategory(t *testing.T) {
	createRandomDishCategory(t)
}

func TestLinkMerchantDishCategory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	createAndLinkRandomDishCategory(t, merchant.ID)
}

func TestGetDishCategory(t *testing.T) {
	category1 := createRandomDishCategory(t)

	category2, err := testStore.GetDishCategory(context.Background(), category1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, category2)

	require.Equal(t, category1.ID, category2.ID)
	require.Equal(t, category1.Name, category2.Name)
}

func TestListDishCategories(t *testing.T) {
	merchant := createRandomMerchantForDish(t)

	// 创建多个分类
	for i := 0; i < 5; i++ {
		createAndLinkRandomDishCategory(t, merchant.ID)
	}

	categories, err := testStore.ListDishCategories(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(categories), 5)

	for _, category := range categories {
		require.NotZero(t, category.ID)
		require.NotEmpty(t, category.Name)
	}
}

func TestUpdateMerchantDishCategoryOrder(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category, mdc := createAndLinkRandomDishCategory(t, merchant.ID)

	newSortOrder := int16(util.RandomInt(1, 100))

	arg := UpdateMerchantDishCategoryOrderParams{
		MerchantID: merchant.ID,
		CategoryID: category.ID,
		SortOrder:  newSortOrder,
	}

	updatedMdc, err := testStore.UpdateMerchantDishCategoryOrder(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedMdc)

	require.Equal(t, mdc.MerchantID, updatedMdc.MerchantID)
	require.Equal(t, mdc.CategoryID, updatedMdc.CategoryID)
	require.Equal(t, newSortOrder, updatedMdc.SortOrder)
}

func TestUnlinkMerchantDishCategory(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category, _ := createAndLinkRandomDishCategory(t, merchant.ID)

	arg := UnlinkMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: category.ID,
	}

	err := testStore.UnlinkMerchantDishCategory(context.Background(), arg)
	require.NoError(t, err)

	// 验证已断开关联
	mdc, err := testStore.GetMerchantDishCategory(context.Background(), GetMerchantDishCategoryParams{
		MerchantID: merchant.ID,
		CategoryID: category.ID,
	})
	require.Error(t, err)
	require.Empty(t, mdc)
}

// ============================================
// 菜品测试
// ============================================

func TestCreateDish(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	createRandomDish(t, merchant.ID, category.ID)
}

func TestGetDish(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)

	dish2, err := testStore.GetDish(context.Background(), dish1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, dish2)

	require.Equal(t, dish1.ID, dish2.ID)
	require.Equal(t, dish1.Name, dish2.Name)
	require.Equal(t, dish1.Price, dish2.Price)
	require.Equal(t, dish1.MerchantID, dish2.MerchantID)
}

func TestListDishesByMerchant(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	// 创建多个菜品
	for i := 0; i < 5; i++ {
		createRandomDish(t, merchant.ID, category.ID)
	}

	arg := ListDishesByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      10,
		Offset:     0,
	}

	dishes, err := testStore.ListDishesByMerchant(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(dishes), 5)

	for _, dish := range dishes {
		require.Equal(t, merchant.ID, dish.MerchantID)
	}
}

func TestListDishesByMerchantWithFilters(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	// 创建在线和离线菜品
	createRandomDish(t, merchant.ID, category.ID)

	testCases := []struct {
		name   string
		params ListDishesByMerchantParams
	}{
		{
			name: "FilterByCategory",
			params: ListDishesByMerchantParams{
				MerchantID: merchant.ID,
				CategoryID: pgtype.Int8{Int64: category.ID, Valid: true},
				Limit:      10,
				Offset:     0,
			},
		},
		{
			name: "FilterByOnlineStatus",
			params: ListDishesByMerchantParams{
				MerchantID: merchant.ID,
				IsOnline:   pgtype.Bool{Bool: true, Valid: true},
				Limit:      10,
				Offset:     0,
			},
		},
		{
			name: "FilterByAvailability",
			params: ListDishesByMerchantParams{
				MerchantID:  merchant.ID,
				IsAvailable: pgtype.Bool{Bool: true, Valid: true},
				Limit:       10,
				Offset:      0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dishes, err := testStore.ListDishesByMerchant(context.Background(), tc.params)
			require.NoError(t, err)
			require.NotEmpty(t, dishes)
		})
	}
}

func TestSearchDishesByName(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	// 搜索菜品名称的前几个字符
	searchTerm := dish.Name[:3]

	arg := SearchDishesByNameParams{
		MerchantID: merchant.ID,
		Column2:    pgtype.Text{String: searchTerm, Valid: true},
		Limit:      10,
		Offset:     0,
	}

	dishes, err := testStore.SearchDishesByName(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, dishes)

	// 验证找到创建的菜品
	found := false
	for _, d := range dishes {
		if d.ID == dish.ID {
			found = true
			break
		}
	}
	require.True(t, found, "Created dish should be found in search results")
}

func TestUpdateDish(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	newName := util.RandomString(10)
	newPrice := util.RandomMoney()

	arg := UpdateDishParams{
		ID:    dish.ID,
		Name:  pgtype.Text{String: newName, Valid: true},
		Price: pgtype.Int8{Int64: newPrice, Valid: true},
	}

	updatedDish, err := testStore.UpdateDish(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedDish)

	require.Equal(t, dish.ID, updatedDish.ID)
	require.Equal(t, newName, updatedDish.Name)
	require.Equal(t, newPrice, updatedDish.Price)
}

func TestUpdateDishAvailability(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	arg := UpdateDishAvailabilityParams{
		ID:          dish.ID,
		IsAvailable: false,
	}

	err := testStore.UpdateDishAvailability(context.Background(), arg)
	require.NoError(t, err)

	// 验证更新
	updatedDish, err := testStore.GetDish(context.Background(), dish.ID)
	require.NoError(t, err)
	require.False(t, updatedDish.IsAvailable)
}

func TestUpdateDishOnlineStatus(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	arg := UpdateDishOnlineStatusParams{
		ID:       dish.ID,
		IsOnline: false,
	}

	err := testStore.UpdateDishOnlineStatus(context.Background(), arg)
	require.NoError(t, err)

	// 验证更新
	updatedDish, err := testStore.GetDish(context.Background(), dish.ID)
	require.NoError(t, err)
	require.False(t, updatedDish.IsOnline)
}

func TestDeleteDish(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	err := testStore.DeleteDish(context.Background(), dish.ID)
	require.NoError(t, err)

	// 验证已删除
	dish2, err := testStore.GetDish(context.Background(), dish.ID)
	require.Error(t, err)
	require.Empty(t, dish2)
}

func TestCountDishesByMerchant(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	// 创建一些菜品
	for i := 0; i < 3; i++ {
		createRandomDish(t, merchant.ID, category.ID)
	}

	arg := CountDishesByMerchantParams{
		MerchantID: merchant.ID,
	}

	count, err := testStore.CountDishesByMerchant(context.Background(), arg)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(3))
}

// ============================================
// 菜品-食材关联测试
// ============================================

func TestAddDishIngredient(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	ingredient := createRandomIngredient(t, true)

	arg := AddDishIngredientParams{
		DishID:       dish.ID,
		IngredientID: ingredient.ID,
	}

	_, err := testStore.AddDishIngredient(context.Background(), arg)
	require.NoError(t, err)

	// 验证关联
	ingredients, err := testStore.ListDishIngredients(context.Background(), dish.ID)
	require.NoError(t, err)
	require.NotEmpty(t, ingredients)

	found := false
	for _, ing := range ingredients {
		if ing.ID == ingredient.ID {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestRemoveDishIngredient(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	ingredient := createRandomIngredient(t, true)

	// 先添加
	addArg := AddDishIngredientParams{
		DishID:       dish.ID,
		IngredientID: ingredient.ID,
	}
	_, err := testStore.AddDishIngredient(context.Background(), addArg)
	require.NoError(t, err)

	// 再移除
	removeArg := RemoveDishIngredientParams{
		DishID:       dish.ID,
		IngredientID: ingredient.ID,
	}
	err = testStore.RemoveDishIngredient(context.Background(), removeArg)
	require.NoError(t, err)

	// 验证已移除
	ingredients, err := testStore.ListDishIngredients(context.Background(), dish.ID)
	require.NoError(t, err)

	for _, ing := range ingredients {
		require.NotEqual(t, ingredient.ID, ing.ID)
	}
}

// ============================================
// 菜品-标签关联测试
// ============================================

func TestAddDishTag(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	// 创建一个dish类型的tag
	tagArg := CreateTagParams{
		Name: util.RandomString(5),
		Type: "dish",
	}
	tag, err := testStore.CreateTag(context.Background(), tagArg)
	require.NoError(t, err)

	// 添加tag到dish
	arg := AddDishTagParams{
		DishID: dish.ID,
		TagID:  tag.ID,
	}

	_, err = testStore.AddDishTag(context.Background(), arg)
	require.NoError(t, err)

	// 验证关联
	tags, err := testStore.ListDishTags(context.Background(), dish.ID)
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

func TestRemoveDishTag(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	// 创建并添加tag
	tagArg := CreateTagParams{
		Name: util.RandomString(5),
		Type: "dish",
	}
	tag, err := testStore.CreateTag(context.Background(), tagArg)
	require.NoError(t, err)

	addArg := AddDishTagParams{
		DishID: dish.ID,
		TagID:  tag.ID,
	}
	_, err = testStore.AddDishTag(context.Background(), addArg)
	require.NoError(t, err)

	// 移除tag
	removeArg := RemoveDishTagParams{
		DishID: dish.ID,
		TagID:  tag.ID,
	}
	err = testStore.RemoveDishTag(context.Background(), removeArg)
	require.NoError(t, err)

	// 验证已移除
	tags, err := testStore.ListDishTags(context.Background(), dish.ID)
	require.NoError(t, err)

	for _, tg := range tags {
		require.NotEqual(t, tag.ID, tg.ID)
	}
}

// ============================================
// 复杂查询测试 - GetDishWithDetails
// ============================================

func TestGetDishWithDetails(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	// 添加食材
	ingredient1 := createRandomIngredient(t, true)
	ingredient2 := createRandomIngredient(t, true)
	_, err := testStore.AddDishIngredient(context.Background(), AddDishIngredientParams{
		DishID:       dish.ID,
		IngredientID: ingredient1.ID,
	})
	require.NoError(t, err)
	_, err = testStore.AddDishIngredient(context.Background(), AddDishIngredientParams{
		DishID:       dish.ID,
		IngredientID: ingredient2.ID,
	})
	require.NoError(t, err)

	// 添加标签
	tag, err := testStore.CreateTag(context.Background(), CreateTagParams{
		Name: util.RandomString(5),
		Type: "dish",
	})
	require.NoError(t, err)
	_, err = testStore.AddDishTag(context.Background(), AddDishTagParams{
		DishID: dish.ID,
		TagID:  tag.ID,
	})
	require.NoError(t, err)

	// 获取完整信息
	dishDetails, err := testStore.GetDishWithDetails(context.Background(), dish.ID)
	require.NoError(t, err)
	require.NotEmpty(t, dishDetails)

	// 验证基本信息
	require.Equal(t, dish.ID, dishDetails.ID)
	require.Equal(t, dish.Name, dishDetails.Name)
	require.Equal(t, category.Name, dishDetails.CategoryName.String)

	// 验证JSON字段不为空（具体解析在API层）
	require.NotEmpty(t, dishDetails.Ingredients)
	require.NotEmpty(t, dishDetails.Tags)
}

// ============================================
// 全局搜索测试 - SearchDishesGlobal
// ============================================

func TestSearchDishesGlobal(t *testing.T) {
	// 创建已批准的商户
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	// 将商户状态改为 approved
	_, err := testStore.UpdateMerchantStatus(context.Background(), UpdateMerchantStatusParams{
		ID:     merchant.ID,
		Status: "approved",
	})
	require.NoError(t, err)

	category := createRandomDishCategory(t)

	// 使用足够唯一的名称确保搜索结果准确
	uniqueSuffix := util.RandomString(12)
	uniqueName := "XYZGlobalDish_" + uniqueSuffix
	dish, err := testStore.CreateDish(context.Background(), CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        uniqueName,
		Price:       util.RandomMoney(),
		IsAvailable: true,
		IsOnline:    true,
	})
	require.NoError(t, err)

	// 用完整的唯一名称搜索
	dishes, err := testStore.SearchDishesGlobal(context.Background(), SearchDishesGlobalParams{
		Column1: pgtype.Text{String: uniqueName, Valid: true},
		Limit:   10,
		Offset:  0,
	})
	require.NoError(t, err)
	require.NotEmpty(t, dishes)

	// 验证找到创建的菜品
	found := false
	for _, d := range dishes {
		if d.ID == dish.ID {
			found = true
			break
		}
	}
	require.True(t, found, "Created dish should be found in global search results")
}

func TestSearchDishesGlobal_OnlyApprovedMerchants(t *testing.T) {
	// 创建一个用户
	owner := createRandomUser(t)
	region := createRandomRegion(t)

	// 手动创建pending状态的商户
	appData, _ := json.Marshal(map[string]string{"test": "data"})
	merchant, err := testStore.CreateMerchant(context.Background(), CreateMerchantParams{
		OwnerUserID:     owner.ID,
		Name:            "PendingMerchant_" + util.RandomString(6),
		Phone:           fmt.Sprintf("138%08d", util.RandomInt(10000000, 99999999)),
		Address:         "test address " + util.RandomString(10),
		Status:          "pending", // pending 状态
		ApplicationData: appData,
		RegionID:        region.ID,
	})
	require.NoError(t, err)

	category := createRandomDishCategory(t)

	uniqueName := "TestPendingMerchantDish_" + util.RandomString(6)
	_, err = testStore.CreateDish(context.Background(), CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        uniqueName,
		Price:       util.RandomMoney(),
		IsAvailable: true,
		IsOnline:    true,
	})
	require.NoError(t, err)

	// 全局搜索不应找到未批准商户的菜品
	dishes, err := testStore.SearchDishesGlobal(context.Background(), SearchDishesGlobalParams{
		Column1: pgtype.Text{String: uniqueName, Valid: true},
		Limit:   10,
		Offset:  0,
	})
	require.NoError(t, err)
	require.Empty(t, dishes, "Dishes from pending merchant should not appear in global search")
}

func TestSearchDishesGlobal_Pagination(t *testing.T) {
	// 创建已批准的商户
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_, err := testStore.UpdateMerchantStatus(context.Background(), UpdateMerchantStatusParams{
		ID:     merchant.ID,
		Status: "approved",
	})
	require.NoError(t, err)

	category := createRandomDishCategory(t)

	// 创建多个菜品
	prefix := "TestPagination_" + util.RandomString(4) + "_"
	for i := 0; i < 5; i++ {
		_, err := testStore.CreateDish(context.Background(), CreateDishParams{
			MerchantID:  merchant.ID,
			CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
			Name:        prefix + util.RandomString(4),
			Price:       util.RandomMoney(),
			IsAvailable: true,
			IsOnline:    true,
		})
		require.NoError(t, err)
	}

	// 第一页
	page1, err := testStore.SearchDishesGlobal(context.Background(), SearchDishesGlobalParams{
		Column1: pgtype.Text{String: prefix, Valid: true},
		Limit:   2,
		Offset:  0,
	})
	require.NoError(t, err)
	require.Len(t, page1, 2)

	// 第二页
	page2, err := testStore.SearchDishesGlobal(context.Background(), SearchDishesGlobalParams{
		Column1: pgtype.Text{String: prefix, Valid: true},
		Limit:   2,
		Offset:  2,
	})
	require.NoError(t, err)
	require.Len(t, page2, 2)

	// 确保两页不重复
	for _, d1 := range page1 {
		for _, d2 := range page2 {
			require.NotEqual(t, d1.ID, d2.ID)
		}
	}
}

func TestCountSearchDishesGlobal(t *testing.T) {
	// 创建已批准的商户
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	_, err := testStore.UpdateMerchantStatus(context.Background(), UpdateMerchantStatusParams{
		ID:     merchant.ID,
		Status: "approved",
	})
	require.NoError(t, err)

	category := createRandomDishCategory(t)

	prefix := "TestCount_" + util.RandomString(6) + "_"

	// 创建3个菜品
	for i := 0; i < 3; i++ {
		_, err := testStore.CreateDish(context.Background(), CreateDishParams{
			MerchantID:  merchant.ID,
			CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
			Name:        prefix + util.RandomString(4),
			Price:       util.RandomMoney(),
			IsAvailable: true,
			IsOnline:    true,
		})
		require.NoError(t, err)
	}

	// 计数
	count, err := testStore.CountSearchDishesGlobal(context.Background(), CountSearchDishesGlobalParams{
		Column1: pgtype.Text{String: prefix, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func TestCountSearchDishesByName(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	prefix := "TestCountByName_" + util.RandomString(6) + "_"

	// 创建4个菜品
	for i := 0; i < 4; i++ {
		_, err := testStore.CreateDish(context.Background(), CreateDishParams{
			MerchantID:  merchant.ID,
			CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
			Name:        prefix + util.RandomString(4),
			Price:       util.RandomMoney(),
			IsAvailable: true,
			IsOnline:    true,
		})
		require.NoError(t, err)
	}

	// 计数
	count, err := testStore.CountSearchDishesByName(context.Background(), CountSearchDishesByNameParams{
		MerchantID: merchant.ID,
		Column2:    pgtype.Text{String: prefix, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(4), count)
}

func TestCountSearchDishesByName_EmptyResult(t *testing.T) {
	merchant := createRandomMerchantForDish(t)

	count, err := testStore.CountSearchDishesByName(context.Background(), CountSearchDishesByNameParams{
		MerchantID: merchant.ID,
		Column2:    pgtype.Text{String: "NonExistentDishName12345", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

// ============================================
// GetDishesWithMerchantByIDs 测试（推荐流用）
// ============================================

func TestGetDishesWithMerchantByIDs(t *testing.T) {
	// 创建商户和菜品
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)

	// 批量查询
	dishIDs := []int64{dish1.ID, dish2.ID}
	results, err := testStore.GetDishesWithMerchantByIDs(context.Background(), dishIDs)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// 验证返回数据结构
	for _, r := range results {
		require.NotZero(t, r.ID)
		require.NotEmpty(t, r.Name)
		require.NotZero(t, r.Price)
		require.NotZero(t, r.MerchantID)
		require.NotEmpty(t, r.MerchantName)
		require.NotZero(t, r.MerchantRegionID)
		// MonthlySales 新菜品应该是0
		require.GreaterOrEqual(t, r.MonthlySales, int32(0))
	}
}

func TestGetDishesWithMerchantByIDs_WithMemberPrice(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	// 创建带会员价的菜品
	memberPrice := int64(1500) // 15元会员价
	arg := CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        "会员价测试菜品_" + util.RandomString(6),
		Price:       2000, // 20元原价
		MemberPrice: pgtype.Int8{Int64: memberPrice, Valid: true},
		IsAvailable: true,
		IsOnline:    true,
	}
	dish, err := testStore.CreateDish(context.Background(), arg)
	require.NoError(t, err)

	// 查询
	results, err := testStore.GetDishesWithMerchantByIDs(context.Background(), []int64{dish.ID})
	require.NoError(t, err)
	require.Len(t, results, 1)

	// 验证会员价
	require.True(t, results[0].MemberPrice.Valid)
	require.Equal(t, memberPrice, results[0].MemberPrice.Int64)
}

func TestGetDishesWithMerchantByIDs_EmptyIDs(t *testing.T) {
	// 空ID列表应该返回空结果
	results, err := testStore.GetDishesWithMerchantByIDs(context.Background(), []int64{})
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestGetDishesWithMerchantByIDs_NonExistentIDs(t *testing.T) {
	// 不存在的ID应该返回空结果
	results, err := testStore.GetDishesWithMerchantByIDs(context.Background(), []int64{999999999})
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestGetDishesWithMerchantByIDs_FilterOffline(t *testing.T) {
	merchant := createRandomMerchantForDish(t)
	category := createRandomDishCategory(t)

	// 创建一个下架菜品
	arg := CreateDishParams{
		MerchantID:  merchant.ID,
		CategoryID:  pgtype.Int8{Int64: category.ID, Valid: true},
		Name:        "下架菜品_" + util.RandomString(6),
		Price:       1000,
		IsAvailable: true,
		IsOnline:    false, // 下架
	}
	offlineDish, err := testStore.CreateDish(context.Background(), arg)
	require.NoError(t, err)

	// 查询应该过滤掉下架菜品
	results, err := testStore.GetDishesWithMerchantByIDs(context.Background(), []int64{offlineDish.ID})
	require.NoError(t, err)
	require.Empty(t, results, "下架菜品不应被返回")
}
