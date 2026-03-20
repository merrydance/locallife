-- 回退：仅恢复字段结构，不恢复历史数据
ALTER TABLE dishes
    ADD COLUMN IF NOT EXISTS image_url text;

ALTER TABLE combo_sets
    ADD COLUMN IF NOT EXISTS image_url text;

ALTER TABLE reviews
    ADD COLUMN IF NOT EXISTS images text[];

ALTER TABLE table_images
    ADD COLUMN IF NOT EXISTS image_url text;
