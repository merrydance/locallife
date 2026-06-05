# Cloud Printer Provider Contract Matrix

**Scope:** LocalLife multi-provider cloud-printer contract for Feieyun baseline plus Yilianyun and Shangpeng first production rollout.

**Status:** contract document only. Use this as the source of truth before changing Mini Program or backend implementation.

**Risk Class:** G2. Cloud-printer providers are external APIs that affect order fulfillment observability, async retry, merchant-visible device state, and customer service diagnosis. Provider field drift must fail closed or surface a stable retry/rebind state; it must not silently mark failed printing as success.

---

## 1. Hard Decisions

1. Yilianyun LocalLife rollout uses **open application** mode. Do not implement Yilianyun self-owned application binding unless product/ops explicitly changes the application type and this document is revised first.
2. Yilianyun open application mode does **not** call `/printer/addprinter`. Official docs state that open-app authorization success means the printer is bound. LocalLife binding is therefore: obtain per-printer `access_token` / `refresh_token` through authorization, persist encrypted token state, then create/activate the local `cloud_printers` record.
3. Yilianyun scan-code authorization is not "just a single arbitrary string" in the API contract. The official `/oauth/scancodemodel` endpoint requires `machine_code` plus exactly one of `qr_key` or `msign`. The official preparation page defines terminal identifiers as `终端号(machine_code)` plus `终端密钥(msign)`, so first-release manual merchant entry is a two-value flow like Feieyun/Shangpeng. A Mini Program scan UX may parse `machine_code + qr_key` or `machine_code + msign` from a QR payload, but implementation must first capture real QR samples and document the parser.
4. Shangpeng supports remote device add through `POST /v1/printer/add`. LocalLife should not require an operator to manually add every merchant printer in the Shangpeng management console before binding.
5. Shangpeng `business` is the provider's device business type, not LocalLife merchant id. Official docs describe `1` as print business and `2` as scan business. Receipt printers must use `business=1`.
6. Feieyun remains the compatibility baseline. Any Yilianyun/Shangpeng implementation must not change current Feieyun request shape, callback handling, or status semantics.
7. Provider request/response/callback DTOs stay under the `cloudprint` or provider-specific boundary. API, worker, and Mini Program must consume normalized LocalLife semantics.
8. Do not add provider token env vars for open-app Yilianyun printers. `access_token` and `refresh_token` are obtained from authorization flows and stored encrypted per printer authorization.

---

## 2. Official Source Set

### Yilianyun

Current official docs are docsify markdown pages under `https://doc2.10ss.net/`.

| Topic | Source |
| --- | --- |
| Directory | https://doc2.10ss.net/_sidebar.md |
| Preparation and terminal identifiers | https://doc2.10ss.net/md/使用前准备.md |
| Calling protocol, host, signature | https://doc2.10ss.net/md/调用协议.md |
| Self-owned application token, contrast only | https://doc2.10ss.net/md/自有型应用.md |
| Open application OAuth authorization-code mode | https://doc2.10ss.net/md/开放型应用.md |
| Open application scan-code authorization mode | https://doc2.10ss.net/md/扫码授权.md |
| Device binding | https://doc2.10ss.net/md/设备绑定.md |
| Device unbind / cancel authorization | https://doc2.10ss.net/md/设备解绑或取消授权.md |
| Text print | https://doc2.10ss.net/md/文本打印.md |
| Single order status | https://doc2.10ss.net/md/单订单状态获取.md |
| Printer online/paper status | https://doc2.10ss.net/md/状态获取.md |
| Push URL setup | https://doc2.10ss.net/md/推送地址设置.md |
| Print completion push | https://doc2.10ss.net/md/打印完成推送.md |

Yilianyun current calling protocol says domestic base URL is `https://open-api.10ss.net/v2`, overseas base URL is `https://open-api-os.10ss.net/v2`, and form requests use `application/x-www-form-urlencoded`.

### Shangpeng

Official open platform page:

- https://spyun.net/open/index.html

The page is a client-rendered single-page app. The official bundle exposes the documented host `https://open.spyun.net`, endpoint paths, request fields, signature examples, and provider field descriptions. Local contract rows below reference that official page and should be re-checked against the rendered page or bundle before production enablement.

---

## 3. Provider Binding Model

| Provider | Local provider key | Device identity stored in `cloud_printers.printer_sn` | Device secret stored in `cloud_printers.printer_key` | Provider-side bind method | Can add to provider backend by API? | Local binding invariant |
| --- | --- | --- | --- | --- | --- | --- |
| Feieyun | `feieyun` | Feieyun `sn` | Feieyun device key | `POST /Api/Open/printerAddlist` | Yes | provider add succeeds before local row create; local delete calls provider delete first |
| Yilianyun | `yilianyun` | `machine_code` | empty or non-secret marker; never token | open-app authorization-code or scan-code authorization | Yes, but through authorization, not `/printer/addprinter` | encrypted authorization token and local printer row must agree on merchant + machine_code |
| Shangpeng | `shangpeng` | Shangpeng `sn` | Shangpeng `pkey` | `POST /v1/printer/add` | Yes | provider add succeeds with `business=1` before local row create; local delete calls provider delete with `business=1` |

### Yilianyun Mode Boundary

| Application mode | Token scope | Bind endpoint | Applies to LocalLife now? | Notes |
| --- | --- | --- | --- | --- |
| Self-owned application | `access_token` binds to developer `client_id` and covers devices under that developer account | `POST /printer/addprinter` | No | Official `设备绑定` page says this endpoint introduces self-owned device binding and does not support open applications. |
| Open application OAuth authorization-code | `access_token` binds to a single `machine_code` | `GET /oauth/authorize` then `POST /oauth/oauth` | Yes | Authorization success returns `access_token`, `refresh_token`, and `machine_code`; that is the provider-side binding. |
| Open application scan-code authorization | `access_token` binds to a single `machine_code` | `POST /oauth/scancodemodel` | Yes | Requires `machine_code` plus exactly one of `qr_key` or `msign`; authorization success is binding success. |

---

## 4. Yilianyun Contract

### Common Protocol

| Field | Direction | Type/unit | Requiredness | Rule |
| --- | --- | --- | --- | --- |
| `client_id` | request | string | required | Yilianyun app ID from backend console. Do not use customer/user ID here. |
| `client_secret` | signing secret | string | required | Yilianyun app secret from backend console; not sent directly as a form field. |
| `sign` | request | lowercase MD5 hex | required | `md5(client_id + timestamp + client_secret)`. |
| `timestamp` | request | unix seconds | required | 10-digit second timestamp. |
| `id` | request | UUIDv4 string | required | Request unique id. |
| `access_token` | authorized request | string | required for authorized APIs | Per-printer token from authorization store. |
| `error` | response | int/string | required | `0` success; non-zero is provider error. |
| `error_description` | response | string | required | Provider diagnostic; log safely, do not expose raw as stable public error. |
| `body` | response | JSON object | endpoint-specific | Parse strict required fields per endpoint. |

### Authorization-Code Bind

| Step | Method/Path | Request fields | Response/callback fields | LocalLife owner | Contract |
| --- | --- | --- | --- | --- | --- |
| Start authorization | `GET /oauth/authorize` | `client_id`, `response_type=code`, `redirect_uri`, `state` | Browser redirects to Yilianyun | API start endpoint or web/operator flow | Persist unguessable one-time `state` before redirect. |
| Callback | configured callback URL | Provider query/form includes `code`, `state` | `code` is terminal authorization code returned by OAuth redirect, not a merchant-entered printed machine value | API callback handler | Validate state ownership and expiry before token exchange. |
| Exchange token | `POST /oauth/oauth` | common fields plus `grant_type=authorization_code`, `scope=all`, `code` | `body.access_token`, `body.refresh_token`, `body.machine_code`, `body.expires_in`, `body.refresh_expires_in`, `body.scope` | `cloudprint` OAuth client + authorization persistence service | Store token pair encrypted and scoped to merchant/printer/machine_code. |
| Refresh token | `POST /oauth/oauth` | common fields plus `grant_type=refresh_token`, `scope=all`, `refresh_token` | new `access_token`, `refresh_token`, `machine_code`, expiry fields | scheduler/service, not request constructor | Refresh before expiry with durable locking; old token may remain valid briefly but must be replaced. |

### Scan-Code Bind

| Method/Path | Request fields | Response fields | LocalLife owner | Contract |
| --- | --- | --- | --- | --- |
| `POST /oauth/scancodemodel` | common fields plus `scope=all`, `machine_code`, exactly one of `qr_key` or `msign` | `body.access_token`, `body.refresh_token`, `body.machine_code`, `body.expires_in`, `body.refresh_expires_in`, `body.scope` | Mini Program bind API + `cloudprint` OAuth client + authorization persistence service | API validation must reject neither credential and both credentials. QR parser must be evidence-backed before auto-fill. |

Important scan UX note:

- Official API requires structured fields. If the Mini Program scan returns a single QR string, LocalLife must parse it into `machine_code` and `qr_key` or `msign` only after real printer QR samples are documented. Until then, manual field entry plus scan-assisted parsing should be treated as two separate UI states.
- Mini Program provider selection must appear before provider-specific credential controls. Feieyun and Shangpeng show SN/key input. Yilianyun first release shows `machine_code` plus terminal secret `msign`, matching the official structured authorization fields and the same two-value merchant task shape as Feieyun/Shangpeng.
- Do not call Yilianyun first-release merchant entry a one-value "authorization code" flow. The printed/manual values are `machine_code` and `msign`; the OAuth authorization-code `code` belongs only to the redirect callback path.
- `qr_key` is a terminal temporary key for scan-code authorization, not the ordinary first-release merchant input. Keep it supported at the backend contract boundary for future scan-assisted binding, but do not expose it as the default merchant field until real QR samples and operations wording are confirmed.
- The Mini Program must not display `qr_key`, `msign`, raw QR payload, `access_token`, or `refresh_token` back to the merchant. After scan or authorization, show at most masked `machine_code` plus a stable authorization/bind status.
- If scan parsing is not evidence-backed yet, do not auto-fill from `wx.scanCode`. A future parser may accept real provider QR payloads only after samples are documented; until then the production UX is manual `machine_code + msign`.

### Self-Owned Bind Endpoint, Not Current Target

| Method/Path | Fields | LocalLife decision |
| --- | --- | --- |
| `POST /printer/addprinter` | common authorized fields plus `machine_code`, optional `msign`, optional `qr_key`, optional `phone`, optional `print_name` | Do not use for current open-app rollout. Official docs state open applications do not support this endpoint and authorization success already means binding success. |

### Unbind / Cancel Authorization

| Method/Path | Request fields | Response fields | LocalLife owner | Contract |
| --- | --- | --- | --- | --- |
| `POST /printer/deleteprinter` | common authorized fields plus `machine_code` | common `error`, `error_description`, `timestamp`, `body` | provider unbind service | For open app this cancels authorization. Local delete should deactivate/remove local row and revoke provider auth in a reconciled order with retry job if remote/local state drifts. |

### Text Print

| Method/Path | Request fields | Response fields | LocalLife owner | Contract |
| --- | --- | --- | --- | --- |
| `POST /print/index` | common authorized fields plus `machine_code`, `origin_id`, `content`, optional `idempotence` | `body.id`, `body.origin_id` | Yilianyun print adapter | Use `idempotence=1`. `origin_id` is developer-side order id within 64 bytes per current docs. Persist provider `body.id` as `vendor_order_id` and LocalLife `origin_id` as provider origin/idempotency id. |

### Print Status

| Method/Path | Request fields | Response fields | Normalized mapping |
| --- | --- | --- | --- |
| `POST /printer/getorderstatus` | common authorized fields plus `machine_code`, `order_id` | `body.id`, `body.origin_id`, `body.status` | `0` -> pending/unprinted; `1` -> success/printed; `2` -> cancelled |

### Printer Status

| Method/Path | Request fields | Response fields | Normalized mapping |
| --- | --- | --- | --- |
| `POST /printer/getprintstatus` | common authorized fields plus `machine_code` | `body.machine_code`, `body.state` | `0` -> offline; `1` -> online; `2` -> out_of_paper |

### Print Completion Push

| Capability | Contract |
| --- | --- |
| Setup | `POST /oauth/setpushurl` with common authorized fields, `machine_code` required for open apps, `cmd=oauth_finish`, `url`, `status=open|close`. URL must return `{"data":"OK"}` on GET health check. |
| Callback payload | `cmd=oauth_finish`, `machine_code`, `state`, `print_time`, `origin_id`, `push_time`, `sign`. Official print completion doc states `state=1` means printed. |
| Signature | Current print completion page uses `sign`; callback signature details must be re-confirmed against current Yilianyun docs/support before production callback enablement because adjacent encrypted K8 docs describe a different RSA/ciphertext model. Polling can be the first release fallback if callback signing is not confirmed. |

---

## 5. Shangpeng Contract

### Common Protocol

| Field | Direction | Type/unit | Requiredness | Rule |
| --- | --- | --- | --- | --- |
| `appid` | request | string | required | Shangpeng app id. |
| `appsecret` | signing secret | string | required | Used only to build `sign`; never sent as ordinary field. |
| `timestamp` | request | unix seconds | required | 10-digit second timestamp. |
| `sign` | request | uppercase MD5 hex | required | Remove `sign`, drop empty values, sort params by ASCII key, join `key=value`, append `appsecret=<secret>`, MD5 uppercase. |
| `errorcode` | response | integer | required | `0` success; global errors include `-1` appid empty, `-2` appid not found, `-3` timestamp empty, `-4` sign error. |
| `errormsg` / `message` / `msg` | response | string | optional | Provider diagnostic only; map to stable LocalLife public error. |

### Remote Add / Bind

| Method/Path | Request fields | Response fields | LocalLife owner | Contract |
| --- | --- | --- | --- | --- |
| `POST /v1/printer/add` | common fields plus `business`, `sn`, `pkey`, `name` | `errorcode` success/error body | `cloudprint.ShangpengClient.AddPrinter` | This adds the device to the Shangpeng app/backend. For receipt printers, send `business=1`, not merchant id. Only create LocalLife local row after provider add succeeds, or record reconciliation if local create fails after remote add. |

`business` values from official page:

| Value | Meaning | LocalLife use |
| --- | --- | --- |
| `1` | print business, default for printers/integrated devices | Use for receipt printer add/delete. |
| `2` | scan business, default for meal-ready/scan devices; integrated devices can support both | Do not use for receipt printer rollout unless a scan device feature is designed separately. |

### Batch Add

| Method/Path | Request fields | LocalLife decision |
| --- | --- | --- |
| `POST /v1/printer/addList` | common fields plus `business` and provider batch content fields | Not first release. Use single add for merchant Mini Program binding to keep failure isolation and reconciliation simple. |

### Remote Delete / Unbind

| Method/Path | Request fields | Response fields | LocalLife owner | Contract |
| --- | --- | --- | --- | --- |
| `DELETE /v1/printer/delete` | common fields plus `sn`, optional `business` | `errorcode` success/error body | `cloudprint.ShangpengClient.RemovePrinter` | For receipt printers send `business=1` to avoid deleting scan-business binding by accident. Local delete should be reconciled if provider delete succeeds but local delete fails. |

### Text Print

| Method/Path | Request fields | Response fields | LocalLife owner | Contract |
| --- | --- | --- | --- | --- |
| `POST /v1/printer/print` | common fields plus `sn`, `content`, optional `times` | `id`, `create_time`, `errorcode` | `cloudprint.ShangpengClient.Print` | Persist response `id` as provider order id. No provider idempotency field is documented; rely on LocalLife print-log/task idempotency and poll status by `id`. |

Receipt markup note:

- Shangpeng markup is not Feieyun/Yilianyun HTML. Official examples include tags such as `<BR>`, `<CUT>`, `<IMAGE>`, `<L1>`, `<L2>`, `<C>`, `<H>`, `<W>`, `<R>`, `<B>`, `<QRCODE>`, and barcode tags.
- Do not send current Feieyun-shaped receipt content to Shangpeng unchanged. Add a provider-specific receipt renderer or a provider-neutral receipt AST with provider renderers.

### Print Status

| Method/Path | Request fields | Response fields | Normalized mapping |
| --- | --- | --- | --- |
| `GET /v1/printer/order/status` | common fields plus `id` | `status`, `print_time`, `errorcode` | `status=true` -> success/printed; `status=false` -> pending/not yet printed |

### Printer Info / Status

| Method/Path | Request fields | Response fields | Normalized mapping |
| --- | --- | --- | --- |
| `GET /v1/printer/info` | common fields plus `sn` | `online`, `status`, `sqsnum`, other model/status fields, `errorcode` | `online=1` and `status=0` -> online/normal; `online=0` -> offline; `status=1` -> abnormal |

### Device Model And Maintenance

| Method/Path | Fields | First-release decision |
| --- | --- | --- |
| `POST /v1/printer/getModel` | common fields plus `sn`, `key` | Optional. Use only if product needs model/width display. |
| `DELETE /v1/printer/cleansqs` | common fields plus `sn` | Operator maintenance only, not merchant default binding flow. |

### Callback

No official Shangpeng receipt-print completion callback was found in the current open platform page. First release should use provider order polling. Do not create `/webhooks/shangpeng/print-result` unless provider confirms a receipt print callback contract including payload, signature, retry, and ACK semantics.

---

## 6. Unified Local Contract

### Capability Matrix

| Capability | Feieyun | Yilianyun open app | Shangpeng |
| --- | --- | --- | --- |
| Remote provider bind | yes, add list | yes, authorization success | yes, add endpoint |
| Merchant manual provider-console setup required | no | no, merchant must authorize printer | no |
| Device secret entered by merchant | SN + key | scan/auth values only; token returned by provider | SN + pkey |
| Per-printer token | no | yes, encrypted at rest | no |
| Provider idempotency | not explicit | `origin_id` + `idempotence=1` | not documented |
| Print callback | current Feieyun callback exists | possible via `oauth_finish`, signature to confirm before enablement | not found |
| Poll print status | yes | yes | yes |
| Poll printer status | yes | yes | yes |
| Receipt renderer | Feieyun markup | Yilianyun command/content compatibility must be tested | Shangpeng-specific markup required |

### Local Data Rules

| Local field/table | Contract |
| --- | --- |
| `cloud_printers.printer_type` | `feieyun`, `yilianyun`, `shangpeng`. |
| `cloud_printers.printer_sn` | Provider device identity: Feieyun `sn`, Yilianyun `machine_code`, Shangpeng `sn`. |
| `cloud_printers.printer_key` | Provider device secret only when needed: Feieyun key, Shangpeng `pkey`; never store Yilianyun access/refresh tokens here. |
| Yilianyun authorization table | Required before runtime print enablement. Store encrypted `access_token` and `refresh_token`, token expiry, `machine_code`, provider app id, merchant/printer ownership, authorization status, and last provider error summary. |
| `print_logs.vendor_order_id` | Provider print order id: Feieyun order id, Yilianyun `body.id`, Shangpeng `id`. |
| `print_logs.provider_origin_id` | Needed for Yilianyun `origin_id` and callback lookup. Should be provider-scoped and generated under provider constraints. |
| Reconciliation jobs | Required when remote bind/unbind succeeds but local DB write fails, or local row exists but provider authorization/binding is missing. |

### Failure And Safety Rules

1. Remote add/auth failure must return stable LocalLife failure and must not create an active local printer row.
2. Local DB failure after remote add/auth must create a reconciliation job with enough provider identity to undo or complete the operation.
3. Provider timeouts and malformed provider responses must fail closed; do not infer success from HTTP 200 without provider success code and required response fields.
4. Provider secrets, printer keys, access tokens, refresh tokens, signatures, and raw callback payloads must not be returned to Mini Program or written to ordinary public logs.
5. Token refresh errors should disable or mark only the affected Yilianyun printer/authorization, not Feieyun or Shangpeng runtime.
6. Provider-specific receipt content must be rendered per provider. Feieyun tags must not be reused for Shangpeng without a renderer test.

---

## 7. Implementation Planning Gate

Before any additional implementation, create or revise the unified implementation plan from this contract. The plan must include these phases:

1. **Contract tests first:** provider request signing, endpoint path/method, required fields, strict response parsing, provider error mapping.
2. **Provider manager boundary:** Feieyun baseline unchanged; providers selected by `printer_type`.
3. **Receipt renderer boundary:** provider-neutral receipt model or provider-specific renderers with fixtures.
4. **Shangpeng bind/print/poll:** remote add/delete with `business=1`, print, order status, printer info.
5. **Yilianyun authorization storage:** DB table, encryption, state validation, scan-code and authorization-code flows.
6. **Yilianyun print runtime:** token lookup, refresh owner, idempotent print, status polling, optional callback only after signature contract confirmation.
7. **Mini Program UX:** only after backend contract is stable; Yilianyun scan form must not assume an opaque one-string bind unless QR samples are confirmed.
8. **Review checkpoint:** after each phase, run focused tests and review for Feieyun regression, provider contract drift, data leakage, retry/idempotency, and reconciliation.

Mini Program UX acceptance for the first Yilianyun rollout:

- Provider selector is the first editable decision on the add-printer page.
- Feieyun and Shangpeng credentials remain SN/key fields.
- Yilianyun credentials use two fields: `机器码` (`machine_code`) and `终端密钥` (`msign`). Do not expose `qr_key` in the first-release merchant form unless QR sample parsing is confirmed.
- Raw scan/auth fields are never echoed after submission. Merchant-facing status uses masked machine code and stable labels only.
- Editing an existing printer must not expose or overwrite Yilianyun token state through the printer key field.

---

## 8. Open Verification Items

These are blockers before production enablement, not optional cleanup:

1. Capture real Yilianyun printer QR/self-test receipt scan payloads and document how they map to `machine_code`, `qr_key`, or `msign`.
2. Confirm Yilianyun current print-completion callback signature and ACK semantics for `oauth_finish` under the current docsify version. If not confirmed, first production release must use polling only.
3. Re-check Yilianyun API base URL in deployment config: current docs use `/v2` base paths.
4. Confirm Shangpeng endpoint-specific positive error codes and retryability with provider docs/support before adding fine-grained user-facing messages.
5. Confirm Shangpeng receipt markup against at least one physical printer or provider sandbox before enabling merchant printing.
6. Review existing in-progress code against this contract before continuing implementation; do not layer more fixes onto code that violates these decisions.
