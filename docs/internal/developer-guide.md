## Developer Guide

Quick start for contributors and engineers working on the s3 server.

### Repository layout

- `cmd/api`: entrypoint for the HTTP server
- `internal/application`: business logic and use-cases
- `internal/domain`: domain models and interfaces
- `internal/infrastructure`: DB and storage adapters
- `database`: migrations and DB utilities

### Setup

1. Install Go (>=1.20)
2. Start Postgres and run migrations
3. Set `DATABASE_URL` and `STORAGE_ROOT`
4. Build and run with `go run ./cmd/api` or use Docker Compose

### Testing

- Unit tests live under `tests/unit` and `internal/*` packages.
- Use `go test ./...` for the full suite; set `-run` to target specific packages.

### Contributing

- Follow existing patterns for error handling and logging.
- Add migrations when changing DB schema.
