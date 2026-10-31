# JKB

JKB is a compact single-board Kanban service: a Go binary with an embedded Vue
frontend, PostgreSQL, local attachment storage, personal API tokens and a
stateless remote MCP endpoint.

## Production-style Compose start

No credential is committed. Copy `.env.example` to `.env`, set a long random
`POSTGRES_PASSWORD`, and set `JKB_PUBLIC_URL` to the externally visible HTTPS
origin. Keep `JKB_COOKIE_SECURE=true` behind HTTPS; the example disables it only
for local HTTP.

```sh
docker compose build
docker compose up -d db
docker compose run --rm app /usr/local/bin/jkb-migrate up
docker compose run --rm app /usr/local/bin/jkb-admin admin "Administrator"
docker compose up -d app
```

The admin command requests the password interactively and never prints it.
Migrations are intentionally explicit, making deploy and rollback order
observable. Use `jkb-migrate down` only after checking the migration impact.

The application is available on `JKB_PORT` (8080 by default), health endpoints
are `/health/live` and `/health/ready`, REST is under `/api/v1`, and remote MCP
is `/mcp` with `Authorization: Bearer jkb_pat_…`.

## Configuration

| Variable                   | Required   | Default     | Meaning                              |
| -------------------------- | ---------- | ----------- | ------------------------------------ |
| `JKB_DATABASE_URL`         | yes        | —           | PostgreSQL connection URL            |
| `JKB_STORAGE_PATH`         | yes        | `./storage` | local attachment root                |
| `JKB_LISTEN_ADDRESS`       | no         | `:8080`     | HTTP listen address                  |
| `JKB_PUBLIC_URL`           | deployment | —           | externally visible origin            |
| `JKB_COOKIE_SECURE`        | no         | `true`      | send session cookies only over HTTPS |
| `JKB_ATTACHMENT_MAX_BYTES` | no         | `26214400`  | maximum attachment size              |
| `JKB_SHUTDOWN_TIMEOUT`     | no         | `15s`       | graceful shutdown deadline           |

## Backup and restore

Back up both persistent volumes as one operational snapshot. Stop writes (or
briefly stop `app`), then dump PostgreSQL and archive the attachment volume:

```sh
docker compose stop app
docker compose exec -T db pg_dump -U jkb -d jkb -Fc > jkb.dump
docker compose run --rm --no-deps -v "$PWD":/backup app \
  tar -C /var/lib/jkb/attachments -czf /backup/jkb-attachments.tar.gz .
docker compose start app
```

Restore into an empty deployment before starting `app`: restore the database
with `pg_restore --clean --if-exists` and unpack the attachment archive into the
attachments volume. Test restore procedures regularly; an untested backup is
merely an optimistic file collection.

## Local development

Run PostgreSQL and migrations, then start Go and Vite separately. Vite proxies
`/api` and `/mcp` to port 8080. A production build is embedded automatically by
the Dockerfile; locally, run `npm run build` before `go build ./cmd/jkb` when an
all-in-one binary is needed.
