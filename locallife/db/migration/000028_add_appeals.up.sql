-- =====================================================================
-- 申诉系统 - 商户/骑手对索赔的申诉
-- =====================================================================
-- 业务背景：
-- 1. 顾客索赔时，系统自动从商户/骑手押金中扣款赔付顾客
-- 2. 商户/骑手可对索赔提起申诉，由运营商审核
-- 3. 申诉成功：平台垫付给商户/骑手，顾客信用分下降
-- 4. 申诉失败：押金扣除不变，流程结束
-- =====================================================================

CREATE TABLE IF NOT EXISTS appeals (
    id BIGSERIAL PRIMARY KEY,
    
    -- 关联索赔
    claim_id BIGINT NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
    
    -- 申诉发起人（商户或骑手）
    appellant_type TEXT NOT NULL CHECK (appellant_type IN ('merchant', 'rider')),
    appellant_id BIGINT NOT NULL,  -- merchant_id 或 rider_id
    
    -- 申诉内容
    reason TEXT NOT NULL,           -- 申诉理由
    evidence_urls TEXT[],           -- 证据图片URLs
    
    -- 审核状态
    status TEXT NOT NULL DEFAULT 'pending' 
        CHECK (status IN ('pending', 'approved', 'rejected')),
    
    -- 审核信息（运营商）
    reviewer_id BIGINT,             -- 审核的运营商ID
    review_notes TEXT,              -- 审核备注
    reviewed_at TIMESTAMPTZ,
    
    -- 补偿信息（申诉成功时）
    compensation_amount BIGINT,     -- 补偿金额（分）= 原索赔从押金扣除的金额
    compensated_at TIMESTAMPTZ,     -- 补偿时间
    
    -- 关联区域（用于运营商权限过滤）
    region_id BIGINT NOT NULL REFERENCES regions(id),
    
    -- 时间戳
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 索引
CREATE INDEX idx_appeals_claim_id ON appeals(claim_id);
CREATE INDEX idx_appeals_appellant ON appeals(appellant_type, appellant_id);
CREATE INDEX idx_appeals_status ON appeals(status);
CREATE INDEX idx_appeals_region_id ON appeals(region_id);
CREATE INDEX idx_appeals_reviewer_id ON appeals(reviewer_id) WHERE reviewer_id IS NOT NULL;
CREATE INDEX idx_appeals_created_at ON appeals(created_at);
CREATE INDEX idx_appeals_pending_region ON appeals(region_id, status) WHERE status = 'pending';

-- 注释
COMMENT ON TABLE appeals IS '申诉表 - 商户/骑手对索赔的申诉';
COMMENT ON COLUMN appeals.appellant_type IS '申诉人类型：merchant=商户, rider=骑手';
COMMENT ON COLUMN appeals.appellant_id IS '申诉人ID（商户ID或骑手ID）';
COMMENT ON COLUMN appeals.compensation_amount IS '补偿金额（申诉成功时平台垫付给申诉人）';
COMMENT ON COLUMN appeals.region_id IS '关联区域（用于运营商按区域过滤）';

-- 每个索赔只能有一个申诉（防止重复申诉）
CREATE UNIQUE INDEX idx_appeals_unique_claim ON appeals(claim_id);
