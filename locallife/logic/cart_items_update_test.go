package logic

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUpdateCartItem_NotOwner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	cartItem := db.GetCartItemRow{ID: 1, CartID: 2}

	store.EXPECT().
		GetCartItem(gomock.Any(), int64(1)).
		Times(1).
		Return(cartItem, nil)
	store.EXPECT().
		GetCart(gomock.Any(), int64(2)).
		Times(1).
		Return(db.Cart{ID: 2, UserID: 99}, nil)

	_, err := UpdateCartItem(context.Background(), store, UpdateCartItemInput{UserID: 1, ItemID: 1, MaxQuantity: 99})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
}

func TestUpdateCartItem_InvalidQuantity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	cartItem := db.GetCartItemRow{ID: 1, CartID: 2}
	qty := int16(100)

	store.EXPECT().
		GetCartItem(gomock.Any(), int64(1)).
		Times(1).
		Return(cartItem, nil)
	store.EXPECT().
		GetCart(gomock.Any(), int64(2)).
		Times(1).
		Return(db.Cart{ID: 2, UserID: 1}, nil)

	_, err := UpdateCartItem(context.Background(), store, UpdateCartItemInput{UserID: 1, ItemID: 1, Quantity: &qty, MaxQuantity: 99})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
}

func TestUpdateCartItem_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	cartItem := db.GetCartItemRow{ID: 1, CartID: 2, DishID: pgtype.Int8{Int64: 10, Valid: true}}
	qty := int16(2)

	store.EXPECT().
		GetCartItem(gomock.Any(), int64(1)).
		Times(1).
		Return(cartItem, nil)
	store.EXPECT().
		GetCart(gomock.Any(), int64(2)).
		Times(1).
		Return(db.Cart{ID: 2, UserID: 1}, nil)
	store.EXPECT().
		GetDish(gomock.Any(), int64(10)).
		Times(1).
		Return(db.Dish{ID: 10, MerchantID: 3, IsOnline: true, IsAvailable: true}, nil)
	store.EXPECT().
		UpdateCartItem(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CartItem{}, nil)

	result, err := UpdateCartItem(context.Background(), store, UpdateCartItemInput{UserID: 1, ItemID: 1, Quantity: &qty, MaxQuantity: 99})
	require.NoError(t, err)
	require.Equal(t, int64(2), result.Cart.ID)
}
