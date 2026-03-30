---
name: "业务流程 Mermaid 生成模板"
description: "Use when turning rough business steps into a reviewable Mermaid flowchart. Trigger phrases: draw business flow, generate Mermaid process, add decision branches, model approval flow, convert notes to Mermaid. 适用于把零散业务描述整理为结构完整、带异常分支的 Mermaid 流程图。"
---
# Business Flow Mermaid Template

Use this template when asking for a Mermaid flowchart from a business process description.

## Flowchart Request

Request:

- Convert <business process> into runnable Mermaid flowchart code
- Include a clear start node, end node, decisions, exception paths, and explicit branch labels
- Distinguish user-provided logic from branches inferred for completeness
- Ask for confirmation if key business rules are still ambiguous

Required context:

- Actors involved: <list>
- Trigger condition: <details>
- Main happy path: <steps>

Optional context:

- Approval, timeout, cancellation, rollback, or callback branches: <details>
- Terms that must remain in Chinese or English: <details>
- Preferred diagram split or subgraph boundaries: <details>

Output expectations:

- Return Mermaid code first
- Then list inferred branches added beyond the original description
- Then ask whether those inferred branches match the real business flow