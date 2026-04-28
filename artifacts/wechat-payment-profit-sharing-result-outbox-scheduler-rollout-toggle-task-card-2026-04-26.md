# TASK-PAY-007L: Profit Sharing Result Outbox Scheduler Rollout Toggle

Date: 2026-04-26
Risk: G3 payment/profit-sharing async scheduler path

Status: Historical. Superseded by TASK-PAY-007M direct outbox cutover; the rollout toggle was removed and `profit_sharing_result_ready` scanning is now the default baseline.

## Objective

Wire a disabled-by-default rollout toggle for scanning real `profit_sharing_result_ready` payment domain outbox events.

## Scope

- Add `PAYMENT_DOMAIN_OUTBOX_PROFIT_SHARING_RESULT_SCHEDULER_ENABLED` to backend config.
- Default the toggle to `false`.
- Keep production `main.go` probe-only unless the toggle is explicitly enabled.
- Document the toggle in `app.env.example` with the dual-track duplicate side-effect warning.
- Add config tests for default false and explicit true.

## Not In Scope

- Do not enable the toggle by default.
- Do not remove or suppress old `payment:process_profit_sharing_result`.
- Do not claim notification migration is complete.
- Do not add notification table unique constraints in this slice.

## Review Notes

When enabled, scheduler scanning can enqueue outbox dispatch for `profit_sharing_result_ready` while old callback/query paths may still enqueue `payment:process_profit_sharing_result`. TASK-PAY-007K added enqueue-level notification dedupe, but broad production enablement still needs an explicit dual-track rollout decision and monitoring.

## Validation

Planned validation:

```bash
gofmt -w locallife/main.go locallife/util/config.go locallife/util/config_test.go
go -C /home/sam/locallife/locallife test ./util -run 'TestLoadConfig_DefaultsAndTrimQuotes|TestLoadConfig_ReadsPaymentDomainOutboxSchedulerToggle' -count=1
go -C /home/sam/locallife/locallife test . ./util ./logic ./worker -count=1
```
