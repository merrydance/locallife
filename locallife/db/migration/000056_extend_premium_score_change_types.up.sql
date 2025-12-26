-- =============================================
-- 扩展高值单资格积分变更类型
-- =============================================
-- 新增类型：
-- - timeout: 超时扣分 (-5)
-- - damage: 餐损扣分 (-10)
-- 
-- 完整积分规则：
-- - normal_order: 完成普通单 +1
-- - premium_order: 完成高值单 -3
-- - timeout: 超时 -5
-- - damage: 餐损 -10
-- - adjustment: 人工调整（可正可负）

-- 删除旧的 CHECK 约束
ALTER TABLE rider_premium_score_logs
DROP CONSTRAINT IF EXISTS rider_premium_score_logs_change_type_check;

-- 添加新的 CHECK 约束，包含 timeout 和 damage
ALTER TABLE rider_premium_score_logs
ADD CONSTRAINT rider_premium_score_logs_change_type_check 
    CHECK (change_type IN ('normal_order', 'premium_order', 'timeout', 'damage', 'adjustment'));

-- 更新注释
COMMENT ON COLUMN rider_profiles.premium_score IS '高值单资格积分：普通单+1，高值单-3，超时-5，餐损-10，≥0可接高值单';
