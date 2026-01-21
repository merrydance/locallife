ALTER TABLE "web_login_sessions" ADD COLUMN "poll_token" text;

CREATE UNIQUE INDEX IF NOT EXISTS "web_login_sessions_poll_token_key"
  ON "web_login_sessions" ("poll_token");
