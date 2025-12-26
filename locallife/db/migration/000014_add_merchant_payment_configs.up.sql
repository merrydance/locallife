-- 商户支付配置表
-- 存储商户的微信平台收付通配置，与 merchants 表解耦

CREATE TABLE "merchant_payment_configs" (
    "id" bigserial PRIMARY KEY,
    "merchant_id" bigint NOT NULL UNIQUE,
    "sub_mch_id" text NOT NULL,                          -- 微信平台收付通二级商户号
    "status" text NOT NULL DEFAULT 'active',             -- active, suspended
    "created_at" timestamptz NOT NULL DEFAULT now(),
    "updated_at" timestamptz NOT NULL DEFAULT now()
);

-- 索引
CREATE INDEX merchant_payment_configs_sub_mch_id_idx ON merchant_payment_configs(sub_mch_id);

-- 外键
ALTER TABLE "merchant_payment_configs" ADD FOREIGN KEY ("merchant_id") REFERENCES "merchants" ("id");

-- 注释
COMMENT ON TABLE merchant_payment_configs IS '商户微信支付配置（平台收付通）';
COMMENT ON COLUMN merchant_payment_configs.sub_mch_id IS '微信平台收付通二级商户号';
COMMENT ON COLUMN merchant_payment_configs.status IS '配置状态：active-启用, suspended-暂停';
