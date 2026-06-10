package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestMerchantStaffInviteCodeMigration263CleansDuplicateActiveCodes(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 262)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	firstMerchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	secondMerchantID := insertMembershipSettingsMigrationMerchant(t, pool)
	duplicateCode := "mig263duplicatecode000000000001"

	_, err := pool.Exec(context.Background(), `
		UPDATE merchants
		SET bind_code = $1,
		    bind_code_expires_at = $2,
		    updated_at = now() - interval '1 hour'
		WHERE id = $3
	`, duplicateCode, time.Now().Add(time.Hour), firstMerchantID)
	require.NoError(t, err)

	_, err = pool.Exec(context.Background(), `
		UPDATE merchants
		SET bind_code = $1,
		    bind_code_expires_at = $2,
		    updated_at = now()
		WHERE id = $3
	`, duplicateCode, time.Now().Add(2*time.Hour), secondMerchantID)
	require.NoError(t, err)

	runMembershipSettingsMigrations(t, dbSource, 263)

	var retainedCount int
	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM merchants
		WHERE bind_code = $1
		  AND deleted_at IS NULL
	`, duplicateCode).Scan(&retainedCount)
	require.NoError(t, err)
	require.Equal(t, 1, retainedCount)

	var retainedMerchantID int64
	err = pool.QueryRow(context.Background(), `
		SELECT id
		FROM merchants
		WHERE bind_code = $1
	`, duplicateCode).Scan(&retainedMerchantID)
	require.NoError(t, err)
	require.Equal(t, secondMerchantID, retainedMerchantID)

	_, err = pool.Exec(context.Background(), `
		UPDATE merchants
		SET bind_code = $1,
		    bind_code_expires_at = now() + interval '3 hours'
		WHERE id = $2
	`, duplicateCode, firstMerchantID)
	requireMerchantInviteCodeMigrationUniqueViolation(t, err)
}

func requireMerchantInviteCodeMigrationUniqueViolation(t *testing.T, err error) {
	t.Helper()

	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, UniqueViolation, pgErr.Code)
	require.Equal(t, "merchants_active_bind_code_uidx", pgErr.ConstraintName)
}
