# School MDM

Go + Neon (optional) school iOS MDM control plane: allowlists, access requests, and profile generation.

Nano protocol libraries will be wired for real device enrollments later. This phase runs without APNs or hardware — Approve/Deny uses a stub command enqueuer.

## Requirements

- Go 1.24+ (this project installs/uses `$HOME/.local/go` if present)
- Optional: [Neon](https://neon.tech) Postgres (`DATABASE_URL`)

## Quick start

```bash
cp .env.example .env
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
make tidy
make test
make run
```

### Live reload (recommended in your terminal)

Watches the repo and rebuilds/restarts the server when code changes:

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd ~/Projects/school-mdm
make dev
```

Then open http://localhost:8080/admin and http://localhost:8080/d/demo-ipad.  
Leave that terminal open. `Ctrl+C` stops it.

(`make dev` installs [Air](https://github.com/air-verse/air) on first run if needed.)

With no `DATABASE_URL`, the server uses an in-memory store.

### Neon (claimable DB — recommended)

Creates a temporary hosted Postgres with **no Neon login** (expires in ~72h unless you claim it):

```bash
make neon          # writes DATABASE_URL into .env
make run           # or: make dev
```

Then open http://localhost:8080/healthz — you should see `"store":"postgres"`.

Claim the DB to your Neon account (optional, keeps it permanently) using `PUBLIC_POSTGRES_CLAIM_URL` in `.env`.
## HTTP

| Method | Path | Notes |
|--------|------|-------|
| GET | `/` | explains device-scoped portal |
| GET | `/d/{deviceID}` | student portal (device id in URL; `?url=` optional) |
| GET | `/admin` | admin queue |
| GET | `/api/apps/search?q=` | App Store search (cache + iTunes fallback) |
| GET | `/api/apps/{bundleID}` | lookup/cached metadata |
| POST | `/api/requests` | create request (`enrollment_id` usually from `/d/...`) |
| GET | `/api/requests` | list/filter requests (`status`, `type`, `enrollment_id`, `q`, `sort`) |
| GET | `/api/allowances` | allowlists (`scope=global\|device\|all`, `kind`, `enrollment_id`, `q`) |
| GET | `/api/devices` | known device IDs (admin) |
| POST | `/api/requests/{id}/approve` | approve/resolve (admin) |
| POST | `/api/requests/{id}/deny` | deny (admin) |
| GET | `/api/allowlist` | effective allowlist |
| GET | `/api/stub-commands` | profiles the stub would have pushed |

Access approvals update allowlists and stub-enqueue a profile. General/bug tickets only change status.
App metadata is cached in `app_metadata` and refreshed from the iTunes Search/Lookup API.

## Layout

See `internal/` — `policy` and `approvals` have no HTTP or Nano imports.
