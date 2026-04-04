# OpenChip Federation RFC

## Summary

OpenChip is no longer framed as a single hosted pet registry. It is now a reference implementation of an open, founder-independent, federated pet microchip registry protocol.

Phase 1 keeps OpenChip usable as a single-node deployment, but that node is explicitly modeled as one operator in a future network rather than the permanent center of the system.

## Why the previous design is insufficient

The previous implementation assumed:

- one operator
- one mutable database as canonical truth
- one local admin authority
- one JWT issuer and one deployment trust domain

That design is useful for an MVP, but it is not durable public infrastructure. It creates avoidable trust concentration:

- the founder or host can disappear
- the canonical registry can be lost or censored
- mirroring is hard because truth is embedded in mutable rows
- federation is awkward because there is no node identity or export contract

## Reference node definition

A reference node is a deployable OpenChip server that:

- runs the OpenChip API and web app for local operators
- maintains local projections for usability
- emits append-only signed-event-compatible history
- can export public snapshots and event streams
- can be mirrored or replaced by another operator later

In phase 1, the reference node is still a single deployment. The important shift is that it is no longer treated as the final authority model.

## Protocol-first design principles

1. Protocol first, app second.
2. Federation over centralization.
3. Append-only events over mutable rows as the canonical history.
4. Public/private data separation by default.
5. Exportability and mirroring are required features, not optional extras.
6. Governance and multi-operator readiness must be visible in the schema now.

## Public vs private boundaries

Public data:

- node metadata
- public registration metadata
- chip identifiers and manufacturer hints
- public pet profile metadata safe for publication
- lookup and dispute audit metadata that does not reveal owner contact data
- public snapshots and event stream exports

Private data:

- owner contact channels
- authenticated owner profile management
- mediated contact workflow execution
- operator-only dispute details when required

## Interoperability model

Future independent nodes should be able to interoperate through:

- published node metadata
- public snapshot export
- append-only event stream export
- published actor key metadata
- deterministic event hashing and future signature verification

AAHA and manufacturer registry integrations remain compatibility adapters. They are not the protocol core.

## Phase 1 centralized elements

Phase 1 still keeps some centralized behavior:

- one local PostgreSQL database per node
- one local JWT issuer for node-local sessions
- one local admin workflow
- one local outbound email sender
- one local dispute resolver

These remain acceptable in phase 1 because they are node-local operational concerns, not the long-term trust model for protocol truth.

## Phase 1 deliverables

- federation-ready schema foundations
- append-only event table with hash chaining
- node and organization records
- owner private-contact split
- public snapshot and event-stream exports
- no raw owner contact data in public or compatibility lookup responses

## Non-goals for phase 1

- full multi-node synchronization
- mandatory cryptographic signature enforcement
- consensus protocols
- blockchain dependency

Optional future anchoring of snapshot or event-log hashes may be useful, but it is explicitly outside the core design.
