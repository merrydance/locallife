# Agents Index

This directory contains the small set of custom agents that justify a real boundary instead of a prompt-only workflow.

## Active Agent Set

- `wechat-mini-program-audit.agent.md`
  Role: read-only Mini Program audit mode for diagnosis without edits or terminal work.
  Boundary reason: explicit read-only mode.

- `delivery-loop-orchestrator.agent.md`
  Role: user-facing closed delivery workflow that runs implement -> review -> fix -> review -> doc-sync until the task list is complete or blocked.
  Boundary reason: real multi-agent orchestration mode.

- `delivery-implementer.agent.md`
- `delivery-reviewer.agent.md`
- `delivery-doc-sync.agent.md`
  Role: internal support agents used by `delivery-loop-orchestrator.agent.md`.
  Boundary reason: each one has a narrower tool and responsibility boundary inside the delivery loop.
  Usage rule: keep these as internal workflow helpers, not as separate user-facing agent modes unless the workflow model changes.

## Routing Rule

- Default to prompts unless a task needs a real tool boundary, read-only mode, or explicit multi-agent orchestration.
- Treat the delivery helper trio as implementation details of the delivery loop, not as standalone routing surfaces.
- If a new agent cannot justify a distinct boundary that a prompt cannot express, do not add it here.

## Maintenance Rule

- Keep this index aligned with the actual files in this directory.
- If a support agent is removed, merged, or made user-facing, update this file and `.github/README.md` together.
- If a workflow uses helper agents, document the parent workflow first and list helpers as internal support rather than as equal top-level modes.