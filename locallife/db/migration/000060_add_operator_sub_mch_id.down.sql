-- 回滚：移除运营商表的微信二级商户号字段

-- 删除索引
DROP INDEX IF EXISTS operators_sub_mch_id_idx;

-- 删除字段
ALTER TABLE "operators" DROP COLUMN IF EXISTS "sub_mch_id";
