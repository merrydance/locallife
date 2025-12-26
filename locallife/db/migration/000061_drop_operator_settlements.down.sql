-- 重建 operator_settlements 表 (用于回滚)
-- 注意：此回滚只创建空表结构，不恢复数据

CREATE TABLE IF NOT EXISTS operator_settlements (
    id BIGSERIAL PRIMARY KEY,
    region_id BIGINT NOT NULL REFERENCES regions(id),
    month VARCHAR(7) NOT NULL,
    total_orders INTEGER DEFAULT 0,
    total_gmv BIGINT DEFAULT 0,
    platform_commission BIGINT DEFAULT 0,
    operator_commission BIGINT DEFAULT 0,
    status VARCHAR(20) DEFAULT 'pending',
    settled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(region_id, month)
);

CREATE INDEX IF NOT EXISTS idx_operator_settlements_region_id ON operator_settlements(region_id);
CREATE INDEX IF NOT EXISTS idx_operator_settlements_month ON operator_settlements(month);
CREATE INDEX IF NOT EXISTS idx_operator_settlements_status ON operator_settlements(status);
