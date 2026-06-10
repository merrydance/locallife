package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestOrderDisplayConfigAutoAcceptMigrationCleanDatabaseContract(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 262)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	requireOrderDisplayConfigPrintAutoAcceptConstraint(t, pool)
	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)

	_, err := pool.Exec(context.Background(), `
		INSERT INTO order_display_configs (
			merchant_id,
			enable_print,
			auto_accept_paid_orders
		) VALUES ($1, false, true)
	`, merchantID)
	requireOrderDisplayConfigPrintAutoAcceptConstraintError(t, err)
}

func TestOrderDisplayConfigAutoAcceptMigration262CleansHistoricalDirtyRows(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 261)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	dirtyMerchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	validEnabledMerchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	validDisabledMerchantID := insertMembershipSettingsMigrationMerchant(t, pool)

	_, err := pool.Exec(context.Background(), `
		INSERT INTO order_display_configs (
			merchant_id,
			enable_print,
			auto_accept_paid_orders
		) VALUES
			($1, false, true),
			($2, true, true),
			($3, false, false)
	`, dirtyMerchantID, validEnabledMerchantID, validDisabledMerchantID)
	require.NoError(t, err)

	runMembershipSettingsMigrations(t, dbSource, 262)
	requireOrderDisplayConfigPrintAutoAcceptConstraint(t, pool)

	rows, err := pool.Query(context.Background(), `
		SELECT merchant_id, enable_print, auto_accept_paid_orders
		FROM order_display_configs
		WHERE merchant_id = ANY($1::bigint[])
	`, []int64{dirtyMerchantID, validEnabledMerchantID, validDisabledMerchantID})
	require.NoError(t, err)
	defer rows.Close()

	got := map[int64]struct {
		enablePrint          bool
		autoAcceptPaidOrders bool
	}{}
	for rows.Next() {
		var merchantID int64
		var row struct {
			enablePrint          bool
			autoAcceptPaidOrders bool
		}
		require.NoError(t, rows.Scan(&merchantID, &row.enablePrint, &row.autoAcceptPaidOrders))
		got[merchantID] = row
	}
	require.NoError(t, rows.Err())
	require.Len(t, got, 3)
	require.False(t, got[dirtyMerchantID].enablePrint)
	require.False(t, got[dirtyMerchantID].autoAcceptPaidOrders)
	require.True(t, got[validEnabledMerchantID].enablePrint)
	require.True(t, got[validEnabledMerchantID].autoAcceptPaidOrders)
	require.False(t, got[validDisabledMerchantID].enablePrint)
	require.False(t, got[validDisabledMerchantID].autoAcceptPaidOrders)

	_, err = pool.Exec(context.Background(), `
		INSERT INTO order_display_configs (
			merchant_id,
			enable_print,
			auto_accept_paid_orders
		) VALUES ($1, false, true)
	`, insertMembershipSettingsMigrationMerchant(t, pool))
	requireOrderDisplayConfigPrintAutoAcceptConstraintError(t, err)
}

func requireOrderDisplayConfigPrintAutoAcceptConstraint(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	var def string
	err := pool.QueryRow(context.Background(), `
		SELECT pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conrelid = 'order_display_configs'::regclass
			AND conname = 'order_display_configs_print_auto_accept_check'
	`).Scan(&def)
	require.NoError(t, err)
	require.Contains(t, def, "enable_print")
	require.Contains(t, def, "auto_accept_paid_orders")
}
