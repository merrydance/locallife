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

Current implementation has good combo create/update transaction boundaries, but it has drift around child dish availability, public readers, combo-tag creation permission, category sort-only persistence, and category unlink effects on existing dishes.

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

8. `resolveComboSummary` normalizes selected dishes, verifies the dish exists and belongs to the current merchant, and snapshots price/customization totals. It does not check `dish.is_online` or `dish.is_available`.
   Evidence: `locallife/api/combo.go:242`, `locallife/api/combo.go:257`, `locallife/api/combo.go:265`, `locallife/api/combo.go:272`, `locallife/api/combo.go:276`.

9. `CreateComboSetTx` and `UpdateComboSetTx` own the durable combo write boundary. Create writes combo, dishes, and tags in one transaction; update writes combo and atomically replaces dish/tag associations when supplied.
   Evidence: `locallife/api/combo.go:337`, `locallife/api/combo.go:777`, `locallife/db/sqlc/tx_combo.go:1`, `locallife/db/sqlc/tx_combo.go:77`.

10. `toggleComboOnline` only updates `combo_sets.is_online`; it does not revalidate child dishes before publishing.
    Evidence: `locallife/api/combo.go:822`, `locallife/api/combo.go:847`, `locallife/api/combo.go:863`, `locallife/db/query/combo.sql:112`.

11. `deleteComboSet` soft-deletes the combo. Association rows are not physically removed by the SQL update, despite the handler comment saying cascade delete.
    Evidence: `locallife/api/combo.go:899`, `locallife/api/combo.go:931`, `locallife/api/combo.go:932`, `locallife/db/query/combo.sql:119`.

12. Direct combo-dish add/remove wrappers and routes exist, but the current Mini Program edit page uses full combo create/update. Direct add only accepts `dish_id`, defaults quantity to 1, and does not support fixed customization payloads.
    Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:909`, `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:924`, `locallife/api/combo.go:963`, `locallife/api/combo.go:1020`, `locallife/api/combo.go:1056`.

13. Public combo detail checks only combo online status before returning details. It computes merchant `is_open`, but does not reuse the public storefront loader or filter child dish availability.
    Evidence: `locallife/api/combo.go:485`, `locallife/api/combo.go:503`, `locallife/api/combo.go:516`, `locallife/api/combo.go:522`.

14. Public merchant combo list does call `loadPublicStorefrontMerchant`, then reads `GetMerchantOnlineCombos`; the SQL returns online combos and child dishes without child `deleted_at`, `is_online`, or `is_available` filters.
    Evidence: `locallife/api/merchant.go:1566`, `locallife/api/merchant.go:1570`, `locallife/db/query/merchant_stats.sql:300`, `locallife/db/query/merchant_stats.sql:320`, `locallife/db/query/merchant_stats.sql:329`.

15. Scan-table menu lists online combos and exposes `IsAvailable = combo.IsOnline`; the backing SQL does not inspect child dishes.
    Evidence: `locallife/api/scan.go:291`, `locallife/api/scan.go:299`, `locallife/api/scan.go:303`, `locallife/db/query/combo.sql:291`, `locallife/db/query/combo.sql:310`.

16. Cart add/update, direct order creation, and reservation pre-order validation check combo ownership and combo `is_online`, but not child dish availability.
    Evidence: `locallife/logic/cart_items.go:109`, `locallife/logic/cart_items.go:120`, `locallife/logic/cart_items.go:290`, `locallife/logic/cart_items.go:295`, `locallife/logic/order_items.go:75`, `locallife/logic/order_items.go:85`, `locallife/logic/reservation.go:798`, `locallife/logic/reservation.go:809`.

17. The merchant category page loads merchant categories and global categories, then creates, links, renames, or deletes categories through `DishManagementService` wrappers.
    Evidence: `weapp/miniprogram/pages/merchant/dishes/categories/index.ts:54`, `weapp/miniprogram/pages/merchant/dishes/categories/index.ts:72`, `weapp/miniprogram/pages/merchant/dishes/categories/index.ts:225`, `weapp/miniprogram/pages/merchant/dishes/categories/index.ts:256`, `weapp/miniprogram/pages/merchant/dishes/categories/index.ts:291`, `weapp/miniprogram/pages/merchant/dishes/categories/index.ts:321`.

18. Backend category routes are under owner/manager/chef middleware. Create/link uses `CreateDishCategory` then `LinkMerchantDishCategory` as two separate writes outside an explicit transaction.
    Evidence: `locallife/api/server.go:787`, `locallife/api/server.go:792`, `locallife/db/query/dish.sql:9`, `locallife/db/query/dish.sql:17`.

19. Category rename uses `RenameMerchantDishCategoryTx`, which creates/reuses a global category, links it, migrates merchant dishes to the new category, and unlinks the old category in one transaction.
    Evidence: `locallife/api/dish.go:281`, `locallife/api/dish.go:284`, `locallife/db/sqlc/tx_dish_category.go:25`, `locallife/db/sqlc/tx_dish_category.go:55`, `locallife/db/sqlc/tx_dish_category.go:64`.

20. Category sort-only update computes the new sort order and returns it, but does not call `UpdateMerchantDishCategoryOrder`.
    Evidence: `locallife/api/dish.go:273`, `locallife/api/dish.go:277`, `locallife/api/dish.go:297`, `locallife/api/dish.go:302`, `locallife/db/query/dish.sql:43`.

21. Category delete only unlinks `merchant_dish_categories`; it does not migrate or clear `dishes.category_id` for existing dishes.
    Evidence: `locallife/api/dish.go:328`, `locallife/api/dish.go:350`, `locallife/api/dish.go:363`, `locallife/db/query/dish.sql:54`, `locallife/db/query/dish.sql:58`.

22. Combo edit exposes tag creation through `TagService.createTag({ type: 'combo' })`, but backend `POST /v1/tags` is admin-only.
    Evidence: `weapp/miniprogram/pages/merchant/combos/edit/index.ts:360`, `weapp/miniprogram/pages/merchant/combos/edit/index.ts:373`, `weapp/miniprogram/pages/merchant/_main_shared/api/dish.ts:566`, `locallife/api/server.go:778`, `locallife/api/server.go:782`.

## Reverse-Reference Findings

- Combo child dish visibility is stricter in the Mini Program picker than in backend writers and customer-facing readers.
- `GetComboSetWithDetails`, `ListComboSetsByMerchant`, `ListOnlineCombosByMerchant`, `SearchCombosGlobal`, and `GetComboMemberImagesByCombos` join child dishes by non-deleted status at most; they do not consistently filter child `is_online` or `is_available`.
- Direct combo-dish add/remove endpoints and Mini Program wrappers appear to be legacy/narrow paths relative to the current full `PUT /v1/combos/:id` edit workflow.
- Category rename is transaction-protected, but category create/link and delete are not symmetrical with rename.
- Category sort-order SQL exists but the sort-only handler branch does not invoke it.
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
- Combo create/update/add validates child dish merchant ownership; combo get/update/toggle/delete/remove validates combo ownership.
- Category create/list/update/delete resolve the current merchant and validate merchant-category links before update/delete.
- Customer-facing combo readers must not rely on Mini Program picker filtering; backend readers and validators need their own child-dish orderability checks if that is the product contract.

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
- Downstream customer carts/orders/reservations revalidate combo `is_online` at action time, but not child dish availability.

## Frontend Draft And Backend Rehydration

- Combo list has no long-lived draft; it stores per-row pending flags and rehydrates by list reload.
- Combo edit keeps a local selected-dish/tag/price/name draft. After save it applies limited persisted combo fields, then navigates back and relies on list reload.
- Already selected dishes remain visible in edit even if no longer online/available, making it possible for a merchant to resave a stale child dish selection.
- Category page has no long-lived draft; it uses modal/popup inputs and reloads from backend truth after each operation.

## Test Coverage Signals

Observed tests/signals:

- Combo list uses real backend `total`, unlike several other merchant list flows.
- Existing order/cart/reservation tests cover combo offline rejection, but only at `combo_sets.is_online` level.
- Category rename has an explicit transaction implementation.

Missing high-value tests:

- Combo create/update/toggle rejects publishing or retaining offline/unavailable child dishes if that is the intended product invariant.
- Public combo detail, public merchant combos, scan menu, cart, direct order, and reservation all agree on child dish availability.
- Category sort-only update persists `merchant_dish_categories.sort_order`.
- Category delete either migrates/clears existing dish categories or deliberately exposes an uncategorized state in every reader.
- Merchant combo tag creation is hidden or backed by a merchant-authorized backend route.
- Direct combo-dish add/remove endpoints are either covered by current product semantics or retired/quarantined.

## Gaps And Refactor Notes

- Decide the combo child-dish availability contract. If combos must only contain orderable dishes, enforce it in create/update/toggle and customer validators; if combos can contain temporarily unavailable children, product copy and public readers should expose that honestly.
- Align public combo readers with `loadPublicStorefrontMerchant` and child availability checks, especially public combo detail and scan menu.
- Fix category sort-only update to persist through `UpdateMerchantDishCategoryOrder` or remove sort controls until supported.
- Decide category delete semantics for dishes currently assigned to that category: block delete, migrate to uncategorized, or explicitly allow hidden category ids and update readers.
- Remove merchant-facing tag mutation controls for combo tags or add a merchant-authorized tag creation contract with tests.
- Retire direct combo-dish add/remove wrappers/routes if the full combo update path is the only supported workflow.

## Branch Exhaustion

- Entry branches checked: Mini Program combo list, combo edit/create, child dish picker, combo status toggle/delete, combo tags, dish category page, sort-only category update, direct combo-dish wrappers, public combo/detail/search/scan readers, cart/order/reservation combo validators. Flutter App has no combo/category management entry in `merchant_app/lib/features/**`.
- Request branches checked: combo CRUD/status/delete/detail/list, full combo update, direct add/remove combo-dish routes, combo tag wrappers, category create/link/update/delete/list/sort, public online combo readers, search, cart add/update, direct order creation, and reservation item validation.
- Backend state branches checked: `combo_sets.is_online/deleted_at`, child `combo_dishes`, fixed customizations and price snapshots, combo tags, global `dish_categories`, merchant category links/sort order, `dishes.category_id`, and child dish online/availability state.
- Async branches checked: combo/category writes are synchronous only; no worker, scheduler, websocket, or outbox repair was found. Downstream cart/order/reservation revalidation occurs synchronously when a customer acts.
- Failure/retry branches checked: local duplicate guards, last-write-wins edits, stale selected child dish resave, category sort-only success-without-persist, category delete leaving dish category ids, merchant tag create backend admin denial, direct add/remove payload drift, and repeated direct add without durable idempotency.
- Reader/consumer branches checked: merchant combo/category pages, public storefront combo list/detail, scan menu, cart, direct order, reservation prepay, search, and child image enrichment.
- Authorization/tenant branches checked: owner/manager/chef routes, combo ownership checks, child dish ownership validation on create/update/add, category merchant-link validation, and customer readers needing backend-side orderability checks rather than trusting picker filters.
- Zombie/unreachable branches checked: direct combo-dish routes/wrappers are stale relative to full edit workflow; merchant-facing combo tag mutation calls admin-only backend routes; category sort SQL exists but sort branch does not invoke it; category delete does not reconcile `dishes.category_id`.
- Test-proof gaps checked: existing signals cover combo offline rejection at combo-set level and transactional category rename. Missing proof remains for child dish availability contract, all public/order readers agreeing on child orderability, category sort persistence, category delete semantics, merchant tag creation contract, and retirement/coverage for direct add/remove routes.
