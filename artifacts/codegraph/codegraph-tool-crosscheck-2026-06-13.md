# CodeGraph Tool Cross-Check

Date: 2026-06-13
Tool: `@colbymchenry/codegraph@1.0.0`
Scope: existing merchant and rider codegraph artifacts under:

- `artifacts/codegraph/merchant-state-flows/*.edges.json`
- `artifacts/codegraph/rider-state-flows/*.edges.json`

Current workflow entrypoint: `artifacts/codegraph/README.md`.

## Setup

Installed the third-party CodeGraph CLI globally through npm:

```bash
npm install -g @colbymchenry/codegraph@1.0.0
codegraph telemetry off
codegraph init -i /home/sam/locallife
```

The project index was created at `.codegraph/codegraph.db`. It is local-only and was added to `.git/info/exclude`, not to the repository `.gitignore`.

Index status after initialization:

- Files: 2,167
- Nodes: 48,777
- Edges: 161,978
- DB size: 141.23 MB
- Backend: `node:sqlite`, WAL enabled
- Route nodes: 565

## Batch Cross-Check Summary

Compared the 23 existing merchant/rider `*.edges.json` files against CodeGraph's SQLite index.

Artifact totals:

- Slices: 23
- Artifact nodes: 1,294
- Artifact edges: 1,689

Node mapping against CodeGraph:

- 959 nodes mapped to containing CodeGraph symbols.
- 257 nodes point at files that CodeGraph intentionally does not index, mostly `*.sql` query sources and artifact/concept nodes.
- 70 nodes are conceptual or have no file path.
- 5 nodes point at indexed files but no matching symbol at the recorded line.
- 3 nodes matched by name rather than line containment.

Edge mapping:

- 1,029 artifact edges had both endpoints mappable to concrete CodeGraph nodes.
- 74 of those had a direct CodeGraph edge.

The low direct-edge overlap is expected: our artifacts model business-semantic edges such as table writes, status transitions, provider callbacks, scheduler effects, and frontend-to-backend HTTP contracts. CodeGraph mostly emits static `calls`, `references`, `imports`, `contains`, `implements`, and `extends`.

## Confirmed Useful Coverage

CodeGraph independently confirmed the basic structural shape for the audited areas:

- It indexed Go backend symbols, TypeScript Mini Program wrappers, Flutter/Dart files, and Gin route nodes.
- It found Gin route registration nodes and route-to-handler `references` edges in `locallife/api/server.go`.
- It connected interface methods to implementations in some paths, for example `Store::GrabOrderTx` to `SQLStore::GrabOrderTx`.
- It was useful for discovering duplicate/stale frontend API wrappers.

Examples:

- `grabOrder` search surfaced the rider Mini Program wrappers, backend handler `Server::grabOrder`, logic function `GrabDeliveryOrder`, store interface `GrabOrderTx`, and SQL transaction implementation `SQLStore::GrabOrderTx`.
- `submitMerchantApplication` search surfaced backend handler `Server::submitMerchantApplication`, generated sqlc method `Queries::SubmitMerchantApplication`, and multiple Mini Program wrapper copies.
- `codegraph explore "rider delivery grab order flow..."` correctly found the `GrabDeliveryOrder -> GrabOrderTx -> SQLStore::GrabOrderTx` path and included test blast-radius hints.
- `codegraph explore "merchant application onboarding submit flow..."` correctly found `Server::submitMerchantApplication`, `MerchantOnboardingReviewService::ProcessSubmittedApplication`, and review-related dependencies.

## Tool-Found Review Signals

The tool surfaced several concrete maintenance signals worth using:

1. Route line drift exists in merchant/rider artifact nodes.
   Many `locallife/api/server.go` references are still close, but shifted by 1-7 lines. This does not necessarily invalidate the slice semantics, but it makes line-level evidence less sharp.

2. CodeGraph route nodes do not preserve full Gin group prefixes.
   It can emit `POST /grab/:order_id` and link it to `Server::grabOrder`, but it does not reconstruct the full `/v1/delivery/grab/:order_id` path. Our human slice remains better for full contract paths.

3. Several artifact nodes are intentionally conceptual, but should stay clearly labeled.
   Examples include table nodes, provider nodes, middleware group nodes, route-group nodes, and SQL source nodes.

4. CodeGraph is good at detecting duplicate frontend wrappers.
   It found duplicated onboarding/delivery wrappers under merchant/operator/register/user_center shared copies. The existing rider README already tracks some of these as stale/dead copies; CodeGraph makes that easier to rediscover.

5. A few artifact line anchors should be refreshed when those slices are next touched.
   Examples from the batch scan:
   - `merchant-app-bind-and-device`: app-bind route anchors drifted.
   - `merchant-application-onboarding`: `route.ocrJobs` anchor drifted.
   - `merchant-device-display-config`: merchant device route anchors drifted.
   - `merchant-staff-and-group`: staff and group route anchors drifted.
   - `rider-application-onboarding`: rider application/media/OCR route-group anchors drifted.
   - `rider-workbench-status-location`: rider runtime/notification route-group anchors drifted.

## CodeGraph Blind Spots For This Repository

CodeGraph is helpful, but it is not a replacement for the LocalLife artifact method.

- It does not index `locallife/db/query/*.sql`, so it cannot directly validate query-source nodes or table-write semantics.
- It does not infer SQL table effects from generated sqlc code as business facts.
- It does not fully reconstruct Gin group prefixes or route middleware semantics.
- It does not understand provider callbacks, payment facts, outbox semantics, recovery schedulers, or status-machine invariants as domain concepts.
- Some `callers` queries miss real Go call sites. Example: `callers GrabDeliveryOrder` returned test callers but did not list the `Server::grabOrder` handler, while `explore` and search still found the surrounding path.
- It can over-broaden natural-language `explore` queries. The merchant onboarding query pulled unrelated `Review` symbols because of the word "review".

## Verdict

Yes, this helps me.

Best use:

- Fast symbol discovery.
- Finding duplicate wrappers and stale entry points.
- Route-to-handler confirmation.
- Interface-to-implementation hints.
- Blast-radius and test-surface hints before changing code.
- Mechanical drift checks for artifact line anchors.

Not enough for:

- Money movement correctness.
- Async recovery semantics.
- SQL/table mutation truth.
- Provider callback trust boundaries.
- Full frontend-to-backend contract paths when Gin groups are involved.

Recommended workflow:

1. Use CodeGraph first for broad discovery and candidate call paths.
2. Use existing LocalLife `*.slice.md` and `*.edges.json` artifacts for domain truth.
3. For high-risk G2/G3 flows, keep human-audited slices as the source of invariants.
4. Add a periodic lightweight drift check that compares artifact file/line anchors against CodeGraph's current symbols.
