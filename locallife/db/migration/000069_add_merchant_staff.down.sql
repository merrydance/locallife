-- 回滚多店铺员工管理系统

-- 1. 删除商户表新增字段
ALTER TABLE merchants DROP COLUMN IF EXISTS pending_owner_bind;
ALTER TABLE merchants DROP COLUMN IF EXISTS bind_code;
ALTER TABLE merchants DROP COLUMN IF EXISTS bind_code_expires_at;

-- 2. 删除员工表
DROP TABLE IF EXISTS merchant_staff;
