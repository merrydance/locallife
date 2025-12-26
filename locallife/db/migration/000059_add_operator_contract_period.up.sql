-- =============================================
-- 为运营商表添加合同期限字段
-- 运营商有合同有效期，与商户/骑手不同
-- =============================================

-- 添加合同期限字段
ALTER TABLE operators
ADD COLUMN contract_start_date DATE,              -- 合同开始日期
ADD COLUMN contract_end_date DATE,                -- 合同到期日期
ADD COLUMN contract_years INT NOT NULL DEFAULT 1; -- 合同年限

-- 添加状态：增加 expired（已过期）状态
ALTER TABLE operators DROP CONSTRAINT IF EXISTS operators_status_check;
ALTER TABLE operators ADD CONSTRAINT operators_status_check 
    CHECK (status IN ('active', 'suspended', 'expired'));

-- 添加索引用于查询即将到期的合同
CREATE INDEX operators_contract_end_date_idx ON operators(contract_end_date);

-- 注释
COMMENT ON COLUMN operators.contract_start_date IS '合同开始日期';
COMMENT ON COLUMN operators.contract_end_date IS '合同到期日期';
COMMENT ON COLUMN operators.contract_years IS '合同年限（1/2/3年等）';
