# Merchant Reservation And Table Slice

Status: merchant-state flow slice created
Risk class: G3 - reservation payment/refund, dine-in session activation, table occupancy, inventory holds, no-show behavior decisions, and merchant/customer shared state
Scope: merchant Mini Program reservation workbench and table management -> reservation/table APIs -> reservation status, table occupancy, dining sessions, payment facts, inventory, alerts, and refund/recovery paths

## Variant Coverage

This slice covers:

- Merchant reservation list/workbench, create/edit form, and status actions.
- Merchant table list/edit/release, QR code, tags, and table image binding.
- Flutter merchant App table grid, table status actions, table edit/create sheet, table image upload/binding, and websocket table-status updates.
- Backend reservation routes under `/v1/reservations/**`, table routes under `/v1/tables/**`, and dining-session routes that create the real occupied-table state.
- Reservation payment success, payment timeout, cancellation/refund, no-show, inventory hold/release, and Baofu profit-sharing trigger paths where they affect merchant-visible reservation/table state.

This slice does not fully cover:

- Customer room-browsing UI, customer reservation checkout, and customer table-scan UX except where shared APIs mutate the same reservation/table state.
- Full reservation add-on payment/refund UX; this slice records its persistence and async impact because it blocks merchant terminal actions.
- Full Baofu refund/profit-sharing provider details; provider-specific facts remain in payment-domain slices.

## Product Invariant

Merchant reservation and table state must remain coherent:

- A merchant may create, edit, confirm, cancel, complete, mark no-show, and start cooking only reservations owned by the resolved merchant.
- A reservation time conflict must be checked against the table, date, and business-hour-derived slot configuration, including the final locked transaction check.
- A confirmed reservation is not the same thing as an occupied table. Actual occupancy comes from dining-session open, which marks the reservation checked in and the table occupied.
- Completing, cancelling, no-showing, closing a dining session, or manually releasing a table must clear table occupancy only when the current reservation/session matches the table truth.
- Reservation payment success and add-on payment success must update reservation/payment/inventory truth only from terminal payment facts.
- Table edits, images, tags, and QR codes must stay scoped to the merchant's own table and must not leave partial metadata that changes customer room discovery unexpectedly.

## Primary Forward Chain

1. Merchant reservation workbench loads date-filtered reservation truth and preserves prior list data on silent refresh failures.
   Evidence: `weapp/miniprogram/pages/merchant/reservations/index.ts:204`, `weapp/miniprogram/pages/merchant/reservations/index.ts:320`, `weapp/miniprogram/pages/merchant/reservations/index.ts:512`.

2. Merchant create/edit form loads existing reservation from the previous page or backend detail, loads enabled tables, and filters out disabled tables except the reservation's current table.
   Evidence: `weapp/miniprogram/pages/merchant/reservations/edit/index.ts:123`, `weapp/miniprogram/pages/merchant/reservations/edit/index.ts:147`, `weapp/miniprogram/pages/merchant/reservations/edit/index.ts:152`, `weapp/miniprogram/pages/merchant/reservations/edit/index.ts:155`.

3. Merchant reservation submit calls `/v1/reservations/merchant/create` for new records and `/v1/reservations/:id/update` for edits.
   Evidence: `weapp/miniprogram/pages/merchant/reservations/edit/index.ts:406`, `weapp/miniprogram/pages/merchant/reservations/edit/index.ts:408`, `weapp/miniprogram/pages/merchant/_main_shared/api/reservation.ts:687`, `weapp/miniprogram/pages/merchant/_main_shared/api/reservation.ts:699`.

4. Backend reservation routes split permissions: owner/manager/cashier can list, create, confirm, and complete; owner/manager can edit and mark no-show.
   Evidence: `locallife/api/server.go:933`, `locallife/api/server.go:934`, `locallife/api/server.go:941`, `locallife/api/server.go:942`, `locallife/api/server.go:943`, `locallife/api/server.go:946`, `locallife/api/server.go:947`, `locallife/api/server.go:949`, `locallife/api/server.go:950`.

5. Merchant-created reservations are direct confirmed reservations with zero deposit/prepaid amount and a far-future payment deadline.
   Evidence: `locallife/api/table_reservation.go:1726`, `locallife/logic/reservation.go:309`, `locallife/logic/reservation.go:335`, `locallife/logic/reservation.go:357`, `locallife/logic/reservation.go:360`.

6. Customer-created reservations use payment modes, create pending rows, can create reservation items, and schedule payment timeout.
   Evidence: `locallife/api/table_reservation.go:290`, `locallife/logic/reservation.go:129`, `locallife/db/sqlc/tx_reservation.go:621`, `locallife/db/sqlc/tx_reservation.go:633`, `locallife/api/table_reservation.go:386`.

7. Reservation conflict checks happen before write and again inside `CreateReservationTx` after locking the table.
   Evidence: `locallife/logic/reservation.go:193`, `locallife/logic/reservation.go:827`, `locallife/db/sqlc/tx_reservation.go:628`, `locallife/logic/reservation.go:234`, `locallife/logic/reservation.go:260`.

8. Conflict-window duration is derived from business hours, but only the first two same-day business-hour rows are mapped into lunch/dinner slots.
   Evidence: `locallife/logic/reservation.go:868`, `locallife/logic/reservation.go:881`, `locallife/logic/reservation.go:894`, `locallife/logic/reservation.go:899`.

9. Confirming a paid reservation changes the reservation to `confirmed` but intentionally does not occupy the table.
   Evidence: `locallife/api/table_reservation.go:1233`, `locallife/logic/reservation.go:379`, `locallife/db/sqlc/tx_reservation.go:194`, `locallife/db/sqlc/tx_reservation.go:197`, `locallife/db/sqlc/tx_reservation.go:209`.

10. Actual occupied-table state is created by dining-session open: it can import reservation items, activate a paid reservation order, mark the reservation `checked_in`, and set the table to `occupied`.
    Evidence: `locallife/api/server.go:959`, `locallife/logic/dining_session.go:149`, `locallife/logic/dining_session.go:310`, `locallife/db/sqlc/tx_dining_session.go:88`, `locallife/db/sqlc/tx_dining_session.go:134`, `locallife/db/sqlc/tx_dining_session.go:157`, `locallife/db/sqlc/tx_dining_session.go:164`.

11. Manual reservation check-in updates the reservation to `checked_in` and sends websocket notification, but it does not update table occupancy directly.
    Evidence: `locallife/api/table_reservation.go:1816`, `locallife/logic/reservation.go:661`, `locallife/logic/reservation.go:712`, `locallife/api/table_reservation.go:1832`.

12. Completion, cancellation, and no-show use transactions that can release the table if the table's `current_reservation_id` still points at the same reservation.
    Evidence: `locallife/logic/reservation.go:454`, `locallife/db/sqlc/tx_reservation.go:239`, `locallife/logic/reservation.go:554`, `locallife/db/sqlc/tx_reservation.go:35`, `locallife/logic/reservation.go:636`, `locallife/db/sqlc/tx_reservation.go:128`.

13. Cancel and no-show also release reservation inventory; cancel does it in `CancelReservationTx`, no-show calls `ReleaseReservationInventoryTx` after `MarkNoShowTx`.
    Evidence: `locallife/db/sqlc/tx_reservation.go:72`, `locallife/logic/reservation.go:648`, `locallife/db/sqlc/tx_reservation.go:540`.

14. Cancelling a paid/confirmed reservation can create a refund order after the reservation has already been marked cancelled; provider refund submission is asynchronous.
    Evidence: `locallife/logic/reservation.go:516`, `locallife/logic/reservation.go:554`, `locallife/logic/reservation.go:568`, `locallife/logic/reservation.go:576`, `locallife/worker/refund_recovery_scheduler.go:193`, `locallife/worker/refund_recovery_scheduler.go:247`.

15. Payment success for reservation and reservation add-on is applied through payment fact application and `ProcessPaymentSuccessTx`; it writes reservation payment/prepaid state and syncs reserved inventory.
    Evidence: `locallife/logic/payment_fact_application_service.go:282`, `locallife/db/sqlc/tx_payment_success.go:141`, `locallife/db/sqlc/tx_payment_success.go:162`, `locallife/db/sqlc/tx_payment_success.go:168`, `locallife/db/sqlc/tx_payment_success.go:172`, `locallife/db/sqlc/tx_payment_success.go:209`.

16. Payment timeout cancels pending reservations and releases reservation inventory.
    Evidence: `locallife/worker/task_reservation_timeout.go:117`, `locallife/worker/task_reservation_timeout.go:154`, `locallife/worker/task_reservation_timeout.go:162`.

17. Merchant table list and edit pages use `/v1/tables/**` wrappers for list, create, update, delete, release, QR code, image binding, and tags.
    Evidence: `weapp/miniprogram/pages/merchant/tables/index.ts:105`, `weapp/miniprogram/pages/merchant/tables/index.ts:236`, `weapp/miniprogram/pages/merchant/tables/index.ts:274`, `weapp/miniprogram/pages/merchant/tables/edit/index.ts:223`, `weapp/miniprogram/pages/merchant/tables/edit/index.ts:362`, `weapp/miniprogram/pages/merchant/tables/edit/index.ts:626`, `weapp/miniprogram/pages/merchant/tables/edit/index.ts:629`.

18. Table routes split permissions: owner/manager/cashier can read and patch status; owner/manager can create, edit, delete, tag, image, and generate QR code.
    Evidence: `locallife/api/server.go:873`, `locallife/api/server.go:874`, `locallife/api/server.go:878`, `locallife/api/server.go:887`, `locallife/api/server.go:888`, `locallife/api/server.go:890`, `locallife/api/server.go:904`.

19. Manual table release calls `PATCH /v1/tables/:id/status` with `available`; backend first tries to close an active dining session and then falls back to forced table status update.
    Evidence: `weapp/miniprogram/pages/merchant/tables/index.ts:236`, `weapp/miniprogram/api/table-device-management.ts:355`, `locallife/api/table.go:765`, `locallife/api/table.go:805`, `locallife/api/table.go:826`.

20. Fixed 2026-06-08: table create/update with `tag_ids` validates positive, non-duplicate, existing `table` tags before writes and persists table/tag changes through sqlc transactions.
    Evidence: `locallife/api/table.go:183`, `locallife/api/table.go:256`, `locallife/api/table.go:293`, `locallife/api/table.go:714`, `locallife/api/table.go:762`, `locallife/db/sqlc/tx_table.go:43`, `locallife/db/sqlc/tx_table.go:73`.

21. Fixed 2026-06-08: table image binding validates the media asset before mutating `table_images`, and set-primary/delete image actions are bound to the path table. The asset must be a confirmed, approved, public `media.CategoryTableImage` upload whose uploader is the merchant owner or an active non-pending merchant staff member; invalid assets are rejected before primary-image reset or insert. Setting a missing or foreign image as primary returns 404 without clearing the current primary; deleting an image is constrained by `(table_id, image_id)`.
    Evidence: `weapp/miniprogram/api/table-device-management.ts:428`, `locallife/api/table.go:218`, `locallife/api/table.go:251`, `locallife/api/table.go:1152`, `locallife/api/table.go:1311`, `locallife/api/table.go:1386`, `locallife/db/query/table.sql:149`, `locallife/db/query/table.sql:161`, `locallife/db/sqlc/tx_table.go:152`, `locallife/api/table_test.go:2111`, `locallife/api/table_test.go:2193`, `locallife/api/table_test.go:2576`, `locallife/api/table_test.go:2686`, `locallife/db/sqlc/table_test.go:767`.

22. Table QR code generation is owner/manager-only and updates `tables.qr_code_url`; changing table number clears the QR URL.
    Evidence: `locallife/api/server.go:904`, `locallife/api/table.go:675`, `locallife/api/scan.go:582`, `locallife/db/query/table.sql:72`.

23. Flutter merchant App exposes table management from the order-list drawer and routes to `TableGridScreen`.
    Evidence: `merchant_app/lib/features/order/order_list_page.dart:187`, `merchant_app/lib/features/order/order_list_page.dart:192`, `merchant_app/lib/app.dart:53`, `merchant_app/lib/app.dart:55`.

24. Flutter table repository calls the same `/tables/**` backend contract for list, create, patch update, delete, status, tags, images, and QR code.
    Evidence: `merchant_app/lib/features/table/repositories/table_repository.dart:17`, `merchant_app/lib/features/table/repositories/table_repository.dart:23`, `merchant_app/lib/features/table/repositories/table_repository.dart:40`, `merchant_app/lib/features/table/repositories/table_repository.dart:64`, `merchant_app/lib/features/table/repositories/table_repository.dart:90`, `merchant_app/lib/features/table/repositories/table_repository.dart:95`, `merchant_app/lib/features/table/repositories/table_repository.dart:150`, `merchant_app/lib/features/table/repositories/table_repository.dart:180`.

25. Flutter table status actions update the local row from backend response and single-flight duplicate taps per table id.
    Evidence: `merchant_app/lib/features/table/ui/table_grid_screen.dart:29`, `merchant_app/lib/features/table/ui/table_grid_screen.dart:36`, `merchant_app/lib/features/table/providers/table_provider.dart:57`, `merchant_app/lib/features/table/providers/table_provider.dart:64`, `merchant_app/lib/features/table/providers/table_provider.dart:178`.

26. Flutter table create/edit sheet writes table metadata first, then binds a selected uploaded image if present, and finally reloads the table list.
    Evidence: `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:66`, `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:83`, `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:97`, `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:111`, `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:121`.

27. Flutter table image upload uses `businessType='table'` and `mediaCategory='table_cover'`, which differs from the Mini Program trace's `businessType='merchant'` and `mediaCategory='table'` wording. Backend media policy and table-image binding now enforce the accepted category as `table`, so Flutter upload payloads still need contract alignment.
    Evidence: `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:523`, `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:565`, `merchant_app/lib/core/service/media_upload_service.dart:21`, `merchant_app/lib/core/service/media_upload_service.dart:72`.

28. Flutter websocket client handles `table_status_change` by patching the in-memory table row.
    Evidence: `merchant_app/lib/core/network/ws_client.dart:127`, `merchant_app/lib/features/table/providers/table_provider.dart:204`.

## Reverse-Reference Findings

- `table_reservations.status` has multiple writers: merchant/customer actions, payment fact application, timeout worker, dining-session open/close, and manual table release fallback.
- `tables.status/current_reservation_id` has multiple writers: manual table status API, dining-session open/close/transfer, and reservation terminal transactions.
- Confirmed reservation and table occupancy are deliberately decoupled; any UI that interprets `confirmed` as occupied will drift from backend truth.
- Manual table release is broad: if an active dining session exists, it closes the session; if the table has `current_reservation_id`, it completes that reservation before updating the table status.
- Fixed 2026-06-08: merchant list date filtering still paginates in memory after fetching date rows, but `total` now uses the filtered row count when `date+status` or `date+exception` is present. Backend coverage exists in `TestListMerchantReservationsDateScopedFilters`.
- `status=exception` is accepted only with a date in `listMerchantReservations`; without date it returns 400, matching the workbench page but fragile for generic wrappers.
- Fixed 2026-06-08: table create/update tag persistence now prevalidates submitted tag ids and uses `CreateTableTx`/`UpdateTableTx`, so a tag insert failure rolls back the table create/update and tag replacement together.
- Fixed 2026-06-08: table image media binding now rejects cross-merchant/non-staff uploads, pending-staff uploads, non-`table` media categories, unconfirmed uploads, non-public assets, and unapproved media before mutating table-image state. Set-primary and delete image writes are now constrained by the path table id, and missing/foreign primary targets do not clear the existing primary image.
- Flutter merchant App still has a second table-management client using `businessType='table'` and `mediaCategory='table_cover'`, while backend media policy accepts table images as `media_category='table'`.
- Merchant-created reservations use the operator user id as `user_id`; user reservation list intentionally filters out non-online sources, but downstream user/risk/payment assumptions must remember this is not the real customer account.
- No-show writes a `behavior_decisions` user risk record against reservation user id. For merchant-created phone/walk-in reservations that user id is the operator, which can wrongly attribute no-show behavior to staff.
- Payment timeout worker updates pending reservations to `cancelled` with `payment timeout`; it does not use `CancelReservationTx`, but pending reservations should not have current table occupancy.
- Table delete blocks `current_reservation_id` and future reservations through `DeleteTableTx`, but direct `UpdateTable` can disable or change a table while future reservations still exist.

## SQL And Durable State Boundaries

- `tables`: table/room identity, status, `current_reservation_id`, QR URL, access code hash, and customer-visible room metadata.
- `table_tags`: table/tag association; modified by table create/update/tag endpoints.
- `table_images`: table media association and primary image flag.
- `table_reservations`: reservation status, timing, contact, source, payment mode, payment/refund deadlines, prepaid/deposit amounts, and lifecycle timestamps.
- `reservation_items`: preordered dishes/combos for reservation and add-on modification flows.
- `reservation_inventory`: reservation-held dish inventory synchronized from reservation items.
- `daily_inventory.reserved_quantity`: inventory hold truth for reservation items.
- `payment_orders`, `reservation_payments`, `refund_orders`, `external_payment_facts`, and `external_payment_fact_applications`: reservation payment/refund terminal truth.
- `dining_sessions`, `billing_groups`, `billing_group_members`, `carts`, and `orders`: dine-in session and reservation-to-table occupancy truth.
- `behavior_decisions`: no-show risk record.
- `table_transfer_logs`: transfer audit when an open dining session moves tables.

## Trust, Authorization, And Tenant Checks

- Reservation merchant list/create/confirm/complete routes use merchant staff middleware for owner/manager/cashier.
- Reservation edit and no-show routes use owner/manager only.
- Generic reservation detail/cancel/check-in/start-cooking routes perform owner-or-merchant checks inside logic/handler.
- Table read/status routes use owner/manager/cashier; table management, image, tag, delete, and QR routes use owner/manager.
- Table handlers resolve the merchant server-side and check table ownership before reads/writes.
- Dining-session open/transfer permits reservation owner or merchant staff access, but also uses `CheckUserHasMerchantAccess`, which was already flagged in staff flow as role-agnostic around pending staff.
- Payment/refund fact application validates business owner and payment order linkage before mutating reservation payment/refund state.

## Idempotency And Duplicate-Submit Checks

- Frontend pages use local loading/submitting/action keys, but APIs have no client-provided idempotency key for merchant-created reservations, edits, status actions, or table writes.
- Reservation conflict checks plus table row locking protect concurrent creation for the same table/date/time window.
- Payment fact application and `ProcessPaymentSuccessTx` are idempotent around existing reservation payment rows and terminal fact application.
- Completion/cancel/no-show status updates are conditional at the logic layer but not request-idempotent; retry after success can return conflict rather than the already-terminal truth.
- Manual table release can be repeated and tends to converge to `available`, but its broad fallback can complete a reservation if `current_reservation_id` is still set.
- Table create/update tag replacement is fixed as an atomic per-request workflow, but remains unversioned/last-write-wins. Table image add remains a partial-progress workflow without versioning, while set-primary is now an atomic transaction and delete is table-bound.
- Flutter table status actions are single-flight per table id; create/update/delete are not durable-idempotent and rely on local submit/action guards.

## Recovery And Async Convergence Paths

- Reservation payment timeout task cancels pending reservations and releases inventory.
- Reservation no-show alert task sends merchant notification around reservation time if still paid/confirmed.
- Food-safety alert task notifies future reservation users if the merchant remains suspended.
- Payment fact application applies reservation and reservation add-on payment success, syncs inventory, and emits reservation payment outbox.
- Refund recovery scheduler discovers cancelled reservation payment orders and pending reservation refund orders, then enqueues retry/fact application paths.
- Dining-session close, manual table release, and transfer are synchronous transaction boundaries for table occupancy convergence.
- Baofu payment recovery/profit-sharing paths pick up completed reservations for provider-side settlement/profit sharing.

## Frontend Draft And Backend Rehydration

- Reservation list/workbench rebuilds from backend truth and preserves trusted data on refresh failure.
- Reservation edit form loads table options from backend; after save it asks the previous list page to refresh before navigating back.
- Reservation action dialogs do not optimistically patch the row; they execute backend action, refresh the page, then show success.
- Table list refreshes backend truth after manual release and after returning from edit.
- Table edit uploads and binds images immediately for existing tables; for newly created tables it uploads media first, creates the table, then tries to bind pending images. Partial image binding failure is surfaced in a toast but leaves the table created.
- Table edit navigates back after save rather than rehydrating the detail response into the same page.
- Flutter table grid reloads the list after table config save and applies websocket table-status patches in-memory.
- Flutter table create/edit can create or update a table, then fail image binding, without surfacing a repair path beyond later re-entry.

## Test Coverage Signals

Observed tests:

- `locallife/api/table_reservation_test.go` covers create/get/list/confirm/cancel/complete/statistics, merchant create success, cashier create allowed, and cashier update forbidden.
- `locallife/api/table_reservation_workbench_test.go` covers date-scoped filters, exception requiring date, and workbench summary.
- `locallife/db/sqlc/tx_reservation_test.go` covers reservation create, confirm not occupying near reservation, complete/cancel/no-show/table consistency, and active adjustment guards.
- `locallife/db/sqlc/tx_reservation_inventory_test.go` covers reserve/release inventory sync.
- `locallife/db/sqlc/tx_dining_session_test.go` and `tx_dining_session_transfer_test.go` cover open/transfer table occupancy and concurrent target behavior.
- `locallife/api/table_test.go` covers table CRUD, status update, delete, tag/image APIs, QR behavior, table tag prevalidation, and tagged create/update transaction entrypoints.
- Reservation payment/refund and food-safety/no-show alert workers have focused backend tests. `locallife/db/sqlc/table_test.go` covers tagged table create/update rollback on tag insert failure.

Missing high-value tests:

- Mini Program contract tests for reservation action permissions and frontend consumption of date+status total semantics.
- Backend test for merchant-created reservation no-show behavior attribution to operator user id.
- Backend test deciding whether disabling/changing a table with future reservations should be blocked or should notify/cancel/reassign those reservations.
- Fixed 2026-06-08: table image media ownership/category validation tests cover owner upload, active staff upload, cross-merchant rejection, pending-staff rejection, missing media rejection, wrong category rejection before primary reset, unconfirmed upload rejection, private media rejection, unapproved media rejection, set-primary missing/foreign-image no-clear behavior, and table-bound delete behavior.
- End-to-end reservation open/close path from merchant-created reservation -> dining session open -> table occupied -> close/release -> reservation completed.
- Frontend recovery test for new table created but pending image binding partially failed.
- Flutter table-management coverage for create/edit/status/image flows and websocket table-status patching.

## Gaps And Refactor Notes

- Decide whether merchant-created phone/walk-in reservations should use staff `user_id` as the customer identity. Current no-show behavior decisions can punish the operator account rather than a real customer.
- Fixed 2026-06-08: merchant reservation list `total` counts filtered date rows when both `date` and `status` are present; `date+exception` follows the same filtered-row semantics.
- Fixed 2026-06-08: table tag replacement validates tag ids before writes and is transactional through `CreateTableTx`/`UpdateTableTx`; API tests cover pre-write rejection and DB tests cover rollback on tag insert failure.
- Fixed 2026-06-08: table image binding enforces media ownership/category/status before primary-image reset and insert; set-primary and delete image actions are constrained by `(table_id, image_id)`.
- Normalize table media upload category semantics across Mini Program and Flutter App; backend accepts table images as `media_category='table'`.
- Decide the product contract for disabling/editing/deleting tables with future reservations; currently delete blocks future reservations, but update/disable does not visibly enforce the same invariant.
- Clarify in UI/API docs that confirm does not occupy a table; only dining-session open does.
- Consider making terminal reservation actions idempotent from the API caller perspective by returning current terminal truth on replay.
- Review manual table release fallback: it is useful for repair, but it can close sessions and complete reservations from a simple table-status endpoint.

## Branch Exhaustion

- Entry branches checked: Mini Program reservation workbench/list/edit/actions, table list/edit/status/tag/image/QR flows, scan/dining-session adjacency, Flutter table grid/detail/config/status/action-sheet/image/gallery/tag selector, websocket table status patches, and reservation payment/refund async readers.
- Request branches checked: merchant reservation list/create/update/confirm/complete/cancel/no-show/detail/statistics/workbench, table CRUD/status/delete/tag/image/QR, dining-session open/close/transfer/manual release, reservation payment/refund facts, table image upload/bind, and Flutter table repository endpoints.
- Backend state branches checked: reservation pending/confirmed/paid/completed/cancelled/no-show, merchant-created reservations, table availability/occupied/disabled, `current_reservation_id`, dining sessions, table transfer logs, inventory reserve/release, payment/refund terminal states, no-show behavior decisions, tag associations, and table image media bindings.
- Async branches checked: reservation payment timeout, no-show alert, food-safety reservation alert, payment fact application, reservation outbox, refund recovery scheduler, Baofu profit-sharing after completed reservation, websocket table-status updates, and Flutter in-memory patching.
- Failure/retry branches checked: frontend local action guards, no request idempotency key, conflict checks with row locks, terminal action replay returning conflicts, partial table image binding after table creation, table tag validation and rollback, manual table release broad fallback, disabled/edit table with future reservations, and date+status total semantics.
- Reader/consumer branches checked: reservation list/workbench/edit, table list/detail/QR, customer scan-table/menu/cart/order, dining-session billing, kitchen/order fulfillment, inventory, payment/refund recovery, Flutter table management, and public/table readers.
- Authorization/tenant branches checked: reservation staff roles by action, owner/manager-only edit/no-show, generic detail/cancel/check-in owner-or-merchant checks, table owner/manager/cashier reads, owner/manager writes, table ownership checks, dining-session owner-or-staff access, role-agnostic pending-staff caveat, and payment/refund fact owner validation.
- Zombie/unreachable branches checked: merchant-created reservation uses staff user as customer identity; table media categories differ between Mini Program and Flutter; table disable/update lacks delete-like future-reservation enforcement; manual release can complete reservation through status endpoint; Flutter table flows exist and are in scope.
- Test-proof gaps checked: backend tests cover reservation CRUD/actions, date+status total semantics, table CRUD/status/images/tags/QR, table image media validation plus table-bound set/delete image actions, inventory sync, dining-session transfer, and payment/refund workers. Missing proof remains for Mini Program action permissions and frontend date+status total consumption, merchant-created no-show attribution, table disable/update future-reservation contract, full reservation-to-dining-session e2e, partial table-image recovery, and Flutter table flow/websocket tests.
