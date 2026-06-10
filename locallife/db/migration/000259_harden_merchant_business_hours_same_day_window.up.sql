DELETE FROM merchant_business_hours
WHERE is_closed = false
  AND open_time >= close_time;

ALTER TABLE merchant_business_hours
DROP CONSTRAINT IF EXISTS merchant_business_hours_same_day_window_chk;

ALTER TABLE merchant_business_hours
ADD CONSTRAINT merchant_business_hours_same_day_window_chk
CHECK (is_closed OR open_time < close_time);
