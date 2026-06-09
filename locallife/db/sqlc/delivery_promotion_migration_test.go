package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestDeliveryPromotionConstraintMigrationCleanDatabaseContract(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 254)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	requireDeliveryPromotionConstraints(t, pool)

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	now := time.Now()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO merchant_delivery_promotions (
			merchant_id,
			name,
			min_order_amount,
			discount_amount,
			valid_from,
			valid_until,
			is_active
		) VALUES ($1, 'valid-equal-threshold', 1000, 1000, $2, $3, true)
	`, merchantID, now, now.Add(time.Hour))
	require.NoError(t, err)

	_, err = pool.Exec(context.Background(), `
		INSERT INTO merchant_delivery_promotions (
			merchant_id,
			name,
			min_order_amount,
			discount_amount,
			valid_from,
			valid_until,
			is_active
		) VALUES ($1, 'invalid-after-clean-migration', 1000, 1001, $2, $3, true)
	`, merchantID, now, now.Add(time.Hour))
	requireDeliveryPromotionConstraintError(t, err, "merchant_delivery_promotions_discount_threshold_check")
}

func TestDeliveryPromotionConstraintMigration254CleansHistoricalDirtyRows(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 253)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	now := time.Now()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO merchant_delivery_promotions (
			merchant_id,
			name,
			min_order_amount,
			discount_amount,
			valid_from,
			valid_until,
			is_active
		) VALUES
			($1, 'zero-min-and-bad-date', 0, 100, $2, $2, true),
			($1, 'zero-discount', 1000, 0, $2, $3, true),
			($1, 'too-large-discount', 1000, 2000, $2, $3, true),
			($1, 'valid-row-preserved', 2000, 500, $2, $3, true)
	`, merchantID, now, now.Add(time.Hour))
	require.NoError(t, err)

	runMembershipSettingsMigrations(t, dbSource, 254)
	requireDeliveryPromotionConstraints(t, pool)

	var invalidCount int64
	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchant_delivery_promotions
		WHERE min_order_amount <= 0
		   OR discount_amount <= 0
		   OR discount_amount > min_order_amount
		   OR valid_until <= valid_from
	`).Scan(&invalidCount)
	require.NoError(t, err)
	require.Zero(t, invalidCount)

	rows, err := pool.Query(context.Background(), `
		SELECT name, min_order_amount, discount_amount, valid_from, valid_until
		FROM merchant_delivery_promotions
		WHERE merchant_id = $1
		ORDER BY name
	`, merchantID)
	require.NoError(t, err)
	defer rows.Close()

	got := map[string]struct {
		minOrderAmount int64
		discountAmount int64
		validFrom      time.Time
		validUntil     time.Time
	}{}
	for rows.Next() {
		var name string
		var row struct {
			minOrderAmount int64
			discountAmount int64
			validFrom      time.Time
			validUntil     time.Time
		}
		require.NoError(t, rows.Scan(&name, &row.minOrderAmount, &row.discountAmount, &row.validFrom, &row.validUntil))
		got[name] = row
	}
	require.NoError(t, rows.Err())

	require.Equal(t, int64(1), got["zero-min-and-bad-date"].minOrderAmount)
	require.Equal(t, int64(1), got["zero-min-and-bad-date"].discountAmount)
	require.True(t, got["zero-min-and-bad-date"].validUntil.After(got["zero-min-and-bad-date"].validFrom))
	require.Equal(t, int64(1000), got["zero-discount"].minOrderAmount)
	require.Equal(t, int64(1), got["zero-discount"].discountAmount)
	require.Equal(t, int64(1000), got["too-large-discount"].minOrderAmount)
	require.Equal(t, int64(1000), got["too-large-discount"].discountAmount)
	require.Equal(t, int64(2000), got["valid-row-preserved"].minOrderAmount)
	require.Equal(t, int64(500), got["valid-row-preserved"].discountAmount)

	_, err = pool.Exec(context.Background(), `
		INSERT INTO merchant_delivery_promotions (
			merchant_id,
			name,
			min_order_amount,
			discount_amount,
			valid_from,
			valid_until,
			is_active
		) VALUES ($1, 'invalid-after-254', 1000, 1001, $2, $3, true)
	`, merchantID, now, now.Add(time.Hour))
	requireDeliveryPromotionConstraintError(t, err, "merchant_delivery_promotions_discount_threshold_check")
}

func requireDeliveryPromotionConstraints(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	rows, err := pool.Query(context.Background(), `
		SELECT conname, pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conrelid = 'merchant_delivery_promotions'::regclass
			AND conname IN (
				'merchant_delivery_promotions_positive_amounts_check',
				'merchant_delivery_promotions_discount_threshold_check',
				'merchant_delivery_promotions_valid_period_check'
			)
		ORDER BY conname
	`)
	require.NoError(t, err)
	defer rows.Close()

	defs := map[string]string{}
	for rows.Next() {
		var name, def string
		require.NoError(t, rows.Scan(&name, &def))
		defs[name] = def
	}
	require.NoError(t, rows.Err())

	require.Contains(t, defs, "merchant_delivery_promotions_positive_amounts_check")
	require.Contains(t, defs["merchant_delivery_promotions_positive_amounts_check"], "min_order_amount > 0")
	require.Contains(t, defs["merchant_delivery_promotions_positive_amounts_check"], "discount_amount > 0")
	require.Contains(t, defs, "merchant_delivery_promotions_discount_threshold_check")
	require.Contains(t, defs["merchant_delivery_promotions_discount_threshold_check"], "discount_amount <= min_order_amount")
	require.Contains(t, defs, "merchant_delivery_promotions_valid_period_check")
	require.Contains(t, defs["merchant_delivery_promotions_valid_period_check"], "valid_until > valid_from")
}
