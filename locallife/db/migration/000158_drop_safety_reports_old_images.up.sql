-- 删除 safety_reports 旧图片数组字段
--
-- 前置条件（运行此迁移前必须满足）：
--   1. safety_report_images 关联表 (000157) 已迁移，所有图片已写入新表
--   2. 安全事件上报相关 API handler 已切换到 safety_report_images
--   3. 已在非生产环境完整验证功能正常
--
-- 回退说明：down 迁移仅恢复字段结构（nullable），不恢复数据。

ALTER TABLE safety_reports
    DROP COLUMN IF EXISTS images;
