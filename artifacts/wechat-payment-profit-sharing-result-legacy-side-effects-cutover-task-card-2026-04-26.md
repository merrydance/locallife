# TASK-PAY-007M: Profit Sharing Result Outbox Cutover

Date: 2026-04-26
Risk: G3 payment/profit-sharing async side-effect path

## Objective

Switch profit-sharing result side effects directly to `payment_domain_outbox` ownership. The legacy `payment:process_profit_sharing_result` task is no longer a compatibility layer or fallback execution path.

## Scope

- Make the payment domain outbox scheduler scan `profit_sharing_result_ready` by default.
- Remove callback/query enqueueing of the legacy `payment:process_profit_sharing_result` task.
- Remove the legacy worker handler registration and config toggles.
- Keep outbox dispatcher strict behavior unchanged: published means side effects were durably accepted.
- Update focused tests so callback/query terminal facts rely on fact application and outbox instead of the legacy result task.

## Not In Scope

- Do not add database-level notification uniqueness in this slice.

## Review Notes

This slice intentionally avoids a dual-track compatibility layer. Terminal callback/query facts are applied through `external_payment_fact_applications`, which creates `profit_sharing_result_ready` outbox events. The outbox scheduler and dispatcher are the only production execution surface for merchant notifications, failure alerts, and profit-sharing retry enqueueing.

## Validation

Planned validation:

```bash
gofmt -w locallife/main.go locallife/api/payment_callback.go locallife/api/payment_callback_test.go locallife/worker/processor.go locallife/worker/task_process_payment.go locallife/worker/task_process_payment_test.go locallife/worker/payment_domain_outbox_scheduler.go locallife/worker/task_payment_domain_outbox_test.go locallife/util/config.go locallife/util/config_test.go
go -C /home/sam/locallife/locallife test ./worker -run 'TestProcessTaskProfitSharing|TestProcessTaskPaymentDomainOutbox|TestPaymentDomainOutboxScheduler' -count=1
go -C /home/sam/locallife/locallife test ./api -run 'TestHandleProfitSharingNotify' -count=1
go -C /home/sam/locallife/locallife test . ./util ./logic ./worker -count=1
```
