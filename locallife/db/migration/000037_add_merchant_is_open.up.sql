-- 添加商户营业状态字段
-- is_open: 商户是否正在营业（不同于status审核状态）
-- auto_close_at: 自动打烊时间（可选，用于定时打烊）

ALTER TABLE merchants ADD COLUMN is_open BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE merchants ADD COLUMN auto_close_at TIMESTAMPTZ;

-- 添加注释
COMMENT ON COLUMN merchants.is_open IS '商户营业状态: true=营业中, false=已打烊';
COMMENT ON COLUMN merchants.auto_close_at IS '自动打烊时间（可选）';

-- 创建索引方便查询营业中的商户
CREATE INDEX idx_merchants_is_open ON merchants(is_open);
