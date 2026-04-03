# OpenChip

OpenChip is a production-oriented, self-hostable open pet microchip registry monorepo. It is designed as a public-good alternative to proprietary registries and includes a Go API, PostgreSQL migrations, a Next.js owner/public portal, and containerized local development.

## Stack

- `api/`: Go 1.22, chi, pgx v5, golang-migrate, resend, JWT auth
- `web/`: Next.js 14 app router, TypeScript, Tailwind CSS
- `db/`: PostgreSQL 16 migrations
- `infra/`: Docker Compose for local development and self-hosting
- `docs/`: OpenAPI 3.0 spec

## Quickstart

1. Run `cp .env.example infra/.env`, then edit `infra/.env`.
2. Start the stack:

```bash
cd infra
docker compose up --build
```

3. Open:

- Web UI: [http://localhost:3000](http://localhost:3000)
- API: [http://localhost:8080](http://localhost:8080)
- Adminer: [http://localhost:8081](http://localhost:8081)

## Local development without Docker

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

## API reference

The hand-written OpenAPI document lives at [docs/openapi.yaml](/Users/jlsegb/Desktop/open_registry/openchip/docs/openapi.yaml).

## Self-hosting guide

- Provision PostgreSQL 16.
- Set `DATABASE_URL`, `JWT_SECRET`, `BASE_URL`, `FROM_EMAIL`, and `ADMIN_EMAIL`.
- `RESEND_API_KEY` is only required when `DISABLE_EMAIL` is not set to `true`.
- Add a `RESEND_API_KEY` to enable live transactional email. When `DISABLE_EMAIL=true`, email calls are stubbed for local or test environments.
- Configure `SHELTER_API_KEYS` as `key:Organization` pairs.
- Run the API and web containers behind TLS.
- Back up the PostgreSQL volume regularly.

## AAHA federation

OpenChip exposes `GET /api/v1/aaha/lookup/{chip_id}` as a provisional federation endpoint. Before production enrollment, validate the payload against the latest AAHA federation requirements and adapt the response shape if their contract has changed.

## Security and privacy notes

- Public lookup responses protect direct contact details and instead support owner notification requests.
- Lookup contact details are exposed only to authenticated shelter or vet API key callers.
- Magic links are single-use and expire after 15 minutes.
- JWTs are signed with HS256 and expire after 30 days.
- Owner deletion anonymizes the owner record while preserving registry data integrity.
- The implementation avoids logging full owner contact info.

## Contributing

1. Create a feature branch.
2. Keep API responses wrapped in `{ data, error }`.
3. Add or update migrations and OpenAPI docs when changing contracts.
4. Verify Go formatting, TypeScript checks, and container builds before opening a PR.

## Repository layout

```text
openchip/
  api/
  web/
  infra/
  db/
  docs/
  .env.example
```
