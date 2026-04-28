ALTER TABLE recovery_disputes
DROP CONSTRAINT IF EXISTS recovery_disputes_status_check;

ALTER TABLE recovery_disputes
ALTER COLUMN status SET DEFAULT 'submitted';

UPDATE recovery_disputes
SET status = 'submitted'
WHERE status = 'pending';

ALTER TABLE recovery_disputes
ADD CONSTRAINT recovery_disputes_status_check
CHECK (status IN ('submitted', 'approved', 'rejected'));

DROP INDEX IF EXISTS idx_recovery_disputes_pending_region;

CREATE INDEX IF NOT EXISTS idx_recovery_disputes_submitted_region
ON recovery_disputes(region_id, status)
WHERE status = 'submitted';