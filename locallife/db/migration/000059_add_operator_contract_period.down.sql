-- 回滚运营商合同期限字段

-- 恢复原状态约束
ALTER TABLE operators DROP CONSTRAINT IF EXISTS operators_status_check;
ALTER TABLE operators ADD CONSTRAINT operators_status_check 
    CHECK (status IN ('active', 'suspended'));

-- 删除索引
DROP INDEX IF EXISTS operators_contract_end_date_idx;

-- 删除合同期限字段
ALTER TABLE operators
DROP COLUMN IF EXISTS contract_start_date,
DROP COLUMN IF EXISTS contract_end_date,
DROP COLUMN IF EXISTS contract_years;
