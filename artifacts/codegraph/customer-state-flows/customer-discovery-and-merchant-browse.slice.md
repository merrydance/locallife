# Customer Discovery And Merchant Browse Slice

Status: customer-state flow slice created 2026-06-14
Risk class: G2 boundary - customer discovery, merchant/item availability, coupon claim, wanted-merchant demand signal, role-registration exit
Scope: Mini Program takeout home/search/category/merchant detail/dish detail/combo detail/wanted merchants -> search/public merchant/voucher/wanted backend routes -> SQL search, merchant, dish, combo, room, voucher, wanted merchant, search history tables

## Variant Coverage

This slice covers:

- Takeout home load, category grid, region/location refresh, merchant feed, cart summary, search navigation, merchant/category/dish/combo navigation, and wanted-merchant card.
- Search page initial data, suggestions/history/popular, merchant/dish/combo/room search, and result navigation.
- Category and restaurant detail pages, including merchant public detail, dishes, combos, rooms, promotions, recharge rules, coupon claim, merchant info, dish detail, combo detail, and reviews read.
- Wanted-merchant leaderboard, candidate submit, existing vote, map pick, duplicate-submit guards, and result scroll.
- Backend search/public merchant/dish/combo/room/wanted/voucher routes and durable SQL reads/writes.

This slice does not fully cover:

- Order/cart mutation after item detail add-to-cart; covered by `customer-takeout-cart-checkout-payment.slice.md`.
- Merchant onboarding/invitation after wanted-merchant votes; this is platform/merchant-owned.
- Merchant/operator registration buttons on takeout home; they are explicit role-registration exits and excluded from ordinary-customer closure.
- Merchant-side dish/combo/room/status management; customer discovery only consumes public/orderable projections.

## Product Invariant

Customer discovery is read-mostly and must never invent orderability:

- The Mini Program may cache a selected region/location and local feed state, but merchant/item/room availability is read from backend search/public routes.
- Customer-facing dish, combo, merchant, room, promotion, voucher, and recharge-rule views must come from backend-visible, online/open/active states.
- Coupon claim and wanted-merchant vote are the only writes in this slice; both persist under the authenticated user and must be guarded by backend constraints.
- Role registration links are navigation exits, not customer discovery state transitions.

## Primary Forward Chain

1. Customer Mini Program declares takeout home plus customer takeout subpackages for search, category, merchant info, restaurant detail, dish detail, combo detail, wanted merchants, cart, and order confirm.
   Evidence: `weapp/miniprogram/app.json:3`, `weapp/miniprogram/app.json:126`, `weapp/miniprogram/app.json:132`, `weapp/miniprogram/app.json:138`, `weapp/miniprogram/app.json:144`, `weapp/miniprogram/app.json:150`, `weapp/miniprogram/app.json:156`, `weapp/miniprogram/app.json:162`, `weapp/miniprogram/app.json:168`, `weapp/miniprogram/app.json:174`.

2. Takeout home loads categories and feed after page entry, region/location changes, pull refresh, and pagination. It also exposes search, wanted-merchant, category, merchant, dish/combo, cart, and role-registration exits.
   Evidence: `weapp/miniprogram/pages/takeout/index.ts:49`, `weapp/miniprogram/pages/takeout/index.ts:105`, `weapp/miniprogram/pages/takeout/index.ts:164`, `weapp/miniprogram/pages/takeout/index.ts:166`, `weapp/miniprogram/pages/takeout/index.ts:248`, `weapp/miniprogram/pages/takeout/index.ts:259`, `weapp/miniprogram/pages/takeout/index.ts:303`, `weapp/miniprogram/pages/takeout/index.ts:546`.

3. Search page loads initial history/popular/suggestions, calls unified search wrappers, and navigates to dish or merchant detail.
   Evidence: `weapp/miniprogram/pages/takeout/search/index.ts:41`, `weapp/miniprogram/pages/takeout/search/index.ts:79`, `weapp/miniprogram/pages/takeout/search/index.ts:318`, `weapp/miniprogram/pages/takeout/search/index.ts:323`, `weapp/miniprogram/api/search.ts:173`, `weapp/miniprogram/api/search.ts:261`, `weapp/miniprogram/api/search.ts:289`, `weapp/miniprogram/api/search.ts:313`, `weapp/miniprogram/api/search.ts:357`.

4. Category and restaurant detail pages query public merchant/search APIs, then present merchant detail, dish/combo/room projections, promotions, recharge rules, has-ordered state, and coupon claim.
   Evidence: `weapp/miniprogram/pages/takeout/category/index.ts:44`, `weapp/miniprogram/pages/takeout/category/index.ts:102`, `weapp/miniprogram/pages/takeout/restaurant-detail/index.ts:103`, `weapp/miniprogram/pages/takeout/restaurant-detail/index.ts:167`, `weapp/miniprogram/pages/takeout/restaurant-detail/index.ts:228`, `weapp/miniprogram/api/merchant.ts:277`, `weapp/miniprogram/api/merchant.ts:337`, `weapp/miniprogram/api/merchant.ts:385`.

5. Dish and combo detail pages load public detail, reviews/merchant handoff, add-to-cart handoff, and detail-to-detail navigation.
   Evidence: `weapp/miniprogram/pages/takeout/dish-detail/index.ts:85`, `weapp/miniprogram/pages/takeout/dish-detail/index.ts:137`, `weapp/miniprogram/pages/takeout/dish-detail/index.ts:376`, `weapp/miniprogram/pages/takeout/dish-detail/index.ts:382`, `weapp/miniprogram/pages/takeout/combo-detail/index.ts:55`, `weapp/miniprogram/pages/takeout/combo-detail/index.ts:111`, `weapp/miniprogram/pages/takeout/combo-detail/index.ts:254`, `weapp/miniprogram/api/dish.ts:619`, `weapp/miniprogram/api/dish.ts:809`.

6. Wanted-merchant page loads leaderboard, guards local submit/vote state, submits new candidates or votes existing rows, and re-scrolls to the affected row.
   Evidence: `weapp/miniprogram/pages/takeout/wanted-merchants/index.ts:62`, `weapp/miniprogram/pages/takeout/wanted-merchants/index.ts:107`, `weapp/miniprogram/pages/takeout/wanted-merchants/index.ts:244`, `weapp/miniprogram/pages/takeout/wanted-merchants/index.ts:286`, `weapp/miniprogram/pages/takeout/wanted-merchants/index.ts:296`, `weapp/miniprogram/pages/takeout/wanted-merchants/index.ts:360`, `weapp/miniprogram/api/wanted-merchant.ts:60`, `weapp/miniprogram/api/wanted-merchant.ts:75`, `weapp/miniprogram/api/wanted-merchant.ts:88`.

7. Backend search and public merchant routes are customer-authenticated but do not require merchant/staff roles.
   Evidence: `locallife/api/server.go:583`, `locallife/api/server.go:589`, `locallife/api/server.go:591`, `locallife/api/server.go:592`, `locallife/api/server.go:593`, `locallife/api/server.go:594`, `locallife/api/server.go:628`, `locallife/api/server.go:632`, `locallife/api/server.go:635`.

8. Backend handlers map search/public reads to merchant/dish/combo/room/search-history SQL. Wanted-merchant writes persist demand/vote state.
   Evidence: `locallife/api/search.go:259`, `locallife/api/search.go:426`, `locallife/api/search.go:625`, `locallife/api/search.go:1014`, `locallife/api/search.go:1386`, `locallife/api/search.go:1477`, `locallife/api/search.go:1525`, `locallife/api/search.go:1677`, `locallife/api/merchant.go:1048`, `locallife/api/merchant.go:1286`, `locallife/api/merchant.go:1443`, `locallife/api/dish.go:959`, `locallife/api/combo.go:528`, `locallife/api/wanted_merchant.go:101`, `locallife/api/wanted_merchant.go:159`, `locallife/api/wanted_merchant.go:212`.

9. Coupon claim enters the user voucher route surface and persists a user voucher; available coupons are later visible in checkout/user-center.
   Evidence: `weapp/miniprogram/pages/takeout/restaurant-detail/_services/customer-discovery-workflow.ts:77`, `weapp/miniprogram/pages/takeout/restaurant-detail/_main_shared/api/coupon.ts:331`, `locallife/api/server.go:1693`, `locallife/api/voucher.go:555`, `locallife/db/query/voucher.sql:101`.

## SQL And Durable State Boundaries

- `merchants`: public merchant status, name, address, region, tags, open state, and search rows.
- `dishes`, `dish_categories`, customization tables, and dish tags: dish search/detail/orderability source.
- `combo_sets`, combo member dishes, and combo tags: combo search/detail/orderability source.
- `tables`, table images/tags, and room availability queries: room search and reservation discovery source.
- `search_history`: authenticated customer history, delete, clear, and popular keywords.
- `vouchers` and `user_vouchers`: coupon claim and availability.
- `wanted_merchants` and `wanted_merchant_votes`: customer demand signal and dedupe.
- `reviews`: merchant/dish detail review read boundary.

## Trust, Authorization, And Tenant Checks

- All routes in this slice are under `authGroup`, so anonymous search/public browse is not current behavior.
- Public merchant/dish/combo handlers are customer-facing but must still preserve merchant/item visibility/orderability checks.
- Wanted-merchant and coupon writes use current authenticated user id, not a client-provided user id.
- Region and location selection are client hints for query scoping; backend SQL remains the truth for active region/merchant/item sets.

## Idempotency And Duplicate-Submit Checks

- Search reads/history delete/clear are idempotent from the customer's perspective.
- Wanted-merchant page guards duplicate local submit/vote by `submitting`, `votingId`, and in-flight leaderboard request state.
- Backend wanted-merchant SQL creates or gets active candidate rows and records user votes; repeated submissions should converge to an existing leaderboard row.
- Coupon claim must rely on backend voucher/user_voucher constraints for already-claimed and remaining-quantity enforcement; the frontend `claimingCouponId` guard is only UX-level.

## Recovery And Async Convergence Paths

- Takeout home can retry, pull refresh, region refresh, location change, and pagination.
- Search page can reload initial data, delete/clear history, and rerun suggestions/search.
- Restaurant/dish/combo pages expose retry on load failure and rehydrate public detail on page entry.
- Coupon claim and wanted-merchant submit refresh the affected list after write.
- No provider callback or worker directly owns discovery state in this slice.

## Frontend Draft And Backend Rehydration

- Search keyword, selected category, selected region, and location are local drafting state; results are rehydrated from backend search/public routes.
- Merchant/detail pages can pass ids by navigation params, but the detail payload is backend-sourced.
- Wanted-merchant map pick and candidate text are drafts until `/v1/wanted-merchants/votes` persists them.
- Role-registration navigation from takeout home is not rehydrated as customer discovery state.

## Test Coverage Signals

Observed tests and signals:

- Backend search/public route handlers and wanted merchant handlers have source-level routes and SQL coverage signals through existing API/query code.
- Voucher claim is covered in the voucher/membership/payment-adjacent backend test surface.
- CodeGraph query for `favorite`, `reservations`, and `createOrderFromCart` confirmed duplicated customer API wrappers and entrypoint spread.

Missing high-value tests:

- Mini Program contract test for takeout home category/feed/search/detail navigation under weak network.
- Backend integration test that search results never expose offline/unavailable merchant/dish/combo rows as orderable.
- Wanted-merchant duplicate vote/candidate regression from the customer page through backend SQL.
- Coupon claim idempotency and sold-out/already-claimed UI copy regression.

## Gaps And Refactor Notes

- Takeout home role registration actions are out of customer scope, but they remain reachable from the customer first screen. Future docs should avoid treating those as consumer lifecycle flows.
- Page-local coupon and merchant APIs are copied under feature folders; a contract change should audit all customer copies.
- Search/category/detail views are only as correct as backend orderability filters. Do not move those filters into frontend-only code.

## Branch Exhaustion

- Entry branches checked: takeout home direct open, region/location change, search open, category open, merchant detail, dish detail, combo detail, merchant info, wanted merchants, cart handoff, registration exit.
- Request branches checked: `/v1/search/dishes`, `/merchants`, `/combos`, `/rooms`, `/categories`, `/history`, `/popular`, `/suggestions`, `/public/merchants/:id`, `/dishes`, `/combos`, `/rooms`, `/vouchers/:id/claim`, `/wanted-merchants`, `/wanted-merchants/votes`, `/:id/votes`.
- Backend state branches checked: empty result, popular/history, unavailable search candidate, coupon already claimed/sold out boundary, wanted candidate found-in-rank, new candidate, existing vote.
- Dead/orphan branches checked: no ordinary customer discovery page omitted; role-registration links are explicit non-customer exits.
