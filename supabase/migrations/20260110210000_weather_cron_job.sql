-- Enable extensions
CREATE EXTENSION IF NOT EXISTS pg_cron;
CREATE EXTENSION IF NOT EXISTS pg_net;

-- Create a scheduled job for weather sync (every 15 minutes)
-- Note: Replace <PROJECT_REF> and <SERVICE_ROLE_KEY> with actual values when deploying to production
-- For local development, we use the edge function name directly if possible, 
-- or use the fully qualified URL.

SELECT cron.schedule(
    'weather-sync-task',
    '*/15 * * * *',
    $$
    SELECT net.http_post(
        url := 'https://ls.merrydance.cn/functions/v1/weather-sync',
        headers := jsonb_build_object(
            'Content-Type', 'application/json',
            'Authorization', 'Bearer ' || current_setting('app.settings.service_role_key', true)
        ),
        body := '{}'
    )
    $$
);

COMMENT ON COLUMN public.weather_coefficients.weather_type IS 'sunny/cloudy/rainy/heavy_rain/snowy/heavy_snow/extreme';
