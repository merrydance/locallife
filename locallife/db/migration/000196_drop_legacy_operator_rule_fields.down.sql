ALTER TABLE operators
ADD COLUMN commission_rate NUMERIC(5,4) NOT NULL DEFAULT 0.0300 CHECK (commission_rate >= 0 AND commission_rate <= 1),
ADD COLUMN merchant_deposit BIGINT NOT NULL DEFAULT 500000 CHECK (merchant_deposit >= 0);

ALTER TABLE region_rule_configs
ADD COLUMN commission_rate NUMERIC(5,4) NOT NULL DEFAULT 0.0300 CHECK (commission_rate >= 0 AND commission_rate <= 1),
ADD COLUMN merchant_deposit BIGINT NOT NULL DEFAULT 500000 CHECK (merchant_deposit >= 0);