# Baofoo Fee, Commission, and Settlement Migration Design

Date: 2026-05-05
Risk class: G3 - payment, profit sharing, callbacks, settlement, finance, and payout ledger behavior.

## 1. Goal

Align LocalLife's Baofoo main-business money model with the latest business rules before changing production code:

- Baofoo charges LocalLife/payment side `0.3%`; base is the customer's paid order total.
  - Confirmed business rule: this fee is deducted in real time at payment; funds reaching the BaoCaiTong reserve/shareable pool are already net of Baofoo's fee.
- LocalLife charges merchants `0.6%` payment service fee.
  - Takeout base: customer paid total minus delivery fee.
  - Dine-in, takeaway/self-pickup, and reservation base: customer paid total.
- LocalLife charges riders `0.6%` payment service fee on the rider delivery-fee receivable.
- LocalLife charges `5%` commission only on commissionable business.
  - Takeout base: customer paid total minus delivery fee.
  - Reservation base: customer paid total.
  - Dine-in and takeaway/self-pickup: no commission.
- The `0.6%` payment service fees are independent from the `5%` commission.

The design separates provider cost, merchant fee, rider fee, and business commission so finance, reconciliation, and Baofoo `share_after_pay` payloads do not keep overloading the existing `payment_fee` field.

## 2. Current Implementation Gaps

| Area | Current behavior | Required change |
| --- | --- | --- |
| Provider fee | `BaofuPaymentFeeRateBps = 30`; `payment_fee` currently stores 0.3% and is deducted from merchant share. | Keep 0.3% as `provider_payment_fee`; do not expose it as the merchant-facing 0.6% fee. |
| Merchant payment fee | No 0.6% merchant fee model. | Add `merchant_payment_fee`, rate/base/source fields, and merchant finance display. |
| Rider payment fee | Rider currently receives gross delivery fee. | Add `rider_payment_fee`; rider net share is delivery fee minus rider fee. |
| Commission base | Baofoo code computes platform/operator commission from total paid amount. | Compute commission from net-of-delivery for takeout; total for reservation; zero for dine-in/takeaway. |
| Settlement mode | Baofoo route currently sets `RequiresProfitSharing=true` for all main-business orders. | Replace the boolean decision with a settlement-mode decision: commission share, fee-only settlement, or direct/no share. |
| Finance APIs | Merchant finance aggregates `profit_sharing_orders.payment_fee`. | Merchant finance must show merchant 0.6% fee; rider finance must show rider gross fee/net income; platform finance must show provider cost and fee margin. |
| Baofoo actual fee | `feeAmt` is parsed in DTO/fact raw data but not persisted as a first-class reconciled amount. | Store actual `feeAmt` as provider fee observation/ledger and reconcile with estimated provider fee. |
| Account detail | Baofoo `accDetails / T-1001-013-11` is not implemented. | Separate enhancement for Baofoo secondary-account transaction detail; not required to fix fee formulas. |

## 3. Terminology and Field Semantics

Use these names consistently in code, SQL, API responses, tests, and docs.

| Name | Meaning | Payer | Payee / economic owner | Base |
| --- | --- | --- | --- | --- |
| `provider_payment_fee` | Baofoo upstream cost charged by Baofoo | LocalLife / platform settlement pool | Baofoo | Customer paid total |
| `merchant_payment_fee` | 0.6% payment service fee charged by LocalLife to merchant | Merchant | LocalLife platform | Merchant fee base |
| `rider_payment_fee` | 0.6% payment service fee charged by LocalLife to rider | Rider | LocalLife platform | Rider gross delivery receivable |
| `platform_commission` | Platform's 2% business commission | Merchant-side business proceeds | LocalLife platform | Commission base |
| `operator_commission` | Operator's 3% business commission | Merchant-side business proceeds | Operator | Commission base |
| `merchant_amount` | Actual net amount shared/settled to merchant | N/A | Merchant | Merchant gross base minus merchant fees/commissions |
| `rider_amount` | Actual net amount shared/settled to rider | N/A | Rider | Rider gross receivable minus rider fee |
| `platform_receiver_amount` | Actual amount sent to platform in Baofoo sharing details | N/A | LocalLife platform | Platform commission + merchant/rider fees minus provider cost; Baofoo has already deducted that provider cost before funds enter the shareable reserve pool |

`profit_sharing_orders.payment_fee` becomes legacy/deprecated. It must not be the source of truth for new finance logic. During migration it can remain populated for backward compatibility, but new code should read explicit fields.

## 4. Calculation Rules

### 4.1 Shared Helper

Use one deterministic helper for payment service fees and provider fee estimates. The Baofoo fee timing is not estimated: it is real-time deducted before reserve. The estimate is only the local amount used before a callback/query supplies actual `feeAmt`.

```go
func ceilFeeFen(baseFen int64, rateBps int32) int64 {
    if baseFen <= 0 || rateBps <= 0 {
        return 0
    }
    return (baseFen*int64(rateBps) + 9999) / 10000
}
```

Commission can preserve the existing floor behavior unless finance explicitly decides to change historical commission rounding:

```go
func commissionFen(baseFen int64, rateBps int32) int64 {
    if baseFen <= 0 || rateBps <= 0 {
        return 0
    }
    return baseFen * int64(rateBps) / 10000
}
```

Rationale: provider/payment service fees are cost-recovery charges and should not undercharge fractional fen; commission should not silently increase beyond existing behavior without a separate business decision.

### 4.2 Base Resolution

| Order scene | Provider fee base | Merchant payment fee base | Rider payment fee base | Commission base | Business commission? |
| --- | --- | --- | --- | --- | --- |
| Takeout | `total_amount` | `max(total_amount - rider_gross_amount, 0)` | `rider_gross_amount` | `max(total_amount - rider_gross_amount, 0)` | Yes, 2% + 3% |
| Reservation payment | `total_amount` | `total_amount` | `0` | `total_amount` | Yes, 2% + 3% |
| Dine-in | `total_amount` | `total_amount` | `0` | `0` | No |
| Takeaway/self-pickup | `total_amount` | `total_amount` | `0` | `0` | No, unless product explicitly changes this order type |

`rider_gross_amount` is normally `delivery_fee`, capped at `total_amount` as the current code already does for rider sharing safety.

Reservation-linked dine-in orders must be disambiguated by payment scene, not only by raw `orders.order_type`. If the payment is a reservation/prepaid/deposit payment, it uses reservation rules. If it is an ordinary dine-in table payment, it uses dine-in rules.

### 4.3 Formula: Commission-Share Orders

For takeout and reservation:

```text
provider_payment_fee = actual Baofoo feeAmt if known, else ceilFeeFen(total_amount, 30)
merchant_payment_fee = ceilFeeFen(merchant_payment_fee_base, 60)
rider_payment_fee = ceilFeeFen(rider_payment_fee_base, 60)
platform_commission = commissionFen(commission_base, 200)
operator_commission = commissionFen(commission_base, 300)
rider_amount = rider_gross_amount - rider_payment_fee
merchant_amount = merchant_payment_fee_base - merchant_payment_fee - platform_commission - operator_commission
platform_gross_revenue = platform_commission + merchant_payment_fee + rider_payment_fee
platform_receiver_amount = platform_gross_revenue - provider_payment_fee
shareable_amount = total_amount - provider_payment_fee
```

Validation:

```text
merchant_amount >= 0
rider_amount >= 0
platform_receiver_amount >= 0
merchant_amount + rider_amount + operator_commission + platform_receiver_amount == shareable_amount
```

Confirmed rule: Baofoo's `feeAmt` has already been deducted before the reserve/shareable pool is available. Therefore LocalLife's share instruction must balance against `shareable_amount = total_amount - provider_payment_fee`, and the platform receiver is the net platform cash receiver after absorbing that upstream provider cost from the pool.

### 4.4 Formula: Dine-In / Takeaway Fee-Only Orders

Business rule says no 5% commission and no business profit sharing. The new rider/merchant payment-fee rule still requires collecting 0.6% from the merchant.

There are two settlement modes conceptually, but only `fee_only_share` is implemented for the current BaoCaiTong aggregate contract path:

1. `direct_no_share`: if Baofoo's official non-sharing/direct-settlement order product settles funds directly to the merchant and still lets LocalLife bill/collect the 0.6% separately. In this mode LocalLife records a merchant fee receivable and collects it through a future settlement/offset/withdrawal process.
2. `fee_only_share`: if the current BaoCaiTong `SHARING` product is the only available main-business payment path. In this mode LocalLife sends a technical `share_after_pay` with no platform/operator commission, only merchant net amount and platform payment-fee/cost-recovery amount. Merchant-facing UI labels this as payment service fee settlement, not business commission.

Current implementation: `fee_only_share`. This avoids creating unpaid merchant fee receivables and keeps each order self-settling while the documented BaoCaiTong aggregate product remains the only active main-business payment path.

For `fee_only_share` dine-in/takeaway:

```text
provider_payment_fee = actual Baofoo feeAmt if known, else ceilFeeFen(total_amount, 30)
merchant_payment_fee = ceilFeeFen(total_amount, 60)
rider_payment_fee = 0
platform_commission = 0
operator_commission = 0
merchant_amount = total_amount - merchant_payment_fee
platform_receiver_amount = merchant_payment_fee - provider_payment_fee
shareable_amount = total_amount - provider_payment_fee
```

Validation:

```text
merchant_amount >= 0
platform_receiver_amount >= 0
merchant_amount + platform_receiver_amount == shareable_amount
```

This is technically a Baofoo settlement instruction, but not a business commission split. If product language must reserve "分账" only for commission-bearing orders, name this mode "payment-fee settlement" in APIs/docs.

## 5. Worked Examples

### 5.1 Takeout: total 100.00, delivery 5.00

```text
total_amount = 10000
rider_gross_amount = 500
merchant_payment_fee_base = 9500
commission_base = 9500
provider_payment_fee = ceil(10000 * 0.3%) = 30
merchant_payment_fee = ceil(9500 * 0.6%) = 57
rider_payment_fee = ceil(500 * 0.6%) = 3
platform_commission = floor(9500 * 2%) = 190
operator_commission = floor(9500 * 3%) = 285
merchant_amount = 9500 - 57 - 190 - 285 = 8968
rider_amount = 500 - 3 = 497
platform_receiver_amount = 190 + 57 + 3 - 30 = 220
shareable_amount = 10000 - 30 = 9970
check = 8968 + 497 + 285 + 220 = 9970
```

Economic view:

- Merchant gross goods/service base: 95.00; merchant net settlement: 89.68.
- Rider gross delivery receivable: 5.00; rider net settlement: 4.97.
- Operator commission: 2.85.
- Platform gross revenue: 1.90 + 0.57 + 0.03 = 2.50.
- Baofoo provider cost: 0.30.
- Platform net cash receiver amount after Baofoo's real-time provider-fee deduction: 2.20.

### 5.2 Reservation: total 100.00

```text
provider_payment_fee = 30
merchant_payment_fee = 60
rider_payment_fee = 0
platform_commission = 200
operator_commission = 300
merchant_amount = 10000 - 60 - 200 - 300 = 9440
platform_receiver_amount = 200 + 60 - 30 = 230
shareable_amount = 9970
check = 9440 + 300 + 230 = 9970
```

### 5.3 Dine-In Fee-Only Settlement: total 100.00

```text
provider_payment_fee = 30
merchant_payment_fee = 60
platform_commission = 0
operator_commission = 0
merchant_amount = 10000 - 60 = 9940
platform_receiver_amount = 60 - 30 = 30
shareable_amount = 9970
check = 9940 + 30 = 9970
```

Merchant is charged only 0.60 payment service fee. There is no 5% commission.

## 6. Data Model Migration

### 6.1 Additive Columns on `profit_sharing_orders`

Add explicit calculation fields. Keep existing columns and indexes during rollout.

```sql
ALTER TABLE profit_sharing_orders
    ADD COLUMN IF NOT EXISTS calculation_version TEXT NOT NULL DEFAULT 'legacy_v1',
    ADD COLUMN IF NOT EXISTS settlement_mode TEXT NOT NULL DEFAULT 'commission_share',
    ADD COLUMN IF NOT EXISTS provider_payment_fee BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS provider_payment_fee_rate_bps INTEGER NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS provider_payment_fee_base_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS provider_payment_fee_source TEXT NOT NULL DEFAULT 'estimated',
    ADD COLUMN IF NOT EXISTS merchant_payment_fee BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS merchant_payment_fee_rate_bps INTEGER NOT NULL DEFAULT 60,
    ADD COLUMN IF NOT EXISTS merchant_payment_fee_base_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rider_gross_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rider_payment_fee BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rider_payment_fee_rate_bps INTEGER NOT NULL DEFAULT 60,
    ADD COLUMN IF NOT EXISTS rider_payment_fee_base_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS commission_base_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS platform_receiver_amount BIGINT NOT NULL DEFAULT 0;

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_settlement_mode_check
    CHECK (settlement_mode IN ('commission_share', 'fee_only_share', 'direct_no_share'));

ALTER TABLE profit_sharing_orders
    ADD CONSTRAINT profit_sharing_orders_new_fee_amounts_check
    CHECK (
        provider_payment_fee >= 0 AND provider_payment_fee_base_amount >= 0 AND
        merchant_payment_fee >= 0 AND merchant_payment_fee_base_amount >= 0 AND
        rider_gross_amount >= 0 AND rider_payment_fee >= 0 AND rider_payment_fee_base_amount >= 0 AND
        commission_base_amount >= 0 AND platform_receiver_amount >= 0
    );
```

Backfill for existing non-production/test rows:

```sql
UPDATE profit_sharing_orders
SET
    calculation_version = 'baofu_legacy_provider_fee_v1',
    provider_payment_fee = payment_fee,
    provider_payment_fee_rate_bps = payment_fee_rate_bps,
    provider_payment_fee_base_amount = total_amount,
    merchant_payment_fee = 0,
    rider_payment_fee = 0,
    rider_gross_amount = rider_amount,
    commission_base_amount = total_amount,
    platform_receiver_amount = platform_commission
WHERE provider = 'baofu'
  AND calculation_version = 'legacy_v1';
```

Do not reinterpret historical `payment_fee` as 0.6%. Historical rows remain legacy rows.

### 6.2 Add Fee Ledger for Payer-Level Finance

Create a payer-level ledger. This should become the finance source for payment service fees and provider cost reconciliation. It avoids forcing dine-in fee-only or direct-no-share records into the business profit-sharing table.

```sql
CREATE TABLE IF NOT EXISTS order_payment_fee_ledgers (
    id BIGSERIAL PRIMARY KEY,
    provider TEXT NOT NULL,
    channel TEXT NOT NULL,
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id),
    profit_sharing_order_id BIGINT REFERENCES profit_sharing_orders(id),
    fee_type TEXT NOT NULL,
    payer_type TEXT NOT NULL,
    payer_id BIGINT,
    payee_type TEXT NOT NULL,
    base_amount BIGINT NOT NULL,
    rate_bps INTEGER NOT NULL,
    amount BIGINT NOT NULL,
    amount_source TEXT NOT NULL DEFAULT 'calculated',
    external_payment_fact_id BIGINT REFERENCES external_payment_facts(id),
    status TEXT NOT NULL DEFAULT 'recorded',
    calculation_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT order_payment_fee_ledgers_fee_type_check CHECK (fee_type IN (
        'provider_payment_fee',
        'merchant_payment_service_fee',
        'rider_payment_service_fee'
    )),
    CONSTRAINT order_payment_fee_ledgers_payer_type_check CHECK (payer_type IN ('platform', 'merchant', 'rider')),
    CONSTRAINT order_payment_fee_ledgers_payee_type_check CHECK (payee_type IN ('baofu', 'platform')),
    CONSTRAINT order_payment_fee_ledgers_status_check CHECK (status IN ('recorded', 'reconciled', 'adjusted')),
    CONSTRAINT order_payment_fee_ledgers_amount_check CHECK (base_amount >= 0 AND rate_bps >= 0 AND amount >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS order_payment_fee_ledgers_once_per_payer_uidx
    ON order_payment_fee_ledgers(payment_order_id, fee_type, payer_type, COALESCE(payer_id, 0));

CREATE INDEX IF NOT EXISTS order_payment_fee_ledgers_payer_idx
    ON order_payment_fee_ledgers(payer_type, payer_id, created_at DESC, id DESC);
```

Ledger rows per takeout order:

| fee_type | payer_type | payer_id | payee_type | amount |
| --- | --- | --- | --- | --- |
| `provider_payment_fee` | `platform` | null | `baofu` | Baofoo 0.3% actual/estimated |
| `merchant_payment_service_fee` | `merchant` | merchant id | `platform` | Merchant 0.6% |
| `rider_payment_service_fee` | `rider` | rider id | `platform` | Rider 0.6% |

### 6.3 Persist Actual Baofoo `feeAmt`

When applying Baofoo payment callback/query facts:

- If `PaymentFact.FeeAmountFen > 0`, upsert `provider_payment_fee` ledger with `amount_source='actual_callback'` or `actual_query`.
- If no actual fee is available, keep calculated estimate and mark `amount_source='calculated'`.
- If actual differs from the estimate used to create `share_after_pay`, record the mismatch and surface it in platform reconciliation.

This is safer than only parsing `feeAmt` into raw JSON.

## 7. Service and Worker Design

### 7.1 New Calculator Boundary

Create a focused calculator, for example:

- `locallife/logic/baofu_fee_calculator.go`
- `locallife/logic/baofu_fee_calculator_test.go`

Public input/output shape:

```go
type BaofuSettlementCalculationInput struct {
    OrderScene                 string
    TotalAmountFen             int64
    DeliveryFeeFen             int64
    ProviderPaymentFeeFen      int64 // optional actual feeAmt; 0 means estimate
    HasRiderReceiver           bool
    HasOperatorReceiver        bool
    PlatformCommissionRateBps  int32
    OperatorCommissionRateBps  int32
    MerchantPaymentFeeRateBps  int32
    RiderPaymentFeeRateBps     int32
}

type BaofuSettlementCalculationResult struct {
    CalculationVersion          string
    SettlementMode              string
    TotalAmountFen              int64
    ShareableAmountFen          int64
    ProviderPaymentFeeFen       int64
    ProviderPaymentFeeSource    string
    MerchantPaymentFeeBaseFen   int64
    MerchantPaymentFeeFen       int64
    RiderGrossAmountFen         int64
    RiderPaymentFeeBaseFen      int64
    RiderPaymentFeeFen          int64
    CommissionBaseFen           int64
    PlatformCommissionFen       int64
    OperatorCommissionFen       int64
    MerchantAmountFen           int64
    RiderAmountFen              int64
    PlatformReceiverAmountFen   int64
}
```

The calculator is the only place allowed to decide bases, rates, rounding, and settlement-mode totals.

### 7.2 Payment Order Settlement Mode

Do not keep using `RequiresProfitSharing` as the only business decision. Add a Baofoo settlement mode decision near payment order creation:

| Payment scene | `requires_profit_sharing` | `baofu_settlement_mode` |
| --- | --- | --- |
| Takeout | true | `commission_share` |
| Reservation payment | true | `commission_share` |
| Dine-in | true | `fee_only_share` |
| Takeaway/self-pickup | true | `fee_only_share` |

Confirmed first implementation: use `fee_only_share` for dine-in/takeaway while Baofoo main-business payment remains `prodType=SHARING/orderType=7`. This preserves real-time fee collection, avoids creating offline receivables, and records no 5% commission for these scenes.

### 7.3 Profit Sharing Order Creation

Update `BaofuProfitSharingService.CreatePendingOrder` to:

- Require `SettlementMode` and `OrderScene`.
- Use the new calculator result.
- Store merchant/rider fees and provider cost separately.
- Build `sharing_detail_snapshot` with both receiver amounts and fee components.
- Create `order_payment_fee_ledgers` rows in the same transaction as the `profit_sharing_orders` insert.

Snapshot shape:

```json
{
  "provider": "baofu",
  "channel": "baofu_aggregate",
  "calculation_version": "baofu_fee_v2",
  "settlement_mode": "commission_share",
  "shareable_amount": 9970,
  "platform_receiver_amount": 220,
  "fees": {
    "provider_payment_fee": 30,
    "merchant_payment_fee": 57,
    "rider_payment_fee": 3,
    "provider_payment_fee_source": "estimated",
    "provider_payment_fee_timing": "realtime_deducted_before_reserve"
  },
  "bases": {
    "total_amount": 10000,
    "merchant_payment_fee_base": 9500,
    "rider_payment_fee_base": 500,
    "commission_base": 9500
  },
  "receivers": [
    {"role":"merchant","sharing_mer_id":"...","amount":8968},
    {"role":"rider","sharing_mer_id":"...","amount":497},
    {"role":"operator","sharing_mer_id":"...","amount":285},
    {"role":"platform","sharing_mer_id":"...","amount":220}
  ]
}
```

### 7.4 Share Request Builder

Update `worker/task_baofu_profit_sharing.go` to read the new snapshot receiver amounts. It should not recompute fees. It should send only positive receiver amounts.

For `fee_only_share`, receiver details normally contain only:

- merchant net amount;
- platform receiver amount for payment fee net of provider cost.

No operator receiver is sent. No platform commission is recorded.

### 7.5 Callback and Query Fact Application

Update Baofoo payment fact application to preserve actual `feeAmt`:

- payment callback parser already normalizes `PaymentFact.FeeAmountFen`;
- `BaofuPaymentService.RecordPaymentFact` should pass it into first-class storage;
- fact application should upsert provider fee ledger and mark source as actual;
- reconciliation should compare actual provider fee with the estimate used in any already-created settlement.

## 8. API and Finance Changes

### 8.1 Merchant Finance

Merchant-facing finance must show merchant charges only. It must not include rider fee or Baofoo provider cost in merchant deductions.

Add or remap response fields:

| Existing / new field | Meaning |
| --- | --- |
| `payment_fee` | Backward-compatible alias for merchant payment service fee after migration. |
| `merchant_payment_fee` | Explicit 0.6% fee charged to merchant. |
| `platform_commission` | Platform 2% commission only. |
| `operator_commission` | Operator 3% commission only. |
| `total_deduction_fee` | `merchant_payment_fee + platform_commission + operator_commission`. |
| `merchant_amount` | Merchant net settlement amount. |

Do not expose `provider_payment_fee` to merchant finance unless an admin/debug scope explicitly requests platform costs.

### 8.2 Rider Finance

Rider-facing income should distinguish gross delivery receivable, rider payment service fee, and net received amount.

Add fields to rider income/detail responses where applicable:

| Field | Meaning |
| --- | --- |
| `rider_gross_amount` | Delivery fee before rider payment service fee. |
| `rider_payment_fee` | 0.6% rider payment service fee. |
| `rider_amount` | Net rider settlement amount. |

Existing summaries that currently sum `rider_amount` can continue representing net income after migration.

### 8.3 Platform/Admin Finance

Platform reconciliation should show:

- paid amount;
- provider payment fee cost;
- merchant payment service fee income;
- rider payment service fee income;
- platform commission income;
- operator commission passthrough;
- platform net payment-fee margin: `merchant_payment_fee + rider_payment_fee - provider_payment_fee`;
- fee ledger mismatch count.

## 9. Migration Execution Plan

### Phase 1 - Add Tests and Calculator

1. Add failing tests for:
   - takeout 100.00 + delivery 5.00 example;
   - reservation 100.00 example;
   - dine-in 100.00 fee-only example;
   - tiny amount rounding for merchant/rider provider fees;
   - operator-missing redirect behavior if still required.
2. Implement calculator only.
3. Run focused tests: `go test ./logic -run 'TestCalculateBaofu.*|TestBaofuSettlement'`.

### Phase 2 - Add Additive Schema

1. Add migration for `profit_sharing_orders` columns and `order_payment_fee_ledgers`.
2. Add SQL queries for creating/upserting fee ledgers.
3. Run `make sqlc` from `locallife/`.
4. Run sqlc compile tests or package tests that touch generated types.

### Phase 3 - Persistence and Snapshot Migration

1. Update `CreateBaofuProfitSharingOrderTx` to insert the new fields and fee ledger rows atomically.
2. Update `BaofuProfitSharingService.CreatePendingOrder` to use calculator output.
3. Keep writing legacy `payment_fee` as `provider_payment_fee` during the transition; finance queries should stop reading it.
4. Update unit tests for tx params and snapshot JSON.

### Phase 4 - Settlement Mode Routing

1. Add `baofu_settlement_mode` or equivalent local field on payment order, or derive mode deterministically at scheduler time if schema scope must be smaller.
2. Update Baofoo payment creation:
   - takeout/reservation -> `commission_share`;
   - dine-in/takeaway -> `fee_only_share` unless direct-no-share is confirmed and implemented.
3. Update `ListBaofuOrdersReadyForProfitSharing` filters to include fee-only settlement rows but exclude true direct-no-share rows.
4. Add tests proving dine-in has no platform/operator commission but still records merchant 0.6% fee.

### Phase 5 - Callback Actual Fee Persistence

1. Add provider fee actual persistence from `PaymentFact.FeeAmountFen`.
2. Use actual provider fee when available before share creation; otherwise use estimate.
3. Add reconciliation mismatch query and alert field.
4. Add callback/query fact tests with `feeAmt` present, absent, and mismatched.

### Phase 6 - Finance API Updates

1. Update merchant finance SQL to sum `merchant_payment_fee`, not legacy `payment_fee`.
2. Update rider finance SQL/API to expose gross, fee, and net rider income.
3. Update platform Baofoo reconciliation SQL/API to show provider cost, merchant fee, rider fee, and fee margin.
4. Update API tests for response fields and backward-compatible aliases.

### Phase 7 - Deployment and Backfill

1. Deploy additive migration first.
2. Deploy code that writes both legacy and new fields.
3. Run a smoke order in sandbox/production-first-order flow:
   - unified order;
   - payment callback/query with `feeAmt`;
   - settlement/share creation;
   - share query/callback;
   - merchant/rider/platform finance views.
4. For any pre-migration test rows, leave `calculation_version='baofu_legacy_provider_fee_v1'` and exclude them from production finance if needed by date/environment.
5. After one stable reconciliation cycle, mark legacy `payment_fee` as deprecated in code comments and docs.

## 10. Validation Matrix

| Validation | Command / evidence | Required before deploy? |
| --- | --- | --- |
| Calculator unit tests | `go test ./logic -run 'TestBaofuSettlement'` | Yes |
| Profit-sharing service tests | `go test ./logic -run 'TestCalculateBaofu|TestBaofuProfitSharing'` | Yes |
| SQL generation | `make sqlc` | Yes after query/migration changes |
| Worker share request tests | `go test ./worker -run 'TestProcessTaskBaofuProfitSharing|TestBuildBaofu'` | Yes |
| Callback actual fee tests | `go test ./baofu/aggregatepay/notification ./logic ./api -run 'Baofu|Fee'` | Yes |
| Merchant/rider finance API tests | `go test ./api -run 'MerchantFinance|RiderIncome'` | Yes |
| Safety tests | `make test-safety` | Yes before production rollout |
| Sandbox smoke | existing `cmd/baofu_*_smoke` tools | Useful but not sufficient for real callback/share |
| Production first-order evidence | masked evidence in `artifacts/baofu-payment/baofu-sandbox-evidence.md` or successor | Required to close C4 |

## 11. Open Decisions Before Implementation

These are not blockers for the calculator/schema design, but they determine exact routing behavior.

1. Baofoo non-sharing/direct-settlement path: current BaoCaiTong aggregate docs used by LocalLife do not expose a direct-settlement path that also lets LocalLife collect the 0.6% fee. First implementation is therefore `fee_only_share`; `direct_no_share` requires a separate official-contract design if Baofoo later opens such a product for this contract.
2. Commission rounding: this design preserves existing floor rounding for 2%/3%; changing it requires an explicit finance decision.
3. Payer-level fee rounding: this design uses per-payer upward rounding for merchant/rider 0.6%. This can make merchant+rider fees differ by a fen from `ceil(total * 0.6%)`, but it correctly charges each payer by their own base.
4. Existing test rows: because main-business Baofoo is not yet live, historical Baofoo rows can remain legacy/test data. Do not run destructive cleanup without a separate approval.

## 12. Recommended Implementation Order

1. Calculator tests and calculator implementation.
2. Additive migration + sqlc.
3. Profit-sharing persistence + fee ledgers.
4. Settlement mode routing for takeout/reservation/dine-in/takeaway.
5. Callback actual `feeAmt` persistence and reconciliation.
6. Merchant/rider/platform finance query/API updates.
7. Smoke/evidence update and production-first-order checklist update.

This order keeps the money formula testable before touching callbacks or worker side effects, and keeps all schema changes additive until production evidence proves the new fields.

## 13. Implementation Status - 2026-05-05

Implemented in this pass:

- Added `CalculateBaofuSettlementAmounts` with `baofu_fee_v2` examples for takeout, reservation, dine-in/takeaway fee-only settlement, actual provider fee input, and negative receiver guards.
- Added additive migrations `000230_add_baofu_fee_breakdown` and `000231_relax_profit_sharing_rider_payment_fee`.
- Added `order_payment_fee_ledgers` queries and sqlc models for provider cost, merchant payment service fee, and rider payment service fee.
- Updated Baofoo profit-sharing creation to persist the v2 fee breakdown, receiver snapshot, legacy provider-fee compatibility ledger, and payer-level fee ledgers in one transaction.
- Updated Baofoo payment fact recording to upsert actual provider `feeAmt` as `actual_callback` or `actual_query` when Baofoo supplies it.
- Updated merchant finance SQL to treat `payment_fee` as merchant-facing 0.6% for `baofu_fee_v2`, while preserving legacy fallback for old rows.
- Updated rider income/workbench SQL, logic, API responses, and Swagger to expose rider gross delivery receivable, rider payment service fee, and rider net income.
- Updated platform Baofoo reconciliation SQL/API to expose provider cost, merchant fee income, rider fee income, platform payment-fee income, and net fee margin.
- Recorded provider fee timing in Baofoo share snapshots as `realtime_deducted_before_reserve`; `shareable_amount` and `platform_receiver_amount` are now persisted in the snapshot to make the reserve-pool balancing explicit.
- Added Baofoo reservation-profit-sharing recovery: paid Baofoo reservation/reservation-addon payment orders are scanned by `ListBaofuOrdersReadyForProfitSharing`, create `commission_share` Baofoo sharing orders with no rider receiver, and enqueue the Baofoo `share_after_pay` worker.
- Updated reservation payment outbox dispatch so Baofoo reservation payments do not enqueue the legacy WeChat profit-sharing worker; Baofoo reservation sharing is owned by the Baofoo recovery scheduler/worker path.
- Verified the current BaoCaiTong aggregate document set contains `unified_order` and no BCT aggregate combined-payment interface. Public Baofoo docs expose a separate acquiring-side "合并支付" product, but it is not part of the LocalLife BaoCaiTong aggregate contract path. Backend Baofoo combined payment stays fail-closed, and shopping-cart UX should split payments when Baofoo main-business is enabled.
- Added `GET /v1/payments/capabilities` so the Mini Program can consume backend payment-channel truth before checkout. When Baofoo main-business is enabled it returns `combined_payment_supported=false` and `split_checkout_required=true`.
- Updated the takeout cart and order-confirm pages to honor `split_checkout_required`: cart selection is limited to one merchant, order confirm blocks stale multi-merchant submissions before creating local orders, and any backend Baofoo combined-payment fail-close is shown as "逐笔支付" guidance instead of "继续完成合单支付".

Validation completed:

- `go test -count=1 ./logic -run 'Baofu|MerchantFinance|RiderIncome|RiderWorkbench'`
- `go test -count=1 ./db/sqlc -run 'Baofu|OrderPaymentFeeLedger|ProfitSharing|MerchantFinance|RiderFinance'`
- `go test -count=1 ./worker -run 'Baofu'`
- `go test -count=1 ./api -run 'Baofu|MerchantFinance|RiderIncome|RiderWorkbench|PlatformBaofu'`
- `go test -count=1 ./api -run 'TestGetPaymentCapabilitiesAPI_BaofuMainBusinessRequiresSplitCheckout|TestCreateCombinedPaymentOrderAPI_BaofuMainBusinessFailsClosed'`
- `PATH="$HOME/.local/bin:$PATH" npm run compile`
- `PATH="$HOME/.local/bin:$PATH" npm run lint -- --quiet`
- `make check-generated`
- `make test-safety`
- `git diff --check`

Known remaining gaps before declaring the Baofoo main-business finance path C4/production-complete:

- Baofoo sandbox/production-first evidence still needs to cover a real payment callback, share command, share query/callback, refund path, and withdrawal path with the new fee fields.
- `profit_sharing_orders.provider_payment_fee` stores the estimated fee used during share-order creation; actual provider `feeAmt` is first-class in `order_payment_fee_ledgers`, but profit-sharing rows are not retroactively recalculated after a later actual fee observation.
- Dine-in/takeaway no-commission orders are implemented as `fee_only_share` technical settlement through the current Baofoo sharing path, not a confirmed `direct_no_share` product.
- Reservation-specific Baofoo local C3 coverage exists, but C4 still needs real `business_type='reservation'` payment/share callback or query evidence.
- Baofoo combined payment remains unavailable in the current BaoCaiTong aggregate contract path. Backend now exposes that as a payment capability and the Mini Program splits checkout by merchant; if product later wants cart-level multi-merchant one-click payment, it needs Baofoo to open a BCT aggregate combined interface and a new contract group.
