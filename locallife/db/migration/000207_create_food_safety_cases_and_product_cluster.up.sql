CREATE TABLE food_safety_cases (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    region_id BIGINT NOT NULL REFERENCES regions(id) ON DELETE CASCADE,
    primary_product_key TEXT NOT NULL DEFAULT '',
    primary_product_label TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'merchant-suspended' CHECK (status IN ('merchant-suspended', 'investigating', 'resolved')),
    trigger_reason TEXT NOT NULL,
    investigation_report TEXT,
    merchant_rectification_report TEXT,
    resolution TEXT,
    suspended_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_food_safety_cases_region_status ON food_safety_cases(region_id, status, created_at DESC, id DESC);
CREATE INDEX idx_food_safety_cases_merchant_status ON food_safety_cases(merchant_id, status, created_at DESC, id DESC);
CREATE INDEX idx_food_safety_cases_merchant_product_status ON food_safety_cases(merchant_id, primary_product_key, status, created_at DESC, id DESC);

ALTER TABLE food_safety_incidents
    ADD COLUMN primary_product_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN primary_product_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN case_id BIGINT REFERENCES food_safety_cases(id) ON DELETE SET NULL;

CREATE INDEX idx_food_safety_incidents_case_id ON food_safety_incidents(case_id);
CREATE INDEX idx_food_safety_incidents_merchant_product_created_at ON food_safety_incidents(merchant_id, primary_product_key, created_at DESC);