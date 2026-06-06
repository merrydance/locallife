package logic

import (
	"context"
	"errors"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCalculateOrderItems(t *testing.T) {
	merchantID := int64(10)
	dishID := int64(20)
	comboID := int64(30)

	defaultDish := db.Dish{
		ID:          dishID,
		MerchantID:  merchantID,
		Name:        "Dish A",
		Price:       1200,
		IsOnline:    true,
		IsAvailable: true,
	}
	defaultCombo := db.ComboSet{
		ID:         comboID,
		MerchantID: merchantID,
		Name:       "Combo A",
		ComboPrice: 3000,
		IsOnline:   true,
	}

	testCases := []struct {
		name       string
		items      []OrderItemInput
		normalize  NormalizeDishCustomizationsFunc
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, subtotal int64, orderItems []db.CreateOrderItemParams, err error)
	}{
		{
			name:  "MissingIDs",
			items: []OrderItemInput{{Quantity: 1}},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "each item must have either dish_id or combo_id", err.Error())
			},
		},
		{
			name:  "BothIDs",
			items: []OrderItemInput{{DishID: &dishID, ComboID: &comboID, Quantity: 1}},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "each item can only have one of dish_id or combo_id", err.Error())
			},
		},
		{
			name:  "DishNotFound",
			items: []OrderItemInput{{DishID: &dishID, Quantity: 1}},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDish(gomock.Any(), dishID).
					Times(1).
					Return(db.Dish{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "dish 20 not found", err.Error())
			},
		},
		{
			name:  "DishOtherMerchant",
			items: []OrderItemInput{{DishID: &dishID, Quantity: 1}},
			buildStubs: func(store *mockdb.MockStore) {
				dish := defaultDish
				dish.MerchantID = merchantID + 1
				store.EXPECT().
					GetDish(gomock.Any(), dishID).
					Times(1).
					Return(dish, nil)
			},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "dish 20 does not belong to this merchant", err.Error())
			},
		},
		{
			name:  "DishOffline",
			items: []OrderItemInput{{DishID: &dishID, Quantity: 1}},
			buildStubs: func(store *mockdb.MockStore) {
				dish := defaultDish
				dish.IsOnline = false
				store.EXPECT().
					GetDish(gomock.Any(), dishID).
					Times(1).
					Return(dish, nil)
			},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "dish Dish A is offline", err.Error())
			},
		},
		{
			name:  "DishUnavailable",
			items: []OrderItemInput{{DishID: &dishID, Quantity: 1}},
			buildStubs: func(store *mockdb.MockStore) {
				dish := defaultDish
				dish.IsAvailable = false
				store.EXPECT().
					GetDish(gomock.Any(), dishID).
					Times(1).
					Return(dish, nil)
			},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "dish Dish A is not available today", err.Error())
			},
		},
		{
			name:  "MissingCustomizationHandler",
			items: []OrderItemInput{{DishID: &dishID, Quantity: 1}},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDish(gomock.Any(), dishID).
					Times(1).
					Return(defaultDish, nil)
			},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "customizations handler is not configured", err.Error())
			},
		},
		{
			name:  "CustomizationError",
			items: []OrderItemInput{{DishID: &dishID, Quantity: 1}},
			normalize: func(ctx context.Context, dishID int64, _ map[string]interface{}) ([]byte, int64, error) {
				return nil, 0, errors.New("normalize failed")
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDish(gomock.Any(), dishID).
					Times(1).
					Return(defaultDish, nil)
			},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "normalize failed", err.Error())
			},
		},
		{
			name:  "SuccessDish",
			items: []OrderItemInput{{DishID: &dishID, Quantity: 2, Customizations: map[string]interface{}{"spice": "mild"}}},
			normalize: func(ctx context.Context, dishID int64, _ map[string]interface{}) ([]byte, int64, error) {
				return []byte(`{"spice":"mild"}`), 100, nil
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDish(gomock.Any(), dishID).
					Times(1).
					Return(defaultDish, nil)
			},
			check: func(t *testing.T, subtotal int64, orderItems []db.CreateOrderItemParams, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(2600), subtotal)
				require.Len(t, orderItems, 1)
				item := orderItems[0]
				require.True(t, item.DishID.Valid)
				require.Equal(t, dishID, item.DishID.Int64)
				require.False(t, item.ComboID.Valid)
				require.Equal(t, int64(1300), item.UnitPrice)
				require.Equal(t, int16(2), item.Quantity)
				require.Equal(t, int64(2600), item.Subtotal)
				require.Equal(t, []byte(`{"spice":"mild"}`), item.Customizations)
			},
		},
		{
			name:  "ComboCustomizationsNotAllowed",
			items: []OrderItemInput{{ComboID: &comboID, Quantity: 1, Customizations: map[string]interface{}{"x": "y"}}},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "customizations only supported for dish items", err.Error())
			},
		},
		{
			name:  "ComboNotFound",
			items: []OrderItemInput{{ComboID: &comboID, Quantity: 1}},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetComboSet(gomock.Any(), comboID).
					Times(1).
					Return(db.ComboSet{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "combo 30 not found", err.Error())
			},
		},
		{
			name:  "ComboOtherMerchant",
			items: []OrderItemInput{{ComboID: &comboID, Quantity: 1}},
			buildStubs: func(store *mockdb.MockStore) {
				combo := defaultCombo
				combo.MerchantID = merchantID + 1
				store.EXPECT().
					GetComboSet(gomock.Any(), comboID).
					Times(1).
					Return(combo, nil)
			},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "combo 30 does not belong to this merchant", err.Error())
			},
		},
		{
			name:  "ComboOffline",
			items: []OrderItemInput{{ComboID: &comboID, Quantity: 1}},
			buildStubs: func(store *mockdb.MockStore) {
				combo := defaultCombo
				combo.IsOnline = false
				store.EXPECT().
					GetComboSet(gomock.Any(), comboID).
					Times(1).
					Return(combo, nil)
			},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "combo Combo A is offline", err.Error())
			},
		},
		{
			name:  "ComboChildUnavailable",
			items: []OrderItemInput{{ComboID: &comboID, Quantity: 1}},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetComboSet(gomock.Any(), comboID).
					Times(1).
					Return(defaultCombo, nil)
				store.EXPECT().
					ListComboDishOrderability(gomock.Any(), comboID).
					Times(1).
					Return([]db.ListComboDishOrderabilityRow{
						comboDishOrderabilityRow(dishID, "Dish A", true, true, false),
					}, nil)
			},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "combo Combo A contains unavailable dish Dish A", err.Error())
			},
		},
		{
			name:  "ComboChildOrderabilityStoreError",
			items: []OrderItemInput{{ComboID: &comboID, Quantity: 1}},
			buildStubs: func(store *mockdb.MockStore) {
				storeErr := errors.New("list combo dish orderability failed")
				store.EXPECT().
					GetComboSet(gomock.Any(), comboID).
					Times(1).
					Return(defaultCombo, nil)
				store.EXPECT().
					ListComboDishOrderability(gomock.Any(), comboID).
					Times(1).
					Return(nil, storeErr)
			},
			check: func(t *testing.T, _ int64, _ []db.CreateOrderItemParams, err error) {
				require.Error(t, err)
				require.Equal(t, "list combo dish orderability failed", err.Error())
			},
		},
		{
			name:  "SuccessCombo",
			items: []OrderItemInput{{ComboID: &comboID, Quantity: 3}},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetComboSet(gomock.Any(), comboID).
					Times(1).
					Return(defaultCombo, nil)
				store.EXPECT().
					ListComboDishOrderability(gomock.Any(), comboID).
					Times(1).
					Return([]db.ListComboDishOrderabilityRow{
						comboDishOrderabilityRow(dishID, "Dish A", true, true, true),
					}, nil)
			},
			check: func(t *testing.T, subtotal int64, orderItems []db.CreateOrderItemParams, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(9000), subtotal)
				require.Len(t, orderItems, 1)
				item := orderItems[0]
				require.True(t, item.ComboID.Valid)
				require.Equal(t, comboID, item.ComboID.Int64)
				require.False(t, item.DishID.Valid)
				require.Equal(t, int64(3000), item.UnitPrice)
				require.Equal(t, int16(3), item.Quantity)
				require.Equal(t, int64(9000), item.Subtotal)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			if tc.buildStubs != nil {
				tc.buildStubs(store)
			}

			subtotal, orderItems, err := CalculateOrderItems(context.Background(), store, merchantID, tc.items, tc.normalize)
			tc.check(t, subtotal, orderItems, err)
		})
	}
}
