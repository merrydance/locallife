---
name: "骑手状态与押金规则实现"
description: "Use when implementing or finishing the backend refactor that removes rider-only applyment logic and aligns rider application state, operational status, deposit thresholds, and order freeze rules with the finalized business model. Trigger phrases: implement rider status refactor, remove rider applyment, align rider deposit threshold, fix rider approved active transitions. 适用于落实骑手审核状态、运营状态、押金阈值与接单门禁的最终规则。"
argument-hint: "目标范围、相关文件、是否需要同步 sqlc / mock / swagger"
agent: "agent"
---

# Rider Status And Deposit Implementation Template

Use this prompt when implementing or finishing the rider-domain refactor for the finalized status, deposit, payout, and rider-only applyment cleanup model.

## Objective

Refactor the rider domain so that it no longer uses WeChat sub-merchant onboarding, uses the configured rider deposit threshold consistently, and cleanly separates application review state from rider operational state.

## Business Rules

1. Riders do not use WeChat sub-merchant onboarding.
2. Rider payout uses personal profit-sharing receivers only.
3. Remove rider-specific applyment and onboarding logic completely.
4. Do not affect merchant or operator applyment flows.
5. Rider application failure returns to `draft` with failure reason preserved.
6. Rider application no longer exposes or uses `rejected` as an external state.
7. Rider entity status machine removes `pending` completely.
8. Rider entity statuses are:
   - `approved`: review passed, deposit principal below platform threshold, not yet operational
   - `active`: review passed, deposit principal at or above platform threshold, operational
   - `suspended`: manual or risk suspension
9. Rider records are created only when application review succeeds.
10. Rider user role is created at the same time as rider creation.
11. Application approval, rider creation, and rider role creation must be in one transaction.
12. Rider role creation must not wait for deposit top-up.
13. `submitted` remains as an application audit and process state.
14. Online gate equals the platform rider deposit threshold directly.
15. Rider status upgrades and downgrades depend only on deposit principal crossing the configured threshold.
16. Deposit freeze and unfreeze must never change rider status.
17. All takeout orders follow the same proxy-pickup freeze model.
18. A rider may accept a new takeout order only if current frozen demand plus the new order freeze amount does not exceed current available deposit.
19. Per-order freeze amount is the order final payable total.
20. When deposit principal falls below the configured threshold, rider operational status becomes `approved` immediately, but an online rider with active deliveries may stay online only to finish current work.
21. A rider in `approved` status must not receive new-order recommendations, broadcasts, or successfully accept new orders, even if they remain online to finish current deliveries.
22. Once the rider has no active deliveries, a non-`active` online rider must be forced offline automatically.
23. Manual or risk suspension must force the rider offline.
24. Resuming a suspended rider must restore status by threshold reconciliation, not by unconditionally setting `active`.

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

## Required Changes

1. Refactor rider approval flow so that review success creates approved application, rider record with status `approved`, and rider user role in one transaction.
2. Refactor deposit-success handling so that when deposit principal reaches the configured threshold, rider status becomes `active`.
3. Refactor deposit-refund success handling so that when deposit principal drops below the configured threshold, rider status becomes `approved`.
4. Refactor online eligibility to use the platform-configured rider deposit threshold instead of any hardcoded minimum.
5. Refactor takeout order freeze checks so they use the order final payable total and the rider's remaining available deposit capacity.
6. Remove rider WeChat applyment runtime logic, including routes, callbacks, notifications, comments, tests, and compatibility branches.
7. Remove rider-specific applyment schema remnants where safe, including rider-only onboarding fields and rider-only onboarding statuses, without breaking merchant or operator applyment support.
8. Remove rider `pending`, `pending_bindbank`, `bindbank_submitted`, and external `rejected` usage from rider-specific code paths, docs, tests, and constraints.
9. Keep merchant and operator applyment schema and runtime behavior intact.
10. Keep downgraded riders from taking or seeing new orders while still allowing them to finish already assigned active deliveries.
11. Auto-offline downgraded riders after the last active delivery completes or is canceled.
12. Make suspend and resume paths converge to the same operational-status rules as deposit reconciliation.

## Constraints

1. Do not add compatibility code for old rider data.
2. Prefer removing dead paths over adding fallbacks.
3. Do not change merchant or operator onboarding semantics except where shared code must be narrowed to exclude riders.
4. Do not make rider status depend on deposit freeze amount.
5. Do not delay rider role creation until deposit is paid.
6. Do not force-cancel or interrupt already assigned active deliveries merely because deposit principal dropped below threshold.

## Important Clarifications

1. `submitted` is retained only as an application process and audit state.
2. Rider operational status is separate from application process state.
3. Refund-based deposit return must drive status changes from resulting principal balance, not from refund initiation itself.
4. The takeout freeze model is unified; there is no separate normal takeout versus proxy-pickup branch for rider deposit logic.
5. Existing high-value-order premium-score logic is separate from deposit freeze rules unless explicit cleanup is required for consistency.
6. The online gate applies both to manual go-online and to new-order discovery and acceptance. It does not require immediately kicking a rider offline while active deliveries still exist.

## Validation Requirements

1. Verify rider approval now creates rider and rider role transactionally and leaves rider in `approved`, not `active`.
2. Verify topping up deposit across the configured threshold promotes rider to `active`.
3. Verify refunding deposit below the configured threshold demotes rider to `approved`.
4. Verify online eligibility reads the configured rider deposit threshold.
5. Verify freeze and unfreeze do not mutate rider status.
6. Verify new takeout-order acceptance is blocked when cumulative freeze demand would exceed available deposit.
7. Verify rider applyment routes, callback branches, notifications, and tests are removed or narrowed away from rider usage.
8. Verify merchant and operator applyment flows still compile and behave correctly.
9. Verify rider application no longer depends on external `rejected` state.
10. Verify downgraded but still-online riders cannot receive or accept new orders, while they can still finish already assigned active deliveries.
11. Verify a downgraded online rider is auto-offlined after the last active delivery completes or is canceled.
12. Verify suspend and resume paths force offline on suspension and restore `approved` or `active` via threshold reconciliation on resume.
13. Run the smallest relevant tests for rider application, rider status, deposit, delivery grab and freeze, recommendation or discovery gates, and applyment-related code paths.

## Output Expectations

- Implement the code directly rather than stopping at design notes.
- Keep changes scoped and remove obsolete rider-onboarding code thoroughly.
- Preserve existing merchant and operator patterns.
- In the hand-off, state which layers changed, which regeneration steps were required, which tests ran, and which relevant paths remain unverified.