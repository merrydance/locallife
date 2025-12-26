package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

// addFavoriteMerchantHelper 添加收藏商户
func addFavoriteMerchantHelper(t *testing.T, userID, merchantID int64) Favorite {
	favorite, err := testStore.AddFavoriteMerchant(context.Background(), AddFavoriteMerchantParams{
		UserID:     userID,
		MerchantID: pgtype.Int8{Int64: merchantID, Valid: true},
	})
	require.NoError(t, err)
	require.NotZero(t, favorite.ID)

	return favorite
}

// addFavoriteDishHelper 添加收藏菜品
func addFavoriteDishHelper(t *testing.T, userID, dishID int64) Favorite {
	favorite, err := testStore.AddFavoriteDish(context.Background(), AddFavoriteDishParams{
		UserID: userID,
		DishID: pgtype.Int8{Int64: dishID, Valid: true},
	})
	require.NoError(t, err)
	require.NotZero(t, favorite.ID)

	return favorite
}

// ==================== AddFavoriteMerchant Tests ====================

func TestAddFavoriteMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	favorite := addFavoriteMerchantHelper(t, user.ID, merchant.ID)

	require.Equal(t, user.ID, favorite.UserID)
	require.Equal(t, "merchant", favorite.FavoriteType)
	require.True(t, favorite.MerchantID.Valid)
	require.Equal(t, merchant.ID, favorite.MerchantID.Int64)
	require.False(t, favorite.DishID.Valid)
	require.NotZero(t, favorite.CreatedAt)
}

func TestAddFavoriteMerchant_Duplicate(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	// 第一次收藏
	_ = addFavoriteMerchantHelper(t, user.ID, merchant.ID)

	// 重复收藏 (ON CONFLICT DO NOTHING, 不应返回数据)
	_, err := testStore.AddFavoriteMerchant(context.Background(), AddFavoriteMerchantParams{
		UserID:     user.ID,
		MerchantID: pgtype.Int8{Int64: merchant.ID, Valid: true},
	})
	require.Error(t, err) // DO NOTHING 会导致 no rows in result set
}

func TestAddFavoriteMerchant_MultipleMerchants(t *testing.T) {
	owner1 := createRandomUser(t)
	owner2 := createRandomUser(t)
	merchant1 := createRandomMerchantWithOwner(t, owner1.ID)
	merchant2 := createRandomMerchantWithOwner(t, owner2.ID)
	user := createRandomUser(t)

	fav1 := addFavoriteMerchantHelper(t, user.ID, merchant1.ID)
	fav2 := addFavoriteMerchantHelper(t, user.ID, merchant2.ID)

	require.NotEqual(t, fav1.ID, fav2.ID)
	require.Equal(t, merchant1.ID, fav1.MerchantID.Int64)
	require.Equal(t, merchant2.ID, fav2.MerchantID.Int64)
}

// ==================== AddFavoriteDish Tests ====================

func TestAddFavoriteDish(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	user := createRandomUser(t)

	favorite := addFavoriteDishHelper(t, user.ID, dish.ID)

	require.Equal(t, user.ID, favorite.UserID)
	require.Equal(t, "dish", favorite.FavoriteType)
	require.True(t, favorite.DishID.Valid)
	require.Equal(t, dish.ID, favorite.DishID.Int64)
	require.False(t, favorite.MerchantID.Valid)
	require.NotZero(t, favorite.CreatedAt)
}

func TestAddFavoriteDish_Duplicate(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	user := createRandomUser(t)

	// 第一次收藏
	_ = addFavoriteDishHelper(t, user.ID, dish.ID)

	// 重复收藏
	_, err := testStore.AddFavoriteDish(context.Background(), AddFavoriteDishParams{
		UserID: user.ID,
		DishID: pgtype.Int8{Int64: dish.ID, Valid: true},
	})
	require.Error(t, err) // DO NOTHING 会导致 no rows in result set
}

func TestAddFavoriteDish_MultipleDishes(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)
	user := createRandomUser(t)

	fav1 := addFavoriteDishHelper(t, user.ID, dish1.ID)
	fav2 := addFavoriteDishHelper(t, user.ID, dish2.ID)

	require.NotEqual(t, fav1.ID, fav2.ID)
	require.Equal(t, dish1.ID, fav1.DishID.Int64)
	require.Equal(t, dish2.ID, fav2.DishID.Int64)
}

// ==================== IsMerchantFavorited Tests ====================

func TestIsMerchantFavorited(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	// 未收藏时
	isFav, err := testStore.IsMerchantFavorited(context.Background(), IsMerchantFavoritedParams{
		UserID:     user.ID,
		MerchantID: pgtype.Int8{Int64: merchant.ID, Valid: true},
	})
	require.NoError(t, err)
	require.False(t, isFav)

	// 收藏后
	_ = addFavoriteMerchantHelper(t, user.ID, merchant.ID)

	isFav, err = testStore.IsMerchantFavorited(context.Background(), IsMerchantFavoritedParams{
		UserID:     user.ID,
		MerchantID: pgtype.Int8{Int64: merchant.ID, Valid: true},
	})
	require.NoError(t, err)
	require.True(t, isFav)
}

func TestIsMerchantFavorited_DifferentUser(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user1 := createRandomUser(t)
	user2 := createRandomUser(t)

	// user1 收藏
	_ = addFavoriteMerchantHelper(t, user1.ID, merchant.ID)

	// user2 不应该显示收藏
	isFav, err := testStore.IsMerchantFavorited(context.Background(), IsMerchantFavoritedParams{
		UserID:     user2.ID,
		MerchantID: pgtype.Int8{Int64: merchant.ID, Valid: true},
	})
	require.NoError(t, err)
	require.False(t, isFav)
}

// ==================== IsDishFavorited Tests ====================

func TestIsDishFavorited(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	user := createRandomUser(t)

	// 未收藏时
	isFav, err := testStore.IsDishFavorited(context.Background(), IsDishFavoritedParams{
		UserID: user.ID,
		DishID: pgtype.Int8{Int64: dish.ID, Valid: true},
	})
	require.NoError(t, err)
	require.False(t, isFav)

	// 收藏后
	_ = addFavoriteDishHelper(t, user.ID, dish.ID)

	isFav, err = testStore.IsDishFavorited(context.Background(), IsDishFavoritedParams{
		UserID: user.ID,
		DishID: pgtype.Int8{Int64: dish.ID, Valid: true},
	})
	require.NoError(t, err)
	require.True(t, isFav)
}

// ==================== CountFavoriteMerchants Tests ====================

func TestCountFavoriteMerchants(t *testing.T) {
	owner1 := createRandomUser(t)
	owner2 := createRandomUser(t)
	owner3 := createRandomUser(t)
	merchant1 := createRandomMerchantWithOwner(t, owner1.ID)
	merchant2 := createRandomMerchantWithOwner(t, owner2.ID)
	merchant3 := createRandomMerchantWithOwner(t, owner3.ID)
	user := createRandomUser(t)

	// 初始计数
	count, err := testStore.CountFavoriteMerchants(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)

	// 添加收藏
	_ = addFavoriteMerchantHelper(t, user.ID, merchant1.ID)
	_ = addFavoriteMerchantHelper(t, user.ID, merchant2.ID)
	_ = addFavoriteMerchantHelper(t, user.ID, merchant3.ID)

	count, err = testStore.CountFavoriteMerchants(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

// ==================== CountFavoriteDishes Tests ====================

func TestCountFavoriteDishes(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)
	user := createRandomUser(t)

	// 初始计数
	count, err := testStore.CountFavoriteDishes(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)

	// 添加收藏
	_ = addFavoriteDishHelper(t, user.ID, dish1.ID)
	_ = addFavoriteDishHelper(t, user.ID, dish2.ID)

	count, err = testStore.CountFavoriteDishes(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}

// ==================== ListFavoriteMerchants Tests ====================

func TestListFavoriteMerchants(t *testing.T) {
	owner1 := createRandomUser(t)
	owner2 := createRandomUser(t)
	merchant1 := createRandomMerchantWithOwner(t, owner1.ID)
	merchant2 := createRandomMerchantWithOwner(t, owner2.ID)
	user := createRandomUser(t)

	_ = addFavoriteMerchantHelper(t, user.ID, merchant1.ID)
	_ = addFavoriteMerchantHelper(t, user.ID, merchant2.ID)

	favorites, err := testStore.ListFavoriteMerchants(context.Background(), ListFavoriteMerchantsParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, favorites, 2)

	// 按创建时间倒序
	require.True(t, favorites[0].CreatedAt.After(favorites[1].CreatedAt) ||
		favorites[0].CreatedAt.Equal(favorites[1].CreatedAt))

	// 验证返回了商户信息
	for _, fav := range favorites {
		require.NotEmpty(t, fav.MerchantName)
		require.NotEmpty(t, fav.MerchantAddress)
	}
}

func TestListFavoriteMerchants_Pagination(t *testing.T) {
	owner1 := createRandomUser(t)
	owner2 := createRandomUser(t)
	owner3 := createRandomUser(t)
	merchant1 := createRandomMerchantWithOwner(t, owner1.ID)
	merchant2 := createRandomMerchantWithOwner(t, owner2.ID)
	merchant3 := createRandomMerchantWithOwner(t, owner3.ID)
	user := createRandomUser(t)

	_ = addFavoriteMerchantHelper(t, user.ID, merchant1.ID)
	_ = addFavoriteMerchantHelper(t, user.ID, merchant2.ID)
	_ = addFavoriteMerchantHelper(t, user.ID, merchant3.ID)

	// 第一页
	page1, err := testStore.ListFavoriteMerchants(context.Background(), ListFavoriteMerchantsParams{
		UserID: user.ID,
		Limit:  2,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, page1, 2)

	// 第二页
	page2, err := testStore.ListFavoriteMerchants(context.Background(), ListFavoriteMerchantsParams{
		UserID: user.ID,
		Limit:  2,
		Offset: 2,
	})
	require.NoError(t, err)
	require.Len(t, page2, 1)
}

func TestListFavoriteMerchants_Empty(t *testing.T) {
	user := createRandomUser(t)

	favorites, err := testStore.ListFavoriteMerchants(context.Background(), ListFavoriteMerchantsParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Empty(t, favorites)
}

// ==================== ListFavoriteDishes Tests ====================

func TestListFavoriteDishes(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)
	user := createRandomUser(t)

	_ = addFavoriteDishHelper(t, user.ID, dish1.ID)
	_ = addFavoriteDishHelper(t, user.ID, dish2.ID)

	favorites, err := testStore.ListFavoriteDishes(context.Background(), ListFavoriteDishesParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, favorites, 2)

	// 验证返回了菜品和商户信息
	for _, fav := range favorites {
		require.NotEmpty(t, fav.DishName)
		require.NotZero(t, fav.DishPrice)
		require.NotEmpty(t, fav.MerchantName)
	}
}

func TestListFavoriteDishes_Empty(t *testing.T) {
	user := createRandomUser(t)

	favorites, err := testStore.ListFavoriteDishes(context.Background(), ListFavoriteDishesParams{
		UserID: user.ID,
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Empty(t, favorites)
}

// ==================== RemoveFavoriteMerchant Tests ====================

func TestRemoveFavoriteMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	// 先收藏
	_ = addFavoriteMerchantHelper(t, user.ID, merchant.ID)

	// 验证已收藏
	isFav, err := testStore.IsMerchantFavorited(context.Background(), IsMerchantFavoritedParams{
		UserID:     user.ID,
		MerchantID: pgtype.Int8{Int64: merchant.ID, Valid: true},
	})
	require.NoError(t, err)
	require.True(t, isFav)

	// 取消收藏
	err = testStore.RemoveFavoriteMerchant(context.Background(), RemoveFavoriteMerchantParams{
		UserID:     user.ID,
		MerchantID: pgtype.Int8{Int64: merchant.ID, Valid: true},
	})
	require.NoError(t, err)

	// 验证已取消
	isFav, err = testStore.IsMerchantFavorited(context.Background(), IsMerchantFavoritedParams{
		UserID:     user.ID,
		MerchantID: pgtype.Int8{Int64: merchant.ID, Valid: true},
	})
	require.NoError(t, err)
	require.False(t, isFav)
}

func TestRemoveFavoriteMerchant_NotExist(t *testing.T) {
	user := createRandomUser(t)

	// 删除不存在的收藏（应该不报错）
	err := testStore.RemoveFavoriteMerchant(context.Background(), RemoveFavoriteMerchantParams{
		UserID:     user.ID,
		MerchantID: pgtype.Int8{Int64: 99999999, Valid: true},
	})
	require.NoError(t, err)
}

// ==================== RemoveFavoriteDish Tests ====================

func TestRemoveFavoriteDish(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	user := createRandomUser(t)

	// 先收藏
	_ = addFavoriteDishHelper(t, user.ID, dish.ID)

	// 验证已收藏
	isFav, err := testStore.IsDishFavorited(context.Background(), IsDishFavoritedParams{
		UserID: user.ID,
		DishID: pgtype.Int8{Int64: dish.ID, Valid: true},
	})
	require.NoError(t, err)
	require.True(t, isFav)

	// 取消收藏
	err = testStore.RemoveFavoriteDish(context.Background(), RemoveFavoriteDishParams{
		UserID: user.ID,
		DishID: pgtype.Int8{Int64: dish.ID, Valid: true},
	})
	require.NoError(t, err)

	// 验证已取消
	isFav, err = testStore.IsDishFavorited(context.Background(), IsDishFavoritedParams{
		UserID: user.ID,
		DishID: pgtype.Int8{Int64: dish.ID, Valid: true},
	})
	require.NoError(t, err)
	require.False(t, isFav)
}

// ==================== Edge Cases Tests ====================

func TestFavorite_MixedTypes(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)
	user := createRandomUser(t)

	// 同时收藏商户和菜品
	_ = addFavoriteMerchantHelper(t, user.ID, merchant.ID)
	_ = addFavoriteDishHelper(t, user.ID, dish.ID)

	// 商户收藏计数
	merchantCount, err := testStore.CountFavoriteMerchants(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), merchantCount)

	// 菜品收藏计数
	dishCount, err := testStore.CountFavoriteDishes(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), dishCount)
}

func TestFavorite_DifferentUsersCanFavoriteSameMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user1 := createRandomUser(t)
	user2 := createRandomUser(t)

	// 两个用户都收藏同一商户
	fav1 := addFavoriteMerchantHelper(t, user1.ID, merchant.ID)
	fav2 := addFavoriteMerchantHelper(t, user2.ID, merchant.ID)

	require.NotEqual(t, fav1.ID, fav2.ID)
	require.Equal(t, user1.ID, fav1.UserID)
	require.Equal(t, user2.ID, fav2.UserID)
}

func TestFavorite_ManyFavorites(t *testing.T) {
	user := createRandomUser(t)

	// 收藏多个商户
	for i := 0; i < 10; i++ {
		owner := createRandomUser(t)
		merchant := createRandomMerchantWithOwner(t, owner.ID)
		_ = addFavoriteMerchantHelper(t, user.ID, merchant.ID)
	}

	count, err := testStore.CountFavoriteMerchants(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(10), count)

	// 分页查询
	page1, err := testStore.ListFavoriteMerchants(context.Background(), ListFavoriteMerchantsParams{
		UserID: user.ID,
		Limit:  5,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, page1, 5)
}
