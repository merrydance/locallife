-- 删除商户侧旧 URL 字段
--
-- 前置条件（运行此迁移前必须满足）：
--   1. merchants.logo_media_asset_id (000142) 已迁移并回填完成
--   2. merchant_brands.logo_media_asset_id (000145) 已迁移并回填完成
--   3. merchant_applications 的 business_license/food_permit 媒体字段 (000148) 已迁移并回填完成
--   4. 所有读取上述旧字段的 API handler 已切换到 media_asset_id
--   5. 已在非生产环境完整验证功能正常
--
-- 回退说明：down 迁移仅恢复字段结构（nullable），不恢复数据。

ALTER TABLE merchants
    DROP COLUMN IF EXISTS logo_url;

ALTER TABLE merchant_brands
    DROP COLUMN IF EXISTS logo_url;

ALTER TABLE merchant_applications
    DROP COLUMN IF EXISTS business_license_image_url,
    DROP COLUMN IF EXISTS food_permit_url,
    DROP COLUMN IF EXISTS legal_person_id_front_url,
    DROP COLUMN IF EXISTS legal_person_id_back_url;
