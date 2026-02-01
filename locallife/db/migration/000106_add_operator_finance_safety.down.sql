DROP TABLE IF EXISTS safety_reports;
DROP TABLE IF EXISTS withdrawal_records;
ALTER TABLE regions DROP COLUMN IF EXISTS status;
ALTER TABLE operators DROP COLUMN IF EXISTS balance;
ALTER TABLE operators DROP COLUMN IF EXISTS wallet_account;
