# Cloud Printer Multi-Provider Unified Implementation Plan

**Goal:** implement Shangpeng and Yilianyun cloud-printer support under one provider-neutral LocalLife design, without regressing the existing Feieyun production path.

**Source Contract:** [cloud-printer-provider-contract-matrix-2026-06-05.md](cloud-printer-provider-contract-matrix-2026-06-05.md)

**Risk Class:** G2. The work touches external provider APIs, merchant device binding, async printing, polling/callback recovery, retry/idempotency, device status, and merchant-visible order fulfillment observability.

**Status:** implementation planning only. No new code should be added until each phase starts with contract tests.

---

## 1. Design Baseline

LocalLife must treat cloud printers as a provider-neutral capability:

- `cloud_printers.printer_type` selects the provider.
- `cloud_printers.printer_sn` stores provider device identity.
- `cloud_printers.printer_key` stores provider device secret only when the provider needs one.
- Yilianyun open-app access/refresh tokens are stored only in encrypted authorization state, never in `printer_key` or env files.
- `print_logs.vendor_order_id` stores provider print order id.
- `print_logs.provider_origin_id` stores the LocalLife-generated provider idempotency/correlation id where needed.
- API, worker, scheduler, Mini Program, and future web surfaces consume normalized provider capabilities and statuses, not raw provider enums.

Do not add provider support by scattering `if printer_type == ...` branches through API handlers and workers. The provider boundary belongs in `locallife/cloudprint`, with callers selecting by provider type and checking capabilities.

## 2. Non-Negotiable Contract Decisions

1. Feieyun is the compatibility baseline. Provider-manager refactoring must prove Feieyun print, callback, device status, anomaly retry, and receipt output remain unchanged.
2. Shangpeng supports remote add through `POST /v1/printer/add`. Merchant/operator manual pre-add in Shangpeng backend must not be required for normal binding.
3. Shangpeng receipt printers must send provider `business=1`. This field is not a LocalLife merchant id.
4. Yilianyun current target is open application mode. Binding is successful authorization, not self-owned `/printer/addprinter`.
5. Yilianyun scan-code authorization requires `machine_code` plus exactly one of `qr_key` or `msign`. The first-release manual Mini Program flow uses two printed/device values: `机器码/终端号(machine_code)` and `终端密钥(msign)`. OAuth authorization-code `code` is a redirect callback value, not a merchant-entered printer value. A single scanned QR string may be parsed only after real QR samples are captured and documented.
6. Yilianyun authorization-code and scan-code flows both return per-printer token state. Runtime printing must load encrypted per-printer access token state from DB.
7. Do not configure `YILIANYUN_ACCESS_TOKEN` or `YILIANYUN_REFRESH_TOKEN`. Do not add `YILIANYUN_CLIENT_ID` / `YILIANYUN_CLIENT_SECRET`; use `YILIANYUN_APP_ID` and `YILIANYUN_APP_SECRET`.
8. Shangpeng has no confirmed receipt-print callback. First release uses polling for Shangpeng print completion.
9. Yilianyun print callback is optional until callback signature/ACK semantics are confirmed against the current docs/provider support. Polling must be sufficient for first safe release.
10. Provider-specific receipt content must be rendered per provider. Current Feieyun markup must not be sent unchanged to Shangpeng.

## 3. Current Code Risk To Reconcile Before Continuing

The repository already contains in-progress cloud-printer changes from earlier implementation work. Before adding more implementation, review those changes against the contract document and either keep, revise, or remove them deliberately.

Known areas that need contract reconciliation:

- Shangpeng `business` handling must be `1` for receipt printer add/delete.
- Yilianyun API base URL should be checked against the current docsify `/v2` host contract before runtime enablement.
- Yilianyun callback implementation must not be treated production-ready until current `oauth_finish` signature and ACK semantics are confirmed.
- Any Mini Program scan UX must not assume an opaque one-string bind without real QR sample parser evidence.
- Any runtime code that uses env-provided Yilianyun access/refresh tokens must be removed.

Do this reconciliation as a formal review checkpoint before the next code phase. Do not continue layering fixes onto code that conflicts with the contract.

---

## 4. Target Architecture

### Provider Manager

Add a provider manager owned by `cloudprint`:

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
    QueryPrintState(ctx context.Context, input QueryPrintStateInput) (PrintState, error)
    QueryPrinterStatus(ctx context.Context, input QueryPrinterStatusInput) (PrinterStatus, error)
    GetPrinterInfo(ctx context.Context, input PrinterInfoInput) (PrinterInfo, error)
}

type Capabilities struct {
    RemoteBind          bool
    RemoteUnbind        bool
    PrintText           bool
    ProviderIdempotency bool
    PrintCallback       bool
    PrintStatusQuery    bool
    PrinterStatusQuery  bool
    DeviceInfo          bool
}
```

Exact names can adapt to the codebase, but the boundary must remain:

- Provider DTOs and request builders stay provider-specific.
- Callers use normalized inputs/results.
- Unsupported capability returns a typed stable error before a remote call.
- Provider parse failures fail closed.

### Receipt Rendering

Introduce provider-aware rendering:

- Keep current Feieyun golden output unchanged.
- Render Shangpeng with its supported tags such as `<BR>`, `<CUT>`, `<C>`, `<B>`, `<QRCODE>`.
- Render Yilianyun from a conservative text/command subset verified by docs and real-device probes.
- Build receipt domain data once in worker code, then render per provider.

### Binding Flow

| Provider | Merchant input | Provider action | Local action |
| --- | --- | --- | --- |
| Feieyun | SN + key + name | add list API | create local printer row after remote add succeeds |
| Shangpeng | SN + pkey + name | `POST /v1/printer/add` with `business=1` | create local printer row after remote add succeeds |
| Yilianyun scan | `machine_code` plus `qr_key` or `msign`; optionally parsed from scan | `POST /oauth/scancodemodel` | store encrypted auth state and create/activate local printer row |
| Yilianyun OAuth code | redirect authorization | `GET /oauth/authorize` then `POST /oauth/oauth` | validate state, store encrypted auth state, create/activate local printer row |

### Print Completion

- Feieyun continues using existing callback behavior.
- Shangpeng first release uses scheduler polling via `/v1/printer/order/status`.
- Yilianyun first release can use polling via `/printer/getorderstatus`; callback is enabled only after current callback signature/ACK contract is confirmed and tested.

---

## 5. Data Model

Required or likely schema work:

- Add `shangpeng` to `cloud_printers.printer_type` constraints.
- Add `print_logs.provider_origin_id TEXT`.
- Add unique partial index on `provider_origin_id` when not null, unless a provider-scoped `(provider_type, provider_origin_id)` design is introduced.
- Add or confirm `print_logs.status` supports normalized `cancelled` before mapping Yilianyun status `2`.
- Add provider-aware print-log lookup by provider type + `vendor_order_id`.
- Add provider-aware print-log lookup by provider type + `provider_origin_id`.
- Add durable polling metadata or row-claiming fields for provider-status polling, for example checked-at, attempts, last sanitized error, and claim lease.
- Add Yilianyun authorization tables for encrypted token storage, authorization sessions, state validation, status, expiry, refresh metadata, and merchant/printer ownership.

Yilianyun authorization table must support:

- provider type
- merchant id
- optional local printer id
- `machine_code`
- app id used at authorization time
- encrypted `access_token`
- encrypted `refresh_token`
- access token expiry
- refresh token expiry
- authorization status
- last provider error summary
- optimistic concurrency or durable refresh locking

## 6. Config

Provider config should be explicit and disabled by default:

- `YILIANYUN_ENABLED`
- `YILIANYUN_API_BASE_URL`
- `YILIANYUN_CUSTOMER_ID` optional traceability only
- `YILIANYUN_APP_ID`
- `YILIANYUN_APP_SECRET`
- `YILIANYUN_HTTP_TIMEOUT`
- `YILIANYUN_AUTH_CALLBACK_URL` only required for authorization-code flow
- `YILIANYUN_PRINT_CALLBACK_URL` only required if callback is enabled
- `SHANGPENG_ENABLED`
- `SHANGPENG_API_BASE_URL`
- `SHANGPENG_APPID`
- `SHANGPENG_APPSECRET`
- `SHANGPENG_HTTP_TIMEOUT`
- `CLOUD_PRINTER_FAIL_ON_PROVIDER_CONFIG_ERROR`
- provider-status polling interval, batch size, initial delay, max age, timeout, and rate limit settings

Default behavior:

- Missing config for a disabled provider is fine.
- Missing config for an enabled provider disables that provider and logs structured operator-visible configuration failure by default.
- `CLOUD_PRINTER_FAIL_ON_PROVIDER_CONFIG_ERROR=true` may opt into whole-service fail-fast.
- Feieyun must continue to start and run if Yilianyun or Shangpeng config is invalid and fail-fast is disabled.

Secrets must be masked in logs and must never be returned to frontend clients.

---

## 7. Implementation Phases

### Phase 0: Contract Freeze And Existing Diff Review

**Files:**

- `artifacts/cloud-printer-provider-contract-matrix-2026-06-05.md`
- `artifacts/cloud-printer-multi-provider-yilianyun-design-plan-2026-06-05.md`
- all currently modified cloud-printer implementation files

**Tasks:**

1. Treat the contract matrix as source of truth.
2. Review current in-progress code against the contract.
3. List mismatches before editing code.
4. Fix or remove mismatches only through test-first tasks in later phases.
5. Confirm web remains frozen.

**Validation:**

- `git diff --check`
- documentation review only; no code validation expected in this phase.

### Phase 1: Provider Contract Tests

**Files:**

- `locallife/cloudprint/feieyun*_test.go`
- `locallife/cloudprint/shangpeng_test.go`
- `locallife/cloudprint/yilianyun*_test.go`

**Tasks:**

1. Add tests locking Shangpeng signing, `POST /v1/printer/add`, `business=1`, delete with `business=1`, print request, order-status request, info request, malformed response handling, and provider error handling.
2. Add tests locking Yilianyun signing, `/oauth/oauth`, `/oauth/scancodemodel`, exactly-one credential validation, `/print/index`, `/printer/getorderstatus`, `/printer/getprintstatus`, token response parsing, malformed response handling, and provider error handling.
3. Add Feieyun compatibility tests where missing, especially receipt output and callback lookup.

**Validation:**

- `cd locallife && go test ./cloudprint`

### Phase 2: Provider Manager With Feieyun Only

**Files:**

- `locallife/cloudprint/manager.go`
- `locallife/cloudprint/manager_test.go`
- `locallife/api/server.go`
- `locallife/worker/processor.go`
- focused API/worker tests

**Tasks:**

1. Introduce manager and provider capabilities.
2. Register Feieyun only.
3. Wire API and worker through manager without changing public behavior.
4. Keep old Feieyun client compatibility where needed until all callers migrate.

**Validation:**

- `cd locallife && go test ./cloudprint ./api ./worker`

### Phase 3: Schema And Config Foundation

**Files:**

- `locallife/db/migration/*`
- `locallife/db/query/*`
- generated sqlc and mocks
- `locallife/util/config.go`
- `locallife/app.env.example`
- config tests

**Tasks:**

1. Add provider type/schema support for `shangpeng`.
2. Add `provider_origin_id` and provider-aware lookup queries.
3. Add `cancelled` only if normalized Yilianyun cancelled status will be written.
4. Add polling metadata or durable claim design.
5. Add Yilianyun authorization/session/token storage tables and queries.
6. Add config fields and validation rules from this plan.
7. Ensure no Yilianyun access/refresh token env vars exist.

**Validation:**

- `cd locallife && make sqlc`
- `cd locallife && make mock` if store interface changes require it
- `cd locallife && go test ./util ./db/sqlc ./cloudprint`
- `cd locallife && make check-generated`

### Phase 4: Shangpeng Client And Device CRUD

**Files:**

- `locallife/cloudprint/shangpeng.go`
- `locallife/cloudprint/shangpeng_test.go`
- `locallife/api/device.go`
- `locallife/api/device_test.go`
- `locallife/api/device_reconciliation.go`
- `locallife/api/device_reconciliation_test.go`

**Tasks:**

1. Implement Shangpeng signed client strictly from the contract.
2. Add/remove printers through provider manager.
3. Always send `business=1` for receipt printer add/delete.
4. Create local printer row only after provider add succeeds.
5. Preserve reconciliation when remote/local state drifts.
6. Return stable unsupported-provider guidance when Shangpeng is not configured.

**Validation:**

- `cd locallife && go test ./cloudprint ./api`

### Phase 5: Receipt Rendering And Print Worker

**Files:**

- `locallife/worker/task_print_order.go`
- `locallife/worker/task_print_order_test.go`
- optional `locallife/cloudprint/receipt_renderer.go`
- receipt renderer tests

**Tasks:**

1. Preserve current Feieyun receipt golden output.
2. Add provider-aware receipt rendering.
3. Render Shangpeng without Feieyun-only tags.
4. Create `print_logs` before provider call.
5. Generate provider-safe `provider_origin_id`.
6. Store provider order id in `vendor_order_id`.
7. Treat callbackless provider print response as accepted/pending, not terminal success.

**Validation:**

- `cd locallife && go test ./cloudprint ./worker`

### Phase 6: Provider Status Polling

**Files:**

- scheduler/worker polling files
- print-log SQL queries
- API or worker tests for status update helpers

**Tasks:**

1. Poll pending provider print logs for providers with `PrintStatusQuery`.
2. Claim rows durably without holding DB transactions across provider calls.
3. Query Shangpeng `/v1/printer/order/status`.
4. Query Yilianyun `/printer/getorderstatus` when Yilianyun runtime is enabled.
5. Conditionally update only `pending` rows to terminal statuses.
6. Mark expired callbackless rows failed with a stable LocalLife error.
7. Record sanitized polling metadata.

**Validation:**

- `cd locallife && go test ./worker ./api`

### Phase 7: Yilianyun Authorization And Runtime

**Files:**

- `locallife/cloudprint/yilianyun*.go`
- `locallife/api/yilianyun_auth.go`
- `locallife/api/yilianyun_auth_test.go`
- authorization SQL/query/generated files
- Mini Program bind API types only after backend contract is stable

**Tasks:**

1. Implement authorization-code start/callback with state persistence and one-time validation.
2. Implement scan-code authorization with `machine_code` plus exactly one of `qr_key` or `msign`.
3. Store token pairs encrypted and scoped to merchant/printer/machine_code.
4. Add refresh ownership with durable locking and rate-limit protection.
5. Print through authorized client using stored per-printer access token.
6. Unbind/cancel authorization through `/printer/deleteprinter`.
7. Expose stable merchant/operator states for unbound, expired, refresh failed, and provider denied.

**Validation:**

- `cd locallife && make sqlc`
- `cd locallife && go test ./cloudprint ./api ./worker`
- `cd locallife && make check-generated`

### Phase 8: Mini Program Binding UX

**Files:**

- `weapp/miniprogram/api/table-device-management.ts`
- `weapp/miniprogram/pages/merchant/printers/**`

**Tasks:**

1. Put provider selection above provider-specific credential controls. The selected provider determines the next binding panel.
2. Keep Feieyun and Shangpeng SN/key binding simple and do not require manual provider-console pre-add for Shangpeng.
3. For Yilianyun first release, show two merchant input fields: `机器码` (`machine_code`) and `终端密钥` (`msign`). This matches the official `/oauth/scancodemodel` structured fields and the same two-value task shape as Feieyun/Shangpeng.
4. Keep backend support for `qr_key` as a future scan-assisted temporary-key path, but do not expose `qr_key` in the default merchant form until real QR samples and operations wording are confirmed.
5. If using `wx.scanCode` in a later phase, parse QR payload only after real QR samples are documented. Until then, do not auto-fill from scan output.
6. Do not expose `qr_key`, `msign`, raw QR payload, access tokens, refresh tokens, provider signatures, or raw provider errors in Mini Program copy, response rendering, ordinary logs, or toasts.
7. Show only masked Yilianyun `machine_code` and stable bind status after authorization.
8. Editing an existing Yilianyun printer must not expose or overwrite token state through the printer key field.

**Validation:**

- `cd weapp && PATH="$HOME/.local/bin:$PATH" npm run quality:check`

### Phase 9: Optional Yilianyun Callback

**Entry condition:** current Yilianyun `oauth_finish` signature, retry, and ACK contract has been confirmed and documented.

**Files:**

- `locallife/api/yilianyun_callback.go`
- `locallife/api/yilianyun_callback_test.go`
- route registration
- optional operator command for `POST /oauth/setpushurl`

**Tasks:**

1. Add GET health check returning `{"data":"OK"}`.
2. Add POST callback handler only after signature contract is confirmed.
3. Verify signature and freshness before state mutation.
4. Update print log idempotently by provider origin id or provider order id.
5. Keep polling as fallback.

**Validation:**

- `cd locallife && go test ./api ./worker`

### Phase 10: Real Device Probes And Release Gate

**Files:**

- optional probe commands
- evidence notes under `artifacts/`

**Tasks:**

1. Bind one Shangpeng test printer through API.
2. Print Shangpeng test receipt and verify paper output.
3. Poll Shangpeng print status and printer status.
4. Bind one Yilianyun test printer through the chosen authorization flow.
5. Print Yilianyun test receipt and verify paper output.
6. Poll Yilianyun print status and printer status.
7. Capture Yilianyun QR payload samples if scan UX is planned.
8. Unbind only test devices.
9. Record sanitized evidence.

**Validation:**

- local targeted tests from all changed packages
- real-device evidence note
- release checklist proving Feieyun still works when Yilianyun/Shangpeng are disabled

---

## 8. Recommended Execution Order

1. Freeze contract and review existing diff against it.
2. Contract tests.
3. Provider manager with Feieyun only.
4. Schema/config foundation.
5. Shangpeng backend bind/print/poll.
6. Provider-aware receipt rendering.
7. Yilianyun DB-backed authorization.
8. Yilianyun runtime print/poll.
9. Mini Program UX.
10. Optional Yilianyun callback only after provider callback contract confirmation.
11. Real-device evidence and release gate.

This order intentionally avoids starting with Mini Program forms. Backend provider contracts and persistence invariants must be stable first; otherwise the UI will encode the wrong provider model.

## 9. Review Checkpoints

After every phase:

- Review Feieyun regression risk.
- Review provider contract drift against the matrix.
- Review secret/token leakage.
- Review idempotency and retry behavior.
- Review local/remote reconciliation behavior.
- Run the smallest relevant tests before moving to the next phase.

Do not batch all review to the end. Provider integrations fail messily when small contract mistakes accumulate.

## 10. Open Blockers Before Production

1. Capture Yilianyun QR/self-test scan payload samples.
2. Confirm Yilianyun current `oauth_finish` callback signature and ACK semantics if callback is used.
3. Confirm deployment base URL choice for Yilianyun current `/v2` API host.
4. Confirm Shangpeng positive error-code semantics and retryability.
5. Verify Shangpeng receipt markup on a real or provider-confirmed test printer.
6. Verify Yilianyun receipt content on a real or provider-confirmed test printer.
7. Review all existing in-progress implementation files against the contract before continuing code work.

## 11. Validation Summary

Minimum local validation after full implementation:

- `cd locallife && go test ./cloudprint`
- `cd locallife && go test ./util`
- `cd locallife && go test ./api`
- `cd locallife && go test ./worker`
- `cd locallife && make sqlc` when SQL/query/schema changes
- `cd locallife && make mock` when store mock interfaces change
- `cd locallife && make swagger` when public API annotations/routes change
- `cd locallife && make check-generated`
- `cd weapp && PATH="$HOME/.local/bin:$PATH" npm run quality:check` when Mini Program changes
- `git diff --check`

Residual risk if real-device validation is skipped: local tests can prove request shape and state transitions, but cannot prove that Shangpeng/Yilianyun accepts our credentials, receipt markup, QR parsing, authorization state, callback URL, or provider status timing in production.
