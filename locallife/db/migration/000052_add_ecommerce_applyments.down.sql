-- 恢复骑手状态约束
ALTER TABLE "riders" DROP CONSTRAINT IF EXISTS riders_status_check;
ALTER TABLE "riders" ADD CONSTRAINT riders_status_check CHECK (status IN ('pending', 'approved', 'active', 'suspended', 'rejected'));

-- 恢复商户状态约束
ALTER TABLE "merchants" DROP CONSTRAINT IF EXISTS merchants_status_check;
ALTER TABLE "merchants" ADD CONSTRAINT merchants_status_check CHECK (status IN ('pending', 'approved', 'active', 'suspended', 'rejected'));

-- 恢复运营商状态约束
ALTER TABLE "operators" DROP CONSTRAINT IF EXISTS operators_status_check;
ALTER TABLE "operators" ADD CONSTRAINT operators_status_check CHECK (status IN ('active', 'suspended'));

-- 删除骑手的sub_mch_id字段
ALTER TABLE "riders" DROP COLUMN IF EXISTS "sub_mch_id";

-- 删除进件表
DROP TABLE IF EXISTS "ecommerce_applyments";
