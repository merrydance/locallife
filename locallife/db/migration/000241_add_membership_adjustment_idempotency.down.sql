DROP INDEX IF EXISTS membership_transactions_adjustment_idempotency_uq;

COMMENT ON COLUMN membership_transactions.idempotency_key IS '商户代录会员充值请求幂等键，仅 recharge 交易使用';
