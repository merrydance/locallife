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
			ID:              10,
			Quantity:        2,
			DishID:          pgtype.Int8{Int64: 3, Valid: true},
			DishName:        pgtype.Text{String: "Dish", Valid: true},
			DishImageUrl:    pgtype.Text{String: "/img.png", Valid: true},
			DishPrice:       pgtype.Int8{Int64: 500, Valid: true},
			DishMemberPrice: pgtype.Int8{Int64: 450, Valid: true},
			DishIsAvailable: pgtype.Bool{Bool: true, Valid: true},
		},
	}

	resp := BuildCartResponse(cart, items, func(s string) string { return "norm:" + s })

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
	require.Equal(t, "norm:/img.png", item.ImageURL)
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
			ID:               10,
			Quantity:         1,
			ComboID:          pgtype.Int8{Int64: 7, Valid: true},
			ComboName:        pgtype.Text{String: "Combo", Valid: true},
			ComboImageUrl:    pgtype.Text{String: "/combo.png", Valid: true},
			ComboPrice:       pgtype.Int8{Int64: 1200, Valid: true},
			ComboIsAvailable: pgtype.Bool{Bool: false, Valid: true},
			Customizations:   blob,
		},
	}

	resp := BuildCartResponse(cart, items, func(s string) string { return s })

	require.Equal(t, 1, len(resp.Items))
	require.NotNil(t, resp.Items[0].Customizations)
	require.Equal(t, float64(1), resp.Items[0].Customizations["a"])
}
