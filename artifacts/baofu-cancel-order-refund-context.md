# Baofoo Cancel-Order Refund Context

Date: 2026-05-20
Worktree: `.worktrees/feat-cancel-refund-progress`
Last updated: 2026-05-20 19:52 Asia/Shanghai

## What this task is about

When a paid takeaway order is canceled, the system must keep the order itself in the canceled terminal state and repair the refund chain separately. The bug was not "order status wrong"; it was "refund chain failed and cannot self-heal".

## Confirmed production facts

- Order `202605191920420fbb3f`:
  - `orders.id = 7`
  - `orders.status = cancelled`
  - `orders.cancel_reason = 配送时间太长`
- Payment:
  - `payment_orders.id = 19`
  - `payment_orders.status = paid`
  - `payment_orders.amount = 1459`
  - `payment_orders.payment_channel = baofu_aggregate`
  - `payment_orders.business_type = order`
- Refund:
  - `refund_orders.id = 78`
  - `refund_orders.out_refund_no = RF19_7`
  - `refund_orders.status = failed`
  - `refund_orders.refund_amount = 1459`
  - Baofoo callback failure reason: `CM_NOT_ENOUGH_BALANCE_REFUND` / `没有足够的余额退款`
- Recovery refund created on 2026-05-20:
  - created through `POST /v1/refunds` on production localhost as merchant owner user `37`
  - `refund_orders.id = 79`
  - `refund_orders.out_refund_no = R20260520193201058221a1`
  - `refund_orders.refund_id = 260520111106224546778528`
  - `refund_orders.status = processing` as of 2026-05-20 19:51 Asia/Shanghai
  - `external_payment_commands.id = 30`
  - `external_payment_commands.command_status = accepted`
  - Baofoo admin showed the same refund as `退款中` at apply time `2026-05-20 19:32:01`
  - Production refund recovery queried Baofoo at `2026-05-20 19:50:00` and logged `stuck refund query still reports processing; keep waiting`
- Profit sharing:
  - no `profit_sharing_orders` row exists for `payment_order_id = 19`
- External payment trail:
  - `external_payment_commands.id = 29` is `accepted`
  - `external_payment_facts.id = 57` is terminal `failed`
  - `external_payment_fact_applications.id = 39` is `applied`
- Payment-domain outbox:
  - `payment_domain_outbox.id = 3` keeps retrying `order_payment_succeeded`
  - current failure: `notify merchant new order: merchant new order notification requires paid order: order_id=7 status=cancelled`
  - this is a separate post-payment notification/outbox issue; it is not required to submit the refund

## Important conclusions

- Do not restore the order back to normal. That would drift the business state.
- The Baofoo refund pipeline currently reuses a fixed `out_refund_no` pattern (`RF{paymentOrderID}_{orderID}`), so reusing the same refund record is not a safe recovery path.
- The cancel-order refund recovery task still uses the fixed `RF{paymentOrderID}_{orderID}` pattern. For this order it resolves the old terminal failed `RF19_7` row and skips, so simply re-enqueueing `payment:initiate_refund` is not enough.
- `ListPaidUnrefundedPaymentOrders` in local code excludes any payment order that has any refund row. A local/manual query adjusted to ignore terminal failed refunds would find payment order `19`, but the worker would still hit `RF19_7` and skip.
- The production-safe compensation performed here was to create a fresh refund command / refund order through the merchant refund API, not to change `orders.status`.
- Baofoo returned synchronous accept for the new refund; final settlement still depends on callback or stuck-refund query recovery.
- Baofoo docs note that after Baofoo receives refund funds it submits to the bank within 1 business day, and actual refund arrival is commonly 3-7 business days depending on the bank. For this incident, `退款中` / local `processing` is consistent with that async settlement window; do not re-submit another refund while this order remains in Baofoo `退款中`.
- `refund_recovery_scheduler.recoverStuckProcessingRefunds` has Baofu capability checks and can query stuck `processing` refunds after about 15 minutes, but verify the exact Baofu order-refund fact application path if callback does not arrive.

## Relevant code paths

- Refund creation:
  - `locallife/logic/refund_service.go`
  - `locallife/api/payment_order.go`
- Refund worker:
  - `locallife/worker/task_process_payment.go`
- Refund recovery:
  - `locallife/worker/refund_recovery_scheduler.go`
- SQL:
  - `locallife/db/query/refund_order.sql`
  - `locallife/db/query/payment_order.sql`

## Operational note

The production server is reachable through `ssh -p 22333 sam@aliyun`. The DB was inspected directly from the live server to confirm the real state above.

## Commands/actions already performed

- Generated a short-lived Paseto access token on the production host for merchant owner user `37`.
- Called:

```bash
curl -X POST http://127.0.0.1:8080/v1/refunds \
  -H "Authorization: Bearer <short-lived-token>" \
  -H "Content-Type: application/json" \
  --data '{"payment_order_id":19,"refund_type":"full","refund_amount":1459,"refund_reason":"配送时间太长（旧退款失败后重试）"}'
```

- API returned HTTP `201` with `refund_orders.id = 79` and `status = processing`.
- Follow-up production DB checks confirmed:
  - old refund `78` remains `failed`
  - new refund `79` is `processing`
  - external command `30` is `accepted`
  - no new refund callback fact had arrived yet as of 2026-05-20 19:51 Asia/Shanghai
  - `external_payment_facts` still only has the old failed refund fact `id=57` for `RF19_7`; no fact exists yet for `R20260520193201058221a1`
  - `payment_orders.id=19` remains `paid` until the new refund reaches a terminal success state and the fact application updates it

## Next step to continue from here

1. Use this document as the source of truth for the current incident state.
2. Re-check `refund_orders.id = 79` and external facts for out refund no `R20260520193201058221a1`.
3. If a success callback/fact arrives, confirm `refund_orders.id = 79` becomes `success`, `payment_orders.id = 19` becomes `refunded`, and an `order_refund_succeeded` outbox is created/published.
4. If it remains `processing`, keep checking callback/query facts for `R20260520193201058221a1` / `260520111106224546778528`. The 2026-05-20 19:50 recovery query still reported processing, which matches Baofoo's documented 1 business day submission plus possible 3-7 business day bank arrival window.
5. Treat `payment_domain_outbox.id = 3` as a separate follow-up: it is repeatedly trying to send a new-order notification for an already cancelled order. Do not mark it published unless the product/ops decision is to explicitly discard that stale side effect.
