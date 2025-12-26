-- =============================================
-- M8: 骑手与配送系统
-- =============================================

-- 骑手表
CREATE TABLE "riders" (
    "id" bigserial PRIMARY KEY,
    "user_id" bigint NOT NULL REFERENCES "users"("id"),
    "real_name" varchar(50) NOT NULL,
    "id_card_no" varchar(18) NOT NULL,
    "phone" varchar(20) NOT NULL,
    
    -- 押金
    "deposit_amount" bigint NOT NULL DEFAULT 0,
    "frozen_deposit" bigint NOT NULL DEFAULT 0,
    
    -- 状态
    "status" varchar(20) NOT NULL DEFAULT 'pending',
    "is_online" boolean NOT NULL DEFAULT false,
    "credit_score" smallint NOT NULL DEFAULT 100,
    
    -- 实时位置（最新一次上报）
    "current_longitude" decimal(10,7),
    "current_latitude" decimal(10,7),
    "location_updated_at" timestamptz,
    
    -- 统计
    "total_orders" integer NOT NULL DEFAULT 0,
    "total_earnings" bigint NOT NULL DEFAULT 0,
    "online_duration" integer NOT NULL DEFAULT 0,
    
    "created_at" timestamptz NOT NULL DEFAULT now(),
    "updated_at" timestamptz,
    
    -- 状态约束
    CONSTRAINT riders_status_check CHECK (status IN ('pending', 'approved', 'active', 'suspended', 'rejected')),
    CONSTRAINT riders_credit_score_check CHECK (credit_score >= 0 AND credit_score <= 100),
    CONSTRAINT riders_deposit_check CHECK (deposit_amount >= 0 AND frozen_deposit >= 0)
);

CREATE UNIQUE INDEX riders_user_id_idx ON riders(user_id);
CREATE INDEX riders_status_idx ON riders(status);
CREATE INDEX riders_is_online_idx ON riders(is_online);
CREATE INDEX riders_phone_idx ON riders(phone);
CREATE INDEX riders_location_idx ON riders(current_longitude, current_latitude);

-- 骑手押金流水表
CREATE TABLE "rider_deposits" (
    "id" bigserial PRIMARY KEY,
    "rider_id" bigint NOT NULL REFERENCES "riders"("id"),
    "amount" bigint NOT NULL,
    "type" varchar(20) NOT NULL,
    "related_order_id" bigint REFERENCES "orders"("id"),
    "balance_after" bigint NOT NULL,
    "remark" text,
    "created_at" timestamptz NOT NULL DEFAULT now(),
    
    -- 类型约束
    CONSTRAINT rider_deposits_type_check CHECK (type IN ('deposit', 'withdraw', 'freeze', 'unfreeze', 'deduct'))
);

CREATE INDEX rider_deposits_rider_id_idx ON rider_deposits(rider_id);
CREATE INDEX rider_deposits_created_at_idx ON rider_deposits(created_at);
CREATE INDEX rider_deposits_type_idx ON rider_deposits(type);

-- 配送单表
CREATE TABLE "deliveries" (
    "id" bigserial PRIMARY KEY,
    "order_id" bigint NOT NULL REFERENCES "orders"("id"),
    "rider_id" bigint REFERENCES "riders"("id"),
    
    -- 取餐点（商户位置）
    "pickup_address" text NOT NULL,
    "pickup_longitude" decimal(10,7) NOT NULL,
    "pickup_latitude" decimal(10,7) NOT NULL,
    "pickup_contact" varchar(50),
    "pickup_phone" varchar(20),
    "picked_at" timestamptz,
    
    -- 送餐点（用户位置）
    "delivery_address" text NOT NULL,
    "delivery_longitude" decimal(10,7) NOT NULL,
    "delivery_latitude" decimal(10,7) NOT NULL,
    "delivery_contact" varchar(50),
    "delivery_phone" varchar(20),
    "delivered_at" timestamptz,
    
    -- 距离与费用
    "distance" integer NOT NULL,
    "delivery_fee" bigint NOT NULL,
    "rider_earnings" bigint NOT NULL DEFAULT 0,
    
    -- 状态
    "status" varchar(20) NOT NULL DEFAULT 'pending',
    
    -- 预计时间
    "estimated_pickup_at" timestamptz,
    "estimated_delivery_at" timestamptz,
    
    -- 问题记录
    "is_damaged" boolean NOT NULL DEFAULT false,
    "is_delayed" boolean NOT NULL DEFAULT false,
    "damage_amount" bigint NOT NULL DEFAULT 0,
    "damage_reason" text,
    
    "created_at" timestamptz NOT NULL DEFAULT now(),
    "assigned_at" timestamptz,
    "completed_at" timestamptz,
    
    -- 状态约束
    CONSTRAINT deliveries_status_check CHECK (status IN ('pending', 'assigned', 'picking', 'picked', 'delivering', 'delivered', 'completed', 'cancelled'))
);

CREATE UNIQUE INDEX deliveries_order_id_idx ON deliveries(order_id);
CREATE INDEX deliveries_rider_id_idx ON deliveries(rider_id);
CREATE INDEX deliveries_status_idx ON deliveries(status);
CREATE INDEX deliveries_created_at_idx ON deliveries(created_at);

-- 骑手位置记录表
CREATE TABLE "rider_locations" (
    "id" bigserial PRIMARY KEY,
    "rider_id" bigint NOT NULL REFERENCES "riders"("id"),
    "delivery_id" bigint REFERENCES "deliveries"("id"),
    "longitude" decimal(10,7) NOT NULL,
    "latitude" decimal(10,7) NOT NULL,
    "accuracy" decimal(6,2),
    "speed" decimal(6,2),
    "heading" decimal(5,2),
    "recorded_at" timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX rider_locations_rider_recorded_idx ON rider_locations(rider_id, recorded_at);
CREATE INDEX rider_locations_delivery_recorded_idx ON rider_locations(delivery_id, recorded_at);

-- 可接单订单池表
CREATE TABLE "delivery_pool" (
    "id" bigserial PRIMARY KEY,
    "order_id" bigint NOT NULL REFERENCES "orders"("id"),
    "merchant_id" bigint NOT NULL REFERENCES "merchants"("id"),
    "pickup_longitude" decimal(10,7) NOT NULL,
    "pickup_latitude" decimal(10,7) NOT NULL,
    "delivery_longitude" decimal(10,7) NOT NULL,
    "delivery_latitude" decimal(10,7) NOT NULL,
    "distance" integer NOT NULL,
    "delivery_fee" bigint NOT NULL,
    "expected_pickup_at" timestamptz NOT NULL,
    "expires_at" timestamptz NOT NULL,
    "priority" integer NOT NULL DEFAULT 0,
    "created_at" timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX delivery_pool_order_id_idx ON delivery_pool(order_id);
CREATE INDEX delivery_pool_expires_at_idx ON delivery_pool(expires_at);
CREATE INDEX delivery_pool_pickup_location_idx ON delivery_pool(pickup_longitude, pickup_latitude);
CREATE INDEX delivery_pool_priority_idx ON delivery_pool(priority);

-- 推荐配置表
CREATE TABLE "recommend_configs" (
    "id" bigserial PRIMARY KEY,
    "name" varchar(50) NOT NULL,
    "distance_weight" decimal(3,2) NOT NULL DEFAULT 0.40,
    "route_weight" decimal(3,2) NOT NULL DEFAULT 0.30,
    "urgency_weight" decimal(3,2) NOT NULL DEFAULT 0.20,
    "profit_weight" decimal(3,2) NOT NULL DEFAULT 0.10,
    "max_distance" integer NOT NULL DEFAULT 5000,
    "max_results" integer NOT NULL DEFAULT 20,
    "is_active" boolean NOT NULL DEFAULT true,
    "created_at" timestamptz NOT NULL DEFAULT now(),
    "updated_at" timestamptz
);

CREATE UNIQUE INDEX recommend_configs_name_idx ON recommend_configs(name);
CREATE INDEX recommend_configs_is_active_idx ON recommend_configs(is_active);

-- 插入默认配置
INSERT INTO recommend_configs (name, distance_weight, route_weight, urgency_weight, profit_weight, max_distance, max_results, is_active)
VALUES ('default', 0.40, 0.30, 0.20, 0.10, 5000, 20, true);

COMMENT ON TABLE riders IS '骑手表';
COMMENT ON TABLE rider_deposits IS '骑手押金流水';
COMMENT ON TABLE deliveries IS '配送单表';
COMMENT ON TABLE rider_locations IS '骑手位置记录';
COMMENT ON TABLE delivery_pool IS '可接单订单池';
COMMENT ON TABLE recommend_configs IS '推荐算法配置';

COMMENT ON COLUMN riders.deposit_amount IS '可用押金余额（分）';
COMMENT ON COLUMN riders.frozen_deposit IS '冻结押金（分）';
COMMENT ON COLUMN riders.online_duration IS '累计在线时长（秒）';
COMMENT ON COLUMN deliveries.distance IS '配送距离（米）';
COMMENT ON COLUMN deliveries.rider_earnings IS '骑手配送收益（分）';
COMMENT ON COLUMN deliveries.status IS '状态：pending待分配/assigned已分配/picking取餐中/picked已取餐/delivering配送中/delivered已送达/completed已完成/cancelled已取消';
COMMENT ON COLUMN rider_locations.accuracy IS '定位精度（米）';
COMMENT ON COLUMN rider_locations.speed IS '速度 m/s';
COMMENT ON COLUMN rider_locations.heading IS '方向角度';
COMMENT ON COLUMN delivery_pool.distance IS '配送距离（米）';
COMMENT ON COLUMN recommend_configs.max_distance IS '最大推荐距离（米）';
