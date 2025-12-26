-- 回滚：删除菜品预估制作时间字段

ALTER TABLE dishes DROP COLUMN IF EXISTS prepare_time;
