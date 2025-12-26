-- 删除高值单资格积分变更日志表
DROP TABLE IF EXISTS rider_premium_score_logs;

-- 从rider_profiles表中删除高值单资格积分字段
ALTER TABLE rider_profiles
DROP COLUMN IF EXISTS premium_score;
