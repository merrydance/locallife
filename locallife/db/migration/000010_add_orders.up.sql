-- M7: 订单核心系统
-- 支持外卖(takeout)、堂食(dine_in)、打包(takeaway)、预定(reservation)四种订单类型
-- 订单状态流转：pending → paid → preparing → ready → delivering → completed / cancelled

-- 订单主表
CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    order_no TEXT UNIQUE NOT NULL,
    
    -- 关联
    user_id BIGINT NOT NULL REFERENCES users(id),
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    
    -- 订单类型
    order_type TEXT NOT NULL,
    
    -- 配送信息（外卖专用）
    address_id BIGINT REFERENCES user_addresses(id),
    delivery_fee BIGINT NOT NULL DEFAULT 0,
    delivery_distance INTEGER,
    
    -- 桌台信息（堂食专用）
    table_id BIGINT REFERENCES tables(id),
    
    -- 预定关联（预定包间的菜品订单）
    reservation_id BIGINT REFERENCES table_reservations(id),
    
    -- 金额（单位：分）
    subtotal BIGINT NOT NULL,
    discount_amount BIGINT NOT NULL DEFAULT 0,
    packing_fee BIGINT NOT NULL DEFAULT 0,
    delivery_fee_discount BIGINT NOT NULL DEFAULT 0,
    total_amount BIGINT NOT NULL,
    
    -- 状态
    status TEXT NOT NULL DEFAULT 'pending',
    
    -- 支付信息
    payment_method TEXT,
    paid_at TIMESTAMPTZ,
    
    -- 备注
    notes TEXT,
    
    -- 时间戳
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    cancel_reason TEXT,
    
    -- 约束
    CONSTRAINT orders_order_type_check CHECK (order_type IN ('takeout', 'dine_in', 'takeaway', 'reservation')),
    CONSTRAINT orders_status_check CHECK (status IN ('pending', 'paid', 'preparing', 'ready', 'delivering', 'completed', 'cancelled')),
    CONSTRAINT orders_payment_method_check CHECK (payment_method IS NULL OR payment_method IN ('wechat', 'balance')),
    CONSTRAINT orders_subtotal_check CHECK (subtotal >= 0),
    CONSTRAINT orders_total_amount_check CHECK (total_amount >= 0)
);

-- 索引
CREATE UNIQUE INDEX orders_order_no_idx ON orders(order_no);
CREATE INDEX orders_user_id_idx ON orders(user_id);
CREATE INDEX orders_merchant_id_idx ON orders(merchant_id);
CREATE INDEX orders_status_idx ON orders(status);
CREATE INDEX orders_created_at_idx ON orders(created_at);
CREATE INDEX orders_merchant_status_created_idx ON orders(merchant_id, status, created_at);
CREATE INDEX orders_user_status_idx ON orders(user_id, status);

-- 订单明细表
CREATE TABLE order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    
    -- 商品信息（可以是菜品或套餐）
    dish_id BIGINT REFERENCES dishes(id),
    combo_id BIGINT REFERENCES combo_sets(id),
    
    -- 快照数据
    name TEXT NOT NULL,
    unit_price BIGINT NOT NULL,
    quantity SMALLINT NOT NULL,
    subtotal BIGINT NOT NULL,
    
    -- 个性化选项
    customizations JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    -- 约束：菜品或套餐二选一
    CONSTRAINT order_items_dish_or_combo CHECK (
        (dish_id IS NOT NULL AND combo_id IS NULL) OR
        (dish_id IS NULL AND combo_id IS NOT NULL)
    ),
    CONSTRAINT order_items_quantity_check CHECK (quantity > 0),
    CONSTRAINT order_items_unit_price_check CHECK (unit_price >= 0),
    CONSTRAINT order_items_subtotal_check CHECK (subtotal >= 0)
);

-- 索引
CREATE INDEX order_items_order_id_idx ON order_items(order_id);
CREATE INDEX order_items_dish_id_idx ON order_items(dish_id);
CREATE INDEX order_items_combo_id_idx ON order_items(combo_id);

-- 订单状态变更日志表
CREATE TABLE order_status_logs (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    
    from_status TEXT,
    to_status TEXT NOT NULL,
    
    operator_id BIGINT REFERENCES users(id),
    operator_type TEXT,
    notes TEXT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    -- 约束
    CONSTRAINT order_status_logs_operator_type_check CHECK (
        operator_type IS NULL OR operator_type IN ('user', 'merchant', 'system')
    )
);

-- 索引
CREATE INDEX order_status_logs_order_id_idx ON order_status_logs(order_id);
CREATE INDEX order_status_logs_created_at_idx ON order_status_logs(created_at);

-- 云打印机配置表
CREATE TABLE cloud_printers (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    
    printer_name TEXT NOT NULL,
    printer_sn TEXT UNIQUE NOT NULL,
    printer_key TEXT NOT NULL,
    printer_type TEXT NOT NULL,
    
    -- 打印场景配置
    print_takeout BOOLEAN NOT NULL DEFAULT true,
    print_dine_in BOOLEAN NOT NULL DEFAULT true,
    print_reservation BOOLEAN NOT NULL DEFAULT true,
    
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    
    -- 约束
    CONSTRAINT cloud_printers_printer_type_check CHECK (printer_type IN ('feieyun', 'yilianyun', 'other'))
);

-- 索引
CREATE UNIQUE INDEX cloud_printers_printer_sn_idx ON cloud_printers(printer_sn);
CREATE INDEX cloud_printers_merchant_id_idx ON cloud_printers(merchant_id);
CREATE INDEX cloud_printers_merchant_active_idx ON cloud_printers(merchant_id, is_active);

-- 打印日志表
CREATE TABLE print_logs (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    printer_id BIGINT NOT NULL REFERENCES cloud_printers(id),
    
    print_content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT,
    
    printed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    -- 约束
    CONSTRAINT print_logs_status_check CHECK (status IN ('pending', 'success', 'failed'))
);

-- 索引
CREATE INDEX print_logs_order_id_idx ON print_logs(order_id);
CREATE INDEX print_logs_printer_id_idx ON print_logs(printer_id);
CREATE INDEX print_logs_status_idx ON print_logs(status);
CREATE INDEX print_logs_created_at_idx ON print_logs(created_at);

-- 订单展示配置表
CREATE TABLE order_display_configs (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT UNIQUE NOT NULL REFERENCES merchants(id),
    
    -- 打印配置
    enable_print BOOLEAN NOT NULL DEFAULT true,
    print_takeout BOOLEAN NOT NULL DEFAULT true,
    print_dine_in BOOLEAN NOT NULL DEFAULT true,
    print_reservation BOOLEAN NOT NULL DEFAULT true,
    
    -- 语音播报配置
    enable_voice BOOLEAN NOT NULL DEFAULT false,
    voice_takeout BOOLEAN NOT NULL DEFAULT true,
    voice_dine_in BOOLEAN NOT NULL DEFAULT true,
    
    -- KDS大屏配置
    enable_kds BOOLEAN NOT NULL DEFAULT false,
    kds_url TEXT,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

-- 索引
CREATE UNIQUE INDEX order_display_configs_merchant_id_idx ON order_display_configs(merchant_id);
