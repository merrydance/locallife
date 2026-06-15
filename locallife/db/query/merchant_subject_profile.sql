-- name: UpsertMerchantSubjectProfile :one
INSERT INTO merchant_subject_profiles (
    merchant_application_id,
    merchant_id,
    user_id,
    business_license_number,
    business_license_name,
    business_license_address,
    legal_person_name,
    legal_person_id_number,
    food_permit_number,
    food_permit_company_name,
    business_license_media_asset_id,
    food_permit_media_asset_id,
    id_card_front_media_asset_id,
    id_card_back_media_asset_id,
    business_license_payload,
    food_permit_payload,
    legal_person_payload,
    source_snapshot
) VALUES (
    sqlc.arg(merchant_application_id),
    sqlc.narg(merchant_id),
    sqlc.arg(user_id),
    sqlc.arg(business_license_number),
    sqlc.arg(business_license_name),
    sqlc.arg(business_license_address),
    sqlc.arg(legal_person_name),
    sqlc.arg(legal_person_id_number),
    sqlc.arg(food_permit_number),
    sqlc.arg(food_permit_company_name),
    sqlc.narg(business_license_media_asset_id),
    sqlc.narg(food_permit_media_asset_id),
    sqlc.narg(id_card_front_media_asset_id),
    sqlc.narg(id_card_back_media_asset_id),
    sqlc.arg(business_license_payload),
    sqlc.arg(food_permit_payload),
    sqlc.arg(legal_person_payload),
    sqlc.arg(source_snapshot)
)
ON CONFLICT (merchant_application_id) DO UPDATE SET
    merchant_id = COALESCE(EXCLUDED.merchant_id, merchant_subject_profiles.merchant_id),
    user_id = EXCLUDED.user_id,
    business_license_number = EXCLUDED.business_license_number,
    business_license_name = EXCLUDED.business_license_name,
    business_license_address = EXCLUDED.business_license_address,
    legal_person_name = EXCLUDED.legal_person_name,
    legal_person_id_number = EXCLUDED.legal_person_id_number,
    food_permit_number = EXCLUDED.food_permit_number,
    food_permit_company_name = EXCLUDED.food_permit_company_name,
    business_license_media_asset_id = EXCLUDED.business_license_media_asset_id,
    food_permit_media_asset_id = EXCLUDED.food_permit_media_asset_id,
    id_card_front_media_asset_id = EXCLUDED.id_card_front_media_asset_id,
    id_card_back_media_asset_id = EXCLUDED.id_card_back_media_asset_id,
    business_license_payload = EXCLUDED.business_license_payload,
    food_permit_payload = EXCLUDED.food_permit_payload,
    legal_person_payload = EXCLUDED.legal_person_payload,
    source_snapshot = EXCLUDED.source_snapshot,
    version = CASE WHEN (
        merchant_subject_profiles.merchant_id IS DISTINCT FROM COALESCE(EXCLUDED.merchant_id, merchant_subject_profiles.merchant_id)
        OR merchant_subject_profiles.user_id IS DISTINCT FROM EXCLUDED.user_id
        OR merchant_subject_profiles.business_license_number IS DISTINCT FROM EXCLUDED.business_license_number
        OR merchant_subject_profiles.business_license_name IS DISTINCT FROM EXCLUDED.business_license_name
        OR merchant_subject_profiles.business_license_address IS DISTINCT FROM EXCLUDED.business_license_address
        OR merchant_subject_profiles.legal_person_name IS DISTINCT FROM EXCLUDED.legal_person_name
        OR merchant_subject_profiles.legal_person_id_number IS DISTINCT FROM EXCLUDED.legal_person_id_number
        OR merchant_subject_profiles.food_permit_number IS DISTINCT FROM EXCLUDED.food_permit_number
        OR merchant_subject_profiles.food_permit_company_name IS DISTINCT FROM EXCLUDED.food_permit_company_name
        OR merchant_subject_profiles.business_license_media_asset_id IS DISTINCT FROM EXCLUDED.business_license_media_asset_id
        OR merchant_subject_profiles.food_permit_media_asset_id IS DISTINCT FROM EXCLUDED.food_permit_media_asset_id
        OR merchant_subject_profiles.id_card_front_media_asset_id IS DISTINCT FROM EXCLUDED.id_card_front_media_asset_id
        OR merchant_subject_profiles.id_card_back_media_asset_id IS DISTINCT FROM EXCLUDED.id_card_back_media_asset_id
        OR merchant_subject_profiles.business_license_payload IS DISTINCT FROM EXCLUDED.business_license_payload
        OR merchant_subject_profiles.food_permit_payload IS DISTINCT FROM EXCLUDED.food_permit_payload
        OR merchant_subject_profiles.legal_person_payload IS DISTINCT FROM EXCLUDED.legal_person_payload
        OR merchant_subject_profiles.source_snapshot IS DISTINCT FROM EXCLUDED.source_snapshot
    ) THEN merchant_subject_profiles.version + 1 ELSE merchant_subject_profiles.version END,
    updated_at = CASE WHEN (
        merchant_subject_profiles.merchant_id IS DISTINCT FROM COALESCE(EXCLUDED.merchant_id, merchant_subject_profiles.merchant_id)
        OR merchant_subject_profiles.user_id IS DISTINCT FROM EXCLUDED.user_id
        OR merchant_subject_profiles.business_license_number IS DISTINCT FROM EXCLUDED.business_license_number
        OR merchant_subject_profiles.business_license_name IS DISTINCT FROM EXCLUDED.business_license_name
        OR merchant_subject_profiles.business_license_address IS DISTINCT FROM EXCLUDED.business_license_address
        OR merchant_subject_profiles.legal_person_name IS DISTINCT FROM EXCLUDED.legal_person_name
        OR merchant_subject_profiles.legal_person_id_number IS DISTINCT FROM EXCLUDED.legal_person_id_number
        OR merchant_subject_profiles.food_permit_number IS DISTINCT FROM EXCLUDED.food_permit_number
        OR merchant_subject_profiles.food_permit_company_name IS DISTINCT FROM EXCLUDED.food_permit_company_name
        OR merchant_subject_profiles.business_license_media_asset_id IS DISTINCT FROM EXCLUDED.business_license_media_asset_id
        OR merchant_subject_profiles.food_permit_media_asset_id IS DISTINCT FROM EXCLUDED.food_permit_media_asset_id
        OR merchant_subject_profiles.id_card_front_media_asset_id IS DISTINCT FROM EXCLUDED.id_card_front_media_asset_id
        OR merchant_subject_profiles.id_card_back_media_asset_id IS DISTINCT FROM EXCLUDED.id_card_back_media_asset_id
        OR merchant_subject_profiles.business_license_payload IS DISTINCT FROM EXCLUDED.business_license_payload
        OR merchant_subject_profiles.food_permit_payload IS DISTINCT FROM EXCLUDED.food_permit_payload
        OR merchant_subject_profiles.legal_person_payload IS DISTINCT FROM EXCLUDED.legal_person_payload
        OR merchant_subject_profiles.source_snapshot IS DISTINCT FROM EXCLUDED.source_snapshot
    ) THEN now() ELSE merchant_subject_profiles.updated_at END
RETURNING id, merchant_application_id, merchant_id, user_id, business_license_number,
    business_license_name, business_license_address, legal_person_name,
    legal_person_id_number, food_permit_number, food_permit_company_name,
    business_license_media_asset_id, food_permit_media_asset_id,
    id_card_front_media_asset_id, id_card_back_media_asset_id,
    business_license_payload, food_permit_payload, legal_person_payload,
    source_snapshot, version, created_at, updated_at;

-- name: DetachMerchantSubjectProfileMerchantFromOtherApplications :execrows
UPDATE merchant_subject_profiles
SET
    merchant_id = NULL,
    version = version + 1,
    updated_at = now()
WHERE merchant_id = sqlc.arg(merchant_id)
  AND merchant_application_id <> sqlc.arg(merchant_application_id);

-- name: GetMerchantSubjectProfileByApplication :one
SELECT id, merchant_application_id, merchant_id, user_id, business_license_number,
    business_license_name, business_license_address, legal_person_name,
    legal_person_id_number, food_permit_number, food_permit_company_name,
    business_license_media_asset_id, food_permit_media_asset_id,
    id_card_front_media_asset_id, id_card_back_media_asset_id,
    business_license_payload, food_permit_payload, legal_person_payload,
    source_snapshot, version, created_at, updated_at
FROM merchant_subject_profiles
WHERE merchant_application_id = $1;

-- name: GetMerchantSubjectProfileByMerchant :one
SELECT id, merchant_application_id, merchant_id, user_id, business_license_number,
    business_license_name, business_license_address, legal_person_name,
    legal_person_id_number, food_permit_number, food_permit_company_name,
    business_license_media_asset_id, food_permit_media_asset_id,
    id_card_front_media_asset_id, id_card_back_media_asset_id,
    business_license_payload, food_permit_payload, legal_person_payload,
    source_snapshot, version, created_at, updated_at
FROM merchant_subject_profiles
WHERE merchant_id = $1
  AND EXISTS (
    SELECT 1
    FROM merchant_applications
    WHERE merchant_applications.id = merchant_subject_profiles.merchant_application_id
      AND merchant_applications.status = 'approved'
  );

-- name: CreateMerchantSubjectProfileVersion :one
INSERT INTO merchant_subject_profile_versions (
    profile_id,
    merchant_application_id,
    merchant_id,
    user_id,
    version,
    snapshot
) VALUES (
    sqlc.arg(profile_id),
    sqlc.arg(merchant_application_id),
    sqlc.narg(merchant_id),
    sqlc.arg(user_id),
    sqlc.arg(version),
    sqlc.arg(snapshot)
)
ON CONFLICT (profile_id, version) DO NOTHING
RETURNING id, profile_id, merchant_application_id, merchant_id, user_id,
    version, snapshot, created_at;
