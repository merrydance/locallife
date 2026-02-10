-- P1-001: 防止同桌多开台
-- 确保每个桌台在同一时间只能有一个 open 状态的会话
CREATE UNIQUE INDEX unique_active_dining_session ON dining_sessions (table_id) WHERE status = 'open';
