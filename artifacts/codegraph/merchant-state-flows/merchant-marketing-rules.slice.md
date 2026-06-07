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
- Voucher lifecycle semantics must be explicit: disabling or deleting a voucher template should either stop only future claims, or also stop future use of already issued user vouchers.
- Pagination metadata on merchant rule pages should be real backend truth, not a current-page count that requires client probing.

Current implementation still violates the first invariant for delivery promotions and leaves voucher template disable/delete semantics implicit. Fixed 2026-06-08: discount-rule update now merges existing rule values with partial PATCH fields and applies the same create-level value validation before writing.

## Primary Forward Chain

1. Merchant dashboard/config exposes marketing entries around membership/recharge, vouchers, delivery promotions, and discount rules.
   Evidence: `weapp/miniprogram/pages/merchant/_utils/merchant-dashboard-view.ts:188`, `weapp/miniprogram/pages/merchant/config/index.ts:61`.

2. Voucher list page syncs current merchant context, loads paged vouchers, probes the next page when backend total is unreliable, and preserves old data on refresh failure.
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

8. Voucher list returns `Total: int64(len(rsp))`, which is current page length, not a full matched count.
   Evidence: `locallife/api/voucher.go:174`, `locallife/api/voucher.go:192`, `locallife/api/voucher.go:207`, `locallife/api/voucher.go:209`.

9. Voucher update verifies merchant ownership and effective time range, but does not prevalidate effective `total_quantity >= claimed_quantity`; that relies on the DB check constraint.
   Evidence: `locallife/api/voucher.go:320`, `locallife/api/voucher.go:339`, `locallife/api/voucher.go:403`, `locallife/api/voucher.go:417`, `locallife/db/migration/000019_add_membership_marketing_system.up.sql:130`.

10. Voucher delete blocks when there are unexpired unused user vouchers, then soft-deletes the template.
    Evidence: `locallife/api/voucher.go:450`, `locallife/api/voucher.go:486`, `locallife/api/voucher.go:492`, `locallife/api/voucher.go:497`, `locallife/db/query/voucher.sql:91`.

11. Voucher claim uses a transaction: locks the template, checks active/current/not-depleted/duplicate claim, increments claimed count, and creates `user_vouchers`.
    Evidence: `locallife/db/sqlc/tx_voucher.go:23`, `locallife/db/sqlc/tx_voucher.go:30`, `locallife/db/sqlc/tx_voucher.go:36`, `locallife/db/sqlc/tx_voucher.go:48`, `locallife/db/sqlc/tx_voucher.go:60`, `locallife/db/sqlc/tx_voucher.go:66`.

12. User available voucher list and direct voucher validation join the template but do not filter current template `is_active`, `deleted_at`, or template valid period after claim.
    Evidence: `locallife/db/query/voucher.sql:137`, `locallife/db/query/voucher.sql:141`, `locallife/db/query/voucher.sql:143`, `locallife/logic/order_pricing.go:71`, `locallife/logic/order_pricing.go:82`, `locallife/logic/order_pricing.go:85`, `locallife/logic/order_pricing.go:88`.

13. Order creation marks user vouchers used inside `CreateOrderTx`; order cancellation rolls them back to unused or expired and decrements used count.
    Evidence: `locallife/db/sqlc/tx_create_order.go:167`, `locallife/db/sqlc/tx_create_order.go:169`, `locallife/db/sqlc/tx_create_order.go:178`, `locallife/db/sqlc/tx_order_status.go:256`, `locallife/db/sqlc/tx_order_status.go:263`, `locallife/db/sqlc/tx_order_status.go:281`.

14. Scheduler marks expired unused user vouchers asynchronously.
    Evidence: `locallife/scheduler/data_cleanup.go:1520`, `locallife/scheduler/data_cleanup.go:1524`, `locallife/db/query/voucher.sql:180`.

15. Recharge-rule pages load the full list, find target rules by id for edit, toggle status, delete rules, and submit full create/update payloads.
    Evidence: `weapp/miniprogram/pages/merchant/settings/recharge-rules/index.ts:271`, `weapp/miniprogram/pages/merchant/settings/recharge-rules/index.ts:302`, `weapp/miniprogram/pages/merchant/settings/recharge-rules/index.ts:375`, `weapp/miniprogram/pages/merchant/settings/recharge-rules/edit/index.ts:173`, `weapp/miniprogram/pages/merchant/settings/recharge-rules/edit/index.ts:276`, `weapp/miniprogram/pages/merchant/settings/recharge-rules/edit/index.ts:313`.

16. Recharge wrappers call `/v1/merchants/:id/recharge-rules`; backend routes use owner/manager middleware.
    Evidence: `weapp/miniprogram/api/merchant.ts:804`, `weapp/miniprogram/api/merchant.ts:815`, `weapp/miniprogram/api/merchant.ts:827`, `weapp/miniprogram/api/merchant.ts:839`, `locallife/api/server.go:1599`, `locallife/api/server.go:1601`.

17. Recharge create validates time, update merges existing and incoming times before validating, and delete physically removes the row.
    Evidence: `locallife/logic/recharge_rule.go:43`, `locallife/logic/recharge_rule.go:44`, `locallife/logic/recharge_rule.go:105`, `locallife/logic/recharge_rule.go:122`, `locallife/logic/recharge_rule.go:130`, `locallife/logic/recharge_rule.go:176`, `locallife/db/query/membership.sql:127`.

18. Membership recharge resolves an exact active matching recharge rule at prepare/record time, stores the nullable rule id on transaction creation, and reloads idempotent merchant recharge by key.
    Evidence: `locallife/logic/membership_recharge.go:77`, `locallife/logic/membership_recharge.go:126`, `locallife/logic/membership_recharge.go:131`, `locallife/logic/membership_recharge.go:198`, `locallife/db/migration/000019_add_membership_marketing_system.up.sql:86`.

19. Delivery-promotion pages load all merchant promotions, find target edit rows by list lookup, toggle status, delete, and submit full create/update payloads.
    Evidence: `weapp/miniprogram/pages/merchant/delivery-promotions/index.ts:270`, `weapp/miniprogram/pages/merchant/delivery-promotions/index.ts:301`, `weapp/miniprogram/pages/merchant/delivery-promotions/index.ts:382`, `weapp/miniprogram/pages/merchant/delivery-promotions/edit/index.ts:172`, `weapp/miniprogram/pages/merchant/delivery-promotions/edit/index.ts:275`, `weapp/miniprogram/pages/merchant/delivery-promotions/edit/index.ts:323`.

20. Delivery wrappers call `/v1/delivery-fee/merchants/:merchant_id/promotions`.
    Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/delivery-fee.ts:260`, `weapp/miniprogram/pages/merchant/_main_shared/api/delivery-fee.ts:270`, `weapp/miniprogram/pages/merchant/_main_shared/api/delivery-fee.ts:281`, `weapp/miniprogram/pages/merchant/_main_shared/api/delivery-fee.ts:292`.

21. Delivery-promotion create validates time and `discount_amount <= min_order_amount`, but update only parses incoming fields and writes them through partial SQL.
    Evidence: `locallife/api/delivery_fee.go:900`, `locallife/api/delivery_fee.go:940`, `locallife/api/delivery_fee.go:945`, `locallife/api/delivery_fee.go:1134`, `locallife/api/delivery_fee.go:1184`, `locallife/api/delivery_fee.go:1210`, `locallife/db/query/delivery_promotion.sql:42`.

22. Delivery-fee calculation reads active merchant delivery promotions and applies the maximum discount as an order-total discount, not as a rider-fee reduction.
    Evidence: `locallife/api/delivery_fee.go:1356`, `locallife/api/delivery_fee.go:1373`, `locallife/api/delivery_fee.go:1489`, `locallife/api/delivery_fee.go:1491`, `locallife/api/delivery_fee.go:1502`.

23. Discount-rule pages probe pagination like vouchers, toggle status, delete, and edit through a single-rule API.
    Evidence: `weapp/miniprogram/pages/merchant/discount-rules/index.ts:229`, `weapp/miniprogram/pages/merchant/discount-rules/index.ts:252`, `weapp/miniprogram/pages/merchant/discount-rules/index.ts:504`, `weapp/miniprogram/pages/merchant/discount-rules/index.ts:584`, `weapp/miniprogram/pages/merchant/discount-rules/edit/index.ts:188`.

24. Discount wrappers call list/detail/create/update/delete endpoints. Update wrapper includes the rule id in the JSON body as required by the current handler.
    Evidence: `weapp/miniprogram/api/merchant.ts:885`, `weapp/miniprogram/api/merchant.ts:897`, `weapp/miniprogram/api/merchant.ts:908`, `weapp/miniprogram/api/merchant.ts:920`, `weapp/miniprogram/api/merchant.ts:935`.

25. Discount routes register `/:id` before `/applicable` and `/best`, making static route reachability a risk signal that should be checked with a focused Gin route test.
    Evidence: `locallife/api/server.go:1686`, `locallife/api/server.go:1695`, `locallife/api/server.go:1698`.

26. Discount create validates time and `discount_amount < min_order_amount`, but update only maps partial fields into SQL and returns the persisted row.
    Evidence: `locallife/logic/discount_rule.go:75`, `locallife/logic/discount_rule.go:76`, `locallife/logic/discount_rule.go:79`, `locallife/logic/discount_rule.go:190`, `locallife/logic/discount_rule.go:203`, `locallife/logic/discount_rule.go:235`.

27. Discount list also returns `Total: len(current page)`.
    Evidence: `locallife/api/discount.go:187`, `locallife/api/discount.go:206`, `locallife/api/discount.go:225`, `locallife/api/discount.go:227`.

28. Promotion engine reads active discount rules, selects the best rule per stacking group, blocks voucher/membership stacking based on selected rules, and builds ladder/voucher trial hints.
    Evidence: `locallife/logic/promotion_engine.go:104`, `locallife/logic/promotion_engine.go:111`, `locallife/logic/promotion_engine.go:119`, `locallife/logic/promotion_engine.go:127`, `locallife/logic/promotion_engine.go:181`, `locallife/logic/promotion_engine.go:237`.

29. Cart preview and order preview run `PromotionEngine.CalculateFinalPrice`; direct order creation separately resolves discount, validates voucher, validates membership payment, computes totals, and persists totals.
    Evidence: `locallife/logic/cart_calculation.go:134`, `locallife/logic/order_calculation.go:239`, `locallife/logic/order_service.go:182`, `locallife/logic/order_service.go:195`, `locallife/logic/order_service.go:221`, `locallife/logic/order_service.go:237`, `locallife/logic/order_service.go:313`.

30. Public merchant promotions and public merchant detail read active delivery, discount, voucher, and recharge rule truth for customer-visible surfaces.
    Evidence: `locallife/api/merchant.go:965`, `locallife/api/merchant.go:991`, `locallife/api/merchant.go:1007`, `locallife/api/merchant.go:1024`, `locallife/api/merchant.go:1049`, `locallife/api/merchant.go:1307`, `locallife/api/merchant.go:1320`, `locallife/api/merchant.go:1333`.

## Reverse-Reference Findings

- Voucher, discount, delivery-promotion, and recharge-rule pages are separate merchant UI surfaces, but the changed state converges in shared checkout and public merchant readers.
- Recharge-rule update has effective time validation and is not the same defect class as discount/delivery updates.
- `membership_transactions.recharge_rule_id` has `ON DELETE SET NULL`, so physical recharge-rule deletion keeps the transaction row but removes direct rule provenance.
- Voucher templates are soft-deleted, discount rules are soft-deleted, while recharge rules and delivery promotions are physically deleted.
- User vouchers are durable independent rows after claim. They remain queryable through joins even if template active/deleted/current-valid semantics later change, unless the query or validation layer explicitly filters template state.
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
- Voucher and discount list pages implement lookahead probing to infer `hasMore` because backend list `total` is not trustworthy.
- Edit pages use local form drafts and navigate back after save instead of rehydrating in place. Parent pages reload on return.
- Discount edit has the most direct rehydration path because it uses `GET /discounts/:id`.
- Voucher edit is the most brittle detail-loading path because it scans up to 20 list pages looking for a target voucher id.

## Test Coverage Signals

Observed tests:

- Voucher API status/claim tests and `tx_voucher_test.go` cover claim, duplicate claim, stock, inactive/expired, use, and concurrent claim behavior.
- `tx_create_order_test.go` and `tx_order_status_test.go` cover using vouchers during order creation and rolling them back on cancellation.
- Recharge-rule logic/API tests cover create/update/delete/list authz and update invalid dates.
- Membership recharge tests cover matching active recharge rules and idempotent merchant recharge behavior.
- Discount-rule API/logic tests cover applicable and best rule lookups, update invalid effective values, and create/update shared value validation; order-pricing tests cover discount stacking with vouchers.
- Delivery-fee sqlc tests cover delivery promotion create/list/active/delete.
- Merchant public detail tests cover active discount/voucher/delivery promotion response assembly.
- Order calculation and promotion engine tests cover voucher trials, merchant discount stacking, and payment assessment interactions.

Missing high-value tests:

- Delivery-promotion update rejects effective `valid_until < valid_from`.
- Delivery-promotion update rejects effective `discount_amount > min_order_amount`.
- Voucher update returns product-level 4xx for `total_quantity < claimed_quantity` instead of an internal DB error.
- Merchant voucher and discount list pagination exposes full count or explicit `has_more`.
- Discount `/applicable` and `/best` route reachability is locked by a focused route test.
- Product-decided semantics for issued vouchers after template disable/soft-delete are covered in preview and direct order creation.

## Gaps And Refactor Notes

- Fixed 2026-06-08: discount-rule update computes effective merged values and applies the same business validation as create before SQL update, including positive amounts, date range, and discount threshold checks.
- Delivery-promotion update should compute effective merged values and apply the same business validation as create before SQL update.
- Voucher template disable/delete semantics need a durable product decision. Current behavior is closer to "disable/delete stops future claims but already issued user vouchers can remain usable until user-voucher expiry."
- Backend list `total` contract should be corrected before relying on list pages for precise counts; frontend probe logic is a workaround, not product truth.
- Discount static routes should be registered before `/:id` or verified by a route test.
- Recharge-rule physical delete is technically protected by `ON DELETE SET NULL`, but deactivation would preserve stronger audit provenance for transactions.
- Delivery promotions have no DB check constraints for valid period or discount/threshold relation. Fixing only the handler still leaves direct SQL or future writers able to persist invalid rows.

## Branch Exhaustion

- Entry branches checked: Mini Program vouchers list/edit/toggle/delete, discount rules list/edit/toggle/delete, delivery promotions list/edit/toggle/delete, recharge rules list/edit/delete, member recharge consumer, public merchant promotion readers, cart/order preview, direct order creation, voucher claim/use/rollback, and voucher expiry scheduler. Flutter App has no marketing-rule management entry in `merchant_app/lib/features/**`.
- Request branches checked: merchant voucher CRUD/status/list/detail-by-scan workaround, discount CRUD/applicable/best, delivery promotion CRUD/active/list, recharge-rule CRUD/list, member recharge, customer voucher claim/list, cart/order preview, direct order, cancellation rollback, and public promotion assembly.
- Backend state branches checked: voucher templates and issued `user_vouchers`, discount rules, delivery promotions, recharge rules, membership transaction rule link, order persisted pricing fields, active/expired/soft-deleted states, issued-voucher usability after template change, and route ordering for static discount endpoints.
- Async branches checked: rule CRUD is synchronous; voucher expiry runs in cleanup scheduler; order cancellation can roll voucher state back; cart/order preview re-reads active rules; no repair worker exists for already-invalid rule rows.
- Failure/retry branches checked: local submit/toggle guards, no CRUD idempotency/version, list total drift with frontend lookahead probe, voucher update quantity below claimed quantity, delivery update missing effective merged validation, discount route shadowing risk, physical recharge-rule delete with historical transactions, and invalid direct SQL/future writers without DB constraints.
- Reader/consumer branches checked: merchant marketing pages, member recharge, customer voucher center, public merchant detail, cart/order preview, direct order creation, promotion engine, order cancellation, and settlement/order pricing fields.
- Authorization/tenant branches checked: owner/manager merchant route groups, merchant match checks, recharge current merchant check, discount ownership checks, customer voucher ownership validation, and downstream order/cart using authenticated/derived merchant/user context.
- Zombie/unreachable branches checked: voucher/discount frontend list probe compensates for backend total drift; voucher edit scans list pages instead of detail truth; discount `/applicable` and `/best` route reachability needs proof; recharge-rule delete may erase live rule provenance from UI while DB only preserves nullable transaction reference.
- Test-proof gaps checked: existing tests cover voucher claim/use/rollback, recharge-rule CRUD/idempotent recharge, discount lookup/order pricing, discount update merged validation, delivery SQL, public promotion assembly, and promotion engine. Missing proof remains for delivery update merged validation, voucher update product 4xx, precise pagination contract, discount static route reachability, issued voucher after template disable/delete, and delivery DB constraints.
