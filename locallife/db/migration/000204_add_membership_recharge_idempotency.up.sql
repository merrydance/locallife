ALTER TABLE membership_transactions
ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS membership_transactions_recharge_idempotency_uq
    ON membership_transactions(membership_id, idempotency_key)
    WHERE type = 'recharge' AND idempotency_key IS NOT NULL;

COMMENT ON COLUMN membership_transactions.idempotency_key IS '商户代录会员充值请求幂等键，仅 recharge 交易使用';