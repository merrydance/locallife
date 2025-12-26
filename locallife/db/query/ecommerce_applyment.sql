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
    bank_address_code,
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
    $21, $22, $23, $24, $25, $26, $27, 'pending'
) RETURNING *;

-- name: GetEcommerceApplyment :one
SELECT * FROM ecommerce_applyments
WHERE id = $1 LIMIT 1;

-- name: GetEcommerceApplymentByOutRequestNo :one
SELECT * FROM ecommerce_applyments
WHERE out_request_no = $1 LIMIT 1;

-- name: GetEcommerceApplymentByApplymentID :one
SELECT * FROM ecommerce_applyments
WHERE applyment_id = $1 LIMIT 1;

-- name: GetEcommerceApplymentBySubject :one
SELECT * FROM ecommerce_applyments
WHERE subject_type = $1 AND subject_id = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: GetLatestEcommerceApplymentBySubject :one
SELECT * FROM ecommerce_applyments
WHERE subject_type = $1 AND subject_id = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: ListEcommerceApplymentsBySubject :many
SELECT * FROM ecommerce_applyments
WHERE subject_type = $1 AND subject_id = $2
ORDER BY created_at DESC;

-- name: ListEcommerceApplymentsByStatus :many
SELECT * FROM ecommerce_applyments
WHERE status = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListPendingEcommerceApplyments :many
SELECT * FROM ecommerce_applyments
WHERE status IN ('submitted', 'auditing', 'to_be_signed', 'signing')
ORDER BY created_at ASC
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
    status = $2,
    reject_reason = sqlc.narg(reject_reason),
    sign_url = sqlc.narg(sign_url),
    sign_state = sqlc.narg(sign_state),
    sub_mch_id = sqlc.narg(sub_mch_id),
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
