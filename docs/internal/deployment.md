## Deployment & Runtime

Guidance for running the s3 server in development and CI environments.

### Local development

- Use the provided `Dockerfile` and `docker-compose.yml` for a reproducible environment.
- Configure environment variables: `DATABASE_URL`, `STORAGE_ROOT`, `PORT`, `LOG_LEVEL`.

### Database migrations

- Migrations are in `database/migrations`; run them with the project's migrator tool or `migrate` binary before starting the service.

### Running the service

```bash
# build
go build ./cmd/api

# run (example)
DATABASE_URL=postgres://user:pass@localhost:5432/s3db STORAGE_ROOT=/data ./api
```

### Monitoring & logs

- Expose Prometheus metrics on `/metrics`.
- Use structured JSON logs to make log ingestion easier.
