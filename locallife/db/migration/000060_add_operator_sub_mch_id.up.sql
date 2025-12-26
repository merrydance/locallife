-- 为运营商表添加微信二级商户号字段
-- 用于微信收付通分账

-- 添加 sub_mch_id 字段
ALTER TABLE "operators" ADD COLUMN IF NOT EXISTS "sub_mch_id" TEXT;

-- 创建索引
CREATE INDEX IF NOT EXISTS operators_sub_mch_id_idx ON operators(sub_mch_id);

-- 添加注释
COMMENT ON COLUMN operators.sub_mch_id IS '微信平台收付通二级商户号（开户成功后返回）';
