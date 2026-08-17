# KFilter — student companion app

Expo (React Native) shell that opens the school portal in a WebView, with MDM auto-enrollment and push notifications.

Built for **cloud iOS builds** (EAS) — no Mac required on your PC. **Expo SDK 54**.

## Features

- **Managed App Config** — reads `enrollment_id` (+ optional `portal_base_url`) from MDM; skips the manual setup screen
- **Push** — registers Expo push token with `POST /api/device/{id}/push-token`; taps open the portal updates tab
- **Fallback** — manual enrollment id for TestFlight / reviewers / non-MDM installs
- Simulate MDM binding locally: `EXPO_PUBLIC_DEBUG_ENROLLMENT_ID=00008140-…`

## Dev (Windows)

```bash
cd mobile
npm install
npx expo start
```

## Production / App Review checklist

### 1) Apple Developer — Push Notifications (do this before rebuild)

Production build **#3** failed until Push is on the App ID:

`Provisioning profile doesn't support the Push Notifications capability` / missing `aps-environment`.

1. [Apple Identifiers](https://developer.apple.com/account/resources/identifiers/list) → `com.kfilter.portal` → enable **Push Notifications** → Save
2. `cd mobile && eas credentials -p ios` → regenerate the **App Store provisioning profile** (so it includes Push)
3. Confirm an **APNs Key** exists under EAS iOS credentials
4. See also `docs/kfilter-app-review.md`

### Pre-review MDM config test

Full store install via MDM usually waits until the Custom App is approved in ASM. To test **device identity** earlier:

1. Install the TestFlight build on the supervised phone  
2. Admin → device → **שליחת מזהה מכשיר לאפליקציה**  
3. Force-quit and reopen KFilter — should open the portal with no typing  

### 2) EAS rebuild

```bash
cd mobile
eas build --platform ios --profile preview     # ad hoc / internal test
eas build --platform ios --profile production  # App Store / TestFlight
eas submit --platform ios --profile production # if not auto-submitted
```

Install the new build on a test iPhone. Confirm:

- Manual enrollment still works (TestFlight)
- With `EXPO_PUBLIC_DEBUG_ENROLLMENT_ID` on a preview build, boot skips typing
- Approve a request in Admin → notification arrives → tap opens portal (`?tab=updates`)
- After Custom App appears in ASM + Adam ID saved in Admin → reconcile queues `InstallApplication`

### 3) school-mdm Admin

1. **Settings → KFilter** — set Adam/iTunes ID when ASM lists the app; enable auto-install
2. Upload **Apps & Books content token** (VPP)
3. Device page → **דחיפת KFilter למכשיר** (or Reconcile)

### 4) App Store Connect — Custom App review

1. App **KFilter** (`6800684446`), Private / Custom App → org **61069358** (Or-Efraim)
2. Complete listing: screenshots, privacy URL, support URL, age rating
3. Notes for reviewer: enrollment is automatic under MDM; for TestFlight use the manual id screen; provide a test enrollment id if needed
4. **Add for Review**

## Config

| Variable | Purpose |
|----------|---------|
| `EXPO_PUBLIC_PORTAL_BASE_URL` | Portal API + WebView base (default `https://nanok.kfilter.net`) |
| `EXPO_PUBLIC_DEBUG_ENROLLMENT_ID` | Simulate Managed App Config in non-MDM builds |

- Bundle id: `com.kfilter.portal`
- Display name: `KFilter`
