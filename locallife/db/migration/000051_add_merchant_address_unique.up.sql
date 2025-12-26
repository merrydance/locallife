-- 添加商户地址唯一约束
-- 同一地址不能注册两家餐厅

-- 为merchants表的address字段添加唯一索引
-- 只对未删除的商户生效
CREATE UNIQUE INDEX "idx_merchants_address_unique" 
ON "merchants" ("address") 
WHERE deleted_at IS NULL;

COMMENT ON INDEX "idx_merchants_address_unique" IS '同一地址只能注册一家餐厅';
