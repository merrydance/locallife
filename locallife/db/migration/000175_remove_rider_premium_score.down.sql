ALTER TABLE rider_profiles
ADD COLUMN IF NOT EXISTS premium_score SMALLINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN rider_profiles.premium_score IS '高值单资格积分：普通单+1，高值单-3，超时-5，餐损-10，≥0可接高值单';

CREATE TABLE IF NOT EXISTS rider_premium_score_logs (
    id BIGSERIAL PRIMARY KEY,
    rider_id BIGINT NOT NULL REFERENCES riders(id) ON DELETE CASCADE,
    change_amount SMALLINT NOT NULL,
    old_score SMALLINT NOT NULL,
    new_score SMALLINT NOT NULL,
    change_type VARCHAR(32) NOT NULL,
    related_order_id BIGINT REFERENCES orders(id),
    related_delivery_id BIGINT REFERENCES deliveries(id),
    remark TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT rider_premium_score_logs_change_type_check
        CHECK (change_type IN ('normal_order', 'premium_order', 'timeout', 'damage', 'adjustment'))
);

CREATE INDEX IF NOT EXISTS idx_rider_premium_score_logs_rider_id
    ON rider_premium_score_logs(rider_id);
CREATE INDEX IF NOT EXISTS idx_rider_premium_score_logs_created_at
    ON rider_premium_score_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_rider_premium_score_logs_change_type
    ON rider_premium_score_logs(change_type);

COMMENT ON TABLE rider_premium_score_logs IS '高值单资格积分变更日志表';