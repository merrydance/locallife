package logic

import (
	"context"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAddCartItem_MissingItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	_, err := AddCartItem(context.Background(), store, AddCartItemInput{UserID: 1, MerchantID: 2})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
}

func TestAddCartItem_ComboWithCustomizations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	comboID := int64(10)

	_, err := AddCartItem(context.Background(), store, AddCartItemInput{
		UserID:         1,
		MerchantID:     2,
		ComboID:        &comboID,
		Quantity:       1,
		Customizations: map[string]interface{}{"x": 1},
		MaxQuantity:    99,
	})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
}

func TestAddCartItem_ExistingDishOverMax(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	dishID := int64(10)
	cart := db.Cart{ID: 9}

	store.EXPECT().
		GetMerchant(gomock.Any(), int64(2)).
		Times(1).
		Return(db.Merchant{ID: 2, Status: "active", IsOpen: true}, nil)
	store.EXPECT().
		GetDish(gomock.Any(), dishID).
		Times(1).
		Return(db.Dish{ID: dishID, MerchantID: 2, IsOnline: true, IsAvailable: true}, nil)
	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
		Times(1).
		Return(cart, nil)
	store.EXPECT().
		GetCartItemByDishAndCustomizations(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CartItem{ID: 1, Quantity: 99}, nil)

	_, err := AddCartItem(context.Background(), store, AddCartItemInput{
		UserID:      1,
		MerchantID:  2,
		DishID:      &dishID,
		Quantity:    1,
		MaxQuantity: 99,
	})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
}

func TestAddCartItem_NewDish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	dishID := int64(10)
	cart := db.Cart{ID: 9}

	store.EXPECT().
		GetMerchant(gomock.Any(), int64(2)).
		Times(1).
		Return(db.Merchant{ID: 2, Status: "active", IsOpen: true}, nil)
	store.EXPECT().
		GetDish(gomock.Any(), dishID).
		Times(1).
		Return(db.Dish{ID: dishID, MerchantID: 2, IsOnline: true, IsAvailable: true}, nil)
	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
		Times(1).
		Return(cart, nil)
	store.EXPECT().
		GetCartItemByDishAndCustomizations(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CartItem{}, db.ErrRecordNotFound)
	store.EXPECT().
		AddCartItem(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CartItem{}, nil)

	result, err := AddCartItem(context.Background(), store, AddCartItemInput{
		UserID:      1,
		MerchantID:  2,
		DishID:      &dishID,
		Quantity:    1,
		MaxQuantity: 99,
	})
	require.NoError(t, err)
	require.Equal(t, cart.ID, result.Cart.ID)
}
