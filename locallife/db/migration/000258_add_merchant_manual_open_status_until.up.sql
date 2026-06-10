ALTER TABLE merchants
ADD COLUMN manual_open_status_until TIMESTAMPTZ;

COMMENT ON COLUMN merchants.manual_open_status_until IS '手动开关店临时覆盖自动营业时间的截止点';
