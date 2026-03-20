-- 回退：仅恢复字段结构，不恢复历史数据
ALTER TABLE merchant_group_applications
    ADD COLUMN IF NOT EXISTS license_image_url text;

ALTER TABLE merchant_groups
    ADD COLUMN IF NOT EXISTS license_image_url text;
