---
description: Scaffolds a new Go microservice following the request-service structure. Use when starting inspect-service, notification-service, api-gateway, or any new service.
disable-model-invocation: true
argument-hint: <service-name> e.g. inspect-service
---

# Scaffold new service: $ARGUMENTS

## Steps

1. **Read the `request-service` repo** — it is the reference implementation. Study:
   - `cmd/server/main.go` — entry point and dependency wiring
   - `internal/domain/` — entities and errors
   - `internal/repository/` — interface + PostgreSQL implementation
   - `internal/service/` — business logic and state machine
   - `internal/handler/` — chi router, DTOs
   - `internal/middleware/` — JWT and role checks
   - `config/config.go` — ENV-based config
   - `migrations/` — first migration as a model

2. **Create `services/$ARGUMENTS/`** with the same layout:
   ```
   services/$ARGUMENTS/
     cmd/server/main.go
     internal/
       domain/         # service-specific entities and domain errors
       repository/     # interface + postgres.go
       service/        # business logic
       handler/        # HTTP handlers, request/response DTOs
       middleware/     # copy from request-service
       httputil/       # copy from request-service
     config/config.go
     migrations/000001_init.up.sql
     migrations/000001_init.down.sql
     go.mod
     Makefile
     README.md
   ```

3. **go.mod** — same module path convention and dependency versions as request-service

4. **config.go** — `os.Getenv` only. Default ports:
   - `inspect-service`: 8082
   - `notification-service`: 8083
   - `review-service`: 8084
   - `api-gateway`: 8080

5. **First migration** — create the tables for this service's domain

6. **README.md** — include: how to run, required ENV vars, list of endpoints

7. **docker-compose** — propose what to add to `docker-compose.yml` in the request-service repo root (new DB, new service container under the `app` profile)

Ask for clarification on domain entities before writing them if needed.
