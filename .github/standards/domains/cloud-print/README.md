# Cloud Print Provider Contract

This domain standard records provider-facing contract details for LocalLife cloud print integrations.

## Feieyun Print Result Callback

Source: Feieyun China Open Platform documentation `https://help.feieyun.com/#/home/doc/zh;nav=1-8`, section `打印状态回调`.

China station facts verified from the current documentation bundle:

- Admin URL: `https://admin.feieyun.com/`.
- API base URL: `https://api.feieyun.cn/Api/Open/`.
- Print callback public key download path: `https://help.feieyun.com/assets/public.key`.

### Print Request

Capability: receipt print task submission.

Provider operation: `Open_printMsg`.

Local boundary: `locallife/cloudprint/feieyun.go`.

Relevant fields:

| Field | Type | Required | Local source | Notes |
| --- | --- | --- | --- | --- |
| `sn` | string | yes | `cloudprint.PrintInput.SN` | Feieyun printer SN. |
| `content` | string | yes | `cloudprint.PrintInput.Content` | Receipt content. |
| `times` | int | no | `cloudprint.PrintInput.Copies` | Defaults to `1`. |
| `backurl` | string | conditional | `FEIEYUN_PRINT_CALLBACK_URL` | Required when LocalLife expects Feieyun print result callbacks. |
| `sig` | string | yes | `SHA1(user + FEIEYUN_UKEY + stime)` | Request signature sent to Feieyun; distinct from callback `sign`. |

The synchronous `Open_printMsg` response returns a Feieyun order ID. In callback-enabled mode this means Feieyun accepted the print job; it is not treated as terminal printed success.

### Callback

Route: `POST /v1/webhooks/feieyun/print-result`.

Content type: `application/x-www-form-urlencoded`.

Required fields:

| Field | Type | Required | Meaning | Local handling |
| --- | --- | --- | --- | --- |
| `orderId` | string | yes | Feieyun order ID returned by `Open_printMsg` | Matches `print_logs.vendor_order_id`. |
| `status` | int | yes | Print status | Only `1` is documented as `打印成功`. |
| `stime` | int | yes | Status change UNIX timestamp in seconds | Included in signature verification. |
| `sign` | string | yes | Base64 RSA signature | Verified before any state update. |

Signature verification:

1. Use all POST form fields except `sign`.
2. Drop fields whose value is empty.
3. Sort keys by ASCII ascending order.
4. Join as `key=value&key=value`.
5. Base64-decode `sign`.
6. Verify the joined string with Feieyun public key and `SHA256WithRSA`.

Runtime config:

- `FEIEYUN_PRINT_CALLBACK_URL`: URL passed to Feieyun as `backurl`.
- `FEIEYUN_CALLBACK_PUBLIC_KEY_PEM`: Feieyun China station callback public key PEM from `/assets/public.key`.
- `FEIEYUN_CALLBACK_PUBLIC_KEY_PATH`: file path to the same Feieyun callback public key PEM.

Callback mode is enabled only when both `FEIEYUN_PRINT_CALLBACK_URL` and a Feieyun callback public key are configured. This prevents LocalLife from submitting `backurl` and then being unable to verify the resulting callback.

`FEIEYUN_UKEY` is only for Feieyun API request signing (`sig=SHA1(user+UKEY+stime)`). It is not the callback verification public key. The callback `sign` field is verified with Feieyun's public key and `SHA256WithRSA`; that public key is not a TLS certificate and is not the domain verification file.

### ACK And State Semantics

Feieyun requires the receiver to return plain text `SUCCESS` within 5 seconds; otherwise Feieyun retries.

LocalLife behavior:

- Invalid signature, malformed required fields, or missing public key: return non-`SUCCESS` and do not update state.
- `status=1`: mark the matching `print_logs` row as `success`.
- Unknown `status`: log and return `SUCCESS`, leaving local status unchanged.
- Unknown `orderId`: return non-`SUCCESS` so Feieyun retries. This avoids losing a callback that arrives before the worker has persisted `vendor_order_id`.

The provider documentation currently only states `status=1` for successful print. Do not invent failed status enums in callback handling without provider-confirmed documentation or samples.

### Feieyun Callback URL Verification File

Feieyun's console can require a downloaded verification file, for example `feieyun_verify_lPcw9LunHs8pqSvB.txt`, to be reachable under the configured domain or path.

That file is deployment/static-site configuration and is not served by the Go backend. Place it in the production site root or configure Nginx/gateway static serving so Feieyun can fetch it before enabling the callback URL.
