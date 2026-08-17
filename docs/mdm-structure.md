# MDM code structure (DRY)

See the reviewed plan; this is the layout as implemented.

```text
internal/mdm/           CommandEnqueuer + StubEnqueuer
internal/mdm/commands/  MDM command plists
internal/mdm/enqueue/   LiveEnqueuer (nanomdm + APNs)
internal/mdmhub/        /mdm /scep /enroll /version mount
internal/mdmscep/       SCEP CA depot (schema mdm)
internal/mdmstore/      school-facing MDM store + import writers
internal/devicepush/    single Reconcile(allowlist→profile→enqueue)
cmd/mdmimport/          transform import from nanok DB
migrations/011_mdm_schema.sql   Postgres schema `mdm`
```

Rules:

- School `store.Store` stays product-only (no SCEP/APNs).
- Approvals and allowance CRUD call `devicepush.Reconcile` only.
- Protocol libs stay behind `mdmhub` / `mdmstore` / `mdm/enqueue`.
