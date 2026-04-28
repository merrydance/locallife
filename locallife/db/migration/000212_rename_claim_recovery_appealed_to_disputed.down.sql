ALTER TABLE claim_recoveries DROP CONSTRAINT IF EXISTS claim_recoveries_status_check;

UPDATE claim_recoveries
SET status = 'appealed'
WHERE status = 'disputed';

ALTER TABLE claim_recoveries
ADD CONSTRAINT claim_recoveries_status_check
CHECK (status IN ('pending', 'paid', 'overdue', 'waived', 'appealed'));