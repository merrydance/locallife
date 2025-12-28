-- 预订系统增强
-- 1. 添加 checked_in 状态
-- 2. 添加 checked_in_at 时间戳
-- 3. 添加 cooking_started_at 起菜时间

-- 修改状态约束，添加 checked_in 状态
ALTER TABLE table_reservations 
DROP CONSTRAINT IF EXISTS table_reservations_status_check;

ALTER TABLE table_reservations 
ADD CONSTRAINT table_reservations_status_check 
CHECK (status IN ('pending', 'paid', 'confirmed', 'checked_in', 'completed', 'cancelled', 'expired', 'no_show'));

-- 添加到店时间和起菜时间字段
ALTER TABLE table_reservations 
ADD COLUMN IF NOT EXISTS checked_in_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS cooking_started_at TIMESTAMPTZ;

-- 添加来源字段，区分线上预订和商户代客预订
ALTER TABLE table_reservations 
ADD COLUMN IF NOT EXISTS source VARCHAR(20) DEFAULT 'online';

COMMENT ON COLUMN table_reservations.checked_in_at IS '顾客到店签到时间';
COMMENT ON COLUMN table_reservations.cooking_started_at IS '厨房开始制作时间';
COMMENT ON COLUMN table_reservations.source IS '预订来源：online(线上)、phone(电话)、walkin(现场)、merchant(商户代订)';
