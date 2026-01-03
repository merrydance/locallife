-- 修改员工状态约束，添加 pending 状态
-- pending: 已入职但尚未分配权限（老板需要分配角色后才能工作）

ALTER TABLE merchant_staff DROP CONSTRAINT IF EXISTS merchant_staff_status_check;
ALTER TABLE merchant_staff ADD CONSTRAINT merchant_staff_status_check 
    CHECK (status IN ('active', 'pending', 'disabled'));

COMMENT ON COLUMN merchant_staff.status IS '状态: active=启用, pending=待分配权限, disabled=禁用';
