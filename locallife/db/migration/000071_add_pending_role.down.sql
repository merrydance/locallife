-- 回滚：移除 pending 角色

-- 先将 pending 角色的员工改为 cashier
UPDATE merchant_staff SET role = 'cashier' WHERE role = 'pending';

ALTER TABLE merchant_staff DROP CONSTRAINT IF EXISTS merchant_staff_role_check;
ALTER TABLE merchant_staff ADD CONSTRAINT merchant_staff_role_check 
    CHECK (role IN ('owner', 'manager', 'chef', 'cashier'));
