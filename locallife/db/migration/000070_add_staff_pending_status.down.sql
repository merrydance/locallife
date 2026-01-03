-- 回滚：移除 pending 状态

-- 先将 pending 状态的员工改为 disabled
UPDATE merchant_staff SET status = 'disabled' WHERE status = 'pending';

ALTER TABLE merchant_staff DROP CONSTRAINT IF EXISTS merchant_staff_status_check;
ALTER TABLE merchant_staff ADD CONSTRAINT merchant_staff_status_check 
    CHECK (status IN ('active', 'disabled'));
