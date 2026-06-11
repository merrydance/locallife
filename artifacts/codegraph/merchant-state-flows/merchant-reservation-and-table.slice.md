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
- Fixed 2026-06-10/2026-06-11: merchant-created phone/walk-in reservations use an explicit offline/phone customer identity model for durable customer traceability; no-show/customer-risk records and customer-side reservation actions are not attributed or authorized through the operator staff account.

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

6. Fixed 2026-06-10/2026-06-11: merchant-created phone/walk-in reservations keep `table_reservations.user_id` as the creating operator for compatibility, but `CreateMerchantReservationTx` now upserts `merchant_offline_customers`, links `table_reservations.offline_customer_id`, and records `created_by_user_id`. Phone/contact inputs are trimmed before durable writes; migration `000264` trims historical non-online reservation contacts, normalizes blank historical contact names to an offline-customer fallback before grouping, adds a supporting `table_reservations(offline_customer_id)` index, and the offline-customer lookup is merchant-scoped because rows contain contact PII. Merchant reservation edits that change offline contact name/phone re-upsert and repoint `offline_customer_id`. Offline no-show behavior decisions leave `behavior_decisions.user_id` null and store `customer_identity_type`, `offline_customer_id`, `created_by_user_id`, and `reservation_source` in `fact_snapshot`; online no-show attribution remains unchanged. Customer-owner checks now only treat `NULL`, blank, trim-space, or `online` source rows as user-owned reservations, so phone/walk-in/merchant rows cannot be viewed, cancelled, checked in, started-cooking, modified for dishes, used for customer reservation payment/order/session flows, or counted in `/v1/reservations/me` totals through the operator `user_id`; merchant staff routes continue through merchant auth.
   Evidence: `locallife/db/migration/000264_add_merchant_offline_customers.up.sql`, `locallife/db/query/merchant_offline_customer.sql`, `locallife/db/query/table_reservation.sql`, `locallife/db/sqlc/tx_reservation.go`, `locallife/db/sqlc/tx_reservation_adjustment.go`, `locallife/db/sqlc/tx_create_partner_payment.go`, `locallife/logic/reservation.go`, `locallife/logic/reservation_update.go`, `locallife/logic/reservation_dishes.go`, `locallife/logic/order_session.go`, `locallife/logic/payment_order_service.go`, `locallife/logic/dining_session.go`, `locallife/logic/dining_session_precheck.go`, `locallife/logic/replace_order.go`, `locallife/api/table_reservation.go`, `locallife/db/sqlc/tx_reservation_test.go`, `locallife/db/sqlc/merchant_offline_customer_migration_test.go`.

7. Customer-created reservations use payment modes, create pending rows, can create reservation items, and schedule payment timeout.
   Evidence: `locallife/api/table_reservation.go:290`, `locallife/logic/reservation.go:129`, `locallife/db/sqlc/tx_reservation.go:621`, `locallife/db/sqlc/tx_reservation.go:633`, `locallife/api/table_reservation.go:386`.

8. Fixed 2026-06-08: customer and merchant reservation create paths both precheck table truth, then lock the table row and revalidate the latest merchant/type/status/capacity state before insert. Customer create also reapplies locked minimum-spend pricing, and both create paths run the final conflict check after the lock. Merchant reservation edit now calls `UpdateReservationTx`, locks the reservation row and target table row when table/date/time/guest count changes, revalidates merchant/type/status/capacity/minimum-spend, and checks the final table/date/time conflict before update.
   Evidence: `locallife/logic/reservation.go:175`, `locallife/logic/reservation.go:253`, `locallife/logic/reservation.go:297`, `locallife/logic/reservation.go:322`, `locallife/logic/reservation.go:358`, `locallife/logic/reservation.go:405`, `locallife/logic/reservation_update.go:63`, `locallife/logic/reservation_update.go:77`, `locallife/db/sqlc/tx_reservation.go:643`, `locallife/db/sqlc/tx_reservation.go:677`, `locallife/db/sqlc/tx_reservation.go:757`, `locallife/db/sqlc/tx_reservation.go:790`, `locallife/db/sqlc/tx_reservation.go:822`, `locallife/db/sqlc/tx_reservation.go:841`, `locallife/db/sqlc/tx_reservation.go:867`, `locallife/db/sqlc/tx_reservation.go:926`.

9. Conflict-window duration is derived from business hours, but only the first two same-day business-hour rows are mapped into lunch/dinner slots.
   Evidence: `locallife/logic/reservation.go:868`, `locallife/logic/reservation.go:881`, `locallife/logic/reservation.go:894`, `locallife/logic/reservation.go:899`.

10. Confirming a paid reservation changes the reservation to `confirmed` but intentionally does not occupy the table.
   Evidence: `locallife/api/table_reservation.go:1233`, `locallife/logic/reservation.go:379`, `locallife/db/sqlc/tx_reservation.go:194`, `locallife/db/sqlc/tx_reservation.go:197`, `locallife/db/sqlc/tx_reservation.go:209`.

11. Actual occupied-table state is created by dining-session open: it can import reservation items, activate a paid reservation order, mark the reservation `checked_in`, and set the table to `occupied`.
    Evidence: `locallife/api/server.go:959`, `locallife/logic/dining_session.go:149`, `locallife/logic/dining_session.go:310`, `locallife/db/sqlc/tx_dining_session.go:88`, `locallife/db/sqlc/tx_dining_session.go:134`, `locallife/db/sqlc/tx_dining_session.go:157`, `locallife/db/sqlc/tx_dining_session.go:164`.

12. Manual reservation check-in updates the reservation to `checked_in` and sends websocket notification, but it does not update table occupancy directly.
    Evidence: `locallife/api/table_reservation.go:1816`, `locallife/logic/reservation.go:661`, `locallife/logic/reservation.go:712`, `locallife/api/table_reservation.go:1832`.

13. Completion, cancellation, and no-show use transactions that can release the table if the table's `current_reservation_id` still points at the same reservation.
    Evidence: `locallife/logic/reservation.go:454`, `locallife/db/sqlc/tx_reservation.go:239`, `locallife/logic/reservation.go:554`, `locallife/db/sqlc/tx_reservation.go:35`, `locallife/logic/reservation.go:636`, `locallife/db/sqlc/tx_reservation.go:128`.

14. Cancel and no-show also release reservation inventory; cancel does it in `CancelReservationTx`, no-show calls `ReleaseReservationInventoryTx` after `MarkNoShowTx`.
    Evidence: `locallife/db/sqlc/tx_reservation.go:72`, `locallife/logic/reservation.go:648`, `locallife/db/sqlc/tx_reservation.go:540`.

15. Cancelling a paid/confirmed reservation can create a refund order after the reservation has already been marked cancelled; provider refund submission is asynchronous.
    Evidence: `locallife/logic/reservation.go:516`, `locallife/logic/reservation.go:554`, `locallife/logic/reservation.go:568`, `locallife/logic/reservation.go:576`, `locallife/worker/refund_recovery_scheduler.go:193`, `locallife/worker/refund_recovery_scheduler.go:247`.

16. Payment success for reservation and reservation add-on is applied through payment fact application and `ProcessPaymentSuccessTx`; it writes reservation payment/prepaid state and syncs reserved inventory.
    Evidence: `locallife/logic/payment_fact_application_service.go:282`, `locallife/db/sqlc/tx_payment_success.go:141`, `locallife/db/sqlc/tx_payment_success.go:162`, `locallife/db/sqlc/tx_payment_success.go:168`, `locallife/db/sqlc/tx_payment_success.go:172`, `locallife/db/sqlc/tx_payment_success.go:209`.

17. Payment timeout cancels pending reservations and releases reservation inventory.
    Evidence: `locallife/worker/task_reservation_timeout.go:117`, `locallife/worker/task_reservation_timeout.go:154`, `locallife/worker/task_reservation_timeout.go:162`.

18. Merchant table list and edit pages use `/v1/tables/**` wrappers for list, create, update, delete, release, QR code, image binding, and tags.
    Evidence: `weapp/miniprogram/pages/merchant/tables/index.ts:105`, `weapp/miniprogram/pages/merchant/tables/index.ts:236`, `weapp/miniprogram/pages/merchant/tables/index.ts:274`, `weapp/miniprogram/pages/merchant/tables/edit/index.ts:223`, `weapp/miniprogram/pages/merchant/tables/edit/index.ts:362`, `weapp/miniprogram/pages/merchant/tables/edit/index.ts:626`, `weapp/miniprogram/pages/merchant/tables/edit/index.ts:629`.

19. Table routes split permissions: owner/manager/cashier can read and patch status; owner/manager can create, edit, delete, tag, image, and generate QR code.
    Evidence: `locallife/api/server.go:873`, `locallife/api/server.go:874`, `locallife/api/server.go:878`, `locallife/api/server.go:887`, `locallife/api/server.go:888`, `locallife/api/server.go:890`, `locallife/api/server.go:904`.

20. Manual table release calls `PATCH /v1/tables/:id/status` with `available`; backend first tries to close an active dining session and then falls back to forced table status update.
    Evidence: `weapp/miniprogram/pages/merchant/tables/index.ts:236`, `weapp/miniprogram/api/table-device-management.ts:355`, `locallife/api/table.go:765`, `locallife/api/table.go:805`, `locallife/api/table.go:826`.

21. Fixed 2026-06-08: table create/update with `tag_ids` validates positive, non-duplicate, existing `table` tags before writes and persists table/tag changes through sqlc transactions.
    Evidence: `locallife/api/table.go:183`, `locallife/api/table.go:256`, `locallife/api/table.go:293`, `locallife/api/table.go:714`, `locallife/api/table.go:762`, `locallife/db/sqlc/tx_table.go:43`, `locallife/db/sqlc/tx_table.go:73`.

22. Fixed 2026-06-08: table image binding validates the media asset before mutating `table_images`, and set-primary/delete image actions are bound to the path table. The asset must be a confirmed, approved, public `media.CategoryTableImage` upload whose uploader is the merchant owner or an active non-pending merchant staff member; invalid assets are rejected before primary-image reset or insert. Setting a missing or foreign image as primary returns 404 without clearing the current primary; deleting an image is constrained by `(table_id, image_id)`.
    Evidence: `weapp/miniprogram/api/table-device-management.ts:428`, `locallife/api/table.go:218`, `locallife/api/table.go:251`, `locallife/api/table.go:1152`, `locallife/api/table.go:1311`, `locallife/api/table.go:1386`, `locallife/db/query/table.sql:149`, `locallife/db/query/table.sql:161`, `locallife/db/sqlc/tx_table.go:152`, `locallife/api/table_test.go:2111`, `locallife/api/table_test.go:2193`, `locallife/api/table_test.go:2576`, `locallife/api/table_test.go:2686`, `locallife/db/sqlc/table_test.go:767`.

23. Fixed 2026-06-08: table delete, fulfillment-field update, disable, and reservation create/edit now share one table-row concurrency contract. `DELETE /v1/tables/:id` uses `DeleteTableTx`, locks the table row first, maps locked missing rows to 404, active `current_reservation_id` to 409, and future `pending|paid|confirmed` reservations to 409. `PATCH /v1/tables/:id` blocks actual table number/type/capacity/minimum-spend changes and new `disabled` status when the table has future reservations; `PATCH /v1/tables/:id/status` blocks disabling in the same case. The update/disable transaction paths map locked missing rows to 404, and the non-critical direct update branch also maps post-precheck missing rows to 404. The guards are enforced after locking the table row; reservation creation revalidates the locked row before inserting, and merchant reservation edit locks the reservation plus target table before updating, so concurrent delete/disable/update cannot leave a reservation on stale table truth.
    Evidence: `locallife/api/table.go:278`, `locallife/api/table.go:816`, `locallife/api/table.go:862`, `locallife/api/table.go:947`, `locallife/api/table.go:1002`, `locallife/api/table.go:1105`, `locallife/logic/reservation_update.go:63`, `locallife/db/sqlc/tx_table.go:11`, `locallife/db/sqlc/tx_table.go:110`, `locallife/db/sqlc/tx_table.go:149`, `locallife/db/sqlc/tx_table.go:228`, `locallife/db/sqlc/tx_reservation.go:790`, `locallife/db/sqlc/tx_reservation.go:822`, `locallife/db/sqlc/tx_reservation.go:841`, `locallife/db/sqlc/tx_reservation.go:867`, `locallife/db/sqlc/tx_reservation.go:926`, `locallife/api/table_test.go:1054`, `locallife/api/table_test.go:1179`, `locallife/api/table_test.go:1472`, `locallife/api/table_reservation_test.go:1951`, `locallife/api/table_reservation_test.go:2108`, `locallife/api/table_reservation_test.go:2168`, `locallife/api/table_reservation_test.go:2210`, `locallife/db/sqlc/table_test.go:540`, `locallife/db/sqlc/tx_reservation_test.go:113`, `locallife/db/sqlc/tx_reservation_test.go:197`, `locallife/db/sqlc/tx_reservation_test.go:225`, `locallife/db/sqlc/tx_reservation_test.go:251`.

24. Table QR code generation is owner/manager-only and updates `tables.qr_code_url`; changing table number clears the QR URL.
    Evidence: `locallife/api/server.go:904`, `locallife/api/table.go:675`, `locallife/api/scan.go:582`, `locallife/db/query/table.sql:72`.

25. Flutter merchant App exposes table management from the order-list drawer and routes to `TableGridScreen`.
    Evidence: `merchant_app/lib/features/order/order_list_page.dart:187`, `merchant_app/lib/features/order/order_list_page.dart:192`, `merchant_app/lib/app.dart:53`, `merchant_app/lib/app.dart:55`.

26. Flutter table repository calls the same `/tables/**` backend contract for list, create, patch update, delete, status, tags, images, and QR code.
    Evidence: `merchant_app/lib/features/table/repositories/table_repository.dart:17`, `merchant_app/lib/features/table/repositories/table_repository.dart:23`, `merchant_app/lib/features/table/repositories/table_repository.dart:40`, `merchant_app/lib/features/table/repositories/table_repository.dart:64`, `merchant_app/lib/features/table/repositories/table_repository.dart:90`, `merchant_app/lib/features/table/repositories/table_repository.dart:95`, `merchant_app/lib/features/table/repositories/table_repository.dart:150`, `merchant_app/lib/features/table/repositories/table_repository.dart:180`.

27. Flutter table status actions update the local row from backend response and single-flight duplicate taps per table id.
    Evidence: `merchant_app/lib/features/table/ui/table_grid_screen.dart:29`, `merchant_app/lib/features/table/ui/table_grid_screen.dart:36`, `merchant_app/lib/features/table/providers/table_provider.dart:57`, `merchant_app/lib/features/table/providers/table_provider.dart:64`, `merchant_app/lib/features/table/providers/table_provider.dart:178`.

28. Flutter table create/edit sheet writes table metadata first, then binds a selected uploaded image if present, and finally reloads the table list.
    Evidence: `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:66`, `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:83`, `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:97`, `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:111`, `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:121`.

29. Fixed 2026-06-09: Flutter table image upload now uses `businessType='table'` and `mediaCategory='table'` in both the config sheet and gallery entrypoints, matching backend table-image binding's accepted `media_category='table'` contract.
    Evidence: `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:523`, `merchant_app/lib/features/table/ui/widgets/table_config_sheet.dart:566`, `merchant_app/lib/features/table/ui/widgets/table_image_gallery.dart:47`, `merchant_app/lib/features/table/ui/widgets/table_image_gallery.dart:87`, `locallife/api/table.go:232`, `merchant_app/test/table_image_media_category_contract_test.dart:6`.

30. Flutter websocket client handles `table_status_change` by patching the in-memory table row.
    Evidence: `merchant_app/lib/core/network/ws_client.dart:127`, `merchant_app/lib/features/table/providers/table_provider.dart:204`.

## Reverse-Reference Findings

- `table_reservations.status` has multiple writers: merchant/customer actions, payment fact application, timeout worker, dining-session open/close, and manual table release fallback.
- `tables.status/current_reservation_id` has multiple writers: manual table status API, dining-session open/close/transfer, and reservation terminal transactions.
- Confirmed reservation and table occupancy are deliberately decoupled; any UI that interprets `confirmed` as occupied will drift from backend truth.
- Manual table release is broad: if an active dining session exists, it closes the session; if the table has `current_reservation_id`, it completes that reservation before updating the table status. Fixed 2026-06-09: API proof now locks the active-session branch so manual release calls `CloseDiningSessionTx` and returns the released table rather than bypassing the dining-session close boundary. Mini Program release copy and contract proof now disclose that breadth before sending `available`.
- Fixed 2026-06-08: merchant list date filtering still paginates in memory after fetching date rows, but `total` now uses the filtered row count when `date+status` or `date+exception` is present. Backend coverage exists in `TestListMerchantReservationsDateScopedFilters`.
- `status=exception` is accepted only with a date in `listMerchantReservations`; without date it returns 400, matching the workbench page but fragile for generic wrappers.
- Fixed 2026-06-08: table create/update tag persistence now prevalidates submitted tag ids and uses `CreateTableTx`/`UpdateTableTx`, so a tag insert failure rolls back the table create/update and tag replacement together.
- Fixed 2026-06-08: table image media binding now rejects cross-merchant/non-staff uploads, pending-staff uploads, non-`table` media categories, unconfirmed uploads, non-public assets, and unapproved media before mutating table-image state. Set-primary and delete image writes are now constrained by the path table id, and missing/foreign primary targets do not clear the existing primary image.
- Fixed 2026-06-08: table delete, disable, and fulfillment-field updates now share the same locked table-row contract, and reservation create/edit paths now revalidate locked table truth before mutating. Delete locks the table row before rejecting active `current_reservation_id`, future reservations, or locked missing rows with stable 409/404 API responses. Future reservations are checked inside table transactions after row lock, so table number/type/capacity/minimum-spend changes and new disabled status return 409 instead of leaving future reservations tied to an unavailable or materially changed table; update/disable locked missing rows and non-critical direct-update missing rows return 404; concurrent reservation creation waits on the same row lock and fails if the table is now missing, disabled, non-room, wrong merchant, over capacity, or under minimum spend. Merchant reservation edit also locks the reservation row and target table row, rejects disabled/missing/wrong-merchant/non-room/over-capacity target tables, preserves online minimum-spend constraints, and checks final time conflicts inside the transaction.
- Fixed 2026-06-09: Flutter merchant App table-management image uploads now use `mediaCategory='table'`; `table_image_media_category_contract_test.dart` locks both Flutter upload entrypoints against backend table-image category enforcement.
- Fixed 2026-06-10/2026-06-11: merchant-created phone/walk-in reservations now create or reuse a `merchant_offline_customers` identity keyed by `(merchant_id, contact_phone)` and link it through `table_reservations.offline_customer_id`; contacts are trimmed before durable writes, historical non-online rows are normalized during migration so whitespace does not split one offline customer into multiple identities, and blank historical contact names converge to an offline-customer fallback instead of blocking upgrade. `GetMerchantOfflineCustomer` is merchant-scoped because the row contains contact PII, and `table_reservations.offline_customer_id` has a supporting index for FK cleanup. `created_by_user_id` records the operator that created the reservation while keeping the legacy `user_id` compatibility field available for existing reservation logic. Customer-owner authorization and user reservation counts now ignore non-online sources, so the compatibility `user_id` cannot be used as a customer identity for phone/walk-in/merchant rows.
- Fixed 2026-06-10: merchant reservation edits that change an offline reservation contact name or phone re-upsert the offline customer and repoint `table_reservations.offline_customer_id`, so later no-show facts reference the current offline identity rather than a stale customer row.
- Fixed 2026-06-10: no-show behavior decisions no longer punish the operator account for offline/phone/walk-in reservations. `MarkNoShowTx` leaves `behavior_decisions.user_id` null for non-online sources and persists offline identity/operator/source details in `fact_snapshot`; online reservations still attribute no-show to the real online user id.
- Payment timeout worker updates pending reservations to `cancelled` with `payment timeout`; it does not use `CancelReservationTx`, but pending reservations should not have current table occupancy.
- Fixed 2026-06-08: table delete locks through `DeleteTableTx` before checking `current_reservation_id` and future reservations; locked missing rows map to 404, active rows map to 409, and future reservations map to 409. API table update/disable requests route reservation-critical changes through `UpdateTableTx` / `UpdateTableStatusTx`, with locked missing rows also mapped to 404. Customer and merchant reservation creation route through `CreateReservationTx` / `CreateMerchantReservationTx` after table row lock, and merchant reservation edit routes through `UpdateReservationTx`. Direct generated `UpdateTable`, `UpdateTableStatus`, `DeleteTable`, `CreateTableReservation*`, and `UpdateReservation` remain low-level helpers and must not be used by future callers to bypass the transaction guard or locked-table reservation revalidation.

## SQL And Durable State Boundaries

- `tables`: table/room identity, status, `current_reservation_id`, QR URL, access code hash, and customer-visible room metadata.
- `table_tags`: table/tag association; modified by table create/update/tag endpoints.
- `table_images`: table media association and primary image flag.
- `table_reservations`: reservation status, timing, contact, source, offline customer link, creator/operator link, payment mode, payment/refund deadlines, prepaid/deposit amounts, and lifecycle timestamps.
- `merchant_offline_customers`: merchant-owned offline/phone/walk-in customer identity and contact history for merchant-created reservations.
- `reservation_items`: preordered dishes/combos for reservation and add-on modification flows.
- `reservation_inventory`: reservation-held dish inventory synchronized from reservation items.
- `daily_inventory.reserved_quantity`: inventory hold truth for reservation items.
- `payment_orders`, `reservation_payments`, `refund_orders`, `external_payment_facts`, and `external_payment_fact_applications`: reservation payment/refund terminal truth.
- `dining_sessions`, `billing_groups`, `billing_group_members`, `carts`, and `orders`: dine-in session and reservation-to-table occupancy truth.
- `behavior_decisions`: no-show risk record; offline reservation no-show leaves `user_id` null and stores offline identity details in `fact_snapshot`.
- `table_transfer_logs`: transfer audit when an open dining session moves tables.

## Trust, Authorization, And Tenant Checks

- Reservation merchant list/create/confirm/complete routes use merchant staff middleware for owner/manager/cashier.
- Reservation edit and no-show routes use owner/manager only.
- Generic reservation detail/cancel/check-in/start-cooking routes perform owner-or-merchant checks inside logic/handler; the customer-owner branch only applies to `NULL`, blank, trim-space, or `online` reservation sources, while phone/walk-in/merchant reservations require merchant authorization.
- Table read/status routes use owner/manager/cashier; table management, image, tag, delete, and QR routes use owner/manager.
- Table handlers resolve the merchant server-side and check table ownership before reads/writes.
- Dining-session open/transfer permits online reservation owner or merchant staff access, but also uses `CheckUserHasMerchantAccess`, which was already flagged in staff flow as role-agnostic around pending staff. Phone/walk-in/merchant reservation operators are not treated as reservation owners for precheck/open/order billing.
- Payment/refund fact application validates business owner and payment order linkage before mutating reservation payment/refund state.

## Idempotency And Duplicate-Submit Checks

- Frontend pages use local loading/submitting/action keys, but APIs have no client-provided idempotency key for merchant-created reservations, edits, status actions, or table writes.
- Reservation conflict checks plus table row locking protect concurrent creation and merchant edits for the same table/date/time window.
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
- Fixed 2026-06-09: Mini Program table edit uploads and binds images immediately for existing tables; for newly created tables it uploads media first, creates the table, then tries to bind pending images. If pending image binding partially fails after creation, the page now stays open as the newly created table's edit page, preserves uploaded `mediaId` values without pretending they are persisted image rows, shows a persistent warning, and retries pending bindings on the next save before navigating back.
- Table edit normally navigates back after a fully successful save; after a created-table partial image-binding failure it rehydrates the created table id into the same page so the retry path has a real table target.
- Fixed 2026-06-09: Flutter table grid reloads the list after table config save, table status actions single-flight duplicate taps per table id, and websocket `table_status_change` messages patch only the matching in-memory table row.
- Flutter table create/edit can create or update a table, then fail image binding, without surfacing a repair path beyond later re-entry.

## Test Coverage Signals

Observed tests:

- `locallife/api/table_reservation_test.go` covers create/get/list/confirm/cancel/complete/statistics, merchant create success, cashier create allowed, cashier update forbidden, merchant update transaction entrypoint, and locked disabled/missing target-table API mapping.
- `locallife/api/table_reservation_workbench_test.go` covers date-scoped filters, exception requiring date, and workbench summary.
- `locallife/db/sqlc/tx_reservation_test.go` covers reservation create, locked disabled-table rejection for customer and merchant create, merchant edit locked-table disabled/capacity/time-conflict rejection, confirm not occupying near reservation, complete/cancel/no-show/table consistency, active adjustment guards, online no-show user attribution, offline/phone no-show non-attribution to the operator account, user reservation count exclusion for offline operator rows, offline contact edit `offline_customer_id` repointing before no-show fact creation, and the merchant-created reservation edit -> dining-session open -> checked-in/occupied -> close -> completed/available path.
- `locallife/db/sqlc/tx_reservation_inventory_test.go` covers reserve/release inventory sync.
- `locallife/db/sqlc/tx_dining_session_test.go` and `tx_dining_session_transfer_test.go` cover open/transfer table occupancy and concurrent target behavior.
- `locallife/api/table_test.go` covers table CRUD, status update, manual release closing active dining sessions through `CloseDiningSessionTx`, delete, tag/image APIs, QR behavior, table tag prevalidation, and tagged create/update transaction entrypoints.
- Reservation payment/refund and food-safety/no-show alert workers have focused backend tests. `locallife/db/sqlc/merchant_offline_customer_migration_test.go` covers migration `000264` historical offline-customer backfill, including contact trim normalization, blank-name fallback cleanup, earliest `first_seen_at`, latest `last_seen_at`, reservation links, and the supporting `table_reservations_offline_customer_id_idx`. `locallife/db/sqlc/table_test.go` covers tagged table create/update rollback on tag insert failure. `weapp/scripts/check-merchant-reservation-table-status-contract.test.js` covers Mini Program copy for confirm-not-occupy, scoped completion release, and manual release closing active dining sessions/completing linked reservations. `weapp/scripts/check-merchant-reservation-actions-contract.test.js` covers backend-derived reservation action permissions plus selected `date/status` filters and backend `total` consumption. `weapp/scripts/check-merchant-table-image-bind-recovery.test.js` covers Mini Program created-table image-binding partial failure recovery and retry. `merchant_app/test/table_flow_websocket_contract_test.dart` covers Flutter table repository endpoint contracts plus table Provider load/create/update/status/delete state flow, single-flight duplicate status actions, and websocket table-status patching. `merchant_app/test/ws_client_test.dart` covers `WsClient` routing backend `table_status_change` payloads to the table callback.

Missing high-value tests:

- Fixed 2026-06-09: Mini Program reservation action-permission and date+status total-consumption contract proof covers backend `MerchantActionState` exposure, frontend preference for `merchant_action_state` and `primary_action_key`, primary/more-action rendering from view state, selected `date/status` list params, and `response.total` pagination/summary consumption.
- Fixed 2026-06-09: Mini Program reservation/table status-copy contract test covers confirm-not-occupy, scoped completion release, and broad manual table release semantics.
- Fixed 2026-06-10/2026-06-11: backend tests cover the offline/phone customer identity model, including migration `000264` backfill with contact normalization, blank-name fallback cleanup, index proof, merchant-scoped offline-customer lookup, contact edit identity repointing, API blank-contact rejection, customer detail/action denial for offline operator rows, `/v1/reservations/me` count/list consistency, dish adjustment denial, reservation payment/order/session denial, and no-show behavior attribution that does not punish the operator user id.
- Fixed 2026-06-08: future-reservation table update/disable/delete tests cover API 409/404 mapping and sqlc transaction rollback for guarded fulfillment-field updates, disabled status, locked missing update/disable/delete, non-critical direct-update missing rows, and locked active-reservation delete; reservation-create tests cover locked disabled-table rejection for customer and merchant create plus API 409/404 mapping for merchant create; merchant reservation edit tests cover transaction entrypoint, locked disabled/missing target-table API mapping, and sqlc disabled/capacity/time-conflict rejection.
- Fixed 2026-06-08: table image media ownership/category validation tests cover owner upload, active staff upload, cross-merchant rejection, pending-staff rejection, missing media rejection, wrong category rejection before primary reset, unconfirmed upload rejection, private media rejection, unapproved media rejection, set-primary missing/foreign-image no-clear behavior, and table-bound delete behavior.
- Fixed 2026-06-09: end-to-end reservation open/close path from merchant-created reservation edit -> dining-session open -> table occupied/checked-in -> close -> reservation completed/table available is covered at the sqlc transaction boundary, and manual table release of an active dining session is covered at the API boundary.
- Fixed 2026-06-09: Mini Program frontend recovery test covers a new table whose pending image binding partially fails: the page remains open in edit mode with the created table id, retains pending media for retry, renders a persistent TDesign warning, retries image binding on the next save, clears recovery state after success, and only then navigates back.
- Fixed 2026-06-09: Flutter table-management contract proof covers `/tables/**` list/detail/create/patch/delete/status/image/QR endpoints, local table Provider load/create/update/status/delete state convergence, duplicate status-action single-flight behavior, in-memory websocket table-status patching, and `WsClient` table-status callback routing.

## Gaps And Refactor Notes

- Fixed 2026-06-10/2026-06-11: merchant-created phone/walk-in reservations use an offline/phone customer identity model rather than treating staff `user_id` as customer identity. Migration `000264` adds `merchant_offline_customers`, `table_reservations.offline_customer_id`, `table_reservations.created_by_user_id`, contact normalization, blank-name fallback cleanup, and an offline-customer FK support index; merchant create links the offline customer, merchant edit repoints the link when offline contact identity changes, no-show risk decisions for non-online reservations record offline details in `fact_snapshot` without setting `behavior_decisions.user_id`, and customer-facing reservation owner checks/counts/payment/order/session paths now fail closed for non-online sources.
- Fixed 2026-06-08: merchant reservation list `total` counts filtered date rows when both `date` and `status` are present; `date+exception` follows the same filtered-row semantics.
- Fixed 2026-06-09: Mini Program reservation actions are derived from backend `merchant_action_state`, and the list view sends selected `date/status` filters while deriving pagination and summary copy from backend `total`; `check:merchant-reservation-actions-contract` covers the end-to-end frontend contract.
- Fixed 2026-06-08: table tag replacement validates tag ids before writes and is transactional through `CreateTableTx`/`UpdateTableTx`; API tests cover pre-write rejection and DB tests cover rollback on tag insert failure.
- Fixed 2026-06-08: table image binding enforces media ownership/category/status before primary-image reset and insert; set-primary and delete image actions are constrained by `(table_id, image_id)`.
- Fixed 2026-06-09: table media upload category semantics are normalized across Mini Program and Flutter App; backend accepts table images as `media_category='table'`, and Flutter's config/gallery upload entrypoints now send `mediaCategory='table'`.
- Fixed 2026-06-08: future-reservation contract now matches locked delete behavior for fulfillment-field edits and disable actions, and reservation create/edit rechecks locked table truth after waiting on concurrent table changes; active/future reservations must be cleared before deleting, and future reservations must be cancelled/reassigned before changing reservation-critical table fields or disabling the table. Merchant reservation edit now rejects disabled/missing/wrong-merchant/non-room/over-capacity target tables and time conflicts inside `UpdateReservationTx`.
- Fixed 2026-06-09: backend end-to-end proof now covers merchant-created reservation edit, dining-session open occupancy, dining-session close completion/release, and manual table release closing an active dining session through `CloseDiningSessionTx`.
- Fixed 2026-06-09: Mini Program status copy clarifies that confirm does not occupy a table; only dining-session open/开台 moves table truth to occupied/就餐中.
- Fixed 2026-06-09: Mini Program table-image partial recovery now keeps the created table edit page open when post-create image binding fails, preserves pending uploaded media, retries pending bindings on the next save, and guards the flow with `check:merchant-table-image-bind-recovery`.
- Fixed 2026-06-09: Flutter table flow/websocket proof now locks repository endpoint usage, Provider state convergence, status-action single-flight behavior, websocket patching, and `WsClient` routing with `table_flow_websocket_contract_test.dart` plus `ws_client_test.dart`.
- Consider making terminal reservation actions idempotent from the API caller perspective by returning current terminal truth on replay.
- Fixed 2026-06-09: Mini Program manual release copy now states that releasing a table can close the current dining session and complete the linked reservation before the table becomes available.

## Branch Exhaustion

- Entry branches checked: Mini Program reservation workbench/list/edit/actions, table list/edit/status/tag/image/QR flows, scan/dining-session adjacency, Flutter table grid/detail/config/status/action-sheet/image/gallery/tag selector, websocket table status patches, and reservation payment/refund async readers.
- Request branches checked: merchant reservation list/create/update/confirm/complete/cancel/no-show/detail/statistics/workbench, table CRUD/status/delete/tag/image/QR, dining-session open/close/transfer/manual release, reservation payment/refund facts, table image upload/bind, and Flutter table repository endpoints.
- Backend state branches checked: reservation pending/confirmed/paid/completed/cancelled/no-show, merchant-created reservations, table availability/occupied/disabled, `current_reservation_id`, dining sessions, table transfer logs, inventory reserve/release, payment/refund terminal states, no-show behavior decisions, tag associations, and table image media bindings.
- Async branches checked: reservation payment timeout, no-show alert, food-safety reservation alert, payment fact application, reservation outbox, refund recovery scheduler, Baofu profit-sharing after completed reservation, websocket table-status updates, and Flutter in-memory patching.
- Failure/retry branches checked: frontend local action guards, no request idempotency key, conflict checks with row locks, reservation-create/edit locked-table revalidation, terminal action replay returning conflicts, partial table image binding after table creation, table tag validation and rollback, manual table release broad fallback, table future-reservation update/disable blocking, and date+status total semantics.
- Reader/consumer branches checked: reservation list/workbench/edit, table list/detail/QR, customer scan-table/menu/cart/order, dining-session billing, kitchen/order fulfillment, inventory, payment/refund recovery, Flutter table management, and public/table readers.
- Authorization/tenant branches checked: reservation staff roles by action, owner/manager-only edit/no-show, generic detail/cancel/check-in owner-or-merchant checks, table owner/manager/cashier reads, owner/manager writes, table ownership checks, dining-session owner-or-staff access, role-agnostic pending-staff caveat, and payment/refund fact owner validation.
- Zombie/unreachable branches checked: merchant-created reservation staff-as-customer identity drift is fixed through `merchant_offline_customers`, offline no-show non-attribution, and customer-owner/source guards for detail/actions/counts/payment/order/session paths; table media category drift between Mini Program and Flutter is fixed; manual release can complete reservation through status endpoint; Flutter table flows exist and are in scope.
- Test-proof gaps checked: backend tests cover reservation CRUD/actions, reservation-create locked-table disabled/missing rejection, merchant reservation-edit transaction entrypoint and locked-table/time-conflict rejection, date+status total semantics, merchant reservation edit -> dining-session open/close convergence, manual release active-session close routing, online no-show attribution, offline/phone no-show non-attribution plus fact snapshot, offline operator denial on customer reservation detail/actions/dishes/payment/order/session paths, `/v1/reservations/me` total exclusion for offline operator rows, contact edit offline-customer repointing, migration `000264` historical offline-customer backfill with trim/blank-name fallback/index proof, table CRUD/status/images/tags/QR, table image media validation plus table-bound set/delete image actions, table locked delete, non-critical direct-update missing rows, future-reservation update/disable blocking, inventory sync, dining-session transfer, and payment/refund workers. Mini Program status-copy contract proof covers confirm-not-occupy, scoped completion release, and broad manual table release semantics. Mini Program action/total contract proof covers backend-derived permissions, primary/more-action rendering, selected `date/status` params, and backend `total` consumption. Mini Program table-image recovery proof covers created-table partial binding failure staying retryable. Flutter media-category contract proof covers table config/gallery upload alignment with backend `media_category='table'`. Flutter table flow/websocket proof covers table repository endpoints, Provider state convergence, duplicate status-action single-flight, in-memory websocket patching, and `WsClient` table-status routing.
