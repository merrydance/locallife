-- name: UpsertBaofuAccountBinding :one
INSERT INTO baofu_account_bindings (
    owner_type,
    owner_id,
    account_type,
    login_no,
    open_state,
    wechat_sub_mch_id,
    last_open_trans_serial_no,
    raw_snapshot
) VALUES (
    sqlc.arg(owner_type),
    sqlc.arg(owner_id),
    sqlc.arg(account_type),
    sqlc.narg(login_no),
    sqlc.arg(open_state),
    sqlc.narg(wechat_sub_mch_id),
    sqlc.narg(last_open_trans_serial_no),
    sqlc.arg(raw_snapshot)
)
ON CONFLICT (owner_type, owner_id)
DO UPDATE SET
    account_type = EXCLUDED.account_type,
    login_no = EXCLUDED.login_no,
    open_state = EXCLUDED.open_state,
    wechat_sub_mch_id = EXCLUDED.wechat_sub_mch_id,
    last_open_trans_serial_no = EXCLUDED.last_open_trans_serial_no,
    raw_snapshot = EXCLUDED.raw_snapshot,
    updated_at = now()
RETURNING id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at;

-- name: GetBaofuAccountBinding :one
SELECT id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at
FROM baofu_account_bindings
WHERE id = $1
LIMIT 1;

-- name: GetBaofuAccountBindingByOwner :one
SELECT id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at
FROM baofu_account_bindings
WHERE owner_type = $1 AND owner_id = $2
LIMIT 1;

-- name: GetBaofuAccountBindingByContractNo :one
SELECT id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at
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
RETURNING id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at;

-- name: MarkBaofuAccountBindingActive :one
UPDATE baofu_account_bindings
SET open_state = 'active',
    contract_no = sqlc.narg(contract_no),
    sharing_mer_id = COALESCE(NULLIF(sqlc.narg(sharing_mer_id)::text, ''), NULLIF(sqlc.narg(contract_no)::text, '')),
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at;

-- name: MarkBaofuAccountBindingFailed :one
UPDATE baofu_account_bindings
SET open_state = 'failed',
    raw_snapshot = sqlc.arg(raw_snapshot),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at;

-- name: ListProcessingBaofuAccountBindings :many
SELECT id, owner_type, owner_id, account_type, contract_no, sharing_mer_id, login_no, open_state, wechat_sub_mch_id, bank_card_last4, last_open_trans_serial_no, raw_snapshot, created_at, updated_at
FROM baofu_account_bindings
WHERE open_state IN ('processing', 'abnormal')
  AND updated_at <= sqlc.arg(before_at)
ORDER BY updated_at ASC, id ASC
LIMIT sqlc.arg(limit_count);
