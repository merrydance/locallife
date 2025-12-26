-- 移除商户入驻申请表的经纬度字段
ALTER TABLE "merchant_applications" 
DROP COLUMN IF EXISTS "longitude",
DROP COLUMN IF EXISTS "latitude";
