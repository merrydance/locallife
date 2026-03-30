---
name: "骑手状态与押金方案规划"
description: "Use when planning or auditing backend work that aligns rider application state, rider operational status, deposit thresholds, and rider-only applyment cleanup with the finalized business model. Trigger phrases: rider status deposit plan, audit rider applyment cleanup, plan rider threshold refactor, rider onboarding removal plan. 适用于规划或审查骑手审核状态、运营状态、押金阈值与骑手开户残留清理。"
argument-hint: "目标分支、已知差异、是否允许改 migration / swagger / tests"
agent: "Plan"
---

# Rider Status And Deposit Planning Template

Use this prompt when planning, auditing, or closing a backend change that must align the rider domain with the finalized rider status and deposit model.

## Task

Produce a gap analysis and execution plan for this rider-domain change.

Required focus:

- separate rider application process state from rider operational status
- remove rider-only WeChat applyment and onboarding semantics without affecting merchant or operator applyment behavior
- drive rider status transitions only from review result, suspension state, and deposit principal threshold reconciliation
- verify recommendation, broadcast, online gate, and grab-order behavior all follow the same operational rules
- check whether routes, SQL/sqlc, worker flows, tests, Swagger, and docs are fully propagated

## Target Model

Application state:

- `draft`: editable draft
- `submitted`: submitted for process or audit tracking
- `approved`: application review passed
- review failure returns to `draft` and preserves the failure reason
- rider application must not expose `rejected` as an external lifecycle state

Rider operational state:

- `approved`: review passed, deposit principal below threshold, not yet operational
- `active`: review passed, deposit principal at or above threshold, operational
- `suspended`: manual or risk suspension
- rider operational state must not use `pending`

Status triggers:

- review success creates the rider and leaves rider status at `approved`
- deposit principal crossing the configured threshold promotes or demotes between `approved` and `active`
- freeze and unfreeze must not change rider status
- suspension must force offline
- resume must reconcile back to `approved` or `active`, not force `active`

Order and eligibility model:

- all takeout orders follow the same proxy-pickup freeze logic
- per-order freeze amount is the order final payable total
- a rider may accept a new order only when current frozen demand plus the new freeze amount does not exceed currently available deposit
- online gate equals the platform-configured rider deposit threshold directly
- downgraded but still-online riders may finish active deliveries, but must not see or accept new orders
- once no active deliveries remain, a non-`active` online rider must be forced offline

## Files To Inspect First

- [locallife/db/sqlc/tx_rider_application.go](../../locallife/db/sqlc/tx_rider_application.go)
- [locallife/api/rider_application.go](../../locallife/api/rider_application.go)
- [locallife/api/rider.go](../../locallife/api/rider.go)
- [locallife/db/sqlc/tx_payment_success.go](../../locallife/db/sqlc/tx_payment_success.go)
- [locallife/logic/rider_deposit_refund_service.go](../../locallife/logic/rider_deposit_refund_service.go)
- [locallife/logic/delivery_grab.go](../../locallife/logic/delivery_grab.go)
- [locallife/db/sqlc/tx_delivery.go](../../locallife/db/sqlc/tx_delivery.go)
- [locallife/api/payment_callback.go](../../locallife/api/payment_callback.go)
- [locallife/worker/task_process_payment.go](../../locallife/worker/task_process_payment.go)
- [locallife/api/ecommerce_applyment.go](../../locallife/api/ecommerce_applyment.go)
- [locallife/db/migration/000052_add_ecommerce_applyments.up.sql](../../locallife/db/migration/000052_add_ecommerce_applyments.up.sql)
- [locallife/db/sqlc/rider_application.sql.go](../../locallife/db/sqlc/rider_application.sql.go)

## Planning Checklist

1. Confirm the rider approval flow creates approved application, rider record, and rider user role in one transaction.
2. Confirm rider records are created only on review success and role creation does not wait for deposit top-up.
3. Confirm rider status upgrades and downgrades depend only on deposit principal threshold reconciliation.
4. Confirm online eligibility, recommendation, broadcast, and grab-order gates all exclude non-`active` riders from new work.
5. Confirm freeze and unfreeze affect deposit capacity only, not rider status.
6. Confirm downgraded riders can finish active deliveries and are auto-offlined after the last active delivery completes or is canceled.
7. Confirm rider applyment routes, callback branches, notification flows, compatibility branches, and tests are removed or narrowed away from rider usage.
8. Confirm merchant and operator applyment schema and runtime semantics remain intact.
9. List any remaining mentions of rider-only `pending`, `pending_bindbank`, `bindbank_submitted`, or external `rejected` in code, tests, docs, or constraints.
10. State which regeneration steps are required: `make sqlc`, `make mock`, `make swagger`, or none.
11. State which validations should run and which high-risk paths remain unverified if evidence is missing.

## Expected Output

Return the result in this order:

1. Findings first, if current code still misses the target model.
2. A concise execution plan grouped by layer or risk area.
3. Required regeneration steps.
4. Validation plan.
5. Residual risks or open questions.

If the code already matches the target model, say so explicitly and list only the remaining cleanup or verification items.
