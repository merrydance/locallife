package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestPackagingDomainMigrationShape(t *testing.T) {
	ctx := context.Background()
	pool := testStore.(*SQLStore).connPool

	owner := createRandomUser(t)
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	cart := createRandomCart(t, user.ID, merchant.ID)
	order := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)

	var packagingFee int64
	err := pool.QueryRow(ctx, "SELECT packaging_fee FROM orders WHERE id = $1", order.ID).Scan(&packagingFee)
	require.NoError(t, err)
	require.Equal(t, int64(0), packagingFee)

	var settingID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO merchant_packaging_settings (merchant_id, enabled, required, applicable_order_types)
		VALUES ($1, true, true, ARRAY['takeout','takeaway']::TEXT[])
		RETURNING id
	`, merchant.ID).Scan(&settingID)
	require.NoError(t, err)
	require.NotZero(t, settingID)

	invalidMerchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	_, err = pool.Exec(ctx, `
		INSERT INTO merchant_packaging_settings (merchant_id, applicable_order_types)
		VALUES ($1, ARRAY['dine_in']::TEXT[])
	`, invalidMerchant.ID)
	require.Equal(t, "23514", ErrorCode(err))

	optionName := "普通餐盒-" + util.RandomString(8)
	var optionID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO merchant_packaging_options (merchant_id, name, description, price, is_enabled, sort_order)
		VALUES ($1, $2, '环保纸盒', 100, true, 1)
		RETURNING id
	`, merchant.ID, optionName).Scan(&optionID)
	require.NoError(t, err)
	require.NotZero(t, optionID)

	_, err = pool.Exec(ctx, `
		INSERT INTO merchant_packaging_options (merchant_id, name, price)
		VALUES ($1, $2, 200)
	`, merchant.ID, strings.ToUpper(optionName))
	require.Equal(t, UniqueViolation, ErrorCode(err))

	var selectionVersion int64
	err = pool.QueryRow(ctx, `
		INSERT INTO cart_packaging_selections (cart_id, packaging_option_id)
		VALUES ($1, $2)
		RETURNING selection_version
	`, cart.ID, optionID).Scan(&selectionVersion)
	require.NoError(t, err)
	require.Equal(t, int64(1), selectionVersion)

	var orderPackagingItemID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO order_packaging_items (order_id, packaging_option_id, name, unit_price, quantity, subtotal)
		VALUES ($1, $2, $3, 100, 1, 100)
		RETURNING id
	`, order.ID, optionID, optionName).Scan(&orderPackagingItemID)
	require.NoError(t, err)
	require.NotZero(t, orderPackagingItemID)

	_, err = pool.Exec(ctx, `
		INSERT INTO order_packaging_items (order_id, packaging_option_id, name, unit_price, quantity, subtotal)
		VALUES ($1, $2, $3, 100, 1, 100)
	`, order.ID, optionID, optionName)
	require.Equal(t, UniqueViolation, ErrorCode(err))

	anotherOrder := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)
	_, err = pool.Exec(ctx, `
		INSERT INTO order_packaging_items (order_id, packaging_option_id, name, unit_price, quantity, subtotal)
		VALUES ($1, $2, $3, 100, 2, 100)
	`, anotherOrder.ID, optionID, optionName)
	require.Equal(t, "23514", ErrorCode(err))
}

func TestCartPackagingSelectionVersionIsIdempotent(t *testing.T) {
	ctx := context.Background()

	owner := createRandomUser(t)
	user := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	cart := createRandomCart(t, user.ID, merchant.ID)

	firstOption, err := testStore.CreateMerchantPackagingOption(ctx, CreateMerchantPackagingOptionParams{
		MerchantID:   merchant.ID,
		Name:         "普通餐盒-" + util.RandomString(8),
		Description:  pgtype.Text{String: "默认包装", Valid: true},
		Price:        100,
		IsEnabled:    true,
		SortOrder:    1,
		LegacyDishID: pgtype.Int8{},
	})
	require.NoError(t, err)

	secondOption, err := testStore.CreateMerchantPackagingOption(ctx, CreateMerchantPackagingOptionParams{
		MerchantID:   merchant.ID,
		Name:         "保温袋-" + util.RandomString(8),
		Description:  pgtype.Text{String: "保温包装", Valid: true},
		Price:        200,
		IsEnabled:    true,
		SortOrder:    2,
		LegacyDishID: pgtype.Int8{},
	})
	require.NoError(t, err)

	clearedMissing, err := testStore.ClearCartPackagingSelection(ctx, cart.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), clearedMissing.SelectionVersion)
	require.False(t, clearedMissing.PackagingOptionID.Valid)

	selected, err := testStore.UpsertCartPackagingSelection(ctx, UpsertCartPackagingSelectionParams{
		CartID:            cart.ID,
		PackagingOptionID: pgtype.Int8{Int64: firstOption.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), selected.SelectionVersion)
	require.Equal(t, firstOption.ID, selected.PackagingOptionID.Int64)

	repeated, err := testStore.UpsertCartPackagingSelection(ctx, UpsertCartPackagingSelectionParams{
		CartID:            cart.ID,
		PackagingOptionID: pgtype.Int8{Int64: firstOption.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, selected.SelectionVersion, repeated.SelectionVersion)
	require.Equal(t, selected.UpdatedAt, repeated.UpdatedAt)

	changed, err := testStore.UpsertCartPackagingSelection(ctx, UpsertCartPackagingSelectionParams{
		CartID:            cart.ID,
		PackagingOptionID: pgtype.Int8{Int64: secondOption.ID, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), changed.SelectionVersion)
	require.Equal(t, secondOption.ID, changed.PackagingOptionID.Int64)

	cleared, err := testStore.ClearCartPackagingSelection(ctx, cart.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), cleared.SelectionVersion)
	require.False(t, cleared.PackagingOptionID.Valid)

	clearedAgain, err := testStore.ClearCartPackagingSelection(ctx, cart.ID)
	require.NoError(t, err)
	require.Equal(t, cleared.SelectionVersion, clearedAgain.SelectionVersion)
	require.Equal(t, cleared.UpdatedAt, clearedAgain.UpdatedAt)
	require.False(t, clearedAgain.PackagingOptionID.Valid)
}

func TestSoftDeleteMerchantPackagingOptionIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := testStore.(*SQLStore).connPool

	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	option, err := testStore.CreateMerchantPackagingOption(ctx, CreateMerchantPackagingOptionParams{
		MerchantID:   merchant.ID,
		Name:         "餐具套装-" + util.RandomString(8),
		Description:  pgtype.Text{String: "一次性餐具", Valid: true},
		Price:        50,
		IsEnabled:    true,
		SortOrder:    1,
		LegacyDishID: pgtype.Int8{},
	})
	require.NoError(t, err)

	deleted, err := testStore.SoftDeleteMerchantPackagingOption(ctx, SoftDeleteMerchantPackagingOptionParams{
		ID:         option.ID,
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)
	require.False(t, deleted.IsEnabled)
	require.True(t, deleted.DeletedAt.Valid)

	fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err = pool.Exec(ctx, `
		UPDATE merchant_packaging_options
		SET deleted_at = $1, updated_at = $1, is_enabled = false
		WHERE id = $2
	`, fixedTime, option.ID)
	require.NoError(t, err)

	deletedAgain, err := testStore.SoftDeleteMerchantPackagingOption(ctx, SoftDeleteMerchantPackagingOptionParams{
		ID:         option.ID,
		MerchantID: merchant.ID,
	})
	require.NoError(t, err)
	require.False(t, deletedAgain.IsEnabled)
	require.True(t, deletedAgain.DeletedAt.Valid)
	require.True(t, deletedAgain.UpdatedAt.Valid)
	require.Equal(t, fixedTime, deletedAgain.DeletedAt.Time.UTC())
	require.Equal(t, fixedTime, deletedAgain.UpdatedAt.Time.UTC())
}
