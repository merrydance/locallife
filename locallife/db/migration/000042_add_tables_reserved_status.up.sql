-- 添加 reserved 状态到 tables 表的状态约束
ALTER TABLE tables DROP CONSTRAINT IF EXISTS tables_status_check;
ALTER TABLE tables ADD CONSTRAINT tables_status_check CHECK (status IN ('available', 'occupied', 'disabled', 'reserved'));
