# OpenChip

OpenChip is a reference implementation of an open, federated pet microchip registry protocol.

It is not just a centralized pet registry app. The goal is to provide a durable, founder-independent recovery layer that can coexist with manufacturer registries and AAHA-style lookup systems while remaining useful as a single-node deployment today.

## What OpenChip Is

- A reference node for a future federated registry network.
- A practical single-node deployment for owners, shelters, and operators.
- A protocol-first system built around append-only event history, public/private data separation, and exportability.

## What OpenChip Is Not

- Not a replacement for chip manufacturers.
- Not a design that assumes one permanent operator or one permanent hosted service.
- Not a public directory of owner email, phone, or address data.

## Current Architecture

OpenChip is in a migration phase from a centralized CRUD-style registry toward a federation-ready reference node.

Current foundations in the repo:

- `ownership_events` provide append-only event history for critical state changes.
- `owner_contacts` separate private contact channels from public registry metadata.
- `organizations` and `nodes` model future independent operators.
- public snapshot and event-stream exports provide a path toward mirroring and survivability.
- legacy tables still exist as local projections to keep the current app usable.

Architecture docs:

- [federation.md](/Users/jlsegb/Desktop/openchip/docs/architecture/federation.md)
- [data-model.md](/Users/jlsegb/Desktop/openchip/docs/architecture/data-model.md)
- [threat-model.md](/Users/jlsegb/Desktop/openchip/docs/architecture/threat-model.md)
- [roadmap.md](/Users/jlsegb/Desktop/openchip/docs/architecture/roadmap.md)

## Stack

- `api/`: Go API, local auth, event/projection writes, export endpoints
- `web/`: Next.js owner and public portal
- `db/`: PostgreSQL schema, migrations, and seeds
- `infra/`: Docker Compose for local development
- `docs/`: OpenAPI and architecture RFCs

## Privacy Model

- Public lookup must never expose raw owner PII.
- Owner contact is mediated by default.
- Trusted organization workflows should also move toward mediated contact by default.
- Any future direct-contact exception must be explicit, auditable, role-scoped, and easy to disable by policy.
- Shelter lookup responses are minimized to pet metadata plus mediated-contact semantics; they do not return raw owner contact data or owner names by default.

## Quickstart

1. Create the local env file:

```bash
cp .env.example infra/.env
```

2. Start the stack:

```bash
cd infra
docker-compose up --build
```

3. Open:

- Web UI: [http://localhost:3000](http://localhost:3000)
- API: [http://localhost:8080](http://localhost:8080)
- Adminer: [http://localhost:8081](http://localhost:8081)

4. Check health:

```bash
curl http://localhost:8080/health
```

## Local Development Without Docker

### API

```bash
cd api
go mod tidy
go run ./cmd/server
```

### Web

```bash
cd web
npm install
npm run dev
```

The API expects PostgreSQL 16 and runs migrations automatically on startup.

## Federation-Ready Endpoints

- `GET /.well-known/openchip-node`
- `GET /api/v1/federation/snapshot`
- `GET /api/v1/federation/events`

These are phase 1 reference-node surfaces, not a full federation implementation yet.

## Compatibility Endpoints

- `GET /api/v1/aaha/lookup/{chip_id}` remains a compatibility layer.
- AAHA-style interoperability is not the protocol core.

## API Reference

The OpenAPI document lives at [docs/openapi.yaml](/Users/jlsegb/Desktop/openchip/docs/openapi.yaml).

## Self-Hosting Notes

- Provision PostgreSQL 16.
- Set `DATABASE_URL`, `JWT_SECRET`, `BASE_URL`, `FROM_EMAIL`, and `ADMIN_EMAIL`.
- `RESEND_API_KEY` is only required when `DISABLE_EMAIL` is not `true`.
- Configure `SHELTER_API_KEYS` as `key:Organization` pairs if needed.
- Run the API and web behind TLS in non-local environments.
- Back up the database and exported snapshots.

## Security Notes

- Magic links are single-use and expire after 15 minutes.
- Magic-link completion is a deliberate `POST` flow rather than a side-effecting `GET`.
- Browser sessions use an `HttpOnly` cookie; the web app does not persist auth tokens in `localStorage`.
- JWTs are node-local session tokens, not federation truth.
- Transfer tokens are hashed at rest before storage.
- Default owner exports redact lookup requester IP and user-agent metadata.
- Owner deletion anonymizes contact details while preserving auditability.
- Critical state changes should be represented as append-only events.
- Avoid logging full owner contact data.

## Contributing

1. Inspect the existing implementation before making major changes.
2. Update [docs/architecture/](/Users/jlsegb/Desktop/openchip/docs/architecture) before changing schema or API behavior.
3. Update [docs/openapi.yaml](/Users/jlsegb/Desktop/openchip/docs/openapi.yaml) when endpoints change.
4. Run the relevant build/test commands before marking work complete.
5. Summarize what changed, what remains centralized, and what federation path the work enables next.

## Repository Layout

```text
openchip/
  api/
  web/
  infra/
  db/
  docs/
  .env.example
  AGENTS.md
```
