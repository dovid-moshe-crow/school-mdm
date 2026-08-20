import { Card, Switch, Typography } from 'antd'
import { App } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type SystemAllowlistItem } from '../api'
import { AppIdentity } from '../appMeta'
import { he } from '../he'
import { appTitle } from '../knownApps'
import { ListSearchBar } from './ListSearch'
import { VirtualList } from './VirtualList'
import { useListSearch } from '../hooks/useListSearch'

export function SystemAllowlistSettings() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const query = useQuery({
    queryKey: ['system-allowlist'],
    queryFn: () => api.systemAllowlist(),
  })
  const items = query.data ?? []
  const search = useListSearch(items, (it) => `${appTitle(undefined, it.value)} ${it.value}`)
  const listing = search.query.trim().length >= 2

  const toggle = useMutation({
    mutationFn: ({ item, enabled }: { item: SystemAllowlistItem; enabled: boolean }) =>
      api.setSystemAllowlistEnabled(item.kind || 'app', item.value, enabled),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['system-allowlist'] })
    },
    onError: (err) => message.error((err as Error).message),
  })

  return (
    <Card size="small" title={he.systemAppsTitle}>
      <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
        {he.systemAppsLead}
      </Typography.Paragraph>
      {query.isError ? (
        <Typography.Text type="danger">{(query.error as Error).message}</Typography.Text>
      ) : (
        <>
          <ListSearchBar
            value={search.query}
            onChange={search.setQuery}
            placeholder={he.effectiveSearch}
            total={items.length}
            shown={listing ? search.visible.length : undefined}
          />
          {!listing ? (
            <Typography.Text type="secondary">{he.systemAppsSearchHint}</Typography.Text>
          ) : (
            <div style={{ marginTop: 8 }}>
              <VirtualList
                items={search.visible}
                rowHeight={52}
                height={360}
                itemKey={(it) => it.value}
                renderRow={(it) => (
                  <div className="virtual-list-row-inner">
                    <AppIdentity bundleId={it.value} size={32} />
                    <Switch
                      checked={it.enabled}
                      checkedChildren={he.systemAppsEnabled}
                      unCheckedChildren={he.systemAppsDisabled}
                      loading={toggle.isPending && toggle.variables?.item.value === it.value}
                      onChange={(on) => toggle.mutate({ item: it, enabled: on })}
                    />
                  </div>
                )}
              />
            </div>
          )}
        </>
      )}
    </Card>
  )
}
