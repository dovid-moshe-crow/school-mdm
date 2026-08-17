import type { ExpoConfig, ConfigContext } from 'expo/config'

/** Student companion app — wraps the school portal Web UI. */
export default ({ config }: ConfigContext): ExpoConfig => ({
  ...config,
  name: 'KFilter',
  slug: 'kfilter-portal',
  version: '1.0.0',
  orientation: 'portrait',
  icon: './assets/icon.png',
  userInterfaceStyle: 'light',
  scheme: 'kfilter',
  splash: {
    image: './assets/splash-icon.png',
    resizeMode: 'contain',
    backgroundColor: '#0b3d2e',
  },
  ios: {
    supportsTablet: true,
    bundleIdentifier: 'com.kfilter.portal',
    entitlements: {
      'aps-environment': 'production',
    },
    infoPlist: {
      CFBundleAllowMixedLocalizations: true,
      ITSAppUsesNonExemptEncryption: false,
      UIBackgroundModes: ['remote-notification'],
      NSAppTransportSecurity: {
        NSAllowsArbitraryLoads: false,
      },
    },
  },
  android: {
    package: 'com.kfilter.portal',
    adaptiveIcon: {
      backgroundColor: '#0b3d2e',
      foregroundImage: './assets/android-icon-foreground.png',
      backgroundImage: './assets/android-icon-background.png',
      monochromeImage: './assets/android-icon-monochrome.png',
    },
  },
  web: {
    favicon: './assets/favicon.png',
  },
  plugins: [
    [
      'expo-notifications',
      {
        color: '#0b3d2e',
        defaultChannel: 'default',
      },
    ],
    [
      'expo-mdm',
      {
        android: {
          AppRestrictionsMap: {
            enrollment_id: {
              title: 'Enrollment ID',
              description: 'School device enrollment identifier',
              type: 'string',
              defaultValue: '',
            },
            portal_base_url: {
              title: 'Portal base URL',
              description: 'Optional override for the student portal',
              type: 'string',
              defaultValue: 'https://nanok.kfilter.net',
            },
          },
        },
      },
    ],
  ],
  extra: {
    portalBaseUrl: process.env.EXPO_PUBLIC_PORTAL_BASE_URL || 'https://nanok.kfilter.net',
    // Local/TestFlight simulation of Managed App Config (never use in production MDM).
    debugEnrollmentId: process.env.EXPO_PUBLIC_DEBUG_ENROLLMENT_ID || '',
    eas: {
      projectId: '16fee8be-d2c9-4054-9e72-1dde57aaf72e',
    },
  },
})
