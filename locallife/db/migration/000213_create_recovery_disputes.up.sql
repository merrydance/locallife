CREATE TABLE IF NOT EXISTS recovery_disputes (
    id BIGSERIAL PRIMARY KEY,
    claim_id BIGINT NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
    appellant_type TEXT NOT NULL CHECK (appellant_type IN ('merchant', 'rider')),
    appellant_id BIGINT NOT NULL,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewer_id BIGINT,
    review_notes TEXT,
    reviewed_at TIMESTAMPTZ,
    compensation_amount BIGINT,
    compensated_at TIMESTAMPTZ,
    region_id BIGINT NOT NULL REFERENCES regions(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recovery_disputes_claim_id ON recovery_disputes(claim_id);
CREATE INDEX IF NOT EXISTS idx_recovery_disputes_appellant ON recovery_disputes(appellant_type, appellant_id);
CREATE INDEX IF NOT EXISTS idx_recovery_disputes_status ON recovery_disputes(status);
CREATE INDEX IF NOT EXISTS idx_recovery_disputes_region_id ON recovery_disputes(region_id);
CREATE INDEX IF NOT EXISTS idx_recovery_disputes_reviewer_id ON recovery_disputes(reviewer_id) WHERE reviewer_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_recovery_disputes_created_at ON recovery_disputes(created_at);
CREATE INDEX IF NOT EXISTS idx_recovery_disputes_pending_region ON recovery_disputes(region_id, status) WHERE status = 'pending';
CREATE UNIQUE INDEX IF NOT EXISTS idx_recovery_disputes_claim_id_appellant_unique
ON recovery_disputes (claim_id, appellant_type);

COMMENT ON TABLE recovery_disputes IS '追偿争议表 - 商户/骑手对平台追偿发起的争议';
COMMENT ON COLUMN recovery_disputes.appellant_type IS '争议发起人类型：merchant=商户, rider=骑手';
COMMENT ON COLUMN recovery_disputes.appellant_id IS '争议发起人ID（商户ID或骑手ID）';
COMMENT ON COLUMN recovery_disputes.compensation_amount IS '补偿金额（追偿争议审核通过时平台赔付给争议发起方）';
COMMENT ON COLUMN recovery_disputes.region_id IS '关联区域（用于运营商按区域过滤）';

INSERT INTO recovery_disputes (
    id,
    claim_id,
    appellant_type,
    appellant_id,
    reason,
    status,
    reviewer_id,
    review_notes,
    reviewed_at,
    compensation_amount,
    compensated_at,
    region_id,
    created_at
)
SELECT
    id,
    claim_id,
    appellant_type,
    appellant_id,
    reason,
    status,
    reviewer_id,
    review_notes,
    reviewed_at,
    compensation_amount,
    compensated_at,
    region_id,
    created_at
FROM appeals
ON CONFLICT (claim_id, appellant_type) DO NOTHING;

SELECT setval(
    pg_get_serial_sequence('recovery_disputes', 'id'),
    COALESCE((SELECT MAX(id) FROM recovery_disputes), 1),
    EXISTS (SELECT 1 FROM recovery_disputes)
);