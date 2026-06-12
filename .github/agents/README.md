# Agents Index

This directory would contain custom agents only when a real tool boundary is justified instead of a prompt-only workflow. That is a routing preference, not a capability restriction.

## Active Agent Set

There are currently no repository-backed `.agent.md` files in this workspace.
Default to prompts and instructions unless a future workflow lands real agent assets. This default keeps the surface small; it does not reduce what the workflow can do.

## Routing Rule

- Default to prompts unless a task needs a real tool boundary, read-only mode, or explicit multi-agent orchestration. This chooses the right execution surface; it does not limit superpowers capabilities.
- If a new agent cannot justify a distinct boundary that a prompt cannot express, do not add it here.

## Maintenance Rule

- Keep this index aligned with the actual files in this directory.
- If a real `.agent.md` file is added later, update this file and `.github/README.md` together.
- Do not document hypothetical agents here before the corresponding files actually exist.
