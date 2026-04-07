---
name: "事故回灌请求模板"
description: "Use when drafting an incident follow-up or escaped-defect closure request that should convert findings into standards, prompts, workflows, tests, or runbooks. Trigger phrases: incident follow-up, postmortem closure, escaped defect closure, near miss hardening, convert incident to guardrails. 适用于把事故复盘结论落成工程约束与门禁。"
routing-hints: "事故回灌|线上事故|incident follow-up|postmortem closure|escaped defect closure|near miss hardening|guardrails|runbook 更新清单"
---
# General Incident Follow-Up Template

Use this template when the request is not just to fix a bug, but to convert an incident or major escaped defect into new default guardrails.

Read first:

- `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md`
- `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`
- `.github/standards/engineering/INCIDENT_FEEDBACK_LOOP.md`

Request:

- Summarize the incident or escaped defect in one short paragraph
- Classify the affected path as `G0`, `G1`, `G2`, or `G3`
- Explain which system layer failed first: design, implementation, review, tests, workflow gate, observability, or runbook
- Convert the incident into concrete follow-up changes across standards, instructions, prompts, workflows, tests, shared code, or runbooks
- Identify which follow-up items are mandatory before closure and which can be scheduled after
- If the incident touched release, rollback, or operations handling, say whether runbooks or domain docs must change
- If a repeated bug class is better prevented by automation, prefer workflow or test changes over reviewer memory

Required input:

- Incident or escaped defect summary: <details>
- Affected areas: <backend, web, weapp, mixed>
- Current fix status: <already fixed, partially fixed, not fixed>

Optional input:

- Known root cause: <details>
- Existing review findings or postmortem notes: <details>
- Relevant files or docs: <paths>

Expected output:

- Findings or gaps first
- Concrete artifact update plan with paths
- Closure checklist naming what must land before the incident can be considered closed
- Residual risk if some guardrails are still missing