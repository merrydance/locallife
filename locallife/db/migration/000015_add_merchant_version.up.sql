-- ✅ P1-2: 添加乐观锁版本字段，防止商户信息并发更新丢失
ALTER TABLE "merchants" ADD COLUMN "version" int NOT NULL DEFAULT 1;

-- 为version字段添加注释
COMMENT ON COLUMN "merchants"."version" IS 'Optimistic locking version for concurrent updates';
