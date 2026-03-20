-- 回退：仅恢复字段结构，不恢复历史数据
ALTER TABLE merchants
    ADD COLUMN IF NOT EXISTS logo_url text;

ALTER TABLE merchant_brands
    ADD COLUMN IF NOT EXISTS logo_url text;

ALTER TABLE merchant_applications
    ADD COLUMN IF NOT EXISTS business_license_image_url  text,
    ADD COLUMN IF NOT EXISTS food_permit_url             text,
    ADD COLUMN IF NOT EXISTS legal_person_id_front_url   text,
    ADD COLUMN IF NOT EXISTS legal_person_id_back_url    text;
