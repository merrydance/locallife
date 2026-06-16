CREATE INDEX IF NOT EXISTS idx_dining_sessions_open_active_order_opened_at
    ON dining_sessions(opened_at, id)
    WHERE status = 'open'
      AND active_order_id IS NOT NULL;
