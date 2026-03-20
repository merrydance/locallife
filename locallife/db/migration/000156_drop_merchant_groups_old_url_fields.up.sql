-- 删除集团申请和集团表旧营业执照 URL 字段
--
-- 前置条件（运行此迁移前必须满足）：
--   1. merchant_group_applications.license_media_asset_id (000155) 已迁移并回填完成
--   2. merchant_groups.license_media_asset_id (000155) 已迁移并回填完成
--   3. 集团申请和集团管理相关 API handler 已切换到 media_asset_id
--   5. 已在非生产环境完整验证功能正常
--
-- 回退说明：down 迁移仅恢复字段结构（nullable），不恢复数据。

ALTER TABLE merchant_group_applications
    DROP COLUMN IF EXISTS license_image_url;

ALTER TABLE merchant_groups
    DROP COLUMN IF EXISTS license_image_url;
