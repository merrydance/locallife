-- 回退：仅恢复字段结构，不恢复历史数据
ALTER TABLE rider_applications
    ADD COLUMN IF NOT EXISTS id_card_front_url text,
    ADD COLUMN IF NOT EXISTS id_card_back_url  text,
    ADD COLUMN IF NOT EXISTS health_cert_url   text;

ALTER TABLE operator_applications
    ADD COLUMN IF NOT EXISTS business_license_url text,
    ADD COLUMN IF NOT EXISTS id_card_front_url    text,
    ADD COLUMN IF NOT EXISTS id_card_back_url     text;
