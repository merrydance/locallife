DROP INDEX IF EXISTS tables_access_code_hash_idx;

ALTER TABLE tables
DROP COLUMN IF EXISTS access_code_hash;
