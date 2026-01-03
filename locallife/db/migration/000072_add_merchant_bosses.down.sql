-- 回滚 Boss 店铺认领系统

ALTER TABLE merchants DROP COLUMN IF EXISTS boss_bind_code_expires_at;
ALTER TABLE merchants DROP COLUMN IF EXISTS boss_bind_code;

DROP TABLE IF EXISTS merchant_bosses;
