-- M1: 用户认证与授权系统
-- 严格遵循架构约束：bigint 自增主键 + text 类型

-- 创建 users 表（微信小程序用户）
CREATE TABLE "users" (
  "id" bigserial PRIMARY KEY,
  "wechat_openid" text UNIQUE NOT NULL,
  "wechat_unionid" text UNIQUE,
  "full_name" text NOT NULL,
  "phone" text UNIQUE,
  "avatar_url" text,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- 创建 user_roles 表（多对多角色系统）
CREATE TABLE "user_roles" (
  "id" bigserial PRIMARY KEY,
  "user_id" bigint NOT NULL,
  "role" text NOT NULL,
  "status" text NOT NULL DEFAULT 'active',
  "related_entity_id" bigint,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- 创建 sessions 表（access token + refresh token）
CREATE TABLE "sessions" (
  "id" bigserial PRIMARY KEY,
  "user_id" bigint NOT NULL,
  "access_token" text UNIQUE NOT NULL,
  "refresh_token" text UNIQUE NOT NULL,
  "access_token_expires_at" timestamptz NOT NULL,
  "refresh_token_expires_at" timestamptz NOT NULL,
  "user_agent" text NOT NULL,
  "client_ip" text NOT NULL,
  "is_revoked" boolean NOT NULL DEFAULT false,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- 创建 user_devices 表（设备指纹）
CREATE TABLE "user_devices" (
  "id" bigserial PRIMARY KEY,
  "user_id" bigint NOT NULL,
  "device_id" text NOT NULL,
  "device_type" text NOT NULL,
  "device_model" text,
  "os_version" text,
  "app_version" text,
  "user_agent" text,
  "ip_address" text,
  "last_login_at" timestamptz NOT NULL DEFAULT (now()),
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- 创建 wechat_access_tokens 表（微信Access Token缓存）
CREATE TABLE "wechat_access_tokens" (
  "id" bigserial PRIMARY KEY,
  "app_type" text NOT NULL,
  "access_token" text NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- 添加外键约束
ALTER TABLE "user_roles" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE;
ALTER TABLE "sessions" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE;
ALTER TABLE "user_devices" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE CASCADE;

-- 创建索引
CREATE UNIQUE INDEX ON "users" ("wechat_openid");
CREATE UNIQUE INDEX ON "users" ("wechat_unionid") WHERE "wechat_unionid" IS NOT NULL;
CREATE INDEX ON "users" ("phone");

CREATE INDEX ON "user_roles" ("user_id");
CREATE UNIQUE INDEX ON "user_roles" ("user_id", "role");
CREATE INDEX ON "user_roles" ("role");

CREATE INDEX ON "sessions" ("user_id");
CREATE UNIQUE INDEX ON "sessions" ("access_token");
CREATE UNIQUE INDEX ON "sessions" ("refresh_token");
CREATE INDEX ON "sessions" ("user_id", "is_revoked");

CREATE INDEX ON "user_devices" ("user_id");
CREATE INDEX ON "user_devices" ("device_id");
CREATE INDEX ON "user_devices" ("user_id", "device_id");

CREATE UNIQUE INDEX ON "wechat_access_tokens" ("app_type");

-- 添加注释
COMMENT ON COLUMN "users"."wechat_openid" IS '微信小程序唯一标识';
COMMENT ON COLUMN "users"."wechat_unionid" IS '微信开放平台unionid';
COMMENT ON COLUMN "users"."phone" IS '手机号，可选绑定';
COMMENT ON COLUMN "users"."avatar_url" IS '微信头像URL';

COMMENT ON COLUMN "user_roles"."role" IS 'customer/merchant/rider/operator/staff';
COMMENT ON COLUMN "user_roles"."status" IS 'active/suspended';
COMMENT ON COLUMN "user_roles"."related_entity_id" IS '关联实体ID（如商户ID、骑手ID等）';

COMMENT ON COLUMN "user_devices"."device_id" IS '设备唯一标识';
COMMENT ON COLUMN "user_devices"."device_type" IS 'ios/android/web';

COMMENT ON COLUMN "wechat_access_tokens"."app_type" IS 'miniprogram/official-account';
