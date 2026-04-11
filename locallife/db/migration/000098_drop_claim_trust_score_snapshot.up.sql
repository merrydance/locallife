ALTER TABLE claims DROP COLUMN IF EXISTS trust_score_snapshot;
DROP INDEX IF EXISTS idx_claims_trust_snapshot;
