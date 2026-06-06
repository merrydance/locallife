# Merchant Combo And Catalog Slice

Status: merchant-state flow slice created
Risk class: G2 - merchant-controlled combo/catalog state affects customer menu visibility, cart/order/reservation validation, and dish/category organization
Scope: merchant combo list/edit and dish-category pages -> combo/category APIs -> combo/category durable state -> public menu/search/cart/order/reservation readers

## Variant Coverage

This slice covers:

- Merchant Mini Program combo list, combo create/update/delete, and combo online toggle.
- Merchant Mini Program dish category create/link/rename/delete page.
- Backend combo and dish-category route groups, handlers, transactions, and SQL writes.
- Downstream readers for public merchant combo list, public combo detail, scan-table menu, cart, direct order creation, and reservation pre-order validation.

This slice does not fully cover:

- Base dish create/update/status/inventory, already captured by `merchant-dish-status-and-inventory`.
- Marketing price/promotion semantics for combos beyond combo base price.
- Order/refund lifecycle after a combo item has already been accepted into an order.

## Product Invariant

Combos and categories should preserve one coherent catalog truth:

- A merchant should only publish a combo whose child dishes are owned by that merchant and currently orderable under the same availability rules customers see.
- Public menu/search/cart/order/reservation readers should agree on whether a combo is orderable.
- Category rename/delete should not leave dishes pointing at merchant-hidden categories unless that is a deliberate uncategorized state.
- Tag/category creation controls visible in the merchant Mini Program should match backend permissions.
- Re-entry after save/delete should reload backend truth, not rely on local draft state.

Current implementation has good combo create/update transaction boundaries. Fixed 2026-06-06: online-combo write entrypoints, public combo detail/list/scan/search/recommendation readers, and cart/order/reservation validators now fail closed when combo children are empty, missing, soft-deleted, offline, or unavailable. Category sort-only persistence was also fixed on 2026-06-06. Remaining drift is around combo-tag creation permission, category unlink effects on existing dishes, direct combo-dish endpoint shape, and combo delete association/comment semantics.

## Primary Forward Chain

1. Merchant combo list loads `GET /v1/combos` with optional `is_online`, uses backend `total` to compute `hasMore`, and preserves current content on refresh failure.
   Evidence: `weapp/miniprogram/pages/merchant/combos/index.ts:175`, `weapp/miniprogram/pages/merchant/combos/index.ts:212`, `weapp/miniprogram/pages/merchant/combos/index.ts:223`, `weapp/miniprogram/pages/merchant/combos/index.ts:228`.

2. Combo list toggles online status with per-row pending state and calls `PUT /v1/combos/:id/online`.
   Evidence: `weapp/miniprogram/pages/merchant/combos/index.ts:293`, `weapp/miniprogram/pages/merchant/combos/index.ts:304`, `weapp/miniprogram/pages/merchant/combos/index.ts:311`, `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:935`.

3. Combo list soft-deletes a combo through `DELETE /v1/combos/:id`, then removes it locally from the list.
   Evidence: `weapp/miniprogram/pages/merchant/combos/index.ts:392`, `weapp/miniprogram/pages/merchant/combos/index.ts:415`, `weapp/miniprogram/pages/merchant/combos/index.ts:416`, `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:898`.

4. Combo edit loads all dishes, existing combo detail, and combo tags in parallel. Tag-load failure is downgraded to a warning.
   Evidence: `weapp/miniprogram/pages/merchant/combos/edit/index.ts:119`, `weapp/miniprogram/pages/merchant/combos/edit/index.ts:152`, `weapp/miniprogram/pages/merchant/combos/edit/index.ts:169`, `weapp/miniprogram/pages/merchant/combos/edit/index.ts:170`.

5. Combo edit only shows dishes that are both `is_online` and `is_available`, while preserving already selected dishes even if they no longer satisfy that filter.
   Evidence: `weapp/miniprogram/pages/merchant/combos/edit/index.ts:583`, `weapp/miniprogram/pages/merchant/combos/edit/index.ts:584`, `weapp/miniprogram/pages/merchant/combos/edit/index.ts:585`.

6. Combo edit submit builds one combo payload with selected dishes, quantities, fixed customizations, selected tags, price, and online status; create and update use `POST /v1/combos` and `PUT /v1/combos/:id`.
   Evidence: `weapp/miniprogram/pages/merchant/combos/edit/index.ts:485`, `weapp/miniprogram/pages/merchant/combos/edit/index.ts:509`, `weapp/miniprogram/pages/merchant/combos/edit/index.ts:515`, `weapp/miniprogram/pages/merchant/combos/edit/index.ts:524`.

7. Backend combo routes are under merchant staff middleware for owner/manager/chef.
   Evidence: `locallife/api/server.go:812`, `locallife/api/server.go:814`, `locallife/api/server.go:818`, `locallife/api/server.go:827`.

8. `resolveComboSummary` normalizes selected dishes, verifies the dish exists and belongs to the current merchant, snapshots price/customization totals, and rejects offline/unavailable child dishes when the caller is publishing or keeping a combo online.
   Evidence: `locallife/api/combo.go:265`, `locallife/api/combo.go:281`, `locallife/api/combo.go:289`, `locallife/api/combo.go:296`, `locallife/api/combo.go:299`.

9. `CreateComboSetTx` and `UpdateComboSetTx` own the durable combo write boundary. Create writes combo, dishes, and tags in one transaction; update writes combo and atomically replaces dish/tag associations when supplied. The API calls orderability validation before entering these transaction boundaries for online combos.
   Evidence: `locallife/api/combo.go:342`, `locallife/api/combo.go:366`, `locallife/api/combo.go:788`, `locallife/api/combo.go:816`, `locallife/db/sqlc/tx_combo.go:1`, `locallife/db/sqlc/tx_combo.go:77`.

10. `toggleComboOnline` validates existing combo child dishes before setting `combo_sets.is_online=true`.
    Evidence: `locallife/api/combo.go:901`, `locallife/api/combo.go:902`, `locallife/api/combo.go:914`, `locallife/db/query/combo.sql:112`.

11. `deleteComboSet` soft-deletes the combo. Association rows are not physically removed by the SQL update, despite the handler comment saying cascade delete.
    Evidence: `locallife/api/combo.go:899`, `locallife/api/combo.go:931`, `locallife/api/combo.go:932`, `locallife/db/query/combo.sql:119`.

12. Direct combo-dish add/remove wrappers and routes exist, but the current Mini Program edit page uses full combo create/update. Direct add only accepts `dish_id`, defaults quantity to 1, and does not support fixed customization payloads; for online combos it now validates existing and newly added child dish orderability before insert.
    Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:909`, `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:924`, `locallife/api/combo.go:1014`, `locallife/api/combo.go:1069`, `locallife/api/combo.go:1079`.

13. Public combo detail still uses `GetComboSetWithDetails` so merchant/admin detail behavior remains inspectable, but the public handler now rejects non-online combos and wraps the response with `validateExistingComboDishesOrderable(..., requireNonEmpty=true)`. Business-invalid child state maps to `404`, while store/dependency failures still go through `internalError`.
    Evidence: `locallife/api/combo.go:535`, `locallife/api/combo.go:546`, `locallife/api/combo.go:552`, `locallife/api/combo.go:555`, `locallife/api/combo.go:558`.

14. Public merchant combo list calls `loadPublicStorefrontMerchant`, then reads `GetMerchantOnlineCombos`; the SQL now requires at least one orderable child and no missing, soft-deleted, offline, or unavailable child. It also only emits orderable child rows and original-price components.
    Evidence: `locallife/api/merchant.go:1566`, `locallife/api/merchant.go:1570`, `locallife/db/query/merchant_stats.sql:300`, `locallife/db/query/merchant_stats.sql:310`, `locallife/db/query/merchant_stats.sql:320`, `locallife/db/query/merchant_stats.sql:341`, `locallife/db/query/merchant_stats.sql:350`.

15. Scan-table menu still maps returned combos into menu items, but `ListOnlineCombosByMerchant` now only returns online combos whose children all satisfy the child-orderability invariant, so `IsAvailable = combo.IsOnline` is backed by SQL filtering.
    Evidence: `locallife/api/scan.go:291`, `locallife/api/scan.go:299`, `locallife/api/scan.go:303`, `locallife/db/query/combo.sql:368`, `locallife/db/query/combo.sql:390`, `locallife/db/query/combo.sql:399`.

16. Cart add/update, direct order creation, and reservation pre-order validation check combo ownership, combo `is_online`, and `validateComboChildDishesOrderable`. The shared helper reads `ListComboDishOrderability`, treats empty children as invalid, and fails closed on missing, soft-deleted, offline, or unavailable child dishes.
    Evidence: `locallife/logic/combo_orderability.go:10`, `locallife/logic/combo_orderability.go:15`, `locallife/logic/combo_orderability.go:23`, `locallife/logic/cart_items.go:123`, `locallife/logic/cart_items.go:301`, `locallife/logic/order_items.go:88`, `locallife/logic/reservation.go:815`, `locallife/db/query/combo.sql:158`.

17. The merchant category page loads merchant categories and global categories, then creates, links, renames, or deletes categories through `DishManagementService` wrappers.
    Evidence: `weapp/miniprogram/pages/merchant/dishes/categories/index.ts:54`, `weapp/miniprogram/pages/merchant/dishes/categories/index.ts:72`, `weapp/miniprogram/pages/merchant/dishes/categories/index.ts:225`, `weapp/miniprogram/pages/merchant/dishes/categories/index.ts:256`, `weapp/miniprogram/pages/merchant/dishes/categories/index.ts:291`, `weapp/miniprogram/pages/merchant/dishes/categories/index.ts:321`.

18. Backend category routes are under owner/manager/chef middleware. Create/link uses `CreateDishCategory` then `LinkMerchantDishCategory` as two separate writes outside an explicit transaction.
    Evidence: `locallife/api/server.go:787`, `locallife/api/server.go:792`, `locallife/db/query/dish.sql:9`, `locallife/db/query/dish.sql:17`.

19. Category rename uses `RenameMerchantDishCategoryTx`, which creates/reuses a global category, links it, migrates merchant dishes to the new category, and unlinks the old category in one transaction.
    Evidence: `locallife/api/dish.go:281`, `locallife/api/dish.go:284`, `locallife/db/sqlc/tx_dish_category.go:25`, `locallife/db/sqlc/tx_dish_category.go:55`, `locallife/db/sqlc/tx_dish_category.go:64`.

20. Fixed 2026-06-06: category sort-only update calls `UpdateMerchantDishCategoryOrder` after merchant-category ownership is verified, and the response uses the persisted `merchant_dish_categories.sort_order`.
    Evidence: `locallife/api/dish.go:273`, `locallife/api/dish.go:297`, `locallife/api/dish.go:298`, `locallife/api/dish.go:307`, `locallife/db/query/dish.sql:43`, `locallife/api/dish_test.go:255`.

21. Category delete only unlinks `merchant_dish_categories`; it does not migrate or clear `dishes.category_id` for existing dishes.
    Evidence: `locallife/api/dish.go:328`, `locallife/api/dish.go:350`, `locallife/api/dish.go:363`, `locallife/db/query/dish.sql:54`, `locallife/db/query/dish.sql:58`.

22. Combo edit exposes tag creation through `TagService.createTag({ type: 'combo' })`, but backend `POST /v1/tags` is admin-only.
    Evidence: `weapp/miniprogram/pages/merchant/combos/edit/index.ts:360`, `weapp/miniprogram/pages/merchant/combos/edit/index.ts:373`, `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:566`, `locallife/api/server.go:778`, `locallife/api/server.go:782`.

## Reverse-Reference Findings

- Fixed 2026-06-06: combo child dish visibility/orderability is now enforced by online-combo write entrypoints, public combo detail, public list/scan/search/recommendation SQL, and cart/order/reservation validators for historical or post-publish child dish drift.
- `ListComboDishOrderability` is the dedicated fail-closed dependency query for child existence, soft delete, online, and availability checks. `GetComboSetWithDetails` intentionally remains the merchant/admin detail reader; public detail wraps it with orderability validation instead of globally hiding problematic management state.
- `ListOnlineCombosByMerchant`, `GetMerchantOnlineCombos`, `GetCombosByIDs`, `GetCombosWithMerchantByIDs`, `SearchComboIDsGlobal`, `SearchCombosGlobal`, `CountSearchCombosGlobal`, `GetPopularCombos`, and `GetComboMemberImagesByCombos` now align on the child-orderability invariant for customer-facing reads and images.
- Direct combo-dish add/remove endpoints and Mini Program wrappers appear to be legacy/narrow paths relative to the current full `PUT /v1/combos/:id` edit workflow.
- Category rename is transaction-protected, but category create/link and delete are not symmetrical with rename.
- Fixed 2026-06-06: category sort-order SQL is now invoked by the sort-only handler branch and covered by API/sqlc regressions.
- Combo tag create/delete wrappers exist on the merchant shared API surface, but backend mutation routes require admin.

## SQL And Durable State Boundaries

- `combo_sets`: owns combo name, description, original price, combo price, online flag, image, soft-delete state.
- `combo_dishes`: owns selected child dish ids, quantities, base price snapshots, fixed customizations, and customization extra price.
- `combo_tags`: owns selected combo tag ids.
- `dish_categories`: global category name catalog.
- `merchant_dish_categories`: merchant-specific category links and sort order.
- `dishes.category_id`: durable dish classification; rename migrates it, delete currently leaves it untouched.

## Trust, Authorization, And Tenant Checks

- Combo and category route groups use `MerchantStaffMiddleware("owner", "manager", "chef")`.
- Combo create/update/add validates child dish merchant ownership; online create/update/toggle/add also validates child dish orderability; combo get/update/toggle/delete/remove validates combo ownership.
- Category create/list/update/delete resolve the current merchant and validate merchant-category links before update/delete.
- Fixed 2026-06-06: customer-facing combo readers no longer rely on Mini Program picker filtering. Public SQL and action validators now enforce backend-side child-dish orderability.

## Idempotency And Duplicate-Submit Checks

- Combo list uses per-row status/delete pending flags.
- Combo edit uses `submitting` to block duplicate create/update locally.
- Combo create/update transactions converge each single request, but no request idempotency key or optimistic version exists; concurrent edits are last-write-wins.
- Category page has a single `submitting` flag for create/link/edit and per-delete pending id. Category writes are last-write-wins or direct unlink.
- Direct add/remove combo-dish endpoints are not idempotency-keyed; repeated add can create duplicate rows unless a DB constraint exists elsewhere.

## Recovery And Async Convergence Paths

- Combo/category writes are synchronous; no websocket, worker, scheduler, or outbox path was found.
- Combo list reloads after filtered toggle, navigates back from edit after applying local persisted state and asking previous page to reload.
- Category page reloads after create/link/edit/delete.
- Downstream customer carts/orders/reservations revalidate combo `is_online` and child dish orderability at action time, so historical or later child availability drift is blocked synchronously when a customer acts.

## Frontend Draft And Backend Rehydration

- Combo list has no long-lived draft; it stores per-row pending flags and rehydrates by list reload.
- Combo edit keeps a local selected-dish/tag/price/name draft. After save it applies limited persisted combo fields, then navigates back and relies on list reload.
- Already selected dishes remain visible in edit even if no longer online/available, but online saves now reject stale child dish selections; offline drafts can still retain them until publish.
- Category page has no long-lived draft; it uses modal/popup inputs and reloads from backend truth after each operation.

## Test Coverage Signals

Observed tests/signals:

- Combo list uses real backend `total`, unlike several other merchant list flows.
- Existing order/cart/reservation tests cover combo offline rejection at `combo_sets.is_online` level.
- API tests now cover online combo create/update/toggle/direct-add rejection for offline/unavailable child dishes, public combo detail rejection for unavailable child dishes, and 500 mapping for existing-child lookup failures.
- Logic tests now cover cart add/update, direct order calculation, and reservation item validation rejecting unavailable combo child dishes.
- SQLC tests now cover public combo queries excluding missing, soft-deleted, offline, or unavailable child dishes across public list/search/recommendation readers.
- Category rename has an explicit transaction implementation.

Missing high-value tests:

- Fixed 2026-06-06: combo create/update/toggle/direct-add rejects publishing or retaining missing, soft-deleted, offline, unavailable, or empty child selections on online write entrypoints.
- Fixed 2026-06-06: public combo detail, public merchant combos, scan menu, search/recommendation readers, cart, direct order, and reservation all agree on child dish availability/orderability.
- Fixed 2026-06-06: category sort-only update persists `merchant_dish_categories.sort_order`.
- Category delete either migrates/clears existing dish categories or deliberately exposes an uncategorized state in every reader.
- Merchant combo tag creation is hidden or backed by a merchant-authorized backend route.
- Direct combo-dish add/remove endpoints are either covered by current product semantics or retired/quarantined.

## Gaps And Refactor Notes

- Fixed 2026-06-06: online combo create/update/toggle/direct-add, public combo detail/list/scan/search/recommendation readers, and cart/order/reservation validators now enforce orderable child dishes. If product later wants degraded combo visibility, it should be introduced as an explicit new contract with copy and tests rather than by weakening the fail-closed default.
- Public combo detail still uses the merchant/admin detail query, then applies public orderability validation. Keep that boundary so merchants can inspect and repair invalid combos while customers cannot order them.
- Fixed 2026-06-06: category sort-only update persists through `UpdateMerchantDishCategoryOrder`; remaining category work is delete semantics and reader copy/contracts.
- Decide category delete semantics for dishes currently assigned to that category: block delete, migrate to uncategorized, or explicitly allow hidden category ids and update readers.
- Remove merchant-facing tag mutation controls for combo tags or add a merchant-authorized tag creation contract with tests.
- Retire direct combo-dish add/remove wrappers/routes if the full combo update path is the only supported workflow.

## Branch Exhaustion

- Entry branches checked: Mini Program combo list, combo edit/create, child dish picker, combo status toggle/delete, combo tags, dish category page, sort-only category update, direct combo-dish wrappers, public combo/detail/search/scan readers, cart/order/reservation combo validators. Flutter App has no combo/category management entry in `merchant_app/lib/features/**`.
- Request branches checked: combo CRUD/status/delete/detail/list, full combo update, direct add/remove combo-dish routes, combo tag wrappers, category create/link/update/delete/list/sort, public online combo readers, search, cart add/update, direct order creation, and reservation item validation.
- Backend state branches checked: `combo_sets.is_online/deleted_at`, child `combo_dishes`, fixed customizations and price snapshots, combo tags, global `dish_categories`, merchant category links/sort order, `dishes.category_id`, and child dish online/availability state.
- Async branches checked: combo/category writes are synchronous only; no worker, scheduler, websocket, or outbox repair was found. Downstream cart/order/reservation revalidation occurs synchronously when a customer acts.
- Failure/retry branches checked: local duplicate guards, last-write-wins edits, stale selected child dish online-save rejection, public/action child-orderability rejection, existing-child lookup failure mapping, fixed category sort-only persistence, category delete leaving dish category ids, merchant tag create backend admin denial, direct add/remove payload drift, and repeated direct add without durable idempotency.
- Reader/consumer branches checked: merchant combo/category pages, public storefront combo list/detail, scan menu, cart, direct order, reservation prepay, search, and child image enrichment.
- Authorization/tenant branches checked: owner/manager/chef routes, combo ownership checks, child dish ownership validation on create/update/add, online-combo child dish orderability validation on create/update/toggle/add, category merchant-link validation, and customer readers/action validators enforcing backend-side orderability checks rather than trusting picker filters.
- Zombie/unreachable branches checked: direct combo-dish routes/wrappers are stale relative to full edit workflow; merchant-facing combo tag mutation calls admin-only backend routes; category sort branch now invokes its existing SQL; category delete does not reconcile `dishes.category_id`.
- Test-proof gaps checked: existing signals cover combo offline rejection at combo-set level, online-combo write-side child dish orderability, public/read/action child orderability, transactional category rename, and category sort-only persistence. Missing proof remains for category delete semantics, merchant tag creation contract, and retirement/coverage for direct add/remove routes.
