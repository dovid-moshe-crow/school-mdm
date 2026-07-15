# School MDM

Go + Neon (optional) school iOS MDM control plane: allowlists, access requests, and profile generation.

Nano protocol libraries will be wired for real device enrollments later. This phase runs without APNs or hardware — Approve/Deny uses a stub command enqueuer.

## Requirements

- Go 1.24+ (this project installs/uses `$HOME/.local/go` if present)
- Optional: [Neon](https://neon.tech) Postgres (`DATABASE_URL`)

## Quick start

```bash
cp .env.example .env
export PATH="$HOME/.local/go/bin:$PATH"
make tidy
make test
make run
```

With no `DATABASE_URL`, the server uses an in-memory store.

### Neon (reclaimable DB)

```bash
npm install -g neonctl   # if needed
export NEON_API_KEY=...  # from Neon console
# optional: export NEON_PROJECT_ID=...
make neon                # creates branch school-mdm-dev and prints DATABASE_URL
# paste into .env, then make run (uses postgres store + auto-migrations)
```

Without credentials the server uses the in-memory store.
## HTTP

| Method | Path | Notes |
|--------|------|-------|
| GET | `/healthz` | liveness + store kind |
| GET | `/` | student portal (HTML) |
| GET | `/admin` | admin queue (HTML) |
| POST | `/api/requests` | create access request (JSON) |
| GET | `/api/requests` | list requests (admin Bearer) |
| POST | `/api/requests/{id}/approve` | approve (admin Bearer) |
| POST | `/api/requests/{id}/deny` | deny (admin Bearer) |
| GET | `/api/allowlist` | effective allowlist |
| GET | `/api/stub-commands` | profiles the stub would have pushed |

Admin auth: `Authorization: Bearer <token>` matching `ADMIN_TOKENS`.

## Layout

See `internal/` — `policy` and `approvals` have no HTTP or Nano imports.
