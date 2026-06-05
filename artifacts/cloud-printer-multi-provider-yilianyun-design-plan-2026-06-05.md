# Cloud Printer Multi-Provider Unified Design Plan

**Goal:** refactor the existing Feieyun-only cloud-printer path into a provider-neutral capability, then add Yilianyun and Shangpeng under the same model.

**Risk Class:** G2. This touches external provider contracts, async printing, callback or polling recovery, retry/idempotency, merchant-visible printer state, and order fulfillment observability. It is not money movement, but provider drift or status mistakes can hide failed order printing.

**Status:** unified research and design plan only. No implementation has been started in this document.

---

## 1. Unified Direction

The current LocalLife code is Feieyun-shaped even though `cloud_printers.printer_type` already mentions more than Feieyun. Do not add Yilianyun or Shangpeng by copying Feieyun branches into API handlers and workers. The implementation should first introduce a provider dispatch layer owned by `cloudprint`, then plug Feieyun, Yilianyun, and Shangpeng into that layer.

The stable internal model should be:

- `cloud_printers.printer_type` chooses the provider.
- `cloud_printers.printer_sn` stores the provider device identity.
- `cloud_printers.printer_key` stores the provider device secret when the provider needs one.
- `print_logs.vendor_order_id` stores the provider print job/order id.
- `print_logs.provider_origin_id` should be added for provider idempotency and callback lookup.
- API, worker, scheduler, and UI consume normalized provider capabilities and statuses, not raw provider enums.

Compatibility guardrails:

- Phase 1 must be Feieyun-only at runtime and must preserve existing Feieyun request shape, callback route, callback lookup, and status-query behavior.
- Existing `print_logs` rows without `provider_origin_id` must remain readable and updatable by the existing `vendor_order_id` path.
- New provider state transitions must be persisted through one provider-neutral helper so callback, polling, manual retry, and status-query paths do not create competing writers for `print_logs.status`.

## 2. Official Source Set

### Yilianyun

Official docs used for the Yilianyun assessment:

- Platform overview: https://www.kancloud.cn/elind-dev/openapi/331992
- API protocol and request signing: https://www.kancloud.cn/elind-dev/openapi/332000
- Open application service mode: https://www.kancloud.cn/elind-dev/openapi/371769
- Self-owned application service mode, contrast only and not the LocalLife rollout target: https://www.kancloud.cn/elind-dev/openapi/371770
- Text print: https://www.kancloud.cn/elind-dev/openapi/372519
- Delete printer authorization: https://www.kancloud.cn/elind-dev/openapi/372520
- Printer model and print width: https://www.kancloud.cn/elind-dev/openapi/372524
- Set push URL: https://www.kancloud.cn/elind-dev/openapi/736520
- Query print order status: https://www.kancloud.cn/elind-dev/openapi/736521
- Query terminal status: https://www.kancloud.cn/elind-dev/openapi/751625
- Callback signature: https://www.kancloud.cn/elind-dev/openapi/372533
- Print completion callback: https://www.kancloud.cn/elind-dev/openapi/372534

### Shangpeng

Official docs used for the Shangpeng assessment:

- Open platform docs: https://spyun.net/open/index.html
- API host observed in the official open-platform bundle: `https://open.spyun.net/v1/printer/...`

The official page is client-rendered, but the same official bundle exposes the documented endpoint list, request fields, response examples, signature algorithm, and receipt markup.

Contract rule from `.github/standards/backend/EXTERNAL_API_CONTRACT_STANDARDS.md`: official provider docs and provider-confirmed samples are truth. Do not infer provider fields from Feieyun, from adjacent providers, from frontend needs, or from old DTO names.

Before production code relies on a Yilianyun or Shangpeng endpoint, create a field matrix for every request, response, callback, and error-code structure used by that endpoint. The matrix must record provider spelling, nesting path, type and unit, requiredness, conditional rules, enum/error values, LocalLife parser or mapper owner, and the official source reference. Fixture tests should lock the same field names and enum values.

The first implementation must include `artifacts/cloud-printer-provider-contract-matrix-2026-06-05.md` in the same change set as provider client code. It is a release blocker, not an optional follow-up. Use this minimum table shape for each provider endpoint:

| Provider | Capability | Method/Path | Direction | Provider field/path | Type/unit | Requiredness | Enum/error values | Local owner | Source |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

`Direction` should be one of `request`, `response`, `callback`, or `error`. `Local owner` should name the Go DTO/parser/validator/error mapper that owns the field.

## 3. Current LocalLife Baseline

Current Feieyun integration lives in `locallife/cloudprint/feieyun.go`. The `cloudprint.Client` interface currently contains:

- `AddPrinter`
- `RemovePrinter`
- `Print`
- `PrintResultCallbackEnabled`
- `QueryOrderState`
- `QueryPrinterStatus`
- `GetPrinterInfo`

Important Feieyun coupling points:

- `locallife/api/server.go` constructs exactly one `cloudprint.NewFeieyunClientFromConfig(config)`.
- `locallife/worker/processor.go` does the same for async order printing.
- `locallife/api/device.go` has `const printerTypeFeieyun = "feieyun"` and create request validation currently allows only `printer_type=feieyun`.
- `locallife/worker/task_print_order.go` filters active printers to Feieyun only.
- `locallife/api/feieyun_callback.go` handles only `/v1/webhooks/feieyun/print-result`.
- `locallife/db/migration/000010_add_orders.up.sql` allows `feieyun`, `yilianyun`, and `other`, but not `shangpeng` / `spyun`.
- `print_logs.vendor_order_id` exists and is suitable for provider print order ids, but its lookup is not provider-scoped.
- `print_logs.task_key` exists for local task re-entry idempotency, but it is not a provider idempotency id because it can contain punctuation and can exceed provider constraints.
- `print_logs.status` currently allows only `pending`, `success`, and `failed`; adding a normalized `cancelled` state requires an explicit schema migration, API filter update, and frontend/status-summary propagation.

## 4. Provider Contract Matrix

| Capability | Provider-neutral need | Feieyun baseline | Yilianyun | Shangpeng |
| --- | --- | --- | --- | --- |
| Local provider key | `printer_type` value | `feieyun` | `yilianyun` | `shangpeng` |
| Device identity | Stable `printer_sn` equivalent | `sn` | `machine_code` | `sn` |
| Device secret | `printer_key` equivalent | device key | not used by the open-app authorization flow; do not store access tokens in `printer_key` | `pkey` for add, `key` in getModel |
| App credential | Platform-level credential | `user`, `ukey` | backend shows customer/user ID, app ID, app secret; API sends app ID as `client_id` and signs with app secret as `client_secret` | `appid`, `appsecret` |
| Access token | Token lifetime and storage owner | none | open-app per-printer token from merchant `code` exchange; encrypted DB storage owned by authorization flow | none documented |
| Bind device | Remote registration endpoint | `/Api/Open/printerAddlist` | open-app authorization-code flow or open-app scan-code flow; self-owned `/printer/addprinter` is not the LocalLife default | `POST /v1/printer/add` |
| Unbind device | Remote deregistration endpoint | `/Api/Open/printerDelList` | `POST /printer/deleteprinter` | `DELETE /v1/printer/delete` |
| Text receipt print | Endpoint and content format | `/Api/Open/printMsg` | `POST /print/index` | `POST /v1/printer/print` |
| Print idempotency | Provider duplicate guard | no explicit local origin id | `origin_id`, <= 32 chars, alnum only, unique per `client_id`; `idempotence=1` | no native idempotency field documented; use local idempotency and provider status query |
| Provider job id | Store in `print_logs.vendor_order_id` | response `data` string | response `body.id` | response `id` |
| Provider origin id | Store in `print_logs.provider_origin_id` | ignored | required for idempotent print and callback lookup | optional local-only value |
| Print status query | Normalize to pending/success/failed/cancelled | `queryOrderState` bool | `0` unprinted, `1` printed, `2` cancelled | `status` bool plus `print_time` from `/order/status` |
| Printer status query | Normalize online/working/paper | `queryPrinterStatus` string plus `printerInfo` | `0` offline, `1` online, `2` out of paper | `/info`: `online` 1/0, `status` 0 normal / 1 abnormal, `sqsnum` |
| Print callback | Payload and ACK | configured `backurl`, RSA signature, status success callback | `oauth_finish`, `state=1/2`, ACK `{"data":"OK"}` | no receipt print callback found in official docs; only scan callback documented |
| Callback signature | Verification and replay | RSA signature over form fields | `md5(client_id + push_time + client_secret)` | not applicable for receipt print callback unless provider confirms one |
| Push URL setup | Provider-side callback registration | print request `backurl` | `POST /oauth/setpushurl`, GET health check returns `{"data":"OK"}` | no print push URL found |
| Error model | Stable mapped local errors | `ret != 0`, `msg` | `error != "0"` with numeric provider error codes | `errorcode`, global `-1..-4`, endpoint-specific positive codes |

## 5. Provider-Specific Contract Notes

### Yilianyun

Yilianyun must use open application service mode for LocalLife. Do not implement the self-owned application mode unless the application type is changed by product/ops and this document is revised first.

Official terminology and credential mapping:

- Official docs distinguish self-owned application service mode and open application service mode. Third-party service providers use open application service mode, which can authorize multiple merchants.
- Yilianyun backend may display three values such as customer/user ID, app ID, and app secret. Official API docs name the runtime pair `client_id` and `client_secret`: app ID maps to API `client_id`, and app secret maps to the signing secret `client_secret`.
- Customer/user ID is an operator-facing developer/account identifier. Do not put it into API signatures or provider request bodies unless an official endpoint or provider support explicitly confirms that field.
- Local config should use `YILIANYUN_APP_ID` and `YILIANYUN_APP_SECRET` as the primary names, with `YILIANYUN_CLIENT_ID` and `YILIANYUN_CLIENT_SECRET` kept only as compatibility aliases because the provider API calls them `client_id` and `client_secret`.

Open-app authorization-code flow:

- Merchant/operator starts Yilianyun authorization with `GET /oauth/authorize`, `response_type=code`, `client_id`, URL-encoded `redirect_uri`, and an unguessable `state`.
- Yilianyun redirects to `YILIANYUN_AUTH_CALLBACK_URL` with `code` and `state`. The code is valid for 600 seconds, so the callback handler must exchange it immediately.
- The backend exchanges `code` through `POST /oauth/oauth` with `grant_type=authorization_code`, `scope=all`, request signature, timestamp, and UUID request id.
- The response returns `access_token`, `refresh_token`, `machine_code`, and `expires_in`; official docs state that every printer has a unique `access_token`.
- Store access/refresh tokens encrypted in a DB-backed authorization table scoped to merchant/store/printer/provider. Do not store tokens in `cloud_printers.printer_key`, and do not use global `YILIANYUN_ACCESS_TOKEN` / `YILIANYUN_REFRESH_TOKEN` config as runtime credentials.
- Refresh uses `grant_type=refresh_token` and is limited to 20 times per day per terminal, so refresh must be scheduler/command owned with a durable lock and must not run in constructors, startup loops, or every print call.

Open-app scan-code / rapid authorization flow:

- Official docs state scan-code rapid authorization only supports open application service mode.
- The Mini Program can scan the printer body QR code or self-test receipt QR code to obtain `machine_code` plus `qr_key` or `msign`.
- The backend exchanges those values through `POST /oauth/scancodemodel` with `client_id`, `machine_code`, exactly one of `qr_key` or `msign`, `scope=all`, signature, timestamp, and UUID request id.
- The response returns `access_token`, `refresh_token`, and `expires_in`; store the token pair encrypted and scoped to the LocalLife printer authorization.
- Prefer scan-code authorization for Mini Program device binding when product/ops wants an in-app bind flow and the available printer QR material is reliable. Keep authorization-code redirect flow available for operator/web flows where redirect UX is acceptable.

Request protocol:

- HTTP POST, `application/x-www-form-urlencoded`.
- Authorized API calls use common fields `client_id`, `access_token`, `sign`, `timestamp`, `id` as UUID4. OAuth code exchange and refresh use `grant_type` and do not already have an `access_token`.
- Signature: `lower(md5(client_id + timestamp + client_secret))`.
- Domestic host: `https://open-api.10ss.net`.
- Overseas host: `https://open-api-os.10ss.net`. The official docs state that the interface method/path stays unchanged when switching host.
- Default LocalLife config should use the domestic host unless product/ops explicitly needs overseas routing.

Initial authorization recommendation:

- Require `YILIANYUN_APP_ID`, `YILIANYUN_APP_SECRET`, and `YILIANYUN_API_BASE_URL` before Yilianyun can be registered. `YILIANYUN_CUSTOMER_ID` may be stored for operator traceability but must not affect request signing unless a future official contract requires it.
- `YILIANYUN_AUTH_CALLBACK_URL` is required only for the authorization-code redirect flow. It is optional for scan-code / rapid authorization.
- Keep `YILIANYUN_CLIENT_ID` and `YILIANYUN_CLIENT_SECRET` as compatibility aliases for `YILIANYUN_APP_ID` and `YILIANYUN_APP_SECRET`.
- Keep `YILIANYUN_ACCESS_TOKEN` and `YILIANYUN_REFRESH_TOKEN` as temporary compatibility placeholders only. They are not required, and the open-app runtime must not read them as global provider credentials.
- Add the DB-backed authorization table, callback handler, CSRF `state` persistence, encrypted token storage, refresh ownership, and operator/merchant error states before enabling Yilianyun print runtime.
- When `YILIANYUN_ENABLED=true`, missing app ID, app secret, or API base URL disables only Yilianyun registration by default and emits a loud structured error. Feieyun and other enabled providers must still start. Add an explicit `CLOUD_PRINTER_FAIL_ON_PROVIDER_CONFIG_ERROR=true` override only if ops wants whole-service fail-fast behavior for provider config mistakes.
- Token-related provider errors should disable only the affected Yilianyun authorization/printer at runtime; Feieyun and Shangpeng must remain available.

### Shangpeng

Shangpeng request protocol:

- HTTP form requests, `application/x-www-form-urlencoded`.
- Common fields: `appid`, `timestamp`, `sign`.
- Signature:
  - Remove `sign`.
  - Sort non-empty params by ASCII key.
  - Join as `key=value&...`.
  - Append `&appsecret=<appsecret>`.
  - MD5 and uppercase.
- Official global errors include `0` success, `-1` appid empty, `-2` appid not found, `-3` timestamp empty, `-4` signature error; endpoint-specific positive error codes vary.

Core endpoints:

- Add device: `POST /v1/printer/add`, fields include `appid`, `timestamp`, `business`, `sn`, `pkey`, `name`, `sign`.
- Delete device: `DELETE /v1/printer/delete`, fields include `appid`, `timestamp`, `sn`, optional `business`, `sign`.
- Device info: `GET /v1/printer/info`, fields include `appid`, `timestamp`, `sn`, `sign`.
- Print receipt: `POST /v1/printer/print`, fields include `appid`, `timestamp`, `content`, `sn`, optional `times`, `sign`.
- Print order status: `GET /v1/printer/order/status`, fields include `appid`, `timestamp`, `id`, `sign`.
- Clear pending queue: `DELETE /v1/printer/cleansqs`, fields include `appid`, `timestamp`, `sn`, `sign`.
- Query device model: `POST /v1/printer/getModel`, fields include `appid`, `timestamp`, `sn`, `key`, `sign`.

Shangpeng receipt print semantics:

- `print` returns provider order `id` and `create_time`.
- Printed status is available through `/order/status` as `status` bool plus `print_time`.
- No official receipt print callback was found. Do not invent `/webhooks/shangpeng/print-result` unless provider confirms one.
- A submitted order is cached for 12 hours by the provider; unprinted orders can later print after the device reconnects.

Shangpeng receipt markup differs from current Feieyun-shaped content:

- Supported tags include `<BR>`, `<CUT>`, `<IMAGE>`, `<L1>`, `<L2>`, `<C>`, `<H>`, `<W>`, `<R>`, `<B>`, `<QRCODE>`, and barcode tags.
- Current LocalLife content uses tags such as `<CB>`, `<BOLD>`, and `<QR>`, which should not be sent to Shangpeng unchanged.

## 6. Proposed Unified Architecture

### Provider Manager

Introduce a provider registry or manager owned by `cloudprint`, not by API handlers or workers.

Suggested shape:

```go
type ProviderType string

const (
    ProviderFeieyun   ProviderType = "feieyun"
    ProviderYilianyun ProviderType = "yilianyun"
    ProviderShangpeng ProviderType = "shangpeng"
)

type Manager interface {
    Provider(providerType string) (Provider, bool)
    Supported(providerType string) bool
}

type Provider interface {
    Type() string
    Capabilities() Capabilities
    AddPrinter(ctx context.Context, input AddPrinterInput) error
    RemovePrinter(ctx context.Context, input RemovePrinterInput) error
    Print(ctx context.Context, input PrintInput) (PrintResult, error)
    QueryOrderState(ctx context.Context, input QueryOrderStateInput) (PrintState, error)
    QueryPrinterStatus(ctx context.Context, input QueryPrinterStatusInput) (PrinterStatus, error)
    GetPrinterInfo(ctx context.Context, input PrinterInfoInput) (PrinterInfo, error)
}

type Capabilities struct {
    RemoteBind         bool
    RemoteUnbind       bool
    PrintText          bool
    ProviderIdempotency bool
    PrintCallback      bool
    PrintStatusQuery   bool
    PrinterStatusQuery bool
    DeviceInfo         bool
}
```

Keep exact names flexible during implementation. The important boundary is:

- API and worker choose a provider by `printer.PrinterType`.
- Provider-specific request/response structs stay inside provider files.
- Business layers do not parse provider error strings or raw enums.
- Missing provider support returns a stable LocalLife error, not a generic provider panic or silent no-op.
- Unsupported capabilities must return a typed stable error such as `ErrUnsupportedCapability` before any remote call. Callers must check `Capabilities()` before invoking optional operations, and tests must cover unsupported bind, unbind, status-query, and device-info branches.
- If future providers support only a subset of operations, split smaller capability interfaces during implementation rather than forcing no-op methods into provider files.

### Provider-Neutral Types

Recommended internal fields:

- `PrintInput.PrinterSN`
- `PrintInput.PrinterKey`
- `PrintInput.Content`
- `PrintInput.Copies`
- `PrintInput.ProviderOriginID`
- `PrintInput.ExpiredAt`
- `PrintResult.ProviderOrderID`
- `PrintResult.ProviderOriginID`
- `PrintResult.AcceptedAt`
- `PrintState.Status`: `pending`, `success`, `failed`, `cancelled`
- `PrintState.PrintedAt`
- `PrinterStatus.ProviderStatus`
- `PrinterStatus.Online`
- `PrinterStatus.Working`
- `PrinterStatus.PaperStatus`

Provider origin id policy:

- Generate a provider-safe `provider_origin_id` after creating `print_logs`, for example `LLP` + uppercase base36 print log id.
- Store it in `print_logs.provider_origin_id`.
- Pass it to providers that support or require it.
- Ignore it for Feieyun and Shangpeng provider calls when unsupported, but keep it for local traceability.
- Keep the generated value globally unique in LocalLife. Prefer a partial unique index on `provider_origin_id` when not null; do not scope uniqueness only to `printer_id` because Yilianyun treats `origin_id` as unique per `client_id`, not per printer.
- If a future provider requires provider-scoped rather than globally unique origin ids, add a persisted provider column to `print_logs` and enforce uniqueness on `(provider_type, provider_origin_id)` instead of weakening the constraint.

### Receipt Rendering

Do not keep one Feieyun-formatted receipt string and send it to every provider. Introduce a provider-aware renderer:

- Feieyun renderer preserves the current output.
- Shangpeng renderer maps LocalLife receipt concepts to Shangpeng-supported tags.
- Yilianyun renderer should be implemented from its official content/format rules and tested with a real device.

The worker should build receipt domain data once, then render per provider. This avoids duplicating order/item/settlement loading while still respecting provider markup.

Renderer tests:

- Add golden-output tests for the current Feieyun receipt so the provider-manager refactor cannot change existing paper output accidentally.
- Add provider-specific renderer tests that reject unsupported tags, especially preventing Feieyun-only tags such as `<CB>`, `<BOLD>`, and `<QR>` from being sent to Shangpeng.
- Real-device probes remain required because golden strings cannot prove printer-side markup behavior.

### Startup Wiring

Replace:

- API `cloudprint.NewFeieyunClientFromConfig(config)`
- worker `cloudprint.NewFeieyunClientFromConfig(config)`

with:

- `cloudprint.NewManagerFromConfig(config)` that registers enabled providers.

This keeps the API server and worker processor symmetric.

### Callback And Polling

Keep provider-specific webhook routes only where the provider actually supports callbacks:

- existing `/v1/webhooks/feieyun/print-result`
- new `/v1/webhooks/yilianyun/print-result`
- no Shangpeng print-result route in the initial design because no receipt print callback was found

Share the post-verification state update helper:

- lookup by provider type plus `vendor_order_id`, or by provider type plus `provider_origin_id`.
- update `print_logs` idempotently.
- log unknown callbacks with provider, machine code/SN, provider order id, provider origin id, and callback state.

Yilianyun push URL registration:

- Register `YILIANYUN_PRINT_CALLBACK_URL` through `POST /oauth/setpushurl` before enabling Yilianyun printing in production.
- Add an operator-only command or admin-only startup check that can set and verify the push URL without printing a receipt.
- The provider health-check `GET /v1/webhooks/yilianyun/print-result` must return `{"data":"OK"}` and must not mutate state.
- If push URL registration fails, do not register Yilianyun as an enabled print provider. Emit a structured startup/operator error and keep Feieyun unaffected.

Webhook security:

- Verify provider signature before parsing state-changing fields or writing logs that include provider identifiers.
- Enforce a freshness window for Yilianyun `push_time`: reject callbacks more than 10 minutes away from server time, unless official docs require a different window and the contract matrix records it.
- First-release ACK policy: invalid signatures return HTTP 400 and never mutate state. Validly signed but stale, future-dated, unknown-order, or duplicate callbacks return the provider-required ACK only after logging `callback_rejected=true` or `callback_duplicate=true` and must not create or regress state. If official Yilianyun retry semantics require a different ACK policy, record that in the contract matrix and cover it with tests before changing this default.
- Duplicate valid callbacks must be idempotent: terminal `success`, `failed`, or `cancelled` rows stay terminal and log a duplicate-delivery event without changing `printed_at` or error details.
- Unknown valid callbacks must be logged with sanitized provider identifiers and must not create new `print_logs`.
- Tests must cover valid callback, duplicate valid callback, stale callback, future-dated callback, invalid signature, unknown order, and terminal-row replay.

For providers without print callbacks, add provider-status polling for recent pending print logs when the provider supports print status query. This is especially important for Shangpeng because the synchronous `print` response means "accepted by provider", not necessarily "paper printed".

Pending-print reconciliation requirements:

- Own the polling path in scheduler/worker, not in request handlers.
- Scan only `print_logs.status='pending'` rows whose provider supports `PrintStatusQuery`, whose `vendor_order_id` is present, and whose created time is within the provider retention window. For Shangpeng the first window should be at most 12 hours unless real-device evidence proves a longer safe horizon.
- Use a small provider-specific initial delay after print submission so a just-created provider order is not queried immediately.
- Use bounded batches, per-provider rate limits, request timeouts, and row-claiming or `FOR UPDATE SKIP LOCKED` semantics so multiple workers cannot poll the same log concurrently.
- Do not hold a database transaction open while calling provider APIs. Use short transactions to claim rows and persist claim metadata, commit, call the provider, then write the result back with a conditional update.
- Update `print_logs` idempotently with conditional writes such as `WHERE status='pending'`. Terminal provider states may move to `success`, `failed`, or `cancelled`; transient transport/provider timeout failures must stay `pending` while recording sanitized poll metadata.
- Define a terminal local timeout. When the provider retention window expires without proof of printing, mark the log `failed` with a stable LocalLife error message such as `provider_print_status_expired`.
- Emit structured logs and metrics for polled, terminal-success, terminal-failed, terminal-cancelled, expired, provider-error, and transport-error outcomes. Logs must not include provider secrets, access tokens, full printer keys, or raw provider payloads.

First-release polling defaults:

- `CLOUD_PRINTER_STATUS_POLL_INTERVAL`: 1 minute.
- `CLOUD_PRINTER_STATUS_POLL_BATCH_SIZE`: 50 rows per scheduler tick.
- `CLOUD_PRINTER_STATUS_POLL_INITIAL_DELAY`: 30 seconds after `print_logs.created_at`.
- `CLOUD_PRINTER_STATUS_POLL_MAX_AGE`: 12 hours for Shangpeng, 24 hours for any future callbackless provider unless its contract says otherwise.
- Per-provider status-query timeout: 5 seconds.
- Per-provider rate limit: start at 60 status queries per minute per provider process; lower it if real-provider probes or provider docs require stricter limits.
- Alert if expired pending logs exceed 5 in 15 minutes, provider status-query error rate exceeds 10% for 10 minutes, or any provider credential/auth error appears for 3 consecutive checks.

## 7. Data Model And Migration Notes

Likely schema additions:

- `print_logs.provider_origin_id TEXT`
- unique partial index on `provider_origin_id` when not null, assuming LocalLife-generated origin ids remain globally unique
- if provider-scoped origin ids are later required, add `print_logs.provider_type` and use a unique partial index on `(provider_type, provider_origin_id)`
- provider-aware lookup query joining `print_logs` to `cloud_printers` by provider type and `vendor_order_id`
- provider-aware lookup query by provider type and `provider_origin_id`
- migration to add `shangpeng` to `cloud_printers_printer_type_check`
- migration to extend `print_logs_status_check` with `cancelled`; do not enable Yilianyun status handling before this migration is present
- durable polling metadata or an equivalent persisted claim/lock design for scheduler-owned status reconciliation: suggested fields include `provider_status_checked_at`, `provider_status_check_attempts`, and sanitized `provider_status_last_error`
- partial index for pending provider-status scans, for example rows with `status='pending'` and `vendor_order_id IS NOT NULL`

Potential token table if automatic Yilianyun token refresh is implemented:

- provider type
- access token
- refresh token
- expires_at
- refreshed_at
- version / updated_at for optimistic concurrency

Recommendation for first release:

- Add `provider_origin_id` now because it supports Yilianyun idempotency and future callback lookup.
- Avoid token table initially; require configured Yilianyun token.
- Use `shangpeng` as the local Shangpeng provider key in schema, config, tests, and UI labels unless a pre-existing production ops convention requires a separate alias migration.
- Add the `cancelled` status in the same migration slice that first stores normalized provider status, before Yilianyun status `2` can reach production writes.

Config and secret safety:

- Config constructors must validate all required credentials when a provider is enabled.
- Disabled providers should not register in the manager and should return stable unsupported-provider errors if selected accidentally.
- Structured logs must mask `client_secret`, `access_token`, `refresh_token`, `appsecret`, `printer_key`, `pkey`, and provider callback signatures.
- Public API responses and frontend copy must use stable LocalLife messages and must not expose raw provider errors or credentials.
- Probe commands must require explicit test printer identifiers and must default to non-production config.
- `CLOUD_PRINTER_FAIL_ON_PROVIDER_CONFIG_ERROR` defaults to `false` in production so provider config mistakes isolate the provider instead of taking down the API/worker process. Tests must cover both default isolate-provider behavior and opt-in whole-service fail-fast behavior.

Release and rollback gates:

- New providers must be disabled by default in production config. Enabling Yilianyun or Shangpeng requires explicit `*_ENABLED=true`.
- Device creation APIs may accept new provider types only when that provider is registered and enabled; otherwise return a stable unsupported-provider error.
- A provider-level disable must stop new binds and new print submissions for that provider while preserving existing Feieyun behavior and allowing already-created pending logs to be reconciled or manually failed.
- Rollback plan: disabling `YILIANYUN_ENABLED` or `SHANGPENG_ENABLED` must not require reverting schema migrations and must not block Feieyun print, Feieyun callback, Feieyun status query, or existing print-log reads.
- Migration rollback must be documented before merge. If the `cancelled` status migration is not safely reversible after rows exist, the release note must call it forward-only.
- Production enablement requires: contract matrix merged, provider tests passing, Feieyun regression passing, real-device evidence recorded, push URL registered for Yilianyun, metrics/alerts configured, and an ops runbook for credential rotation and provider disable.

Regeneration if SQL changes:

- `cd locallife && make sqlc`
- Update mocks if store interface changes.

## 8. Implementation Plan

### Phase 0: Contract Matrix And Test Fixtures

Files:

- Create `artifacts/cloud-printer-provider-contract-matrix-2026-06-05.md`.
- Create provider tests in `locallife/cloudprint/*_test.go`.

Tasks:

- Record field matrices for Yilianyun text print, add/delete printer, set push URL, query print order status, query terminal status, print callback, and provider error model.
- Record field matrices for Shangpeng add/delete/info/print/order-status/clean-queue/get-model endpoints and provider error model.
- Add Yilianyun signing, response parsing, status mapping, and callback signature tests.
- Add Shangpeng signing, response parsing, status mapping, and endpoint error-code tests.
- Add fixture-style tests that lock exact official field names, requiredness, enum values, malformed payload handling, unknown enum handling, and error-code mapping.
- Use `shangpeng` as the local provider key in fixtures.

Validation:

- `cd locallife && go test ./cloudprint`

### Phase 1: Provider Manager Without Behavior Change

Files:

- Modify `locallife/cloudprint/feieyun.go`
- Create `locallife/cloudprint/manager.go`
- Modify `locallife/api/server.go`
- Modify `locallife/worker/processor.go`
- Update affected tests in `locallife/api/*device*_test.go` and `locallife/worker/task_print_order_test.go`

Tasks:

- Introduce provider manager.
- Register Feieyun only.
- Keep public behavior unchanged: only Feieyun printers should work.
- Prove existing Feieyun unit tests still pass.
- Add regression coverage that Feieyun print request content, callback route, callback signature verification, callback lookup by existing `vendor_order_id`, anomaly retry, and order-state query behavior remain unchanged.

Validation:

- `cd locallife && go test ./cloudprint ./api ./worker`

### Phase 2: Shared Schema And Config

Files:

- Modify `locallife/db/migration/**`
- Modify `locallife/db/query/print_log.sql`
- Modify `locallife/util/config.go`
- Modify `locallife/util/config_test.go`

Tasks:

- Add `print_logs.provider_origin_id`.
- Add `cancelled` to `print_logs.status`.
- Add provider-aware print-log lookup queries.
- Add `shangpeng` to printer type constraint.
- Add polling metadata or equivalent durable scheduler ownership for provider-status reconciliation.
- Add config for Yilianyun and Shangpeng.
- Keep Feieyun config behavior unchanged.
- Add config tests for enabled-provider missing credential failures and disabled-provider non-registration.
- Add release/rollback tests proving provider disable rejects new binds and new print submissions for that provider, keeps existing pending logs eligible for reconciliation or manual failure, and leaves Feieyun print/callback/status-query paths usable.
- Add config tests for default isolate-provider behavior and opt-in `CLOUD_PRINTER_FAIL_ON_PROVIDER_CONFIG_ERROR=true` fail-fast behavior.
- Add secret masking tests for config/log helpers if the implementation introduces provider-specific logging helpers.

Config candidates:

- `YILIANYUN_ENABLED`
- `YILIANYUN_API_BASE_URL`
- `YILIANYUN_CUSTOMER_ID` for operator traceability only; do not include it in signatures or provider calls unless a future official contract requires it.
- `YILIANYUN_APP_ID`, primary config for the provider API `client_id`.
- `YILIANYUN_APP_SECRET`, primary config for the provider signing secret documented as `client_secret`.
- `YILIANYUN_CLIENT_ID` and `YILIANYUN_CLIENT_SECRET` remain compatibility aliases for older rollout drafts and tests.
- `YILIANYUN_HTTP_TIMEOUT`
- `YILIANYUN_AUTH_CALLBACK_URL` is required only for authorization-code redirect flow, not for scan-code / rapid authorization.
- `YILIANYUN_PRINT_CALLBACK_URL`
- `YILIANYUN_ACCESS_TOKEN` and `YILIANYUN_REFRESH_TOKEN` may remain as compatibility placeholders during rollout, but open-app runtime code must not treat them as global credentials.
- `SHANGPENG_ENABLED`
- `SHANGPENG_API_BASE_URL`
- `SHANGPENG_APPID`
- `SHANGPENG_APPSECRET`
- `SHANGPENG_HTTP_TIMEOUT`
- `CLOUD_PRINTER_STATUS_POLL_INTERVAL`
- `CLOUD_PRINTER_STATUS_POLL_BATCH_SIZE`
- `CLOUD_PRINTER_STATUS_POLL_INITIAL_DELAY`
- `CLOUD_PRINTER_STATUS_POLL_MAX_AGE`

Validation:

- `cd locallife && make sqlc`
- `cd locallife && go test ./util ./cloudprint`

### Phase 3A: Shangpeng Provider Client

Files:

- Create `locallife/cloudprint/shangpeng.go`
- Create `locallife/cloudprint/shangpeng_test.go`

Tasks:

- Implement Shangpeng appid/appsecret signed client.
- Keep provider DTOs, request builders, response parsers, status mappers, and error classifiers inside provider files.
- Return normalized `PrintResult`, `PrintState`, `PrinterStatus`, and `PrinterInfo`.
- Return typed unsupported-capability errors before remote calls for unsupported optional operations.
- Classify missing required provider fields, malformed payloads, unknown enums, signature failures, timeout, and provider error codes as stable LocalLife errors with safe structured log context.

Validation:

- `cd locallife && go test ./cloudprint`

### Phase 3B: Yilianyun Open-App Authorization Foundation

Files:

- Add a DB migration and sqlc queries for Yilianyun authorization state and encrypted token storage.
- Create backend authorization start/callback handlers, for example under `locallife/api/yilianyun_auth.go`.
- Add token encryption/decryption tests using the existing LocalLife encryption helper.
- Add focused sqlc/store tests when the repository pattern for similar authorization state has examples.

Tasks:

- Create a durable authorization session with merchant/store/printer intent, provider type, nonce/state, expiry, and one-time consumption.
- For authorization-code flow, generate Yilianyun authorize URL with `response_type=code`, `client_id`, URL-encoded `redirect_uri=YILIANYUN_AUTH_CALLBACK_URL`, and opaque `state`.
- For authorization-code flow, handle the Yilianyun redirect callback by validating `state`, expiry, one-time use, tenant ownership, and provider.
- For authorization-code flow, exchange `code` for `access_token`, `refresh_token`, `machine_code`, and `expires_in` immediately; code is valid for 600 seconds.
- For scan-code / rapid authorization, accept Mini Program scanned `machine_code` plus exactly one of `qr_key` or `msign`, then call `POST /oauth/scancodemodel` to obtain the printer authorization token pair.
- Store tokens encrypted and scoped to the resulting printer/provider authorization. Do not write tokens to `cloud_printers.printer_key`.
- Design refresh as a durable scheduler or operator command with rate-limit protection. Do not refresh in constructors, startup, request handlers, or every print call.
- Surface stable merchant/operator errors for unbound, expired, refresh-failed, and provider-denied authorizations without leaking token values or raw provider payloads.

Validation:

- `cd locallife && make sqlc`
- `cd locallife && go test ./util ./api ./cloudprint`

### Phase 3C: Yilianyun Authorized Provider Client

Files:

- Create `locallife/cloudprint/yilianyun.go`
- Create `locallife/cloudprint/yilianyun_test.go`

Tasks:

- Implement the Yilianyun request signer and authorized API client using the per-printer access token loaded from the authorization store, not static config.
- Keep OAuth exchange/refresh DTOs separate from authorized print/status DTOs.
- Return normalized `PrintResult`, `PrintState`, `PrinterStatus`, and `PrinterInfo`.
- Return typed unsupported-capability errors before remote calls for unsupported optional operations.
- Classify missing required provider fields, malformed payloads, unknown enums, signature failures, timeout, and provider error codes as stable LocalLife errors with safe structured log context.

Validation:

- `cd locallife && go test ./cloudprint`

### Phase 4: Device CRUD Support

Files:

- Modify `locallife/api/device.go`
- Modify `locallife/api/device_test.go`
- Modify `locallife/api/device_reconciliation.go`
- Modify `locallife/api/device_reconciliation_test.go`

Tasks:

- Allow `printer_type` values supported by provider manager.
- For Yilianyun, device identity comes from the open-app authorization result `machine_code`; do not map `printer_key` to `msign` or require a device secret in the merchant setup flow.
- For Shangpeng, map `printer_sn` to `sn`, `printer_key` to `pkey`.
- Call provider manager on create/delete.
- Preserve reconciliation job behavior for remote-local drift and record the actual provider type.
- Update Swagger if annotations or accepted enum docs change.

Validation:

- `cd locallife && go test ./api`
- `cd locallife && make swagger` if annotations change.

### Phase 5: Provider-Aware Receipt Rendering And Worker Print

Files:

- Modify `locallife/worker/task_print_order.go`
- Modify `locallife/worker/task_print_order_test.go`
- Consider adding `locallife/cloudprint/receipt_renderer.go`

Tasks:

- Replace Feieyun-only eligibility check with provider-manager support check.
- Create `print_logs` before provider print as today.
- Generate and store provider-safe `provider_origin_id`.
- Render content per provider.
- Store provider order id as `vendor_order_id`.
- Keep callback-enabled providers pending until callback.
- For callbackless providers with status query, keep accepted jobs pending and let status query or polling mark success.
- Avoid treating provider acceptance as terminal printed success unless the provider contract explicitly says it is terminal.
- Preserve Feieyun receipt output through golden tests before enabling provider-aware rendering.
- Add tests proving Shangpeng rendering does not emit unsupported Feieyun tags.

Validation:

- `cd locallife && go test ./worker`

### Phase 6: Callbacks And Pending-Print Reconciliation

Files:

- Keep `locallife/api/feieyun_callback.go`
- Create `locallife/api/yilianyun_callback.go`
- Create `locallife/api/yilianyun_callback_test.go`
- Modify `locallife/api/server.go`
- Add scheduler/worker support for callbackless provider status polling.
- Add or modify SQL queries for pending status polling and conditional terminal updates.

Tasks:

- Add GET `/v1/webhooks/yilianyun/print-result` returning `{"data":"OK"}` for provider health check.
- Add an operator-only command or startup/admin path to register and verify Yilianyun `YILIANYUN_PRINT_CALLBACK_URL` through `POST /oauth/setpushurl`.
- Add Yilianyun POST callback handler.
- Verify callback signature using `md5(client_id + push_time + client_secret)`.
- Enforce callback freshness using `push_time` and the configured 10-minute default window.
- Parse `order_id`, `origin_id`, `machine_code`, and `state`.
- On `state=1`, mark matching print log success.
- On `state=2`, mark matching print log `cancelled`; do not enable Yilianyun callbacks before the schema supports this state.
- Add provider-aware helper for callback status updates.
- For Shangpeng, poll `/v1/printer/order/status` for recent pending jobs instead of adding an unsupported callback endpoint.
- Implement polling with bounded batch size, worker-safe short-transaction row claiming, provider retention cutoff, provider-specific rate limit, sanitized poll metadata, provider calls outside DB transactions, and conditional `status='pending'` updates.
- Mark expired callbackless pending logs failed with a stable LocalLife error after the retention window passes without printed evidence.
- Test duplicate callback delivery, unknown callback order ids, signature failures, repeated polling of the same pending log, terminal success, terminal failed/cancelled, expired pending, provider timeout, and unsupported status-query branches.
- Test stale and future-dated Yilianyun callbacks, provider health-check GET, and push URL registration failure keeping Yilianyun disabled.

Validation:

- `cd locallife && go test ./api ./worker ./scheduler`

### Phase 7: Merchant API Surfaces

Files:

- Modify `locallife/api/order.go`
- Modify `locallife/api/order_test.go`
- Modify `locallife/api/device.go`
- Update `weapp/miniprogram/pages/merchant/printers/**`
- Update `web/src/types/merchant-settings.ts` and any web printer settings surfaces that are still active.

Tasks:

- Status query endpoint routes by provider type.
- Print anomaly retry allows any provider that supports print.
- Device live status normalizes provider status for Feieyun, Yilianyun, and Shangpeng.
- User-facing messages say cloud printer/provider unavailable, not Feieyun, when not provider-specific.
- Merchant UI exposes provider selection only where needed for setup; daily surfaces should prefer human labels and normalized status.
- Include `cancelled` in API filters, response DTOs, summaries, and frontend status displays.
- Make credential/provider configuration failures visible as stable business-readable guidance, not raw provider errors.

Validation:

- `cd locallife && go test ./api ./worker`
- frontend validation from each changed frontend project.

### Phase 8: Real Device Probes

Files:

- Optional `locallife/cmd/yilianyun_probe/main.go`
- Optional `locallife/cmd/shangpeng_probe/main.go`
- Add evidence notes under `artifacts/` for real-device observations.

Tasks:

- Bind one test printer per provider.
- Print a small receipt.
- Confirm receipt formatting.
- Confirm callback or polling updates `print_logs`.
- Query printer live status.
- Retry a failed/pending print job.
- Unbind only test devices.

Evidence note must include:

- provider account mode and endpoint host
- device SN or redacted identifier
- request timestamp and sanitized provider order id
- observed callback or polling timing
- receipt markup used and paper-output observation
- provider status response mapping
- any provider doc mismatch or undocumented behavior

Safety:

- Probe commands must never print to production merchant devices by default.
- Mask credentials and device keys in logs.
- Probe commands must require explicit test printer SNs and refuse production merchant devices unless an operator-only override is deliberately added with audit logging.

## 9. Recommended First Implementation Slice

Build in this sequence:

1. Provider manager with Feieyun only, no behavior change.
2. Shared schema/config for provider origin id, `cancelled` status, polling metadata, and `shangpeng` provider type.
3. Shangpeng client with contract tests.
4. Shangpeng device create/delete support.
5. Provider-aware receipt rendering and worker print path for Feieyun plus Shangpeng.
6. Shangpeng pending-job status polling with scheduler-owned reconciliation.
7. Yilianyun open-app authorization foundation, including auth callback, encrypted token storage, and refresh ownership.
8. Yilianyun authorized provider client and device/print support using stored per-printer tokens.
9. Yilianyun print callback and generic callback update helper.
10. Merchant device status, anomaly retry, and frontend provider labels.
11. Real-device probe and evidence.

Do not start by adding `if printer.PrinterType == "yilianyun"` and `if printer.PrinterType == "shangpeng"` next to every existing Feieyun branch. That will make each future provider pay the same integration tax again.

## 10. Release Decisions And Open Questions

First-release decisions:

- Local Shangpeng provider key: `shangpeng`.
- Callbackless provider acceptance must not mark local print success. Use `pending` plus status query or scheduler polling unless product explicitly accepts weaker observability in a separate decision record.
- Yilianyun application type: open application service mode. Token lifecycle must be DB-backed per printer authorization; no global config token runtime and no automatic refresh in constructors or print calls.
- Yilianyun push URL: register and verify through an operator-only command or admin-only startup check before enabling Yilianyun printing.
- Merchant daily surfaces should show normalized human labels and statuses. Raw provider names are shown only during setup, troubleshooting, and operator diagnostics.
- Provider probes should start as local/operator commands, not public merchant APIs.
- First-release polling and alert thresholds are the defaults listed in "Callback And Polling" and may be tightened after real-device evidence.

Remaining open questions:

1. What exact Yilianyun receipt markup subset should the first renderer support? Until answered, Phase 5 must use a conservative text-only receipt with line breaks, alignment, and cut commands verified by the field matrix and real-device probe.
2. Should automatic DB-backed Yilianyun token refresh be added after first production use, and who owns the refresh runbook?
3. Should future provider health/probe commands become an operator API after access control and audit requirements are defined?

## 11. Validation Plan

Minimum local validation after implementation:

- `cd locallife && go test ./cloudprint`
- `cd locallife && go test ./util`
- `cd locallife && go test ./api`
- `cd locallife && go test ./worker`
- `cd locallife && go test ./scheduler`
- `cd locallife && make sqlc` if SQL/query/schema changes
- `cd locallife && make swagger` if route annotations or public API docs change
- `cd locallife && make check-generated` after SQL or Swagger generation
- `artifacts/cloud-printer-provider-contract-matrix-2026-06-05.md` must exist and cover every provider endpoint used by code.
- Feieyun regression tests must cover no-behavior-change manager wiring, callback handling, anomaly retry, and receipt golden output before enabling new providers.
- Provider contract tests must cover official field names, requiredness, unknown enums, malformed payloads, provider error codes, timeout mapping, and unsupported capabilities.
- Yilianyun callback tests must cover signature, freshness, duplicate delivery, unknown order, invalid signature, stale callback, future-dated callback, health-check GET, and push URL registration failure.
- Pending-print polling tests must cover row claiming/idempotency, provider retention expiry, duplicate scheduler runs, default batch/rate-limit behavior, and sanitized failure metadata.
- Release-gate validation must prove disabling Yilianyun or Shangpeng leaves Feieyun print, callback, status query, and existing print-log reads working.

Real-provider validation required before production enablement:

- bind test printer
- print test receipt
- register and verify Yilianyun push URL before Yilianyun enablement
- verify receipt markup on paper
- receive print callback or poll print status
- query print order status
- query terminal status
- retry failed/pending print job
- unbind test printer

Residual risk if real-device validation is skipped: API field tests can prove request shape and local state transitions, but cannot prove that Yilianyun or Shangpeng accepts our credentials, receipt markup, callback URL, authorization state, or status timing in production.
