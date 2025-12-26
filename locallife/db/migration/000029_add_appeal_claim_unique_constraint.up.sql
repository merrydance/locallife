-- =====================================================================
-- 添加申诉表的唯一约束
-- 业务规则：每个索赔只能有一个申诉
-- =====================================================================

-- 添加唯一索引确保每个claim_id只能有一个申诉
CREATE UNIQUE INDEX IF NOT EXISTS idx_appeals_claim_id_unique ON appeals(claim_id);

-- 添加注释
COMMENT ON INDEX idx_appeals_claim_id_unique IS '确保每个索赔只能有一个申诉';
