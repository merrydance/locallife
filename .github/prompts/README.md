# Prompt Templates Index

This directory contains reusable prompt templates with normalized names.

## Naming Rule

- Use `<scope>-<intent>.prompt.md`
- Use `general-` for cross-workspace prompts
- Use `backend-`, `web-`, or `weapp-` when the prompt is area-specific

## Current Templates

- `general-implementation.prompt.md`
- `general-review.prompt.md`
- `backend-implementation.prompt.md`
- `backend-task-card-implementation.prompt.md`
- `backend-phase-batch-implementation.prompt.md`
- `backend-review-closure.prompt.md`
- `backend-integration-test.prompt.md`
- `backend-payment-runbook.prompt.md`
- `web-implementation.prompt.md`
- `web-review.prompt.md`
- `weapp-implementation.prompt.md`
- `weapp-review.prompt.md`

## Usage Rule

Prefer the most specific prompt template that matches the task. If the task is general and spans multiple areas, start with a `general-` template and add concrete context paths.