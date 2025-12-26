-- 回滚：将 TEXT 改回 VARCHAR(n)
ALTER TABLE user_devices 
    ALTER COLUMN device_id TYPE VARCHAR(255),
    ALTER COLUMN device_type TYPE VARCHAR(50);
