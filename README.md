# School MDM

Go + Neon (optional) school iOS MDM: allowlists, access requests, credits, **and** an additive Apple MDM protocol plane (NanoMDM + SCEP + APNs).

Default `MDM_ENQUEUE=stub` keeps Approve/Deny offline. Set `MDM_ENQUEUE=live` with `DATABASE_URL` + `MDM_SCEP_CAPASS` to serve `/mdm`, `/scep`, `/enroll` and push real profiles. Continuity/import docs: [`docs/mdm-continuity.md`](docs/mdm-continuity.md).

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

Starts **Vite** (UI with HMR) and **Air** (Go API) together:

```bash
export PATH="$HOME/.local/go/bin:$HOME/go/bin:$PATH"
cd ~/Projects/school-mdm
make dev
```

Then open **http://127.0.0.1:5173** (UI). The API still listens on http://127.0.0.1:8080 — Vite proxies `/api` there.  
Leave that terminal open. `Ctrl+C` stops both.

(`make dev` installs [Air](https://github.com/air-verse/air) on first run if needed.)

For a production-style embed (built UI served by Go on :8080): `make run` or `make web` then restart the server.

With no `DATABASE_URL`, the server uses an in-memory store.

### Neon (claimable DB — recommended)

Creates a temporary hosted Postgres with **no Neon login** (expires in ~72h unless you claim it):

```bash
make neon          # writes DATABASE_URL into .env
make run           # or: make dev
```

Then open http://localhost:8080/healthz — you should see `"store":"postgres"`.

Claim the DB to your Neon account (optional, keeps it permanently) using `PUBLIC_POSTGRES_CLAIM_URL` in `.env`.

## Admin login

The admin UI (`/admin`, `/api-docs`) requires a signed-in admin. **Sign in with Google** is the production path (`golang.org/x/oauth2`):

1. Google Cloud Console → APIs & Services → Credentials → Create OAuth client (Web application).
2. Add authorized redirect URI `https://YOUR_HOST/api/auth/google/callback`.
3. Set `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, and `ADMIN_EMAILS` (comma-separated) or `ADMIN_GOOGLE_HOSTED_DOMAIN`.
4. Restart the server. Open `/admin` and use **התחברות עם Google**.

Scripts can still send `Authorization: Bearer <ADMIN_TOKENS>`. Locally, if Google is not configured, `/admin` accepts the `ADMIN_TOKENS` value (default `dev-admin-token`).

Student portal routes stay public and are scoped by device id. Admin JSON routes (devices, packs, groups, approve/deny, MDM, credits admin, webhooks, …) return 401 without a session cookie or Bearer token.
## HTTP

| Method | Path | Notes |
|--------|------|-------|
| GET | `/` | explains device-scoped portal |
| GET | `/d/{deviceID}` | student portal (device id in URL; `?url=` optional) |
| GET | `/admin` | admin queue (Google sign-in or `ADMIN_TOKENS`) |
| GET | `/api-docs` | interactive API reference + webhook manager |
| GET | `/api/openapi.json` | OpenAPI 3.1 for every admin capability |
| GET/POST/PATCH/DELETE | `/api/webhooks` | outbound activity webhooks (Bearer `ADMIN_TOKENS`) |
| GET/POST/PATCH/DELETE | `/api/timers` | scheduled add/remove of whitelist packs and custom profiles on devices/groups (Bearer `ADMIN_TOKENS`; weekly clocks are Asia/Jerusalem) |
| GET/POST/PATCH/DELETE | `/api/profiles` | uploaded Apple `.mobileconfig` profiles, assignable to everyone / groups / devices (Bearer `ADMIN_TOKENS`) |
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
| GET | `/version` | MDM version JSON (when live) |
| GET | `/enroll` | enrollment mobileconfig (when configured) |
| * | `/mdm`, `/scep` | Apple MDM + SCEP (when `MDM_ENQUEUE=live`) |
| GET | `/api/mdm/status` | MDM status |
| GET/POST | `/api/mdm/devices/...` | thin MDM admin (Bearer `ADMIN_TOKENS`) |

Access approvals update allowlists and enqueue a profile (`devicepush.Reconcile`). Allowance CRUD also reconciles. General/bug tickets only change status.
App metadata is cached in `app_metadata` and refreshed from the iTunes Search/Lookup API.

Import nanok continuity data: `go run ./cmd/mdmimport -src "$NANOK_DATABASE_URL" -dst "$DATABASE_URL"` (see `docs/mdm-continuity.md`).

## Credits + Nedarim Plus

Access requests (app/URL) cost credits per device (`enrollment_id`). General and bug tickets are free.
Denied access requests refund the credit.

| Env | Default | Notes |
|-----|---------|-------|
| `NEDARIM_MODE` | `fake` | `fake` = local DebitIframe simulation; `live` = real Nedarim |
| `NEDARIM_MOSAD_ID` | | Required for live |
| `NEDARIM_API_VALID` | | Required for live |
| `NEDARIM_API_PASSWORD` | | Optional; used when creating DebitIframe transactions |
| `CREDITS_ACCESS_COST` | `1` | Credits spent per access request |
| `MDM_ENQUEUE` | `stub` | `live` enables protocol + APNs enqueue |
| `MDM_PUBLIC_URL` | | HTTPS base for `/enroll` profile URLs |
| `MDM_TOPIC` | | APNs MDM topic |
| `MDM_SCEP_CAPASS` | | Required for live (encrypts/decrypts SCEP CA key) |
| `MDM_CHECKIN` | `false` | Separate `/checkin` if production profiles used it |

### Fake payment flow (local)

1. Open portal `/d/{deviceId}` → **רכישת קרדיטים**
2. Pick a package → fake Nedarim iframe opens
3. Click **תשלום** → server marks purchase paid via the same webhook path and credits the ledger
4. Parent confirms via `POST /api/credits/confirm` and refreshes balance
5. Access requests now succeed; deny in admin refunds the credit

Packages seeded: 10 / 50 / 100 credits (₪10 / ₪45 / ₪80). Admins can gift credits on the Devices tab.

## Layout

See `docs/mdm-structure.md`. School domain packages (`policy`, `approvals`, `credits`) stay free of HTTP/protocol imports; MDM lives under `mdmhub` / `mdmstore` / `devicepush`.
