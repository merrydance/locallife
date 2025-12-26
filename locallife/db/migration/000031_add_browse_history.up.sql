-- 用户浏览历史表
-- 记录用户浏览商户和菜品的历史

CREATE TABLE IF NOT EXISTS browse_history (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- 浏览目标类型: merchant, dish
    target_type TEXT NOT NULL CHECK (target_type IN ('merchant', 'dish')),
    
    -- 目标ID（商户ID或菜品ID）
    target_id BIGINT NOT NULL,
    
    -- 最后浏览时间（更新而非插入新记录）
    last_viewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- 浏览次数
    view_count INT NOT NULL DEFAULT 1,
    
    -- 唯一约束：一个用户对一个目标只有一条记录
    CONSTRAINT unique_user_target UNIQUE (user_id, target_type, target_id)
);

-- 索引
CREATE INDEX idx_browse_history_user ON browse_history(user_id);
CREATE INDEX idx_browse_history_user_time ON browse_history(user_id, last_viewed_at DESC);
CREATE INDEX idx_browse_history_target ON browse_history(target_type, target_id);

-- 注释
COMMENT ON TABLE browse_history IS '用户浏览历史';
COMMENT ON COLUMN browse_history.target_type IS '浏览目标类型: merchant(商户), dish(菜品)';
COMMENT ON COLUMN browse_history.view_count IS '浏览次数';
