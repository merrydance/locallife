package logic

import (
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestBuildCartResponse_DishItem(t *testing.T) {
	cart := db.Cart{
		ID:            1,
		MerchantID:    2,
		OrderType:     "takeout",
		TableID:       pgtype.Int8{Int64: 5, Valid: true},
		ReservationID: pgtype.Int8{Int64: 8, Valid: true},
	}

	items := []db.ListCartItemsRow{
		{
			ID:                    10,
			Quantity:              2,
			DishID:                pgtype.Int8{Int64: 3, Valid: true},
			DishName:              pgtype.Text{String: "Dish", Valid: true},
			DishImageMediaAssetID: pgtype.Int8{},
			DishPrice:             pgtype.Int8{Int64: 500, Valid: true},
			DishMemberPrice:       pgtype.Int8{Int64: 450, Valid: true},
			DishIsAvailable:       pgtype.Bool{Bool: true, Valid: true},
		},
	}

	resp := BuildCartResponse(cart, items)

	require.Equal(t, int64(1), resp.ID)
	require.Equal(t, int64(2), resp.MerchantID)
	require.Equal(t, "takeout", resp.OrderType)
	require.Equal(t, int64(5), *resp.TableID)
	require.Equal(t, int64(8), *resp.ReservationID)
	require.Equal(t, 1, len(resp.Items))
	require.Equal(t, 2, resp.TotalCount)
	require.Equal(t, int64(1000), resp.Subtotal)

	item := resp.Items[0]
	require.Equal(t, int64(10), item.ID)
	require.NotNil(t, item.DishID)
	require.Equal(t, int64(3), *item.DishID)
	require.Equal(t, "Dish", item.Name)
	require.Nil(t, item.ImageAssetID)
	require.Equal(t, int64(500), item.UnitPrice)
	require.NotNil(t, item.MemberPrice)
	require.Equal(t, int64(450), *item.MemberPrice)
	require.True(t, item.IsAvailable)
	require.Equal(t, int64(1000), item.Subtotal)
}

func TestBuildCartResponse_Customizations(t *testing.T) {
	custom := map[string]interface{}{"a": 1}
	blob, err := json.Marshal(custom)
	require.NoError(t, err)

	cart := db.Cart{ID: 1, MerchantID: 2, OrderType: "dine_in"}
	items := []db.ListCartItemsRow{
		{
			ID:                     10,
			Quantity:               1,
			ComboID:                pgtype.Int8{Int64: 7, Valid: true},
			ComboName:              pgtype.Text{String: "Combo", Valid: true},
			ComboImageMediaAssetID: pgtype.Int8{},
			ComboPrice:             pgtype.Int8{Int64: 1200, Valid: true},
			ComboIsAvailable:       pgtype.Bool{Bool: false, Valid: true},
			Customizations:         blob,
		},
	}

	resp := BuildCartResponse(cart, items)

	require.Equal(t, 1, len(resp.Items))
	require.NotNil(t, resp.Items[0].Customizations)
	require.Equal(t, float64(1), resp.Items[0].Customizations["a"])
}

func TestBuildCartResponse_HidesLegacyPackagingDishWhenRequested(t *testing.T) {
	cart := db.Cart{ID: 1, MerchantID: 2, OrderType: "takeout"}
	items := []db.ListCartItemsRow{
		{
			ID:              10,
			Quantity:        1,
			DishID:          pgtype.Int8{Int64: 3, Valid: true},
			DishName:        pgtype.Text{String: "旧餐盒", Valid: true},
			DishPrice:       pgtype.Int8{Int64: 100, Valid: true},
			DishIsAvailable: pgtype.Bool{Bool: true, Valid: true},
			DishIsPackaging: pgtype.Bool{Bool: true, Valid: true},
		},
		{
			ID:              11,
			Quantity:        2,
			DishID:          pgtype.Int8{Int64: 4, Valid: true},
			DishName:        pgtype.Text{String: "牛肉饭", Valid: true},
			DishPrice:       pgtype.Int8{Int64: 1500, Valid: true},
			DishIsAvailable: pgtype.Bool{Bool: true, Valid: true},
		},
	}

	resp := BuildCartResponse(cart, items, CartResponseOptions{HideLegacyPackagingDishes: true})

	require.Len(t, resp.Items, 1)
	require.Equal(t, int64(11), resp.Items[0].ID)
	require.Equal(t, "牛肉饭", resp.Items[0].Name)
	require.Equal(t, 2, resp.TotalCount)
	require.Equal(t, int64(3000), resp.Subtotal)
}

func TestBuildCartResponse_HidesComboWithLegacyPackagingChildWhenRequested(t *testing.T) {
	cart := db.Cart{ID: 1, MerchantID: 2, OrderType: "takeout"}
	items := []db.ListCartItemsRow{
		{
			ID:               10,
			Quantity:         1,
			ComboID:          pgtype.Int8{Int64: 30, Valid: true},
			ComboName:        pgtype.Text{String: "含餐盒套餐", Valid: true},
			ComboPrice:       pgtype.Int8{Int64: 2500, Valid: true},
			ComboIsAvailable: pgtype.Bool{Bool: true, Valid: true},
		},
		{
			ID:               11,
			Quantity:         2,
			ComboID:          pgtype.Int8{Int64: 31, Valid: true},
			ComboName:        pgtype.Text{String: "牛肉饭套餐", Valid: true},
			ComboPrice:       pgtype.Int8{Int64: 1800, Valid: true},
			ComboIsAvailable: pgtype.Bool{Bool: true, Valid: true},
		},
	}

	resp := BuildCartResponse(cart, items, CartResponseOptions{
		HideLegacyPackagingDishes: true,
		LegacyPackagingComboIDs:   map[int64]bool{30: true},
	})

	require.Len(t, resp.Items, 1)
	require.Equal(t, int64(11), resp.Items[0].ID)
	require.Equal(t, "牛肉饭套餐", resp.Items[0].Name)
	require.Equal(t, 2, resp.TotalCount)
	require.Equal(t, int64(3600), resp.Subtotal)
}
