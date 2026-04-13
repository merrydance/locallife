---
applyTo: "locallife/wechat/**"
---

# Backend WeChat Instructions

Apply these rules for files under `locallife/wechat/`.

## Read First

- `.github/standards/domains/wechat-payment/README.md`

Use `.github/standards/domains/wechat-payment/README.md` as the payment domain index. Open the operations runbook for payment operations and recovery work, and open the complaint or subsidy spec only when the active change crosses backend and client behavior in those flows.

If the change touches `/v3/merchant/media/upload` or the `UploadImage` path, also read `.github/standards/domains/wechat-payment/WECHAT_PAYMENT_MERCHANT_MEDIA_UPLOAD_CONTRACT_2026-04-13.md`.

## Historical Rollout References

- Consult the `historical/` execution plan referenced by `.github/standards/domains/wechat-payment/README.md` only when a task changes the rollout baseline, stage ownership, or historical migration assumptions.

## Role Of This Layer

- Keep this package responsible for WeChat-facing client integrations such as payment, ecommerce, shipping, complaint, bill, wxacode, and related security or content-check helpers.
- Use this package as an external integration boundary, not as a place to hide unrelated business workflows that should stay in logic, worker, or scheduler layers.

## Integration Conventions

- Preserve explicit client and interface boundaries so callers depend on stable integration interfaces instead of concrete implementation details.
- Keep request signing, transport details, and provider-specific error handling inside this integration boundary.
- Reuse existing payment, complaint, subsidy, and shipping client patterns instead of inventing a parallel client style for one endpoint.
- Keep business status transitions, ledger updates, and domain decisions outside this package unless they are strictly required to shape an external request or response.
- For `/v3/merchant/media/upload`, keep service-provider signing on `spMchID`, reject empty or fake image payloads locally, and preserve the current 2MB local limit unless the active domain standard is explicitly updated.

## Boundary Checks

- Changes to WeChat payment flows should be reflected in upstream logic, callbacks, worker processing, and audit records rather than stopping inside the client package.
- If config, credential, or app/mch routing assumptions change, update the corresponding standards and validation paths instead of relying on implicit defaults.
- Payment or refund behavior should stay auditable and traceable through persisted records rather than becoming an in-memory side effect.

## High-Risk WeChat Gates

- Treat callback signature verification, replay resistance, and idempotent repeated-callback handling as mandatory checks for payment, refund, complaint, and shipping callback paths.
- Do not silently fall back from service-provider config to direct-merchant config or from explicit app/mch routing to implicit defaults without an updated documented standard.
- Keep secrets, certificates, APIv3 keys, callback payloads, and decrypted provider fields out of unsafe logs, responses, and client-visible fields.
- If a payment or refund path can produce an ambiguous state, do not report success until the persisted record and downstream business transition agree on the final state.
- When changing payment-adjacent behavior, verify whether audit records, outbox or task enqueue paths, recovery schedulers, and operator-facing troubleshooting material remain consistent.

## Validation Defaults

- Prefer focused unit tests for client, payment, callback, and error-mapping behavior.
- If a change affects payment runbooks, service-provider config, or callbacks, update the linked domain standards alongside code.
- If a payment task touches execution plans or rollout-only docs, report whether those documents are still active or should move to historical/archive status.