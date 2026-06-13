-- name: UpsertBaofuAccountBinding :one
INSERT INTO baofu_account_bindings (
    owner_type,
    owner_id,
    account_type,
    opening_mode,
    login_no,
    open_state,
    wechat_sub_mch_id,
    last_open_trans_serial_no,
    raw_snapshot
) VALUES (
    sqlc.arg(owner_type),
    sqlc.arg(owner_id),
    sqlc.arg(account_type),
    sqlc.arg(opening_mode),
    sqlc.narg(login_no),
    sqlc.arg(open_state),
    sqlc.narg(wechat_sub_mch_id),
    sqlc.narg(last_open_trans_serial_no),
    sqlc.arg(raw_snapshot)
)
ON CONFLICT (owner_type, owner_id)
DO UPDATE SET
    account_type = EXCLUDED.account_type,
    opening_mode = EXCLUDED.opening_mode,
    login_no = EXCLUDED.login_no,
    open_state = EXCLUDED.open_state,
    wechat_sub_mch_id = EXCLUDED.wechat_sub_mch_id,
    last_open_trans_serial_no = EXCLUDED.last_open_trans_serial_no,
    raw_snapshot = EXCLUDED.raw_snapshot,
    updated_at = now()
RETURNING id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at, opening_mode;

-- name: GetBaofuAccountBinding :one
SELECT id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at, opening_mode
FROM baofu_account_bindings
WHERE id = $1
LIMIT 1;

-- name: GetBaofuAccountBindingByOwner :one
SELECT id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at, opening_mode
FROM baofu_account_bindings
WHERE owner_type = $1 AND owner_id = $2
LIMIT 1;

-- name: GetBaofuAccountBindingByOwnerForUpdate :one
SELECT id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at, opening_mode
FROM baofu_account_bindings
WHERE owner_type = $1 AND owner_id = $2
LIMIT 1
FOR UPDATE;

-- name: GetBaofuAccountBindingByContractNo :one
SELECT id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at, opening_mode
FROM baofu_account_bindings
WHERE contract_no = $1
LIMIT 1;

-- name: MarkBaofuAccountBindingProcessing :one
UPDATE baofu_account_bindings
SET open_state = 'processing',
    last_open_trans_serial_no = sqlc.narg(last_open_trans_serial_no),
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at, opening_mode;

-- name: MarkBaofuAccountBindingActive :one
UPDATE baofu_account_bindings
SET open_state = 'active',
    contract_no = sqlc.narg(contract_no),
    sharing_mer_id = NULLIF(sqlc.narg(sharing_mer_id)::text, ''),
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at, opening_mode;

-- name: MarkBaofuAccountBindingFailed :one
UPDATE baofu_account_bindings
SET open_state = 'failed',
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at, opening_mode;

-- name: MarkBaofuAccountBindingAbnormal :one
UPDATE baofu_account_bindings
SET open_state = 'abnormal',
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at, opening_mode;

-- name: ListProcessingBaofuAccountBindings :many
SELECT id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at, opening_mode
FROM baofu_account_bindings
WHERE open_state IN ('processing', 'abnormal')
  AND updated_at <= sqlc.arg(before_at)
ORDER BY updated_at ASC, id ASC
LIMIT sqlc.arg(limit_count);
