-- =============================================
-- M84: 配送围栏事件
-- =============================================

CREATE TABLE "delivery_location_events" (
    "id" bigserial PRIMARY KEY,
    "delivery_id" bigint NOT NULL REFERENCES "deliveries"("id"),
    "order_id" bigint NOT NULL REFERENCES "orders"("id"),
    "rider_id" bigint NOT NULL REFERENCES "riders"("id"),
    "longitude" decimal(10,7) NOT NULL,
    "latitude" decimal(10,7) NOT NULL,
    "accuracy" decimal(6,2),
    "speed" decimal(6,2),
    "event_type" varchar(30) NOT NULL,
    "source" varchar(30) NOT NULL DEFAULT 'gps',
    "recorded_at" timestamptz NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT delivery_location_events_type_check CHECK (event_type IN ('arrive_pickup', 'dwell_pickup', 'arrive_dropoff', 'dwell_dropoff'))
);

CREATE UNIQUE INDEX delivery_location_events_delivery_type_idx ON delivery_location_events(delivery_id, event_type);
CREATE INDEX delivery_location_events_delivery_created_idx ON delivery_location_events(delivery_id, created_at);
CREATE INDEX delivery_location_events_rider_created_idx ON delivery_location_events(rider_id, created_at);

COMMENT ON TABLE delivery_location_events IS '配送围栏事件（到店/驻留/到达收货点）';
COMMENT ON COLUMN delivery_location_events.event_type IS 'arrive_pickup/dwell_pickup/arrive_dropoff/dwell_dropoff';
COMMENT ON COLUMN delivery_location_events.source IS '上报来源，例如 gps';
COMMENT ON COLUMN delivery_location_events.recorded_at IS '事件对应的定位时间';
