-- 回滚：取消商户region_id的NOT NULL约束

ALTER TABLE merchants ALTER COLUMN region_id DROP NOT NULL;
