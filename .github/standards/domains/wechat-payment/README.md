# WeChat Payment Domain

WeChat payment is no longer the main-business payment provider. Main-business payment, refund, share, account opening, withdrawal, and provider callbacks belong to the Baofu/BaoCaiTong domain.

## Current Scope

Active WeChat payment capabilities are limited to direct WeChat merchant flows:

- Rider deposit payment and deposit refund.
- Merchant or rider recovery payments to the platform.
- Merchant transfer compensation and related recovery.
- Direct WeChat refund callbacks for the retained direct-payment flows.
- WeChat Mini Program shipping and settlement-event helpers that are not main-business payment acquisition.

Do not add new main-business payment, refund, share, account-opening, settlement-account, withdrawal, complaint, or fund-management behavior through WeChat. Use `.github/standards/domains/baofu-payment/README.md` for those capabilities.

## Implementation Rules

- Keep direct WeChat client wiring separate from Baofu wiring.
- Direct WeChat config is only `WECHAT_PAY_*`, Mini Program config, and shipping callback config.
- Do not reintroduce old platform client config, runtime builders, routes, validators, generated mocks, or documentation-audit surfaces.
- Do not create fallback behavior from Baofu to WeChat for main-business flows.
- Keep callback signature verification, notification decryption, idempotency, and ownership checks mandatory for retained direct WeChat callbacks.
- Keep provider DTOs and request signing inside `locallife/wechat/**`; business orchestration belongs in logic, worker, scheduler, and persistence layers.
- If a retained direct WeChat flow changes, update focused tests for the direct client/callback path and run the smallest relevant Go package tests.

## Active Callback Routes

The retained WeChat callback routes are:

- `POST /v1/webhooks/wechat-pay/notify`
- `POST /v1/webhooks/wechat-pay/refund-notify`
- `POST /v1/webhooks/wechat-pay/merchant-transfer-notify`
- `POST /v1/webhooks/wechat-miniprogram/settlement-notify`

Baofu callback routes are documented and validated in the Baofu payment domain.

## Validation

For direct WeChat payment changes, prefer focused tests first:

```bash
cd locallife
go test ./wechat ./internal/wechatruntime ./api -count=1
```

If route annotations change, run `make swagger`. If interfaces used by mocks change, regenerate mocks with the project command before closing the task.
