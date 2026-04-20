UPDATE claims
SET status = CASE
        WHEN status = 'waiting_customer_confirmation' THEN 'auto-approved'
        WHEN status = 'withdrawn' THEN 'rejected'
        ELSE status
    END,
    rejection_reason = CASE
        WHEN status = 'withdrawn' AND COALESCE(rejection_reason, '') = '' THEN 'customer_withdrew_before_compensation'
        ELSE rejection_reason
    END,
    review_notes = CASE
        WHEN status = 'withdrawn' AND COALESCE(review_notes, '') = '' THEN 'claim withdrawn by customer before compensation execution'
        ELSE review_notes
    END,
    reviewed_at = CASE
        WHEN status = 'withdrawn' AND reviewed_at IS NULL THEN NOW()
        ELSE reviewed_at
    END;

ALTER TABLE claims
    DROP CONSTRAINT IF EXISTS claims_status_check;

ALTER TABLE claims
    ADD CONSTRAINT claims_status_check
    CHECK (status IN ('pending', 'auto-approved', 'approved', 'rejected'));