# Merchant Order Detail Delivery Plan

Date: 2026-04-28
Branch: `main`
Risk class: `G3`
Status: Stage 5 complete; ready for human device verification

## 1. Problem Statement

Merchant-facing clients must show enough order detail for a merchant to accept, reject, prepare, and print an order reliably. This includes Android merchant App and the merchant side of the WeChat Mini Program. The current path is incomplete:

- Backend WebSocket `new_order` snapshots can include `items`, but clients do not currently consume one shared `new_order` envelope reliably.
- Native push payload and the shared `PushMessage` model do not carry order items.
- Merchant order list responses do not hydrate `items`, while the app list UI expects `order.items`.
- App order parsing expects `order_num`, `amount`, `note`, and item `price`; backend responses commonly use `order_no`, `total_amount`, `notes`, `unit_price`, and `subtotal`. Mini Program merchant order pages already align more closely with backend fields but previously rendered customization names from a drifted shape instead of the stable `specs_text` field.
- Order item specifications live in `order_items.customizations`; the app has no stable display field for规格/specification text.
- Receipt printing is driven from `PushMessage`, so it can only print order number and amount, not菜品、数量、规格、备注.

This is `G3` because it affects the new-order alert and acceptance path. A merchant seeing incomplete or wrong order information can miss preparation requirements, print incomplete tickets, or make a wrong accept/reject decision.

## 2. Target User Outcome

When a paid order reaches a merchant client through WebSocket, native push, or polling:

- The merchant sees order number, amount, order type, table or pickup context when present, and customer/user remarks.
- The merchant sees every order item with name, quantity, unit price, subtotal, and human-readable specifications. Product images are not part of the merchant order-detail contract; specifications are mandatory for merchant-facing order detail.
- Duplicate delivery across WebSocket, native push, and polling does not duplicate alerts, but still refreshes stale order detail.
- If a detail fetch temporarily fails, the app degrades visibly and retries; it must not silently show an empty商品清单 as if the order has no items.
- Printing uses the same hydrated order detail that the merchant sees on screen.

## 3. Source Of Truth And Contract Direction

Backend remains the source of truth for order state, money fields, item names, quantities, specifications, and user remarks. The merchant App and Mini Program merchant side must consume the same merchant `new_order` event data structure because the business use is identical.

Contract direction:

- Preserve existing backend field names for compatibility: `order_no`, `total_amount`, `notes`, item `unit_price`, item `subtotal`, item `customizations`.
- Add explicit merchant-client-friendly derived fields only where they remove ambiguity without breaking existing callers, especially item `specs_text`. `specs_text` must be present in merchant-facing item payloads, using an empty string only when the item truly has no specification/customization.
- Treat money values from backend as integer cents unless a field name explicitly says otherwise. The app display model converts cents to yuan for UI and printing.
- Treat `customizations.meta_specs` as the first source for human-readable规格 text. If absent, derive a readable string from structured customization entries where possible; otherwise keep the raw customization structure for future UI use but do not display JSON blobs to merchants.

## 4. Stage Gate Workflow

Each stage follows the existing backend/app quality gate:

1. Implement a focused patch set.
2. Run focused validation.
3. Review against backend and Flutter standards.
4. Fix all findings.
5. Re-run the relevant validation.
6. Sync docs and task-card status.
7. Move to the next stage only after the current stage is clean.

## 5. Stage 1: Backend Order Detail Contract Hardening

Goal: make the backend consistently provide complete merchant-facing order detail, including items, specifications, and notes.

Task cards:

- [x] Card 1.1 Add a shared order-item response mapper.
- [x] Card 1.2 Add batch item loading for merchant order lists.
- [x] Card 1.3 Update merchant order list responses to include `items` without N+1 queries.
- [x] Card 1.4 Extend new-order notification payload/snapshot to include `items`, `notes`, and `specs_text`.
- [x] Card 1.5 Review, regenerate, test, and sync docs.

Implementation shape:

- Keep handler transport mapping in `locallife/api/`; keep item/spec extraction helper small and local to order response code unless existing patterns suggest a better owner.
- Add a sqlc query such as `ListOrderItemsWithDishByOrderIDs` to load all visible order items for a merchant page in one query, ordered by `order_id, id`.
- Extend `orderItemResponse` and worker `orderItemSnapshot` with `specs_text` while retaining `customizations`.
- Use a single helper to decode `customizations` and derive `specs_text`, so list, detail, WebSocket snapshot, and future native push payload do not drift.
- If customization JSON is malformed, return a controlled server error for HTTP detail/list response; for async notification snapshot, fail the payment outbox dispatch before publishing `new_order` so the existing worker retry boundary recovers server-side instead of exposing manual sync to the merchant.

Acceptance criteria:

- `GET /v1/merchant/orders` returns each order with `items` populated for the current page.
- `GET /v1/merchant/orders/{id}` returns `items` with `name`, `quantity`, `unit_price`, `subtotal`, stable `specs_text`, and `customizations` when present. Product image fields are not emitted on merchant-facing order items.
- WebSocket `new_order` data includes `items` and `notes`; item loading and specs decoding must succeed before the event is published.
- No query uses `SELECT *`; no unbounded item loading is introduced.
- Merchant ownership remains server-side; no client-provided merchant identity is trusted.

Validation:

- `make sqlc` if a new query is added.
- `make swagger` if response annotations or docs are changed.
- `make check-generated` after generation.
- `go test ./api -run 'Test.*MerchantOrder|TestNotifyMerchantNewOrder'`
- `go test ./logic -run 'Test.*MerchantOrder|Test.*OrderQuery'`
- `go test ./worker -run 'TestNotifyMerchantNewOrder_PublishesMerchantAppPayload|Test.*OrderPayment'`
- SQL guard for changed query files.

Stage 1 closeout evidence:

- Shared backend item view derives `specs_text` from the real order-time source `order_items.customizations.meta_specs`, matching the Mini Program's group-to-spec selection flow.
- The backend new-order event identity is now shared as `merchant:new_order:{order_id}`, not App-only; Android App native Push can still reuse the same business payload as its provider data.
- Mini Program merchant order list/detail now consume the same `specs_text` item field and no longer rely on a drifted `customizations.option_name` shape.
- Product image fields are excluded from the merchant-facing order list/detail/new-order structure.
- Merchant order list now batch-loads items for the visible page through `ListOrderItemsWithDishByOrderIDs`, avoiding N+1 queries.
- WebSocket `new_order` snapshots include item `specs_text`; item loading or customization decoding failure keeps the payment outbox unpublished and schedules server-side retry instead of emitting an incomplete merchant payload.
- Validation passed: `make sqlc`, `make swagger`, `make check-generated`, focused tests for merchant list/detail, shared specs decoding, order query service, and new-order WebSocket payload; related test files passed with 150 tests.
- Stage review finding fixed: initial tests used a structured array shape, but real order creation stores normalized selection objects with `meta_specs`; tests now cover the real persisted shape.

## 6. Stage 2: App Order Contract Normalization

Goal: make the app parse backend truth correctly and stop relying on ambiguous push-only models for full order detail.

Task cards:

- [x] Card 2.1 Introduce or tighten a mapper for backend order JSON to `OrderModel`.
- [x] Card 2.2 Add `OrderItem.specsText`, `unitPrice`, `subtotal`, and `customizations` support.
- [x] Card 2.3 Normalize aliases for compatibility: `order_no/order_num`, `total_amount/amount`, `notes/note`, `unit_price/price`.
- [x] Card 2.4 Convert backend cent values to yuan display values in one place.
- [x] Card 2.5 Add unit tests for realistic backend payloads with items, specs, notes, and empty/degraded cases.

Implementation shape:

- Keep parsing and normalization outside widgets. Widgets should consume `OrderModel` only.
- Prefer a feature-layer repository or mapper under `merchant_app/lib/features/order/` or model-level factory helpers; do not let pages import `ApiClient` directly.
- `OrderModel` should preserve both display amounts and raw cent values if needed for printing or future calculations.
- `OrderItem` should expose a computed display line such as `规格：大份 / 少辣` without leaking raw JSON to UI.

Acceptance criteria:

- App correctly parses backend `order_no`, `total_amount`, `notes`, `unit_price`, `subtotal`, and item `specs_text`.
- Existing app-local or legacy payloads using `order_num`, `amount`, `note`, and `price` remain compatible during rollout.
- Empty `items` is represented as unknown/degraded only when the source indicates item load failure; a real empty list is not confused with a failed load.

Validation:

- `flutter test` for order JSON mapper/model tests.
- `flutter analyze`.

Stage 2 closeout evidence:

- `OrderModel.fromJson` now accepts backend truth fields `id/order_id`, `order_no`, `total_amount`, `notes`, `items_load_failed`, and item `unit_price`, `subtotal`, `specs_text`, while retaining legacy aliases for rollout compatibility.
- `OrderItem` preserves display yuan values plus raw cent values for later printing/native-push serialization work.
- `PushMessage.fromJson` now carries snapshot `items`, `specs_text`, notes, and item-load failure state; message IDs such as `merchant:new_order:{order_id}` remain intact.
- Stage review finding fixed: push payload `amount` is a backend cent field, while legacy order `amount` is still treated as yuan for compatibility.
- Validation passed: `flutter test test/order_alert_coordinator_test.dart` and `flutter analyze`.

## 7. Stage 3: Unified New-Order Intake And Hydration

Goal: make WebSocket, native push, and polling converge into one reliable order intake path. Android App and Mini Program merchant side should consume the same merchant `new_order` data contract; App additionally receives native Push as an offline delivery channel.

Task cards:

- [x] Card 3.1 Update Android App `WsClient` and Mini Program merchant WebSocket runtime to accept the current backend envelope: wrapped `NotificationPushMessage` with `message.type == "new_order"`, plus any legacy top-level notification shapes.
- [x] Card 3.2 Extend `PushMessage` or replace it with a lightweight `OrderAlertEvent` that carries `message_id`, `order_id`, optional snapshot, and source channel.
- [x] Card 3.3 Route every accepted message through one orchestrator that dedups by `message_id` and `order_id`.
- [x] Card 3.4 Hydrate order detail from `GET /v1/merchant/orders/{id}` before showing the full actionable order view when possible.
- [x] Card 3.5 On hydration failure, preserve the incoming snapshot and keep retry/recovery system-owned; visible degraded UI is completed in Stage 4 without a merchant manual sync action.

Implementation shape:

- WebSocket transports remain transport-only and should not own UI state.
- The order alert coordinator or order provider should own hydrate/retry behavior.
- Duplicate messages should not replay sound/full-screen alert, but should be allowed to refresh an existing order if the local detail is incomplete.
- Polling should merge with the same order store, not create a separate display path with different fields.

Acceptance criteria:

- `new_order` WebSocket messages trigger the same dedup + alert path as native push and polling on App, and the same order refresh/alert path on Mini Program merchant pages.
- Full-screen alert and order detail can show item name, quantity, specs, notes, and total.
- Repeated WebSocket/push/polling delivery for the same order does not duplicate sound or alert.
- A notification tap or cold-start path can reconstruct the order from `order_id` even if the original push payload only contained summary fields.

Validation:

- Flutter unit tests for WebSocket envelope parsing and dedup behavior.
- Flutter tests for order alert coordinator hydration success/failure.
- Mini Program validation for merchant WebSocket `new_order` parsing and order-list refresh behavior.
- Manual scenarios: foreground WS, duplicate WS, polling fallback, notification tap/cold start if local tooling supports it.

Stage 3 closeout evidence:

- Android App `WsClient` now accepts the backend `type: "new_order"` envelope, preserves `merchant:new_order:{order_id}` from the outer message ID when needed, and keeps legacy notification payloads compatible.
- `PushMessage` carries order items, notes, `specs_text`, and historical `items_load_failed` compatibility; detail hydration preserves the original message identity instead of replacing it with a polling ID.
- `OrderNotifier.fetchOrders` now accepts both legacy list payloads and backend `data.orders`; `fetchOrderDetail` updates the shared order store by `order_id`.
- `OrderAlertCoordinator` writes incoming snapshots into the order store and attempts detail hydration before local notification, auto-accept, printing, or full-screen alert.
- Mini Program merchant order list subscribes to shared `new_order`, dedups by `message_id`, and silently refreshes the current list while preserving existing rows during refresh failures.
- Validation passed: `flutter test test/ws_client_test.dart test/order_alert_coordinator_test.dart`, `flutter analyze`, targeted Mini Program ESLint for changed files, and changed-file diagnostics. `npm run compile` remains blocked by pre-existing `miniprogram_npm/tdesign-miniprogram` and qrcode typing errors outside this path.

## 8. Stage 4: Merchant UI And Printing Completion

Goal: ensure the merchant sees and prints the same complete order truth.

Task cards:

- [x] Card 4.1 Update order list cards to show item rows only when hydrated, and an explicit loading/degraded state when details are pending.
- [x] Card 4.2 Update order detail page to display item specs and user notes clearly in Chinese.
- [x] Card 4.3 Update order alert page to include compact item summary, specs, notes, amount, and order number without overcrowding.
- [x] Card 4.4 Update ESC/POS receipt generation to use hydrated `OrderModel`, printing item name, specs, quantity, unit price/subtotal, order notes, and total.
- [x] Card 4.5 Review UI against Flutter design standards and run validation.

Implementation shape:

- Keep page UI dense and operational, not explanatory. Use inline states near the item section instead of a large global notice.
- Preserve button single-flight behavior for accept/reject.
- Avoid hiding accept/reject behind a full failure state unless the order itself cannot be loaded; if only item details are retrying, show the order shell and make the detail state explicit.

Acceptance criteria:

- Merchant can see菜品名称、数量、规格、备注 before accepting or printing; product images are intentionally outside this flow.
- Receipt output includes the same item lines and notes.
- UI does not display raw JSON, backend field names, or English technical errors.

Validation:

- `flutter analyze`
- `flutter test`
- Widget tests where practical for item/spec/note rendering.
- Manual check on narrow Android viewport for text wrapping and no overlap.

Stage 4 closeout evidence:

- App order list cards now show item specs under item names, use line subtotal, display remarks, and only use `明细同步中` as historical incomplete-payload compatibility instead of treating an empty list as a real商品清单.
- App order detail now shows item specs, quantity, unit price, subtotal, notes, and the same degraded state when details are incomplete; the item section relies on automatic system hydration and does not expose a merchant manual sync action.
- Full-screen new-order alert now includes a compact item/spec/note summary when the hydrated event carries details, and uses automatic syncing wording for historical incomplete snapshots.
- ESC/POS receipt generation now prints item name, quantity, specs, unit price, subtotal, notes, and total from the hydrated `PushMessage`; accepted-order printing preserves `OrderModel` items and notes and refuses incomplete item details.
- `merchant_app/docs/software_manual.md` was updated for order detail, alert, and receipt behavior.
- Validation passed: `flutter test test/ws_client_test.dart test/order_alert_coordinator_test.dart test/esc_pos_utils_test.dart`, `flutter analyze`, and changed-file diagnostics.

## 9. Stage 5: End-To-End Review And Release Readiness

Goal: close the G3 reliability path with evidence.

Task cards:

- [x] Card 5.1 Backend review: contract completeness, no N+1 query, ownership checks, item-load failure behavior.
- [x] Card 5.2 App review: one intake path, dedup, hydration retry, no widget-layer networking, Chinese degraded states.
- [x] Card 5.3 Cross-channel scenario matrix.
- [x] Card 5.4 Docs sync and release notes.

Scenario matrix:

- Foreground WebSocket `new_order` with complete items/specs/notes.
- WebSocket duplicate followed by polling duplicate.
- WebSocket item snapshot missing but detail API succeeds.
- Detail API temporarily fails, then retry succeeds.
- Native push summary-only payload opens app and hydrates by `order_id`.
- Merchant accepts order after detail hydration; duplicate tap does not submit twice.
- Receipt printing after hydration includes items/specs/notes.

Validation closeout:

- Backend focused tests listed in Stage 1.
- `make check-generated` if generated artifacts changed.
- `flutter analyze`.
- `flutter test`.
- Manual Android device testing should be recorded. If Huawei/Xiaomi/OPPO/vivo native push cannot be tested in this pass, record it as residual risk, not as completed validation.

Stage 5 closeout evidence:

- Backend review: merchant list now batch-loads items once per page and detail/list/new-order all use the same `OrderItemView` specs mapper. Merchant HTTP responses exclude product images while user-facing order detail still allows image URLs through the existing include-images flag.
- Backend review: merchant ownership remains in existing server-side order service and handler checks; no client-supplied merchant identity was introduced. Async WebSocket snapshot failures now fail the payment outbox dispatch before merchant publish, so server-side retry owns recovery and incomplete item payloads are not emitted as normal `new_order` events.
- App review: WebSocket, polling, notification tap, local notification, auto-accept, full-screen alert, and receipt printing now converge on `PushMessage` + `OrderModel` hydration. Status-only accept responses no longer overwrite already hydrated item/spec/note detail.
- Mini Program review: merchant order list subscribes to the same `new_order` event, dedups by `message_id`, silently refreshes, clears deferred refresh timers on hide/unload, and no longer renders item images or drifted `option_name` fields in merchant order/kitchen detail.
- Scenario matrix covered by code and tests: foreground WebSocket envelope parsing, duplicate `message_id`/`order_id` dedup handoff, list polling compatibility, notification tap detail hydration by `order_id`, server-side item-load retry boundary, historical item-load degraded display, and receipt output with items/specs/notes.
- Validation passed: `git diff --check -- locallife merchant_app weapp artifacts`; `make check-generated`; `go test ./logic -run 'TestBuildMerchantNewOrderNotification|TestDecodeOrderItemCustomizations|TestOrderServiceListMerchantOrders|TestMerchantAppPushDispatcher'`; `go test ./api -run 'TestListMerchantOrdersAPI|TestGetMerchantOrderAPI'`; `go test ./worker -run 'TestNotifyMerchantNewOrder_PublishesMerchantAppPayload'`; `flutter test test/ws_client_test.dart test/order_alert_coordinator_test.dart test/esc_pos_utils_test.dart`; `flutter analyze`; targeted Mini Program ESLint for changed files; changed-file diagnostics.
- Residual risk: `weapp npm run compile` remains blocked by pre-existing unrelated missing `miniprogram_npm/tdesign-miniprogram` toast/qrcode modules and implicit qrcode callback typings in 7 files outside this change. Android real-device checks for Huawei/Honor/Xiaomi/OPPO/vivo native push and Bluetooth printer output were not executed in this pass and must be recorded before release.

## 10. Non-Goals For This Plan

- Implementing real Huawei/Honor/Xiaomi/OPPO/vivo provider HTTP clients.
- Changing payment, refund, or settlement state semantics.
- Redesigning the entire merchant order UI beyond the item/spec/note reliability path.
- Replacing all existing backend order response field names in one breaking change.
- Adding offline local order database storage unless a later stage explicitly scopes it.

## 11. Documentation To Update During Implementation

- `merchant_app/docs/backend-interface-requirements.md`: order payload, items/specs/notes, WebSocket/native push/polling behavior.
- Swagger docs if backend response schemas or route annotations change.
- `merchant_app/docs/software_manual.md` if user-facing order display or printing behavior changes materially.
- This plan: mark each card complete only after implementation, validation, review, fixes, and doc sync.
