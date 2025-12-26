-- 回滚 reserved 状态
ALTER TABLE tables DROP CONSTRAINT IF EXISTS tables_status_check;
ALTER TABLE tables ADD CONSTRAINT tables_status_check CHECK (status IN ('available', 'occupied', 'disabled'));
