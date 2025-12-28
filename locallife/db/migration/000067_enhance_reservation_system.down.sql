-- 回滚预订系统增强

ALTER TABLE table_reservations 
DROP COLUMN IF EXISTS source,
DROP COLUMN IF EXISTS cooking_started_at,
DROP COLUMN IF EXISTS checked_in_at;

ALTER TABLE table_reservations 
DROP CONSTRAINT IF EXISTS table_reservations_status_check;

ALTER TABLE table_reservations 
ADD CONSTRAINT table_reservations_status_check 
CHECK (status IN ('pending', 'paid', 'confirmed', 'completed', 'cancelled', 'expired', 'no_show'));
