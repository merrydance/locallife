package logic

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// OrderItemInput defines an item for order calculation.
type OrderItemInput struct {
	DishID         *int64
	ComboID        *int64
	Quantity       int16
	Customizations map[string]interface{}
}

type CalculateOrderItemsOptions struct {
	RejectLegacyPackagingDishes bool
}

// NormalizeDishCustomizationsFunc normalizes customizations and returns JSON plus extra price.
type NormalizeDishCustomizationsFunc func(ctx context.Context, dishID int64, customizations map[string]interface{}) ([]byte, int64, error)

// CalculateOrderItems builds order items and computes subtotal.
func CalculateOrderItems(ctx context.Context, store db.Store, merchantID int64, items []OrderItemInput, normalize NormalizeDishCustomizationsFunc, options CalculateOrderItemsOptions) (int64, []db.CreateOrderItemParams, error) {
	var subtotal int64
	orderItems := make([]db.CreateOrderItemParams, 0, len(items))

	for _, item := range items {
		if item.DishID == nil && item.ComboID == nil {
			return 0, nil, fmt.Errorf("each item must have either dish_id or combo_id")
		}
		if item.DishID != nil && item.ComboID != nil {
			return 0, nil, fmt.Errorf("each item can only have one of dish_id or combo_id")
		}

		var name string
		var unitPrice int64
		var dishID, comboID pgtype.Int8
		var customizations []byte
		var extraPrice int64

		if item.DishID != nil {
			dish, err := store.GetDish(ctx, *item.DishID)
			if err != nil {
				if errors.Is(err, db.ErrRecordNotFound) {
					return 0, nil, fmt.Errorf("dish %d not found", *item.DishID)
				}
				return 0, nil, err
			}
			if dish.MerchantID != merchantID {
				return 0, nil, fmt.Errorf("dish %d does not belong to this merchant", *item.DishID)
			}
			if options.RejectLegacyPackagingDishes && dish.IsPackaging {
				return 0, nil, fmt.Errorf("包装已迁移到包装设置，请在包装设置中维护")
			}
			if !dish.IsOnline {
				return 0, nil, fmt.Errorf("dish %s is offline", dish.Name)
			}
			if !dish.IsAvailable {
				return 0, nil, fmt.Errorf("dish %s is not available today", dish.Name)
			}

			if normalize == nil {
				return 0, nil, fmt.Errorf("customizations handler is not configured")
			}
			customizations, extraPrice, err = normalize(ctx, dish.ID, item.Customizations)
			if err != nil {
				return 0, nil, err
			}

			name = dish.Name
			unitPrice = dish.Price
			dishID = pgtype.Int8{Int64: *item.DishID, Valid: true}
		} else if item.ComboID != nil {
			if len(item.Customizations) > 0 {
				return 0, nil, fmt.Errorf("customizations only supported for dish items")
			}
			combo, err := store.GetComboSet(ctx, *item.ComboID)
			if err != nil {
				if errors.Is(err, db.ErrRecordNotFound) {
					return 0, nil, fmt.Errorf("combo %d not found", *item.ComboID)
				}
				return 0, nil, err
			}
			if combo.MerchantID != merchantID {
				return 0, nil, fmt.Errorf("combo %d does not belong to this merchant", *item.ComboID)
			}
			if !combo.IsOnline {
				return 0, nil, fmt.Errorf("combo %s is offline", combo.Name)
			}
			if err := validateComboChildDishesOrderable(ctx, store, combo.ID, combo.Name, options.RejectLegacyPackagingDishes); err != nil {
				return 0, nil, err
			}

			name = combo.Name
			unitPrice = combo.ComboPrice
			comboID = pgtype.Int8{Int64: *item.ComboID, Valid: true}
		}

		unitPrice += extraPrice
		itemSubtotal := unitPrice * int64(item.Quantity)
		subtotal += itemSubtotal

		orderItems = append(orderItems, db.CreateOrderItemParams{
			DishID:         dishID,
			ComboID:        comboID,
			Name:           name,
			UnitPrice:      unitPrice,
			Quantity:       item.Quantity,
			Subtotal:       itemSubtotal,
			Customizations: customizations,
		})
	}

	return subtotal, orderItems, nil
}
