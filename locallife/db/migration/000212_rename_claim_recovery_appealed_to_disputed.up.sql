ALTER TABLE claim_recoveries DROP CONSTRAINT IF EXISTS claim_recoveries_status_check;

UPDATE claim_recoveries
SET status = 'disputed'
WHERE status = 'appealed';

ALTER TABLE claim_recoveries
ADD CONSTRAINT claim_recoveries_status_check
CHECK (status IN ('pending', 'paid', 'overdue', 'waived', 'disputed'));