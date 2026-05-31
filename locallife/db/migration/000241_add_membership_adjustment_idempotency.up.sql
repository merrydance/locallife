CREATE UNIQUE INDEX IF NOT EXISTS membership_transactions_adjustment_idempotency_uq
    ON membership_transactions(membership_id, idempotency_key)
    WHERE type IN ('adjustment_credit', 'adjustment_debit') AND idempotency_key IS NOT NULL;

COMMENT ON COLUMN membership_transactions.idempotency_key IS '商户代录会员充值和人工余额调整请求幂等键';
