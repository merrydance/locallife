-- 添加 pending 角色选项
-- pending 角色表示刚入职但尚未分配具体职责的员工

ALTER TABLE merchant_staff DROP CONSTRAINT IF EXISTS merchant_staff_role_check;
ALTER TABLE merchant_staff ADD CONSTRAINT merchant_staff_role_check 
    CHECK (role IN ('owner', 'manager', 'chef', 'cashier', 'pending'));

COMMENT ON COLUMN merchant_staff.role IS '员工角色: owner=店主, manager=店长, chef=厨师长, cashier=收银员, pending=待分配';
