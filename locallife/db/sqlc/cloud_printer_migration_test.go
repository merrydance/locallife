package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestCloudPrinterSoftDeleteMigrationDownHandlesReusedSerialNumber(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 255)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	merchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	printerSN := "MIG-SN-" + time.Now().Format("20060102150405.000000000")

	var deletedPrinterID int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO cloud_printers (
			merchant_id,
			printer_name,
			printer_sn,
			printer_key,
			printer_type,
			printer_role,
			is_active,
			deleted_at
		) VALUES ($1, 'deleted printer', $2, 'key-old', 'feieyun', 'front', false, now())
		RETURNING id
	`, merchantID, printerSN).Scan(&deletedPrinterID)
	require.NoError(t, err)

	var activePrinterID int64
	err = pool.QueryRow(context.Background(), `
		INSERT INTO cloud_printers (
			merchant_id,
			printer_name,
			printer_sn,
			printer_key,
			printer_type,
			printer_role,
			is_active
		) VALUES ($1, 'active printer', $2, 'key-new', 'feieyun', 'front', true)
		RETURNING id
	`, merchantID, printerSN).Scan(&activePrinterID)
	require.NoError(t, err)
	require.NotEqual(t, deletedPrinterID, activePrinterID)

	mig, err := migrate.New(membershipSettingsMigrationURL(t), dbSource)
	require.NoError(t, err)
	err = mig.Steps(-1)
	closeMembershipSettingsMigration(t, mig)
	require.NoError(t, err)

	var hasDeletedAt bool
	err = pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_name = 'cloud_printers'
				AND column_name = 'deleted_at'
		)
	`).Scan(&hasDeletedAt)
	require.NoError(t, err)
	require.False(t, hasDeletedAt)

	var activeSN, tombstoneSN string
	err = pool.QueryRow(context.Background(), `
		SELECT printer_sn
		FROM cloud_printers
		WHERE id = $1
	`, activePrinterID).Scan(&activeSN)
	require.NoError(t, err)
	require.Equal(t, printerSN, activeSN)

	err = pool.QueryRow(context.Background(), `
		SELECT printer_sn
		FROM cloud_printers
		WHERE id = $1
	`, deletedPrinterID).Scan(&tombstoneSN)
	require.NoError(t, err)
	require.NotEqual(t, printerSN, tombstoneSN)
	require.Contains(t, tombstoneSN, fmt.Sprintf("__deleted_%d", deletedPrinterID))

	_, err = pool.Exec(context.Background(), `
		INSERT INTO cloud_printers (
			merchant_id,
			printer_name,
			printer_sn,
			printer_key,
			printer_type,
			printer_role,
			is_active
		) VALUES ($1, 'duplicate after down', $2, 'key-dup', 'feieyun', 'front', true)
	`, merchantID, printerSN)
	requireCloudPrinterMigrationUniqueViolation(t, err, "cloud_printers_printer_sn_idx")
}

func requireCloudPrinterMigrationUniqueViolation(t *testing.T, err error, constraintName string) {
	t.Helper()

	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23505", pgErr.Code)
	require.Equal(t, constraintName, pgErr.ConstraintName)
}
