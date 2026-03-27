ALTER TABLE claims
    DROP CONSTRAINT IF EXISTS claims_approval_type_check;

ALTER TABLE claims
    ADD CONSTRAINT claims_approval_type_check
    CHECK (approval_type IN ('instant', 'auto', 'manual'));

ALTER TABLE claims
    DROP CONSTRAINT IF EXISTS claims_status_check;

ALTER TABLE claims
    ADD CONSTRAINT claims_status_check
    CHECK (status IN ('pending', 'auto-approved', 'manual-review', 'approved', 'rejected'));

COMMENT ON COLUMN claims.approval_type IS 'instant=秒赔(>=750分+<=50元), auto=回溯通过, manual=人工审核';