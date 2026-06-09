# Merchant Marketing Rules Slice

Status: merchant-state flow slice created
Risk class: G2 - merchant configuration changes checkout totals, voucher lifecycle, recharge bonus resolution, delivery-fee discounts, and stacking semantics
Scope: Mini Program marketing rule pages -> merchant rule APIs -> durable rule tables -> public merchant promotion readers, cart/order preview, direct order creation, voucher transactions, membership recharge, and voucher expiry cleanup

## Variant Coverage

This slice covers:

- Merchant voucher list/edit/toggle/delete pages under `weapp/miniprogram/pages/merchant/vouchers`.
- Merchant recharge-rule list/edit/toggle/delete pages under `weapp/miniprogram/pages/merchant/settings/recharge-rules`.
- Merchant delivery-promotion list/edit/toggle/delete pages under `weapp/miniprogram/pages/merchant/delivery-promotions`.
- Merchant discount-rule list/edit/toggle/delete pages under `weapp/miniprogram/pages/merchant/discount-rules`.
- Backend merchant CRUD APIs for vouchers, recharge rules, delivery promotions, and discount rules.
- Customer-facing reads that consume the changed state: public merchant promotions/detail, cart/order preview, direct order creation, delivery fee calculation, membership recharge rule resolution, user voucher claim/use, order cancellation rollback, and voucher expiry cleanup.

This slice does not cover:

- Merchant membership settings, which are captured in `merchant-membership-settings`.
- Manual member balance adjustment and merchant member recharge operation UI, except where recharge rules affect bonus resolution.
- Platform/operator promotion management, if any future path is introduced.
- Payment provider execution after order total calculation.

## Product Invariant

Merchant marketing rules must converge to one backend truth before they influence customer-visible prices:

- A merchant should not be able to persist a rule through update that create would reject.
- Customer preview and final order creation should apply the same effective rule semantics for discount stacking, voucher eligibility, delivery-fee discount, and membership balance compatibility.
- Product decision 2026-06-10: disabling or soft-deleting a voucher template must block new preview/order use; orders already created are not retroactively changed.
- Product decision 2026-06-10: recharge rules should be deactivated/disabled, not physically deleted, so historical recharge provenance remains auditable.
- Pagination metadata on merchant rule pages should be real backend truth, not a current-page count that requires client probing.

Fixed 2026-06-08: voucher quantity update now returns a stable 400 before SQL write when `total_quantity < claimed_quantity`; discount-rule and delivery-promotion updates now merge existing rule values with partial PATCH fields and apply create-level value validation before writing; discount `/applicable` plus `/best` route reachability is locked by a focused API route test; voucher and discount management list `total` now returns the full matched backend count. Fixed 2026-06-09: order preview now passes `order_type` into the typed cart lookup, takeout cart preview/order preview/direct order creation now have focused parity proof for subtotal, merchant discount, voucher amount, delivery fee discount, and persisted total under the same marketing stack, and delivery promotions now have DB constraints for positive amounts, `discount_amount <= min_order_amount`, and `valid_until > valid_from`. Product decisions for voucher disable/delete and recharge-rule deactivation were recorded on 2026-06-10; implementation still needs to align runtime behavior and tests.

## Primary Forward Chain

1. Merchant dashboard/config exposes marketing entries around membership/recharge, vouchers, delivery promotions, and discount rules.
   Evidence: `weapp/miniprogram/pages/merchant/_utils/merchant-dashboard-view.ts:188`, `weapp/miniprogram/pages/merchant/config/index.ts:61`.

2. Voucher list page syncs current merchant context, loads paged vouchers, still has a lookahead compatibility fallback, and preserves old data on refresh failure. Backend `total` is now full matched count.
   Evidence: `weapp/miniprogram/pages/merchant/vouchers/index.ts:159`, `weapp/miniprogram/pages/merchant/vouchers/index.ts:227`, `weapp/miniprogram/pages/merchant/vouchers/index.ts:251`, `weapp/miniprogram/pages/merchant/vouchers/index.ts:346`.

3. Voucher list toggles `is_active` and deletes templates through backend responses, then updates local presentation and reloads after delete.
   Evidence: `weapp/miniprogram/pages/merchant/vouchers/index.ts:503`, `weapp/miniprogram/pages/merchant/vouchers/index.ts:526`, `weapp/miniprogram/pages/merchant/vouchers/index.ts:583`, `weapp/miniprogram/pages/merchant/vouchers/index.ts:599`.

4. Voucher edit page loads detail by scanning merchant voucher pages, validates local form fields, and submits full create/update payloads.
   Evidence: `weapp/miniprogram/pages/merchant/vouchers/edit/index.ts:228`, `weapp/miniprogram/pages/merchant/vouchers/edit/index.ts:241`, `weapp/miniprogram/pages/merchant/vouchers/edit/index.ts:364`, `weapp/miniprogram/pages/merchant/vouchers/edit/index.ts:420`.

5. Voucher wrapper maps list/create/update/delete calls to `/v1/merchants/:id/vouchers`.
   Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/coupon.ts:360`, `weapp/miniprogram/pages/merchant/_main_shared/api/coupon.ts:376`, `weapp/miniprogram/pages/merchant/_main_shared/api/coupon.ts:386`, `weapp/miniprogram/pages/merchant/_main_shared/api/coupon.ts:396`.

6. Backend voucher routes are owner/manager merchant routes, while user voucher claim/list routes are separate customer-authenticated routes.
   Evidence: `locallife/api/server.go:1619`, `locallife/api/server.go:1621`, `locallife/api/server.go:1656`, `locallife/api/server.go:1660`.

7. Voucher create validates time and order-type values, then writes `vouchers`.
   Evidence: `locallife/api/voucher.go:74`, `locallife/api/voucher.go:93`, `locallife/api/voucher.go:99`, `locallife/api/voucher.go:120`.

8. Fixed 2026-06-08: voucher list calls `CountMerchantVouchers` and returns full matched `total`, not the current page length.
   Evidence: `locallife/api/voucher.go:192`, `locallife/api/voucher.go:202`, `locallife/api/voucher.go:213`, `locallife/db/query/voucher.sql:39`, `locallife/api/voucher_test.go:743`.

9. Fixed 2026-06-08: voucher update verifies merchant ownership and effective time range, and prevalidates `total_quantity >= claimed_quantity` before writing so merchants get a stable 400 instead of a DB constraint failure. The DB check remains direct-writer protection.
    Evidence: `locallife/api/voucher.go:320`, `locallife/api/voucher.go:339`, `locallife/api/voucher.go:379`, `locallife/api/voucher.go:380`, `locallife/api/voucher_test.go:473`, `locallife/db/migration/000019_add_membership_marketing_system.up.sql:130`.

10. Voucher delete blocks when there are unexpired unused user vouchers, then soft-deletes the template.
    Evidence: `locallife/api/voucher.go:450`, `locallife/api/voucher.go:486`, `locallife/api/voucher.go:492`, `locallife/api/voucher.go:497`, `locallife/db/query/voucher.sql:91`.

11. Voucher claim uses a transaction: locks the template, checks active/current/not-depleted/duplicate claim, increments claimed count, and creates `user_vouchers`.
    Evidence: `locallife/db/sqlc/tx_voucher.go:23`, `locallife/db/sqlc/tx_voucher.go:30`, `locallife/db/sqlc/tx_voucher.go:36`, `locallife/db/sqlc/tx_voucher.go:48`, `locallife/db/sqlc/tx_voucher.go:60`, `locallife/db/sqlc/tx_voucher.go:66`.

12. User available voucher list and direct voucher validation join the template but do not filter current template `is_active`, `deleted_at`, or template valid period after claim. Product decision 2026-06-10 says disabled/deleted templates must not be usable for new preview/order, while already-created orders are not retroactively changed.
    Evidence: `locallife/db/query/voucher.sql:137`, `locallife/db/query/voucher.sql:141`, `locallife/db/query/voucher.sql:143`, `locallife/logic/order_pricing.go:71`, `locallife/logic/order_pricing.go:82`, `locallife/logic/order_pricing.go:85`, `locallife/logic/order_pricing.go:88`.

13. Order creation marks user vouchers used inside `CreateOrderTx`; order cancellation rolls them back to unused or expired and decrements used count.
    Evidence: `locallife/db/sqlc/tx_create_order.go:167`, `locallife/db/sqlc/tx_create_order.go:169`, `locallife/db/sqlc/tx_create_order.go:178`, `locallife/db/sqlc/tx_order_status.go:256`, `locallife/db/sqlc/tx_order_status.go:263`, `locallife/db/sqlc/tx_order_status.go:281`.

14. Scheduler marks expired unused user vouchers asynchronously.
    Evidence: `locallife/scheduler/data_cleanup.go:1520`, `locallife/scheduler/data_cleanup.go:1524`, `locallife/db/query/voucher.sql:180`.

15. Recharge-rule pages load the full list, find target rules by id for edit, toggle status, delete rules, and submit full create/update payloads.
    Evidence: `weapp/miniprogram/pages/merchant/settings/recharge-rules/index.ts:271`, `weapp/miniprogram/pages/merchant/settings/recharge-rules/index.ts:302`, `weapp/miniprogram/pages/merchant/settings/recharge-rules/index.ts:375`, `weapp/miniprogram/pages/merchant/settings/recharge-rules/edit/index.ts:173`, `weapp/miniprogram/pages/merchant/settings/recharge-rules/edit/index.ts:276`, `weapp/miniprogram/pages/merchant/settings/recharge-rules/edit/index.ts:313`.

16. Recharge wrappers call `/v1/merchants/:id/recharge-rules`; backend routes use owner/manager middleware.
    Evidence: `weapp/miniprogram/api/merchant.ts:804`, `weapp/miniprogram/api/merchant.ts:815`, `weapp/miniprogram/api/merchant.ts:827`, `weapp/miniprogram/api/merchant.ts:839`, `locallife/api/server.go:1599`, `locallife/api/server.go:1601`.

17. Recharge create validates time, update merges existing and incoming times before validating, and delete currently physically removes the row. Product decision 2026-06-10 changes the desired contract to deactivate/disable rather than physical delete.
    Evidence: `locallife/logic/recharge_rule.go:43`, `locallife/logic/recharge_rule.go:44`, `locallife/logic/recharge_rule.go:105`, `locallife/logic/recharge_rule.go:122`, `locallife/logic/recharge_rule.go:130`, `locallife/logic/recharge_rule.go:176`, `locallife/db/query/membership.sql:127`.

18. Membership recharge resolves an exact active matching recharge rule at prepare/record time, stores the nullable rule id on transaction creation, and reloads idempotent merchant recharge by key.
    Evidence: `locallife/logic/membership_recharge.go:77`, `locallife/logic/membership_recharge.go:126`, `locallife/logic/membership_recharge.go:131`, `locallife/logic/membership_recharge.go:198`, `locallife/db/migration/000019_add_membership_marketing_system.up.sql:86`.

19. Delivery-promotion pages load all merchant promotions, find target edit rows by list lookup, toggle status, delete, and submit full create/update payloads.
    Evidence: `weapp/miniprogram/pages/merchant/delivery-promotions/index.ts:270`, `weapp/miniprogram/pages/merchant/delivery-promotions/index.ts:301`, `weapp/miniprogram/pages/merchant/delivery-promotions/index.ts:382`, `weapp/miniprogram/pages/merchant/delivery-promotions/edit/index.ts:172`, `weapp/miniprogram/pages/merchant/delivery-promotions/edit/index.ts:275`, `weapp/miniprogram/pages/merchant/delivery-promotions/edit/index.ts:323`.

20. Delivery wrappers call `/v1/delivery-fee/merchants/:merchant_id/promotions`.
    Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/delivery-fee.ts:260`, `weapp/miniprogram/pages/merchant/_main_shared/api/delivery-fee.ts:270`, `weapp/miniprogram/pages/merchant/_main_shared/api/delivery-fee.ts:281`, `weapp/miniprogram/pages/merchant/_main_shared/api/delivery-fee.ts:292`.

21. Fixed 2026-06-08: delivery-promotion create/update share value validation; update parses incoming fields, merges them with the existing row, and rejects an invalid effective time range or `discount_amount > min_order_amount` before writing partial SQL.
    Evidence: `locallife/api/delivery_fee.go:900`, `locallife/api/delivery_fee.go:940`, `locallife/api/delivery_fee.go:1134`, `locallife/api/delivery_fee.go:1171`, `locallife/api/delivery_fee.go:1211`, `locallife/api/delivery_fee.go:1259`, `locallife/db/query/delivery_promotion.sql:42`.

22. Delivery-fee calculation reads active merchant delivery promotions and applies the maximum discount as an order-total discount, not as a rider-fee reduction.
    Evidence: `locallife/api/delivery_fee.go:1356`, `locallife/api/delivery_fee.go:1373`, `locallife/api/delivery_fee.go:1489`, `locallife/api/delivery_fee.go:1491`, `locallife/api/delivery_fee.go:1502`.

23. Discount-rule pages keep a pagination lookahead compatibility fallback, toggle status, delete, and edit through a single-rule API. Backend `total` is now full matched count.
    Evidence: `weapp/miniprogram/pages/merchant/discount-rules/index.ts:229`, `weapp/miniprogram/pages/merchant/discount-rules/index.ts:252`, `weapp/miniprogram/pages/merchant/discount-rules/index.ts:504`, `weapp/miniprogram/pages/merchant/discount-rules/index.ts:584`, `weapp/miniprogram/pages/merchant/discount-rules/edit/index.ts:188`.

24. Discount wrappers call list/detail/create/update/delete endpoints. Update wrapper includes the rule id in the JSON body as required by the current handler.
    Evidence: `weapp/miniprogram/api/merchant.ts:885`, `weapp/miniprogram/api/merchant.ts:897`, `weapp/miniprogram/api/merchant.ts:908`, `weapp/miniprogram/api/merchant.ts:920`, `weapp/miniprogram/api/merchant.ts:935`.

25. Fixed 2026-06-08: discount static routes `/applicable` and `/best` remain reachable even though `/:id` is registered first; `TestDiscountStaticRoutesReachable` asserts both static paths bypass `GetDiscountRule` and call their dedicated store methods.
    Evidence: `locallife/api/server.go:1717`, `locallife/api/server.go:1726`, `locallife/api/server.go:1729`, `locallife/api/discount_test.go:434`.

26. Fixed 2026-06-08: discount create/update share value validation; update reads the existing row, merges incoming partial fields with current values, and rejects invalid effective values before writing SQL.
    Evidence: `locallife/logic/discount_rule.go:75`, `locallife/logic/discount_rule.go:76`, `locallife/logic/discount_rule.go:187`, `locallife/logic/discount_rule.go:200`, `locallife/logic/discount_rule.go:216`, `locallife/logic/discount_rule.go:220`.

27. Fixed 2026-06-08: discount list calls the authorized `CountMerchantDiscountRules` path and returns full matched `total`, not the current page length.
    Evidence: `locallife/api/discount.go:206`, `locallife/api/discount.go:220`, `locallife/api/discount.go:237`, `locallife/logic/discount_rule.go:144`, `locallife/db/query/discount.sql:30`, `locallife/api/discount_test.go:250`.

28. Promotion engine reads active discount rules, selects the best rule per stacking group, blocks voucher/membership stacking based on selected rules, and builds ladder/voucher trial hints.
    Evidence: `locallife/logic/promotion_engine.go:104`, `locallife/logic/promotion_engine.go:111`, `locallife/logic/promotion_engine.go:119`, `locallife/logic/promotion_engine.go:127`, `locallife/logic/promotion_engine.go:181`, `locallife/logic/promotion_engine.go:237`.

29. Fixed 2026-06-09: cart preview and order preview both look up carts by `order_type` before running `PromotionEngine.CalculateFinalPrice`; direct order creation separately resolves discount, validates voucher, computes totals, and persists totals. `TestOrderServiceCreateOrder_MarketingTotalsMatchCartAndOrderPreview` proves the three paths agree on subtotal, merchant discount, voucher amount, delivery fee, delivery fee discount, and final total for a takeout cart with a merchant discount, user voucher, and delivery promotion.
    Evidence: `locallife/logic/cart_calculation.go:67`, `locallife/logic/cart_calculation.go:134`, `locallife/logic/order_calculation.go:77`, `locallife/logic/order_calculation.go:240`, `locallife/logic/order_service.go:182`, `locallife/logic/order_service.go:195`, `locallife/logic/order_service.go:237`, `locallife/logic/order_service_create_test.go:116`.

30. Public merchant promotions and public merchant detail read active delivery, discount, voucher, and recharge rule truth for customer-visible surfaces.
    Evidence: `locallife/api/merchant.go:965`, `locallife/api/merchant.go:991`, `locallife/api/merchant.go:1007`, `locallife/api/merchant.go:1024`, `locallife/api/merchant.go:1049`, `locallife/api/merchant.go:1307`, `locallife/api/merchant.go:1320`, `locallife/api/merchant.go:1333`.

## Reverse-Reference Findings

- Voucher, discount, delivery-promotion, and recharge-rule pages are separate merchant UI surfaces, but the changed state converges in shared checkout and public merchant readers.
- Recharge-rule update has effective time validation and is not the same defect class as discount/delivery updates.
- `membership_transactions.recharge_rule_id` has `ON DELETE SET NULL`, so physical recharge-rule deletion keeps the transaction row but removes direct rule provenance. Product decision 2026-06-10 says recharge rules should be deactivated/disabled instead of physically deleted.
- Voucher templates are soft-deleted, discount rules are soft-deleted, while recharge rules and delivery promotions are currently physically deleted. Product decision 2026-06-10 requires recharge-rule delete behavior to become deactivate/disable.
- User vouchers are durable independent rows after claim. Product decision 2026-06-10 requires template disable/delete to block future preview/order use even for already issued but unused vouchers; orders already created must not be retroactively changed.
- Public merchant detail uses separate `merchant_stats.sql` active-list queries in addition to the management and promotion-engine SQL queries.
- `logic/replace_order.go` uses the shared best-discount amount path for reservation replacement pricing; discount rule semantics therefore affect more than new order creation.
- Voucher and discount merchant edit pages differ: discount has a single detail API, while voucher edit scans paged list results. Recharge and delivery edit pages also load full lists and find the target id.

## SQL And Durable State Boundaries

- `vouchers`: soft-deleted template truth; owns code, amount, minimum order amount, total/claimed/used quantity, active flag, valid period, and allowed order types.
- `user_vouchers`: issued voucher truth; owns user, status, order id, obtained/used/expiry times. The expiry scheduler writes only this table.
- `discount_rules`: soft-deleted merchant discount truth; owns threshold, amount, stacking flags, stacking group, active flag, and valid period.
- `merchant_delivery_promotions`: physical rows for delivery-fee discounts; owns threshold, discount amount, active flag, and valid period.
- `recharge_rules`: physical rows for recharge bonus rules; owns exact recharge amount, bonus amount, active flag, and valid period.
- `membership_transactions.recharge_rule_id`: nullable historical link to the recharge rule used by a recharge.
- `orders.user_voucher_id`, `orders.voucher_amount`, `orders.discount_amount`, `orders.delivery_fee_discount`, `orders.total_amount`, and membership payment fields persist the order-time pricing result.

## Trust, Authorization, And Tenant Checks

- Merchant management route groups use `MerchantStaffMiddleware("owner", "manager")`.
- Voucher handlers call `requireMerchantMatch`; recharge logic checks target merchant against current merchant; delivery handlers compare route merchant id to the merchant context; discount logic checks fetched rule ownership.
- Customer voucher and order paths use authenticated user id from auth context and validate user voucher ownership before use.
- Downstream order/cart paths read merchant ids from merchant/order/cart context, not client-provided marketing settings.
- Public merchant promotion/detail readers expose active rules after verifying merchant existence or storefront accessibility.

## Idempotency And Duplicate-Submit Checks

- Mini Program pages guard create/update/delete/toggle with local `submitting`, `loading`, status-pending, or dialog-submitting flags.
- Merchant CRUD PATCH/POST/DELETE operations have no idempotency key and no version. Repeated identical saves converge; concurrent saves are last-write-wins.
- Voucher claim is transactional and prevents duplicate user claims with `CheckUserVoucherExists`.
- Voucher usage during order creation is transaction-owned and conditional on unused status.
- Order cancellation voucher rollback is idempotent around used status and matching order id.
- Merchant offline member recharge, which consumes recharge rules, requires and reuses an idempotency key.

## Recovery And Async Convergence Paths

- Rule CRUD has no worker, outbox, websocket, or scheduler path.
- User voucher expiry runs in `DataCleanupScheduler.markExpiredVouchers`.
- Order cancellation can roll a used voucher back to unused or expired.
- Cart/order preview re-reads current active rules every call.
- Existing bad rule rows are not automatically repaired; they remain effective or visible according to reader filters until fixed, disabled, deleted, expired, or corrected.

## Frontend Draft And Backend Rehydration

- List pages load backend truth, preserve last trusted list on refresh failure, and update local state from backend responses for toggle/delete operations.
- Voucher and discount list pages still contain lookahead probing as a compatibility fallback, but backend management-list `total` is now full matched count.
- Edit pages use local form drafts and navigate back after save instead of rehydrating in place. Parent pages reload on return.
- Discount edit has the most direct rehydration path because it uses `GET /discounts/:id`.
- Voucher edit is the most brittle detail-loading path because it scans up to 20 list pages looking for a target voucher id.

## Test Coverage Signals

Observed tests:

- Voucher API status/claim/update tests and `tx_voucher_test.go` cover claim, duplicate claim, stock, inactive/expired, use, concurrent claim behavior, and rejecting `total_quantity < claimed_quantity` before SQL update.
- `tx_create_order_test.go` and `tx_order_status_test.go` cover using vouchers during order creation and rolling them back on cancellation.
- Recharge-rule logic/API tests cover create/update/delete/list authz and update invalid dates.
- Membership recharge tests cover matching active recharge rules and idempotent merchant recharge behavior.
- Discount-rule API/logic tests cover applicable and best rule lookups, static route reachability, full-count management pagination, update invalid effective values, and create/update shared value validation; order-pricing tests cover discount stacking with vouchers.
- Delivery-fee API/sqlc tests cover delivery promotion create/list/active/delete, invalid merged update values, non-positive minimum order amount rejection before store update, valid partial update parameter mapping, direct SQL constraint rejection, and migration `000254` clean/dirty convergence.
- Merchant public detail tests cover active discount/voucher/delivery promotion response assembly.
- Order calculation and promotion engine tests cover voucher trials, merchant discount stacking, and payment assessment interactions. `TestOrderServiceCreateOrder_MarketingTotalsMatchCartAndOrderPreview` covers order preview's typed cart lookup plus cart preview, order preview, and direct order creation parity for a combined merchant discount, voucher, delivery fee, and delivery-fee discount stack.

Missing high-value tests:

- Product-decided semantics for issued vouchers after template disable/soft-delete need focused preview and direct order creation tests: disabled/deleted templates block future preview/order use, and existing orders are not retroactively changed.

## Gaps And Refactor Notes

- Fixed 2026-06-08: discount-rule update computes effective merged values and applies the same business validation as create before SQL update, including positive amounts, date range, and discount threshold checks.
- Fixed 2026-06-08: delivery-promotion update computes effective merged values and applies the same business validation as create before SQL update at the handler boundary.
- Fixed 2026-06-09: delivery-promotion schema hardening adds DB constraints for positive amounts, `discount_amount <= min_order_amount`, and `valid_until > valid_from`; migration `000254` cleans historical invalid rows forward, direct sqlc writes now fail with PostgreSQL CHECK violations, and API binding rejects non-positive minimum order amounts before store update.
- Fixed 2026-06-08: voucher update rejects `total_quantity` below already claimed quantity at the handler boundary before SQL update; DB constraints still protect direct writers.
- Product decision 2026-06-10: voucher template disable/delete blocks future preview/order use, including already issued but unused vouchers; already-created orders are not retroactively changed. Current behavior is closer to "disable/delete stops future claims but already issued user vouchers can remain usable until user-voucher expiry", so runtime readers and UI copy need alignment.
- Fixed 2026-06-08: backend voucher and discount management lists return full matched `total`; frontend probe logic is now only a compatibility fallback.
- Fixed 2026-06-08: discount static route reachability is verified by `TestDiscountStaticRoutesReachable`; `/applicable` and `/best` are not shadowed by `/:id`.
- Fixed 2026-06-09: order preview now includes `order_type` in the cart lookup, matching cart preview; cart preview, order preview, and direct order creation pricing parity is proof-covered for a takeout marketing stack with merchant discount, user voucher, delivery fee, and delivery-fee discount.
- Product decision 2026-06-10: recharge rules should be deactivated/disabled rather than physically deleted. Current physical delete is technically protected by `ON DELETE SET NULL`, but it weakens audit provenance for transactions.

## Branch Exhaustion

- Entry branches checked: Mini Program vouchers list/edit/toggle/delete, discount rules list/edit/toggle/delete, delivery promotions list/edit/toggle/delete, recharge rules list/edit/delete, member recharge consumer, public merchant promotion readers, cart/order preview, direct order creation, voucher claim/use/rollback, and voucher expiry scheduler. Flutter App has no marketing-rule management entry in `merchant_app/lib/features/**`.
- Request branches checked: merchant voucher CRUD/status/list/detail-by-scan workaround, discount CRUD/applicable/best, delivery promotion CRUD/active/list, recharge-rule CRUD/list, member recharge, customer voucher claim/list, cart/order preview, direct order, cancellation rollback, and public promotion assembly.
- Backend state branches checked: voucher templates and issued `user_vouchers`, discount rules, delivery promotions, recharge rules, membership transaction rule link, order persisted pricing fields, active/expired/soft-deleted states, issued-voucher usability after template change, and route ordering for static discount endpoints. Product decisions now require disabled/deleted voucher templates to block future preview/order and recharge rules to deactivate instead of physical delete.
- Async branches checked: rule CRUD is synchronous; voucher expiry runs in cleanup scheduler; order cancellation can roll voucher state back; cart/order preview re-reads active rules; no repair worker exists for already-invalid rule rows.
- Failure/retry branches checked: local submit/toggle guards, no CRUD idempotency/version, full-count backend list total with frontend lookahead fallback, voucher update quantity below claimed quantity, current physical recharge-rule delete with historical transactions, and delivery-promotion direct SQL/future writers protected by DB constraints. Product decision 2026-06-10 requires recharge-rule disable/deactivate instead of physical delete.
- Reader/consumer branches checked: merchant marketing pages, member recharge, customer voucher center, public merchant detail, cart/order preview, direct order creation, promotion engine, order cancellation, and settlement/order pricing fields.
- Authorization/tenant branches checked: owner/manager merchant route groups, merchant match checks, recharge current merchant check, discount ownership checks, customer voucher ownership validation, and downstream order/cart using authenticated/derived merchant/user context.
- Zombie/unreachable branches checked: voucher/discount frontend list probe remains a compatibility fallback after backend full-count repair; voucher edit scans list pages instead of detail truth; discount `/applicable` and `/best` route reachability is now proof-covered; recharge-rule delete may erase live rule provenance from UI while DB only preserves nullable transaction reference.
- Test-proof gaps checked: existing tests cover voucher claim/use/rollback, voucher update product 400 for claimed quantity, voucher/discount full-count management pagination, recharge-rule CRUD/idempotent recharge, discount lookup/order pricing, discount static route reachability, discount update merged validation, delivery update merged validation, delivery SQL and DB constraints, public promotion assembly, promotion engine, order-preview typed cart lookup, and cart/order preview/direct-create marketing-total parity. Missing proof remains for disabled/deleted voucher template blocking future preview/order without retroactive order changes, and recharge-rule deactivate/disable semantics.
