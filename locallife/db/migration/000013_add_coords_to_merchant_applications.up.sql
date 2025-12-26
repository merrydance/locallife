-- 添加经纬度字段到商户入驻申请表
-- 前端通过微信小程序地图选点获取经纬度

ALTER TABLE "merchant_applications" 
ADD COLUMN "longitude" decimal(10,7),
ADD COLUMN "latitude" decimal(10,7);

COMMENT ON COLUMN "merchant_applications"."longitude" IS '商户位置经度，由前端地图选点提供';
COMMENT ON COLUMN "merchant_applications"."latitude" IS '商户位置纬度，由前端地图选点提供';
