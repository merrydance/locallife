DROP INDEX IF EXISTS idx_withdrawal_records_out_request_no;
ALTER TABLE withdrawal_records DROP COLUMN IF EXISTS out_request_no;
