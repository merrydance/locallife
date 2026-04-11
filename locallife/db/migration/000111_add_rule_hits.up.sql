-- Phase1: 规则命中审计表（草案）

CREATE TABLE IF NOT EXISTS rule_hits (
  id BIGSERIAL PRIMARY KEY,
  rule_id BIGINT NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
  rule_version_id BIGINT REFERENCES rule_versions(id) ON DELETE SET NULL,
  domain TEXT NOT NULL,
  decision TEXT NOT NULL, -- allow/deny/adjust/compensate/manual_review/alert
  reason TEXT,
  inputs JSONB NOT NULL DEFAULT '{}'::jsonb,
  outputs JSONB NOT NULL DEFAULT '{}'::jsonb,
  actor_id BIGINT,
  actor_role TEXT,
  region_id BIGINT,
  merchant_id BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_rule_hits_rule_id ON rule_hits(rule_id);
CREATE INDEX IF NOT EXISTS idx_rule_hits_domain ON rule_hits(domain);
CREATE INDEX IF NOT EXISTS idx_rule_hits_region_id ON rule_hits(region_id);
CREATE INDEX IF NOT EXISTS idx_rule_hits_merchant_id ON rule_hits(merchant_id);
CREATE INDEX IF NOT EXISTS idx_rule_hits_created_at ON rule_hits(created_at);

COMMENT ON TABLE rule_hits IS '规则命中审计表（Phase1 草案）';
