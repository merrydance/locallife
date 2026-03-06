-- 修复 operators_status_check 约束，并对齐入驻后置绑卡设计
-- 背景：运营商绑定银行卡的操作已后移到入驻完成之后，且为可选步骤。
-- 因此 pending_bindbank 状态（原「等待强制绑卡」）不再需要。
-- bindbank_submitted 作为「绑卡进件进行中」的瞬时状态保留。
-- 同时迁移已有的 pending_bindbank 数据到 active。
UPDATE operators SET status = 'active' WHERE status = 'pending_bindbank';
ALTER TABLE operators DROP CONSTRAINT IF EXISTS operators_status_check;
ALTER TABLE operators ADD CONSTRAINT operators_status_check
    CHECK (status IN ('active', 'bindbank_submitted', 'suspended', 'expired'));
