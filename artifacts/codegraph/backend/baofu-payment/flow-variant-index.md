# Baofoo Payment Flow Variant Index

Status: working coverage map for LocalLife Baofoo/BaoCaiTong codegraph slices
Risk class: G3 whenever a row touches payment, refund, profit sharing, callbacks, recovery, or money movement

## Why This Exists

LocalLife does not have one "order payment" flow or one "refund" flow. The same provider callback/fact tables can branch by:

- `payment_orders.payment_channel`
- `payment_orders.business_type`
- `payment_orders.order_id` / `reservation_id`
- profit-sharing requirement and current share/refund state
- command source: user cancel, merchant reject, reservation cancel, replace order, recovery, anomaly handling
- provider truth source: callback, query recovery, or synchronous command response

Codegraph slices must therefore name the exact variant they cover. A slice that says "order refund" without business type, provider, source, and terminalizer is incomplete.

## Business Owners And Payment Targets

Observed constants and modeled owners:

| Owner / business type | Typical object | Payment channel(s) seen | Fact consumer/domain | Notes |
| --- | --- | --- | --- | --- |
| `order` | `orders`, `payment_orders`, `refund_orders` | Baofoo aggregate, direct payment | `order_domain` | Main takeout/order path; Baofoo path may require profit sharing. |
| `reservation` | `reservations`, `payment_orders`, `refund_orders` | Baofoo aggregate | `reservation_domain` | Reservation prepaid payment/refund path has prepaid-amount side effects. |
| `reservation_addon` | reservation dish/add-on adjustments | Baofoo aggregate | `reservation_domain` | Stored as a separate payment business type but refund facts route to reservation domain. |
| `rider_deposit` | rider deposit payment/refund | direct payment | `rider_deposit_domain` | Direct WeChat payment/refund fact path, not Baofoo aggregate. |
| `claim_recovery` | claim recovery payment/refund | direct payment | `claim_recovery_domain` | Separate direct-payment owner. |
| `baofu_account_verify_fee` | Baofoo account verification fee | direct payment | `baofu_account_verify_fee_domain` | Payment terminalization differs from main business orders. |
| `profit_sharing` | `profit_sharing_orders`, returns | Baofoo aggregate | `profit_sharing_domain` | Share command/result and return/refund flows should be separate slices. |

## Current Slice Coverage

| Slice | Covered variants | Explicitly not covered |
| --- | --- | --- |
| `aggregate-payment.slice.md` | Baofoo aggregate payment callback for `business_type = order`, order-domain payment application, profit-sharing bill/command/result path. | Reservation payment, reservation add-on payment, direct payment owners, refund paths, post-share returns. |
| `refund.slice.md` | Baofoo aggregate pre-share refund command/callback/query for `order`, `reservation`, and `reservation_addon` fact application. | Rider deposit direct refunds, claim recovery direct refunds, post-share refund/return, replace-order orchestration details, merchant-reject orchestration details. |

## Missing High-Value Slices

The next useful slices should be separate files, not merged into the two current ones:

- Baofoo aggregate reservation payment callback -> reservation payment application -> reservation side effects.
- Baofoo aggregate reservation add-on payment callback -> reservation add-on application.
- Baofoo profit-sharing return / post-share refund path -> refund order terminalization.
- Merchant reject refund orchestration -> Baofoo pre-share refund command/fact application.
- Replace-order flow -> old order refund + new order payment interactions.
- Direct WeChat rider deposit payment/refund fact chain.
- Claim recovery direct payment/refund fact chain.
- Baofoo account verify fee direct payment/query terminalization.
- Payment/refund anomaly handling and automatic refund after closed/failed payment states.

## Slice Requirements

Every new slice should declare:

- Provider and channel.
- `business_type` and owning domain.
- Entry source: API, worker, scheduler, callback, or query recovery.
- Terminalizer: which function mutates business state.
- Money guard: amount, idempotency, lock, and no-race rule.
- Outbox or async recovery boundary.
- Explicit non-coverage list.
