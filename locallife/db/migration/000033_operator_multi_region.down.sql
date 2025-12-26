-- 回滚：恢复运营商单区县限制

-- 1. 恢复UNIQUE约束
ALTER TABLE operators ADD CONSTRAINT operators_region_id_key UNIQUE (region_id);
CREATE UNIQUE INDEX operators_region_id_idx ON operators(region_id);

-- 2. 删除关联表
DROP TABLE IF EXISTS operator_regions;
