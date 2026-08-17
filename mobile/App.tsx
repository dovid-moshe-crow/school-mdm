import AsyncStorage from '@react-native-async-storage/async-storage'
import Constants from 'expo-constants'
import * as Device from 'expo-device'
import * as Notifications from 'expo-notifications'
import { StatusBar } from 'expo-status-bar'
import { getConfiguration } from 'expo-mdm'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ActivityIndicator,
  I18nManager,
  Platform,
  Pressable,
  SafeAreaView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from 'react-native'
import { WebView } from 'react-native-webview'

const ENROLLMENT_KEY = 'school_mdm_enrollment_id'
const MDM_BOUND_KEY = 'school_mdm_from_managed_config'
const PORTAL_BASE_KEY = 'school_mdm_portal_base'

type Extra = {
  portalBaseUrl?: string
  debugEnrollmentId?: string
  eas?: { projectId?: string }
}

Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowBanner: true,
    shouldShowList: true,
    shouldPlaySound: true,
    shouldSetBadge: false,
  }),
})

function extra(): Extra {
  return (Constants.expoConfig?.extra ?? {}) as Extra
}

function defaultPortalBase(): string {
  const fromExtra = (extra().portalBaseUrl || '').replace(/\/$/, '')
  if (fromExtra) return fromExtra
  return 'https://nanok.kfilter.net'
}

async function readManagedConfig(): Promise<{
  enrollmentId?: string
  portalBaseUrl?: string
}> {
  try {
    const cfg = (await getConfiguration()) as Record<string, unknown> | null
    if (!cfg || typeof cfg !== 'object') return {}
    const enrollmentId = String(cfg.enrollment_id ?? cfg.enrollmentId ?? '').trim()
    const portalBaseUrl = String(cfg.portal_base_url ?? cfg.portalBaseUrl ?? '')
      .trim()
      .replace(/\/$/, '')
    return {
      enrollmentId: enrollmentId || undefined,
      portalBaseUrl: portalBaseUrl || undefined,
    }
  } catch {
    return {}
  }
}

async function registerForPushAsync(projectId?: string): Promise<string | null> {
  if (!Device.isDevice) return null
  const { status: existing } = await Notifications.getPermissionsAsync()
  let final = existing
  if (existing !== 'granted') {
    const req = await Notifications.requestPermissionsAsync()
    final = req.status
  }
  if (final !== 'granted') return null
  if (Platform.OS === 'android') {
    await Notifications.setNotificationChannelAsync('default', {
      name: 'default',
      importance: Notifications.AndroidImportance.DEFAULT,
    })
  }
  const token = await Notifications.getExpoPushTokenAsync(
    projectId ? { projectId } : undefined,
  )
  return token.data || null
}

export default function App() {
  const [enrollmentId, setEnrollmentId] = useState<string | null>(null)
  const [mdmBound, setMdmBound] = useState(false)
  const [portalBaseState, setPortalBaseState] = useState(defaultPortalBase())
  const [draft, setDraft] = useState('')
  const [booting, setBooting] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [pathSuffix, setPathSuffix] = useState('')
  const webRef = useRef<WebView>(null)
  const longPressRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (!I18nManager.isRTL) {
      try {
        I18nManager.allowRTL(true)
        I18nManager.forceRTL(true)
      } catch {
        // Expo Go may ignore RTL flips until reload.
      }
    }
    void (async () => {
      try {
        const managed = await readManagedConfig()
        const debugId = (extra().debugEnrollmentId || '').trim()
        let id = managed.enrollmentId || ''
        let fromMdm = !!managed.enrollmentId
        if (!id && debugId) {
          id = debugId
          fromMdm = true
        }
        if (managed.portalBaseUrl) {
          await AsyncStorage.setItem(PORTAL_BASE_KEY, managed.portalBaseUrl)
          setPortalBaseState(managed.portalBaseUrl)
        } else {
          const savedBase = (await AsyncStorage.getItem(PORTAL_BASE_KEY))?.trim()
          if (savedBase) setPortalBaseState(savedBase.replace(/\/$/, ''))
        }
        if (id) {
          await AsyncStorage.setItem(ENROLLMENT_KEY, id)
          await AsyncStorage.setItem(MDM_BOUND_KEY, fromMdm ? '1' : '0')
          setEnrollmentId(id)
          setMdmBound(fromMdm)
        } else {
          const saved = (await AsyncStorage.getItem(ENROLLMENT_KEY))?.trim() || ''
          const bound = (await AsyncStorage.getItem(MDM_BOUND_KEY)) === '1'
          if (saved) {
            setEnrollmentId(saved)
            setMdmBound(bound)
          }
        }
      } finally {
        setBooting(false)
      }
    })()
  }, [])

  useEffect(() => {
    if (!enrollmentId) return
    const projectId = extra().eas?.projectId
    void (async () => {
      try {
        const token = await registerForPushAsync(projectId)
        if (!token) return
        await fetch(
          `${portalBaseState}/api/device/${encodeURIComponent(enrollmentId)}/push-token`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ token, platform: Platform.OS === 'ios' ? 'ios' : 'android' }),
          },
        )
      } catch {
        // Push registration is best-effort; portal still works.
      }
    })()
  }, [enrollmentId, portalBaseState])

  useEffect(() => {
    const sub = Notifications.addNotificationResponseReceivedListener((response) => {
      const data = response.notification.request.content.data as {
        path?: string
        enrollment_id?: string
      }
      const path = typeof data?.path === 'string' ? data.path : ''
      if (path) setPathSuffix(path.startsWith('?') || path.startsWith('/') ? path : `?${path}`)
      const fromNote = typeof data?.enrollment_id === 'string' ? data.enrollment_id.trim() : ''
      if (fromNote && !enrollmentId) {
        void AsyncStorage.setItem(ENROLLMENT_KEY, fromNote)
        setEnrollmentId(fromNote)
      }
    })
    return () => sub.remove()
  }, [enrollmentId])

  const portalURL = useMemo(() => {
    if (!enrollmentId) return ''
    // client=kfilter hides external payment UI in the portal (App Store 3.1.1).
    const u = new URL(`${portalBaseState}/d/${encodeURIComponent(enrollmentId)}`)
    u.searchParams.set('client', 'kfilter')
    if (!pathSuffix) return u.toString()
    if (pathSuffix.startsWith('?')) {
      const extra = new URLSearchParams(pathSuffix.slice(1))
      extra.forEach((v, k) => {
        if (k !== 'client') u.searchParams.set(k, v)
      })
      return u.toString()
    }
    if (pathSuffix.startsWith('/')) {
      const root = portalBaseState.endsWith('/') ? portalBaseState : `${portalBaseState}/`
      const abs = new URL(pathSuffix.replace(/^\//, ''), root)
      abs.searchParams.set('client', 'kfilter')
      return abs.toString()
    }
    u.searchParams.set('tab', pathSuffix)
    return u.toString()
  }, [enrollmentId, portalBaseState, pathSuffix])

  const saveEnrollment = useCallback(async () => {
    const id = draft.trim()
    if (!id) return
    await AsyncStorage.setItem(ENROLLMENT_KEY, id)
    await AsyncStorage.setItem(MDM_BOUND_KEY, '0')
    setMdmBound(false)
    setEnrollmentId(id)
    setLoadError('')
  }, [draft])

  const clearEnrollment = useCallback(async () => {
    await AsyncStorage.removeItem(ENROLLMENT_KEY)
    await AsyncStorage.removeItem(MDM_BOUND_KEY)
    setEnrollmentId(null)
    setMdmBound(false)
    setDraft('')
    setLoadError('')
    setPathSuffix('')
  }, [])

  const onTitlePressIn = useCallback(() => {
    if (!mdmBound) return
    longPressRef.current = setTimeout(() => {
      void clearEnrollment()
    }, 2500)
  }, [mdmBound, clearEnrollment])

  const onTitlePressOut = useCallback(() => {
    if (longPressRef.current) clearTimeout(longPressRef.current)
  }, [])

  if (booting) {
    return (
      <SafeAreaView style={styles.center}>
        <Text style={styles.bootBrand}>KFilter</Text>
        <ActivityIndicator size="large" color="#0b3d2e" style={{ marginTop: 16 }} />
        <Text style={styles.bootHint}>מתחברים…</Text>
        <StatusBar style="dark" />
      </SafeAreaView>
    )
  }

  if (!enrollmentId) {
    return (
      <SafeAreaView style={styles.setup}>
        <View style={styles.heroBand} />
        <Text style={styles.title}>KFilter</Text>
        <Text style={styles.lead}>
          במכשירים שמנוהלים על ידי בית הספר אין צורך להקליד מזהה — האפליקציה מתחברת לבד.
          {'\n'}
          אם התקנתם מ־TestFlight או ידנית, הזינו את מזהה המכשיר מהניהול.
        </Text>
        <TextInput
          style={styles.input}
          value={draft}
          onChangeText={setDraft}
          placeholder="00008140-…"
          placeholderTextColor="#8a9a94"
          autoCapitalize="none"
          autoCorrect={false}
          textAlign="left"
        />
        <Pressable
          style={[styles.button, !draft.trim() && styles.buttonDisabled]}
          disabled={!draft.trim()}
          onPress={() => void saveEnrollment()}
        >
          <Text style={styles.buttonText}>כניסה</Text>
        </Pressable>
        <Text style={styles.hint}>בסיס פורטל: {portalBaseState}</Text>
        <StatusBar style="dark" />
      </SafeAreaView>
    )
  }

  return (
    <SafeAreaView style={styles.flex}>
      {!mdmBound ? (
        <View style={styles.toolbar}>
          <Text style={styles.toolbarTitle} numberOfLines={1}>
            KFilter
          </Text>
          <Pressable onPress={() => void clearEnrollment()} hitSlop={8}>
            <Text style={styles.toolbarAction}>החלפת מכשיר</Text>
          </Pressable>
        </View>
      ) : (
        <Pressable onPressIn={onTitlePressIn} onPressOut={onTitlePressOut} style={styles.mdmBar}>
          <Text style={styles.mdmBarTitle}>KFilter</Text>
        </Pressable>
      )}
      {loadError ? (
        <View style={styles.errorBox}>
          <Text style={styles.errorText}>{loadError}</Text>
          <Pressable
            onPress={() => {
              setLoadError('')
              webRef.current?.reload()
            }}
          >
            <Text style={styles.toolbarAction}>נסה שוב</Text>
          </Pressable>
        </View>
      ) : null}
      <WebView
        ref={webRef}
        source={{ uri: portalURL }}
        style={styles.flex}
        onError={() => setLoadError('טעינת הפורטל נכשלה — בדקו רשת ואת כתובת השרת.')}
        onHttpError={() => setLoadError('השרת החזיר שגיאה. בדקו שהמכשיר רשום בניהול.')}
        startInLoadingState
        renderLoading={() => (
          <View style={styles.webviewLoading}>
            <ActivityIndicator size="large" color="#0b3d2e" />
            <Text style={styles.bootHint}>טוען את הפורטל…</Text>
          </View>
        )}
        allowsBackForwardNavigationGestures
        setSupportMultipleWindows={false}
      />
      <StatusBar style="dark" />
    </SafeAreaView>
  )
}

const styles = StyleSheet.create({
  flex: { flex: 1, backgroundColor: '#eef5f1' },
  center: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#eef5f1',
  },
  bootBrand: {
    fontSize: 32,
    fontWeight: '800',
    color: '#0b3d2e',
    letterSpacing: 0.5,
  },
  bootHint: {
    marginTop: 12,
    color: '#4a635a',
    fontSize: 15,
  },
  setup: {
    flex: 1,
    paddingHorizontal: 24,
    paddingTop: 56,
    backgroundColor: '#eef5f1',
    gap: 12,
  },
  heroBand: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    height: 160,
    backgroundColor: '#0b3d2e',
    opacity: 0.08,
  },
  title: {
    fontSize: 34,
    fontWeight: '800',
    color: '#0b3d2e',
    textAlign: 'right',
  },
  lead: {
    fontSize: 16,
    lineHeight: 24,
    color: '#345048',
    textAlign: 'right',
    marginBottom: 8,
  },
  input: {
    borderWidth: 1,
    borderColor: '#c5d4ce',
    backgroundColor: '#fff',
    borderRadius: 10,
    paddingHorizontal: 14,
    paddingVertical: 12,
    fontSize: 16,
    color: '#122',
  },
  button: {
    backgroundColor: '#0b3d2e',
    borderRadius: 10,
    paddingVertical: 14,
    alignItems: 'center',
    marginTop: 4,
  },
  buttonDisabled: { opacity: 0.45 },
  buttonText: { color: '#fff', fontSize: 17, fontWeight: '600' },
  hint: { marginTop: 16, color: '#6b7f78', fontSize: 13, textAlign: 'right' },
  toolbar: {
    flexDirection: 'row-reverse',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 14,
    paddingVertical: 10,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: '#c5d4ce',
    backgroundColor: '#fff',
  },
  toolbarTitle: { fontSize: 16, fontWeight: '600', color: '#0b3d2e', flex: 1 },
  toolbarAction: { fontSize: 14, color: '#1a6b52', fontWeight: '600' },
  mdmBar: {
    paddingHorizontal: 14,
    paddingVertical: 8,
    backgroundColor: '#0b3d2e',
  },
  mdmBarTitle: {
    color: '#fff',
    fontWeight: '700',
    fontSize: 15,
    textAlign: 'center',
  },
  errorBox: {
    padding: 12,
    backgroundColor: '#fdecea',
    gap: 6,
  },
  errorText: { color: '#8a1f11', textAlign: 'right' },
  webviewLoading: {
    position: 'absolute',
    top: 0,
    right: 0,
    bottom: 0,
    left: 0,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#eef5f1',
    gap: 8,
  },
})
