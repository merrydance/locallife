-- Web login sessions for QR-based web login
CREATE TABLE "web_login_sessions" (
  "id" bigserial PRIMARY KEY,
  "code" text UNIQUE NOT NULL,
  "status" text NOT NULL DEFAULT 'pending',
  "user_id" bigint,
  "expires_at" timestamptz NOT NULL,
  "confirmed_at" timestamptz,
  "consumed_at" timestamptz,
  "web_user_agent" text,
  "web_client_ip" text,
  "confirm_client_ip" text,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  "updated_at" timestamptz NOT NULL DEFAULT (now())
);

ALTER TABLE "web_login_sessions" ADD FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON DELETE SET NULL;

CREATE INDEX ON "web_login_sessions" ("status", "expires_at");
CREATE INDEX ON "web_login_sessions" ("user_id");
