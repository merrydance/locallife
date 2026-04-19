-- name: CreateMerchantCancelWithdrawApplication :one
INSERT INTO merchant_cancel_withdraw_applications (
    merchant_id,
    created_by_user_id,
    sub_mch_id,
    out_request_no,
    withdraw,
    business_license_status_declaration,
    proof_media_asset_ids,
    additional_material_asset_ids,
    remark,
    local_sync_state
) VALUES (
    sqlc.arg(merchant_id),
    sqlc.arg(created_by_user_id),
    sqlc.arg(sub_mch_id),
    sqlc.arg(out_request_no),
    sqlc.arg(withdraw),
    sqlc.narg(business_license_status_declaration),
    sqlc.arg(proof_media_asset_ids),
    sqlc.arg(additional_material_asset_ids),
    sqlc.narg(remark),
    sqlc.arg(local_sync_state)
) RETURNING *;

-- name: GetMerchantCancelWithdrawApplication :one
SELECT id, merchant_id, created_by_user_id, sub_mch_id, out_request_no, applyment_id, withdraw, proof_media_asset_ids, additional_material_asset_ids, remark, local_sync_state, cancel_state, cancel_state_description, withdraw_state, withdraw_state_description, confirm_cancel_url, account_info, account_withdraw_result, latest_query_response, last_error, modify_time, submitted_at, last_query_at, created_at, updated_at, business_license_status_declaration FROM merchant_cancel_withdraw_applications
WHERE id = $1 LIMIT 1;

-- name: GetMerchantCancelWithdrawApplicationByOutRequestNo :one
SELECT id, merchant_id, created_by_user_id, sub_mch_id, out_request_no, applyment_id, withdraw, proof_media_asset_ids, additional_material_asset_ids, remark, local_sync_state, cancel_state, cancel_state_description, withdraw_state, withdraw_state_description, confirm_cancel_url, account_info, account_withdraw_result, latest_query_response, last_error, modify_time, submitted_at, last_query_at, created_at, updated_at, business_license_status_declaration FROM merchant_cancel_withdraw_applications
WHERE out_request_no = $1 LIMIT 1;

-- name: GetMerchantCancelWithdrawApplicationByApplymentID :one
SELECT id, merchant_id, created_by_user_id, sub_mch_id, out_request_no, applyment_id, withdraw, proof_media_asset_ids, additional_material_asset_ids, remark, local_sync_state, cancel_state, cancel_state_description, withdraw_state, withdraw_state_description, confirm_cancel_url, account_info, account_withdraw_result, latest_query_response, last_error, modify_time, submitted_at, last_query_at, created_at, updated_at, business_license_status_declaration FROM merchant_cancel_withdraw_applications
WHERE applyment_id = $1 LIMIT 1;

-- name: ListMerchantCancelWithdrawApplicationsByMerchant :many
SELECT id, merchant_id, created_by_user_id, sub_mch_id, out_request_no, applyment_id, withdraw, proof_media_asset_ids, additional_material_asset_ids, remark, local_sync_state, cancel_state, cancel_state_description, withdraw_state, withdraw_state_description, confirm_cancel_url, account_info, account_withdraw_result, latest_query_response, last_error, modify_time, submitted_at, last_query_at, created_at, updated_at, business_license_status_declaration FROM merchant_cancel_withdraw_applications
WHERE merchant_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: CountMerchantCancelWithdrawApplicationsByMerchant :one
SELECT COUNT(*) FROM merchant_cancel_withdraw_applications
WHERE merchant_id = $1;

-- name: ListPendingMerchantCancelWithdrawApplications :many
SELECT id, merchant_id, created_by_user_id, sub_mch_id, out_request_no, applyment_id, withdraw, proof_media_asset_ids, additional_material_asset_ids, remark, local_sync_state, cancel_state, cancel_state_description, withdraw_state, withdraw_state_description, confirm_cancel_url, account_info, account_withdraw_result, latest_query_response, last_error, modify_time, submitted_at, last_query_at, created_at, updated_at, business_license_status_declaration FROM merchant_cancel_withdraw_applications
WHERE local_sync_state IN ('submit_succeeded', 'submit_unknown')
    AND COALESCE(cancel_state, '') NOT IN ('REJECTED', 'REVOKED', 'CANCELED', 'FINISH')
ORDER BY COALESCE(last_query_at, created_at) ASC, id ASC
LIMIT $1;

-- name: UpdateMerchantCancelWithdrawApplicationSync :one
UPDATE merchant_cancel_withdraw_applications
SET
    applyment_id = sqlc.narg(applyment_id),
    local_sync_state = sqlc.arg(local_sync_state),
    cancel_state = sqlc.narg(cancel_state),
    cancel_state_description = sqlc.narg(cancel_state_description),
    withdraw_state = sqlc.narg(withdraw_state),
    withdraw_state_description = sqlc.narg(withdraw_state_description),
    confirm_cancel_url = sqlc.narg(confirm_cancel_url),
    account_info = sqlc.narg(account_info),
    account_withdraw_result = sqlc.narg(account_withdraw_result),
    latest_query_response = sqlc.narg(latest_query_response),
    last_error = CASE
        WHEN sqlc.arg(clear_last_error)::bool THEN NULL
        ELSE sqlc.narg(last_error)
    END,
    modify_time = sqlc.narg(modify_time),
    submitted_at = CASE
        WHEN sqlc.arg(mark_submitted)::bool AND submitted_at IS NULL THEN now()
        ELSE submitted_at
    END,
    last_query_at = sqlc.narg(last_query_at),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;