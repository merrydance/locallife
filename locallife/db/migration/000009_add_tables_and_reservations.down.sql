-- 删除 table 类型的标签
DELETE FROM tags WHERE type = 'table';

-- 删除外键约束
ALTER TABLE tables DROP CONSTRAINT IF EXISTS tables_current_reservation_fk;

-- 删除表（按依赖顺序）
DROP TABLE IF EXISTS table_reservations;
DROP TABLE IF EXISTS table_tags;
DROP TABLE IF EXISTS tables;
