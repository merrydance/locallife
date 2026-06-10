package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestMerchantBusinessHoursSameDayWindowMigrationRemovesDirtyReverseRows(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 258)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO merchant_business_hours (
			merchant_id,
			day_of_week,
			open_time,
			close_time,
			is_closed
		) VALUES
			($1, 3, '09:00'::time, '18:00'::time, false),
			($1, 3, '21:00'::time, '09:00'::time, false)
	`, merchantID)
	require.NoError(t, err)

	runMembershipSettingsMigrations(t, dbSource, 259)

	var openRows, closedRows, reverseRows int
	err = pool.QueryRow(context.Background(), `
		SELECT
			COUNT(*) FILTER (WHERE is_closed = false),
			COUNT(*) FILTER (WHERE is_closed = true),
			COUNT(*) FILTER (WHERE open_time >= close_time)
		FROM merchant_business_hours
		WHERE merchant_id = $1
	`, merchantID).Scan(&openRows, &closedRows, &reverseRows)
	require.NoError(t, err)
	require.Equal(t, 1, openRows)
	require.Zero(t, closedRows)
	require.Zero(t, reverseRows)

	_, err = pool.Exec(context.Background(), `
		INSERT INTO merchant_business_hours (
			merchant_id,
			day_of_week,
			open_time,
			close_time,
			is_closed
		) VALUES ($1, 3, '21:00'::time, '09:00'::time, false)
	`, merchantID)
	requireBusinessHoursMigrationCheckViolation(t, err)
}

func requireBusinessHoursMigrationCheckViolation(t *testing.T, err error) {
	t.Helper()

	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23514", pgErr.Code)
	require.Equal(t, "merchant_business_hours_same_day_window_chk", pgErr.ConstraintName)
}
