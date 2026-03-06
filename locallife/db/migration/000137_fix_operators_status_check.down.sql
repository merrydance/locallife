-- 回滚：恢复到 migration 000059 之后的状态（仅含 active/suspended/expired）
ALTER TABLE operators DROP CONSTRAINT IF EXISTS operators_status_check;
ALTER TABLE operators ADD CONSTRAINT operators_status_check
    CHECK (status IN ('active', 'suspended', 'expired'));
-- (migration 000059 的原始状态，不含 pending_bindbank / bindbank_submitted)
