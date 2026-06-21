# Packaging Domain Baseline Audit

Date: 2026-06-19
Branch: `feat/packaging-domain-refactor`
Plan source: `artifacts/packaging-domain-production-refactor-development-plan-2026-06-19.md`

## Scope

This audit captures the current "packaging as dish" implementation before adding the dedicated packaging domain.

Risk class: `G3`.

Reason: the planned refactor touches order amount semantics, checkout state, idempotency, object ownership, payment/refund/profit-sharing totals, printing, and historical order evidence.

## Commands Run

Code search:

```bash
rg -n "is_packaging|packaging_required|packing_fee|merchant_packaging|packaging-policy|包装菜品|包装方式|打包费|包装费" locallife weapp web merchant_app artifacts legal_exports
```

Local database snapshot:

```bash
psql "${DB_SOURCE:-postgresql:///locallife_test?sslmode=disable&host=/var/run/postgresql}" -v ON_ERROR_STOP=1 -Atc "
SELECT merchant_id, COUNT(*) AS packaging_dish_count
FROM dishes
WHERE is_packaging = true AND deleted_at IS NULL
GROUP BY merchant_id
ORDER BY merchant_id;
"

psql "${DB_SOURCE:-postgresql:///locallife_test?sslmode=disable&host=/var/run/postgresql}" -v ON_ERROR_STOP=1 -Atc "
SELECT COUNT(*) AS cart_packaging_item_count
FROM cart_items ci
JOIN dishes d ON d.id = ci.dish_id
WHERE d.is_packaging = true AND d.deleted_at IS NULL;

SELECT COUNT(*) AS order_packaging_item_count
FROM order_items oi
JOIN dishes d ON d.id = oi.dish_id
WHERE d.is_packaging = true;

SELECT o.status, COUNT(*) AS order_packaging_item_count
FROM order_items oi
JOIN dishes d ON d.id = oi.dish_id
JOIN orders o ON o.id = oi.order_id
WHERE d.is_packaging = true
GROUP BY o.status
ORDER BY o.status;

SELECT COUNT(*) AS paid_or_active_order_packaging_item_count
FROM order_items oi
JOIN dishes d ON d.id = oi.dish_id
JOIN orders o ON o.id = oi.order_id
WHERE d.is_packaging = true
  AND o.status IN (
    'paid',
    'preparing',
    'ready',
    'courier_accepted',
    'picked',
    'delivering',
    'rider_delivered',
    'user_delivered',
    'completed'
  );

SELECT COUNT(*) AS active_packaging_dish_count
FROM dishes
WHERE is_packaging = true AND deleted_at IS NULL;

SELECT COUNT(*) AS all_packaging_dish_count
FROM dishes
WHERE is_packaging = true;
"
```

Baseline toolchain check:

```bash
PATH="/usr/local/go/bin:$HOME/.local/bin:$PATH"; cd locallife && go test ./logic -run '^$'
PATH="$HOME/.local/bin:$PATH"; cd weapp && npm --version && node --version
```

## Local Data Snapshot

Database source used: `postgresql:///locallife_test?sslmode=disable&host=/var/run/postgresql`.

| Query | Result |
| --- | ---: |
| Active packaging dishes grouped by merchant | no rows |
| Cart items referencing active packaging dishes | 0 |
| Order items referencing packaging dishes | 0 |
| Order items referencing packaging dishes grouped by order status | no rows |
| Paid and post-paid active/completed order items referencing packaging dishes | 0 |
| Active packaging dishes | 0 |
| All packaging dishes including deleted rows | 0 |

Interpretation for the local test database: `no legacy rows`.

Operational caveat: this is local database evidence only. Staging or production-like counts were not available in this worktree session. Before applying Task 8 in a real environment, run the same snapshot queries against the release target database and compare the migrated settings/options count to those results.

## Current Backend Implementation

Legacy schema and history:

- `locallife/db/migration/000010_add_orders.up.sql` originally created `orders.packing_fee`.
- `locallife/db/migration/000062_drop_packing_fee.up.sql` removed `orders.packing_fee` and documented the decision to let merchants sell packaging as a normal product.
- `locallife/db/migration/000181_add_merchant_packaging_policies.up.sql` added an older `merchant_packaging_policies` model with `candidate_dish_ids`.
- `locallife/db/migration/000191_drop_merchant_packaging_policies.up.sql` dropped that older policy table.
- `locallife/db/migration/000190_add_dish_packaging_flag.up.sql` added `dishes.is_packaging` and index `idx_dishes_merchant_packaging_active`.

Persistence surfaces:

- `locallife/db/query/dish.sql` stores and returns `is_packaging` on dish create, update, list, detail, search, combo and count paths.
- `CountActivePackagingDishesByMerchant` counts active packaging dishes by merchant where `is_packaging`, `is_online`, `is_available`, and not deleted are all true.
- `locallife/db/query/cart.sql` projects `d.is_packaging AS dish_is_packaging`, so cart item responses can identify packaging rows.
- `locallife/db/query/combo.sql` includes dish `is_packaging` in combo member projections.

Backend API and logic:

- `locallife/api/dish.go` accepts `is_packaging` on create/update dish requests.
- `normalizePackagingDishCreate` and `resolvePackagingDishUpdate` force packaging dishes to remain online and available.
- `locallife/api/dish.go` also rejects single packaging-dish offline status updates and reports packaging dishes as failed rows during batch offline operations.
- `locallife/api/cart.go` returns each cart item `is_packaging` and a legacy cart-level `packaging_required` boolean.
- `locallife/api/cart.go` computes `packaging_required` only after a cart row exists; the empty-cart not-found branch returns before this requirement is computed.
- `logic.HasPackagingRequirement` checks active packaging dish count for `takeout` and `takeaway`; non-applicable order types return false.
- `locallife/logic/order_service.go` calculates ordinary order items before validating the current packaging policy.
- `OrderService.validatePackagingPolicy` requires exactly one active packaging dish quantity when packaging is required.
- `locallife/logic/order_items.go` treats packaging dish price as ordinary dish item price, so the amount enters food subtotal.
- Current request errors are Chinese product copy: `请先选择包装方式` and `只能选择一种包装方式`.
- Existing tests cover policy behavior in `locallife/logic/packaging_policy_test.go`, order creation expectations in `locallife/logic/order_service_create_test.go`, and API expectations in `locallife/api/cart_test.go`, `locallife/api/order_test.go`, and `locallife/api/dish_test.go`.

Current amount semantics:

- Packaging price enters the order as a normal dish/order item amount.
- There is no durable `orders.packaging_fee` field in current runtime schema.
- There is no `order_packaging_items` snapshot table.
- Payment, refund, fee breakdown, print, and order detail paths can only infer packaging through ordinary order items if those items remain recognizable as packaging dishes.

## Current Weapp Implementation

Merchant side:

- `weapp/miniprogram/pages/merchant/dishes/edit/index.wxml` exposes a "包装菜品" switch in the dish edit page.
- `weapp/miniprogram/pages/merchant/dishes/edit/index.ts` handles switching `is_packaging`.
- `weapp/miniprogram/pages/merchant/_utils/merchant-dish-edit-view.ts` validates packaging dishes and submits `is_packaging`; it also forces `is_available` to true when packaging is enabled.
- `weapp/miniprogram/pages/merchant/dishes/index.ts` blocks taking packaging dishes offline.
- `weapp/miniprogram/pages/merchant/dishes/index.wxml` disables status controls for packaging dishes and tags them as `包装`.
- `weapp/scripts/check-merchant-dish-availability-contract.test.js` has a packaging-dish contract fixture.

Customer side:

- `weapp/miniprogram/api/cart.ts` models `packaging_required` and cart item `is_packaging`.
- `weapp/miniprogram/pages/takeout/cart/_utils/takeout-cart-view.ts` derives checkout blockers by counting selected cart items whose `isPackaging` flag is true.
- `weapp/scripts/check-takeout-cart-packaging-checkout.test.js` locks current behavior: required missing packaging blocks checkout; quantity greater than one blocks checkout.
- Shared generated or copied dish API typings under `weapp/miniprogram/api/dish.ts`, `pages/merchant/_main_shared/api/dish.ts`, `pages/dine-in/_main_shared/api/dish.ts`, `pages/payment/_main_shared/api/dish.ts`, `pages/platform/_main_shared/api/dish.ts`, and takeout detail shared API files expose `is_packaging`.

Documentation and legal references:

- `legal_exports/agreements_v1_2_0/USER_AGREEMENT.html` and migration `000136_update_agreements_v1_2_0_legal.up.sql` mention order amounts may include `打包费`.
- `weapp/docs/MERCHANT_BACKEND_WEAPP_MAPPING_MATRIX_2026-04-06.md` records that older "包装策略" merchant configuration was not actually exposed.
- `artifacts/codegraph/merchant-state-flows/merchant-dish-status-and-inventory.slice.md` records that the packaging switch currently belongs to dish management.

## Migration Risk Classification

Local database classification: `no legacy rows`.

Release-target classification must be recalculated against the target database immediately before Task 8:

- `no legacy rows`: direct cutover remains safe for data migration.
- `legacy merchant rows only`: migrate merchant options from active packaging dishes, then freeze old write paths after frontend cutover.
- `legacy paid orders`: do not rewrite historical `order_items`; preserve old item display while new orders use `orders.packaging_fee` and `order_packaging_items`.

## Risks To Carry Into Later Tasks

- Search/menu/recommendation paths currently can expose packaging dishes because public dish queries return `is_packaging` but do not universally exclude it.
- Existing checkout relies on item quantity, not a dedicated packaging selection row; stale checkout must be redesigned around `selection_version`.
- Empty cart responses currently omit computed packaging requirement, so Task 4/5 must make the backend packaging contract available without relying on a packaging item already being in the cart.
- Existing order creation validates the packaging dish by re-reading current dish state; historical order display has no independent packaging snapshot.
- Merchant configuration is embedded in high-frequency dish management UI, so Task 10 must remove the dish switch only after backend additive APIs exist.
- The old `merchant_packaging_policies` migration pair is historical dead policy surface; new schema should not reuse that table shape.
- Multiple weapp API wrapper copies expose `is_packaging`; frontend cutover must update the active wrappers and avoid leaving customer paths dependent on packaging-as-dish.
- Local DB has zero legacy rows, but target environment data remains the real migration gate.

## Task 0 Review Gate Result

Migration scope is understood for local development: no local legacy rows exist, and no local paid order packaging rows exist.

Production rollout remains gated on running the same snapshot queries against the release target database before applying the migration task. The planned additive-first sequencing remains appropriate because code and UI references to `is_packaging` are broad and cannot be removed safely before backend compatibility and weapp cutover are complete.
