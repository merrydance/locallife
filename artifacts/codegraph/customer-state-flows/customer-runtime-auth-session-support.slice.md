# Customer Runtime Auth Session Support Slice

Status: customer-runtime support slice created 2026-06-14 after completeness re-review
Risk class: G3 boundary - customer identity, token/session lifecycle, cross-device web login confirmation, authenticated telemetry
Scope: Mini Program app startup/request runtime/user-center scan confirmation/logger -> `/v1/auth/**`, `/v1/users/me`, `/v1/logs/error` routes -> auth/session/web-login handlers -> `sessions` and `web_login_sessions` SQL tables -> cross-role/account exits

## Variant Coverage

This slice covers:

- App startup silent login, valid-token reuse, refresh-token renewal, fallback `wx.login`, user profile hydration, and delayed location rehydration after token readiness.
- Request runtime token preflight, 401/envelope token-expired refresh, single-flight refresh behavior, request retry, and auth failure surface.
- User center scan of Web login QR codes, status lookup, signature-bearing confirmation, and Web login session convergence.
- Authenticated frontend error-log upload to `/v1/logs/error` as non-blocking telemetry.
- Boundary exits that are reachable from user center scan but not ordinary customer business closure: merchant invite/bind and app-bind code generation.

This slice does not fully cover:

- Web console QR generation and consume flow beyond the session state touched by customer confirmation.
- Merchant/rider/operator app-bind or invite onboarding after a customer scans an invite code.
- Authorization policy internals for role workbenches. Customer pages consume the resulting roles/workbenches but do not own those role lifecycles.
- Client log storage or analytics durability. The backend currently logs the payload and returns `ok`.

## Product Invariant

All ordinary customer flows depend on runtime identity, but identity support is not itself a takeout, dine-in, reservation, order, or profile business transition:

- Customer pages must not treat local user info or URL params as identity truth. Authenticated routes use backend token payloads and `/v1/users/me`.
- Access-token refresh must be single-flight and retry the original request after backend-confirmed token renewal.
- `wx.login` creates or recovers the customer account by WeChat openid and persists a server-side session.
- Web login confirmation must validate QR signature, session TTL, current authenticated user, and conflict states before marking a session confirmed.
- Client error logging must never block the customer business flow.

## Primary Forward Chain

1. App startup runs `silentLogin`, first reusing a still-valid token, then trying refresh-token renewal, then falling back to full `wx.login`.
   Evidence: `weapp/miniprogram/app.ts:236`, `weapp/miniprogram/app.ts:243`, `weapp/miniprogram/app.ts:249`, `weapp/miniprogram/app.ts:267`, `weapp/miniprogram/app.ts:313`, `weapp/miniprogram/app.ts:338`.

2. Full login calls `wx.login`, posts the code to `/v1/auth/wechat-login`, stores returned access/refresh tokens, and returns the customer user profile.
   Evidence: `weapp/miniprogram/utils/wechat-login-session.ts:27`, `weapp/miniprogram/utils/wechat-login-session.ts:37`, `weapp/miniprogram/utils/wechat-login-session.ts:74`, `weapp/miniprogram/utils/wechat-login-session.ts:128`, `weapp/miniprogram/utils/wechat-login-session.ts:136`, `weapp/miniprogram/api/auth.ts:144`.

3. Request runtime checks token freshness before authenticated requests, refreshes by `/v1/auth/refresh`, and retries the original request on HTTP 401 or envelope token-expired responses.
   Evidence: `weapp/miniprogram/utils/request.ts:336`, `weapp/miniprogram/utils/request.ts:397`, `weapp/miniprogram/utils/request.ts:643`, `weapp/miniprogram/utils/request-auth-refresh.ts:21`, `weapp/miniprogram/utils/request-auth-refresh.ts:77`, `weapp/miniprogram/utils/request-auth-refresh.ts:92`.

4. User center scan can validate and confirm Web login sessions. The customer-visible branch checks session status, requires QR signature fields, and calls `/v1/auth/web-login/confirm` as the authenticated customer.
   Evidence: `weapp/miniprogram/pages/user_center/index.ts:372`, `weapp/miniprogram/pages/user_center/index.ts:380`, `weapp/miniprogram/pages/user_center/index.ts:472`, `weapp/miniprogram/pages/user_center/index.ts:491`, `weapp/miniprogram/services/user-profile.ts:22`, `weapp/miniprogram/services/user-profile.ts:26`, `weapp/miniprogram/api/auth.ts:195`, `weapp/miniprogram/api/auth.ts:206`.

5. Backend public auth routes register WeChat login, token refresh, and Web login session status/creation/consume. Customer confirmation is registered under the authenticated group.
   Evidence: `locallife/api/server.go:527`, `locallife/api/server.go:532`, `locallife/api/server.go:533`, `locallife/api/server.go:534`, `locallife/api/server.go:535`, `locallife/api/server.go:536`, `locallife/api/server.go:654`.

6. Backend WeChat login exchanges code for openid, creates or loads the user, creates access/refresh tokens, persists a session, and returns roles/workbenches with the user response.
   Evidence: `locallife/api/wechat.go:46`, `locallife/api/wechat.go:53`, `locallife/api/wechat.go:75`, `locallife/api/wechat.go:118`, `locallife/api/wechat.go:129`, `locallife/api/wechat.go:152`, `locallife/api/wechat.go:174`, `locallife/db/query/session.sql:1`.

7. Backend token refresh verifies refresh token, reads the stored session, creates replacement access/refresh tokens, and updates the session transactionally with row locking.
   Evidence: `locallife/api/token.go:39`, `locallife/api/token.go:46`, `locallife/api/token.go:58`, `locallife/api/token.go:108`, `locallife/api/token.go:110`, `locallife/db/query/session.sql:22`, `locallife/db/query/session.sql:28`, `locallife/db/query/session.sql:36`.

8. Backend Web login session handlers create/poll/expire/confirm sessions in `web_login_sessions`; Mini Program customer confirmation verifies QR signature and authenticated user before confirming.
   Evidence: `locallife/api/web_login.go:151`, `locallife/api/web_login.go:211`, `locallife/api/web_login.go:316`, `locallife/api/web_login.go:322`, `locallife/api/web_login.go:355`, `locallife/api/web_login.go:366`, `locallife/db/query/web_login_session.sql:1`, `locallife/db/query/web_login_session.sql:23`.

9. Frontend logger uploads non-dev errors to `/v1/logs/error` only when a token exists; backend accepts the authenticated payload, drops scanner traffic, logs, and returns `ok`.
   Evidence: `weapp/miniprogram/utils/logger.ts:166`, `weapp/miniprogram/utils/logger.ts:168`, `weapp/miniprogram/utils/logger.ts:179`, `locallife/api/server.go:568`, `locallife/api/server.go:572`, `locallife/api/client_log.go:19`, `locallife/api/client_log.go:29`, `locallife/api/client_log.go:42`.

10. CodeGraph cross-check was run for `wechatLogin` and `webLoginSession`; it located the Mini Program auth wrapper/session utility and backend auth/session handlers used in this slice.
    Evidence: `/home/sam/.nvm/versions/node/v24.12.0/bin/codegraph query wechatLogin`, `/home/sam/.nvm/versions/node/v24.12.0/bin/codegraph query webLoginSession`.

## SQL And Durable State Boundaries

- `users`: WeChat-openid-owned customer identity and role/workbench response projection.
- `sessions`: access/refresh token hashes, expiry, user agent, client IP, revocation, refresh row lock.
- `web_login_sessions`: Web login QR code, poll token, pending/confirmed/consumed/expired status, confirmed user, TTL, client IPs.
- No durable table for `/v1/logs/error`; current behavior is structured backend logging only.

## Trust, Authorization, And Tenant Checks

- `/v1/auth/wechat-login` and `/v1/auth/refresh` are public but rate limited; authenticated business routes must still require Bearer tokens.
- Refresh verifies token type and stored session ownership before returning new tokens.
- Web login confirmation validates QR signature and requires authenticated Mini Program user context.
- Web login confirmation is idempotent only for the same confirmed user; another user sees a conflict.
- Client error log upload requires an existing token in production and does not expose a customer business mutation.

## Idempotency And Duplicate-Submit Checks

- `ensureWechatLoginSession` reuses an in-flight login promise to avoid repeated `wx.login` calls.
- `performTokenRefresh` reuses an in-flight refresh promise so parallel API calls do not race refresh.
- Backend refresh uses `RefreshSessionTx` and `FOR UPDATE` to update one session row.
- Web login session creation retries code collision; confirmation returns current status for same-user duplicate confirmation.
- Error-log upload is best effort and silently fails client-side.

## Recovery And Async Convergence Paths

- App startup can continue browsing after final silent-login failure; authenticated customer actions fail through request auth errors until token recovery succeeds.
- Request runtime retries once after token refresh on 401/token-expired, then clears token and surfaces a stable login-expired message.
- Location region/geocode waits for token readiness before calling authenticated location routes.
- Web login pending sessions expire on status check or confirmation after TTL.
- Client log reporting does not retry or block customer action.

## Frontend Draft And Backend Rehydration

- Stored token info, cached user profile, QR raw text, and web-login `sig/ts` params are frontend/runtime state.
- `/v1/users/me`, `sessions`, and `web_login_sessions` are backend authority.
- User center invite-code binding is a cross-role account exit, not ordinary customer closure.

## Test Coverage Signals

Observed tests:

- `locallife/api/wechat_test.go` covers WeChat login handler behavior.
- `locallife/api/token_test.go` covers refresh token/session behavior, including app-bind duration branch.
- `locallife/api/web_login_test.go` and related Web login tests cover session creation/confirmation/consume branches.
- CodeGraph query `wechatLogin` confirmed active frontend/backend symbols.

Missing high-value tests:

- Mini Program cold-start regression for valid token, expired token with refresh, and full `wx.login` fallback.
- Request runtime regression for concurrent 401 responses sharing one refresh and retrying original requests once.
- User center Web login QR scan failure branches for missing `sig/ts`, expired session, and conflict user.
- Client-log payload redaction and production-only behavior.

## Gaps And Refactor Notes

- Auth/session support is intentionally separate from the six business slices so each customer business flow can reference it without duplicating runtime details.
- `generateAppBindCode` and invite-code binding are reachable from shared user-profile/auth wrappers but are cross-role/app-binding exits.
- `/v1/role-access` and `/v1/app/version/latest` exist as public metadata routes; no ordinary customer page currently calls them.

## Branch Exhaustion

- Entry branches checked: app launch silent login, token reuse, refresh-token renewal, `wx.login` fallback, request preflight, 401 retry, user center Web login scan, client error logging.
- Request branches checked: `/v1/auth/wechat-login`, `/v1/auth/refresh`, `/v1/users/me`, `/v1/auth/web-login/sessions/:code`, `/v1/auth/web-login/confirm`, `/v1/logs/error`.
- Backend state branches checked: user exists/create, session create, session refresh, revoked/expired/missing session, Web login pending/confirmed/consumed/expired/conflict, scanner client-log traffic.
- Async branches checked: in-flight login promise, in-flight token refresh promise, Web login polling/expiration, non-blocking log upload.
- Dead/orphan branches checked: app-bind, bind-merchant, role metadata, app version, and Web consume/generation are boundary or non-customer page exits, not ordinary customer business closure.
