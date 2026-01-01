-- 移除标签名唯一约束
ALTER TABLE tags DROP CONSTRAINT IF EXISTS tags_name_unique;

-- 将系统标签改回 dish 类型
UPDATE tags SET type = 'dish' WHERE name IN ('热卖', '推荐') AND type = 'system';
