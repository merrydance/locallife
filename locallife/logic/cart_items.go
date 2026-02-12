package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

// NormalizeCustomizationsFunc normalizes dish customizations for cart items.
type NormalizeCustomizationsFunc func(ctx context.Context, dishID int64, customizations map[string]interface{}) (map[string]interface{}, error)

// AddCartItemInput defines cart item creation parameters.
type AddCartItemInput struct {
	UserID                int64
	MerchantID            int64
	OrderType             string
	TableID               *int64
	ReservationID         *int64
	DishID                *int64
	ComboID               *int64
	Quantity              int16
	Customizations        map[string]interface{}
	MaxQuantity           int16
	NormalizeCustomizings NormalizeCustomizationsFunc
}

// AddCartItemResult returns the cart updated by the add flow.
type AddCartItemResult struct {
	Cart db.Cart
}

// UpdateCartItemInput defines cart item update parameters.
type UpdateCartItemInput struct {
	UserID         int64
	ItemID         int64
	Quantity       *int16
	Customizations map[string]interface{}
	MaxQuantity    int16
}

// UpdateCartItemResult returns the cart updated by the update flow.
type UpdateCartItemResult struct {
	Cart db.Cart
}

// AddCartItem validates and inserts or increments a cart item.
func AddCartItem(ctx context.Context, store db.Store, input AddCartItemInput) (AddCartItemResult, error) {
	var result AddCartItemResult

	if input.DishID == nil && input.ComboID == nil {
		return result, NewRequestError(http.StatusBadRequest, errors.New("dish_id or combo_id is required"))
	}
	if input.DishID != nil && input.ComboID != nil {
		return result, NewRequestError(http.StatusBadRequest, errors.New("cannot specify both dish_id and combo_id"))
	}

	if input.OrderType == "" {
		input.OrderType = "takeout"
	}

	if input.Quantity < 1 || (input.MaxQuantity > 0 && input.Quantity > input.MaxQuantity) {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("quantity must be between 1 and %d", input.MaxQuantity))
	}

	if input.DishID != nil && input.NormalizeCustomizings != nil {
		normalized, err := input.NormalizeCustomizings(ctx, *input.DishID, input.Customizations)
		if err != nil {
			return result, NewRequestError(http.StatusBadRequest, err)
		}
		input.Customizations = normalized
	}
	if input.ComboID != nil && len(input.Customizations) > 0 {
		return result, NewRequestError(http.StatusBadRequest, errors.New("customizations not supported for combo items"))
	}

	merchant, err := store.GetMerchant(ctx, input.MerchantID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("merchant not found"))
		}
		return result, err
	}
	if input.OrderType == "takeout" {
		if merchant.Status != "active" || !merchant.IsOpen {
			return result, NewRequestError(http.StatusBadRequest, errors.New("merchant is not accepting takeout orders"))
		}
	}

	if input.DishID != nil {
		dish, err := store.GetDish(ctx, *input.DishID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return result, NewRequestError(http.StatusNotFound, errors.New("dish not found"))
			}
			return result, err
		}
		if dish.MerchantID != input.MerchantID {
			return result, NewRequestError(http.StatusBadRequest, errors.New("dish does not belong to this merchant"))
		}
		if !dish.IsOnline || !dish.IsAvailable {
			return result, NewRequestError(http.StatusBadRequest, errors.New("dish is not available"))
		}
	}

	if input.ComboID != nil {
		combo, err := store.GetComboSet(ctx, *input.ComboID)
		if err != nil {
			if errors.Is(err, db.ErrRecordNotFound) {
				return result, NewRequestError(http.StatusNotFound, errors.New("combo not found"))
			}
			return result, err
		}
		if combo.MerchantID != input.MerchantID {
			return result, NewRequestError(http.StatusBadRequest, errors.New("combo does not belong to this merchant"))
		}
		if !combo.IsOnline {
			return result, NewRequestError(http.StatusBadRequest, errors.New("combo is not available"))
		}
	}

	var tableID, reservationID pgtype.Int8
	if input.TableID != nil {
		tableID = pgtype.Int8{Int64: *input.TableID, Valid: true}
	}
	if input.ReservationID != nil {
		reservationID = pgtype.Int8{Int64: *input.ReservationID, Valid: true}
	}

	cart, err := store.GetCartByUserAndMerchant(ctx, db.GetCartByUserAndMerchantParams{
		UserID:        input.UserID,
		MerchantID:    input.MerchantID,
		OrderType:     input.OrderType,
		TableID:       tableID,
		ReservationID: reservationID,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			cart, err = store.CreateCart(ctx, db.CreateCartParams{
				UserID:        input.UserID,
				MerchantID:    input.MerchantID,
				OrderType:     input.OrderType,
				TableID:       tableID,
				ReservationID: reservationID,
			})
			if err != nil {
				return result, err
			}
		} else {
			return result, err
		}
	}

	customizations, err := MarshalCustomizationsCanonical(input.Customizations)
	if err != nil {
		return result, NewRequestError(http.StatusBadRequest, errors.New("invalid customizations"))
	}

	if input.DishID != nil {
		existingItem, err := store.GetCartItemByDishAndCustomizations(ctx, db.GetCartItemByDishAndCustomizationsParams{
			CartID:         cart.ID,
			DishID:         pgtype.Int8{Int64: *input.DishID, Valid: true},
			Customizations: customizations,
		})
		if err == nil {
			if input.MaxQuantity > 0 && existingItem.Quantity+input.Quantity > input.MaxQuantity {
				return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("单品数量不能超过%d", input.MaxQuantity))
			}
			_, err = store.UpdateCartItemQuantityRelative(ctx, db.UpdateCartItemQuantityRelativeParams{
				ID:     existingItem.ID,
				Amount: input.Quantity,
			})
			if err != nil {
				if errors.Is(err, db.ErrRecordNotFound) {
					return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("单品数量不能超过%d", input.MaxQuantity))
				}
				return result, err
			}
			result.Cart = cart
			return result, nil
		}
		if !errors.Is(err, db.ErrRecordNotFound) {
			return result, err
		}
	}

	if input.ComboID != nil {
		existingItem, err := store.GetCartItemByCombo(ctx, db.GetCartItemByComboParams{
			CartID:  cart.ID,
			ComboID: pgtype.Int8{Int64: *input.ComboID, Valid: true},
		})
		if err == nil {
			if input.MaxQuantity > 0 && existingItem.Quantity+input.Quantity > input.MaxQuantity {
				return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("单品数量不能超过%d", input.MaxQuantity))
			}
			_, err = store.UpdateCartItemQuantityRelative(ctx, db.UpdateCartItemQuantityRelativeParams{
				ID:     existingItem.ID,
				Amount: input.Quantity,
			})
			if err != nil {
				if errors.Is(err, db.ErrRecordNotFound) {
					return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("单品数量不能超过%d", input.MaxQuantity))
				}
				return result, err
			}
			result.Cart = cart
			return result, nil
		}
		if !errors.Is(err, db.ErrRecordNotFound) {
			return result, err
		}
	}

	var dishID, comboID pgtype.Int8
	if input.DishID != nil {
		dishID = pgtype.Int8{Int64: *input.DishID, Valid: true}
	}
	if input.ComboID != nil {
		comboID = pgtype.Int8{Int64: *input.ComboID, Valid: true}
	}

	_, err = store.AddCartItem(ctx, db.AddCartItemParams{
		CartID:         cart.ID,
		DishID:         dishID,
		ComboID:        comboID,
		Quantity:       input.Quantity,
		Customizations: customizations,
	})
	if err != nil {
		return result, err
	}

	result.Cart = cart
	return result, nil
}

// UpdateCartItem validates and updates a cart item.
func UpdateCartItem(ctx context.Context, store db.Store, input UpdateCartItemInput) (UpdateCartItemResult, error) {
	var result UpdateCartItemResult

	cartItem, err := store.GetCartItem(ctx, input.ItemID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("cart item not found"))
		}
		return result, err
	}

	cart, err := store.GetCart(ctx, cartItem.CartID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("cart not found"))
		}
		return result, err
	}

	if cart.UserID != input.UserID {
		return result, NewRequestError(http.StatusForbidden, errors.New("cart item does not belong to you"))
	}

	updateParams := db.UpdateCartItemParams{ID: input.ItemID}

	if input.Quantity != nil {
		if *input.Quantity < 1 || (input.MaxQuantity > 0 && *input.Quantity > input.MaxQuantity) {
			return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("quantity must be between 1 and %d", input.MaxQuantity))
		}
		updateParams.Quantity = pgtype.Int2{Int16: *input.Quantity, Valid: true}
	}

	if input.Customizations != nil {
		customizations, err := MarshalCustomizationsCanonical(input.Customizations)
		if err != nil {
			return result, NewRequestError(http.StatusBadRequest, errors.New("invalid customizations"))
		}
		updateParams.Customizations = customizations
	}

	if cartItem.DishID.Valid {
		dish, err := store.GetDish(ctx, cartItem.DishID.Int64)
		if err != nil {
			return result, err
		}
		if !dish.IsOnline || !dish.IsAvailable {
			return result, NewRequestError(http.StatusBadRequest, errors.New("dish is not available"))
		}
	}
	if cartItem.ComboID.Valid {
		combo, err := store.GetComboSet(ctx, cartItem.ComboID.Int64)
		if err != nil {
			return result, err
		}
		if !combo.IsOnline {
			return result, NewRequestError(http.StatusBadRequest, errors.New("combo is not available"))
		}
	}

	if _, err := store.UpdateCartItem(ctx, updateParams); err != nil {
		return result, err
	}

	result.Cart = cart
	return result, nil
}
