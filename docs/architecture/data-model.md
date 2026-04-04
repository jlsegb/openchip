# OpenChip Data Model

## Intent

The old model treated mutable `pets` rows as the effective source of truth. The new model treats append-only events as the durable history and uses mutable tables as projections for usability.

## Core entities

### Organizations

Represents operator groups that may run one or more nodes.

### Nodes

Represents a deployable OpenChip instance with metadata, export capability, and future federation identity.

### Owners

Represents a local person/account identity used for node-local workflows.

### Owner contacts

Stores private contact channels separately from public registry metadata. This is the private boundary for email and phone.

### Chips

Normalized chip identifiers and manufacturer hints. Chips remain useful as a public coordination key across registries.

### Pet profiles

Public-safe animal metadata associated with a chip and used in public snapshots and mediated recovery workflows.

### Registration claims

Represents a claimant asserting an ownership or caretaker relationship to a chip/pet profile pair. Competing claims are modeled explicitly instead of being hidden as duplicate mutable pet rows.

### Ownership events

Append-only history of important state transitions:

- registration claim created
- profile updated
- transfer initiated
- transfer approved or rejected
- dispute opened or resolved
- lookup recorded
- owner anonymized

### Disputes

Operator workflow records for contested claims. In the long term, disputes map to event history and claim states rather than ad hoc mutable corrections.

### Public snapshots and export batches

Generated artifacts for mirroring, backup, portability, and future federation sync.

### Actor keys

Published metadata about future signing identities for nodes and other actors.

## Canonical history vs projections

Canonical history:

- `ownership_events`

Current projections used by phase 1 application flows:

- `owners`
- `owner_contacts`
- `chips`
- `pets`
- `pet_profiles`
- `registration_claims`
- `lookups`
- `transfers`
- `disputes`

This means phase 1 is hybrid:

- event log is the durability and federation foundation
- existing CRUD-style tables still power current user-facing reads

## Public/private split

Public-facing data should come from:

- `chips`
- `pet_profiles`
- selected `registration_claims`
- `ownership_events` metadata
- `public_snapshots`
- `nodes`

Private contact data should come from:

- `owner_contacts`

Legacy duplication remains in `owners.email` and `owners.phone` for compatibility during migration. That duplication should be removed once auth and projections are fully migrated.

## Event table contract

Each event contains:

- `id`
- `aggregate_type`
- `aggregate_id`
- `event_type`
- `payload_json`
- `actor_type`
- `actor_id`
- `event_hash`
- `previous_event_hash`
- `signature`
- `created_at`

`event_hash` is a deterministic hash of the event envelope plus the previous hash. This creates a per-aggregate tamper-evident chain.

## Snapshot/export model

Public snapshot:

- public-safe registration metadata
- node metadata
- generated timestamp
- hash of the exported payload

Event stream export:

- ordered append-only events
- hash-linked history
- node metadata
- optional time filtering

Owner export:

- private contact section
- public section
- local projections
- lookup history

## Migration posture

Short term:

- write events alongside existing local projections
- keep current UX working

Medium term:

- derive more current state from claims plus events
- reduce trust in mutable tables

Long term:

- projections become rebuildable from imported events and snapshots
