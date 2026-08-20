import { Collapse, Empty, Space, Tag, Typography } from 'antd'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import type { Allowance, AppMeta, Group } from '../api'
import { AppIdentity, useAppMetaStore } from '../appMeta'
import { he } from '../he'
import { appTitle } from '../knownApps'
import { matchesQuery } from '../search'
import { ListSearchBar } from './ListSearch'
import { VirtualList } from './VirtualList'

type Bucket = {
  key: string
  title: string
  items: Allowance[]
}

export function EffectivePolicyView({
  rows,
  groups,
}: {
  rows: Allowance[]
  groups: Group[]
}) {
  const { get } = useAppMetaStore()
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState<string[]>(['device'])

  const groupName = (id?: string) => groups.find((g) => g.id === id)?.name || id || ''
  const buckets = useMemo(() => buildBuckets(rows, groupName), [rows, groups])

  const visible = useMemo(() => {
    const q = query.trim()
    return buckets
      .map((bucket) => {
        const shown = q
          ? matchesQuery(q, bucket.title)
            ? bucket.items
            : bucket.items.filter((row) => itemMatches(q, row, get, groupName(row.group_id)))
          : bucket.items
        return { ...bucket, shown }
      })
      .filter((bucket) => bucket.shown.length > 0)
  }, [buckets, query, get, groups])

  useEffect(() => {
    if (!query.trim()) {
      setOpen(buckets.some((b) => b.key === 'device' && b.items.length > 0 && b.items.length <= 16) ? ['device'] : [])
      return
    }
    setOpen(visible.map((b) => b.key))
    // Only react to the search string so expanding a group is not undone when names load.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query])

  const shownCount = visible.reduce((n, b) => n + b.shown.length, 0)

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <ListSearchBar
        value={query}
        onChange={setQuery}
        placeholder={he.effectiveSearch}
        total={rows.length}
        shown={query.trim() ? shownCount : undefined}
      />
      {!rows.length ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={he.emptyAllow} />
      ) : !visible.length ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={he.effectiveEmptySearch} />
      ) : (
        <Collapse
          activeKey={open}
          onChange={(keys) => setOpen(Array.isArray(keys) ? keys : [keys])}
          items={visible.map((bucket) => ({
            key: bucket.key,
            label: bucket.title,
            extra: (
              <Typography.Text type="secondary">
                {he.effectiveItemCount.replace('{n}', String(bucket.shown.length))}
              </Typography.Text>
            ),
            children: (
              <VirtualList
                items={bucket.shown}
                rowHeight={52}
                height={Math.min(360, Math.max(52, bucket.shown.length * 52))}
                itemKey={(row, i) => `${row.kind}:${row.value}:${row.source}:${i}`}
                renderRow={(row) =>
                  row.kind === 'app' ? (
                    <div className="virtual-list-row-inner">
                      <AppIdentity bundleId={row.value} meta={row.app} size={32} compact />
                      {packTag(row)}
                    </div>
                  ) : (
                    <div className="virtual-list-row-inner">
                      <Typography.Text ellipsis>{row.value}</Typography.Text>
                      {packTag(row)}
                    </div>
                  )
                }
              />
            ),
          }))}
        />
      )}
    </Space>
  )
}

function packTag(row: Allowance): ReactNode {
  if (!row.pack_name) return null
  return <Tag>{row.pack_name}</Tag>
}

function itemMatches(
  q: string,
  row: Allowance,
  get: (bundleId?: string, fallback?: AppMeta | null) => AppMeta | undefined,
  group: string,
) {
  return matchesQuery(
    q,
    appTitle(get(row.value, row.app), row.value),
    row.value,
    row.kind,
    row.source,
    row.pack_name,
    group,
  )
}

function buildBuckets(rows: Allowance[], groupName: (id?: string) => string): Bucket[] {
  const byKey = new Map<string, Bucket>()
  const ensure = (key: string, title: string) => {
    let b = byKey.get(key)
    if (!b) {
      b = { key, title, items: [] }
      byKey.set(key, b)
    }
    return b
  }

  for (const row of rows) {
    if (row.source === 'device' || (row.source === 'pack' && row.target_type === 'device')) {
      if (row.source === 'pack') {
        ensure(`device-pack:${row.pack_id || row.pack_name}`, `${he.effectivePackOnDevice} · ${row.pack_name || he.sourcePack}`).items.push(row)
      } else {
        ensure('device', he.effectiveDeviceBucket).items.push(row)
      }
      continue
    }
    if (row.group_id || row.source === 'group' || (row.source === 'pack' && row.target_type === 'group')) {
      const gid = row.group_id || row.target_id || 'group'
      const packPart = row.pack_name ? ` · ${row.pack_name}` : ''
      const key = row.pack_id ? `group:${gid}:pack:${row.pack_id}` : `group:${gid}`
      ensure(key, `${groupName(gid)}${packPart}`).items.push(row)
      continue
    }
    if (row.source === 'essential') {
      ensure('essential', he.sourceEssential).items.push(row)
      continue
    }
    if (row.source === 'grant') {
      ensure('grant', he.sourceGrant).items.push(row)
      continue
    }
    if (row.source === 'pack') {
      ensure(`pack:${row.pack_id || row.pack_name}`, row.pack_name || he.sourcePack).items.push(row)
      continue
    }
    ensure('global', he.effectiveEveryone).items.push(row)
  }

  const rank = (key: string) => {
    if (key === 'device' || key.startsWith('device-pack:')) return 0
    if (key.startsWith('group:')) return 1
    if (key === 'essential') return 2
    if (key === 'grant') return 3
    return 4
  }
  return [...byKey.values()].sort((a, b) => {
    const d = rank(a.key) - rank(b.key)
    if (d) return d
    return a.title.localeCompare(b.title, 'he')
  })
}
