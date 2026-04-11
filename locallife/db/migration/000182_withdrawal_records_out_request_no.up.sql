ALTER TABLE withdrawal_records ADD COLUMN out_request_no varchar(64);

UPDATE withdrawal_records
SET out_request_no = (account_info->>'out_request_no')
WHERE account_info ? 'out_request_no';

CREATE UNIQUE INDEX idx_withdrawal_records_out_request_no
    ON withdrawal_records(out_request_no)
    WHERE out_request_no IS NOT NULL;
