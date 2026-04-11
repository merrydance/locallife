ALTER TABLE tables
ADD COLUMN access_code_hash TEXT;

CREATE INDEX tables_access_code_hash_idx ON tables(access_code_hash);
