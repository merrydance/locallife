package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestPackagingLegacyDishMigration276NoLegacyRowsProducesNoSettingsOrOptions(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 275)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)

	runMembershipSettingsMigrations(t, dbSource, 276)

	var settingsCount int64
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_packaging_settings
		WHERE merchant_id = $1
	`, merchantID).Scan(&settingsCount)
	require.NoError(t, err)
	require.Equal(t, int64(0), settingsCount)

	var optionCount int64
	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_packaging_options
		WHERE merchant_id = $1
	`, merchantID).Scan(&optionCount)
	require.NoError(t, err)
	require.Equal(t, int64(0), optionCount)
}

func TestPackagingLegacyDishMigration276CreatesSettingsOptionsAndLeavesOrderItems(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 275)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	dishID := insertPackagingMigrationDish(t, pool, packagingMigrationDishInput{
		MerchantID:  merchantID,
		Name:        "环保餐盒",
		Description: "默认环保包装",
		Price:       150,
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   7,
	})
	orderID, orderItemID := insertPackagingMigrationOrderWithItem(t, pool, merchantID, dishID)

	runMembershipSettingsMigrations(t, dbSource, 276)
	executePackagingLegacyDishMigration276SQL(t, pool)

	var enabled bool
	var required bool
	var applicableOrderTypes []string
	err := pool.QueryRow(context.Background(), `
		SELECT enabled, required, applicable_order_types
		FROM merchant_packaging_settings
		WHERE merchant_id = $1
	`, merchantID).Scan(&enabled, &required, &applicableOrderTypes)
	require.NoError(t, err)
	require.True(t, enabled)
	require.True(t, required)
	require.Equal(t, []string{"takeout", "takeaway"}, applicableOrderTypes)

	var optionID int64
	var legacyDishID int64
	var optionName string
	var optionDescription string
	var price int64
	var isEnabled bool
	var sortOrder int16
	err = pool.QueryRow(context.Background(), `
		SELECT id, legacy_dish_id, name, description, price, is_enabled, sort_order
		FROM merchant_packaging_options
		WHERE merchant_id = $1
		  AND legacy_dish_id = $2
	`, merchantID, dishID).Scan(&optionID, &legacyDishID, &optionName, &optionDescription, &price, &isEnabled, &sortOrder)
	require.NoError(t, err)
	require.NotZero(t, optionID)
	require.Equal(t, dishID, legacyDishID)
	require.Equal(t, "环保餐盒", optionName)
	require.Equal(t, "默认环保包装", optionDescription)
	require.Equal(t, int64(150), price)
	require.True(t, isEnabled)
	require.Equal(t, int16(7), sortOrder)

	var optionCount int64
	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_packaging_options
		WHERE merchant_id = $1
		  AND legacy_dish_id = $2
	`, merchantID, dishID).Scan(&optionCount)
	require.NoError(t, err)
	require.Equal(t, int64(1), optionCount)

	var orderItemDishID int64
	var orderItemName string
	err = pool.QueryRow(context.Background(), `
		SELECT dish_id, name
		FROM order_items
		WHERE id = $1
	`, orderItemID).Scan(&orderItemDishID, &orderItemName)
	require.NoError(t, err)
	require.Equal(t, dishID, orderItemDishID)
	require.Equal(t, "环保餐盒", orderItemName)

	var orderPackagingItemCount int64
	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM order_packaging_items
		WHERE order_id = $1
	`, orderID).Scan(&orderPackagingItemCount)
	require.NoError(t, err)
	require.Equal(t, int64(0), orderPackagingItemCount)
}

func TestPackagingLegacyDishMigration276AvoidsExistingAndDuplicateOptionNames(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 275)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	insertPackagingMigrationManualOption(t, pool, merchantID, "环保餐盒")
	firstDishID := insertPackagingMigrationDish(t, pool, packagingMigrationDishInput{
		MerchantID:  merchantID,
		Name:        "环保餐盒",
		Description: "旧包装 1",
		Price:       150,
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   1,
	})
	secondDishID := insertPackagingMigrationDish(t, pool, packagingMigrationDishInput{
		MerchantID:  merchantID,
		Name:        "环保餐盒",
		Description: "旧包装 2",
		Price:       200,
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   2,
	})

	runMembershipSettingsMigrations(t, dbSource, 276)
	executePackagingLegacyDishMigration276SQL(t, pool)

	var manualOptionCount int64
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_packaging_options
		WHERE merchant_id = $1
		  AND legacy_dish_id IS NULL
		  AND name = '环保餐盒'
	`, merchantID).Scan(&manualOptionCount)
	require.NoError(t, err)
	require.Equal(t, int64(1), manualOptionCount)

	names := map[int64]string{}
	rows, err := pool.Query(context.Background(), `
		SELECT legacy_dish_id, name
		FROM merchant_packaging_options
		WHERE merchant_id = $1
		  AND legacy_dish_id IS NOT NULL
		ORDER BY legacy_dish_id
	`, merchantID)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var legacyDishID int64
		var name string
		require.NoError(t, rows.Scan(&legacyDishID, &name))
		names[legacyDishID] = name
	}
	require.NoError(t, rows.Err())
	require.Equal(t, "环保餐盒-legacy-"+fmt.Sprint(firstDishID), names[firstDishID])
	require.Equal(t, "环保餐盒-legacy-"+fmt.Sprint(secondDishID), names[secondDishID])

	var totalOptionCount int64
	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_packaging_options
		WHERE merchant_id = $1
	`, merchantID).Scan(&totalOptionCount)
	require.NoError(t, err)
	require.Equal(t, int64(3), totalOptionCount)
}

func TestPackagingLegacyDishMigration276ReusesExistingLegacyDishOption(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 275)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	dishID := insertPackagingMigrationDish(t, pool, packagingMigrationDishInput{
		MerchantID:  merchantID,
		Name:        "环保餐盒",
		Description: "旧包装",
		Price:       150,
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   3,
	})
	preexistingOptionID := insertPackagingMigrationManualLegacyOption(t, pool, merchantID, dishID)

	runMembershipSettingsMigrations(t, dbSource, 276)

	var optionID int64
	var optionName string
	var description string
	var price int64
	var isEnabled bool
	var sortOrder int16
	err := pool.QueryRow(context.Background(), `
		SELECT id, name, description, price, is_enabled, sort_order
		FROM merchant_packaging_options
		WHERE legacy_dish_id = $1
	`, dishID).Scan(&optionID, &optionName, &description, &price, &isEnabled, &sortOrder)
	require.NoError(t, err)
	require.Equal(t, preexistingOptionID, optionID)
	require.Equal(t, "环保餐盒", optionName)
	require.Equal(t, "旧包装", description)
	require.Equal(t, int64(150), price)
	require.True(t, isEnabled)
	require.Equal(t, int16(3), sortOrder)

	var optionCount int64
	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_packaging_options
		WHERE legacy_dish_id = $1
	`, dishID).Scan(&optionCount)
	require.NoError(t, err)
	require.Equal(t, int64(1), optionCount)
}

func TestPackagingLegacyDishMigration276RepairsExistingLegacyDishOptionMerchantID(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 275)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	wrongMerchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	dishID := insertPackagingMigrationDish(t, pool, packagingMigrationDishInput{
		MerchantID:  merchantID,
		Name:        "环保餐盒",
		Description: "旧包装",
		Price:       150,
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   3,
	})
	preexistingOptionID := insertPackagingMigrationManualLegacyOption(t, pool, wrongMerchantID, dishID)

	runMembershipSettingsMigrations(t, dbSource, 276)

	var optionID int64
	var optionMerchantID int64
	err := pool.QueryRow(context.Background(), `
		SELECT id, merchant_id
		FROM merchant_packaging_options
		WHERE legacy_dish_id = $1
	`, dishID).Scan(&optionID, &optionMerchantID)
	require.NoError(t, err)
	require.Equal(t, preexistingOptionID, optionID)
	require.Equal(t, merchantID, optionMerchantID)
}

func TestPackagingLegacyDishMigration276RestoresSoftDeletedLegacyDishOption(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 275)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	dishID := insertPackagingMigrationDish(t, pool, packagingMigrationDishInput{
		MerchantID:  merchantID,
		Name:        "环保餐盒",
		Description: "旧包装",
		Price:       150,
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   3,
	})
	preexistingOptionID := insertPackagingMigrationManualLegacyOption(t, pool, merchantID, dishID)
	_, err := pool.Exec(context.Background(), `
		UPDATE merchant_packaging_options
		SET deleted_at = now()
		WHERE id = $1
	`, preexistingOptionID)
	require.NoError(t, err)

	runMembershipSettingsMigrations(t, dbSource, 276)

	var optionID int64
	var deletedAt time.Time
	var hasDeletedAt bool
	err = pool.QueryRow(context.Background(), `
		SELECT id, deleted_at IS NOT NULL, COALESCE(deleted_at, 'epoch'::timestamptz)
		FROM merchant_packaging_options
		WHERE legacy_dish_id = $1
	`, dishID).Scan(&optionID, &hasDeletedAt, &deletedAt)
	require.NoError(t, err)
	require.Equal(t, preexistingOptionID, optionID)
	require.False(t, hasDeletedAt)
	require.Equal(t, time.Unix(0, 0).UTC(), deletedAt.UTC())
}

func TestPackagingLegacyDishMigration276DoesNotSkipDeepNameConflicts(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 275)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	baseName := "餐盒"
	dishID := insertPackagingMigrationDish(t, pool, packagingMigrationDishInput{
		MerchantID:  merchantID,
		Name:        baseName,
		Description: "旧包装",
		Price:       150,
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   1,
	})

	for candidateRank := 0; candidateRank <= 100; candidateRank++ {
		insertPackagingMigrationManualOption(
			t,
			pool,
			merchantID,
			packagingMigrationCandidateName(baseName, dishID, candidateRank),
		)
	}

	runMembershipSettingsMigrations(t, dbSource, 276)

	var optionName string
	err := pool.QueryRow(context.Background(), `
		SELECT name
		FROM merchant_packaging_options
		WHERE legacy_dish_id = $1
	`, dishID).Scan(&optionName)
	require.NoError(t, err)
	require.Equal(t, packagingMigrationCandidateName(baseName, dishID, 101), optionName)
}

func TestPackagingLegacyDishMigration276DownPreservesMigratedRows(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 275)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	dishID := insertPackagingMigrationDish(t, pool, packagingMigrationDishInput{
		MerchantID:  merchantID,
		Name:        "环保餐盒",
		Description: "旧包装",
		Price:       150,
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   3,
	})

	runMembershipSettingsMigrations(t, dbSource, 276)
	runPackagingMigrationDownOneStep(t, dbSource)

	var settingsCount int64
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_packaging_settings
		WHERE merchant_id = $1
	`, merchantID).Scan(&settingsCount)
	require.NoError(t, err)
	require.Equal(t, int64(1), settingsCount)

	var optionCount int64
	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_packaging_options
		WHERE legacy_dish_id = $1
	`, dishID).Scan(&optionCount)
	require.NoError(t, err)
	require.Equal(t, int64(1), optionCount)
}

func TestPackagingLegacyDishMigration276DisabledOrDeletedDishesStayDisabled(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 275)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	offlineDishID := insertPackagingMigrationDish(t, pool, packagingMigrationDishInput{
		MerchantID:  merchantID,
		Name:        "下线餐盒",
		Description: "已经下线",
		Price:       100,
		IsAvailable: true,
		IsOnline:    false,
		SortOrder:   1,
	})
	deletedDishID := insertPackagingMigrationDish(t, pool, packagingMigrationDishInput{
		MerchantID:  merchantID,
		Name:        "删除餐盒",
		Description: "已经删除",
		Price:       120,
		IsAvailable: true,
		IsOnline:    true,
		SortOrder:   2,
		DeletedAt:   time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
	})

	runMembershipSettingsMigrations(t, dbSource, 276)

	var settingsCount int64
	err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_packaging_settings
		WHERE merchant_id = $1
	`, merchantID).Scan(&settingsCount)
	require.NoError(t, err)
	require.Equal(t, int64(0), settingsCount)

	rows, err := pool.Query(context.Background(), `
		SELECT legacy_dish_id, is_enabled
		FROM merchant_packaging_options
		WHERE merchant_id = $1
		ORDER BY legacy_dish_id
	`, merchantID)
	require.NoError(t, err)
	defer rows.Close()

	got := map[int64]bool{}
	for rows.Next() {
		var legacyDishID int64
		var isEnabled bool
		require.NoError(t, rows.Scan(&legacyDishID, &isEnabled))
		got[legacyDishID] = isEnabled
	}
	require.NoError(t, rows.Err())
	require.Equal(t, map[int64]bool{
		offlineDishID: false,
		deletedDishID: false,
	}, got)
}

type packagingMigrationDishInput struct {
	MerchantID  int64
	Name        string
	Description string
	Price       int64
	IsAvailable bool
	IsOnline    bool
	SortOrder   int16
	DeletedAt   time.Time
}

func insertPackagingMigrationDish(t *testing.T, pool *pgxpool.Pool, input packagingMigrationDishInput) int64 {
	t.Helper()

	var deletedAt any
	if !input.DeletedAt.IsZero() {
		deletedAt = input.DeletedAt
	}

	var dishID int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO dishes (
			merchant_id,
			name,
			description,
			price,
			is_available,
			is_online,
			is_packaging,
			sort_order,
			prepare_time,
			deleted_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, true, $7, 10, $8
		)
		RETURNING id
	`, input.MerchantID, input.Name, input.Description, input.Price, input.IsAvailable, input.IsOnline, input.SortOrder, deletedAt).Scan(&dishID)
	require.NoError(t, err)
	return dishID
}

func insertPackagingMigrationOrderWithItem(t *testing.T, pool *pgxpool.Pool, merchantID int64, dishID int64) (int64, int64) {
	t.Helper()

	var userID int64
	err := pool.QueryRow(context.Background(), `
		SELECT owner_user_id
		FROM merchants
		WHERE id = $1
	`, merchantID).Scan(&userID)
	require.NoError(t, err)

	var orderID int64
	err = pool.QueryRow(context.Background(), `
		INSERT INTO orders (
			order_no,
			user_id,
			merchant_id,
			order_type,
			subtotal,
			total_amount,
			status
		) VALUES (
			$1, $2, $3, 'takeaway', 150, 150, 'paid'
		)
		RETURNING id
	`, fmt.Sprintf("PKGMIG%d", time.Now().UnixNano()), userID, merchantID).Scan(&orderID)
	require.NoError(t, err)

	var orderItemID int64
	err = pool.QueryRow(context.Background(), `
		INSERT INTO order_items (
			order_id,
			dish_id,
			name,
			unit_price,
			quantity,
			subtotal
		) VALUES (
			$1, $2, '环保餐盒', 150, 1, 150
		)
		RETURNING id
	`, orderID, dishID).Scan(&orderItemID)
	require.NoError(t, err)
	return orderID, orderItemID
}

func insertPackagingMigrationManualOption(t *testing.T, pool *pgxpool.Pool, merchantID int64, name string) int64 {
	t.Helper()

	var optionID int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO merchant_packaging_options (
			merchant_id,
			name,
			description,
			price,
			is_enabled,
			sort_order
		) VALUES (
			$1, $2, '手工配置', 99, true, 0
		)
		RETURNING id
	`, merchantID, name).Scan(&optionID)
	require.NoError(t, err)
	return optionID
}

func insertPackagingMigrationManualLegacyOption(t *testing.T, pool *pgxpool.Pool, merchantID int64, dishID int64) int64 {
	t.Helper()

	var optionID int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO merchant_packaging_options (
			merchant_id,
			legacy_dish_id,
			name,
			description,
			price,
			is_enabled,
			sort_order
		) VALUES (
			$1, $2, '手工旧配置', '迁移前配置', 1, false, 99
		)
		RETURNING id
	`, merchantID, dishID).Scan(&optionID)
	require.NoError(t, err)
	return optionID
}

func packagingMigrationCandidateName(baseName string, dishID int64, candidateRank int) string {
	switch candidateRank {
	case 0:
		return baseName
	case 1:
		return fmt.Sprintf("%s-legacy-%d", baseName, dishID)
	default:
		return fmt.Sprintf("%s-legacy-%d-%d", baseName, dishID, candidateRank)
	}
}

func executePackagingLegacyDishMigration276SQL(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	sqlPath := filepath.Join(filepath.Dir(file), "..", "migration", "000276_migrate_packaging_dishes_to_options.up.sql")
	sqlBytes, err := os.ReadFile(sqlPath)
	require.NoError(t, err)

	_, err = pool.Exec(context.Background(), string(sqlBytes))
	require.NoError(t, err)
}

func runPackagingMigrationDownOneStep(t *testing.T, dbSource string) {
	t.Helper()

	mig, err := migrate.New(membershipSettingsMigrationURL(t), dbSource)
	require.NoError(t, err)
	err = mig.Steps(-1)
	closeMembershipSettingsMigration(t, mig)
	require.NoError(t, err)
}
