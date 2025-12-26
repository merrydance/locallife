-- 支持运营商管理多个区县
-- 创建operator_regions多对多关联表，移除operators表的region_id UNIQUE约束

-- 1. 创建运营商区域关联表
CREATE TABLE operator_regions (
    id BIGSERIAL PRIMARY KEY,
    operator_id BIGINT NOT NULL REFERENCES operators(id) ON DELETE CASCADE,
    region_id BIGINT NOT NULL REFERENCES regions(id),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(operator_id, region_id)
);

-- 创建索引
CREATE INDEX idx_operator_regions_operator_id ON operator_regions(operator_id);
CREATE INDEX idx_operator_regions_region_id ON operator_regions(region_id);
CREATE INDEX idx_operator_regions_status ON operator_regions(status);

-- 2. 迁移现有数据：将operators表中的region_id迁移到新表
INSERT INTO operator_regions (operator_id, region_id)
SELECT id, region_id FROM operators WHERE region_id IS NOT NULL;

-- 3. 移除operators表的region_id UNIQUE约束（保留字段作为"主要区域"）
-- 先删除唯一索引
DROP INDEX IF EXISTS operators_region_id_idx;
-- 注意：UNIQUE CONSTRAINT不能直接删除索引，需要ALTER TABLE
ALTER TABLE operators DROP CONSTRAINT IF EXISTS operators_region_id_key;

-- 4. 添加注释说明
COMMENT ON TABLE operator_regions IS '运营商管理的区域列表，支持一个运营商管理多个区县';
COMMENT ON COLUMN operators.region_id IS '运营商的主要区域（已废弃UNIQUE约束，现通过operator_regions表管理多区域）';
