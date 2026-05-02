-- name: CreateEcommerceApplyment :one
INSERT INTO ecommerce_applyments (
    subject_type,
    subject_id,
    out_request_no,
    organization_type,
    business_license_number,
    business_license_copy,
    merchant_name,
    legal_person,
    id_card_number,
    id_card_name,
    id_card_valid_time,
    id_card_front_copy,
    id_card_back_copy,
    account_type,
    account_bank,
    account_bank_code,
    bank_alias,
    bank_alias_code,
    bank_address_code,
    bank_branch_id,
    bank_name,
    account_number,
    account_name,
    contact_name,
    contact_id_card_number,
    mobile_phone,
    contact_email,
    merchant_shortname,
    qualifications,
    business_addition_pics,
    business_addition_desc,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
    $31, 'pending'
) RETURNING *;

-- name: GetEcommerceApplyment :one
SELECT id, subject_type, subject_id, out_request_no, applyment_id, organization_type, business_license_number, business_license_copy, merchant_name, legal_person, id_card_number, id_card_name, id_card_valid_time, id_card_front_copy, id_card_back_copy, account_type, account_bank, bank_address_code, bank_name, account_number, account_name, contact_name, contact_id_card_number, mobile_phone, contact_email, merchant_shortname, qualifications, business_addition_pics, business_addition_desc, status, sign_url, sign_state, reject_reason, sub_mch_id, created_at, submitted_at, audited_at, updated_at, result_task_processed_state, result_task_processed_at, account_bank_code, bank_alias, bank_alias_code, bank_branch_id, settlement_verify_first_trade_at, settlement_verify_last_checked_at, settlement_verify_check_count, settlement_verify_status, settlement_verify_fail_reason, settlement_verify_failed_notified_at, legal_validation_url, account_validation, account_willingness_business_code, account_willingness_applyment_id, account_willingness_state, account_willingness_qrcode_data, account_willingness_reject_reason, account_authorize_state, account_authorize_state_checked_at FROM ecommerce_applyments
WHERE id = $1 LIMIT 1;

-- name: GetEcommerceApplymentByOutRequestNo :one
SELECT id, subject_type, subject_id, out_request_no, applyment_id, organization_type, business_license_number, business_license_copy, merchant_name, legal_person, id_card_number, id_card_name, id_card_valid_time, id_card_front_copy, id_card_back_copy, account_type, account_bank, bank_address_code, bank_name, account_number, account_name, contact_name, contact_id_card_number, mobile_phone, contact_email, merchant_shortname, qualifications, business_addition_pics, business_addition_desc, status, sign_url, sign_state, reject_reason, sub_mch_id, created_at, submitted_at, audited_at, updated_at, result_task_processed_state, result_task_processed_at, account_bank_code, bank_alias, bank_alias_code, bank_branch_id, settlement_verify_first_trade_at, settlement_verify_last_checked_at, settlement_verify_check_count, settlement_verify_status, settlement_verify_fail_reason, settlement_verify_failed_notified_at, legal_validation_url, account_validation, account_willingness_business_code, account_willingness_applyment_id, account_willingness_state, account_willingness_qrcode_data, account_willingness_reject_reason, account_authorize_state, account_authorize_state_checked_at FROM ecommerce_applyments
WHERE out_request_no = $1 LIMIT 1;

-- name: GetEcommerceApplymentByApplymentID :one
SELECT id, subject_type, subject_id, out_request_no, applyment_id, organization_type, business_license_number, business_license_copy, merchant_name, legal_person, id_card_number, id_card_name, id_card_valid_time, id_card_front_copy, id_card_back_copy, account_type, account_bank, bank_address_code, bank_name, account_number, account_name, contact_name, contact_id_card_number, mobile_phone, contact_email, merchant_shortname, qualifications, business_addition_pics, business_addition_desc, status, sign_url, sign_state, reject_reason, sub_mch_id, created_at, submitted_at, audited_at, updated_at, result_task_processed_state, result_task_processed_at, account_bank_code, bank_alias, bank_alias_code, bank_branch_id, settlement_verify_first_trade_at, settlement_verify_last_checked_at, settlement_verify_check_count, settlement_verify_status, settlement_verify_fail_reason, settlement_verify_failed_notified_at, legal_validation_url, account_validation, account_willingness_business_code, account_willingness_applyment_id, account_willingness_state, account_willingness_qrcode_data, account_willingness_reject_reason, account_authorize_state, account_authorize_state_checked_at FROM ecommerce_applyments
WHERE applyment_id = $1 LIMIT 1;

-- name: GetEcommerceApplymentBySubject :one
SELECT id, subject_type, subject_id, out_request_no, applyment_id, organization_type, business_license_number, business_license_copy, merchant_name, legal_person, id_card_number, id_card_name, id_card_valid_time, id_card_front_copy, id_card_back_copy, account_type, account_bank, bank_address_code, bank_name, account_number, account_name, contact_name, contact_id_card_number, mobile_phone, contact_email, merchant_shortname, qualifications, business_addition_pics, business_addition_desc, status, sign_url, sign_state, reject_reason, sub_mch_id, created_at, submitted_at, audited_at, updated_at, result_task_processed_state, result_task_processed_at, account_bank_code, bank_alias, bank_alias_code, bank_branch_id, settlement_verify_first_trade_at, settlement_verify_last_checked_at, settlement_verify_check_count, settlement_verify_status, settlement_verify_fail_reason, settlement_verify_failed_notified_at, legal_validation_url, account_validation, account_willingness_business_code, account_willingness_applyment_id, account_willingness_state, account_willingness_qrcode_data, account_willingness_reject_reason, account_authorize_state, account_authorize_state_checked_at FROM ecommerce_applyments
WHERE subject_type = $1 AND subject_id = $2
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetLatestEcommerceApplymentBySubject :one
SELECT id, subject_type, subject_id, out_request_no, applyment_id, organization_type, business_license_number, business_license_copy, merchant_name, legal_person, id_card_number, id_card_name, id_card_valid_time, id_card_front_copy, id_card_back_copy, account_type, account_bank, bank_address_code, bank_name, account_number, account_name, contact_name, contact_id_card_number, mobile_phone, contact_email, merchant_shortname, qualifications, business_addition_pics, business_addition_desc, status, sign_url, sign_state, reject_reason, sub_mch_id, created_at, submitted_at, audited_at, updated_at, result_task_processed_state, result_task_processed_at, account_bank_code, bank_alias, bank_alias_code, bank_branch_id, settlement_verify_first_trade_at, settlement_verify_last_checked_at, settlement_verify_check_count, settlement_verify_status, settlement_verify_fail_reason, settlement_verify_failed_notified_at, legal_validation_url, account_validation, account_willingness_business_code, account_willingness_applyment_id, account_willingness_state, account_willingness_qrcode_data, account_willingness_reject_reason, account_authorize_state, account_authorize_state_checked_at FROM ecommerce_applyments
WHERE subject_type = $1 AND subject_id = $2
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListEcommerceApplymentsBySubject :many
SELECT id, subject_type, subject_id, out_request_no, applyment_id, organization_type, business_license_number, business_license_copy, merchant_name, legal_person, id_card_number, id_card_name, id_card_valid_time, id_card_front_copy, id_card_back_copy, account_type, account_bank, bank_address_code, bank_name, account_number, account_name, contact_name, contact_id_card_number, mobile_phone, contact_email, merchant_shortname, qualifications, business_addition_pics, business_addition_desc, status, sign_url, sign_state, reject_reason, sub_mch_id, created_at, submitted_at, audited_at, updated_at, result_task_processed_state, result_task_processed_at, account_bank_code, bank_alias, bank_alias_code, bank_branch_id, settlement_verify_first_trade_at, settlement_verify_last_checked_at, settlement_verify_check_count, settlement_verify_status, settlement_verify_fail_reason, settlement_verify_failed_notified_at, legal_validation_url, account_validation, account_willingness_business_code, account_willingness_applyment_id, account_willingness_state, account_willingness_qrcode_data, account_willingness_reject_reason, account_authorize_state, account_authorize_state_checked_at FROM ecommerce_applyments
WHERE subject_type = $1 AND subject_id = $2
ORDER BY created_at DESC, id DESC;

-- name: ListEcommerceApplymentsByStatus :many
SELECT id, subject_type, subject_id, out_request_no, applyment_id, organization_type, business_license_number, business_license_copy, merchant_name, legal_person, id_card_number, id_card_name, id_card_valid_time, id_card_front_copy, id_card_back_copy, account_type, account_bank, bank_address_code, bank_name, account_number, account_name, contact_name, contact_id_card_number, mobile_phone, contact_email, merchant_shortname, qualifications, business_addition_pics, business_addition_desc, status, sign_url, sign_state, reject_reason, sub_mch_id, created_at, submitted_at, audited_at, updated_at, result_task_processed_state, result_task_processed_at, account_bank_code, bank_alias, bank_alias_code, bank_branch_id, settlement_verify_first_trade_at, settlement_verify_last_checked_at, settlement_verify_check_count, settlement_verify_status, settlement_verify_fail_reason, settlement_verify_failed_notified_at, legal_validation_url, account_validation, account_willingness_business_code, account_willingness_applyment_id, account_willingness_state, account_willingness_qrcode_data, account_willingness_reject_reason, account_authorize_state, account_authorize_state_checked_at FROM ecommerce_applyments
WHERE status = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: ListPendingEcommerceApplyments :many
SELECT id, subject_type, subject_id, out_request_no, applyment_id, organization_type, business_license_number, business_license_copy, merchant_name, legal_person, id_card_number, id_card_name, id_card_valid_time, id_card_front_copy, id_card_back_copy, account_type, account_bank, bank_address_code, bank_name, account_number, account_name, contact_name, contact_id_card_number, mobile_phone, contact_email, merchant_shortname, qualifications, business_addition_pics, business_addition_desc, status, sign_url, sign_state, reject_reason, sub_mch_id, created_at, submitted_at, audited_at, updated_at, result_task_processed_state, result_task_processed_at, account_bank_code, bank_alias, bank_alias_code, bank_branch_id, settlement_verify_first_trade_at, settlement_verify_last_checked_at, settlement_verify_check_count, settlement_verify_status, settlement_verify_fail_reason, settlement_verify_failed_notified_at, legal_validation_url, account_validation, account_willingness_business_code, account_willingness_applyment_id, account_willingness_state, account_willingness_qrcode_data, account_willingness_reject_reason, account_authorize_state, account_authorize_state_checked_at FROM ecommerce_applyments
WHERE status IN ('submitted', 'checking', 'auditing', 'account_need_verify', 'to_be_confirmed', 'to_be_signed', 'signing')
ORDER BY created_at ASC, id ASC
LIMIT $1 OFFSET $2;

-- name: UpdateEcommerceApplymentToSubmitted :one
UPDATE ecommerce_applyments
SET
    applyment_id = $2,
    status = 'submitted',
    submitted_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateEcommerceApplymentStatus :one
UPDATE ecommerce_applyments
SET
    applyment_id = COALESCE(sqlc.narg(applyment_id), applyment_id),
    status = $2,
    reject_reason = sqlc.narg(reject_reason),
    sign_url = sqlc.narg(sign_url),
    sign_state = sqlc.narg(sign_state),
    legal_validation_url = sqlc.narg(legal_validation_url),
    account_validation = sqlc.narg(account_validation),
    sub_mch_id = sqlc.narg(sub_mch_id),
    account_authorize_state = COALESCE(sqlc.narg(account_authorize_state), account_authorize_state),
    account_authorize_state_checked_at = COALESCE(sqlc.narg(account_authorize_state_checked_at), account_authorize_state_checked_at),
    audited_at = CASE WHEN $2 IN ('rejected', 'frozen', 'to_be_signed', 'finish') THEN now() ELSE audited_at END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateEcommerceApplymentSubMchID :one
UPDATE ecommerce_applyments
SET
    sub_mch_id = $2,
    status = 'finish',
    audited_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CountEcommerceApplymentsByStatus :one
SELECT COUNT(*) FROM ecommerce_applyments
WHERE status = $1;

-- name: ListMerchantApplymentsPendingSettlementVerification :many
WITH order_first_paid AS (
    SELECT o.merchant_id, MIN(po.paid_at) AS first_paid_at
    FROM payment_orders po
    JOIN orders o ON o.id = po.order_id
    WHERE po.status = 'paid'
      AND po.paid_at IS NOT NULL
      AND po.order_id IS NOT NULL
    GROUP BY o.merchant_id
),
reservation_first_paid AS (
    SELECT tr.merchant_id, MIN(po.paid_at) AS first_paid_at
    FROM payment_orders po
    JOIN table_reservations tr ON tr.id = po.reservation_id
    WHERE po.status = 'paid'
      AND po.paid_at IS NOT NULL
      AND po.reservation_id IS NOT NULL
    GROUP BY tr.merchant_id
),
merchant_first_trade AS (
    SELECT merchant_id, MIN(first_paid_at) AS first_paid_at
    FROM (
        SELECT merchant_id, first_paid_at FROM order_first_paid
        UNION ALL
        SELECT merchant_id, first_paid_at FROM reservation_first_paid
    ) t
    GROUP BY merchant_id
)
SELECT
    ea.id,
    ea.subject_id,
    ea.out_request_no,
    ea.applyment_id,
    ea.sub_mch_id,
    ea.settlement_verify_first_trade_at,
    ea.settlement_verify_last_checked_at,
    ea.settlement_verify_check_count,
    ea.settlement_verify_status,
    ea.settlement_verify_fail_reason,
    ea.settlement_verify_failed_notified_at,
        mft.first_paid_at::timestamptz AS first_paid_at
FROM ecommerce_applyments ea
JOIN merchant_first_trade mft ON mft.merchant_id = ea.subject_id
WHERE ea.subject_type = 'merchant'
  AND ea.status = 'finish'
  AND ea.sub_mch_id IS NOT NULL
  AND btrim(ea.sub_mch_id) <> ''
  AND COALESCE(ea.settlement_verify_status, '') NOT IN ('success', 'fail')
  AND ea.settlement_verify_check_count < 3
  AND (
        ea.settlement_verify_last_checked_at IS NULL OR
      ea.settlement_verify_last_checked_at < date_trunc('day', sqlc.arg('run_at')::timestamptz)
  )
  AND date_trunc('day', sqlc.arg('run_at')::timestamptz) >= date_trunc('day', COALESCE(ea.settlement_verify_first_trade_at, mft.first_paid_at))
  AND date_trunc('day', sqlc.arg('run_at')::timestamptz) < date_trunc('day', COALESCE(ea.settlement_verify_first_trade_at, mft.first_paid_at)) + INTERVAL '3 day'
ORDER BY COALESCE(ea.settlement_verify_first_trade_at, mft.first_paid_at) ASC, ea.id ASC
LIMIT sqlc.arg('limit');

-- name: UpdateEcommerceApplymentSettlementVerification :one
UPDATE ecommerce_applyments
SET
    settlement_verify_first_trade_at = COALESCE(sqlc.narg('settlement_verify_first_trade_at'), settlement_verify_first_trade_at),
    settlement_verify_last_checked_at = COALESCE(sqlc.narg('settlement_verify_last_checked_at'), settlement_verify_last_checked_at),
    settlement_verify_check_count = COALESCE(sqlc.narg('settlement_verify_check_count'), settlement_verify_check_count),
    settlement_verify_status = COALESCE(sqlc.narg('settlement_verify_status'), settlement_verify_status),
    settlement_verify_fail_reason = COALESCE(sqlc.narg('settlement_verify_fail_reason'), settlement_verify_fail_reason),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: MarkEcommerceApplymentSettlementVerifyFailedNotified :one
UPDATE ecommerce_applyments
SET
    settlement_verify_failed_notified_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;


-- name: UpdateEcommerceApplymentAccountAuthorizeState :one
UPDATE ecommerce_applyments
SET
    account_authorize_state = $2,
    account_authorize_state_checked_at = COALESCE(sqlc.narg(account_authorize_state_checked_at), now()),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateEcommerceApplymentAccountWillingness :one
UPDATE ecommerce_applyments
SET
    account_willingness_business_code = COALESCE(sqlc.narg(account_willingness_business_code), account_willingness_business_code),
    account_willingness_applyment_id = COALESCE(sqlc.narg(account_willingness_applyment_id), account_willingness_applyment_id),
    account_willingness_state = COALESCE(sqlc.narg(account_willingness_state), account_willingness_state),
    account_willingness_qrcode_data = COALESCE(sqlc.narg(account_willingness_qrcode_data), account_willingness_qrcode_data),
    account_willingness_reject_reason = COALESCE(sqlc.narg(account_willingness_reject_reason), account_willingness_reject_reason),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
