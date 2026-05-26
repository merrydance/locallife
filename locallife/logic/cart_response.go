package logic

import (
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// CartItemResponse describes cart item fields needed by API responses.
type CartItemResponse struct {
	ID             int64                  `json:"id"`
	DishID         *int64                 `json:"dish_id,omitempty"`
	ComboID        *int64                 `json:"combo_id,omitempty"`
	Quantity       int16                  `json:"quantity"`
	Customizations map[string]interface{} `json:"customizations,omitempty"`
	Name           string                 `json:"name"`
	ImageAssetID   *int64                 `json:"image_asset_id,omitempty"`
	UnitPrice      int64                  `json:"unit_price"`
	MemberPrice    *int64                 `json:"member_price,omitempty"`
	IsAvailable    bool                   `json:"is_available"`
	IsPackaging    bool                   `json:"is_packaging"`
	Subtotal       int64                  `json:"subtotal"`
}

// CartResponse summarizes cart items for API responses.
type CartResponse struct {
	ID            int64              `json:"id"`
	MerchantID    int64              `json:"merchant_id"`
	OrderType     string             `json:"order_type"`
	TableID       *int64             `json:"table_id,omitempty"`
	ReservationID *int64             `json:"reservation_id,omitempty"`
	Items         []CartItemResponse `json:"items"`
	TotalCount    int                `json:"total_count"`
	Subtotal      int64              `json:"subtotal"`
}

// BuildCartResponse converts cart + item rows into response-friendly structures.
func BuildCartResponse(cart db.Cart, items []db.ListCartItemsRow) CartResponse {
	cartItems := make([]CartItemResponse, 0, len(items))
	var subtotal int64
	var totalCount int

	for _, item := range items {
		var unitPrice int64
		var name string
		var imageAssetID *int64
		var memberPrice *int64
		var isAvailable bool

		if item.DishID.Valid {
			name = item.DishName.String
			if item.DishImageMediaAssetID.Valid {
				v := item.DishImageMediaAssetID.Int64
				imageAssetID = &v
			}
			unitPrice = item.DishPrice.Int64
			if item.DishMemberPrice.Valid {
				memberPrice = &item.DishMemberPrice.Int64
			}
			isAvailable = item.DishIsAvailable.Bool
		} else if item.ComboID.Valid {
			name = item.ComboName.String
			if item.ComboImageMediaAssetID.Valid {
				v := item.ComboImageMediaAssetID.Int64
				imageAssetID = &v
			}
			unitPrice = item.ComboPrice.Int64
			isAvailable = item.ComboIsAvailable.Bool
		}

		isPackaging := false
		if item.DishID.Valid {
			isPackaging = item.DishIsPackaging.Bool
		}

		itemSubtotal := unitPrice * int64(item.Quantity)
		subtotal += itemSubtotal
		totalCount += int(item.Quantity)

		cartItem := CartItemResponse{
			ID:           item.ID,
			Quantity:     item.Quantity,
			Name:         name,
			ImageAssetID: imageAssetID,
			UnitPrice:    unitPrice,
			MemberPrice:  memberPrice,
			IsAvailable:  isAvailable,
			IsPackaging:  isPackaging,
			Subtotal:     itemSubtotal,
		}

		if item.DishID.Valid {
			dishID := item.DishID.Int64
			cartItem.DishID = &dishID
		}
		if item.ComboID.Valid {
			comboID := item.ComboID.Int64
			cartItem.ComboID = &comboID
		}

		if len(item.Customizations) > 0 {
			var customizations map[string]interface{}
			if err := json.Unmarshal(item.Customizations, &customizations); err == nil {
				cartItem.Customizations = customizations
			}
		}

		cartItems = append(cartItems, cartItem)
	}

	return CartResponse{
		ID:            cart.ID,
		MerchantID:    cart.MerchantID,
		OrderType:     cart.OrderType,
		TableID:       nullableInt64(cart.TableID),
		ReservationID: nullableInt64(cart.ReservationID),
		Items:         cartItems,
		TotalCount:    totalCount,
		Subtotal:      subtotal,
	}
}

func nullableInt64(v pgtype.Int8) *int64 {
	if v.Valid {
		return &v.Int64
	}
	return nil
}
