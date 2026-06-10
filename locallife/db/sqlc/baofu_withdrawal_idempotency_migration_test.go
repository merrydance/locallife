package db

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestBaofuWithdrawalIdempotencyMigration261HardensAlreadyAppliedWeak260Schema(t *testing.T) {
	dbSource := createMembershipSettingsMigrationDB(t)
	runMembershipSettingsMigrations(t, dbSource, 259)

	pool := openMembershipSettingsMigrationPool(t, dbSource)
	defer pool.Close()

	_, err := pool.Exec(context.Background(), `
		ALTER TABLE baofu_withdrawal_orders
			ADD COLUMN IF NOT EXISTS idempotency_key TEXT,
			ADD COLUMN IF NOT EXISTS idempotency_request_hash TEXT;

		ALTER TABLE baofu_withdrawal_orders
			ADD CONSTRAINT baofu_withdrawal_orders_idempotency_pair_check
			CHECK (
				(idempotency_key IS NULL AND idempotency_request_hash IS NULL)
				OR
				(length(trim(idempotency_key)) > 0 AND length(trim(idempotency_request_hash)) > 0)
			);

		CREATE UNIQUE INDEX IF NOT EXISTS idx_baofu_withdrawal_orders_idempotency_uq
			ON baofu_withdrawal_orders(owner_type, owner_id, idempotency_key)
			WHERE idempotency_key IS NOT NULL;
	`)
	require.NoError(t, err)

	bindingID := insertBaofuWithdrawalMigrationBinding(t, pool)
	_, err = pool.Exec(context.Background(), `
		INSERT INTO baofu_withdrawal_orders (
			owner_type,
			owner_id,
			account_binding_id,
			out_request_no,
			amount,
			status,
			raw_snapshot,
			idempotency_key,
			idempotency_request_hash
		) VALUES
			('merchant', 91001, $1, 'WD_WEAK_KEY_ONLY', 1000, 'processing', '{}'::jsonb, 'weak-key', NULL),
			('merchant', 91002, $1, 'WD_WEAK_HASH_ONLY', 1000, 'processing', '{}'::jsonb, NULL, 'sha256:legacy')
	`, bindingID)
	require.NoError(t, err)

	forceMembershipSettingsMigrationVersion(t, dbSource, 260)
	runMembershipSettingsMigrations(t, dbSource, 261)

	rows, err := pool.Query(context.Background(), `
		SELECT out_request_no, idempotency_key, idempotency_request_hash
		FROM baofu_withdrawal_orders
		WHERE out_request_no LIKE 'WD_WEAK_%'
		ORDER BY out_request_no
	`)
	require.NoError(t, err)
	defer rows.Close()

	got := map[string]struct {
		key  *string
		hash *string
	}{}
	for rows.Next() {
		var outRequestNo string
		var key, hash *string
		require.NoError(t, rows.Scan(&outRequestNo, &key, &hash))
		got[outRequestNo] = struct {
			key  *string
			hash *string
		}{key: key, hash: hash}
	}
	require.NoError(t, rows.Err())
	require.Len(t, got, 2)
	require.Equal(t, "weak-key", requireStringPtr(t, got["WD_WEAK_KEY_ONLY"].key))
	require.True(t, strings.HasPrefix(requireStringPtr(t, got["WD_WEAK_KEY_ONLY"].hash), "legacy_missing_hash:"))
	require.Nil(t, got["WD_WEAK_HASH_ONLY"].key)
	require.Nil(t, got["WD_WEAK_HASH_ONLY"].hash)

	_, err = pool.Exec(context.Background(), `
		INSERT INTO baofu_withdrawal_orders (
			owner_type,
			owner_id,
			account_binding_id,
			out_request_no,
			amount,
			status,
			raw_snapshot,
			idempotency_key
		) VALUES ('merchant', 91005, $1, 'WD_WEAK_REJECTED_HALF', 1000, 'processing', '{}'::jsonb, 'new-half-key')
	`, bindingID)
	requireBaofuWithdrawalIdempotencyMigrationCheckViolation(t, err)
}

func insertBaofuWithdrawalMigrationBinding(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()

	var bindingID int64
	err := pool.QueryRow(context.Background(), `
		INSERT INTO baofu_account_bindings (
			owner_type,
			owner_id,
			account_type,
			opening_mode,
			open_state,
			raw_snapshot
		) VALUES ('merchant', 91001, 'business', 'business_public', 'processing', '{}'::jsonb)
		RETURNING id
	`).Scan(&bindingID)
	require.NoError(t, err)
	return bindingID
}

func requireStringPtr(t *testing.T, value *string) string {
	t.Helper()
	require.NotNil(t, value)
	return *value
}

func requireBaofuWithdrawalIdempotencyMigrationCheckViolation(t *testing.T, err error) {
	t.Helper()

	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "23514", pgErr.Code)
	require.Equal(t, "baofu_withdrawal_orders_idempotency_pair_check", pgErr.ConstraintName)
}
