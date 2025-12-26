-- 为骑手表添加区域ID字段，实现多租户隔离
-- 骑手必须归属某个区域，只能接该区域内的订单

-- 添加region_id字段
ALTER TABLE riders ADD COLUMN region_id bigint REFERENCES regions(id);

-- 创建索引，加速按区域查询骑手
CREATE INDEX idx_riders_region_id ON riders(region_id);

-- 注意：暂不设置NOT NULL约束
-- 需要先为现有骑手分配区域后，再通过后续迁移设置NOT NULL
-- 可通过骑手的当前位置匹配最近的区域来分配

COMMENT ON COLUMN riders.region_id IS '骑手所属区域ID，用于多租户隔离，骑手只能接该区域内的订单';
