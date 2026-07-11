# Appraisal CRM — inspect-service

Commercial project built for a real client. Code goes to production — treat it accordingly.

**This repo is `inspect-service`.** `request-service` is the reference
implementation — follow its layout and conventions. This file carries the
project-wide charter (each service is a separate repo) plus this service's specifics.

## What it does

CRM for a property appraisal company (apartments, houses, land, vehicles, commercial real estate).
Digitizes the full cycle: client submits request → inspector visits the property → appraiser evaluates → client receives report.

`inspect-service` owns the on-site inspection step: it creates an inspection when
a request is scheduled, lets the assigned inspector fill in property data and
photos, and emits `inspect.completed` when the visit is done.

## Roles

| Role          | What they do in the system                                                         |
|---------------|------------------------------------------------------------------------------------|
| Client        | Submits a request, tracks status, downloads the final report                       |
| Appraiser     | Accepts requests, assigns an inspector, conducts appraisal, sends the report       |
| Inspector     | Receives field assignments, uploads photos and property data                       |
| Administrator | Manages users, monitors the system                                                 |

## Request lifecycle (strictly linear — no going back)

```
New → In Progress → Inspection Scheduled → Inspection Completed → Appraisal → Report Sent → Closed
```

The request state machine lives in request-service. inspect-service reacts to the
`inspection_scheduled` transition and, on completion, emits the event that lets
request-service advance to `inspection_completed`.

## Stack

| Layer              | Technology                               |
|--------------------|------------------------------------------|
| Backend services   | Go (chi, pgx, golang-migrate)            |
| Database per svc   | PostgreSQL (Database-per-Service pattern)|
| Auth               | Keycloak 26 (OAuth2/OIDC)               |
| Events             | Apache Kafka                             |
| Cache / Dedup      | Redis                                    |
| Object storage     | S3 Yandex Cloud                          |
| Frontend (4 SPAs)  | React + TypeScript                       |
| Architecture docs  | Structurizr DSL (C4)                     |

## Inspect Service (this repo)

- **Consumer + producer.** Consumes `request.status_changed` (topic
  `request.events`); produces `inspect.completed` (topic `inspect.events`) via a
  transactional **outbox**.
- **Aggregate:** `inspections` (one per request, `request_id` UNIQUE) +
  `inspection_photos` (S3 object keys) + `inspection_statuses` lookup.
- **Inspection state machine:** `scheduled → completed` only. Completion is the
  single event-producing transition (CAS on `status = 'scheduled'`).
- **Property data:** flexible `property_data` JSONB on the inspection.
- **Photos:** binaries live in S3; only the object key is stored. Uploads go
  through a presigned URL. `internal/storage` is a stub until the S3 SDK is wired.
- **Consumer idempotency:** dedup by `event_id` in **Redis** (`SET NX EX`) — no
  inbox table. Creation is additionally idempotent per request via the
  `request_id` UNIQUE constraint (`INSERT ... ON CONFLICT DO NOTHING`).
- **DB:** `inspect_db`. **Default port:** `8082`.

### How an inspection is born

`inspect-service` does not expose a "create inspection" endpoint. When
request-service moves a request into `inspection_scheduled`, the resulting
`request.status_changed` event creates the inspection row (status `scheduled`,
`inspector_id` NULL). An appraiser/admin then assigns the inspector via
`PATCH /inspections/{id}`.

> Note: `request.status_changed` currently carries only `old_status`/`new_status`.
> The inspection is created unassigned; assigning `inspector_id` is a separate
> appraiser/admin action. If request-service later includes the inspector in the
> event payload, wire it through the consumer (additive, bump event `version`).

## Go module path

Not a monorepo. Every service is a separate repository under the `appraisal-crm`
GitHub organization:
```
github.com/appraisal-crm/inspect-service
github.com/appraisal-crm/<name>-service   # pattern for services
```

## Go service structure (request-service is the reference layout)

```
inspect-service/
  cmd/server/          # entry point, wire DI
  internal/
    domain/            # entities, domain errors (errors.go), events
    repository/        # interface + PostgreSQL implementation
    service/           # business logic, inspection state machine
    handler/           # HTTP (chi router), DTOs
    middleware/        # JWT auth, role-based access
    httputil/          # shared response helpers
    outbox/            # producer + relay (inspect.completed)
    kafka/             # consumer group (request.status_changed)
    dedup/             # Redis event_id dedup
    storage/           # S3 photo storage (stub)
  config/              # ENV config (os.Getenv only)
  migrations/          # SQL files (golang-migrate up/down)
  api/                 # Swagger (swaggo/swag, generated, gitignored)
```

## Code rules

- No magic frameworks — chi, pgx, playground/validator only
- Config via `os.Getenv` only — no viper, no cobra
- Migration files: `000001_<description>.up.sql` / `.down.sql` — sequential, always both up and down
- JWT validation via Keycloak JWKS: `MicahParks/keyfunc` + `JWKS_URL` env var
- Domain errors in `domain/errors.go`; map to HTTP status codes in the handler layer only
- Each Kafka event is a distinct type in `domain/events.go`
- Publish events via the transactional outbox — write to `outbox` in the same tx as the state change; never publish from a handler
- Kafka consumers MUST be idempotent — dedup by `event_id` (Redis `SET NX EX`) before processing
- Optimistic locking for concurrent mutations (CAS on `updated_at` / `status`)
- Swagger annotations required for all public endpoints
- Unit tests for business logic in `service/`

## Kafka events

**One topic per producing service** — events are told apart by `event_type`, NOT by topic.

| event_type               | Topic            | Producer        | This service |
|--------------------------|------------------|-----------------|--------------|
| `request.status_changed` | `request.events` | request-service | consumes     |
| `inspect.completed`      | `inspect.events` | inspect-service | produces     |

**Event conventions:**
- **Message key** = aggregate id (`request_id`) → per-request ordering within a partition
- **Format** = JSON envelope: `event_id`, `event_type`, `version`, `occurred_at`, `request_id`, `data{}`
- **Delivery** = at-least-once → consumers MUST be idempotent (dedup by `event_id`)
- **Schema evolution** = additive only; bump `version` for breaking changes
- **Broker** = KRaft mode (no Zookeeper)

## Commands

```bash
# Start shared infra (Kafka/Keycloak) once, then this service's data infra
docker compose -f ../infra/docker-compose.yml up -d
docker compose up -d

# Run the service (needs DATABASE_URL + REDIS_ADDR — see .env.example)
make run          # or: go run cmd/server/main.go

# Tests
make test         # go test ./...

# Migrations (DB_URL defaults to local inspect_db)
make migrate-up
make migrate-down
```

## Hard rules — do not break

- No synchronous cross-service calls between business services — events via Kafka only
- No cross-database JOINs between services
- Do not switch config from `os.Getenv` to viper without explicit agreement
- Never modify already-applied migrations — new additive migrations only
- Never publish to Kafka straight from a handler/service — go through the outbox

## Workflow

- Tasks tracked in Jira, project **ACRM** (mdrslv.atlassian.net)
- Branch from `dev`: `feature/<scope>` / `fix/<scope>`; PR into `dev`
- `main` is the release branch — updated by merging `dev` → `main`; never PR features directly into `main`
- Conventional commits with the Jira key: `feat(inspect): ... (ACRM-XX)`
