---
name: "后端集成测试请求模板"
description: "Use when drafting a request for new or updated backend integration coverage. Trigger phrases: add integration test, 集成测试, cover cross-layer flow, guard regression end to end, validate persisted state, test worker or scheduler path. 适用于发起后端跨层集成测试任务。"
---
# Backend Integration Test Request Template

Use this template when asking for new or updated integration coverage in this workspace.

If this session is new, compacted, forked, or handed off, rerun routing from `.github/README.md`, reopen the matching instructions, and confirm the test scope before writing the request. Do not keep relying on stale context.

## Backend Integration Test

Target area: `locallife/integration/`

Request:

- Add or update an integration test for <workflow>
- Exercise a real cross-layer path instead of a unit-test-only seam
- Reuse existing journey-style or scenario-style patterns from nearby integration tests
- Validate business outcomes and persisted state, not just the immediate function return
- Tell me whether this change should run `make test-integration`, `make test-unit`, or both
- Run the smallest relevant validation command and report what was executed

Optional context:

- Affected workflow: <details>
- Related endpoint, job, or scheduler: <path>
- Required setup or fixture details: <details>

## Cross-Layer Regression Test

Request:

- Add a regression test that proves this change is wired end to end
- Check API, logic, persistence, worker, or scheduler boundaries as needed for the scenario
- Call out any setup cost or gaps that prevent full integration coverage

Optional context:

- Bug or regression being guarded: <details>
- Changed files or packages: <paths>
