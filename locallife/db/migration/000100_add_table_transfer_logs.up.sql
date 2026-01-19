-- 转台日志表
CREATE TABLE table_transfer_logs (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id),
    dining_session_id BIGINT NOT NULL REFERENCES dining_sessions(id),
    reservation_id BIGINT REFERENCES table_reservations(id),
    from_table_id BIGINT NOT NULL REFERENCES tables(id),
    to_table_id BIGINT NOT NULL REFERENCES tables(id),
    operator_user_id BIGINT NOT NULL REFERENCES users(id),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX table_transfer_logs_merchant_id_idx ON table_transfer_logs(merchant_id);
CREATE INDEX table_transfer_logs_session_id_idx ON table_transfer_logs(dining_session_id);
CREATE INDEX table_transfer_logs_created_at_idx ON table_transfer_logs(created_at);
