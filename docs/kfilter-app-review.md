# KFilter — App Review walkthrough

Companion to `mobile/README.md`. Use this when submitting the Custom App for first review.

## What shipped for review

1. Auto device binding via Managed App Config (`enrollment_id`, optional `portal_base_url`)
2. Expo push on request approve/deny and admin messages
3. Manual enrollment fallback for TestFlight / reviewers
4. MDM `InstallApplication` + managed config on reconcile (Adam ID + VPP token in Admin)

## Pre-review MDM test (managed config)

Apple usually **does not** put a Custom App in ASM Apps & Books until after first review, so `InstallApplication` by Adam ID often cannot be tested yet.

You **can** still test that the payload binds the device:

1. Install KFilter **1.0.0 (4)** from TestFlight on the supervised school iPhone (same enrollment as Admin).
2. Restart `schoolmdm` with the latest code (configure-companion endpoint).
3. Admin → device → **שליחת מזהה מכשיר לאפליקציה** (`POST …/configure-companion`).
4. Force-quit KFilter and reopen — it should skip the manual ID screen and open `/d/{enrollment_id}`.

If iOS ignores the config (app not “managed” yet), that is an Apple limitation for TestFlight installs; the same Settings/`Configurations` payload is what InstallApplication will send after the Custom App appears in ASM. Logic can still be checked with `EXPO_PUBLIC_DEBUG_ENROLLMENT_ID` on a preview build.

## Before Submit

### Apple Developer (required before the next EAS iOS build)

Build `58a21093-7184-4d89-9f9e-2af422eb989a` failed because the App Store provisioning profile does not include Push:

> Provisioning profile doesn't support the Push Notifications capability / `aps-environment`

Do this once, then rebuild:

1. [developer.apple.com/account/resources/identifiers](https://developer.apple.com/account/resources/identifiers/list) → **Identifiers** → `com.kfilter.portal`
2. Enable **Push Notifications** → **Save**
3. Regenerate the Expo profile (picks up the new capability):
   ```bash
   cd mobile
   eas credentials -p ios
   # Select the App Store profile → Delete / regenerate, or:
   eas build --platform ios --profile production --clear-credentials
   ```
   Prefer regenerating only the provisioning profile (keep the distribution cert).
4. Ensure an **APNs Key** is uploaded in EAS credentials (Push key for the team).
5. Rebuild:
   ```bash
   eas build --platform ios --profile production
   ```

### school-mdm

- [ ] Server restarted after deploy (migration `018_kfilter_companion.sql` applies on boot)
- [ ] Admin → Settings → KFilter: Adam ID when available; upload VPP content token
- [ ] Smoke: approve a pending request → push reaches a device with registered token

### EAS / TestFlight

```bash
cd mobile
eas build --platform ios --profile production
eas submit --platform ios --profile production
```

- [ ] Install build on test iPhone
- [ ] Manual id entry works
- [ ] Optional preview with `EXPO_PUBLIC_DEBUG_ENROLLMENT_ID` to simulate MDM bind

### App Store Connect listing

- [ ] Privacy policy URL + support URL
- [ ] Screenshots for the device sizes Apple requires
- [ ] Age rating completed
- [ ] Distribution: **Private / Custom App** → Organization ID `61069358`
- [ ] Review notes: explain MDM auto-bind; provide a TestFlight enrollment id if the reviewer cannot use ASM

Then **Add for Review**.

## API reference (companion)

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/device/{enrollmentId}/push-token` | Register Expo token `{ token, platform }` |
| PUT | `/api/mdm/abm/settings` | `companion_itunes_id`, `companion_bundle_id`, `companion_enabled` |
| PUT | `/api/mdm/vpp/token` | Raw Apps & Books content token body |
| POST | `/api/mdm/devices/{id}/install-companion` | Queue install + managed config |
