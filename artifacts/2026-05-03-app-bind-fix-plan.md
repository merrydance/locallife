# 2026-05-03 App Bind Root-Cause Fix Plan

## Scope

- App changes are implemented in `merchant_app/` only.
- Backend changes are a handoff plan for another engineer; no backend source was changed in this pass.
- Risk class: G3, because the flow touches auth, long-lived app sessions, one-time bind codes, and order-polling reliability.

## Backend Fix Plan

### Backend invariant

A bind code returned by `POST /v1/auth/app-bind/code` must be verifiable by `POST /v1/auth/app-bind/verify` until it expires, unless it was successfully consumed once. The two Redis keys must not diverge:

- `app_bind:<code>` stores `user_id:merchant_id` and is the verification source of truth.
- `app_bind:user:<user_id>` stores the currently displayed code and is only an idempotency index.

### Root cause to fix

The generate endpoint can return the idempotency key value from `app_bind:user:<user_id>` without verifying that `app_bind:<code>` still exists. If the verification key was deleted, consumed, or failed after deletion while the user index remains, the Mini Program keeps showing an un-verifiable code. App verification then fails with `redis_nil` within seconds, even though the UI countdown still says the code is valid.

### Required backend changes

1. In `locallife/api/app_bind.go`, when `existingKey` returns an existing code, also read or atomically validate `app_bind:<existingCode>` before returning it.
2. If `app_bind:<existingCode>` is missing, delete `app_bind:user:<user_id>` and generate a fresh code in the same request.
3. Prefer an atomic Redis flow for generation and reuse:
   - Reuse only when both keys exist and their TTLs are positive.
   - Store both keys with the same TTL.
   - If possible, use a small Lua script or `WATCH`/transaction to prevent races between reuse, generation, and verify.
4. Move one-time deletion later or make it recoverable:
   - Do not permanently consume the code before the session and response can succeed, or
   - Add a short pending/consumed marker that can distinguish duplicate verify from backend post-read failure.
5. Add verify observability fields without leaking the code:
   - reason: `code_missing`, `expired`, `already_consumed`, `redis_error`, `malformed_bind_data`.
   - user index presence and TTL bucket, if safe.
6. Re-check design doc mismatch: `APP_AUTH_BINDING.md` says verify should write `merchant_devices`; current `verifyAppBindCode` only creates session and does not persist device info. Decide whether this is required for push/heartbeat enrollment and add it if it is part of the production contract.
7. Ensure rate limits apply to verify path as designed: 10/min/IP, with stable 429 response and logs.

### Backend regression tests

Add tests around Redis-backed behavior; use the existing backend test patterns and a real or miniredis-style Redis client if available.

1. `generateAppBindCode` returns existing code when both keys exist.
2. `generateAppBindCode` regenerates a fresh code when `app_bind:user:<user_id>` exists but `app_bind:<code>` is missing.
3. `verifyAppBindCode` succeeds once and consumes both keys.
4. A second verify of the same code returns a stable 400 without creating another session.
5. If session creation or downstream user profile construction fails, the chosen consumption behavior is covered explicitly by test.
6. If device registration is part of the contract, verify success creates or updates `merchant_devices` idempotently by user/merchant/device.

### Backend validation commands

From `locallife/`:

```bash
go test ./api -run 'Test.*AppBind|TestRenewAccessToken' -count=1
make test-unit
```

Run `make swagger` if request/response annotations or route docs change. Run `make sqlc` only if adding SQL/query changes for device registration.

## App Fix Plan

### App invariant

The App must not generate avoidable backend 400s or force a healthy session back through bind flow. Specifically:

- Order polling must satisfy the backend list contract by sending `page_id` and `page_size`.
- Auth refresh must parse the backend's unified response envelope.
- Bind login must be single-flight while a bind request is in progress.

### App changes implemented

1. `merchant_app/lib/features/order/order_provider.dart`
   - `fetchOrders()` now calls `/merchant/orders` with `page_id=1&page_size=20`.
   - This stops the 30-second polling 400s shown in production logs.

2. `merchant_app/lib/core/network/api_client.dart`
   - Added `extractApiResponseData()` to unwrap `{code,message,data}` envelopes while preserving legacy unwrapped responses.
   - `_performRefresh()` now reads tokens from the unwrapped data, so interceptor refresh no longer clears a valid session just because the backend returned the standard envelope.

3. Existing working-tree app binding guard
   - `AuthNotifier.loginWithBindingCode()` already has a single-flight guard in the current working tree, covered by `merchant_app/test/auth_notifier_test.dart`.
   - This reduces duplicate App verify calls, but backend must still enforce one-time code correctness.

### App regression tests added

- `merchant_app/test/order_provider_test.dart`
  - Confirms `fetchOrders()` sends `page_id=1&page_size=20`.
- `merchant_app/test/api_client_response_test.dart`
  - Confirms unified envelope unwrap for token refresh data.
  - Confirms legacy unwrapped token payload remains supported.

### App validation commands

From `merchant_app/`:

```bash
PATH="$HOME/.local/bin:$PATH" flutter test
PATH="$HOME/.local/bin:$PATH" flutter analyze
```

Both commands passed in this session.

## Residual Risks

- The backend Redis key inconsistency remains until the backend plan is implemented.
- The App-side duplicate-submit guard reduces accidental duplicate verify, but it cannot make an already stale backend code valid.
- No real-device validation was run on Huawei/Xiaomi/OPPO/vivo in this pass.
