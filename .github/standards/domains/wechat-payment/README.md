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

## Merchant Transfer Compensation Contract

Active provider/capability: WeChat Pay direct merchant, merchant transfer user-confirmation mode.

Official source set:

- Product introduction and real-name amount boundary: `https://pay.wechatpay.cn/doc/v3/merchant/4012711988`
- Create transfer: `https://pay.wechatpay.cn/doc/v3/merchant/4012716434`
- JSAPI user confirmation: `https://pay.wechatpay.cn/doc/v3/merchant/4012716430`
- Query by merchant bill no: `https://pay.wechatpay.cn/doc/v3/merchant/4012716437`
- Query by WeChat bill no: `https://pay.wechatpay.cn/doc/v3/merchant/4012716457`
- Transfer notification: `https://pay.wechatpay.cn/doc/v3/merchant/4012712115`
- Merchant transfer FAQ: `https://pay.wechatpay.cn/doc/v3/merchant/4013778940`

Important boundary:

- Customer-entered payout real name and WeChat's user confirmation page are different steps.
- `users.full_name` is the LocalLife source for WeChat `user_name` when claim payout creates a merchant transfer. The WeChat client encrypts `user_name` before sending it to `/v3/fund-app/mch-transfer/transfer-bills`.
- Because LocalLife claim payout always sends `user_name` for WeChat real-name matching, claim payout amount must be at least 30 fen. WeChat allows optional `user_name` for transfer amounts from 0.30 yuan inclusive to 2,000 yuan exclusive, requires it at 2,000 yuan and above, and does not support `user_name` below 0.30 yuan.
- `package_info` is returned by WeChat only when the transfer bill is in `WAIT_USER_CONFIRM`; it is passed to the Mini Program so the client can call the WeChat confirmation component.
- `requestMerchantTransfer:ok` means the WeChat confirmation page was displayed and control returned to the Mini Program. It is not payout success. LocalLife must wait for WeChat query or notification state `SUCCESS` before treating the claim payout as completed and before continuing recovery-side effects.

Field matrix:

| Provider operation | Provider field | Type/unit | Requiredness | Local owner | Business meaning |
| --- | --- | --- | --- | --- | --- |
| Create transfer `POST /v3/fund-app/mch-transfer/transfer-bills` | `appid` | string | required | `DirectMerchantTransferCreateRequest.AppID` | Mini Program app id used for the transfer. |
| Create transfer | `out_bill_no` | string | required | `claimPayoutOutBillNo(actionID)` | Local idempotency key. Never change this merely to retry an unclear provider result. |
| Create transfer | `transfer_scene_id` | string | required | `DirectMerchantTransferSceneEnterpriseCompensation` | Enterprise compensation scene. Current LocalLife claim payout uses `1011`. |
| Create transfer | `openid` | string | required | `users.wechat_openid` | Receiving customer identity under the Mini Program app id. |
| Create transfer | `user_name` | encrypted string | conditional by WeChat amount; required by LocalLife for claim payout | `users.full_name` -> `EncryptSensitiveData` | Payout real name supplied by the customer for WeChat real-name matching. It is not the WeChat confirmation UI. Not supported by WeChat when `transfer_amount` is below 30 fen; required by WeChat at 200,000 fen and above. |
| Create transfer | `transfer_amount` | integer fen | required | payout action `amount` | Compensation amount in cents/fen. LocalLife claim payout minimum is 30 fen because claim payout sends `user_name`. |
| Create transfer | `transfer_remark` | string | required | payout action `remark` or default compensation reason | User-visible/recorded transfer reason. |
| Create transfer | `notify_url` | string URL | optional request field; required by LocalLife config | `WECHAT_PAY_MERCHANT_TRANSFER_NOTIFY_URL` | Callback destination for terminal transfer state. |
| Create transfer | `user_recv_perception` | enum string | optional | `DirectMerchantTransferUserRecvPerceptionMerchantCompensation` | User-facing receipt perception, currently merchant compensation. |
| Create transfer | `transfer_scene_report_infos[].info_type` | string | required for enterprise compensation | `DirectMerchantTransferReportInfoTypeCompensationReason` | Enterprise compensation report info key, fixed to compensation reason. |
| Create transfer response | `state` | enum string | required | `DirectMerchantTransferCreateResponse.State` | Creation result state; `WAIT_USER_CONFIRM` requires Mini Program confirmation. |
| Create transfer response | `package_info` | string | conditional | payout action `PackageInfo` -> `GET /v1/claims/{id}/payout-confirmation` | Token passed to `wx.requestMerchantTransfer` only while waiting for user confirmation. |
| JSAPI confirmation | `mchId`, `appId`, `package` | strings | required | `RequestMerchantTransferParams` | Mini Program parameters for WeChat's native confirmation page. |
| JSAPI confirmation result | `requestMerchantTransfer:ok` | string | returned by client | Mini Program detail page | Page-display result only; backend status must still be refreshed. |

## Validation

For direct WeChat payment changes, prefer focused tests first:

```bash
cd locallife
go test ./wechat ./internal/wechatruntime ./api -count=1
```

If route annotations change, run `make swagger`. If interfaces used by mocks change, regenerate mocks with the project command before closing the task.
