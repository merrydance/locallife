-- M1: 用户认证与授权系统（回滚）

-- 删除所有表（按依赖顺序）
DROP TABLE IF EXISTS "wechat_access_tokens";
DROP TABLE IF EXISTS "user_devices";
DROP TABLE IF EXISTS "sessions";
DROP TABLE IF EXISTS "user_roles";
DROP TABLE IF EXISTS "users";
