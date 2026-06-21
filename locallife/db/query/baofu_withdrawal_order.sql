-- name: CreateBaofuWithdrawalOrder :one
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
) VALUES (
    sqlc.arg(owner_type),
    sqlc.arg(owner_id),
    sqlc.arg(account_binding_id),
    sqlc.arg(out_request_no),
    sqlc.arg(amount),
    sqlc.arg(status),
    COALESCE(sqlc.narg(raw_snapshot), '{}'::jsonb),
    sqlc.narg(idempotency_key),
    sqlc.narg(idempotency_request_hash)
) RETURNING *;

-- name: GetBaofuWithdrawalOrderForUpdate :one
SELECT id, owner_type, owner_id, account_binding_id, out_request_no, baofu_withdraw_no, amount, status, raw_snapshot, finished_at, created_at, updated_at, idempotency_key, idempotency_request_hash
FROM baofu_withdrawal_orders
WHERE id = $1
LIMIT 1
FOR UPDATE;

-- name: GetBaofuWithdrawalOrder :one
SELECT id, owner_type, owner_id, account_binding_id, out_request_no, baofu_withdraw_no, amount, status, raw_snapshot, finished_at, created_at, updated_at, idempotency_key, idempotency_request_hash
FROM baofu_withdrawal_orders
WHERE id = $1
LIMIT 1;

-- name: GetBaofuWithdrawalOrderByOutRequestNo :one
SELECT id, owner_type, owner_id, account_binding_id, out_request_no, baofu_withdraw_no, amount, status, raw_snapshot, finished_at, created_at, updated_at, idempotency_key, idempotency_request_hash
FROM baofu_withdrawal_orders
WHERE out_request_no = $1
LIMIT 1;

-- name: GetBaofuWithdrawalOrderByIdempotency :one
SELECT id, owner_type, owner_id, account_binding_id, out_request_no, baofu_withdraw_no, amount, status, raw_snapshot, finished_at, created_at, updated_at, idempotency_key, idempotency_request_hash
FROM baofu_withdrawal_orders
WHERE owner_type = sqlc.arg(owner_type)
  AND owner_id = sqlc.arg(owner_id)
  AND idempotency_key = sqlc.arg(idempotency_key)
LIMIT 1;

-- name: ListBaofuWithdrawalOrdersByOwner :many
SELECT id, owner_type, owner_id, account_binding_id, out_request_no, baofu_withdraw_no, amount, status, raw_snapshot, finished_at, created_at, updated_at, idempotency_key, idempotency_request_hash
FROM baofu_withdrawal_orders
WHERE owner_type = sqlc.arg(owner_type)
  AND owner_id = sqlc.arg(owner_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_count)::int
OFFSET sqlc.arg(offset_count)::int;

-- name: CountBaofuWithdrawalOrdersByOwner :one
SELECT COUNT(*)::bigint
FROM baofu_withdrawal_orders
WHERE owner_type = sqlc.arg(owner_type)
  AND owner_id = sqlc.arg(owner_id);

-- name: ListProcessingBaofuWithdrawalOrdersForRecovery :many
SELECT id, owner_type, owner_id, account_binding_id, out_request_no, baofu_withdraw_no, amount, status, raw_snapshot, finished_at, created_at, updated_at, idempotency_key, idempotency_request_hash
FROM baofu_withdrawal_orders
WHERE status = 'processing'
  AND created_at <= sqlc.arg(created_before)
ORDER BY created_at ASC, id ASC
LIMIT sqlc.arg(limit_count)::int;

-- name: UpdateBaofuWithdrawalOrderToProcessing :one
UPDATE baofu_withdrawal_orders
SET
    status = 'processing',
    baofu_withdraw_no = sqlc.narg(baofu_withdraw_no),
    raw_snapshot = COALESCE(sqlc.narg(raw_snapshot), raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND status = 'processing'
RETURNING *;

-- name: UpdateBaofuWithdrawalOrderStatus :one
UPDATE baofu_withdrawal_orders
SET
    status = sqlc.arg(status),
    baofu_withdraw_no = COALESCE(sqlc.narg(baofu_withdraw_no), baofu_withdraw_no),
    raw_snapshot = COALESCE(sqlc.narg(raw_snapshot), raw_snapshot),
    finished_at = CASE WHEN sqlc.arg(status) IN ('succeeded', 'failed', 'returned') THEN now() ELSE finished_at END,
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND status = 'processing'
RETURNING *;

-- name: UpsertBaofuWithdrawalAccountGuardBalance :one
INSERT INTO baofu_withdrawal_account_guards (
    owner_type,
    owner_id,
    account_binding_id,
    provider_available_amount_fen,
    provider_pending_amount_fen,
    provider_ledger_amount_fen,
    provider_frozen_amount_fen,
    provider_balance_observed_at
) VALUES (
    sqlc.arg(owner_type),
    sqlc.arg(owner_id),
    sqlc.arg(account_binding_id),
    sqlc.arg(provider_available_amount_fen),
    sqlc.arg(provider_pending_amount_fen),
    sqlc.arg(provider_ledger_amount_fen),
    sqlc.arg(provider_frozen_amount_fen),
    sqlc.arg(provider_balance_observed_at)
)
ON CONFLICT (owner_type, owner_id, account_binding_id) DO UPDATE SET
    provider_available_amount_fen = CASE
        WHEN baofu_withdrawal_account_guards.provider_balance_observed_at IS NULL
             OR EXCLUDED.provider_balance_observed_at >= baofu_withdrawal_account_guards.provider_balance_observed_at
        THEN EXCLUDED.provider_available_amount_fen
        ELSE baofu_withdrawal_account_guards.provider_available_amount_fen
    END,
    provider_pending_amount_fen = CASE
        WHEN baofu_withdrawal_account_guards.provider_balance_observed_at IS NULL
             OR EXCLUDED.provider_balance_observed_at >= baofu_withdrawal_account_guards.provider_balance_observed_at
        THEN EXCLUDED.provider_pending_amount_fen
        ELSE baofu_withdrawal_account_guards.provider_pending_amount_fen
    END,
    provider_ledger_amount_fen = CASE
        WHEN baofu_withdrawal_account_guards.provider_balance_observed_at IS NULL
             OR EXCLUDED.provider_balance_observed_at >= baofu_withdrawal_account_guards.provider_balance_observed_at
        THEN EXCLUDED.provider_ledger_amount_fen
        ELSE baofu_withdrawal_account_guards.provider_ledger_amount_fen
    END,
    provider_frozen_amount_fen = CASE
        WHEN baofu_withdrawal_account_guards.provider_balance_observed_at IS NULL
             OR EXCLUDED.provider_balance_observed_at >= baofu_withdrawal_account_guards.provider_balance_observed_at
        THEN EXCLUDED.provider_frozen_amount_fen
        ELSE baofu_withdrawal_account_guards.provider_frozen_amount_fen
    END,
    provider_balance_observed_at = CASE
        WHEN baofu_withdrawal_account_guards.provider_balance_observed_at IS NULL
             OR EXCLUDED.provider_balance_observed_at >= baofu_withdrawal_account_guards.provider_balance_observed_at
        THEN EXCLUDED.provider_balance_observed_at
        ELSE baofu_withdrawal_account_guards.provider_balance_observed_at
    END,
    updated_at = now()
RETURNING id, owner_type, owner_id, account_binding_id, provider_available_amount_fen, provider_pending_amount_fen, provider_ledger_amount_fen, provider_frozen_amount_fen, provider_balance_observed_at, reserved_amount_fen, consumed_withdraw_amount_fen, created_at, updated_at;

-- name: GetBaofuWithdrawalAccountGuardByOwner :one
SELECT id, owner_type, owner_id, account_binding_id, provider_available_amount_fen, provider_pending_amount_fen, provider_ledger_amount_fen, provider_frozen_amount_fen, provider_balance_observed_at, reserved_amount_fen, consumed_withdraw_amount_fen, created_at, updated_at
FROM baofu_withdrawal_account_guards
WHERE owner_type = sqlc.arg(owner_type)
  AND owner_id = sqlc.arg(owner_id)
  AND account_binding_id = sqlc.arg(account_binding_id)
LIMIT 1;

-- name: GetBaofuWithdrawalAccountGuardByOwnerForUpdate :one
SELECT id, owner_type, owner_id, account_binding_id, provider_available_amount_fen, provider_pending_amount_fen, provider_ledger_amount_fen, provider_frozen_amount_fen, provider_balance_observed_at, reserved_amount_fen, consumed_withdraw_amount_fen, created_at, updated_at
FROM baofu_withdrawal_account_guards
WHERE owner_type = sqlc.arg(owner_type)
  AND owner_id = sqlc.arg(owner_id)
  AND account_binding_id = sqlc.arg(account_binding_id)
LIMIT 1
FOR UPDATE;

-- name: ReserveBaofuWithdrawalAccountGuardAmount :one
UPDATE baofu_withdrawal_account_guards
SET
    reserved_amount_fen = reserved_amount_fen + sqlc.arg(amount_fen),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND provider_available_amount_fen >= reserved_amount_fen + sqlc.arg(amount_fen)
RETURNING id, owner_type, owner_id, account_binding_id, provider_available_amount_fen, provider_pending_amount_fen, provider_ledger_amount_fen, provider_frozen_amount_fen, provider_balance_observed_at, reserved_amount_fen, consumed_withdraw_amount_fen, created_at, updated_at;

-- name: ConsumeBaofuWithdrawalAccountGuardAmount :one
UPDATE baofu_withdrawal_account_guards
SET
    reserved_amount_fen = reserved_amount_fen - sqlc.arg(amount_fen),
    consumed_withdraw_amount_fen = consumed_withdraw_amount_fen + sqlc.arg(amount_fen),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND reserved_amount_fen >= sqlc.arg(amount_fen)
RETURNING id, owner_type, owner_id, account_binding_id, provider_available_amount_fen, provider_pending_amount_fen, provider_ledger_amount_fen, provider_frozen_amount_fen, provider_balance_observed_at, reserved_amount_fen, consumed_withdraw_amount_fen, created_at, updated_at;

-- name: ReleaseBaofuWithdrawalAccountGuardAmount :one
UPDATE baofu_withdrawal_account_guards
SET
    reserved_amount_fen = reserved_amount_fen - sqlc.arg(amount_fen),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND reserved_amount_fen >= sqlc.arg(amount_fen)
RETURNING id, owner_type, owner_id, account_binding_id, provider_available_amount_fen, provider_pending_amount_fen, provider_ledger_amount_fen, provider_frozen_amount_fen, provider_balance_observed_at, reserved_amount_fen, consumed_withdraw_amount_fen, created_at, updated_at;

-- name: CreateBaofuWithdrawalReservation :one
INSERT INTO baofu_withdrawal_reservations (
    withdrawal_order_id,
    owner_type,
    owner_id,
    account_binding_id,
    amount_fen,
    status,
    reserved_at
) VALUES (
    sqlc.arg(withdrawal_order_id),
    sqlc.arg(owner_type),
    sqlc.arg(owner_id),
    sqlc.arg(account_binding_id),
    sqlc.arg(amount_fen),
    'reserved',
    sqlc.arg(reserved_at)
)
RETURNING id, withdrawal_order_id, owner_type, owner_id, account_binding_id, amount_fen, status, release_reason, reserved_at, consumed_at, released_at, created_at, updated_at;

-- name: GetBaofuWithdrawalReservationByOrderID :one
SELECT id, withdrawal_order_id, owner_type, owner_id, account_binding_id, amount_fen, status, release_reason, reserved_at, consumed_at, released_at, created_at, updated_at
FROM baofu_withdrawal_reservations
WHERE withdrawal_order_id = $1
LIMIT 1;

-- name: GetBaofuWithdrawalReservationByOrderIDForUpdate :one
SELECT id, withdrawal_order_id, owner_type, owner_id, account_binding_id, amount_fen, status, release_reason, reserved_at, consumed_at, released_at, created_at, updated_at
FROM baofu_withdrawal_reservations
WHERE withdrawal_order_id = $1
LIMIT 1
FOR UPDATE;

-- name: ConsumeBaofuWithdrawalReservation :one
UPDATE baofu_withdrawal_reservations
SET
    status = 'consumed',
    consumed_at = sqlc.arg(consumed_at),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND status = 'reserved'
RETURNING id, withdrawal_order_id, owner_type, owner_id, account_binding_id, amount_fen, status, release_reason, reserved_at, consumed_at, released_at, created_at, updated_at;

-- name: ReleaseBaofuWithdrawalReservation :one
UPDATE baofu_withdrawal_reservations
SET
    status = 'released',
    release_reason = sqlc.narg(release_reason),
    released_at = sqlc.arg(released_at),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND status = 'reserved'
RETURNING id, withdrawal_order_id, owner_type, owner_id, account_binding_id, amount_fen, status, release_reason, reserved_at, consumed_at, released_at, created_at, updated_at;
