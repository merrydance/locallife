-- 补充 combo_sets 的媒体资产字段（000142-150 中遗漏）
ALTER TABLE combo_sets
    ADD COLUMN image_media_asset_id bigint REFERENCES media_assets(id);

COMMENT ON COLUMN combo_sets.image_media_asset_id IS '套餐封面图媒体资产 ID，取代 image_url 字段';
