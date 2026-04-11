UPDATE claims
SET status = CASE
        WHEN status = 'manual-review' AND approved_amount IS NOT NULL AND approved_amount > 0 THEN 'auto-approved'
        WHEN status = 'manual-review' THEN 'rejected'
        ELSE status
    END,
    approval_type = CASE
        WHEN approval_type = 'manual' AND status = 'manual-review' AND approved_amount IS NOT NULL AND approved_amount > 0 THEN 'auto'
        WHEN approval_type = 'manual' AND status IN ('approved', 'auto-approved') THEN 'auto'
        WHEN approval_type = 'manual' THEN NULL
        ELSE approval_type
    END,
    review_notes = CASE
        WHEN status = 'manual-review' AND COALESCE(review_notes, '') = '' THEN 'legacy manual-review state normalized during auto-adjudication cleanup'
        ELSE review_notes
    END,
    reviewed_at = CASE
        WHEN status = 'manual-review' AND reviewed_at IS NULL THEN NOW()
        ELSE reviewed_at
    END;

ALTER TABLE claims
    DROP CONSTRAINT IF EXISTS claims_status_check;

ALTER TABLE claims
    ADD CONSTRAINT claims_status_check
    CHECK (status IN ('pending', 'auto-approved', 'approved', 'rejected'));

ALTER TABLE claims
    DROP CONSTRAINT IF EXISTS claims_approval_type_check;

ALTER TABLE claims
    ADD CONSTRAINT claims_approval_type_check
    CHECK (approval_type IN ('instant', 'auto'));

COMMENT ON COLUMN claims.approval_type IS 'instant=秒赔(>=750分+<=50元), auto=自动裁定通过';