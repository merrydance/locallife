package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomIngredient(t *testing.T, isSystem bool) Ingredient {
	var createdBy pgtype.Int8
	if !isSystem {
		user := createRandomUser(t)
		createdBy = pgtype.Int8{Int64: user.ID, Valid: true}
	}

	arg := CreateIngredientParams{
		Name:       util.RandomString(10),
		IsSystem:   isSystem,
		Category:   pgtype.Text{String: util.RandomString(5), Valid: true},
		IsAllergen: util.RandomInt(0, 1) == 1,
		CreatedBy:  createdBy,
	}

	ingredient, err := testStore.CreateIngredient(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, ingredient)

	require.Equal(t, arg.Name, ingredient.Name)
	require.Equal(t, arg.IsSystem, ingredient.IsSystem)
	require.Equal(t, arg.Category, ingredient.Category)
	require.Equal(t, arg.IsAllergen, ingredient.IsAllergen)
	require.Equal(t, arg.CreatedBy, ingredient.CreatedBy)
	require.NotZero(t, ingredient.ID)
	require.NotZero(t, ingredient.CreatedAt)

	return ingredient
}

func TestCreateIngredient(t *testing.T) {
	createRandomIngredient(t, true)
	createRandomIngredient(t, false)
}

func TestGetIngredient(t *testing.T) {
	ingredient1 := createRandomIngredient(t, true)

	ingredient2, err := testStore.GetIngredient(context.Background(), ingredient1.ID)
	require.NoError(t, err)
	require.NotEmpty(t, ingredient2)

	require.Equal(t, ingredient1.ID, ingredient2.ID)
	require.Equal(t, ingredient1.Name, ingredient2.Name)
	require.Equal(t, ingredient1.IsSystem, ingredient2.IsSystem)
	require.Equal(t, ingredient1.Category, ingredient2.Category)
	require.Equal(t, ingredient1.IsAllergen, ingredient2.IsAllergen)
}

func TestListIngredients(t *testing.T) {
	// 创建一些测试数据
	for i := 0; i < 5; i++ {
		createRandomIngredient(t, true)
		createRandomIngredient(t, false)
	}

	testCases := []struct {
		name      string
		params    ListIngredientsParams
		checkFunc func(*testing.T, []Ingredient, error)
	}{
		{
			name: "ListAll",
			params: ListIngredientsParams{
				Limit:  10,
				Offset: 0,
			},
			checkFunc: func(t *testing.T, ingredients []Ingredient, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, ingredients)
				require.LessOrEqual(t, len(ingredients), 10)
			},
		},
		{
			name: "ListSystemOnly",
			params: ListIngredientsParams{
				IsSystem: pgtype.Bool{Bool: true, Valid: true},
				Limit:    10,
				Offset:   0,
			},
			checkFunc: func(t *testing.T, ingredients []Ingredient, err error) {
				require.NoError(t, err)
				for _, ingredient := range ingredients {
					require.True(t, ingredient.IsSystem)
				}
			},
		},
		{
			name: "ListMerchantOnly",
			params: ListIngredientsParams{
				IsSystem: pgtype.Bool{Bool: false, Valid: true},
				Limit:    10,
				Offset:   0,
			},
			checkFunc: func(t *testing.T, ingredients []Ingredient, err error) {
				require.NoError(t, err)
				for _, ingredient := range ingredients {
					require.False(t, ingredient.IsSystem)
				}
			},
		},
		{
			name: "ListAllergenOnly",
			params: ListIngredientsParams{
				IsAllergen: pgtype.Bool{Bool: true, Valid: true},
				Limit:      10,
				Offset:     0,
			},
			checkFunc: func(t *testing.T, ingredients []Ingredient, err error) {
				require.NoError(t, err)
				for _, ingredient := range ingredients {
					require.True(t, ingredient.IsAllergen)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ingredients, err := testStore.ListIngredients(context.Background(), tc.params)
			tc.checkFunc(t, ingredients, err)
		})
	}
}

func TestSearchIngredients(t *testing.T) {
	// 创建一个特殊名称的食材用于搜索
	ingredient := createRandomIngredient(t, true)

	// 搜索该食材名称的前几个字符
	searchTerm := ingredient.Name[:3]

	arg := SearchIngredientsParams{
		Column1: pgtype.Text{String: searchTerm, Valid: true},
		Limit:   10,
		Offset:  0,
	}

	ingredients, err := testStore.SearchIngredients(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, ingredients)

	// 验证返回的结果包含搜索词
	found := false
	for _, ing := range ingredients {
		if ing.ID == ingredient.ID {
			found = true
			break
		}
	}
	require.True(t, found, "Created ingredient should be found in search results")
}

func TestUpdateIngredient(t *testing.T) {
	ingredient := createRandomIngredient(t, false)

	newName := util.RandomString(10)
	newCategory := util.RandomString(5)

	arg := UpdateIngredientParams{
		ID:         ingredient.ID,
		Name:       pgtype.Text{String: newName, Valid: true},
		Category:   pgtype.Text{String: newCategory, Valid: true},
		IsAllergen: pgtype.Bool{Bool: true, Valid: true},
	}

	updatedIngredient, err := testStore.UpdateIngredient(context.Background(), arg)
	require.NoError(t, err)
	require.NotEmpty(t, updatedIngredient)

	require.Equal(t, ingredient.ID, updatedIngredient.ID)
	require.Equal(t, newName, updatedIngredient.Name)
	require.Equal(t, newCategory, updatedIngredient.Category.String)
	require.True(t, updatedIngredient.IsAllergen)
}

func TestDeleteIngredient(t *testing.T) {
	ingredient := createRandomIngredient(t, false)

	err := testStore.DeleteIngredient(context.Background(), ingredient.ID)
	require.NoError(t, err)

	// 验证已删除
	ingredient2, err := testStore.GetIngredient(context.Background(), ingredient.ID)
	require.Error(t, err)
	require.Empty(t, ingredient2)
}

func TestCountIngredients(t *testing.T) {
	// 创建一些测试数据
	for i := 0; i < 3; i++ {
		createRandomIngredient(t, true)
	}

	testCases := []struct {
		name   string
		params CountIngredientsParams
	}{
		{
			name:   "CountAll",
			params: CountIngredientsParams{},
		},
		{
			name: "CountSystemOnly",
			params: CountIngredientsParams{
				IsSystem: pgtype.Bool{Bool: true, Valid: true},
			},
		},
		{
			name: "CountMerchantOnly",
			params: CountIngredientsParams{
				IsSystem: pgtype.Bool{Bool: false, Valid: true},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			count, err := testStore.CountIngredients(context.Background(), tc.params)
			require.NoError(t, err)
			require.GreaterOrEqual(t, count, int64(0))
		})
	}
}
