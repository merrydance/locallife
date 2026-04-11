CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGSERIAL PRIMARY KEY,
  actor_user_id BIGINT,
  actor_role TEXT NOT NULL,
  action TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id BIGINT,
  region_id BIGINT,
  request_id TEXT,
  trace_id TEXT,
  client_ip TEXT,
  user_agent TEXT,
  metadata JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS audit_logs_region_id_idx
  ON audit_logs (region_id);

CREATE INDEX IF NOT EXISTS audit_logs_actor_user_id_idx
  ON audit_logs (actor_user_id);

CREATE INDEX IF NOT EXISTS audit_logs_action_idx
  ON audit_logs (action);
