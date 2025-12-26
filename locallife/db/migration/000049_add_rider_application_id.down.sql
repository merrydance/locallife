-- 移除riders表的application_id字段
DROP INDEX IF EXISTS riders_application_id_idx;
ALTER TABLE riders DROP COLUMN IF EXISTS application_id;
