import {
  Alert,
  App,
  Badge,
  Button,
  Card,
  Collapse,
  Empty,
  Flex,
  Input,
  List,
  Modal,
  Rate,
  Segmented,
  Skeleton,
  Space,
  Spin,
  Tag,
  Typography,
} from 'antd'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  parseAsBoolean,
  parseAsString,
  parseAsStringLiteral,
  useQueryState,
  useQueryStates,
} from 'nuqs'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { api, type AccessStatus, type AppMeta, type CreditPackage, type Request } from '../api'
import { RequestThread } from '../components/RequestThread'
import { ListSearchBar, SearchableCollection } from '../components/ListSearch'
import { useIsMobile } from '../hooks/useIsMobile'
import { he, studentNextAction } from '../he'
import { AppIdentity, useAppMetaStore } from '../appMeta'
import { appTitle } from '../knownApps'
import { AppThumb, normalizeHostPreview, useDebounced } from '../ui'

const categories = ['access-url', 'access-app', 'general', 'bug'] as const
type Category = (typeof categories)[number]

const portalModes = ['store', 'request', 'updates'] as const

const EMPTY_APPS: AppMeta[] = []

function appMetaSearchText(app: AppMeta) {
  return `${appTitle(app, app.bundle_id)} ${app.developer} ${app.bundle_id}`
}

function requestSearchText(r: Request) {
  return `${appTitle(r.app, r.value)} ${r.value} ${r.reason} ${r.status} ${r.type} ${r.last_message?.body || ''}`
}
type PortalMode = (typeof portalModes)[number]

const DESC_PREVIEW = 280

function statusTag(s?: AccessStatus) {
  if (s === 'allowed') return <Tag color="success">{he.alreadyAllowed}</Tag>
  if (s === 'pending') return <Tag color="warning">{he.pendingRequest}</Tag>
  if (s === 'denied') return <Tag color="error">{he.deniedBefore}</Tag>
  return null
}

function nextTagColor(kind: string, status: string) {
  if (kind === 'act') return 'processing'
  if (kind === 'wait') return 'warning'
  if (kind === 'done' && status === 'denied') return 'error'
  return 'success'
}

function fmtTime(v?: string) {
  if (!v) return ''
  try {
    return new Date(v).toLocaleString('he-IL', { dateStyle: 'short', timeStyle: 'short' })
  } catch {
    return v
  }
}

function fmtSize(bytes?: number) {
  if (!bytes || bytes <= 0) return ''
  const mb = bytes / (1024 * 1024)
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`
  return `${mb.toFixed(1)} MB`
}

/** HTTPS App Store product page. Prefer this in Chrome; Safari cannot use it when
 * the App Store *app* is blocked (hard redirect / Universal Link → itms-appss). */
function appStoreInstallURL(app: AppMeta): string {
  if (app.store_url) return app.store_url
  if (app.track_id) return `https://apps.apple.com/app/id${app.track_id}`
  return `https://apps.apple.com/search?term=${encodeURIComponent(app.app_name || app.bundle_id)}`
}

function isIOSDevice(): boolean {
  if (typeof navigator === 'undefined') return false
  const ua = navigator.userAgent || ''
  if (/iPad|iPhone|iPod/i.test(ua)) return true
  // iPadOS desktop UA
  return navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1
}

/** True for Safari / Home Screen web clips — not Chrome/Firefox/Edge on iOS. */
function isIOSSafariFamily(): boolean {
  if (!isIOSDevice()) return false
  const ua = navigator.userAgent || ''
  if (/CriOS|FxiOS|EdgiOS|OPiOS|Chrome|Android/i.test(ua)) return false
  const nav = navigator as Navigator & { standalone?: boolean }
  return /Safari/i.test(ua) || nav.standalone === true
}

/** Open product page in Chrome so Install works without allowing com.apple.AppStore. */
function chromeNavigateURL(httpsURL: string): string {
  const trimmed = httpsURL.trim()
  if (/^https:\/\//i.test(trimmed)) {
    return `googlechromes://${trimmed.slice('https://'.length)}`
  }
  if (/^http:\/\//i.test(trimmed)) {
    return `googlechrome://${trimmed.slice('http://'.length)}`
  }
  return `googlechromes://${trimmed.replace(/^\/\//, '')}`
}

function openAppStore(app: AppMeta, onSafariHandoff?: () => void) {
  const https = appStoreInstallURL(app)
  if (isIOSSafariFamily()) {
    onSafariHandoff?.()
    window.location.assign(chromeNavigateURL(https))
    return
  }
  window.location.assign(https)
}

function updatesSeenKey(deviceId: string) {
  return `portal-updates-seen:${deviceId}`
}

function requestActivityAt(r: Request): number {
  const candidates = [r.last_message?.created_at, r.decided_at, r.created_at]
  let max = 0
  for (const c of candidates) {
    if (!c) continue
    const t = Date.parse(c)
    if (!Number.isNaN(t) && t > max) max = t
  }
  return max
}

function AppDetailsPanel({
  app,
  loading,
}: {
  app: AppMeta
  loading: boolean
}) {
  const { message } = App.useApp()
  const [descOpen, setDescOpen] = useState(false)
  const desc = app.description?.trim() || ''
  const longDesc = desc.length > DESC_PREVIEW
  const shownDesc = !longDesc || descOpen ? desc : `${desc.slice(0, DESC_PREVIEW).trimEnd()}…`

  return (
    <div className="app-details">
      {loading && (
        <Typography.Text type="secondary" className="app-details-loading">
          {he.loadingDetails}
        </Typography.Text>
      )}

      <Flex gap={16} align="start" className="app-details-hero">
        <AppThumb name={app.app_name} url={app.artwork_url} size={88} />
        <div className="app-details-hero-text">
          <Typography.Title level={4} style={{ margin: 0 }}>
            {app.app_name}
          </Typography.Title>
          {app.developer && (
            <Typography.Text type="secondary">
              {he.by} {app.developer}
            </Typography.Text>
          )}
          {!!app.average_rating && (
            <Flex gap={8} align="center" style={{ marginTop: 6 }}>
              <Rate disabled allowHalf value={app.average_rating} style={{ fontSize: 14 }} />
              <Typography.Text type="secondary">
                {app.average_rating.toFixed(1)}
                {app.rating_count
                  ? ` · ${app.rating_count.toLocaleString('he-IL')} דירוגים`
                  : ''}
              </Typography.Text>
            </Flex>
          )}
          <div className="app-chip-row">
            {app.genre && <Tag className="app-chip">{app.genre}</Tag>}
            {app.formatted_price && <Tag className="app-chip">{app.formatted_price}</Tag>}
            {app.content_rating && <Tag className="app-chip">{app.content_rating}</Tag>}
            {!!app.file_size_bytes && <Tag className="app-chip">{fmtSize(app.file_size_bytes)}</Tag>}
            {app.version && <Tag className="app-chip">{he.version} {app.version}</Tag>}
          </div>
        </div>
      </Flex>

      {desc && (
        <div className="app-details-section">
          <Typography.Text strong>{he.description}</Typography.Text>
          <p className="app-description">{shownDesc}</p>
          {longDesc && (
            <Button type="link" onClick={() => setDescOpen((v) => !v)} style={{ paddingInline: 0 }}>
              {descOpen ? he.showLess : he.showMore}
            </Button>
          )}
        </div>
      )}

      {!!app.screenshots?.length && (
        <div className="app-details-section">
          <Typography.Text strong>{he.screenshots}</Typography.Text>
          <div className="screenshot-row">
            {app.screenshots.map((src) => (
              <img key={src} src={src} alt="" loading="lazy" />
            ))}
          </div>
        </div>
      )}

      {(app.store_url || app.track_id) && (
        <Button
          type="link"
          onClick={() =>
            openAppStore(app, () => {
              message.info(he.storeInstallViaChrome)
            })
          }
          style={{ paddingInline: 0 }}
        >
          {he.appStoreLink}
        </Button>
      )}

      <Collapse
        ghost
        size="small"
        items={[
          {
            key: 'advanced',
            label: <Typography.Text type="secondary">{he.advancedInfo}</Typography.Text>,
            children: (
              <Space direction="vertical" size={4}>
                <Typography.Text type="secondary">
                  {he.bundleId}:{' '}
                  <Typography.Text code copyable>
                    {app.bundle_id}
                  </Typography.Text>
                </Typography.Text>
                {app.seller_name && (
                  <Typography.Text type="secondary">
                    {he.seller}: {app.seller_name}
                  </Typography.Text>
                )}
                {app.release_date && (
                  <Typography.Text type="secondary">
                    {he.released}: {fmtTime(app.release_date)}
                  </Typography.Text>
                )}
              </Space>
            ),
          },
        ]}
      />
    </div>
  )
}

function StoreAppActions({
  app,
  onRequest,
  onOpenPending,
}: {
  app: AppMeta
  onRequest: () => void
  onOpenPending: () => void
}) {
  const { message } = App.useApp()
  const status = app.access_status || 'none'
  if (status === 'allowed') {
    return (
      <Button
        type="primary"
        size="small"
        onClick={() =>
          openAppStore(app, () => {
            message.info(he.storeInstallViaChrome)
          })
        }
      >
        {he.storeInstall}
      </Button>
    )
  }
  if (status === 'pending') {
    return (
      <Button size="small" onClick={onOpenPending}>
        {he.storePending}
      </Button>
    )
  }
  return (
    <Button type="primary" size="small" onClick={onRequest}>
      {he.storeRequestAccess}
    </Button>
  )
}

export default function Portal() {
  const { message } = App.useApp()
  const { deviceId = '' } = useParams()
  const location = useLocation()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const historyRef = useRef<HTMLDivElement>(null)
  const isMobile = useIsMobile()
  const isStoreHome = /\/store\/?$/.test(location.pathname)

  const [modeParam, setModeParam] = useQueryState('tab', parseAsStringLiteral(portalModes))
  const mode: PortalMode = modeParam ?? (isStoreHome ? 'store' : 'request')
  // KFilter native shell sets ?client=kfilter — hide external payment UI (ASC 3.1.1).
  const [clientParam] = useQueryState('client', parseAsString)
  const companionClient = clientParam === 'kfilter'

  async function setMode(next: PortalMode) {
    const clientQ = companionClient ? '&client=kfilter' : ''
    if (isStoreHome && next !== 'store') {
      navigate(`/d/${encodeURIComponent(deviceId)}?tab=${next}${clientQ}`)
      return
    }
    if (!isStoreHome && next === 'store') {
      navigate(`/d/${encodeURIComponent(deviceId)}/store${companionClient ? '?client=kfilter' : ''}`)
      return
    }
    await setModeParam(next)
  }
  const [params, setParams] = useQueryStates({
    cat: parseAsStringLiteral(categories),
    url: parseAsString.withDefault(''),
    q: parseAsString.withDefault(''),
    bundle: parseAsString,
    details: parseAsBoolean.withDefault(true),
  })
  const [highlight, setHighlight] = useQueryState('highlight', parseAsString)
  const [storeQ, setStoreQ] = useQueryState('storeq', parseAsString.withDefault(''))

  const category: Category = params.cat ?? (params.url ? 'access-url' : 'access-app')
  const url = params.url
  const query = params.q
  const bundleId = params.bundle
  const detailsOpen = params.details

  const [subject, setSubject] = useState('')
  const [reason, setReason] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [selectedCache, setSelectedCache] = useState<AppMeta | null>(null)
  const [buyOpen, setBuyOpen] = useState(false)
  const [payOpen, setPayOpen] = useState(false)
  const [iframeUrl, setIframeUrl] = useState('')
  const [pendingPurchaseId, setPendingPurchaseId] = useState('')
  const [checkingOut, setCheckingOut] = useState(false)
  const [updatesSeenAt, setUpdatesSeenAt] = useState(() => {
    try {
      const raw = localStorage.getItem(updatesSeenKey(deviceId))
      return raw ? Number(raw) || 0 : 0
    } catch {
      return 0
    }
  })

  const debouncedQ = useDebounced(query, 150)
  const debouncedStoreQ = useDebounced(storeQ, 150)
  const debouncedUrl = useDebounced(url, 350)

  const reasonLabel = useMemo(() => {
    if (category === 'access-url') return he.reasonUrl
    if (category === 'access-app') return he.reasonApp
    if (category === 'bug') return he.reasonBug
    return he.reasonGeneral
  }, [category])

  const urlPreview = useMemo(() => normalizeHostPreview(url), [url])

  const creditsQuery = useQuery({
    queryKey: ['credits', deviceId],
    queryFn: () => api.creditBalance(deviceId),
    enabled: !!deviceId,
    refetchInterval: 15_000,
  })
  const packagesQuery = useQuery({
    queryKey: ['credit-packages'],
    queryFn: () => api.creditPackages(),
    enabled: buyOpen,
  })
  const balance = creditsQuery.data?.available ?? creditsQuery.data?.balance ?? 0
  const accessCost = creditsQuery.data?.access_cost ?? 0
  const creditsEnabled = creditsQuery.data?.enabled !== false && accessCost > 0
  const hidePurchases = companionClient || !creditsEnabled
  const hideCreditsUI = !creditsEnabled
  const isAccess = category === 'access-url' || category === 'access-app'
  const needsCredits = creditsEnabled && isAccess && balance < accessCost

  // TODO(school-mdm): Real lock-screen push for request updates needs a native app
  // or Web Push/PWA — Apple MDM profiles cannot show user banners for approve/deny.
  // Until then, poll while the portal is open and badge the Updates tab.
  const mineQuery = useQuery({
    queryKey: ['my-requests', deviceId],
    queryFn: () => api.myRequests(deviceId),
    enabled: !!deviceId,
    refetchInterval: mode === 'updates' || mode === 'store' ? 15_000 : 20_000,
  })
  const mine = mineQuery.data ?? []

  const updatesBadge = useMemo(() => {
    let n = 0
    for (const r of mine) {
      if (r.status === 'pending') {
        n++
        continue
      }
      if (requestActivityAt(r) > updatesSeenAt) n++
    }
    return n
  }, [mine, updatesSeenAt])

  useEffect(() => {
    if (mode !== 'updates' || !deviceId) return
    const now = Date.now()
    setUpdatesSeenAt(now)
    try {
      localStorage.setItem(updatesSeenKey(deviceId), String(now))
    } catch {
      /* ignore */
    }
  }, [mode, deviceId, mineQuery.dataUpdatedAt])

  const urlStatusQuery = useQuery({
    queryKey: ['access-status', deviceId, 'url', debouncedUrl],
    queryFn: () => api.accessStatus(deviceId, 'url', debouncedUrl.trim()),
    enabled: mode === 'request' && category === 'access-url' && !!debouncedUrl.trim() && !!deviceId,
  })
  const urlStatus = urlStatusQuery.data?.status ?? null
  const checking = urlStatusQuery.isFetching

  const searchQuery = useQuery({
    queryKey: ['app-search', deviceId, debouncedQ],
    queryFn: () => api.searchApps(debouncedQ.trim(), deviceId),
    enabled:
      mode === 'request' &&
      category === 'access-app' &&
      !bundleId &&
      debouncedQ.trim().length >= 2,
  })
  const results = searchQuery.data ?? []
  const searching = searchQuery.isFetching
  const searched = searchQuery.isFetched && debouncedQ.trim().length >= 2

  const storeSearchQuery = useQuery({
    queryKey: ['store-search', deviceId, debouncedStoreQ],
    queryFn: () => api.searchApps(debouncedStoreQ.trim(), deviceId),
    enabled: mode === 'store' && debouncedStoreQ.trim().length >= 2 && !!deviceId,
  })
  const storeResults = storeSearchQuery.data ?? []
  const storeSearching = storeSearchQuery.isFetching
  const storeSearched = storeSearchQuery.isFetched && debouncedStoreQ.trim().length >= 2

  const allowlistQuery = useQuery({
    queryKey: ['effective-allowlist', deviceId],
    queryFn: () => api.effectiveAllowlist(deviceId),
    enabled: mode === 'store' && !!deviceId,
  })

  const { get: getAppMeta } = useAppMetaStore()
  const allowedApps = useMemo(() => {
    const bundles = allowlistQuery.data?.apps || []
    if (!bundles.length) return EMPTY_APPS
    return bundles.map((bundle_id) => ({
      bundle_id,
      app_name: appTitle(undefined, bundle_id),
      developer: '',
    }))
  }, [allowlistQuery.data?.apps])

  const detailsQuery = useQuery({
    queryKey: ['app-lookup', bundleId, deviceId],
    queryFn: () => api.lookupApp(bundleId!, { refresh: true, enrollmentID: deviceId }),
    enabled: mode === 'request' && category === 'access-app' && !!bundleId && !!deviceId,
  })

  useEffect(() => {
    if (detailsQuery.data) setSelectedCache(detailsQuery.data)
  }, [detailsQuery.data])

  useEffect(() => {
    if (detailsQuery.isError) message.error((detailsQuery.error as Error).message)
  }, [detailsQuery.isError, detailsQuery.error, message])

  const selected: AppMeta | null = useMemo(() => {
    if (!bundleId) return null
    if (detailsQuery.data?.bundle_id === bundleId) return detailsQuery.data
    if (selectedCache?.bundle_id === bundleId) return selectedCache
    const fromSearch = results.find((r) => r.bundle_id === bundleId)
    return fromSearch ?? { bundle_id: bundleId, app_name: bundleId, developer: '' }
  }, [bundleId, detailsQuery.data, selectedCache, results])

  async function goRequestApp(app: AppMeta) {
    setSelectedCache(app)
    const q = new URLSearchParams({
      tab: 'request',
      cat: 'access-app',
      bundle: app.bundle_id,
      details: 'true',
    })
    navigate(`/d/${encodeURIComponent(deviceId)}?${q}`)
  }

  async function goUpdatesForPending() {
    await setHighlight(null)
    if (isStoreHome) {
      navigate(`/d/${encodeURIComponent(deviceId)}?tab=updates`)
      return
    }
    await setModeParam('updates')
  }

  async function submit() {
    if (needsCredits) {
      setBuyOpen(true)
      message.warning(he.insufficientCredits)
      return
    }
    setSubmitting(true)
    try {
      const body: Record<string, string> = { enrollment_id: deviceId, reason }
      if (category === 'access-url') {
        Object.assign(body, { type: 'access', kind: 'url', value: url })
      } else if (category === 'access-app') {
        if (!selected) throw new Error('יש לבחור אפליקציה')
        Object.assign(body, { type: 'access', kind: 'app', value: selected.bundle_id })
      } else if (category === 'general') {
        Object.assign(body, { type: 'general', value: subject })
      } else {
        Object.assign(body, { type: 'bug', value: subject })
      }
      const created = await api.createRequest(body)
      message.success(he.sentOk)
      setReason('')
      setSubject('')
      setSelectedCache(null)
      await setParams({ bundle: null, q: '', details: true })
      await setHighlight(created.id)
      await setMode('updates')
      await qc.invalidateQueries({ queryKey: ['my-requests', deviceId] })
      await qc.invalidateQueries({ queryKey: ['credits', deviceId] })
      requestAnimationFrame(() => historyRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
    } catch (err) {
      const msg = (err as Error).message
      if (/insufficient|קרדיט/i.test(msg)) setBuyOpen(true)
      message.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  async function startCheckout(pkg: CreditPackage) {
    setCheckingOut(true)
    try {
      const res = await api.creditCheckout(deviceId, pkg.id)
      setPendingPurchaseId(res.purchase_id)
      setIframeUrl(res.iframe_url)
      setBuyOpen(false)
      setPayOpen(true)
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setCheckingOut(false)
    }
  }

  async function finishPayment() {
    if (!pendingPurchaseId) return
    const purchaseId = pendingPurchaseId
    const hide = message.loading(he.paymentConfirming, 0)
    // Webhook may lag the iframe TransactionResponse — poll confirm briefly.
    const deadline = Date.now() + 45_000
    let lastErr: string = he.paymentPending
    try {
      while (Date.now() < deadline) {
        try {
          await api.creditConfirm(deviceId, purchaseId)
          hide()
          message.success(he.paymentSuccess)
          setPayOpen(false)
          setIframeUrl('')
          setPendingPurchaseId('')
          await qc.invalidateQueries({ queryKey: ['credits', deviceId] })
          return
        } catch (err) {
          lastErr = (err as Error).message || he.paymentPending
          await new Promise((r) => setTimeout(r, 1200))
        }
      }
      hide()
      message.warning(lastErr)
    } catch {
      hide()
      message.error(lastErr)
    }
  }

  useEffect(() => {
    function onMessage(ev: MessageEvent) {
      const data = ev.data as {
        type?: string
        Name?: string
        Value?: { Status?: string; Message?: string }
        error?: string
      }
      if (!data || typeof data !== 'object') return
      if (data.type === 'nedarim-success' || data.Name === 'TransactionResponse') {
        if (data.Value?.Status === 'Error') {
          message.error(data.Value?.Message || he.paymentCancelled)
          return
        }
        void finishPayment()
        return
      }
      if (data.type === 'nedarim-cancel') {
        message.info(he.paymentCancelled)
        setPayOpen(false)
      }
      if (data.type === 'nedarim-error') {
        message.error(data.error || he.paymentCancelled)
        setPayOpen(false)
      }
    }
    window.addEventListener('message', onMessage)
    return () => window.removeEventListener('message', onMessage)
  }, [pendingPurchaseId, deviceId, message])

  const blocked =
    (category === 'access-url' && urlStatus === 'allowed') ||
    (category === 'access-app' && selected?.access_status === 'allowed')

  function fmtILS(agorot: number) {
    return he.priceILS.replace('{n}', (agorot / 100).toFixed(agorot % 100 === 0 ? 0 : 2))
  }

  function renderStoreApp(item: AppMeta) {
    const meta = getAppMeta(item.bundle_id, item) || item
    return (
      <List.Item
        key={item.bundle_id}
        className="store-app-row"
        actions={[
          <StoreAppActions
            key="act"
            app={{ ...meta, access_status: 'allowed' }}
            onRequest={() => void goRequestApp({ ...meta, access_status: 'allowed' })}
            onOpenPending={() => void goUpdatesForPending()}
          />,
        ]}
      >
        <List.Item.Meta
          title={<AppIdentity bundleId={item.bundle_id} meta={meta} size={40} />}
          description={
            <Space direction="vertical" size={2}>
              {meta.developer ? (
                <Typography.Text type="secondary">
                  {he.by} {meta.developer}
                </Typography.Text>
              ) : null}
              {statusTag('allowed')}
            </Space>
          }
        />
      </List.Item>
    )
  }

  return (
    <div className="page-shell">
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <div>
          <Typography.Title level={2} className="page-title" style={{ marginBottom: 8 }}>
            {isStoreHome ? he.portalStoreTitle : he.portalTitle}
          </Typography.Title>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
            {isStoreHome ? he.portalStoreLead : he.portalLead}
          </Typography.Paragraph>
          {!hideCreditsUI ? (
            <Flex gap={8} wrap="wrap" align="center">
              <Tag color={balance > 0 ? 'success' : 'default'}>
                {he.availableBalance}: {creditsQuery.isLoading ? '…' : balance}
              </Tag>
              {!hidePurchases ? (
                <Button size="small" onClick={() => setBuyOpen(true)}>
                  {he.buyCredits}
                </Button>
              ) : null}
            </Flex>
          ) : null}
        </div>

        <Segmented
          block
          size={isMobile ? 'small' : 'middle'}
          value={mode}
          onChange={(v) => void setMode(v as PortalMode)}
          options={[
            { value: 'store', label: he.portalTabStore },
            { value: 'request', label: he.portalTabRequest },
            {
              value: 'updates',
              label: (
                <Badge count={updatesBadge} size="small" offset={[8, -2]}>
                  <span>{he.portalTabUpdates}</span>
                </Badge>
              ),
            },
          ]}
        />

        {needsCredits && mode === 'request' && (
          <Alert
            type="warning"
            showIcon
            message={he.insufficientCredits}
            description={hidePurchases ? he.companionCreditsHint : he.insufficientCreditsHint}
            action={
              hidePurchases ? undefined : (
                <Button size="small" type="primary" onClick={() => setBuyOpen(true)}>
                  {he.buyCredits}
                </Button>
              )
            }
          />
        )}

        {mode === 'store' && (
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              {he.storeLead}
            </Typography.Paragraph>

            <Card size="small" title={he.storeAllowed}>
              {allowlistQuery.isLoading ? (
                <Skeleton active paragraph={{ rows: 2 }} />
              ) : (
                <SearchableCollection
                  items={allowedApps}
                  text={(app) => appMetaSearchText(getAppMeta(app.bundle_id, app) || app)}
                  emptyText={he.storeEmptyAllowed}
                >
                  {(apps) => (
                    <List
                      className="store-app-list"
                      dataSource={apps}
                      renderItem={renderStoreApp}
                    />
                  )}
                </SearchableCollection>
              )}
            </Card>

            <Card size="small" title={he.storeSearch}>
              <ListSearchBar
                value={storeQ}
                onChange={(v) => void setStoreQ(v)}
                placeholder="YouTube"
                style={{ marginBottom: 12 }}
              />
              {storeQ.trim() && storeQ.trim().length < 2 ? (
                <Typography.Text type="secondary">{he.searchMinChars}</Typography.Text>
              ) : null}
              {storeSearching && (
                <Flex gap={8} align="center" style={{ marginBottom: 8 }}>
                  <Spin size="small" />
                  <Typography.Text type="secondary">{he.searching}</Typography.Text>
                </Flex>
              )}
              {storeSearchQuery.isError ? (
                <Alert type="error" showIcon message={he.searchFailed} />
              ) : null}
              {storeSearched && !storeSearching && !storeSearchQuery.isError && !storeResults.length && (
                <Empty description={he.noApps} image={Empty.PRESENTED_IMAGE_SIMPLE} />
              )}
              <List
                className="store-app-list"
                dataSource={storeResults}
                loading={storeSearching && !storeResults.length}
                renderItem={renderStoreApp}
              />
            </Card>
          </Space>
        )}

        {mode === 'request' && (
          <Card>
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <div>
                <Typography.Text type="secondary">{he.category}</Typography.Text>
                <div style={{ marginTop: 8 }}>
                  <Segmented
                    block
                    size={isMobile ? 'small' : 'middle'}
                    value={category}
                    onChange={(v) => {
                      const next = v as Category
                      void setParams({
                        cat: next,
                        bundle: null,
                        q: '',
                        details: true,
                        ...(next !== 'access-url' ? { url: '' } : {}),
                      })
                      setSelectedCache(null)
                    }}
                    options={[
                      { value: 'access-url', label: he.catUrl },
                      { value: 'access-app', label: he.catApp },
                      { value: 'general', label: he.catGeneral },
                      { value: 'bug', label: he.catBug },
                    ]}
                  />
                </div>
                {isAccess && creditsEnabled ? (
                  <Typography.Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
                    {he.accessCostsCredits.replace('{n}', String(accessCost))}
                  </Typography.Text>
                ) : null}
              </div>

              {category === 'access-url' && (
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Typography.Text type="secondary">{he.url}</Typography.Text>
                  <Input
                    value={url}
                    onChange={(e) => void setParams({ url: e.target.value })}
                    placeholder="https://"
                  />
                  {urlPreview && (
                    <Typography.Text type="secondary">
                      {he.urlWillSave}: <Typography.Text code>{urlPreview}</Typography.Text>
                    </Typography.Text>
                  )}
                  {checking && <Typography.Text type="secondary">{he.checkStatus}</Typography.Text>}
                  {statusTag(urlStatus || undefined)}
                  {urlStatus === 'allowed' && (
                    <Alert type="success" showIcon message={he.alreadyAllowedHint} />
                  )}
                </Space>
              )}

              {category === 'access-app' && !bundleId && (
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Typography.Text type="secondary">{he.searchApp}</Typography.Text>
                  <Input
                    value={query}
                    onChange={(e) => void setParams({ q: e.target.value })}
                    placeholder="YouTube"
                    autoComplete="off"
                    allowClear
                  />
                  {query.trim() && query.trim().length < 2 ? (
                    <Typography.Text type="secondary">{he.searchMinChars}</Typography.Text>
                  ) : null}
                  {searching && (
                    <Flex gap={8} align="center">
                      <Spin size="small" />
                      <Typography.Text type="secondary">{he.searching}</Typography.Text>
                    </Flex>
                  )}
                  {searchQuery.isError ? (
                    <Alert type="error" showIcon message={he.searchFailed} />
                  ) : null}
                  {searched && !searching && !searchQuery.isError && results.length === 0 && (
                    <Empty description={he.noApps} image={Empty.PRESENTED_IMAGE_SIMPLE} />
                  )}
                  <List
                    dataSource={results}
                    loading={searching && !results.length}
                    renderItem={(item) => (
                      <List.Item
                        className="tap-row"
                        onClick={() => {
                          setSelectedCache(item)
                          void setParams({ bundle: item.bundle_id, details: true })
                        }}
                        actions={
                          isMobile
                            ? undefined
                            : [
                                <Button key="pick" type="link">
                                  {he.pick}
                                </Button>,
                              ]
                        }
                      >
                        <List.Item.Meta
                          avatar={<AppThumb name={item.app_name} url={item.artwork_url} />}
                          title={item.app_name}
                          description={
                            <Space direction="vertical" size={4}>
                              {item.developer ? `${he.by} ${item.developer}` : null}
                              {item.access_status && item.access_status !== 'none'
                                ? statusTag(item.access_status)
                                : null}
                            </Space>
                          }
                        />
                      </List.Item>
                    )}
                  />
                </Space>
              )}

              {category === 'access-app' && selected && bundleId && (
                <div className="app-selected">
                  <Flex gap={12} align="start" justify="space-between" wrap="wrap">
                    <Flex gap={12} align="center">
                      <AppThumb name={selected.app_name} url={selected.artwork_url} size={56} />
                      <div>
                        <Typography.Text strong>{selected.app_name}</Typography.Text>
                        <div>
                          <Typography.Text type="secondary">
                            {selected.developer ? `${he.by} ${selected.developer}` : ''}
                          </Typography.Text>
                        </div>
                        {statusTag(selected.access_status)}
                      </div>
                    </Flex>
                    <Space>
                      {selected.access_status === 'allowed' ? (
                        <Button
                          type="primary"
                          onClick={() =>
                            openAppStore(selected, () => {
                              message.info(he.storeInstallViaChrome)
                            })
                          }
                        >
                          {he.storeInstall}
                        </Button>
                      ) : null}
                      <Button
                        onClick={() => {
                          setSelectedCache(null)
                          void setParams({ bundle: null, details: true })
                        }}
                      >
                        {he.change}
                      </Button>
                    </Space>
                  </Flex>
                  {selected.access_status === 'allowed' && (
                    <Alert
                      style={{ marginTop: 12 }}
                      type="success"
                      showIcon
                      message={he.alreadyAllowedHint}
                    />
                  )}
                  {detailsOpen && (
                    <Spin spinning={detailsQuery.isFetching && !detailsQuery.data}>
                      <AppDetailsPanel app={selected} loading={detailsQuery.isFetching} />
                    </Spin>
                  )}
                </div>
              )}

              {(category === 'general' || category === 'bug') && (
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Typography.Text type="secondary">
                    {category === 'bug' ? he.bugTitle : he.subject}
                  </Typography.Text>
                  <Input value={subject} onChange={(e) => setSubject(e.target.value)} />
                </Space>
              )}

              {!blocked && (
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Typography.Text type="secondary">{reasonLabel}</Typography.Text>
                  <Input.TextArea
                    value={reason}
                    onChange={(e) => setReason(e.target.value)}
                    rows={3}
                  />
                  <Button type="primary" block loading={submitting} onClick={() => void submit()}>
                    {he.submit}
                  </Button>
                </Space>
              )}
            </Space>
          </Card>
        )}

        {mode === 'updates' && (
          <div ref={historyRef}>
            <Typography.Paragraph type="secondary">{he.updatesLead}</Typography.Paragraph>
            {mineQuery.isLoading && !mine.length ? (
              <Skeleton active paragraph={{ rows: 3 }} />
            ) : (
            <Spin spinning={mineQuery.isFetching && !!mine.length}>
              <SearchableCollection items={mine} text={requestSearchText} emptyText={he.noRequests}>
                {(rows) => (
              <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                {rows.map((r) => {
                  const next = studentNextAction(r.type, r.status, r.last_message?.author_role)
                  const isNew = requestActivityAt(r) > updatesSeenAt - 1 && r.status !== 'pending'
                  return (
                    <Card
                      key={r.id}
                      className="request-card"
                      size="small"
                      style={
                        highlight === r.id
                          ? { outline: '2px solid #0b6e4f', outlineOffset: 2 }
                          : undefined
                      }
                      title={
                        <Flex gap={10} align="center" className="card-title-wrap">
                          {r.type === 'access' && r.kind === 'app' ? (
                            <AppIdentity bundleId={r.value} meta={r.app} size={32} compact />
                          ) : (
                            <span>{r.value}</span>
                          )}
                          {isNew ? <Tag color="blue">{he.updatesBadgeNew}</Tag> : null}
                        </Flex>
                      }
                      extra={<Tag color={nextTagColor(next.kind, r.status)}>{next.label}</Tag>}
                    >
                      <Typography.Text type="secondary">
                        {fmtTime(r.created_at)} · {he.typeLabel[r.type] || r.type}
                      </Typography.Text>
                      {r.last_message?.body ? (
                        <div className="inbox-snip">{r.last_message.body}</div>
                      ) : null}
                      <RequestThread
                        requestId={r.id}
                        role="student"
                        deviceId={deviceId}
                        closed={r.status !== 'pending'}
                        onPosted={() => {
                          void qc.invalidateQueries({ queryKey: ['my-requests', deviceId] })
                        }}
                      />
                    </Card>
                  )
                })}
              </Space>
                )}
              </SearchableCollection>
            </Spin>
            )}
          </div>
        )}
      </Space>

      <Modal
        title={he.buyCreditsTitle}
        open={buyOpen && !hidePurchases}
        onCancel={() => setBuyOpen(false)}
        footer={null}
        destroyOnHidden
        width={isMobile ? '100%' : 520}
        centered={!isMobile}
      >
        <Typography.Paragraph type="secondary">{he.choosePackage}</Typography.Paragraph>
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          {(packagesQuery.data ?? []).map((pkg) => (
            <Card key={pkg.id} size="small">
              <Flex
                justify="space-between"
                align={isMobile ? 'stretch' : 'center'}
                gap={12}
                vertical={isMobile}
              >
                <div>
                  <Typography.Text strong>{pkg.name_he}</Typography.Text>
                  <div>
                    <Typography.Text type="secondary">{fmtILS(pkg.price_agorot)}</Typography.Text>
                  </div>
                </div>
                <Button
                  type="primary"
                  block={isMobile}
                  loading={checkingOut}
                  onClick={() => void startCheckout(pkg)}
                >
                  {he.payNow}
                </Button>
              </Flex>
            </Card>
          ))}
          {packagesQuery.isLoading && <Spin />}
          {!packagesQuery.isLoading && !(packagesQuery.data ?? []).length && (
            <Empty description={he.choosePackage} />
          )}
        </Space>
      </Modal>

      <Modal
        title={he.nedarimPay}
        open={payOpen && !hidePurchases}
        onCancel={() => setPayOpen(false)}
        footer={null}
        width={isMobile ? '100%' : 480}
        centered={!isMobile}
        destroyOnHidden
      >
        {iframeUrl ? (
          <iframe
            title="nedarim"
            src={iframeUrl}
            style={{
              width: '100%',
              minHeight: isMobile ? 'min(60vh, 420px)' : 420,
              border: '1px solid #d7e5dd',
              borderRadius: 8,
            }}
          />
        ) : (
          <Spin />
        )}
      </Modal>
    </div>
  )
}
