-- 更新申诉唯一性范围：claim_id + appellant_type

DROP INDEX IF EXISTS idx_appeals_claim_id_unique;

CREATE UNIQUE INDEX IF NOT EXISTS idx_appeals_claim_id_appellant_unique
ON appeals (claim_id, appellant_type);

COMMENT ON INDEX idx_appeals_claim_id_appellant_unique IS '确保每个索赔在同一申诉方类型下仅有一个申诉';
