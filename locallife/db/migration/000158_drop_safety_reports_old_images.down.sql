-- 回退：仅恢复字段结构，不恢复历史数据
ALTER TABLE safety_reports
    ADD COLUMN IF NOT EXISTS images text[];
