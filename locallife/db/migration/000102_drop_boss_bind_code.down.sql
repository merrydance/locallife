ALTER TABLE merchants
  ADD COLUMN IF NOT EXISTS boss_bind_code VARCHAR(32),
  ADD COLUMN IF NOT EXISTS boss_bind_code_expires_at TIMESTAMPTZ;

COMMENT ON COLUMN merchants.boss_bind_code IS 'Boss 认领码';
COMMENT ON COLUMN merchants.boss_bind_code_expires_at IS 'Boss 认领码过期时间';
