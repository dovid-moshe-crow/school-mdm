import {
  Alert,
  App,
  Button,
  Card,
  Collapse,
  Empty,
  Flex,
  Input,
  List,
  Rate,
  Segmented,
  Skeleton,
  Space,
  Spin,
  Tag,
  Typography,
} from 'antd'
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  parseAsBoolean,
  parseAsString,
  parseAsStringLiteral,
  useQueryState,
  useQueryStates,
} from 'nuqs'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, type AccessStatus, type AppMeta } from '../api'
import { RequestThread } from '../components/RequestThread'
import { useIsMobile } from '../hooks/useIsMobile'
import { he, studentNextAction } from '../he'
import { AppThumb, normalizeHostPreview, useDebounced } from '../ui'

const categories = ['access-url', 'access-app', 'general', 'bug'] as const
type Category = (typeof categories)[number]

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

function AppDetailsPanel({
  app,
  loading,
}: {
  app: AppMeta
  loading: boolean
}) {
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

      {app.store_url && (
        <Button type="link" href={app.store_url} target="_blank" rel="noreferrer" style={{ paddingInline: 0 }}>
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
                  {he.bundleId}: <Typography.Text code copyable>{app.bundle_id}</Typography.Text>
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

export default function Portal() {
  const { message } = App.useApp()
  const { deviceId = '' } = useParams()
  const qc = useQueryClient()
  const historyRef = useRef<HTMLDivElement>(null)

  const [params, setParams] = useQueryStates({
    cat: parseAsStringLiteral(categories),
    url: parseAsString.withDefault(''),
    q: parseAsString.withDefault(''),
    bundle: parseAsString,
    details: parseAsBoolean.withDefault(true),
  })
  const [highlight, setHighlight] = useQueryState('highlight', parseAsString)

  // Default category: access-url when ?url= is present (legacy deep-link), else access-app.
  const category: Category = params.cat ?? (params.url ? 'access-url' : 'access-app')
  const url = params.url
  const query = params.q
  const bundleId = params.bundle
  const detailsOpen = params.details

  const [subject, setSubject] = useState('')
  const [reason, setReason] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [selectedCache, setSelectedCache] = useState<AppMeta | null>(null)
  const debouncedQ = useDebounced(query, 150)
  const debouncedUrl = useDebounced(url, 350)

  const reasonLabel = useMemo(() => {
    if (category === 'access-url') return he.reasonUrl
    if (category === 'access-app') return he.reasonApp
    if (category === 'bug') return he.reasonBug
    return he.reasonGeneral
  }, [category])

  const urlPreview = useMemo(() => normalizeHostPreview(url), [url])

  const mineQuery = useQuery({
    queryKey: ['my-requests', deviceId],
    queryFn: () => api.myRequests(deviceId),
    enabled: !!deviceId,
    refetchInterval: 10_000,
  })
  const mine = mineQuery.data ?? []

  const urlStatusQuery = useQuery({
    queryKey: ['access-status', deviceId, 'url', debouncedUrl],
    queryFn: () => api.accessStatus(deviceId, 'url', debouncedUrl.trim()),
    enabled: category === 'access-url' && !!debouncedUrl.trim() && !!deviceId,
  })
  const urlStatus = urlStatusQuery.data?.status ?? null
  const checking = urlStatusQuery.isFetching

  const searchQuery = useQuery({
    queryKey: ['app-search', deviceId, debouncedQ],
    queryFn: () => api.searchApps(debouncedQ.trim(), deviceId),
    enabled: category === 'access-app' && !bundleId && !!debouncedQ.trim(),
    placeholderData: keepPreviousData,
  })
  const results = searchQuery.data ?? []
  const searching = searchQuery.isFetching
  const searched = searchQuery.isFetched && !!debouncedQ.trim()

  const detailsQuery = useQuery({
    queryKey: ['app-lookup', bundleId, deviceId],
    queryFn: () =>
      api.lookupApp(bundleId!, { refresh: true, enrollmentID: deviceId }),
    enabled: category === 'access-app' && !!bundleId && !!deviceId,
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

  async function submit() {
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
      await qc.invalidateQueries({ queryKey: ['my-requests', deviceId] })
      requestAnimationFrame(() => historyRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  const blocked =
    (category === 'access-url' && urlStatus === 'allowed') ||
    (category === 'access-app' && selected?.access_status === 'allowed')

  const isMobile = useIsMobile()

  return (
    <div className="page-shell">
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <div>
          <Typography.Title level={2} className="page-title" style={{ marginBottom: 8 }}>
            {he.portalTitle}
          </Typography.Title>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
            {he.portalLead}
          </Typography.Paragraph>
          <Tag style={{ maxWidth: '100%', whiteSpace: 'normal', height: 'auto' }}>
            {he.device}{' '}
            <Typography.Text code style={{ wordBreak: 'break-all' }}>
              {deviceId}
            </Typography.Text>
          </Tag>
        </div>

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
                {searching && (
                  <Flex gap={8} align="center">
                    <Spin size="small" />
                    <Typography.Text type="secondary">{he.searching}</Typography.Text>
                  </Flex>
                )}
                {searched && !searching && results.length === 0 && (
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
                            {isMobile && (
                              <Typography.Text type="secondary">{he.pick}</Typography.Text>
                            )}
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
                    <Button
                      type="link"
                      onClick={() => void setParams({ details: !detailsOpen })}
                      style={{ paddingInline: 0 }}
                    >
                      {detailsOpen ? he.hideDetails : he.showDetails}
                    </Button>
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
                    <AppDetailsPanel
                      app={selected}
                      loading={detailsQuery.isFetching}
                    />
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
                <Button type="primary" block loading={submitting} onClick={submit}>
                  {he.submit}
                </Button>
              </Space>
            )}
          </Space>
        </Card>

        <div ref={historyRef}>
          <Typography.Title level={4}>{he.myRequests}</Typography.Title>
          {mineQuery.isLoading && !mine.length && (
            <Skeleton active paragraph={{ rows: 3 }} />
          )}
          {!mineQuery.isLoading && !mine.length && <Empty description={he.noRequests} />}
          <Spin spinning={mineQuery.isFetching && !!mine.length}>
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              {mine.map((r) => {
                const next = studentNextAction(r.type, r.status, r.last_message?.author_role)
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
                        {r.type === 'access' && r.kind === 'app' && (
                          <AppThumb name={r.app?.app_name || r.value} url={r.app?.artwork_url} size={32} />
                        )}
                        <span>{r.app?.app_name || r.value}</span>
                      </Flex>
                    }
                    extra={
                      <Tag color={nextTagColor(next.kind, r.status)}>{next.label}</Tag>
                    }
                  >
                    <Typography.Text type="secondary">
                      {fmtTime(r.created_at)} · {he.typeLabel[r.type] || r.type}
                    </Typography.Text>
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
          </Spin>
        </div>
      </Space>
    </div>
  )
}
