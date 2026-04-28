DROP INDEX IF EXISTS idx_recovery_disputes_submitted_region;

ALTER TABLE recovery_disputes
DROP CONSTRAINT IF EXISTS recovery_disputes_status_check;

ALTER TABLE recovery_disputes
ALTER COLUMN status SET DEFAULT 'pending';

UPDATE recovery_disputes
SET status = 'pending'
WHERE status = 'submitted';

ALTER TABLE recovery_disputes
ADD CONSTRAINT recovery_disputes_status_check
CHECK (status IN ('pending', 'approved', 'rejected'));

CREATE INDEX IF NOT EXISTS idx_recovery_disputes_pending_region
ON recovery_disputes(region_id, status)
WHERE status = 'pending';