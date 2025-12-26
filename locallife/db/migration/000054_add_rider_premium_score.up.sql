-- =============================================
-- 高值单资格积分系统
-- =============================================
-- 业务规则：
-- 1. 初始积分为0
-- 2. 接普通单（运费<10元）：+1分
-- 3. 接高值单（运费≥10元）：-3分
-- 4. 积分≥0才能接高值单
-- 5. 积分可以为负数

-- 在rider_profiles表中添加高值单资格积分字段
ALTER TABLE rider_profiles
ADD COLUMN IF NOT EXISTS premium_score SMALLINT NOT NULL DEFAULT 0;

-- 添加注释说明
COMMENT ON COLUMN rider_profiles.premium_score IS '高值单资格积分，初始0，接普通单+1，接高值单-3，≥0可接高值单';

-- 高值单资格积分变更日志表
CREATE TABLE IF NOT EXISTS rider_premium_score_logs (
    id BIGSERIAL PRIMARY KEY,
    rider_id BIGINT NOT NULL REFERENCES riders(id) ON DELETE CASCADE,
    
    -- 积分变更
    change_amount SMALLINT NOT NULL,          -- 变更量（可正可负）
    old_score SMALLINT NOT NULL,              -- 变更前积分
    new_score SMALLINT NOT NULL,              -- 变更后积分
    
    -- 变更原因
    change_type VARCHAR(32) NOT NULL,         -- normal_order：接普通单，premium_order：接高值单，adjustment：人工调整
    related_order_id BIGINT REFERENCES orders(id), -- 关联订单
    related_delivery_id BIGINT REFERENCES deliveries(id), -- 关联配送单
    remark TEXT,                               -- 备注
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT rider_premium_score_logs_change_type_check 
        CHECK (change_type IN ('normal_order', 'premium_order', 'adjustment'))
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_rider_premium_score_logs_rider_id 
    ON rider_premium_score_logs(rider_id);
CREATE INDEX IF NOT EXISTS idx_rider_premium_score_logs_created_at 
    ON rider_premium_score_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_rider_premium_score_logs_change_type 
    ON rider_premium_score_logs(change_type);

COMMENT ON TABLE rider_premium_score_logs IS '高值单资格积分变更日志表';
