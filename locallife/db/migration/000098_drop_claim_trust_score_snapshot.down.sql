ALTER TABLE claims ADD COLUMN IF NOT EXISTS trust_score_snapshot SMALLINT;
CREATE INDEX IF NOT EXISTS idx_claims_trust_snapshot ON claims(trust_score_snapshot);
COMMENT ON COLUMN claims.trust_score_snapshot IS '用户提交索赔时的信用分快照（决策依据）';
