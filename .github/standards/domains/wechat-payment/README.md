# WeChat Payment Domain Index

This directory contains active payment guidance and historical rollout material.

## Active Guidance

- `WECHAT_PAYMENT_OPERATIONS_RUNBOOK_2026-03-24.md`: current production operations and recovery reference.
- `WECHAT_PAYMENT_COMPLAINT_SUBSIDY_FRONTEND_SPEC_2026-03-22.md`: active cross-surface behavior reference when complaint or subsidy work spans backend and client flows.
- `WECHAT_PAYMENT_MERCHANT_MEDIA_UPLOAD_CONTRACT_2026-04-13.md`: active contract and validation baseline for `/v3/merchant/media/upload`, including service-provider signing, real-image validation, and the current 2MB local enforcement decision.

## Historical Rollout Material

- `historical/WECHAT_PAYMENT_REFACTOR_EXECUTION_PLAN_2026-03-24.md`

Use the execution plan only when a task depends on rollout-stage assumptions, migration history, or unfinished staged work. It should not be the default first read for routine payment maintenance.