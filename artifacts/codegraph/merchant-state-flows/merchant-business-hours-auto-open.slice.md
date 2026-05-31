# Merchant Business Hours Automatic Open Slice

Status: first merchant-state flow slice
Risk class: G2 - merchant availability is an automatic status transition, writes `merchants.is_open`, and affects ordering availability
Scope: merchant business-hours editor -> business-hour persistence -> automatic open/close scheduler -> merchant status websocket publish

## Variant Coverage

This slice covers:

- Merchant Mini Program business-hours editor.
- Weekly business-hour rows and special-date rows submitted through `PUT /v1/merchants/me/business-hours`.
- The `auto_open_by_business_hours` switch stored on `merchants`.
- Automatic open/close by `MerchantOpenStatusScheduler`.
- Websocket publish after scheduler-driven `is_open` changes.

This slice does not cover:

- Manual open/close through `PATCH /v1/merchants/me/status`; it is a neighboring flow because it writes the same `merchants.is_open` state.
- Customer-facing ordering behavior after `is_open` changes.
- Reservation/table time-slot generation beyond noting it reads business hours.
- Payment-config onboarding that determines whether auto-open can become `is_open=true`.

## Product Invariant

When a merchant saves business hours with automatic mode enabled, the backend durable state must be the source of truth. The scheduler may open the merchant only when all of these are true:

- merchant is active
- `merchants.auto_open_by_business_hours = true`
- current date/time is within at least one effective non-closed business-hour row
- required active payment config exists

The Mini Program may maintain local draft state, but saved state must be rehydrated from the backend response.

## Primary Forward Chain

1. Page access begins in the merchant business-hours page. `onLoad` checks merchant console access, then calls `loadBusinessHours`.
   Evidence: `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:198`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:202`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:213`.

2. `loadBusinessHours` calls `getMyMerchantBusinessHours`, normalizes `hours[]` into weekly/special draft state, stores `autoOpenByBusinessHours`, and clears `hasChanges`.
   Evidence: `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:255`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:276`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:277`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:282`.

3. Frontend API wrapper maps this to `GET /v1/merchants/me/business-hours` and declares response field `auto_open_by_business_hours`.
   Evidence: `weapp/miniprogram/api/merchant.ts:451`, `weapp/miniprogram/api/merchant.ts:583`.

4. The page keeps local draft changes for day closed/open toggles, time edits, slot add/remove, auto-open switch, and "sync first business day". The sync action only copies draft slots and does not call the backend.
   Evidence: `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:314`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:340`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:369`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:400`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:422`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:437`.

5. `onSave` validates local draft state, builds the payload, calls `updateMyMerchantBusinessHours`, then rehydrates local initial state from the backend response.
   Evidence: `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:517`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:525`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:531`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:536`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:541`.

6. Payload builder sends one row for each open slot and one closed row for each closed day. It includes `auto_open_by_business_hours`.
   Evidence: `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:134`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:147`, `weapp/miniprogram/pages/merchant/settings/business-hours/index.ts:158`.

7. Frontend API wrapper maps this to `PUT /v1/merchants/me/business-hours`.
   Evidence: `weapp/miniprogram/api/merchant.ts:468`, `weapp/miniprogram/api/merchant.ts:594`.

8. Backend routes register GET and PUT paths. The PUT path is protected by merchant profile write roles `owner` and `manager`.
   Evidence: `locallife/api/server.go:670`, `locallife/api/server.go:675`, `locallife/api/server.go:682`, `locallife/api/server.go:685`.

9. `setMerchantBusinessHours` resolves the merchant from the authenticated user, accepts multiple rows for the same day, parses time/date values, and calls `SetBusinessHoursTx`.
   Evidence: `locallife/api/merchant.go:773`, `locallife/api/merchant.go:784`, `locallife/api/merchant.go:797`, `locallife/api/merchant.go:801`, `locallife/api/merchant.go:821`.

10. `SetBusinessHoursTx` deletes all existing business-hour rows for the merchant, inserts every submitted row, then updates `merchants.auto_open_by_business_hours`.
    Evidence: `locallife/db/sqlc/tx_merchant.go:37`, `locallife/db/sqlc/tx_merchant.go:39`, `locallife/db/sqlc/tx_merchant.go:46`, `locallife/db/sqlc/tx_merchant.go:61`.

11. SQL stores business-hour rows in `merchant_business_hours` and stores automatic mode in `merchants.auto_open_by_business_hours`.
    Evidence: `locallife/db/query/merchant.sql:276`, `locallife/db/query/merchant.sql:293`, `locallife/db/query/merchant.sql:299`, `locallife/db/query/merchant.sql:341`.

12. `MerchantOpenStatusScheduler` runs every minute and calls `SyncMerchantOpenStatusByBusinessHours`.
    Evidence: `locallife/scheduler/merchant_open_status.go:14`, `locallife/scheduler/merchant_open_status.go:37`, `locallife/scheduler/merchant_open_status.go:61`.

13. Scheduler SQL determines effective rows for today, preferring special-date rows over weekly rows. It opens only if at least one non-closed effective row contains the current local time, then also requires active payment config before writing `is_open=true`.
    Evidence: `locallife/db/query/merchant.sql:499`, `locallife/db/query/merchant.sql:511`, `locallife/db/query/merchant.sql:518`, `locallife/db/query/merchant.sql:530`, `locallife/db/query/merchant.sql:536`, `locallife/db/query/merchant.sql:544`, `locallife/db/query/merchant.sql:548`, `locallife/db/query/merchant.sql:558`.

14. After a scheduler update, the scheduler reloads the merchant status and publishes a websocket merchant status change with source `business_hours`.
    Evidence: `locallife/scheduler/merchant_open_status.go:71`, `locallife/scheduler/merchant_open_status.go:83`.

## Reverse-Reference Findings

- `auto_open_by_business_hours` write path found in `SetBusinessHoursTx` through `UpdateMerchantAutoOpenByBusinessHours`; no separate frontend API writes this field.
- `merchants.is_open` has multiple writers: manual `UpdateMerchantIsOpen` via `PATCH /v1/merchants/me/status` and automatic `SyncMerchantOpenStatusByBusinessHours`.
- `ListMerchantBusinessHours` is read by reservation/table flows, so business-hour semantics affect more than this settings page.
- `GetBusinessHourByDayOfWeek` and `GetBusinessHourByDate` are single-row helpers. They can be wrong for multi-slot workflows if reused for editing or status calculation.

## SQL And Durable State Boundaries

- `merchant_business_hours`: durable source for weekly and special-date time windows.
- `merchants.auto_open_by_business_hours`: durable source for whether scheduler is allowed to control `is_open`.
- `merchants.is_open`: derived/current availability state, also writable by manual open/close flow.
- `merchants.auto_close_at`: cleared when automatic mode is enabled and when scheduler writes status.

Multi-slot support:

- The table has no unique constraint on `(merchant_id, day_of_week)` or `(merchant_id, special_date)`.
- PUT handler explicitly allows same-day multiple rows.
- Transaction inserts every submitted row.
- Scheduler uses `BOOL_OR` across effective rows, so any matching non-closed slot can open the merchant.

## Trust, Authorization, And Tenant Checks

- Frontend calls `ensureMerchantConsoleAccess` before loading the page.
- Backend resolves the current merchant from the authenticated user for GET and PUT.
- PUT is protected by merchant profile write staff middleware for `owner` and `manager`.
- Scheduler does not trust frontend state; it reads durable rows directly.

## Idempotency And Duplicate-Submit Checks

- Frontend blocks duplicate save with `saving`.
- Backend PUT is "replace all rows" in a transaction. Repeating the same request is effectively idempotent for business-hour rows, although row IDs are regenerated.
- There is no request idempotency key. Last successful PUT wins.
- Scheduler update is idempotent because it only updates rows where `m.is_open IS DISTINCT FROM desired`.

## Recovery And Async Convergence Paths

- Page silent refresh uses a freshness window and preserves current trusted data if refresh fails.
- Pull refresh is blocked while there are unsaved draft changes.
- Scheduler runs every minute and will converge `is_open` after saved hours or automatic mode changes.
- Websocket publish happens after scheduler writes and reloads the merchant status.

## Frontend Draft And Backend Rehydration

- Draft changes are local until `onSave`.
- The "sync" button copies the first configured business day into other days locally only.
- Save uses backend response to reset `weeklyHours`, `initialWeeklyHours`, `autoOpenByBusinessHours`, and `initialAutoOpenByBusinessHours`.
- If no changes exist, save navigates back without writing.

## Test Coverage Signals

Observed tests:

- `locallife/api/merchant_business_hours_test.go` checks GET includes `auto_open_by_business_hours`.
- `locallife/api/merchant_business_hours_test.go` checks PUT persists `auto_open_by_business_hours` into `SetBusinessHoursTx`.
- `locallife/scheduler/merchant_open_status_test.go` checks scheduler calls sync and publishes status changes.
- `locallife/api/security_authz_test.go` denies unauthorized business-hours PUT.

Missing high-value tests:

- DB or integration test proving `SyncMerchantOpenStatusByBusinessHours` opens during one of multiple same-day slots and closes between slots.
- Test for special-date precedence with multiple rows.
- Test for automatic mode plus manual state interaction, likely in the neighboring manual open/close flow.

## Gaps And Refactor Notes

- Single-row helper queries for day/date business hours are drift candidates under multi-slot semantics.
- Handler fallback comment for `ListMerchantBusinessHoursAll` appears stale because the query exists in sqlc.
- Special-date mixed rows may have coarse semantics: any `is_closed=true` effective row closes the entire day.
- Manual open/close and automatic scheduler share `is_open`; audit them together before changing overwrite behavior.

## Branch Exhaustion

- Entry branches checked: Mini Program business-hours page, weekly/special-date editor component, auto-open switch, first-day sync action, save/reload/pull-refresh, merchant status readers, reservation/table slot readers, and backend scheduler. Flutter App does not edit business hours; it only observes resulting `is_open` through the manual status flow. Web is intentionally out of scope.
- Request branches checked: `GET/PUT /v1/merchants/me/business-hours`, frontend wrappers, backend transaction replace-all, merchant status GET/PATCH adjacency, scheduler SQL, and reservation/table readers using business-hour lists.
- Backend state branches checked: weekly rows, special-date rows, closed rows, multiple same-day slots, replace-all transaction, `auto_open_by_business_hours`, derived `merchants.is_open`, `auto_close_at` clearing, payment-readiness gate during scheduler open, and websocket publish after scheduler change.
- Async branches checked: one-minute scheduler loop, post-update status reload, websocket merchant-status publish, dashboard/kitchen/App readers of status. There is no frontend async write after save besides normal re-entry/refresh.
- Failure/retry branches checked: duplicate save guard, silent refresh fallback, dirty draft pull-refresh block, last-write-wins PUT, scheduler idempotent distinct update, mixed special-date closed/open semantics, missing payment config blocking automatic open, and manual writer race.
- Reader/consumer branches checked: settings page, dashboard status, kitchen status, public merchant status, order validation, reservation/table time slots, and Flutter App working status via shared `is_open`.
- Authorization/tenant branches checked: merchant console access, backend current-merchant resolution, owner/manager profile-write middleware, and scheduler reading durable merchant rows without client input.
- Zombie/unreachable branches checked: single-row business-hour helpers are unsafe for multi-slot reuse; stale handler comment references missing query even though sqlc query exists; no App editing entry was found.
- Test-proof gaps checked: existing tests cover GET/PUT flag persistence and scheduler publish. Missing proof remains for multi-slot scheduler open/close, special-date precedence with multiple rows, mixed closed/open special-date semantics, and explicit manual-vs-automatic overwrite contract.
