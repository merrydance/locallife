-- 回滚：恢复原来的 CHECK 约束
ALTER TABLE rider_premium_score_logs
DROP CONSTRAINT IF EXISTS rider_premium_score_logs_change_type_check;

ALTER TABLE rider_premium_score_logs
ADD CONSTRAINT rider_premium_score_logs_change_type_check 
    CHECK (change_type IN ('normal_order', 'premium_order', 'adjustment'));

COMMENT ON COLUMN rider_profiles.premium_score IS '高值单资格积分，初始0，接普通单+1，接高值单-3，≥0可接高值单';
