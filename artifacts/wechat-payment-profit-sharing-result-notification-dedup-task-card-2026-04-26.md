# TASK-PAY-007K: Profit Sharing Result Notification Enqueue Dedup

Date: 2026-04-26
Risk: G3 payment/profit-sharing async side effect path

Status: Historical. Superseded by TASK-PAY-007M direct outbox cutover; the old result task is no longer the production execution surface.

## Objective

Reduce duplicate merchant success-notification enqueue risk before any production scheduler scan of `profit_sharing_result_ready` is enabled.

## Scope

- Keep old `payment:process_profit_sharing_result` active.
- Keep outbox dispatcher support from TASK-PAY-007I.
- Add a stable `ExpiresAt` calculation for profit-sharing success notification payloads when the stored profit-sharing order has `created_at`.
- Add an `asynq.Unique` window to the success notification enqueue call shared by the old result task and outbox dispatcher.
- Update focused tests to assert the success notification enqueue carries a dedupe option.

## Not In Scope

- Do not add a `notifications` table unique constraint in this slice.
- Do not change global `notification:send` semantics.
- Do not enable production scheduler scanning for `profit_sharing_result_ready`.
- Do not remove the old result task or old callback/query enqueue path.

## Review Notes

This is a mitigation, not the final idempotency boundary. It dedupes duplicate enqueue attempts when the old result task and outbox dispatcher submit the same notification payload inside the unique window. A later production enablement should still review whether DB-level notification idempotency or old-path suppression is required before broad rollout.

## Validation

Planned focused validation:

```bash
gofmt -w locallife/worker/task_process_payment.go locallife/worker/task_process_payment_test.go locallife/worker/task_payment_domain_outbox_test.go
go -C /home/sam/locallife/locallife test ./worker -run 'TestProcessTaskPaymentDomainOutbox|TestPaymentDomainOutboxScheduler|TestProcessTaskProfitSharingResult' -count=1
go -C /home/sam/locallife/locallife test . ./logic ./worker -count=1
```
