# Contributing AI Rules

Use this process when updating standards, instructions, or prompt templates.

## 1. Decide The Source Layer

- Update `.github/standards/` when the change is a project-owned engineering or domain standard.
- Update `.github/instructions/` when the change affects high-frequency implementation, review, regeneration, or validation behavior.
- Update `.github/prompts/` when the change should influence how users ask for common task types.

## 2. Mirror Operationally Important Changes

If a standard changes in `.github/standards/`, update the matching instruction file when the change affects:

- architecture boundaries
- API contract semantics
- generation commands
- validation commands
- review expectations
- high-risk domain workflows

## 3. Keep Naming Normalized

- Instructions: `<scope>-<area>.instructions.md`
- Prompts: `<scope>-<intent>.prompt.md`
- Keep new `applyTo` patterns as narrow as practical

## 4. Prefer Updating Existing Files

- Extend an existing scope-specific instruction before creating a broader duplicate.
- Add a new prompt template only when the task pattern is frequent and distinct.
- Link to canonical standards instead of copying long domain detail into instructions.

## 5. Keep Stable Guidance Separate From Rollout Material

- Prefer long-lived standards, runbooks, and security constraints in `Read First` sections.
- Treat execution plans, cutover checklists, rollout playbooks, and migration diaries as conditional references unless they are still the active operating baseline.
- If a rollout document has completed its purpose, move it toward archive or historical status instead of leaving it in the default hot path.
- Place completed rollout material under the matching domain `historical/` directory and update any active entrypoints that still reference the old location.

## 6. Completion Expectations For Customization Changes

- If you change instructions, verify that the new wording creates a clearer gate or routing behavior instead of only adding more prose.
- If you change prompts, keep them task-shaped; do not move long-lived engineering standards into prompt bodies.
- If a change affects routing or documentation lifecycle, update `.github/README.md`, `.github/prompts/README.md`, or both when needed so the new behavior is discoverable.

## 7. Validate Before Hand-Off

Before considering a customization change complete:

1. Confirm the canonical standard path is correct.
2. Confirm instructions point to the new canonical path.
3. Confirm prompts use normalized naming.
4. Confirm no scattered duplicate or temporary compatibility path has been reintroduced unless the migration explicitly requires one.
5. Update `.github/NORMS_AUDIT.md` if coverage status changes.
6. If rollout or execution-plan docs were touched, state whether they remain active guidance or should be archived or moved out of the default reading path.