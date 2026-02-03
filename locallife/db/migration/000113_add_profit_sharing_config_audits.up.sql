-- Phase2: 分账规则审计与变更记录（草案）

CREATE TABLE IF NOT EXISTS profit_sharing_config_audits (
  id BIGSERIAL PRIMARY KEY,
  config_id BIGINT NOT NULL REFERENCES profit_sharing_configs(id) ON DELETE CASCADE,
  action TEXT NOT NULL, -- create/update/delete
  actor_id BIGINT,
  actor_role TEXT,
  detail JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_profit_sharing_config_audits_config_id ON profit_sharing_config_audits(config_id);
CREATE INDEX IF NOT EXISTS idx_profit_sharing_config_audits_action ON profit_sharing_config_audits(action);

COMMENT ON TABLE profit_sharing_config_audits IS '分账规则配置审计表（Phase2 草案）';

CREATE OR REPLACE FUNCTION fn_profit_sharing_config_audit() RETURNS trigger AS $$
BEGIN
  IF (TG_OP = 'INSERT') THEN
    INSERT INTO profit_sharing_config_audits (config_id, action, actor_id, actor_role, detail)
    VALUES (
      NEW.id,
      'create',
      NULLIF(current_setting('app.actor_id', true), '')::bigint,
      NULLIF(current_setting('app.actor_role', true), ''),
      jsonb_build_object('after', to_jsonb(NEW)) ||
      CASE
        WHEN NULLIF(current_setting('app.actor_detail', true), '') IS NULL THEN '{}'::jsonb
        ELSE jsonb_build_object('note', NULLIF(current_setting('app.actor_detail', true), ''))
      END
    );
    RETURN NEW;
  ELSIF (TG_OP = 'UPDATE') THEN
    INSERT INTO profit_sharing_config_audits (config_id, action, actor_id, actor_role, detail)
    VALUES (
      NEW.id,
      'update',
      NULLIF(current_setting('app.actor_id', true), '')::bigint,
      NULLIF(current_setting('app.actor_role', true), ''),
      jsonb_build_object('before', to_jsonb(OLD), 'after', to_jsonb(NEW)) ||
      CASE
        WHEN NULLIF(current_setting('app.actor_detail', true), '') IS NULL THEN '{}'::jsonb
        ELSE jsonb_build_object('note', NULLIF(current_setting('app.actor_detail', true), ''))
      END
    );
    RETURN NEW;
  ELSIF (TG_OP = 'DELETE') THEN
    INSERT INTO profit_sharing_config_audits (config_id, action, actor_id, actor_role, detail)
    VALUES (
      OLD.id,
      'delete',
      NULLIF(current_setting('app.actor_id', true), '')::bigint,
      NULLIF(current_setting('app.actor_role', true), ''),
      jsonb_build_object('before', to_jsonb(OLD)) ||
      CASE
        WHEN NULLIF(current_setting('app.actor_detail', true), '') IS NULL THEN '{}'::jsonb
        ELSE jsonb_build_object('note', NULLIF(current_setting('app.actor_detail', true), ''))
      END
    );
    RETURN OLD;
  END IF;
  RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_profit_sharing_config_audit ON profit_sharing_configs;
CREATE TRIGGER trg_profit_sharing_config_audit
AFTER INSERT OR UPDATE OR DELETE ON profit_sharing_configs
FOR EACH ROW EXECUTE FUNCTION fn_profit_sharing_config_audit();
