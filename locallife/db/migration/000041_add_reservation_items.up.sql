-- 预定菜品明细表（用于全款模式预点菜品）
CREATE TABLE reservation_items (
    id BIGSERIAL PRIMARY KEY,
    reservation_id BIGINT NOT NULL REFERENCES table_reservations(id) ON DELETE CASCADE,
    dish_id BIGINT REFERENCES dishes(id) ON DELETE SET NULL,
    combo_id BIGINT REFERENCES combo_sets(id) ON DELETE SET NULL,
    quantity SMALLINT NOT NULL,
    unit_price BIGINT NOT NULL,  -- 下单时的单价（分）
    total_price BIGINT NOT NULL, -- 小计（分）
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    
    -- 约束：必须有 dish_id 或 combo_id 其中之一
    CONSTRAINT reservation_items_dish_or_combo_check CHECK (
        (dish_id IS NOT NULL AND combo_id IS NULL) OR 
        (dish_id IS NULL AND combo_id IS NOT NULL)
    ),
    CONSTRAINT reservation_items_quantity_check CHECK (quantity > 0),
    CONSTRAINT reservation_items_price_check CHECK (unit_price >= 0 AND total_price >= 0)
);

-- 索引
CREATE INDEX reservation_items_reservation_id_idx ON reservation_items(reservation_id);
CREATE INDEX reservation_items_dish_id_idx ON reservation_items(dish_id) WHERE dish_id IS NOT NULL;
CREATE INDEX reservation_items_combo_id_idx ON reservation_items(combo_id) WHERE combo_id IS NOT NULL;
