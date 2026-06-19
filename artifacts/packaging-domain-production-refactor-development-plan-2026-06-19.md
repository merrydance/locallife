# Packaging Domain Production Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将当前“包装作为菜品”的过渡实现升级为生产级包装业务域，使商户包装配置、顾客包装选择、订单金额、订单快照、打印、退款、分账和财务展示都拥有明确语义。

**Architecture:** 采用 additive-first 的演进方式：先新增包装设置、包装选项、购物车包装选择、订单包装快照和订单包装费字段，再逐步切换后端主链与小程序入口，最后通过服务端开关冻结旧 `dishes.is_packaging` 写入。订单创建事务是唯一的订单包装快照写入边界；购物车和订单确认页的预览金额必须由后端按同一公式返回 `packaging_fee` 与 `total_amount`；所有支付、退款、分账、打印和展示均读取订单持久化金额与快照，不读取当前商户配置回推历史订单。

**Tech Stack:** Go / Gin / pgx / sqlc / PostgreSQL / Go tests / sqlc generated mocks / WeChat Mini Program TypeScript / TDesign Miniprogram / LocalLife backend and weapp validation commands.

---

## 1. Background

当前实现中，包装以 `dishes.is_packaging=true` 的特殊菜品存在：

- 商户在菜品管理里创建或编辑包装菜品。
- 包装菜品被强制 `is_online=true`、`is_available=true`。
- 外卖 `takeout` 和自取 `takeaway` 如果商户存在 active 包装菜品，则创建订单时必须恰选 1 个包装菜品。
- 包装价格以普通菜品行进入 `orders.subtotal` 和 `order_items`。
- 订单明细没有包装语义快照；后续费用明细、打印、退款、分账只能把包装当作普通商品。

这套实现适合 MVP，但不适合正式向顾客推广后的生产长期模型。包装不是菜品，它有独立的业务语义、账单语义、顾客认知和商户配置习惯。继续保留当前模型会让包装混入菜单、搜索、优惠门槛、菜品统计、商户费用明细、退款解释和小票语义。

当前商户已有入驻，但尚未正式向顾客推广，因此现在是改造窗口：可以采用一次结构性迁移，避免上线后背负大量历史订单和顾客路径。

## 2. Target Product Semantics

### 2.1 Business Semantics

- 包装是商户包装配置，不是菜品。
- 包装仅适用于 `takeout` 和 `takeaway`。
- 首版仅支持“每单选择一种包装方式”，与当前“恰选 1 个包装菜品”的业务能力等价。
- 堂食 `dine_in` 和预约点菜 `reservation` 默认不需要包装。
- 包装费单独展示为 `packaging_fee`，不计入菜品小计 `subtotal`。
- 包装费参与订单用户实付和商户应收；首版不参与满减、券门槛和菜品优惠。
- 历史订单必须保存包装快照，不受商户后续改名、改价、禁用或删除包装选项影响。

### 2.2 User-Facing Semantics

商户端：

- 商户在“包装设置”页面维护包装启用状态、适用订单类型、是否必选和包装选项。
- 菜品管理不再展示“包装菜品”开关。
- 包装设置页面是低频配置入口，不应挤占高频菜品上下架、库存和订单处理路径。

顾客端：

- 菜单、搜索、扫码点餐和推荐流不出现包装项。
- 顾客在购物车或确认页看到“包装方式”，按商户配置选择。
- 金额区单独展示“包装费”。
- 如果包装必选且仅有一个启用选项，可以自动选中并展示。
- 如果包装必选且有多个启用选项，必须由顾客选择。

订单与运营：

- 订单详情、商户订单详情、打印小票、客服和退款上下文均能看到订单当时的包装名称、单价、数量和金额。
- 商户费用明细中包装费与菜品合计分开展示，商户实收包含包装费。

## 3. Non-Goals

本轮不做以下能力：

- 按菜品数量自动计算每份餐盒费。
- 按冷热、餐品类型、规格或重量自动分配包装。
- 每单选择多个包装方式。
- 包装库存。
- 包装图片、规格和复杂定制项。
- 运营后台全局包装模板。
- 立即物理删除 `dishes.is_packaging` 字段。

这些能力可在新模型稳定后扩展。首版必须保持业务面收敛，降低金额、支付、退款和分账改造风险。

## 4. Risk Classification

整体风险等级：`G3`。

原因：

- 触及订单金额公式、支付金额、退款金额、分账金额、商户财务展示和订单创建事务。
- 触及顾客下单路径、商户配置越权边界、购物车状态、订单幂等和历史数据迁移。
- 如果处理不当，会造成金额不一致、重复收费、漏收包装费、跨商户包装选择、历史订单展示漂移或无法退款解释。

执行要求：

- 每个任务只能修改本任务声明的边界。
- 每个任务完成后必须 review；review 有问题先修复并重新验证，没有问题再提交本任务。
- 不允许跨任务攒大提交。
- 高风险任务必须有 focused tests；金额和支付相关任务必须运行 `make test-safety` 或文档中列出的等价 focused safety tests。
- 不允许在事务中加入外部 I/O、websocket emit、打印、支付 provider 调用或异步 side effect。

## 5. Target Data Model

### 5.1 New Tables

#### `merchant_packaging_settings`

One row per merchant.

Fields:

- `id BIGSERIAL PRIMARY KEY`
- `merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE UNIQUE`
- `enabled BOOLEAN NOT NULL DEFAULT false`
- `required BOOLEAN NOT NULL DEFAULT true`
- `applicable_order_types TEXT[] NOT NULL DEFAULT ARRAY['takeout','takeaway']::TEXT[]`
- `default_option_id BIGINT NULL`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- `updated_at TIMESTAMPTZ`

Constraints:

- `applicable_order_types` contains only `takeout` and `takeaway`.
- `default_option_id` is checked in logic because it must belong to the same merchant and be enabled.

#### `merchant_packaging_options`

商户包装选项。

Fields:

- `id BIGSERIAL PRIMARY KEY`
- `merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE`
- `legacy_dish_id BIGINT NULL REFERENCES dishes(id)`
- `name TEXT NOT NULL`
- `description TEXT`
- `price BIGINT NOT NULL DEFAULT 0`
- `is_enabled BOOLEAN NOT NULL DEFAULT true`
- `sort_order SMALLINT NOT NULL DEFAULT 0`
- `deleted_at TIMESTAMPTZ`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- `updated_at TIMESTAMPTZ`

Indexes and constraints:

- `CREATE INDEX idx_merchant_packaging_options_merchant_active ON merchant_packaging_options(merchant_id, is_enabled, sort_order, id) WHERE deleted_at IS NULL;`
- `CREATE UNIQUE INDEX uq_merchant_packaging_options_legacy_dish ON merchant_packaging_options(legacy_dish_id) WHERE legacy_dish_id IS NOT NULL;`
- `CREATE UNIQUE INDEX uq_merchant_packaging_options_name_active ON merchant_packaging_options(merchant_id, lower(name)) WHERE deleted_at IS NULL;`
- `price >= 0`
- `char_length(trim(name)) BETWEEN 1 AND 50`

#### `cart_packaging_selections`

购物车当前包装选择。首版每个 cart 最多一个包装选择。

Fields:

- `cart_id BIGINT PRIMARY KEY REFERENCES carts(id) ON DELETE CASCADE`
- `packaging_option_id BIGINT NULL REFERENCES merchant_packaging_options(id)`
- `selection_version BIGINT NOT NULL DEFAULT 1`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`

Semantics:

- Missing selection row means explicit no selection with `selection_version=0`.
- `packaging_option_id IS NULL` in an existing row means the user cleared a previous selection; clearing preserves the row to keep a monotonic `selection_version`.
- `PUT` selection 是幂等操作；同一 cart 重复选择同一 option 得到同一状态，并且不得递增 `selection_version`。
- `selection_version` 仅在有效包装选择发生变化时递增，订单确认页和订单创建幂等 hash 使用它检测 stale checkout。
- 选择校验必须确认 cart 属于当前用户、option 属于 cart merchant、option enabled、not deleted。

#### `order_packaging_items`

订单包装快照。首版每个 order 最多一行，DB 通过 `UNIQUE(order_id)` 兜底；未来支持多包装时必须先用单独迁移放开该约束，并重新 review 金额、退款、打印和展示任务卡。

Fields:

- `id BIGSERIAL PRIMARY KEY`
- `order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE`
- `packaging_option_id BIGINT NULL REFERENCES merchant_packaging_options(id)`
- `name TEXT NOT NULL`
- `unit_price BIGINT NOT NULL`
- `quantity SMALLINT NOT NULL DEFAULT 1`
- `subtotal BIGINT NOT NULL`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`

Constraints:

- `unit_price >= 0`
- `quantity > 0`
- `subtotal = unit_price * quantity`
- `char_length(trim(name)) BETWEEN 1 AND 50`
- `UNIQUE(order_id)` for the first per-order packaging release.

### 5.2 Existing Table Changes

Add to `orders`:

- `packaging_fee BIGINT NOT NULL DEFAULT 0`

Do not reuse old `packing_fee`; use `packaging_fee` for new API and internal DTOs.

Do not drop `dishes.is_packaging` in this rollout. It remains a legacy compatibility signal until old merchant data has been migrated and old write paths are frozen.

## 6. Target Amount Formula

Current:

```text
total_amount = subtotal - discount_amount - voucher_amount + delivery_fee - delivery_fee_discount
```

Target:

```text
food_payable = subtotal - discount_amount - voucher_amount
delivery_payable = delivery_fee - delivery_fee_discount
total_amount = food_payable + packaging_fee + delivery_payable
```

Rules:

- `subtotal` is dish/combo food subtotal only.
- `packaging_fee` is not part of food subtotal.
- `discount_amount` and `voucher_amount` apply to `subtotal` only in this release.
- `total_amount` is clamped to `0` only after all components are computed, preserving existing behavior for over-discount cases.
- Merchant receivable includes food payable plus packaging fee minus platform/payment fees according to settlement calculation.
- Rider amount remains based on delivery fee fields, not packaging fee.
- `calculateCart` and any order preview endpoint must return backend-computed `packaging_fee` and `total_amount` using this same formula.
- Frontend code must not compute customer payable by locally adding packaging fee to other response fields; it may render backend fields only.

## 7. Security, Authorization, And Idempotency Rules

### 7.1 Authorization

Merchant configuration:

- Packaging settings and options are merchant-owned resources.
- API handlers must resolve current merchant from authenticated merchant staff context.
- Owner and manager may update packaging settings and options.
- Chef/cashier must not update packaging settings or options unless existing project policy explicitly grants them merchant configuration writes.
- Every update/delete/read-by-id must verify `merchant_id` ownership server-side.
- The client must never be trusted to provide price, merchant_id, enabled state, or historical snapshot values during order creation.

Customer cart and order:

- Cart packaging selection must verify cart ownership by current authenticated user.
- Option must belong to cart merchant and be enabled at selection time.
- Order creation must revalidate option ownership, enabled state, settings applicability, order type, and required rule inside the order creation flow.
- Direct order creation must not trust frontend cart snapshots.

### 7.2 Idempotency

Merchant settings:

- `PUT /v1/merchant/packaging-settings` is convergent and idempotent.
- Repeating the same body results in the same durable row.

Merchant options:

- `POST /v1/merchant/packaging-options` is duplicate-guarded by active `(merchant_id, lower(name))` uniqueness.
- UI must disable duplicate submit, but backend uniqueness is the final duplicate guard.
- `PUT /v1/merchant/packaging-options/{id}` is idempotent for the same body.
- `DELETE /v1/merchant/packaging-options/{id}` is soft delete and idempotent: deleting an already-deleted row returns success only if the row belongs to the current merchant.

Cart packaging selection:

- `PUT /v1/cart/packaging-selection` is idempotent by `cart_id` primary key upsert.
- Repeating selection or clear yields the same cart selection and same `selection_version`.
- Changing from one effective option to another, from selected to cleared, or from cleared to selected increments `selection_version`.

Order creation:

- Existing `Idempotency-Key` remains the order create idempotency guard.
- Customer order creation must carry backend-issued packaging selection identity from the latest cart/preview response: `packaging_option_id` or explicit none, plus `packaging_selection_version`.
- The order request hash must include packaging source (`cart`, `direct`, or `none`), cart id when cart-backed, resolved option id or explicit none, and `packaging_selection_version`.
- New order creation must validate that the cart's current `selection_version` and selected option still match the request identity before writing the order; stale identity returns a conflict asking the user to refresh checkout.
- If order create idempotency replays an existing bound order, response must return the existing order and packaging snapshot without revalidating current cart selection or current merchant packaging settings.
- `CreateOrderTx` must write order row, order items, order packaging items, voucher state, balance transaction, and idempotency binding in one transaction.

Migration:

- Legacy packaging migration is idempotent through `merchant_packaging_options.legacy_dish_id` uniqueness.
- Re-running migration must not create duplicate packaging options or settings.

### 7.3 State Sequencing

Required sequence:

1. Add schema and generated query surfaces.
2. Add backend read/write capability behind additive APIs without removing legacy behavior.
3. Add new amount model and order snapshot in backend tests.
4. Add backend preview amount contract that includes packaging fee before customer frontend cutover.
5. Migrate existing `is_packaging` dishes to packaging options.
6. Add legacy packaging dish freeze gate behind a disabled-by-default server flag.
7. Switch weapp merchant settings and customer checkout to new APIs.
8. Enable the legacy freeze flag only after frontend cutover smoke passes.
9. Run global review and safety validation.
10. Only after one stable release, schedule optional cleanup of `dishes.is_packaging`.

Prohibited sequence:

- Do not remove `is_packaging` before migration and frontend cutover.
- Do not switch frontend before backend supports both old and new read paths.
- Do not enable legacy write freeze before weapp merchant settings and customer checkout have been deployed.
- Do not alter payment or refund calculations before order creation stores `packaging_fee`.
- Do not rely on merchant settings at order display time for historical packaging values.

## 8. Execution Loop

Every task follows this loop:

1. Implement only the files listed in the task.
2. Write or update focused tests before or alongside implementation.
3. Run the task's targeted validation commands.
4. Self-review the diff for authz, idempotency, sequencing, failure behavior, generated artifacts, and file-size guardrails.
5. Request or perform task review.
6. If review finds issues, fix within the same task and rerun targeted validation.
7. Commit only that task's files.
8. Move to the next task only after the previous task is reviewed, fixed, validated, and committed.

Suggested commit style:

```bash
git add <task files>
git commit -m "feat: add packaging settings persistence"
```

Never batch multiple tasks into one commit.

## 9. Task Cards

### Task 0: Baseline Audit And Data Snapshot

**Purpose:** Establish current runtime facts and migration risk before schema changes.

**Files:**

- Create: `artifacts/packaging-domain-baseline-audit-2026-06-19.md`
- Read only: `locallife/db/migration/000062_drop_packing_fee.up.sql`
- Read only: `locallife/db/migration/000190_add_dish_packaging_flag.up.sql`
- Read only: `locallife/logic/packaging_policy.go`
- Read only: `locallife/api/cart.go`
- Read only: `weapp/miniprogram/pages/takeout/cart/_utils/takeout-cart-view.ts`

**Steps:**

- [ ] Run code search:

```bash
rg -n "is_packaging|packaging_required|packing_fee|merchant_packaging|packaging-policy|包装菜品|包装方式|打包费|包装费" locallife weapp web merchant_app artifacts legal_exports
```

- [ ] Run DB snapshot queries against staging or local production-like DB:

```sql
SELECT merchant_id, COUNT(*) AS packaging_dish_count
FROM dishes
WHERE is_packaging = true AND deleted_at IS NULL
GROUP BY merchant_id
ORDER BY merchant_id;

SELECT COUNT(*) AS cart_packaging_item_count
FROM cart_items ci
JOIN dishes d ON d.id = ci.dish_id
WHERE d.is_packaging = true AND d.deleted_at IS NULL;

SELECT COUNT(*) AS order_packaging_item_count
FROM order_items oi
JOIN dishes d ON d.id = oi.dish_id
WHERE d.is_packaging = true;
```

- [ ] Record counts, affected merchants, and whether real paid orders contain packaging dishes.
- [ ] Classify migration risk:
  - `no legacy rows`: direct cutover.
  - `legacy merchant rows only`: migrate merchant options.
  - `legacy paid orders`: preserve old order item display and avoid retroactive mutation.
- [ ] Review gate: confirm migration scope is understood before Task 1.
- [ ] Commit:

```bash
git add artifacts/packaging-domain-baseline-audit-2026-06-19.md
git commit -m "docs: capture packaging domain baseline audit"
```

**Validation:** Document-only task; verify audit file exists and has no placeholder sections:

```bash
test -f artifacts/packaging-domain-baseline-audit-2026-06-19.md
rg -n "[T]BD|[T]ODO|待[补]|待[定]" artifacts/packaging-domain-baseline-audit-2026-06-19.md
```

Expected: `test` exits 0; `rg` exits 1.

### Task 1: Add Packaging Schema And SQLC Queries

**Purpose:** Add the durable model without changing runtime behavior.

**Files:**

- Create: `locallife/db/migration/<next>_add_packaging_domain.up.sql`
- Create: `locallife/db/migration/<next>_add_packaging_domain.down.sql`
- Create: `locallife/db/query/packaging.sql`
- Modify generated: `locallife/db/sqlc/*.sql.go`
- Modify generated: `locallife/db/sqlc/models.go`
- Modify generated: `locallife/db/sqlc/querier.go`
- Modify generated mocks if sqlc mock generation updates them: `locallife/db/mock/store.go`

**SQL migration shape:**

```sql
CREATE TABLE merchant_packaging_settings (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT false,
    required BOOLEAN NOT NULL DEFAULT true,
    applicable_order_types TEXT[] NOT NULL DEFAULT ARRAY['takeout','takeaway']::TEXT[],
    default_option_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    CONSTRAINT merchant_packaging_settings_order_types_check CHECK (
        applicable_order_types <@ ARRAY['takeout','takeaway']::TEXT[]
    )
);

CREATE TABLE merchant_packaging_options (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    legacy_dish_id BIGINT REFERENCES dishes(id),
    name TEXT NOT NULL,
    description TEXT,
    price BIGINT NOT NULL DEFAULT 0,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    sort_order SMALLINT NOT NULL DEFAULT 0,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    CONSTRAINT merchant_packaging_options_price_check CHECK (price >= 0),
    CONSTRAINT merchant_packaging_options_name_check CHECK (char_length(trim(name)) BETWEEN 1 AND 50)
);

CREATE INDEX idx_merchant_packaging_options_merchant_active
ON merchant_packaging_options(merchant_id, is_enabled, sort_order, id)
WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uq_merchant_packaging_options_legacy_dish
ON merchant_packaging_options(legacy_dish_id)
WHERE legacy_dish_id IS NOT NULL;

CREATE UNIQUE INDEX uq_merchant_packaging_options_name_active
ON merchant_packaging_options(merchant_id, lower(name))
WHERE deleted_at IS NULL;

CREATE TABLE cart_packaging_selections (
    cart_id BIGINT PRIMARY KEY REFERENCES carts(id) ON DELETE CASCADE,
    packaging_option_id BIGINT REFERENCES merchant_packaging_options(id),
    selection_version BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE order_packaging_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    packaging_option_id BIGINT REFERENCES merchant_packaging_options(id),
    name TEXT NOT NULL,
    unit_price BIGINT NOT NULL,
    quantity SMALLINT NOT NULL DEFAULT 1,
    subtotal BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT order_packaging_items_price_check CHECK (unit_price >= 0),
    CONSTRAINT order_packaging_items_quantity_check CHECK (quantity > 0),
    CONSTRAINT order_packaging_items_subtotal_check CHECK (subtotal = unit_price * quantity),
    CONSTRAINT order_packaging_items_name_check CHECK (char_length(trim(name)) BETWEEN 1 AND 50)
);

CREATE UNIQUE INDEX uq_order_packaging_items_order
ON order_packaging_items(order_id);

ALTER TABLE orders
ADD COLUMN packaging_fee BIGINT NOT NULL DEFAULT 0;
```

**Query surface required in `packaging.sql`:**

- `GetMerchantPackagingSettings :one`
- `UpsertMerchantPackagingSettings :one`
- `CreateMerchantPackagingOption :one`
- `GetMerchantPackagingOption :one`
- `GetMerchantPackagingOptionForUpdate :one`
- `ListMerchantPackagingOptions :many`
- `ListEnabledMerchantPackagingOptions :many`
- `UpdateMerchantPackagingOption :one`
- `SoftDeleteMerchantPackagingOption :one`
- `GetCartPackagingSelection :one`
- `UpsertCartPackagingSelection :one`
- `ClearCartPackagingSelection :one`
- `UpdateCartPackagingSelectionIfChanged :one`
- `CreateOrderPackagingItem :one`
- `ListOrderPackagingItems :many`
- `ListOrderPackagingItemsByOrderIDs :many`

**Steps:**

- [ ] Write migration up/down.
- [ ] Write `packaging.sql` with explicit column lists and stable ordering.
- [ ] Run:

```bash
cd locallife
make sqlc
```

Expected: generated sqlc files update without errors.

- [ ] Run:

```bash
cd locallife
make check-generated
```

Expected: generated artifacts match sources.

- [ ] Add or update db/sqlc tests for migration shape and unique constraints if existing DB test style supports it.
- [ ] Review gate: SQL review checks no broad update/delete, no `SELECT *`, every query has a planned caller, and generated code is not hand-edited.
- [ ] Commit:

```bash
git add locallife/db/migration locallife/db/query/packaging.sql locallife/db/sqlc locallife/db/mock
git commit -m "feat: add packaging domain persistence"
```

**Validation:**

```bash
cd locallife
make sqlc
make check-generated
go test ./db/sqlc -run 'Test.*Packaging|Test.*Migration' -count=1
```

### Task 2: Add Backend Packaging Domain Logic

**Purpose:** Centralize packaging policy, option validation, snapshot building, and amount calculation outside handlers.

**Files:**

- Create: `locallife/logic/packaging_service.go`
- Create: `locallife/logic/packaging_service_test.go`
- Modify: `locallife/logic/order_payment.go`
- Modify: `locallife/logic/merchant_order_fee_breakdown.go`

**Core types:**

```go
type PackagingRequirement struct {
    Enabled bool
    Required bool
    Applicable bool
    Options []db.MerchantPackagingOption
    SelectedOption *db.MerchantPackagingOption
}

type ResolvePackagingInput struct {
    UserID int64
    MerchantID int64
    OrderType string
    CartID *int64
    PackagingOptionID *int64
}

type OrderPackagingSnapshot struct {
    PackagingOptionID *int64
    Name string
    UnitPrice int64
    Quantity int16
    Subtotal int64
}
```

**Required behavior:**

- Non-applicable order types return no requirement and no error.
- Missing settings means disabled.
- Enabled + required + no selected option returns request error `"请先选择包装方式"` for customer flows.
- Selected option must belong to merchant, be enabled, and not deleted.
- Disabled settings ignore cart selection.
- `BuildOrderPackagingSnapshot` uses current option name and price once, then downstream persists the snapshot.
- `ComputeOrderTotals` accepts `PackagingFee` and includes it in total amount.
- Merchant fee breakdown exposes `PackagingFeeAmount`.

**Steps:**

- [ ] Write failing tests:
  - non-applicable order type returns no requirement.
  - enabled required missing option fails.
  - selected option from another merchant fails.
  - disabled option fails.
  - valid selected option builds snapshot and amount.
  - `ComputeOrderTotals` includes packaging fee.
  - merchant fee breakdown includes packaging fee and keeps customer payable consistent.
- [ ] Implement `packaging_service.go` using `db.Store`.
- [ ] Update `OrderTotalsInput` and tests.
- [ ] Update `MerchantOrderFeeBreakdown` and tests.
- [ ] Review gate: logic contains business rules; handlers remain transport-only in later tasks.
- [ ] Commit:

```bash
git add locallife/logic/packaging_service.go locallife/logic/packaging_service_test.go locallife/logic/order_payment.go locallife/logic/merchant_order_fee_breakdown.go locallife/logic/*_test.go
git commit -m "feat: add packaging domain logic"
```

**Validation:**

```bash
cd locallife
go test ./logic -run 'Test.*Packaging|TestComputeOrderTotals|TestBuildMerchantOrderFeeBreakdown' -count=1
```

### Task 3: Add Merchant Packaging APIs

**Purpose:** Give merchants a dedicated packaging management contract and remove the need to create packaging through dish management.

**Files:**

- Create: `locallife/api/merchant_packaging.go`
- Create: `locallife/api/merchant_packaging_test.go`
- Modify: `locallife/api/server.go`
- Modify generated Swagger after annotations: `locallife/docs/docs.go`, `locallife/docs/swagger.json`, `locallife/docs/swagger.yaml`

**Routes:**

```text
GET    /v1/merchant/packaging-settings
PUT    /v1/merchant/packaging-settings
GET    /v1/merchant/packaging-options
POST   /v1/merchant/packaging-options
PUT    /v1/merchant/packaging-options/:id
DELETE /v1/merchant/packaging-options/:id
```

**Request rules:**

- Settings:
  - `enabled`: required bool.
  - `required`: required bool.
  - `applicable_order_types`: optional, allowed only `takeout`, `takeaway`; default both.
  - `default_option_id`: optional; if present, must belong to merchant and be enabled.
- Option create/update:
  - `name`: 1-50 chars after trim.
  - `description`: max 200 chars.
  - `price`: 0-9999900 cents.
  - `is_enabled`: bool.
  - `sort_order`: 0-999.

**Authorization:**

- Use merchant staff middleware.
- Logic must recheck merchant ownership for every option id.
- Owner and manager are allowed. Cashier/chef are denied unless current route group policy already grants equivalent merchant settings writes.

**Steps:**

- [ ] Write API tests:
  - owner can upsert settings.
  - manager can upsert settings if policy allows manager.
  - non-merchant denied.
  - foreign option update denied.
  - invalid order type rejected.
  - duplicate active option name rejected.
  - soft delete is idempotent for owned option.
- [ ] Implement handler DTOs and route wiring.
- [ ] Add Swagger annotations.
- [ ] Run Swagger generation:

```bash
cd locallife
make swagger
make check-generated
```

- [ ] Review gate: verify no client-provided merchant_id is trusted; every resource id checks ownership.
- [ ] Commit:

```bash
git add locallife/api/merchant_packaging.go locallife/api/merchant_packaging_test.go locallife/api/server.go locallife/docs
git commit -m "feat: add merchant packaging APIs"
```

**Validation:**

```bash
cd locallife
go test ./api -run 'TestMerchantPackaging' -count=1
make swagger
make check-generated
```

### Task 4: Add Customer Cart Packaging Selection APIs

**Purpose:** Let customers select packaging as cart state instead of adding a packaging dish item.

**Files:**

- Modify: `locallife/api/cart.go`
- Modify: `locallife/api/cart_test.go`
- Modify: `locallife/logic/cart_response.go`
- Modify: `locallife/api/server.go`
- Modify generated Swagger: `locallife/docs/*`

**Routes:**

```text
PUT    /v1/cart/packaging-selection
DELETE /v1/cart/packaging-selection
```

**Cart response additions:**

```json
{
  "packaging": {
    "enabled": true,
    "required": true,
    "applicable": true,
    "selected_option_id": 1001,
    "selection_version": 3,
    "options": [
      {
        "id": 1001,
        "name": "普通餐盒",
        "description": "",
        "price": 100,
        "is_enabled": true,
        "sort_order": 0
      }
    ]
  }
}
```

Keep legacy `packaging_required` temporarily for compatibility, mapped from `packaging.enabled && packaging.required && packaging.applicable`.

**Request shape:**

```json
{
  "merchant_id": 1,
  "order_type": "takeout",
  "table_id": null,
  "reservation_id": null,
  "packaging_option_id": 1001
}
```

**Response/version semantics:**

- Selection responses include `selection_version`.
- If no selection row exists, responses report explicit no selection with `selection_version=0`.
- Repeating the same `PUT` body returns the same selected option and same `selection_version`.
- Changing selected option, clearing a selected option, or selecting after a cleared state increments `selection_version`.
- Checkout and order creation use `selection_version` to reject stale packaging state instead of silently charging a different packaging fee.

**Steps:**

- [ ] Write cart tests:
  - cart response includes packaging options and selected option.
  - `PUT` validates cart belongs to current user.
  - `PUT` rejects option from another merchant.
  - `PUT` rejects disabled/deleted option.
  - `DELETE` clears selection idempotently.
  - repeating the same `PUT` does not increment `selection_version`.
  - changing option increments `selection_version`.
  - legacy `packaging_required` remains correct.
- [ ] Implement response DTO and selection handlers.
- [ ] Route new handlers.
- [ ] Update Swagger.
- [ ] Review gate: verify no cross-user cart access and no cross-merchant option selection.
- [ ] Commit:

```bash
git add locallife/api/cart.go locallife/api/cart_test.go locallife/logic/cart_response.go locallife/api/server.go locallife/docs
git commit -m "feat: add cart packaging selection"
```

**Validation:**

```bash
cd locallife
go test ./api -run 'Test.*Cart.*Packaging|Test.*PackagingSelection' -count=1
make swagger
make check-generated
```

### Task 5: Update Cart Preview Packaging Amount Contract

**Purpose:** Ensure checkout preview and order creation use the same backend-owned packaging amount formula before customer frontend cutover.

**Files:**

- Modify: `locallife/logic/cart_calculation.go`
- Modify: `locallife/logic/order_calculation.go` only if existing preview helpers share totals.
- Modify: `locallife/logic/order_calculation_test.go`
- Modify: `locallife/api/cart.go`
- Modify: `locallife/api/cart_test.go`
- Modify generated Swagger after annotations: `locallife/docs/*`

**Response additions for `/v1/cart/calculate`:**

```json
{
  "subtotal": 3000,
  "packaging_fee": 100,
  "delivery_fee": 500,
  "delivery_fee_discount": 0,
  "discount_amount": 300,
  "total_amount": 3300,
  "packaging": {
    "enabled": true,
    "required": true,
    "applicable": true,
    "selected_option_id": 1001,
    "selection_version": 3,
    "fee": 100
  }
}
```

**Required behavior:**

- `CalculateCartPreview` resolves packaging through the same domain service as order creation.
- Preview returns backend-computed `packaging_fee` and `total_amount`; frontend must render these values directly.
- Packaging fee is excluded from food subtotal, merchant discount threshold, voucher threshold, and delivery fee calculation input.
- Required packaging with no selected option returns a request error and blocks checkout.
- Stale selected option, disabled option, deleted option, or option from another merchant fails closed.
- Preview exposes `selection_version` so order creation can validate checkout freshness.
- A focused test proves preview total equals create-order total for the same cart, selected packaging, delivery fee, and discounts.

**Steps:**

- [ ] Extend cart preview result and API response with `packaging_fee` and packaging selection identity.
- [ ] Use packaging domain logic to resolve selected packaging from `cart_packaging_selections`.
- [ ] Update total formula in preview to match Section 6.
- [ ] Add tests:
  - selected packaging adds fee to preview total.
  - required missing packaging returns request error.
  - packaging fee does not change discount/voucher threshold input.
  - preview total matches order create total for the same inputs.
  - stale/foreign/disabled/deleted option fails closed.
- [ ] Update Swagger.
- [ ] Review gate: verify no frontend-owned amount calculation is required and preview/create formulas remain identical.
- [ ] Commit:

```bash
git add locallife/logic/cart_calculation.go locallife/logic/order_calculation.go locallife/logic/order_calculation_test.go locallife/api/cart.go locallife/api/cart_test.go locallife/docs
git commit -m "feat: include packaging in cart preview totals"
```

**Validation:**

```bash
cd locallife
go test ./logic -run 'TestCalculateCartPreview.*Packaging|TestCalculateOrderPreview.*Packaging|TestComputeOrderTotals' -count=1
go test ./api -run 'TestCalculateCart.*Packaging' -count=1
make swagger
make check-generated
```

### Task 6: Update Order Creation Transaction And Idempotency

**Purpose:** Persist packaging fee and packaging snapshot as part of order creation.

**Files:**

- Modify: `locallife/logic/order_service.go`
- Modify: `locallife/logic/interfaces.go`
- Modify: `locallife/logic/order_service_create_test.go`
- Modify: `locallife/api/order.go` if create-order request DTO needs packaging identity fields.
- Modify: `locallife/api/order_test.go` if handler request mapping changes.
- Modify: `locallife/db/sqlc/tx_create_order.go`
- Modify: `locallife/db/sqlc/tx_create_order_test.go`
- Modify: `locallife/db/query/order.sql`
- Modify: `locallife/db/query/order_item.sql` only if response preload changes.
- Modify generated sqlc files and mocks.

**Required behavior:**

- Order create validates packaging after food items are calculated and before totals are computed.
- Customer order create request includes packaging identity from the latest preview: `packaging_option_id` or explicit none, plus `packaging_selection_version`.
- If cart selection is authoritative, order create validates the current cart selected option and `selection_version` match the request before creating a new order.
- If an existing idempotency binding already has an order, replay returns that order and packaging snapshot without re-reading current cart selection.
- `CreateOrderParams.PackagingFee` is set before `CreateOrderTx`.
- `CreateOrderTx` writes `order_packaging_items` in the same transaction as order and order items.
- Existing order create idempotency hash includes packaging source, cart id, option id or explicit none, and `packaging_selection_version`.
- Idempotency replay returns existing order and packaging snapshot.
- Reusing the same `Idempotency-Key` with a different packaging identity returns conflict before an order is bound; once an order is bound, replay returns the existing order and snapshot.
- Full balance payment branch keeps current atomic behavior and includes packaging fee in total amount.
- No external side effects inside transaction.

**Steps:**

- [ ] Update order SQL and generated code to include `packaging_fee` in order selects/inserts.
- [ ] Add transaction params for packaging snapshots:

```go
type CreateOrderTxParams struct {
    CreateOrderParams db.CreateOrderParams
    Items []db.CreateOrderItemParams
    PackagingItems []db.CreateOrderPackagingItemParams
    // existing fields remain
}
```

- [ ] Write transaction tests:
  - snapshot rows are written when packaging fee exists.
  - failed snapshot insert rolls back order.
  - idempotent replay returns existing order without duplicate snapshot.
- [ ] Write logic tests:
  - required missing packaging rejects order.
  - valid packaging adds packaging fee and snapshot.
  - `dine_in` ignores packaging settings.
  - foreign option rejected.
  - total amount includes packaging fee.
  - stale `packaging_selection_version` rejects with conflict.
  - same `Idempotency-Key` with changed packaging identity conflicts before order binding.
  - idempotent replay returns existing snapshot without reading current merchant option price/name.
- [ ] Implement logic and transaction changes.
- [ ] Review gate: inspect order create sequence, idempotency hash, transaction rollback, balance-payment branch, and no side effects inside tx.
- [ ] Commit:

```bash
git add locallife/logic/order_service.go locallife/logic/interfaces.go locallife/logic/order_service_create_test.go locallife/api/order.go locallife/api/order_test.go locallife/db/sqlc/tx_create_order.go locallife/db/sqlc/tx_create_order_test.go locallife/db/query locallife/db/sqlc locallife/db/mock
git commit -m "feat: persist order packaging snapshots"
```

**Validation:**

```bash
cd locallife
make sqlc
make check-generated
go test ./logic -run 'TestOrderServiceCreateOrder.*Packaging|TestOrderServiceCreateOrder.*Idempotency' -count=1
go test ./db/sqlc -run 'TestCreateOrderTx.*Packaging|TestCreateOrderTx.*Idempotency' -count=1
```

### Task 7: Update Payment, Refund, Profit Sharing, Fee Breakdown, And Order Responses

**Purpose:** Ensure all money and display paths use the new packaging amount consistently.

**Files:**

- Modify: `locallife/logic/order_payment.go`
- Modify: `locallife/logic/merchant_order_fee_breakdown.go`
- Modify: `locallife/api/order_response.go`
- Modify: `locallife/api/order.go`
- Modify: `locallife/logic/order_query_service.go`
- Modify: `locallife/db/query/order.sql`
- Modify payment/refund/profit-sharing tests where totals are asserted.
- Modify print worker payload if it renders item/fee lines: `locallife/worker/task_print_order.go`

**Required behavior:**

- Payment order amount equals `order.total_amount`, which includes packaging fee.
- Refund upper bound uses `order.total_amount`.
- Profit sharing total amount remains equal to `order.total_amount`.
- Merchant fee breakdown includes `packaging_fee_amount`.
- Customer-facing order response includes `packaging_fee` and `packaging_items`.
- Merchant order response includes same fee and snapshots.
- Print payload includes packaging in fee section or a clearly marked packaging line, not as a kitchen food item.

**Steps:**

- [ ] Add response DTOs:

```go
type orderPackagingItemResponse struct {
    ID int64 `json:"id"`
    PackagingOptionID *int64 `json:"packaging_option_id,omitempty"`
    Name string `json:"name"`
    UnitPrice int64 `json:"unit_price"`
    Quantity int16 `json:"quantity"`
    Subtotal int64 `json:"subtotal"`
}
```

- [ ] Write API/logic tests:
  - order detail returns packaging fee and item snapshot.
  - merchant fee breakdown customer payable remains consistent.
  - cancelled order response still includes historical packaging snapshot.
  - print payload separates packaging from food items.
- [ ] Update payment/refund/profit-sharing focused tests that assert totals.
- [ ] Implement response and fee breakdown changes.
- [ ] Review gate: inspect every amount equality check and ensure no path recomputes packaging from current option state.
- [ ] Commit:

```bash
git add locallife/logic locallife/api locallife/db/query locallife/db/sqlc locallife/db/mock locallife/worker
git commit -m "feat: expose packaging fee across order money paths"
```

**Validation:**

```bash
cd locallife
make sqlc
make check-generated
go test ./logic -run 'Test.*FeeBreakdown|Test.*Payment.*Packaging|Test.*Refund.*Packaging|Test.*ProfitSharing.*Packaging' -count=1
go test ./api -run 'Test.*Order.*Packaging|Test.*Merchant.*FeeBreakdown' -count=1
go test ./worker -run 'Test.*Print.*Packaging' -count=1
make test-safety
```

### Task 8: Migrate Legacy Packaging Dishes To New Options

**Purpose:** Convert existing merchant packaging dishes into packaging options safely and repeatably.

**Files:**

- Create: `locallife/db/migration/<next>_migrate_packaging_dishes_to_options.up.sql`
- Create: `locallife/db/migration/<next>_migrate_packaging_dishes_to_options.down.sql`
- Create or modify migration fixture test if project has a matching pattern: `locallife/db/sqlc/packaging_migration_test.go`

**Migration semantics:**

- For each merchant with active `dishes.is_packaging=true`, create one `merchant_packaging_settings` row with `enabled=true`, `required=true`, applicable `takeout/takeaway`.
- For each packaging dish, create one `merchant_packaging_options` row with:
  - `merchant_id = dishes.merchant_id`
  - `legacy_dish_id = dishes.id`
  - `name = dishes.name`
  - `description = dishes.description`
  - `price = dishes.price`
  - `is_enabled = dishes.is_online AND dishes.is_available AND dishes.deleted_at IS NULL`
  - `sort_order = dishes.sort_order`
- Use `ON CONFLICT (legacy_dish_id) WHERE legacy_dish_id IS NOT NULL DO UPDATE` semantics or equivalent safe pattern.
- Do not mutate historical `order_items`.
- Do not delete legacy dishes in this task.

**Steps:**

- [ ] Write migration.
- [ ] Add fixture proof:
  - no legacy rows produces no settings/options.
  - one merchant with one packaging dish produces settings + option.
  - re-run does not duplicate option.
  - disabled/deleted legacy dish does not force enabled option.
- [ ] Review gate: confirm migration is idempotent and historical orders are untouched.
- [ ] Commit:

```bash
git add locallife/db/migration locallife/db/sqlc/*packaging*_test.go
git commit -m "feat: migrate legacy packaging dishes"
```

**Validation:**

```bash
cd locallife
go test ./db/sqlc -run 'Test.*Packaging.*Migration' -count=1
```

### Task 9: Add Legacy Packaging Dish Freeze Gate

**Purpose:** Add a disabled-by-default server-controlled gate that can freeze old `is_packaging` dish usage after the new merchant and customer flows are deployed.

**Files:**

- Modify: `locallife/api/dish.go`
- Modify: `locallife/api/dish_test.go`
- Modify: `locallife/db/query/dish.sql`
- Modify: `locallife/logic/order_items.go`
- Modify: `locallife/logic/order_items_test.go`
- Modify config/env wiring using the existing backend configuration pattern:
  - `locallife/util/config.go`
  - `locallife/app.env.example`
- Modify generated sqlc and mocks.

**Required behavior:**

- Freeze gate is disabled by default in code and production config for this task.
- When the gate is disabled, legacy behavior remains compatible for old frontend builds.
- When the gate is enabled, new create/update dish requests with `is_packaging=true` are rejected with product copy explaining to use packaging settings.
- When the gate is enabled, existing legacy packaging dishes are hidden from public menu/search/recommendation/scan-table readers.
- When the gate is enabled, direct order creation with a legacy packaging dish as food item is rejected.
- Merchant internal dish list may still show legacy packaging dishes with migration copy until the weapp page removes the old switch.

**Steps:**

- [ ] Add tests:
  - freeze gate disabled preserves legacy create/update/read behavior needed by old frontend builds.
  - create dish with `is_packaging=true` rejected.
  - update dish to `is_packaging=true` rejected.
  - public search/menu excludes packaging dishes.
  - direct order item legacy packaging dish rejected.
- [ ] Add config/env wiring with a clearly named flag, for example `PACKAGING_LEGACY_DISH_FREEZE_ENABLED=false`.
- [ ] Update public reader queries through a flag-aware path:
  - use a query parameter such as `exclude_packaging` or separate query methods,
  - apply `AND is_packaging = false` only when the freeze gate is enabled,
  - keep the disabled-gate path compatible with old frontend builds.
- [ ] Update `CalculateOrderItems` to reject legacy packaging dish as normal order item only when the freeze gate is enabled.
- [ ] Review gate: verify the gate defaults off, merchant internal list still works, and public customer paths do not leak packaging dishes when enabled.
- [ ] Commit:

```bash
git add locallife/api/dish.go locallife/api/dish_test.go locallife/db/query/dish.sql locallife/db/sqlc locallife/db/mock locallife/logic/order_items.go locallife/logic/order_items_test.go locallife/util/config.go locallife/app.env.example
git commit -m "feat: add legacy packaging dish freeze gate"
```

**Validation:**

```bash
cd locallife
make sqlc
make check-generated
go test ./api -run 'Test.*Dish.*Packaging|Test.*Public.*Dish' -count=1
go test ./logic -run 'TestCalculateOrderItems.*Packaging' -count=1
```

### Task 10: Add Weapp Merchant Packaging Settings Page

**Purpose:** Move merchant packaging management out of dish edit and into a dedicated low-frequency settings page.

**Files:**

- Create: `weapp/miniprogram/pages/merchant/packaging/index.ts`
- Create: `weapp/miniprogram/pages/merchant/packaging/index.wxml`
- Create: `weapp/miniprogram/pages/merchant/packaging/index.wxss`
- Create: `weapp/miniprogram/pages/merchant/packaging/index.json`
- Create: `weapp/miniprogram/pages/merchant/packaging/component-policy.json`
- Create or modify API wrapper: `weapp/miniprogram/pages/merchant/_main_shared/api/packaging.ts`
- Modify route registration: `weapp/miniprogram/app.json`
- Modify merchant dashboard/settings entry as appropriate.
- Modify dish edit page to remove packaging switch:
  - `weapp/miniprogram/pages/merchant/dishes/edit/index.wxml`
  - `weapp/miniprogram/pages/merchant/dishes/edit/index.ts`
  - `weapp/miniprogram/pages/merchant/_utils/merchant-dish-edit-view.ts`

**Human-Centered UI Check:**

- Role and primary task: merchant owner/manager configures packaging once during onboarding or menu setup.
- High-frequency path: daily dish status and order handling must not be crowded by packaging setup.
- First-screen priority: enabled state, required state, applicable order types, current options, each option price/enabled state.
- Preserve state: unsaved edits must survive validation failure and pull-down refresh should warn or preserve draft.
- Failure/recovery: save failure renders inline retry state; duplicate option name shows field-level error; network retry keeps draft.
- Non-goals: do not expose per-dish packaging rules or packaging inventory.

**Steps:**

- [ ] Add TypeScript API wrapper matching backend contract.
- [ ] Add page route.
- [ ] Build page with TDesign switches, cells/list, inputs, icon-led add/remove controls, and save button.
- [ ] Remove packaging switch from dish edit form and submit payload.
- [ ] Add contract script:

```text
weapp/scripts/check-merchant-packaging-settings-contract.test.js
```

The script must verify:

- page imports packaging API wrapper.
- dish edit no longer submits `is_packaging`.
- packaging page blocks duplicate option names locally.
- packaging page preserves draft after simulated save failure helper if the helper is extracted.

- [ ] Review gate: confirm page is task-first, does not mirror backend DTO sections blindly, and has loading/empty/error/saving states.
- [ ] Commit:

```bash
git add weapp/miniprogram/pages/merchant/packaging weapp/miniprogram/pages/merchant/_main_shared/api/packaging.ts weapp/miniprogram/app.json weapp/miniprogram/pages/merchant/dishes/edit weapp/miniprogram/pages/merchant/_utils/merchant-dish-edit-view.ts weapp/scripts/check-merchant-packaging-settings-contract.test.js weapp/package.json
git commit -m "feat: add merchant packaging settings page"
```

**Validation:**

```bash
cd weapp
node scripts/check-merchant-packaging-settings-contract.test.js
npm run compile
```

### Task 11: Update Weapp Customer Cart And Order Confirm Packaging Flow

**Purpose:** Replace packaging item selection with explicit packaging option selection.

**Files:**

- Modify: `weapp/miniprogram/api/cart.ts`
- Modify: `weapp/miniprogram/pages/takeout/cart/_utils/takeout-cart-view.ts`
- Modify: `weapp/miniprogram/pages/takeout/cart/index.ts`
- Modify: `weapp/miniprogram/pages/takeout/cart/index.wxml`
- Modify: `weapp/miniprogram/pages/takeout/order-confirm/_utils/takeout-order-confirm-support.ts`
- Modify: `weapp/miniprogram/pages/takeout/order-confirm/index.ts`
- Modify: `weapp/miniprogram/pages/takeout/order-confirm/index.wxml`
- Modify shared order fee breakdown view if needed.
- Add or update contract script: `weapp/scripts/check-takeout-cart-packaging-checkout.test.js`

**Required behavior:**

- Cart response maps `packaging.options` and selected option.
- Packaging area appears for applicable merchant cart groups only.
- Required + one option auto-selects through backend selection API or marks selected only after backend confirms.
- Required + multiple options blocks checkout until selected.
- Optional packaging allows no selection.
- Order confirm displays selected packaging, `packaging_fee`, backend `total_amount`, and `selection_version` from preview.
- Create order request carries packaging identity from backend preview: `packaging_option_id` or explicit none, plus `packaging_selection_version`; it never sends packaging price/name.
- If preview reports stale packaging state or order create returns packaging version conflict, checkout reloads cart/preview before allowing another submit.
- Weak network failure to save selection must not fake success.

**Steps:**

- [ ] Update TypeScript interfaces.
- [ ] Update cart view model to include packaging domain state.
- [ ] Replace `getPackagingCheckoutBlocker` implementation so it checks packaging selection, not packaging item quantity.
- [ ] Add UI for packaging option selection.
- [ ] Update order-confirm snapshot and fee rows.
- [ ] Update contract tests:
  - required with no selection blocks checkout.
  - required with selected option passes.
  - optional with no selection passes.
  - one-option auto-selection does not proceed if backend save fails.
  - packaging fee is displayed separately from subtotal.
  - order create payload includes packaging option identity and `packaging_selection_version`.
  - stale packaging version conflict reloads checkout state before retry.
- [ ] Review gate: confirm no local fake truth, no duplicate checkout, and re-entry preserves selected backend state.
- [ ] Commit:

```bash
git add weapp/miniprogram/api/cart.ts weapp/miniprogram/pages/takeout/cart weapp/miniprogram/pages/takeout/order-confirm weapp/scripts/check-takeout-cart-packaging-checkout.test.js
git commit -m "feat: add customer packaging checkout flow"
```

**Validation:**

```bash
cd weapp
node scripts/check-takeout-cart-packaging-checkout.test.js
npm run compile
```

### Task 12: Update Order Detail, Merchant Detail, And Print Views In Weapp

**Purpose:** Make historical packaging snapshots visible after order creation.

**Files:**

- Modify customer order models/adapters:
  - `weapp/miniprogram/pages/orders/_models/order.ts`
  - `weapp/miniprogram/pages/orders/_adapters/order.ts`
  - `weapp/miniprogram/pages/orders/detail/index.wxml`
  - `weapp/miniprogram/pages/orders/detail/index.ts`
- Modify merchant order detail/list:
  - `weapp/miniprogram/pages/merchant/_main_shared/api/order.ts`
  - `weapp/miniprogram/pages/merchant/_utils/merchant-order-detail-view.ts`
  - `weapp/miniprogram/pages/merchant/orders/detail/index.wxml`
  - `weapp/miniprogram/pages/merchant/orders/list/index.ts`
- Modify fee breakdown shared utilities if needed.
- Add contract script: `weapp/scripts/check-order-packaging-snapshot-view.test.js`

**Required behavior:**

- Customer detail shows packaging name and fee if order has packaging items.
- Merchant detail shows packaging fee in fee breakdown.
- Order list totals remain backend truth.
- Empty packaging snapshot renders nothing, not an empty card.
- Canceled/refunded orders still show original packaging snapshot.

**Steps:**

- [ ] Extend order API types.
- [ ] Update adapters to map packaging snapshots.
- [ ] Update views and fee rows.
- [ ] Add contract tests for packaging snapshot display and empty-state omission.
- [ ] Review gate: confirm no current merchant option lookup is used for historical orders.
- [ ] Commit:

```bash
git add weapp/miniprogram/pages/orders weapp/miniprogram/pages/merchant weapp/scripts/check-order-packaging-snapshot-view.test.js
git commit -m "feat: show order packaging snapshots"
```

**Validation:**

```bash
cd weapp
node scripts/check-order-packaging-snapshot-view.test.js
npm run compile
```

### Task 13: End-To-End Safety Validation And Overall Review

**Purpose:** Confirm the refactor meets design goals without breaking normal ordering, payment, refund, merchant management, or customer checkout.

**Files:**

- Create: `artifacts/packaging-domain-production-refactor-review-2026-06-19.md`
- Read diffs across all task commits.

**Review checklist:**

- Design target:
  - Packaging no longer requires dish creation.
  - Customer menus/search do not expose packaging options as food.
  - Cart/checkout require packaging only when merchant settings require it.
  - Cart preview and order creation return the same backend-computed packaging fee and total amount for the same state.
  - Orders persist packaging snapshots.
  - `orders.packaging_fee` is included in total amount.
  - Merchant/customer displays explain packaging separately.
- Sequencing:
  - Additive migrations before cutover.
  - Legacy migration idempotent.
  - Legacy freeze gate is deployed disabled by default before frontend cutover.
  - Legacy write freeze is enabled only after weapp merchant and customer flows are deployed and smoked.
  - No cleanup deletes historical order evidence.
- Idempotency:
  - Settings `PUT` convergent.
  - Cart selection `PUT` convergent and preserves `selection_version` on repeated same-body requests.
  - Order create idempotency includes packaging identity and replays existing snapshot.
  - Stale `packaging_selection_version` conflicts before new order creation.
  - Migration rerun does not duplicate rows.
- Authorization:
  - Merchant config reads/writes require owned merchant.
  - Customer cart selection requires owned cart.
  - Order create validates option belongs to order merchant.
  - No client-provided price/name/merchant_id is trusted.
- Money:
  - Total amount formula includes packaging fee exactly once.
  - Preview amount and create-order amount match for the same cart, selected packaging, delivery fee, and discounts.
  - Refund upper bound includes packaging fee.
  - Profit sharing total matches order total.
  - Rider delivery amount excludes packaging fee.
  - Merchant receivable includes packaging fee according to settlement formula.
- Robustness:
  - No external side effects inside transactions.
  - Freeze flag defaults off and can be enabled/disabled without redeploying code if existing config patterns support runtime env changes.
  - Weak network and duplicate-tap states are handled in weapp.
  - Failure states preserve drafts and do not show fake success.
  - Generated artifacts match SQL and Swagger sources.
- Project standards:
  - Handler/logic/db split preserved.
  - No business logic in handlers.
  - No unstructured logging.
  - No new runtime globals.
  - File-size guardrail considered.
  - Weapp backend contract remains source of truth.

**Validation commands:**

Backend:

```bash
cd locallife
make sqlc
make swagger
make check-generated
go test ./db/sqlc -run 'Test.*Packaging|TestCreateOrderTx.*Packaging|Test.*Migration' -count=1
go test ./logic -run 'Test.*Packaging|TestOrderServiceCreateOrder.*Packaging|TestCalculateCartPreview.*Packaging|TestCalculateOrderPreview.*Packaging|TestBuildMerchantOrderFeeBreakdown|TestComputeOrderTotals' -count=1
go test ./api -run 'Test.*Packaging|Test.*Cart.*Packaging|TestCalculateCart.*Packaging|Test.*Order.*Packaging|Test.*Merchant.*FeeBreakdown' -count=1
go test ./worker -run 'Test.*Print.*Packaging' -count=1
make test-safety
make lint-filesize
```

Weapp:

```bash
cd weapp
node scripts/check-merchant-packaging-settings-contract.test.js
node scripts/check-takeout-cart-packaging-checkout.test.js
node scripts/check-order-packaging-snapshot-view.test.js
npm run compile
npm run quality:check
```

**Steps:**

- [ ] Run all validation commands above.
- [ ] Write `artifacts/packaging-domain-production-refactor-review-2026-06-19.md` with:
  - changed files summary,
  - validation output summary,
  - review findings,
  - fixes applied,
  - residual risks,
  - release notes,
  - rollback plan.
- [ ] Fix any review findings in the owning task area and rerun affected validation.
- [ ] Re-run final safety commands after fixes.
- [ ] Commit review artifact:

```bash
git add artifacts/packaging-domain-production-refactor-review-2026-06-19.md
git commit -m "docs: review packaging domain refactor"
```

## 10. Rollout And Rollback

### Rollout

1. Deploy backend with additive schema and APIs.
2. Run legacy migration.
3. Verify migrated settings/options count matches baseline audit.
4. Deploy legacy freeze gate with `PACKAGING_LEGACY_DISH_FREEZE_ENABLED=false`.
5. Deploy weapp merchant packaging settings.
6. Deploy weapp customer checkout flow.
7. Run production smoke with freeze flag still disabled:
   - merchant reads packaging settings,
   - merchant edits packaging option,
   - customer adds food to cart,
   - customer selects packaging,
   - order preview/create includes packaging fee,
   - payment amount equals order total,
   - merchant order detail shows packaging snapshot.
8. Enable `PACKAGING_LEGACY_DISH_FREEZE_ENABLED=true`.
9. Run freeze smoke:
   - merchant dish edit no longer submits packaging switch,
   - public menu/search excludes legacy packaging dishes,
   - direct order with legacy packaging dish as food item is rejected,
   - new packaging settings and checkout continue to work.

### Rollback

Backend rollback must preserve historical order evidence:

- Do not drop `order_packaging_items` during emergency rollback.
- If frontend rollback is needed, keep backend accepting legacy packaging dish order items until a later controlled cleanup.
- If frontend rollback is needed after freeze is enabled, first set `PACKAGING_LEGACY_DISH_FREEZE_ENABLED=false`, then verify legacy packaging dish order flow still works.
- If amount bug is detected before payment, disable packaging settings by setting `merchant_packaging_settings.enabled=false` for affected merchants.
- If amount bug is detected after payment, stop rollout, preserve order rows, and run finance/reconciliation review before any data correction.

## 11. Release Readiness Criteria

The refactor is release-ready only when all criteria are true:

- Every task commit exists and each task has passed its review gate.
- Final review artifact exists and records validation evidence.
- Backend SQL, sqlc, Swagger and mocks are generated and checked.
- Backend focused tests and `make test-safety` pass.
- Weapp compile and quality checks pass.
- Cart preview response includes backend-computed `packaging_fee`, `total_amount`, selected option identity, and `selection_version`.
- Order create idempotency rejects stale packaging selection identity and replays existing snapshots without reading current packaging settings.
- Legacy packaging dishes are hidden from customer surfaces.
- Legacy packaging dish freeze gate has been deployed disabled by default, then enabled only after weapp cutover smoke passes.
- New merchant packaging settings page is registered and reachable.
- Customer checkout handles required, optional, selected, missing, disabled, and weak-network packaging states.
- Order details and merchant fee breakdown show packaging fee and snapshot.
- No known authz bypass, duplicate-charge path, idempotency replay drift, or migration duplicate path remains open.

## 12. Open Decisions Locked For This Plan

These decisions are fixed for this implementation plan:

- 首版包装计价范围是 `per_order`。
- 首版每单最多一种包装方式。
- 包装费不参与菜品小计、满减和券门槛。
- 包装费参与用户实付和商户应收。
- 顾客下单首版以购物车包装选择为包装来源，创建订单必须携带后端预览返回的选项身份和 `selection_version`。
- 兼容窗口内保留旧字段 `dishes.is_packaging`。
- 旧包装菜品冻结必须通过默认关闭的服务端开关启用，不能在小程序切换前硬切。
- 不重写历史 legacy order items。

Changing any of these decisions requires updating this plan and re-reviewing affected task cards before implementation continues.
