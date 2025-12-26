-- 为商户申请表添加区域字段
-- 商户入驻时根据定位自动确定区域

ALTER TABLE "merchant_applications" 
ADD COLUMN "region_id" BIGINT;

-- 添加外键约束
ALTER TABLE "merchant_applications" 
ADD CONSTRAINT "fk_merchant_applications_region" 
FOREIGN KEY ("region_id") REFERENCES "regions" ("id");

-- 添加索引
CREATE INDEX "idx_merchant_applications_region_id" ON "merchant_applications" ("region_id");

COMMENT ON COLUMN "merchant_applications"."region_id" IS '区域ID，根据商户定位自动确定';
