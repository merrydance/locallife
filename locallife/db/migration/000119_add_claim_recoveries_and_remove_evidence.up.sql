-- Phase3: Claim recoveries and remove evidence fields

CREATE TABLE IF NOT EXISTS claim_recoveries (
  id BIGSERIAL PRIMARY KEY,
  claim_id BIGINT NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
  order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  responsible_party TEXT NOT NULL CHECK (responsible_party IN ('merchant', 'rider', 'platform_fallback', 'unknown')),
  recovery_target TEXT CHECK (recovery_target IN ('merchant', 'rider')),
  recovery_amount BIGINT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'overdue', 'waived', 'appealed')),
  due_at TIMESTAMPTZ NOT NULL,
  decision_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_claim_recoveries_claim_id ON claim_recoveries(claim_id);
CREATE INDEX idx_claim_recoveries_order_id ON claim_recoveries(order_id);
CREATE INDEX idx_claim_recoveries_status ON claim_recoveries(status);
CREATE INDEX idx_claim_recoveries_due_at ON claim_recoveries(due_at);

-- Remove evidence-related storage (no-evidence policy)
DROP TABLE IF EXISTS behavior_evidence;

ALTER TABLE claims DROP COLUMN IF EXISTS evidence_urls;
ALTER TABLE appeals DROP COLUMN IF EXISTS evidence_urls;
ALTER TABLE food_safety_incidents DROP COLUMN IF EXISTS evidence_urls;
