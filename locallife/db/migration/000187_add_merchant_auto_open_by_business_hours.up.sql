ALTER TABLE merchants
ADD COLUMN auto_open_by_business_hours BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN merchants.auto_open_by_business_hours IS '是否按营业时间自动切换营业状态';