-- 搜索历史表
CREATE TABLE IF NOT EXISTS search_histories (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    keyword    TEXT         NOT NULL,
    type       VARCHAR(20)  NOT NULL DEFAULT 'dish', -- 'dish', 'merchant', 'room', 'combo'
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, keyword, type)
);

CREATE INDEX IF NOT EXISTS idx_search_histories_user_id ON search_histories(user_id, created_at DESC);

-- 热门搜索关键词表（汇总表，由后台定时任务或触发器维护）
CREATE TABLE IF NOT EXISTS search_popular_keywords (
    id         BIGSERIAL PRIMARY KEY,
    keyword    TEXT        NOT NULL,
    type       VARCHAR(20) NOT NULL DEFAULT 'dish',
    count      INT         NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(keyword, type)
);

CREATE INDEX IF NOT EXISTS idx_search_popular_type ON search_popular_keywords(type, count DESC);
