---
applyTo: "locallife/media/**"
---

# Backend Media Instructions

Apply these rules for files under `locallife/media/`.

## Read First

- `.github/standards/domains/media/MEDIA_BACKEND_MODULE_DESIGN_2026-03-18.md`
- `.github/standards/domains/media/MEDIA_API_CONTRACT_DESIGN_2026-03-18.md`
- `.github/standards/domains/media/MEDIA_DATABASE_SCHEMA_DESIGN_2026-03-18.md`
- `.github/standards/domains/media/MEDIA_TEST_AND_ACCEPTANCE_CHECKLIST_2026-03-18.md`

## Role Of This Layer

- Keep this package focused on media-domain infrastructure and policy concerns such as storage abstractions, object-key policy, visibility rules, and upload-session lifecycle helpers.
- Do not move HTTP concerns, request binding, or response shaping into this package.

## Media Conventions

- Preserve explicit dependency injection for storage and store-backed collaborators.
- Reuse existing policy and registry patterns instead of scattering bucket or object-key logic into unrelated packages.
- Keep storage providers behind interfaces so callers do not depend on vendor-specific SDK details.
- Treat upload session creation and completion flows as idempotent where the existing code already does so.

## Boundary Checks

- Visibility, moderation, and source-client semantics should stay consistent with the media docs instead of being redefined ad hoc in callers.
- New media behaviors should propagate through persistence, storage, and calling layers rather than stopping at one helper.
- Avoid reintroducing legacy local-upload assumptions when the module is designed around media assets and storage abstractions.

## Validation Defaults

- Prefer focused package tests for policy, storage, resolver, and registry behavior.
- If media changes also touch SQL or API contracts, run the corresponding regeneration or validation commands in addition to package tests.