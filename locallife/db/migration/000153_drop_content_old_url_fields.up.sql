-- 删除内容侧旧 URL / 数组字段
--
-- 前置条件（运行此迁移前必须满足）：
--   1. dishes.image_media_asset_id (000143) 已迁移并回填完成
--   2. combo_sets.image_media_asset_id (000151) 已迁移并回填完成
--   3. review_images 关联表 (000146) 已迁移，所有评价图片已写入新表
--   4. table_images.media_asset_id (000147) 已迁移并回填完成
--   5. 所有读取上述旧字段的 API handler 已切换到 media_asset_id / 新关联表
--   6. 已在非生产环境完整验证功能正常
--
-- 回退说明：down 迁移仅恢复字段结构（nullable），不恢复数据。
-- 注意：reviews.images 恢复后为空数组，历史数据已不可找回。

ALTER TABLE dishes
    DROP COLUMN IF EXISTS image_url;

ALTER TABLE combo_sets
    DROP COLUMN IF EXISTS image_url;

ALTER TABLE reviews
    DROP COLUMN IF EXISTS images;

-- table_images 的 image_url 最初是 NOT NULL，下线后允许为空
ALTER TABLE table_images
    DROP COLUMN IF EXISTS image_url;
