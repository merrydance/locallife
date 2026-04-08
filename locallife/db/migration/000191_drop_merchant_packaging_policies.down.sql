CREATE TABLE IF NOT EXISTS merchant_packaging_policies (
  id bigserial PRIMARY KEY,
  merchant_id bigint NOT NULL UNIQUE REFERENCES merchants(id) ON DELETE CASCADE,
  applicable_order_types text[] NOT NULL DEFAULT '{}',
  candidate_dish_ids bigint[] NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_merchant_packaging_policies_merchant
ON merchant_packaging_policies(merchant_id);

CREATE TRIGGER update_merchant_packaging_policies_updated_at
BEFORE UPDATE ON merchant_packaging_policies
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE merchant_packaging_policies IS '商户订单级包装策略配置';
COMMENT ON COLUMN merchant_packaging_policies.applicable_order_types IS '触发包装策略的订单类型，当前仅允许 takeout 与 takeaway';
COMMENT ON COLUMN merchant_packaging_policies.candidate_dish_ids IS '可选包装菜品 ID 列表，命中场景时订单中必须恰选 1 个';