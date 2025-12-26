-- 强制商户必须有区域归属
-- 先为现有无区域的商户设置默认区域，然后设置NOT NULL约束

-- 1. 为现有无区域的商户分配区域
-- 策略：根据商户地址的经纬度，找最近的区域
-- 如果没有经纬度，则分配第一个区县级区域
UPDATE merchants m
SET region_id = (
    SELECT r.id 
    FROM regions r 
    WHERE r.level = 3  -- 区县级
    ORDER BY 
        CASE 
            WHEN m.latitude IS NOT NULL AND m.longitude IS NOT NULL 
                 AND r.latitude IS NOT NULL AND r.longitude IS NOT NULL
            THEN (
                6371000 * acos(
                    cos(radians(m.latitude)) * cos(radians(r.latitude)) * 
                    cos(radians(r.longitude) - radians(m.longitude)) + 
                    sin(radians(m.latitude)) * sin(radians(r.latitude))
                )
            )
            ELSE 999999999  -- 无坐标时排到最后
        END
    LIMIT 1
)
WHERE region_id IS NULL;

-- 2. 如果还有NULL（没有任何区域数据），给一个提示
DO $$
DECLARE
    null_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO null_count FROM merchants WHERE region_id IS NULL;
    IF null_count > 0 THEN
        RAISE NOTICE '警告: 仍有 % 个商户没有分配区域，请手动处理', null_count;
    END IF;
END $$;

-- 3. 设置NOT NULL约束（如果所有商户都有区域）
-- 注意：如果有NULL值，这一步会失败，需要先手动处理
ALTER TABLE merchants ALTER COLUMN region_id SET NOT NULL;

-- 4. 添加注释
COMMENT ON COLUMN merchants.region_id IS '商户所属区域ID，必填，用于多租户隔离和运营商管理';
