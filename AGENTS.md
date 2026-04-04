# AGENTS.md

## Project overview
OpenChip is a reference implementation of an open, federated pet microchip registry protocol.
It is not just a centralized pet registry app.
The codebase must evolve toward founder-independent public infrastructure.

## Non-negotiable architecture rules
- Protocol-first, app-second.
- Do not design new features that assume a single permanent operator.
- Critical state changes must produce append-only events.
- Separate public registry metadata from private owner contact data.
- Public lookup must never expose raw owner email, phone, or address.
- Model ownership conflicts as claims/disputes, not duplicate mutable records.
- Keep import/export, snapshots, and future mirroring in mind for any schema change.

## Before making major code changes
- First inspect the existing implementation.
- Then write a gap analysis and phased migration plan.
- Only then edit schema or API behavior.

## Required workflow
- Before changing schema or APIs, update docs in `docs/architecture/`.
- When runtime behavior changes, run the project's build/test stack.
- When adding or changing endpoints, update OpenAPI/docs.
- When work is ready, summarize:
  - what changed
  - what remains centralized
  - what federation path this enables next

## Build/test
- Use the existing repo commands.
- Do not mark work complete if code changes are unverified.

## Security
- Never expose owner PII in public lookup responses.
- Trusted organization workflows should default to mediated contact; any direct-contact exception must be explicit, auditable, role-scoped, and easy to disable by policy.
- Never log full owner contact data.
- Preserve auditability for transfers, disputes, and lookup-triggered notifications.

## Scope discipline
- Do not add speculative features unless they directly support federation, trust minimization, survivability, or practical shelter usability.
