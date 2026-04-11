-- 回滚申诉唯一性范围为 claim_id

DROP INDEX IF EXISTS idx_appeals_claim_id_appellant_unique;

CREATE UNIQUE INDEX IF NOT EXISTS idx_appeals_claim_id_unique
ON appeals (claim_id);

COMMENT ON INDEX idx_appeals_claim_id_unique IS '确保每个索赔只能有一个申诉';
