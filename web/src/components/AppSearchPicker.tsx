import { Alert, Button, Empty, Flex, List, Spin, Tag, Typography } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { api, type AppMeta } from '../api'
import { he } from '../he'
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
  const [adding, setAdding] = useState('')
  const debounced = useDebounced(q.trim(), 320)
  const enabled = debounced.length >= 2
  const query = useQuery({
    queryKey: ['app-search', 'picker', enrollmentId || '', debounced],
    queryFn: () => api.searchApps(debounced, enrollmentId),
    enabled,
  })
  const results = query.data ?? []
  const searched = enabled && query.isFetched && !query.isFetching
  const excluded = excludeBundles instanceof Set ? excludeBundles : new Set(excludeBundles || [])

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
      {!enabled && q.trim() ? (
        <Typography.Text type="secondary">{he.searchMinChars}</Typography.Text>
      ) : null}
      {query.isFetching ? (
        <Flex gap={8} align="center" style={{ marginTop: 8 }}>
          <Spin size="small" />
          <Typography.Text type="secondary">{he.searching}</Typography.Text>
        </Flex>
      ) : null}
      {query.isError ? (
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
            const inList = excluded.has(app.bundle_id)
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
                      onClick={() => {
                        setAdding(app.bundle_id)
                        void Promise.resolve(onPick(app)).finally(() => setAdding(''))
                      }}
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
                    app.developer ? (
                      <Typography.Text type="secondary">{app.developer}</Typography.Text>
                    ) : null
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
