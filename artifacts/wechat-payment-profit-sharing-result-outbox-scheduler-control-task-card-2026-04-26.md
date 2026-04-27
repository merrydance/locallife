# TASK-PAY-007J: Profit Sharing Result Outbox Scheduler Control

Date: 2026-04-26
Risk: G3 payment/profit-sharing async scheduler path

Status: Historical. Superseded by TASK-PAY-007M direct outbox cutover; the current baseline scans `profit_sharing_result_ready` by default and no longer keeps the legacy result task as a fallback.

## Objective

Prepare payment domain outbox scheduling for real `profit_sharing_result_ready` events without changing the default production scan set yet.

## Scope

- Keep `NewPaymentDomainOutboxScheduler` default behavior probe-only.
- Add an explicit scheduler constructor that accepts event types for controlled rollout/tests.
- Verify the scheduler can enqueue both the dispatcher probe event and `profit_sharing_result_ready` when explicitly configured.
- Keep existing outbox dispatcher strict failure behavior from TASK-PAY-007I.

## Not In Scope

- Do not change `main.go` to scan `profit_sharing_result_ready` by default.
- Do not remove or disable `payment:process_profit_sharing_result`.
- Do not suppress old callback/query enqueue behavior.
- Do not claim production notification migration is complete.

## Review Notes

Opening scheduler scanning for `profit_sharing_result_ready` while the old result task remains active can duplicate merchant notifications and failure alerts. This task intentionally adds the scheduling control surface only; the production registration still uses the default probe-only constructor.

Before a later task enables the real event in production, review duplicate-delivery behavior across:

- old callback/query `DistributeTaskProcessProfitSharingResult`
- `payment_domain_outbox` scheduler scan set
- outbox strict dispatch side effects
- success notification enqueue idempotency
- failed/closed alert and profit-sharing retry enqueue semantics

## Validation

Planned focused validation:

```bash
gofmt -w locallife/worker/payment_domain_outbox_scheduler.go locallife/worker/task_payment_domain_outbox_test.go
go -C /home/sam/locallife/locallife test ./worker -run 'TestPaymentDomainOutboxScheduler|TestProcessTaskPaymentDomainOutbox' -count=1
go -C /home/sam/locallife/locallife test . ./logic ./worker -count=1
```
