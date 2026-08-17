# MDM continuity contract

Zero re-enrollment cutover requires school-mdm to present the **same device-facing identity** as the previous NanoHUB (`nanok`) deployment. Schema table *names* may differ (we use Postgres schema `mdm`); crypto and protocol values must not.

## Non-negotiable at cutover

| Item | Requirement |
|------|-------------|
| Public host | Same HTTPS host as installed enrollment profiles (e.g. `https://nanok.kfilter.net`) |
| Paths | `/mdm`, `/scep`; also `/checkin` **only if** production used `MDM_CHECKIN=true` |
| Topic | Same APNs MDM topic as push certificate and enrollment rows |
| SCEP CA | Same CA certificate + encrypted private key; **same** `MDM_SCEP_CAPASS` |
| Push | Same cert PEM + key PEM (topic must match) |
| Device rows | Enrollment id, `push_magic`, `token_hex`, `enabled`; `cert_auth_associations`; identity cert as stored |
| Auth mode | Default Mdm-Signature; set `MDM_CERT_HEADER` only if the reverse proxy previously injected client certs |

## Dump inventory (provide before import)

From the old nanok database (and/or secrets):

- [ ] `enrollments` (all columns)
- [ ] `devices` (MDM inventory — not school nicknames)
- [ ] `users` (if any user enrollments)
- [ ] `push_certs`
- [ ] `cert_auth_associations`
- [ ] `scep_certificates`, `scep_ca_keys`, `scep_serials` (and challenges if used)
- [ ] Env: public URL, topic, SCEP passphrase, CHECKIN flag, optional SCEP challenge
- [ ] Push PEM/key if not only in DB

Optional later: mid-flight `commands` / `enrollment_queue`, DEP (`dep_names`), NanoCMD workflow tables.

## Local / lab (not continuity)

Without an import, school-mdm may generate a **new** SCEP CA when `MDM_SCEP_CAPASS` is set. Lab-enrolled devices will **not** match production devices.

## Identity rule (school product)

For live managed devices, school `enrollment_id` **is** the Apple MDM enrollment id (device UDID for classic device enrollment). Portal paths `/d/{enrollment_id}`, credits, and groups use that same string.

## Import

Use `go run ./cmd/mdmimport` (see command help). It writes through `mdmstore` into schema `mdm` so encodings stay consistent with the protocol stack.
