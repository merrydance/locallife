-- 1. 修复 Schema 访问权限 (确保匿名和认证用户可以访问业务表)
GRANT USAGE ON SCHEMA public TO anon, authenticated;
GRANT ALL ON ALL TABLES IN SCHEMA public TO anon, authenticated;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO anon, authenticated;
GRANT ALL ON ALL FUNCTIONS IN SCHEMA public TO anon, authenticated;

-- 2. 特殊权限补丁 (如果 initial_schema 之后还有缺失字段的动态探测需求)
-- user_devices 表已在 initial_schema 中定义，此处仅保留必要的字段检查/补齐逻辑（以防万一）
ALTER TABLE public.user_devices ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ DEFAULT now();
