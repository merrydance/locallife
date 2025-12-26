package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// ==================== Helper Functions ====================

// createRandomCart 创建一个随机的购物车
func createRandomCart(t *testing.T, userID, merchantID int64) Cart {
	arg := CreateCartParams{
		UserID:     userID,
		MerchantID: merchantID,
	}

	cart, err := testStore.CreateCart(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, cart.ID)
	require.Equal(t, userID, cart.UserID)
	require.Equal(t, merchantID, cart.MerchantID)

	return cart
}

// createRandomCartItem 创建一个随机的购物车项目（菜品）
func createRandomCartItem(t *testing.T, cartID int64, dish Dish) CartItem {
	customizations := []byte(`{"spicy": "medium"}`)

	arg := AddCartItemParams{
		CartID:         cartID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:       2,
		Customizations: customizations,
	}

	item, err := testStore.AddCartItem(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, item.ID)

	return item
}

// createRandomCartItemWithCombo 创建一个随机的购物车项目（套餐）
func createRandomCartItemWithCombo(t *testing.T, cartID int64, comboID int64) CartItem {
	arg := AddCartItemParams{
		CartID:         cartID,
		ComboID:        pgtype.Int8{Int64: comboID, Valid: true},
		Quantity:       1,
		Customizations: []byte(`{}`),
	}

	item, err := testStore.AddCartItem(context.Background(), arg)
	require.NoError(t, err)
	require.NotZero(t, item.ID)

	return item
}

// ==================== CreateCart Tests ====================

func TestCreateCart(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	cart := createRandomCart(t, user.ID, merchant.ID)

	require.Equal(t, user.ID, cart.UserID)
	require.Equal(t, merchant.ID, cart.MerchantID)
	require.NotZero(t, cart.CreatedAt)
	require.NotZero(t, cart.UpdatedAt)
}

func TestCreateCart_UpsertSameUserMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	// 创建第一个购物车
	cart1 := createRandomCart(t, user.ID, merchant.ID)

	// 再次创建相同的用户-商户购物车（应该更新而不是创建新的）
	cart2, err := testStore.CreateCart(context.Background(), CreateCartParams{
		UserID:     user.ID,
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, cart1.ID, cart2.ID) // 应该是同一个购物车
}

func TestCreateCart_DifferentMerchants(t *testing.T) {
	owner1 := createRandomUser(t)
	owner2 := createRandomUser(t)
	merchant1 := createRandomMerchantWithOwner(t, owner1.ID)
	merchant2 := createRandomMerchantWithOwner(t, owner2.ID)
	user := createRandomUser(t)

	// 同一用户在不同商户创建购物车
	cart1 := createRandomCart(t, user.ID, merchant1.ID)
	cart2 := createRandomCart(t, user.ID, merchant2.ID)

	require.NotEqual(t, cart1.ID, cart2.ID)
	require.Equal(t, merchant1.ID, cart1.MerchantID)
	require.Equal(t, merchant2.ID, cart2.MerchantID)
}

// ==================== GetCartByUserAndMerchant Tests ====================

func TestGetCartByUserAndMerchant(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	created := createRandomCart(t, user.ID, merchant.ID)

	got, err := testStore.GetCartByUserAndMerchant(context.Background(), GetCartByUserAndMerchantParams{
		UserID:     user.ID,
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, created.UserID, got.UserID)
	require.Equal(t, created.MerchantID, got.MerchantID)
}

func TestGetCartByUserAndMerchant_NotFound(t *testing.T) {
	_, err := testStore.GetCartByUserAndMerchant(context.Background(), GetCartByUserAndMerchantParams{
		UserID:     99999999,
		MerchantID: 99999999,
	})
	require.Error(t, err)
}

// ==================== AddCartItem Tests ====================

func TestAddCartItem_WithDish(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)
	item := createRandomCartItem(t, cart.ID, dish)

	require.Equal(t, cart.ID, item.CartID)
	require.True(t, item.DishID.Valid)
	require.Equal(t, dish.ID, item.DishID.Int64)
	require.False(t, item.ComboID.Valid)
	require.Equal(t, int16(2), item.Quantity)
}

func TestAddCartItem_WithCombo(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	// 创建套餐
	combo, err := testStore.CreateComboSet(context.Background(), CreateComboSetParams{
		MerchantID:    merchant.ID,
		Name:          "测试套餐",
		Description:   pgtype.Text{String: "套餐描述", Valid: true},
		OriginalPrice: 5000,
		ComboPrice:    4000,
		IsOnline:      true,
	})
	require.NoError(t, err)

	cart := createRandomCart(t, user.ID, merchant.ID)
	item := createRandomCartItemWithCombo(t, cart.ID, combo.ID)

	require.Equal(t, cart.ID, item.CartID)
	require.False(t, item.DishID.Valid)
	require.True(t, item.ComboID.Valid)
	require.Equal(t, combo.ID, item.ComboID.Int64)
	require.Equal(t, int16(1), item.Quantity)
}

func TestAddCartItem_WithCustomizations(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)

	customizations := map[string]interface{}{
		"spicy":  "extra_hot",
		"noodle": "thick",
		"extras": []string{"egg", "green_onion"},
	}
	customizationsJSON, _ := json.Marshal(customizations)

	arg := AddCartItemParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:       3,
		Customizations: customizationsJSON,
	}

	item, err := testStore.AddCartItem(context.Background(), arg)
	require.NoError(t, err)

	// 解析并比较 JSON 内容（避免字段顺序问题）
	var savedCustomizations, expectedCustomizations map[string]interface{}
	err = json.Unmarshal(item.Customizations, &savedCustomizations)
	require.NoError(t, err)
	err = json.Unmarshal(customizationsJSON, &expectedCustomizations)
	require.NoError(t, err)
	require.Equal(t, expectedCustomizations, savedCustomizations)
}

// ==================== GetCartItem Tests ====================

func TestGetCartItem(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)
	created := createRandomCartItem(t, cart.ID, dish)

	got, err := testStore.GetCartItem(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, created.CartID, got.CartID)
	require.True(t, got.DishName.Valid)
	require.Equal(t, dish.Name, got.DishName.String)
	require.True(t, got.DishPrice.Valid)
	require.Equal(t, dish.Price, got.DishPrice.Int64)
}

func TestGetCartItem_NotFound(t *testing.T) {
	_, err := testStore.GetCartItem(context.Background(), 99999999)
	require.Error(t, err)
}

// ==================== GetCartItemByDishAndCustomizations Tests ====================

func TestGetCartItemByDishAndCustomizations(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)

	customizations := []byte(`{"spicy": "medium"}`)
	arg := AddCartItemParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:       2,
		Customizations: customizations,
	}
	created, err := testStore.AddCartItem(context.Background(), arg)
	require.NoError(t, err)

	// 根据菜品和自定义选项查找
	got, err := testStore.GetCartItemByDishAndCustomizations(context.Background(), GetCartItemByDishAndCustomizationsParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		Customizations: customizations,
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
}

func TestGetCartItemByDishAndCustomizations_DifferentCustomizations(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)

	// 创建带不同自定义选项的购物车项目
	customizations1 := []byte(`{"spicy": "mild"}`)
	customizations2 := []byte(`{"spicy": "hot"}`)

	_, err := testStore.AddCartItem(context.Background(), AddCartItemParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:       1,
		Customizations: customizations1,
	})
	require.NoError(t, err)

	_, err = testStore.AddCartItem(context.Background(), AddCartItemParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:       2,
		Customizations: customizations2,
	})
	require.NoError(t, err)

	// 查找第一个自定义选项
	got, err := testStore.GetCartItemByDishAndCustomizations(context.Background(), GetCartItemByDishAndCustomizationsParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		Customizations: customizations1,
	})
	require.NoError(t, err)
	require.Equal(t, int16(1), got.Quantity)
}

// ==================== GetCartItemByCombo Tests ====================

func TestGetCartItemByCombo(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	combo, err := testStore.CreateComboSet(context.Background(), CreateComboSetParams{
		MerchantID:    merchant.ID,
		Name:          "查询测试套餐",
		OriginalPrice: 6000,
		ComboPrice:    5000,
		IsOnline:      true,
	})
	require.NoError(t, err)

	cart := createRandomCart(t, user.ID, merchant.ID)
	created := createRandomCartItemWithCombo(t, cart.ID, combo.ID)

	got, err := testStore.GetCartItemByCombo(context.Background(), GetCartItemByComboParams{
		CartID:  cart.ID,
		ComboID: pgtype.Int8{Int64: combo.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
}

// ==================== ListCartItems Tests ====================

func TestListCartItems(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)

	// 添加多个项目
	_, err := testStore.AddCartItem(context.Background(), AddCartItemParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish1.ID, Valid: true},
		Quantity:       1,
		Customizations: []byte(`{}`),
	})
	require.NoError(t, err)

	_, err = testStore.AddCartItem(context.Background(), AddCartItemParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish2.ID, Valid: true},
		Quantity:       2,
		Customizations: []byte(`{}`),
	})
	require.NoError(t, err)

	items, err := testStore.ListCartItems(context.Background(), cart.ID)
	require.NoError(t, err)
	require.Len(t, items, 2)

	// 验证返回了菜品信息
	for _, item := range items {
		require.True(t, item.DishName.Valid)
		require.True(t, item.DishPrice.Valid)
	}
}

func TestListCartItems_EmptyCart(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	cart := createRandomCart(t, user.ID, merchant.ID)

	items, err := testStore.ListCartItems(context.Background(), cart.ID)
	require.NoError(t, err)
	require.Empty(t, items)
}

// ==================== UpdateCartItem Tests ====================

func TestUpdateCartItem_UpdateQuantity(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)
	item := createRandomCartItem(t, cart.ID, dish)

	// 更新数量
	updated, err := testStore.UpdateCartItem(context.Background(), UpdateCartItemParams{
		ID:       item.ID,
		Quantity: pgtype.Int2{Int16: 5, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int16(5), updated.Quantity)
	require.Equal(t, item.Customizations, updated.Customizations) // 自定义选项不变
}

func TestUpdateCartItem_UpdateCustomizations(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)
	item := createRandomCartItem(t, cart.ID, dish)

	// 更新自定义选项
	newCustomizations := []byte(`{"spicy": "extra_hot", "no_cilantro": true}`)
	updated, err := testStore.UpdateCartItem(context.Background(), UpdateCartItemParams{
		ID:             item.ID,
		Customizations: newCustomizations,
	})
	require.NoError(t, err)
	require.Equal(t, newCustomizations, updated.Customizations)
	require.Equal(t, item.Quantity, updated.Quantity) // 数量不变
}

func TestUpdateCartItem_ZeroQuantity(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)
	item := createRandomCartItem(t, cart.ID, dish)

	// 更新数量为0（边界测试）
	updated, err := testStore.UpdateCartItem(context.Background(), UpdateCartItemParams{
		ID:       item.ID,
		Quantity: pgtype.Int2{Int16: 0, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int16(0), updated.Quantity)
}

// ==================== DeleteCartItem Tests ====================

func TestDeleteCartItem(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)
	item := createRandomCartItem(t, cart.ID, dish)

	err := testStore.DeleteCartItem(context.Background(), item.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = testStore.GetCartItem(context.Background(), item.ID)
	require.Error(t, err)
}

// ==================== ClearCart Tests ====================

func TestClearCart(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant.ID, category.ID)
	dish2 := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)

	// 添加多个项目
	_, _ = testStore.AddCartItem(context.Background(), AddCartItemParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish1.ID, Valid: true},
		Quantity:       1,
		Customizations: []byte(`{}`),
	})
	_, _ = testStore.AddCartItem(context.Background(), AddCartItemParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish2.ID, Valid: true},
		Quantity:       2,
		Customizations: []byte(`{}`),
	})

	// 验证有项目
	items, err := testStore.ListCartItems(context.Background(), cart.ID)
	require.NoError(t, err)
	require.Len(t, items, 2)

	// 清空购物车
	err = testStore.ClearCart(context.Background(), cart.ID)
	require.NoError(t, err)

	// 验证已清空
	items, err = testStore.ListCartItems(context.Background(), cart.ID)
	require.NoError(t, err)
	require.Empty(t, items)

	// 购物车本身仍然存在
	_, err = testStore.GetCartByUserAndMerchant(context.Background(), GetCartByUserAndMerchantParams{
		UserID:     user.ID,
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)
}

// ==================== DeleteCart Tests ====================

func TestDeleteCart(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	cart := createRandomCart(t, user.ID, merchant.ID)

	err := testStore.DeleteCart(context.Background(), cart.ID)
	require.NoError(t, err)

	// 验证已删除
	_, err = testStore.GetCartByUserAndMerchant(context.Background(), GetCartByUserAndMerchantParams{
		UserID:     user.ID,
		MerchantID: merchant.ID,
	})
	require.Error(t, err)
}

// ==================== GetCartWithItems Tests ====================

func TestGetCartWithItems(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)
	_ = createRandomCartItem(t, cart.ID, dish)

	result, err := testStore.GetCartWithItems(context.Background(), GetCartWithItemsParams{
		UserID:     user.ID,
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)
	require.Equal(t, cart.ID, result.ID)
	require.NotNil(t, result.Items)

	// 解析 Items JSON
	var items []map[string]interface{}
	err = json.Unmarshal(result.Items, &items)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, dish.Name, items[0]["dish_name"])
}

func TestGetCartWithItems_EmptyCart(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	_ = createRandomCart(t, user.ID, merchant.ID)

	result, err := testStore.GetCartWithItems(context.Background(), GetCartWithItemsParams{
		UserID:     user.ID,
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)

	// 空购物车应该返回空数组
	var items []map[string]interface{}
	err = json.Unmarshal(result.Items, &items)
	require.NoError(t, err)
	require.Empty(t, items)
}

// ==================== GetUserCarts Tests ====================

func TestGetUserCarts(t *testing.T) {
	owner1 := createRandomUser(t)
	owner2 := createRandomUser(t)
	merchant1 := createRandomMerchantWithOwner(t, owner1.ID)
	merchant2 := createRandomMerchantWithOwner(t, owner2.ID)
	user := createRandomUser(t)
	category1 := createRandomDishCategory(t)
	dish1 := createRandomDish(t, merchant1.ID, category1.ID)

	// 创建两个购物车
	cart1 := createRandomCart(t, user.ID, merchant1.ID)
	_ = createRandomCart(t, user.ID, merchant2.ID)

	// 在第一个购物车添加项目
	_ = createRandomCartItem(t, cart1.ID, dish1)

	carts, err := testStore.GetUserCarts(context.Background(), user.ID)
	require.NoError(t, err)
	require.Len(t, carts, 2)

	// 验证返回了商户信息
	for _, cart := range carts {
		require.NotEmpty(t, cart.MerchantName)
	}
}

func TestGetUserCarts_NoCart(t *testing.T) {
	user := createRandomUser(t)

	carts, err := testStore.GetUserCarts(context.Background(), user.ID)
	require.NoError(t, err)
	require.Empty(t, carts)
}

// ==================== Edge Cases Tests ====================

func TestCart_LargeQuantity(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)

	// 测试大数量（int16 最大值 32767）
	arg := AddCartItemParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:       999,
		Customizations: []byte(`{}`),
	}

	item, err := testStore.AddCartItem(context.Background(), arg)
	require.NoError(t, err)
	require.Equal(t, int16(999), item.Quantity)
}

func TestCart_ComplexCustomizations(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)

	// 复杂的自定义选项
	customizations := map[string]interface{}{
		"spicy_level":    5,
		"no_ingredients": []string{"cilantro", "onion", "garlic"},
		"extra": map[string]interface{}{
			"egg":      2,
			"noodle":   "thick",
			"add_meat": true,
		},
		"note": "请多加辣椒，不要放香菜和葱",
	}
	customizationsJSON, _ := json.Marshal(customizations)

	arg := AddCartItemParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:       1,
		Customizations: customizationsJSON,
	}

	item, err := testStore.AddCartItem(context.Background(), arg)
	require.NoError(t, err)

	// 验证可以正确解析
	var parsedCustomizations map[string]interface{}
	err = json.Unmarshal(item.Customizations, &parsedCustomizations)
	require.NoError(t, err)
	require.Equal(t, float64(5), parsedCustomizations["spicy_level"])
}

func TestCart_MultipleItemsSameDish(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)
	category := createRandomDishCategory(t)
	dish := createRandomDish(t, merchant.ID, category.ID)

	cart := createRandomCart(t, user.ID, merchant.ID)

	// 同一菜品不同自定义选项
	_, err := testStore.AddCartItem(context.Background(), AddCartItemParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:       1,
		Customizations: []byte(`{"spicy": "mild"}`),
	})
	require.NoError(t, err)

	_, err = testStore.AddCartItem(context.Background(), AddCartItemParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:       2,
		Customizations: []byte(`{"spicy": "hot"}`),
	})
	require.NoError(t, err)

	_, err = testStore.AddCartItem(context.Background(), AddCartItemParams{
		CartID:         cart.ID,
		DishID:         pgtype.Int8{Int64: dish.ID, Valid: true},
		Quantity:       3,
		Customizations: []byte(`{"spicy": "extra_hot"}`),
	})
	require.NoError(t, err)

	items, err := testStore.ListCartItems(context.Background(), cart.ID)
	require.NoError(t, err)
	require.Len(t, items, 3) // 三个不同的项目
}
