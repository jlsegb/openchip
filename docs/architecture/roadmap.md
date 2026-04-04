# OpenChip Federation Roadmap

## Gap analysis

### What already helps

- Public lookup already favors mediated contact.
- The repo is small and modular enough to evolve into a reference node.
- Lookup, transfer, dispute, and export ideas already exist in the product.

### What conflicts with federation

- Mutable rows still act as the effective source of truth.
- One deployment is implicitly the sole authority.
- Owner contact data is mixed into business tables.
- Partner-facing responses have exposed owner contact details.
- There is no durable protocol export surface or node identity.

### What gets migrated

- `pets` becomes a projection, not the canonical truth.
- transfers become event-backed claim transitions.
- lookup, dispute, and owner anonymization become auditable event writes.
- public snapshots and event exports become first-class outputs.

### What gets deferred

- multi-node sync protocol
- mandatory event signatures
- cross-node trust and quorum rules
- rebuilding all projections purely from imported history

## Phases

### Phase 1: Reference node foundation

- add node, organization, key, claim, snapshot, and event tables
- add event writes in current single-node workflows
- add public snapshot and event-stream endpoints
- remove raw owner contact exposure from lookup responses

### Phase 2: Projection migration

- move more reads to `pet_profiles` and `registration_claims`
- stop treating `pets` as canonical
- remove legacy owner contact duplication from `owners`
- improve export determinism and import validation

### Phase 3: Federation handshake

- add signed event verification
- add snapshot import and replay
- add node discovery and compatibility contracts
- define conflict handling for competing claims across nodes

### Phase 4: Governance-ready network

- support multiple independent operators
- formalize key rotation and trust announcements
- add mirrored public registries and recovery drills

## Highest-leverage next steps

1. Make projections rebuildable from events and claims.
2. Introduce signed exports and verification using published actor keys.
3. Define import/replay rules for public snapshots and event streams between independent nodes.
