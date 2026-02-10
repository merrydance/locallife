-- P0-003: 防止重复索赔
-- 先清理可能的重复数据（保留最新的）
DELETE FROM claims a USING claims b
WHERE a.id < b.id AND a.order_id = b.order_id;

-- 添加唯一约束
ALTER TABLE claims ADD CONSTRAINT claims_order_id_unique UNIQUE (order_id);
