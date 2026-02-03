-- Phase1: 规则引擎基础表（草案）

CREATE TABLE IF NOT EXISTS rules (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  category TEXT NOT NULL, -- order/reservation/payment/profit_sharing/claim
  status TEXT NOT NULL DEFAULT 'draft', -- draft/active/disabled
  current_version_id BIGINT,
  created_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rule_versions (
  id BIGSERIAL PRIMARY KEY,
  rule_id BIGINT NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
  version INT NOT NULL DEFAULT 1,
  status TEXT NOT NULL DEFAULT 'draft', -- draft/published/disabled
  priority INT NOT NULL DEFAULT 100,
  scope JSONB NOT NULL DEFAULT '{}'::jsonb,
  condition JSONB NOT NULL DEFAULT '{}'::jsonb,
  action JSONB NOT NULL DEFAULT '{}'::jsonb,
  gray_config JSONB NOT NULL DEFAULT '{}'::jsonb,
  effective_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,
  created_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rule_audits (
  id BIGSERIAL PRIMARY KEY,
  rule_id BIGINT NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
  rule_version_id BIGINT REFERENCES rule_versions(id) ON DELETE SET NULL,
  action TEXT NOT NULL, -- create/update/publish/disable/delete
  actor_id BIGINT,
  actor_role TEXT,
  detail JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_rules_category ON rules(category);
CREATE INDEX IF NOT EXISTS idx_rules_status ON rules(status);
CREATE INDEX IF NOT EXISTS idx_rule_versions_rule_id ON rule_versions(rule_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rule_versions_rule_version ON rule_versions(rule_id, version);
CREATE INDEX IF NOT EXISTS idx_rule_versions_status ON rule_versions(status);
CREATE INDEX IF NOT EXISTS idx_rule_versions_priority ON rule_versions(priority);
CREATE INDEX IF NOT EXISTS idx_rule_audits_rule_id ON rule_audits(rule_id);

COMMENT ON TABLE rules IS '规则主体表（Phase1 草案）';
COMMENT ON TABLE rule_versions IS '规则版本表（Phase1 草案）';
COMMENT ON TABLE rule_audits IS '规则审计表（Phase1 草案）';
