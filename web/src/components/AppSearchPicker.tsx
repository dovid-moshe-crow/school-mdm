import { Alert, Button, Empty, Flex, Input, List, Spin, Tag, Typography } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { api, type AppMeta } from '../api'
import { he } from '../he'
import { knownAppName, knownAppsMatching, looksLikeBundleId } from '../knownApps'
import { AppThumb, useDebounced } from '../ui'
import { ListSearchBar } from './ListSearch'

export function AppSearchPicker({
  onPick,
  pickLabel = he.pick,
  enrollmentId,
  autoFocus,
  excludeBundles,
  placeholder,
  hint,
}: {
  onPick: (app: AppMeta) => void | Promise<void>
  pickLabel?: string
  enrollmentId?: string
  autoFocus?: boolean
  excludeBundles?: Set<string> | string[]
  placeholder?: string
  hint?: string
}) {
  const [q, setQ] = useState('')
  const [bundleDraft, setBundleDraft] = useState('')
  const [adding, setAdding] = useState('')
  const debounced = useDebounced(q.trim(), 320)
  const enabled = debounced.length >= 2
  const query = useQuery({
    queryKey: ['app-search', 'picker', enrollmentId || '', debounced],
    queryFn: () => api.searchApps(debounced, enrollmentId),
    enabled,
  })
  const excluded = excludeBundles instanceof Set ? excludeBundles : new Set(excludeBundles || [])
  const results = useMemo(() => {
    const remote = query.data ?? []
    const local = knownAppsMatching(debounced)
    const seen = new Set(remote.map((a) => a.bundle_id.toLowerCase()))
    return [...local.filter((a) => !seen.has(a.bundle_id.toLowerCase())), ...remote]
  }, [query.data, debounced])
  const searched = enabled && query.isFetched && !query.isFetching

  async function pick(app: AppMeta) {
    setAdding(app.bundle_id)
    try {
      await Promise.resolve(onPick(app))
    } finally {
      setAdding('')
    }
  }

  function pickBundle(raw: string) {
    const id = raw.trim()
    if (!id) return
    void pick({
      bundle_id: id,
      app_name: knownAppName(id) || id,
      developer: '',
      source: 'local',
    })
  }

  return (
    <div className="app-search-picker">
      {hint ? (
        <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
          {hint}
        </Typography.Paragraph>
      ) : null}
      <ListSearchBar
        value={q}
        onChange={setQ}
        placeholder={placeholder || he.searchApp}
        autoFocus={autoFocus}
      />
      <Flex gap={8} wrap="wrap" style={{ marginTop: 8 }} align="center">
        <Input
          dir="ltr"
          style={{ flex: 1, minWidth: 180 }}
          value={bundleDraft}
          onChange={(e) => setBundleDraft(e.target.value)}
          placeholder={he.bundleIdPlaceholder}
          onPressEnter={() => pickBundle(bundleDraft)}
        />
        <Button
          size="small"
          disabled={!looksLikeBundleId(bundleDraft) || !!adding}
          loading={adding === bundleDraft.trim()}
          onClick={() => pickBundle(bundleDraft)}
        >
          {he.addByBundleId}
        </Button>
      </Flex>
      <Typography.Text type="secondary" style={{ display: 'block', marginTop: 4 }}>
        {he.bundleIdHint}
      </Typography.Text>
      {!enabled && q.trim() ? (
        <Typography.Text type="secondary">{he.searchMinChars}</Typography.Text>
      ) : null}
      {query.isFetching ? (
        <Flex gap={8} align="center" style={{ marginTop: 8 }}>
          <Spin size="small" />
          <Typography.Text type="secondary">{he.searching}</Typography.Text>
        </Flex>
      ) : null}
      {query.isError && results.length === 0 ? (
        <Alert
          style={{ marginTop: 8 }}
          type="error"
          showIcon
          message={he.searchFailed}
          description={(query.error as Error).message}
        />
      ) : null}
      {searched && !query.isError && results.length === 0 ? (
        <Empty
          style={{ marginTop: 8 }}
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={he.noApps}
        />
      ) : null}
      {results.length ? (
        <List
          size="small"
          className="app-search-picker-results"
          dataSource={results}
          renderItem={(app) => {
            const inList = excluded.has(app.bundle_id) || excluded.has(app.bundle_id.toLowerCase())
            const busy = adding === app.bundle_id
            return (
              <List.Item
                className="tap-row"
                actions={[
                  inList ? (
                    <Tag key="in" color="green">
                      {he.packInPack}
                    </Tag>
                  ) : (
                    <Button
                      key="pick"
                      type="link"
                      loading={busy}
                      disabled={!!adding}
                      onClick={() => void pick(app)}
                    >
                      {pickLabel}
                    </Button>
                  ),
                ]}
              >
                <List.Item.Meta
                  avatar={<AppThumb name={app.app_name} url={app.artwork_url} load />}
                  title={app.app_name}
                  description={
                    <Typography.Text type="secondary" dir="ltr">
                      {app.developer ? `${app.developer} · ` : ''}
                      {app.bundle_id}
                    </Typography.Text>
                  }
                />
              </List.Item>
            )
          }}
        />
      ) : null}
    </div>
  )
}
