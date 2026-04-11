-- 商户包装策略配置表
-- 控制外卖/自提订单是否必须从候选包装菜品中恰选 1 个

CREATE TABLE IF NOT EXISTS merchant_packaging_policies (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE UNIQUE,
    applicable_order_types TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    candidate_dish_ids BIGINT[] NOT NULL DEFAULT ARRAY[]::BIGINT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ
);

CREATE INDEX idx_merchant_packaging_policies_merchant ON merchant_packaging_policies(merchant_id);

CREATE TRIGGER update_merchant_packaging_policies_updated_at
BEFORE UPDATE ON merchant_packaging_policies
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE merchant_packaging_policies IS '商户订单级包装策略配置';
COMMENT ON COLUMN merchant_packaging_policies.applicable_order_types IS '触发包装策略的订单类型，当前仅允许 takeout 与 takeaway';
COMMENT ON COLUMN merchant_packaging_policies.candidate_dish_ids IS '可选包装菜品 ID 列表，命中场景时订单中必须恰选 1 个';