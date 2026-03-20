-- 删除申请表旧 URL 字段
--
-- 前置条件（运行此迁移前必须满足）：
--   1. rider_applications 媒体字段 (000149) 已迁移并回填完成
--   2. operator_applications 媒体字段 (000150) 已迁移并回填完成
--   3. 所有读取上述旧字段的 API handler 已切换到 media_asset_id
--   4. 审核流程（合规/人工审核）已验证能通过 media_asset_id 正常取图
--   5. 已在非生产环境完整验证功能正常
--
-- 回退说明：down 迁移仅恢复字段结构（nullable），不恢复数据。

ALTER TABLE rider_applications
    DROP COLUMN IF EXISTS id_card_front_url,
    DROP COLUMN IF EXISTS id_card_back_url,
    DROP COLUMN IF EXISTS health_cert_url;

ALTER TABLE operator_applications
    DROP COLUMN IF EXISTS business_license_url,
    DROP COLUMN IF EXISTS id_card_front_url,
    DROP COLUMN IF EXISTS id_card_back_url;
