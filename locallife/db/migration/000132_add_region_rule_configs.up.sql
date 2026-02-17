CREATE TABLE region_rule_configs (
    id BIGSERIAL PRIMARY KEY,
    region_id BIGINT NOT NULL UNIQUE REFERENCES regions(id) ON DELETE CASCADE,
    commission_rate NUMERIC(5,4) NOT NULL DEFAULT 0.0300 CHECK (commission_rate >= 0 AND commission_rate <= 1),
    merchant_deposit BIGINT NOT NULL DEFAULT 500000 CHECK (merchant_deposit >= 0),
    rider_deposit BIGINT NOT NULL DEFAULT 20000 CHECK (rider_deposit >= 0),
    weather_coeff_extreme NUMERIC(3,2) NOT NULL DEFAULT 2.00 CHECK (weather_coeff_extreme >= 1.00),
    weather_coeff_heavy NUMERIC(3,2) NOT NULL DEFAULT 1.80 CHECK (weather_coeff_heavy >= 1.00),
    weather_coeff_moderate NUMERIC(3,2) NOT NULL DEFAULT 1.30 CHECK (weather_coeff_moderate >= 1.00),
    weather_coeff_light NUMERIC(3,2) NOT NULL DEFAULT 1.10 CHECK (weather_coeff_light >= 1.00),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

CREATE INDEX idx_region_rule_configs_region_id ON region_rule_configs(region_id);

INSERT INTO region_rule_configs (
    region_id,
    commission_rate,
    merchant_deposit,
    rider_deposit,
    weather_coeff_extreme,
    weather_coeff_heavy,
    weather_coeff_moderate,
    weather_coeff_light
)
SELECT DISTINCT
    rel.region_id,
    o.commission_rate,
    o.merchant_deposit,
    o.rider_deposit,
    o.weather_coeff_extreme,
    o.weather_coeff_heavy,
    o.weather_coeff_moderate,
    o.weather_coeff_light
FROM operators o
JOIN (
    SELECT id AS operator_id, region_id
    FROM operators
    WHERE region_id IS NOT NULL
    UNION
    SELECT operator_id, region_id
    FROM operator_regions
) rel ON rel.operator_id = o.id
WHERE rel.region_id IS NOT NULL
ON CONFLICT (region_id) DO NOTHING;

COMMENT ON TABLE region_rule_configs IS '区县级规则配置表，按 region 维度管理运营规则';
COMMENT ON COLUMN region_rule_configs.region_id IS '区县ID';
COMMENT ON COLUMN region_rule_configs.commission_rate IS '平台抽成比例（0.03=3%）';
COMMENT ON COLUMN region_rule_configs.merchant_deposit IS '商户押金（分）';
COMMENT ON COLUMN region_rule_configs.rider_deposit IS '骑手押金（分）';
COMMENT ON COLUMN region_rule_configs.weather_coeff_extreme IS '极端天气系数';
COMMENT ON COLUMN region_rule_configs.weather_coeff_heavy IS '暴雨雪天气系数';
COMMENT ON COLUMN region_rule_configs.weather_coeff_moderate IS '中雨雪天气系数';
COMMENT ON COLUMN region_rule_configs.weather_coeff_light IS '小雨雪天气系数';
