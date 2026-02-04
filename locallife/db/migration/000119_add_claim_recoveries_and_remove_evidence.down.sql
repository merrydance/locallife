-- Phase3 rollback: restore evidence fields and drop claim recoveries

ALTER TABLE food_safety_incidents ADD COLUMN IF NOT EXISTS evidence_urls TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE appeals ADD COLUMN IF NOT EXISTS evidence_urls TEXT[];
ALTER TABLE claims ADD COLUMN IF NOT EXISTS evidence_urls TEXT[];

CREATE TABLE IF NOT EXISTS behavior_evidence (
  id BIGSERIAL PRIMARY KEY,
  decision_id BIGINT NOT NULL REFERENCES behavior_decisions(id) ON DELETE CASCADE,
  evidence_type TEXT NOT NULL CHECK (evidence_type IN ('image', 'text', 'location', 'device', 'address', 'ip', 'user_agent', 'other')),
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_behavior_evidence_decision_id ON behavior_evidence(decision_id);

DROP TABLE IF EXISTS claim_recoveries;
