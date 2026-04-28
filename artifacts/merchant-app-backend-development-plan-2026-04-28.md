# Merchant App Backend Development Plan

Date: 2026-04-28
Branch: `feature/merchant-android-app`
Risk class: `G2`
Status: closed for the planned Stage 1-4 backend foundation

## 1. Scope And Principles

This plan covers backend work needed by `merchant_app/` after the native-vendor-push route decision.

The first backend gap is the Android merchant app device registry for native push tokens. Existing backend capabilities already cover app bind login, token refresh, merchant order list, merchant accept/reject, agreements, and table management routes. Those existing capabilities should be reused instead of duplicated.

Architecture rules for all phases:

- Keep the HTTP split: `api/` for transport, `logic/` for business semantics, `db/query` + `db/sqlc` for persistence.
- Keep device registration cohesive under a merchant-app-device capability; do not mix it with existing `/v1/merchant/devices` printer/device-management semantics.
- Use authenticated server-side merchant context. Do not trust client-provided `merchant_id`, `owner_id`, role, or status fields.
- Device registration must be idempotent by device identity and must not leave one physical app install actively bound to multiple merchants.
- Do not log raw push tokens or raw provider payloads.
- Run generation immediately after SQL or Swagger source changes, then run the smallest meaningful validation.

## 2. Stage Gate Workflow

Each stage follows the same gate:

1. Implement the stage in small commits or one focused patch set.
2. Run focused validation.
3. Review the changed code against backend standards and the relevant task card acceptance criteria.
4. Fix all review findings.
5. Review again.
6. If no findings remain, sync docs and task-card status.
7. Move to the next stage.

No stage is considered complete until code, generation, validation, review, fixes, and docs are all accounted for.

## 3. Stage 1: Merchant App Push Device Registry

Goal: implement the P0 device registration contract used by the Android app native push layer.

Endpoints:

- `POST /v1/merchant/device/register`
- `PUT /v1/merchant/device/heartbeat`
- `DELETE /v1/merchant/device/{device_id}`

### Card 1.1 Schema And SQL

Files likely touched:

- `locallife/db/migration/000221_create_merchant_app_devices.up.sql`
- `locallife/db/migration/000221_create_merchant_app_devices.down.sql`
- `locallife/db/query/merchant_app_device.sql`
- generated `locallife/db/sqlc/*`

Acceptance criteria:

- Add a dedicated table for merchant app push devices, separate from printer devices and fraud-oriented `user_devices`.
- Store `merchant_id`, `user_id`, `device_id`, `platform`, `provider`, `push_token`, `device_model`, `os_version`, `app_version`, status, registration timestamps, and last heartbeat time.
- Enforce a single active binding for a device install. If the same `device_id` is registered under another merchant, the new authenticated registration must supersede the old active binding.
- Add indexes for merchant lookup, active provider/token lookup, and heartbeat/reconciliation scans.
- Do not use `SELECT *`; all queries list columns explicitly.
- Expose `RegisterMerchantAppDeviceTx` and `UpdateMerchantAppDeviceHeartbeatTx` persistence boundaries so logic does not coordinate token deactivation and active-device updates as separate calls.

Validation:

- `make sqlc`
- `make check-generated`
- `bash /home/sam/ll-merchant-app/.github/scripts/backend_sql_guard.sh HEAD~1 HEAD`
- `go test ./db/sqlc`

Current review result:

- Card 1.1 review passed after adding transaction boundaries for registration and heartbeat token updates.
- Generated code and mocks are in sync.
- No `SELECT *` or unbounded `:many` query was introduced.

Review checkpoint:

- Confirm ownership and idempotency are enforced at persistence/logic boundary, not only by handler checks.
- Confirm no orphan SQL remains without production callers.

### Card 1.2 Logic Service

Files likely touched:

- `locallife/logic/merchant_app_device.go`
- `locallife/logic/merchant_app_device_test.go`

Acceptance criteria:

- Add a cohesive service or functions for register, heartbeat, and unregister.
- Use `context.Context` as the first argument.
- Accept authenticated `merchant_id` and `user_id` from server-side context.
- Normalize provider/platform values and reject unsupported values with stable 400 errors.
- Registration is idempotent and updates token/device metadata without duplicating active rows.
- Heartbeat updates only the authenticated merchant's active device.
- Unregister is idempotent for the authenticated merchant and device ID.
- Return stable result structs for handler response mapping.

Validation:

- `go test ./logic -run 'Test.*MerchantAppDevice'`
- If package-level impact is low, `go test ./logic`

Current review result:

- Card 1.2 review passed. Logic normalizes and validates platform/provider/device metadata, uses authenticated merchant/user IDs, maps missing active heartbeat devices to stable 404 errors, and does not log raw push tokens.
- Focused and package-level logic tests passed.

Review checkpoint:

- Confirm no raw push token logging.
- Confirm wrong-merchant access is rejected or handled as an idempotent no-op only where contract explicitly says so.

### Card 1.3 API Handlers And Routes

Files likely touched:

- `locallife/api/merchant_app_device.go`
- `locallife/api/server.go`
- `locallife/api/merchant_app_device_test.go`
- Swagger generated docs

Acceptance criteria:

- Register routes under authenticated `/v1/merchant/device` path.
- Use existing auth middleware and merchant staff resolution pattern.
- Strong typed request and response structs; no ad hoc `map[string]any` responses.
- Request fields propagate into logic and responses come from logic results.
- Business errors map through existing request-error path.
- Unexpected failures go through existing internal logging boundary.
- Swagger annotations match actual route and method.

Validation:

- `go test ./api -run 'Test.*MerchantAppDeviceAPI'`
- `make swagger`
- `make check-generated`

Current review result:

- Card 1.3 review found Swagger `@Router` paths missing the `/v1` prefix; this was fixed and Swagger was regenerated.
- API routes use existing auth and merchant staff middleware, derive merchant context server-side, and response structs do not include `push_token`.
- Focused API tests passed after the Swagger route fix.

Review checkpoint:

- Confirm route semantics and status codes match API contract standards.
- Confirm object-level merchant boundary is server-side, not client-provided.

### Card 1.4 Stage Review And Docs Sync

Files likely touched:

- `merchant_app/docs/backend-interface-requirements.md`
- this plan document
- optional backend API docs if the repo has a matching active doc

Acceptance criteria:

- Review cards 1.1-1.3 as a complete execution path.
- Fix findings before updating stage status.
- Update docs from "需确认/补齐" to implemented details only after code and validation pass.
- Record validation commands and residual risks.

Current review result:

- Stage 1 review passed after fixing Swagger route paths to `/v1/merchant/device/...`.
- Device registry implementation is cohesive across migration/query/store tx, logic, API, tests, and generated docs.
- `merchant_app/docs/backend-interface-requirements.md` has been synced from "需确认/补齐" to implemented details for register, heartbeat, and unregister.

Validation completed:

- `make sqlc`
- `make swagger`
- `make check-generated` (`generated artifacts are in sync`)
- `bash /home/sam/ll-merchant-app/.github/scripts/backend_sql_guard.sh HEAD~1 HEAD`
- `go test ./db/sqlc`
- `go test ./logic -run 'Test.*MerchantAppDevice'`
- `go test ./logic`
- `go test ./api -run 'Test.*MerchantAppDeviceAPI'`
- `go test ./api -run 'Test.*MerchantAppDeviceAPI' && go test ./logic -run 'Test.*MerchantAppDevice' && go test ./db/sqlc`

Residual risks:

- Stage 1 stores device tokens and exposes lookup queries, but actual vendor REST delivery is deferred to Stage 4.
- SQL guard is diff-based and reported no changed SQL query files matched its guardrails; manual query review confirmed all new queries list columns explicitly and do not use `SELECT *`.

## 4. Stage 2: App Version Query

Goal: provide the P1 endpoint used by app update checks without coupling it to push or order flows.

Endpoint:

- `GET /v1/app/version/latest`

Task cards:

- Card 2.1 schema/query for app versions. Status: complete.
- Card 2.2 logic for platform/channel/package/version selection. Status: complete.
- Card 2.3 API handler and tests. Status: complete.
- Card 2.4 stage review and docs sync. Status: complete.

Acceptance highlights:

- Return explicit no-update state instead of `404` when no newer version is active.
- Keep force-update and checksum fields stable.
- Do not expose internal storage paths or draft releases.

Current review result:

- Stage 2 review found missing nonblank constraints for `package_name` and `version_name`; the migration was fixed and revalidated.
- `GET /v1/app/version/latest` is public under `/v1`, validates platform/channel/package/version inputs, returns active releases only when `published_at <= now()`, and returns `has_update=false` with HTTP 200 when no newer version exists.
- `merchant_app/docs/backend-interface-requirements.md` has been synced from missing to implemented details for online upgrade checks.

Validation completed:

- `make sqlc`
- `make swagger`
- `make check-generated` (`generated artifacts are in sync`)
- `bash /home/sam/ll-merchant-app/.github/scripts/backend_sql_guard.sh HEAD~1 HEAD`
- `go test ./logic -run 'TestGetLatestAppVersion'`
- `go test ./api -run 'TestGetLatestAppVersionAPI'`
- `go test ./logic`
- `go test ./api -run 'Test(GetLatestAppVersionAPI|.*MerchantAppDeviceAPI)'`
- `go test ./db/sqlc`

Residual risks:

- Stage 2 provides query-time release discovery only; release creation/admin workflows are not implemented in this stage.
- Active release rows require operational data seeding or future admin tooling before production clients can receive real APK upgrade metadata.

## 5. Stage 3: New Order Notification Payload Contract

Goal: lock the backend-side payload consumed by WebSocket and future native push delivery.

Task cards:

- Card 3.1 audit current order-created/payment-success notification path. Status: complete.
- Card 3.2 define typed merchant app notification payload builder. Status: complete.
- Card 3.3 add focused tests for payload fields and `message_id` stability. Status: complete.
- Card 3.4 stage review and docs sync. Status: complete.

Acceptance highlights:

- WebSocket and push payloads share the same `message_id` for dedupe.
- Payload contains `message_id`, `order_id`, `title`, `content`, `amount`, and `shop_name`.
- No push side effect is emitted inside transaction-owned critical sections.

Current review result:

- Stage 3 audit confirmed the real merchant new-order path is payment-success worker dispatch, not order creation itself.
- Added a typed `BuildMerchantAppNewOrderNotification` payload builder with stable `merchant_app:new_order:{order_id}` message IDs.
- WebSocket `new_order` messages now set `websocket.Message.ID` and include `message_id`, `event`, `order_id`, `title`, `content`, `amount`, and `shop_name` in `data` while retaining the existing order snapshot fields.
- Review found an accidental station-notification title behavior change; the existing `🆕 新订单` title was restored before docs sync.
- `merchant_app/docs/backend-interface-requirements.md` has been synced from pending contract to locked new-order payload details.

Validation completed:

- `go test ./logic -run 'TestBuildMerchantAppNewOrderNotification'`
- `go test ./worker -run 'TestNotifyMerchantNewOrder_PublishesMerchantAppPayload|TestProcessTaskPaymentDomainOutbox_PublishesOrderPaymentSucceeded'`
- `go test ./logic`
- `go test ./worker -run 'TestNotifyMerchantNewOrder_PublishesMerchantAppPayload|TestProcessTaskPaymentDomainOutbox_PublishesOrderPaymentSucceeded|Test.*OrderPayment'`

Residual risks:

- Stage 3 locks the payload contract and WebSocket shape, but native vendor push dispatch is still deferred to Stage 4.
- Current message ID is stable by order ID and event type; if future retries add multiple notification variants per order, the event segment must remain explicit and versioned.

## 6. Stage 4: Native Push Gateway Boundary

Goal: introduce a backend push gateway abstraction after device registry and payload contract are stable.

Task cards:

- Card 4.1 provider interface and no-op/test provider. Status: complete.
- Card 4.2 device-token lookup by merchant/provider. Status: complete.
- Card 4.3 dispatch orchestration with retry classification and observability. Status: complete.
- Card 4.4 stage review and docs sync. Status: complete.

Acceptance highlights:

- Provider selection is based on persisted `provider`, not client-controlled request-time fields.
- Retryable and non-retryable provider errors are classified explicitly.
- Vendor credentials come from config, never hardcoded.
- No raw token or raw provider payload appears in logs.

Current review result:

- Added `MerchantAppPushProvider`, static registry, no-op provider, retryable/permanent error wrappers, and `MerchantAppPushDispatcher`.
- Dispatcher lists active merchant app devices, selects providers from persisted device `provider`, and sends typed merchant app notification payloads without accepting request-time provider overrides.
- Review found raw provider error strings could leak through dispatch results; results now expose only safe retryable/permanent classification text.
- `merchant_app/docs/backend-interface-requirements.md` has been synced to note the gateway boundary is present while real vendor REST clients remain deferred.

Validation completed:

- `go test ./logic -run 'TestMerchantAppPushDispatcher|TestBuildMerchantAppNewOrderNotification|TestGetLatestAppVersion|Test.*MerchantAppDevice'`
- `go test ./logic`

Residual risks:

- Stage 4 does not implement Huawei/Honor/Xiaomi/OPPO/vivo HTTP clients or credential configuration; those remain deferred until vendor credentials and payload templates are supplied.
- Dispatcher is not yet invoked from the payment-success worker; wiring real dispatch should happen together with provider credentials and retry policy to avoid a half-active production path.

## 7. Deferred Work

The following are intentionally deferred until the P0 registry path is stable:

- Real Huawei/Honor/Xiaomi/OPPO/vivo REST provider clients.
- Timeout re-push scheduler for unaccepted orders.
- Platform operations alerting for missed merchant acceptance.
- App-side expansion of additional mini-program merchant capabilities beyond table management.

## 8. Current Next Action

Stages 1-4 are complete for the planned backend foundation. Next work should decide whether to seed/admin-manage `app_versions`, wire the native push dispatcher into the payment-success worker with real provider clients, or continue merchant App feature parity beyond table management.

## 9. Closeout

Closed on 2026-04-28 after completing the planned backend foundation, focused validation, stage reviews, fixes, generated artifact checks, and App/backend contract documentation sync.

Deferred items in Section 7 are intentionally outside this closed scope and should start from a new plan or follow-up task card before implementation.