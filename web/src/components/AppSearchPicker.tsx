import { Alert, Button, Empty, Flex, Input, List, Spin, Typography } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { api, type AppMeta } from '../api'
import { he } from '../he'
import { AppThumb, useDebounced } from '../ui'

export function AppSearchPicker({
  onPick,
  pickLabel = he.pick,
  enrollmentId,
  autoFocus,
}: {
  onPick: (app: AppMeta) => void
  pickLabel?: string
  enrollmentId?: string
  autoFocus?: boolean
}) {
  const [q, setQ] = useState('')
  const debounced = useDebounced(q.trim(), 320)
  const enabled = debounced.length >= 2
  const query = useQuery({
    queryKey: ['app-search', 'picker', enrollmentId || '', debounced],
    queryFn: () => api.searchApps(debounced, enrollmentId),
    enabled,
  })
  const results = query.data ?? []
  const searched = enabled && query.isFetched && !query.isFetching

  return (
    <div>
      <Input
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder={he.searchApp}
        allowClear
        autoFocus={autoFocus}
        autoComplete="off"
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
          style={{ marginTop: 8 }}
          dataSource={results}
          renderItem={(app) => (
            <List.Item
              className="tap-row"
              actions={[
                <Button key="pick" type="link" onClick={() => onPick(app)}>
                  {pickLabel}
                </Button>,
              ]}
            >
              <List.Item.Meta
                avatar={<AppThumb name={app.app_name} url={app.artwork_url} />}
                title={app.app_name}
                description={app.developer ? `${app.developer} · ${app.bundle_id}` : app.bundle_id}
              />
            </List.Item>
          )}
        />
      ) : null}
    </div>
  )
}
