CREATE TABLE IF NOT EXISTS merchant_subject_profiles (
    id BIGSERIAL PRIMARY KEY,
    merchant_application_id BIGINT NOT NULL REFERENCES merchant_applications(id) ON DELETE CASCADE,
    merchant_id BIGINT REFERENCES merchants(id) ON DELETE SET NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    business_license_number TEXT NOT NULL DEFAULT '',
    business_license_name TEXT NOT NULL DEFAULT '',
    business_license_address TEXT NOT NULL DEFAULT '',
    legal_person_name TEXT NOT NULL DEFAULT '',
    legal_person_id_number TEXT NOT NULL DEFAULT '',
    food_permit_number TEXT NOT NULL DEFAULT '',
    food_permit_company_name TEXT NOT NULL DEFAULT '',
    business_license_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL,
    food_permit_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL,
    id_card_front_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL,
    id_card_back_media_asset_id BIGINT REFERENCES media_assets(id) ON DELETE SET NULL,
    business_license_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    food_permit_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    legal_person_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT merchant_subject_profiles_application_uidx UNIQUE (merchant_application_id),
    CONSTRAINT merchant_subject_profiles_version_positive CHECK (version > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS merchant_subject_profiles_merchant_uidx
    ON merchant_subject_profiles(merchant_id)
    WHERE merchant_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_merchant_subject_profiles_user_updated
    ON merchant_subject_profiles(user_id, updated_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_merchant_subject_profiles_license_number
    ON merchant_subject_profiles(business_license_number)
    WHERE business_license_number <> '';

CREATE INDEX IF NOT EXISTS idx_merchant_subject_profiles_legal_person_id
    ON merchant_subject_profiles(legal_person_id_number)
    WHERE legal_person_id_number <> '';

CREATE TABLE IF NOT EXISTS merchant_subject_profile_versions (
    id BIGSERIAL PRIMARY KEY,
    profile_id BIGINT NOT NULL REFERENCES merchant_subject_profiles(id) ON DELETE CASCADE,
    merchant_application_id BIGINT NOT NULL REFERENCES merchant_applications(id) ON DELETE CASCADE,
    merchant_id BIGINT REFERENCES merchants(id) ON DELETE SET NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL,
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT merchant_subject_profile_versions_version_positive CHECK (version > 0),
    CONSTRAINT merchant_subject_profile_versions_profile_version_uidx UNIQUE (profile_id, version)
);

CREATE INDEX IF NOT EXISTS idx_merchant_subject_profile_versions_application
    ON merchant_subject_profile_versions(merchant_application_id, version DESC, id DESC);

COMMENT ON TABLE merchant_subject_profiles IS '商户主体资料权威聚合表，覆盖入驻 OCR 与人工修正后的营业执照、食品经营许可、法人证件资料';
COMMENT ON COLUMN merchant_subject_profiles.business_license_payload IS '营业执照权威字段与 OCR/修正/确认元数据 JSON';
COMMENT ON COLUMN merchant_subject_profiles.food_permit_payload IS '食品经营许可证权威字段与 OCR/修正/确认元数据 JSON';
COMMENT ON COLUMN merchant_subject_profiles.legal_person_payload IS '法人身份证权威字段与 OCR/修正/确认元数据 JSON';
COMMENT ON TABLE merchant_subject_profile_versions IS '商户主体资料版本审计快照';
