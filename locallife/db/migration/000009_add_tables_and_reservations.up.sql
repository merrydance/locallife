-- M5: 桌台与包间管理
-- 桌台分为普通桌台(table)和包间(room)
-- 包间预定支持两种模式：定金模式(到店点菜)和全款模式(在线点菜)

-- 桌台/包间表
CREATE TABLE tables (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    
    table_no TEXT NOT NULL,
    table_type TEXT NOT NULL DEFAULT 'table',
    capacity SMALLINT NOT NULL,
    description TEXT,
    
    -- 包间专用
    minimum_spend BIGINT,
    
    -- 二维码
    qr_code_url TEXT,
    
    -- 状态
    status TEXT NOT NULL DEFAULT 'available',
    current_reservation_id BIGINT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    
    -- 约束
    CONSTRAINT tables_table_type_check CHECK (table_type IN ('table', 'room')),
    CONSTRAINT tables_status_check CHECK (status IN ('available', 'occupied', 'disabled')),
    CONSTRAINT tables_capacity_check CHECK (capacity > 0),
    CONSTRAINT tables_minimum_spend_check CHECK (minimum_spend IS NULL OR minimum_spend >= 0)
);

-- 索引
CREATE UNIQUE INDEX tables_merchant_table_no_idx ON tables(merchant_id, table_no);
CREATE INDEX tables_merchant_id_idx ON tables(merchant_id);
CREATE INDEX tables_merchant_type_status_idx ON tables(merchant_id, table_type, status);

-- 桌台标签关联表
CREATE TABLE table_tags (
    id BIGSERIAL PRIMARY KEY,
    table_id BIGINT NOT NULL REFERENCES tables(id) ON DELETE CASCADE,
    tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 索引
CREATE INDEX table_tags_table_id_idx ON table_tags(table_id);
CREATE UNIQUE INDEX table_tags_table_tag_idx ON table_tags(table_id, tag_id);

-- 桌台预定表
CREATE TABLE table_reservations (
    id BIGSERIAL PRIMARY KEY,
    table_id BIGINT NOT NULL REFERENCES tables(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    
    -- 预定信息
    reservation_date DATE NOT NULL,
    reservation_time TIME NOT NULL,
    guest_count SMALLINT NOT NULL,
    
    -- 联系人
    contact_name TEXT NOT NULL,
    contact_phone TEXT NOT NULL,
    
    -- 支付模式与金额
    payment_mode TEXT NOT NULL DEFAULT 'deposit',
    deposit_amount BIGINT NOT NULL DEFAULT 0,
    prepaid_amount BIGINT NOT NULL DEFAULT 0,
    
    -- 退款截止时间
    refund_deadline TIMESTAMPTZ NOT NULL,
    
    -- 状态
    status TEXT NOT NULL DEFAULT 'pending',
    
    -- 支付截止时间
    payment_deadline TIMESTAMPTZ NOT NULL,
    
    notes TEXT,
    
    -- 时间戳
    paid_at TIMESTAMPTZ,
    confirmed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    cancel_reason TEXT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    
    -- 约束
    CONSTRAINT table_reservations_payment_mode_check CHECK (payment_mode IN ('deposit', 'full')),
    CONSTRAINT table_reservations_status_check CHECK (status IN ('pending', 'paid', 'confirmed', 'completed', 'cancelled', 'expired', 'no_show')),
    CONSTRAINT table_reservations_guest_count_check CHECK (guest_count > 0),
    CONSTRAINT table_reservations_amounts_check CHECK (deposit_amount >= 0 AND prepaid_amount >= 0)
);

-- 索引
CREATE INDEX table_reservations_table_id_idx ON table_reservations(table_id);
CREATE INDEX table_reservations_user_id_idx ON table_reservations(user_id);
CREATE INDEX table_reservations_merchant_id_idx ON table_reservations(merchant_id);
CREATE INDEX table_reservations_status_idx ON table_reservations(status);
CREATE INDEX table_reservations_table_date_status_idx ON table_reservations(table_id, reservation_date, status);
CREATE INDEX table_reservations_merchant_date_idx ON table_reservations(merchant_id, reservation_date);
CREATE INDEX table_reservations_payment_deadline_idx ON table_reservations(payment_deadline);

-- 添加外键约束（tables.current_reservation_id）
ALTER TABLE tables 
ADD CONSTRAINT tables_current_reservation_fk 
FOREIGN KEY (current_reservation_id) REFERENCES table_reservations(id) ON DELETE SET NULL;

-- 添加 table 类型的标签
INSERT INTO tags (name, type, sort_order, status) VALUES
('靠窗', 'table', 1, 'active'),
('安静', 'table', 2, 'active'),
('大屏', 'table', 3, 'active'),
('包厢', 'table', 4, 'active'),
('露台', 'table', 5, 'active'),
('卡座', 'table', 6, 'active');
