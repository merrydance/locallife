-- Add merchant settlement adjustments for real deductions (claim recoveries)

CREATE TABLE IF NOT EXISTS merchant_settlement_adjustments (
  id BIGSERIAL PRIMARY KEY,
  merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
  adjustment_type TEXT NOT NULL,
  amount BIGINT NOT NULL,
  status TEXT NOT NULL DEFAULT 'finished',
  related_type TEXT,
  related_id BIGINT,
  note TEXT,
  posted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT merchant_settlement_adjustments_status_check
    CHECK (status IN ('finished', 'reversed'))
);

CREATE UNIQUE INDEX IF NOT EXISTS merchant_settlement_adjustments_related_type_id_type_idx
  ON merchant_settlement_adjustments(related_type, related_id, adjustment_type);

CREATE INDEX IF NOT EXISTS merchant_settlement_adjustments_merchant_created_idx
  ON merchant_settlement_adjustments(merchant_id, created_at);

COMMENT ON TABLE merchant_settlement_adjustments IS '商户结算调整流水（追偿真实扣款/回滚）';
COMMENT ON COLUMN merchant_settlement_adjustments.amount IS '调整金额（分，扣款为负，回滚为正）';
COMMENT ON COLUMN merchant_settlement_adjustments.adjustment_type IS '调整类型：claim_recovery_charge/claim_recovery_reversal';
COMMENT ON COLUMN merchant_settlement_adjustments.status IS '状态：finished=已入账，reversed=已冲正';
