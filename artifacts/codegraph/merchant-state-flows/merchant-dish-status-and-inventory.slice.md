# Merchant Dish Status And Inventory Slice

Status: merchant-state flow slice created
Risk class: G2 - merchant-controlled dish availability, customization, and inventory state affect customer ordering and reservation paths, with multiple writers and drift signals
Scope: merchant dish list/edit/inventory pages -> dish and inventory APIs -> durable dish/customization/inventory state -> downstream customer/menu/order/reservation readers

## Variant Coverage

This slice covers:

- Merchant Mini Program dish list, single online-status toggle, dish create/update/delete, featured tags, and customizations.
- Merchant Mini Program daily inventory page and its `GET/PUT /v1/inventory` path.
- Backend dish and inventory route groups, handlers, transactions, SQL writes, and important downstream readers.
- Reverse references for `is_online`, `is_available`, customization groups, featured tags, and `daily_inventory`.

This slice does not fully cover:

- Dish category management as an independent merchant CRUD flow.
- Combo-set CRUD, except for delete-dish cleanup and packaging policy readers.
- Full order status/refund/print operations; those belong in the next `merchant-order-operations` flow.
- Full reservation lifecycle, except where reservation validates dishes and reserves/releases inventory.

## Product Invariant

The merchant-visible dish and inventory workflow should have one coherent availability contract:

- `dishes.is_online` controls whether the merchant intentionally publishes the dish.
- `dishes.is_available` is a real merchant-controlled temporary availability flag and must be honored by every customer-facing reader and validator.
- Packaging dishes must remain online and available.
- `daily_inventory.total_quantity = -1` means unlimited; finite inventory must not oversell across sold and reserved quantities.
- Missing daily inventory rows must have one consistent meaning across display, checks, order payment, and reservation flows.
- Frontend edits may draft locally, but saved state and re-entry must be based on backend truth.

## Primary Forward Chain

1. Merchant entrypoints route to dish and inventory pages from dashboard/config.
   Evidence: `weapp/miniprogram/pages/merchant/_utils/merchant-dashboard-view.ts:172`, `weapp/miniprogram/pages/merchant/_utils/merchant-dashboard-view.ts:175`, `weapp/miniprogram/pages/merchant/config/index.ts:38`, `weapp/miniprogram/app.json:322`.

2. The dish list builds `GET /v1/dishes` params from category and `is_online` filters. It does not send `is_available`.
   Evidence: `weapp/miniprogram/pages/merchant/dishes/index.ts:219`, `weapp/miniprogram/pages/merchant/dishes/index.ts:233`, `weapp/miniprogram/pages/merchant/dishes/index.ts:239`, `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:598`, `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:619`, `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:622`.

3. Single-dish online toggle is optimistic/pending locally, rejects packaging dishes in the page, calls `DishManagementService.updateDishStatus(id, { is_online })`, and reloads filtered views after success.
   Evidence: `weapp/miniprogram/pages/merchant/dishes/index.ts:299`, `weapp/miniprogram/pages/merchant/dishes/index.ts:309`, `weapp/miniprogram/pages/merchant/dishes/index.ts:328`, `weapp/miniprogram/pages/merchant/dishes/index.ts:341`, `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:706`.

4. Backend dish routes are under merchant staff middleware for `owner`, `manager`, and `chef`; single and batch status routes write online status.
   Evidence: `locallife/api/server.go:787`, `locallife/api/server.go:788`, `locallife/api/server.go:804`, `locallife/api/server.go:805`.

5. `updateDishStatus` resolves the current merchant, loads the dish, verifies ownership, rejects packaging offline, and writes `UpdateDishOnlineStatus`.
   Evidence: `locallife/api/dish.go:1421`, `locallife/api/dish.go:1438`, `locallife/api/dish.go:1449`, `locallife/api/dish.go:1460`, `locallife/api/dish.go:1464`, `locallife/api/dish.go:1470`, `locallife/db/query/dish.sql:321`.

6. Fixed 2026-06-03: `batchUpdateDishStatus` is registered and has a frontend wrapper, but no current merchant page caller was found. It filters invalid/foreign dish IDs and packaging dishes before `BatchUpdateDishOnlineStatus`; the SQL now returns the actual updated IDs, and the handler reports only those IDs as updated while moving any stale/pre-filtered miss into `failed`.
   Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:720`, `locallife/api/dish.go:1520`, `locallife/api/dish.go:1542`, `locallife/api/dish.go:1564`, `locallife/api/dish.go:1594`, `locallife/api/dish.go:1604`, `locallife/db/query/dish.sql:711`.

7. Dish edit loads categories, system tags, dish detail, and customizations. Partial tag/customization load failures become warnings rather than blocking base edit.
   Evidence: `weapp/miniprogram/pages/merchant/dishes/edit/index.ts:122`, `weapp/miniprogram/pages/merchant/dishes/edit/index.ts:151`, `weapp/miniprogram/pages/merchant/dishes/edit/index.ts:155`, `weapp/miniprogram/pages/merchant/dishes/edit/index.ts:171`.

8. Fixed 2026-06-03: the edit view maps backend `is_available` into form state, renders a merchant-facing `是否可售` TDesign switch, and submits the merchant-selected value. Packaging dishes remain forced available on submit.
   Evidence: `weapp/miniprogram/pages/merchant/_utils/merchant-dish-edit-view.ts:247`, `weapp/miniprogram/pages/merchant/_utils/merchant-dish-edit-view.ts:527`, `weapp/miniprogram/pages/merchant/dishes/edit/index.wxml:110`, `weapp/miniprogram/pages/merchant/dishes/edit/index.wxml:111`.

9. Packaging switch forces `is_packaging`, `is_online`, and `is_available` to true in the page. Backend create/update normalization also forces packaging dishes online and available.
   Evidence: `weapp/miniprogram/pages/merchant/dishes/edit/index.ts:313`, `weapp/miniprogram/pages/merchant/dishes/edit/index.ts:317`, `weapp/miniprogram/pages/merchant/dishes/edit/index.ts:318`, `locallife/api/dish.go:417`, `locallife/api/dish.go:428`.

10. Base create/update calls are transactional at the backend level. `CreateDishTx` creates dish/tags/image/customizations in one transaction when customizations are included in the base payload; `UpdateDishTx` updates the base dish and optionally replaces tags in one transaction.
    Evidence: `locallife/api/dish.go:378`, `locallife/api/dish.go:1000`, `locallife/api/dish.go:1148`, `locallife/db/sqlc/tx_dish.go:37`, `locallife/db/sqlc/tx_dish.go:139`, `locallife/db/query/dish.sql:296`.

11. Fixed 2026-06-03: the Mini Program dish edit submit remains a multi-call product workflow, but later featured-tag/customization failures now leave the page in edit mode with the saved dish id, a persistent TDesign warning, and the same save action available for retry.
    Evidence: `weapp/miniprogram/pages/merchant/dishes/edit/index.ts:569`, `weapp/miniprogram/pages/merchant/dishes/edit/index.ts:594`, `weapp/miniprogram/pages/merchant/dishes/edit/index.ts:599`, `weapp/miniprogram/pages/merchant/dishes/edit/index.ts:611`, `weapp/miniprogram/pages/merchant/dishes/edit/index.wxml:21`, `weapp/miniprogram/pages/merchant/_utils/merchant-dish-edit-view.ts:343`.

12. `setDishCustomizations` validates merchant/dish ownership and option tags, then replaces all customization groups/options inside `SetDishCustomizationsTx`.
    Evidence: `locallife/api/dish.go:1723`, `locallife/api/dish.go:1734`, `locallife/api/dish.go:1745`, `locallife/api/dish.go:1750`, `locallife/api/dish.go:1788`, `locallife/db/sqlc/tx_dish.go:247`, `locallife/db/sqlc/tx_dish.go:287`.

13. Fixed 2026-06-03: `setDishFeaturedTags` validates ownership, filters names to `推荐` and `热卖`, then calls `SetDishFeaturedTagsTx`; the transaction preserves non-featured dish tags, replaces featured tags atomically, propagates remove/upsert/final-list errors, and rolls back on failure.
    Evidence: `locallife/api/dish.go:1929`, `locallife/api/dish.go:1967`, `locallife/api/dish.go:1973`, `locallife/api/dish.go:1984`, `locallife/db/sqlc/tx_dish.go:315`, `locallife/db/sqlc/tx_dish.go:317`, `locallife/db/sqlc/tx_dish.go:329`, `locallife/db/sqlc/tx_dish.go:342`, `locallife/db/sqlc/tx_dish.go:350`, `locallife/api/dish_test.go:1055`, `locallife/db/sqlc/dish_test.go:672`, `locallife/db/sqlc/dish_test.go:729`.

14. The inventory page checks merchant console access, loads `GET /v1/inventory?date=...`, treats missing rows from the backend list as `total_quantity=-1`, computes local available as `total - sold - reserved`, and saves one row through `PUT /v1/inventory`.
    Evidence: `weapp/miniprogram/pages/merchant/inventory/index.ts:93`, `weapp/miniprogram/pages/merchant/inventory/index.ts:171`, `weapp/miniprogram/pages/merchant/inventory/index.ts:180`, `weapp/miniprogram/pages/merchant/inventory/index.ts:191`, `weapp/miniprogram/pages/merchant/inventory/index.ts:413`, `weapp/miniprogram/pages/merchant/inventory/index.ts:434`, `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:958`, `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:970`.

15. Inventory routes are under the same merchant staff middleware. `listDailyInventory` reads online dishes for the merchant and left-joins any daily row, defaulting missing rows to unlimited.
    Evidence: `locallife/api/server.go:831`, `locallife/api/server.go:832`, `locallife/api/server.go:836`, `locallife/api/inventory.go:162`, `locallife/api/inventory.go:191`, `locallife/db/query/inventory.sql:27`, `locallife/db/query/inventory.sql:33`, `locallife/db/query/inventory.sql:46`.

16. Fixed 2026-06-03: `updateDailyInventory` resolves the merchant, reads the row by current merchant/dish/date, verifies dish ownership before creating a missing row, rejects finite totals lower than sold plus reserved quantities, updates total/sold, and returns computed available using `reserved_quantity`.
    Evidence: `locallife/api/inventory.go:258`, `locallife/api/inventory.go:276`, `locallife/api/inventory.go:287`, `locallife/api/inventory.go:294`, `locallife/api/inventory.go:303`, `locallife/api/inventory.go:323`, `locallife/api/inventory.go:402`, `locallife/api/inventory.go:431`, `locallife/api/inventory_test.go:371`, `locallife/db/migration/000244_add_daily_inventory_committed_quantity_check.up.sql:8`, `locallife/db/sqlc/inventory_test.go:165`.

17. Fixed 2026-06-02: the direct `createDailyInventory` POST path now resolves the current merchant, loads the requested dish, verifies `dish.merchant_id == merchant.id`, and returns 404/403 before insert when the dish is missing or belongs to another merchant.
    Evidence: `locallife/api/server.go:835`, `locallife/api/inventory.go:61`, `locallife/api/inventory.go:79`, `locallife/api/inventory.go:89`, `locallife/api/inventory.go:98`, `locallife/api/inventory.go:104`, `locallife/api/inventory_test.go:101`.

18. Customer order creation enforces dish ownership, `is_online`, and `is_available` before pricing customizations.
    Evidence: `locallife/logic/order_items.go:42`, `locallife/logic/order_items.go:50`, `locallife/logic/order_items.go:53`, `locallife/logic/order_items.go:56`.

19. Payment success decrements inventory inside a transaction after checking paid-state idempotency. Missing inventory rows are treated as unlimited; finite rows use locked reads and conditional decrement.
    Evidence: `locallife/db/sqlc/tx_create_order.go:311`, `locallife/db/sqlc/tx_create_order.go:364`, `locallife/db/sqlc/tx_create_order.go:371`, `locallife/db/sqlc/tx_create_order.go:378`, `locallife/db/sqlc/tx_create_order.go:400`, `locallife/db/query/inventory.sql:77`.

20. Fixed 2026-06-03: reservation item validation checks merchant ownership, `is_online`, and `is_available`. Reservation inventory paths separately reserve/release inventory.
    Evidence: `locallife/logic/reservation.go:786`, `locallife/logic/reservation.go:793`, `locallife/logic/reservation.go:796`, `locallife/logic/reservation_dishes_test.go:132`.

## Reverse-Reference Findings

- Fixed 2026-06-03: `is_available` is persisted in `dishes`, preserved by the Mini Program editor, exposed as a merchant-facing availability switch, and submitted as backend truth.
- Fixed 2026-06-03: public merchant dish list, public dish detail, scan menu, merchant/global search, recommendation ID/detail readers, order/cart paths, and reservation validation now consistently exclude or reject unavailable dishes.
- `UpdateDishAvailability` SQL has coverage signals but no runtime caller was found in the merchant Mini Program or backend handlers; it looks like a legacy/zombie side path.
- Fixed 2026-06-03: unused Mini Program dish/inventory wrapper exposure was cleaned up. `batchUpdateDishStatus`, `checkInventory`, and `getInventoryStats` are no longer exported from the copied Mini Program dish API files because no current Mini Program runtime caller was found. Backend routes remain available and tested.
- `ListDailyInventoryByMerchant` hides offline dishes from the inventory editor even if `daily_inventory` rows already exist for those dishes.
- Fixed 2026-06-03: `GetInventoryStats` classifies sold-out/available using `sold_quantity + reserved_quantity`, matching page/backend available calculations.

## SQL And Durable State Boundaries

- `dishes`: owns `is_online`, `is_available`, `is_packaging`, `deleted_at`, pricing, media, category, and prepare-time state.
- `dish_tags`: owns general dish tags plus featured tags `推荐` and `热卖`.
- `dish_customization_groups` and `dish_customization_options`: own specification/customization state and option extra prices.
- `daily_inventory`: owns `(merchant_id, dish_id, date)` rows with unique index, `total_quantity`, `sold_quantity`, and `reserved_quantity`.
- `total_quantity = -1` is the effective unlimited-stock sentinel.
- `reserved_quantity` was added after the original inventory table and is included in current available calculations, reserve, release, and order decrement checks.
- Fixed 2026-06-03: finite `daily_inventory.total_quantity` is now constrained to stay at or above `sold_quantity + reserved_quantity`; the migration lifts historical inconsistent rows to the committed quantity before adding the check.

## Trust, Authorization, And Tenant Checks

- Dish and inventory route groups use `MerchantStaffMiddleware("owner", "manager", "chef")`.
- Dish create/update/status/customization/featured-tag/delete handlers resolve the current merchant and check dish ownership before writes.
- Inventory `PUT` missing-row path verifies the requested dish belongs to the merchant before creating the inventory row.
- Fixed 2026-06-02: direct inventory `POST` now applies the same dish-ownership verification before creating the inventory row.
- Customer-facing readers must not rely on merchant-provided fields for authorization; they should use persisted merchant/dish state and merchant status.

## Idempotency And Duplicate-Submit Checks

- Dish list status toggle uses per-item `statusPending`; duplicate taps are locally blocked while pending.
- Dish edit uses `submitting` to block duplicate submit, but the backend sees three separate writes for base dish, featured tags, and customizations.
- Base dish create/update transactions are atomic for the specific write they own, but not for the full Mini Program submit workflow.
- Fixed 2026-06-03: featured-tag replacement is transaction-protected and store errors are visible to the caller; repeated calls still converge to the requested featured-tag set.
- Inventory row save uses per-row `submitting` and `save_disabled`; repeated `PUT` is last-write-wins.
- Payment success inventory decrement is protected by paid-state idempotency and locked inventory rows.
- Fixed 2026-06-03: `POST /v1/inventory/check` is a read-only availability check. It verifies the requested dish belongs to the current merchant, treats a missing inventory row as unlimited stock, and leaves `sold_quantity` changes to order/reservation transaction writers.

## Recovery And Async Convergence Paths

- Dish list rolls local status pending state back on status-update failure and reloads filtered views on success.
- Fixed 2026-06-03: dish edit rehydrates base form state from base create/update response and, if featured-tag or customization sync then fails, keeps the merchant on the page with a visible recovery state and retryable save action.
- Inventory page uses backend response after save to rehydrate `sold_quantity`, `reserved_quantity`, `total_quantity`, and `available`.
- Inventory can change after the merchant page loads because payment success, order cancellation, reservation sync, no-show, and timeout workers can adjust sold/reserved quantities.
- No websocket or polling path was found that pushes inventory updates to the merchant inventory page; re-entry, pull refresh, or stale-window reload are the recovery paths.

## Frontend Draft And Backend Rehydration

- Dish list has no long-lived draft for status; it stores pending UI state and either commits local online status or restores from the previous list.
- Dish edit keeps a local form and customization draft. Base response rehydrates only the base dish state; featured tags and customizations are not merged back into one authoritative server response.
- Fixed 2026-06-03: `is_available` is an honest backend-truth field in the edit page; it is loaded from detail, editable for non-packaging dishes, and submitted back to the backend.
- Inventory rows keep draft text and numeric quantity separately; save rehydrates from backend response and resets draft fields to the saved backend total.

## Test Coverage Signals

Observed tests:

- `locallife/api/dish_test.go` covers packaging-dish offline rejection for single status update.
- `locallife/api/dish_test.go` and `locallife/db/sqlc/dish_test.go` cover batch status partial-row reporting from actual SQL-updated IDs.
- `locallife/db/sqlc/dish_test.go` covers create-dish transaction rollback when customization creation fails.
- `locallife/api/dish_test.go` and `locallife/db/sqlc/dish_test.go` cover featured-tag store-error propagation and transactional rollback.
- `locallife/api/inventory_test.go` covers inventory update, including create-when-missing.
- `locallife/api/inventory_test.go` and `locallife/db/sqlc/inventory_test.go` cover finite-total rejection when a merchant or direct store update would set total below sold plus reserved quantities.
- `locallife/api/inventory_test.go` covers direct inventory POST tenant-boundary denial for a foreign dish.
- `locallife/api/inventory_test.go` covers check-only success, no auth, foreign-dish denial, missing-row unlimited semantics, and insufficient inventory response.
- `locallife/db/sqlc/inventory_test.go` covers inventory stats sold-out/available classification using reserved quantity as committed stock.
- `weapp/scripts/check-merchant-dish-availability-contract.test.js` covers Mini Program load/submit/WXML binding for `is_available`.
- `weapp/scripts/check-merchant-dish-edit-partial-save-recovery.test.js` covers Mini Program partial-save recovery patch builders, failed-step tracking, and persistent TDesign warning wiring.
- `locallife/api/dish_test.go`, `locallife/logic/reservation_dishes_test.go`, and `locallife/db/sqlc/dish_test.go` cover public detail, reservation validation, search, scan menu, and recommendation readers excluding/rejecting unavailable dishes.
- SQL inventory tests and reservation inventory transaction tests cover lower-level decrement/reserve/release behavior.

Missing high-value tests:

- Backend single-workflow transaction coverage if dish base, featured tags, and customizations are later moved behind one endpoint.

## Gaps And Refactor Notes

- Fixed 2026-06-03: `is_available` is now treated as a real merchant-controlled temporary availability flag. Mini Program edit preserves/submits it, and public detail, scan menu, search, recommendation readers, and reservation validation honor it consistently.
- Fixed 2026-06-03: the frontend partial-save recovery is now explicit and retryable. A future backend single-workflow endpoint would still reduce the product workflow from three writes to one atomic operation.
- Fixed 2026-06-03: featured-tag updates now use a transaction-backed replace operation that propagates failures.
- Fixed 2026-06-03: batch status now uses actual SQL-returned updated IDs and no longer reports all prefiltered IDs as successful when a stale/deleted row is missed.
- Fixed 2026-06-03: `POST /v1/inventory/check` no longer mutates inventory and now aligns missing-row semantics with list, payment, and reservation paths.
- Fixed 2026-06-03: merchant inventory saves and the database constraint now prevent finite total inventory from falling below `sold + reserved`.
- Fixed 2026-06-03: inventory stats now classify finite rows with `sold + reserved >= total` as sold out and only treat rows with remaining uncommitted stock as available.
- Fixed 2026-06-03: unused Mini Program `batchUpdateDishStatus`, `checkInventory`, and `getInventoryStats` wrapper exposure was removed from all copied dish API files. Backend routes remain tested; future page binding should add a fresh task-owned wrapper and contract test instead of reusing stale copied surface.

## Branch Exhaustion

- Entry branches checked: Mini Program dish list/status toggle, dish edit/create, customization editor, featured tags, category adjacency, inventory page/save, unused batch/check/stats wrappers, public/search/scan readers, cart/order/reservation validators, and payment/reservation inventory writers. Flutter App has no dish/inventory management entry in `merchant_app/lib/features/**`.
- Request branches checked: dish CRUD/status/batch status/delete, customization GET/PUT, featured-tags PUT, inventory list/update/check/stats, order payment decrement, order cancellation restore, reservation reserve/release/sync/no-show/timeout inventory paths, and public dish/menu readers.
- Backend state branches checked: `dishes.is_online`, fixed merchant-controlled `is_available`, `is_packaging`, soft delete, category, tags, customization groups/options, `daily_inventory.total_quantity/sold_quantity/reserved_quantity`, unlimited sentinel, missing-row creation, and reserved inventory accounting.
- Async branches checked: merchant edits are synchronous; inventory changes asynchronously from payment success, order cancellation, reservation payment/sync, reservation timeout, no-show, and release workers. No merchant inventory websocket/polling path was found.
- Failure/retry branches checked: per-row status pending rollback, fixed batch-status partial SQL update reporting, fixed dish edit partial-save recovery, fixed featured-tag transactional failure, inventory last-write-wins, fixed direct inventory POST tenant denial, fixed read-only inventory check semantics, fixed total below sold+reserved rejection, and fixed `is_available` read/write inconsistency.
- Reader/consumer branches checked: dish list/edit, inventory page, public merchant menu, search, scan-table menu, cart, direct order, reservation validation, packaging policy, and inventory stats.
- Authorization/tenant branches checked: owner/manager/chef dish and inventory routes, dish ownership checks for most writes, inventory PUT ownership validation on missing-row create, direct inventory POST ownership check added 2026-06-02, and customer readers relying on persisted state.
- Zombie/unreachable branches checked: legacy `UpdateDishAvailability` SQL remains without a runtime caller found now that edit/update handles the merchant-controlled field; fixed 2026-06-03 Mini Program unused wrapper exposure for batch status/inventory check/stats; single dish-edit submit is split across multiple backend writes.
- Test-proof gaps checked: existing tests cover packaging offline rejection, `is_available` product contract, dish-edit partial-save recovery, batch status partial SQL update reporting, create rollback, featured-tag transactional rollback, inventory update/check-only/decrement lower layers, direct POST tenant denial, finite inventory total guard, reserved-quantity stats classification, and reserve/release lower layers. Backend single-workflow transaction proof remains relevant only if the workflow is collapsed into one endpoint later.
