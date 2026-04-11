ALTER TABLE merchants
  DROP COLUMN IF EXISTS boss_bind_code,
  DROP COLUMN IF EXISTS boss_bind_code_expires_at;
