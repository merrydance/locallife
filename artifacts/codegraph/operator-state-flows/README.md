# Operator State Flows Codegraph

Status: in progress, first slice created 2026-06-14
Risk class: G2/G3 mix - operator regional authority, dispatch alerts, merchant/rider operational visibility, food-safety handling, recovery disputes, rules, finance, Baofu settlement/withdrawal
Scope: WeChat Mini Program operator pages -> operator backend routes -> logic/transactions -> SQL tables -> async notifications/workers/recovery paths -> dead/orphan paths
Boundary note: this directory judges operator-side closure only: what an operator can see, decide, submit, and recover. Merchant, rider, platform, customer, and provider-side source flows are referenced at their boundary but remain owned by their own role/domain slices.

Before creating or refreshing an operator slice, use the workflow in
`artifacts/codegraph/README.md`: CodeGraph may be used for discovery and line
anchor drift checks, but the slice and edge artifacts are the durable
LocalLife-aware source of truth after review.

## Slice Map

- `operator-dashboard-analytics-notifications.slice.md`: operator dashboard, analytics page, managed-region picker, realtime/trend/ranking reads, finance summary card, notification center, read/read-all/detail, and dispatch-hall handoff.

Planned operator slices:

- `operator-dispatch-hall`: pending dispatch monitor, dispatch timeout alert source, region scoped pending-delivery visibility, and rider/merchant action handoff.
- `operator-region-rules-and-expansion`: managed regions, region config, peak hours, delivery-fee config, rules, rule hits, and region expansion applications.
- `operator-merchant-management`: merchant list/detail/summary, capabilities, merchant stats, and related recovery boundaries.
- `operator-rider-management`: rider list/detail/summary/stats, ranking, and rule-driven suspension visibility.
- `operator-safety-and-recovery`: food-safety cases, investigation/resolution, recovery disputes, recoveries, behavior actions, and compensation/release handoff.
- `operator-finance-and-baofu-withdrawal`: finance overview, commission bills, Baofu settlement account, Baofu income withdrawal, provider callbacks, and recovery.
- `flow-variant-index.md`: compact branch/dead-code index across all operator-side slices.
- `operator-related-completeness-audit.md`: explicit verdict for operator-side closure versus all operator-related cross-role/background touchpoints.

Each `*.edges.json` uses the same compact edge schema as the existing merchant and rider slices: only core page/API/logic/transaction/table/provider edges are modeled, while branch detail stays in the Markdown slices.

## Mini Program Entrypoints

The operator-facing Mini Program package is declared in `weapp/miniprogram/app.json:32` through `weapp/miniprogram/app.json:60`:

- `pages/operator/analytics/index`
- `pages/operator/dashboard/index`
- `pages/operator/dispatch-hall/index`
- `pages/operator/notifications/index`
- `pages/operator/notifications/detail/index`
- `pages/operator/merchants/index`
- `pages/operator/merchants/detail/index`
- `pages/operator/riders/index`
- `pages/operator/riders/detail/index`
- `pages/operator/rules/index`
- `pages/operator/region/index`
- `pages/operator/region/config`
- `pages/operator/timeslot/index`
- `pages/operator/delivery-fee/index`
- `pages/operator/safety/report/index`
- `pages/operator/safety/detail/index`
- `pages/operator/finance/withdraw/index`
- `pages/operator/finance/bills/index`
- `pages/operator/finance/withdrawals/index`
- `pages/operator/finance/withdrawals/create/index`
- `pages/operator/finance/withdrawals/detail/index`
- `pages/operator/finance/settlement-account/index`
- `pages/operator/finance/settlement-account/submit/index`
- `pages/operator/region-expansion/index`

## Backend Route Surface

Operator routes under `/v1/operator` are registered at `locallife/api/server.go:1350` through `locallife/api/server.go:1409`.

Operator finance, Baofu, notification, and rules proxy routes under `/v1/operators/me` are registered at `locallife/api/server.go:1411` through `locallife/api/server.go:1440`.

Both route groups use `CasbinRoleMiddleware(RoleOperator)` and `LoadOperatorMiddleware`.

## Validation

This directory is documentation/artifact-only. Validate edge JSON with:

```bash
jq empty artifacts/codegraph/operator-state-flows/*.edges.json
```
