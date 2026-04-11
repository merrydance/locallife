DROP INDEX IF EXISTS "web_login_sessions_poll_token_key";
ALTER TABLE "web_login_sessions" DROP COLUMN IF EXISTS "poll_token";
