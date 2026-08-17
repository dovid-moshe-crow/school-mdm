# Enrollment identity contract

School product keys (`enrollment_id` on requests, groups, credits, portal `/d/{id}`) must match the Apple MDM enrollment identifier used for check-in and APNs.

## Rule

For classic **device** enrollments: `enrollment_id` == device UDID (the NanoMDM enrollment `id`).

Do not invent a separate school UUID that must be translated on every push unless an explicit mapping table is added later.

## Implications

- Student portal links and Web Clips should use the real UDID once devices are managed.
- Demo/free-text portal IDs work only with `MDM_ENQUEUE=stub`.
- Import from nanok maps `enrollments.id` → school usage as `enrollment_id`.
