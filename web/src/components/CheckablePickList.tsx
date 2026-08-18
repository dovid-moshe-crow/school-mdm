import { MobileOutlined } from '@ant-design/icons'
import { Button, Checkbox, Empty, Flex, Segmented, Space, Tag, Typography } from 'antd'
import { useMemo, useState, type ReactNode } from 'react'
import { he } from '../he'
import { matchesQuery } from '../search'
import { formatRelativeHe } from '../time'
import { ListSearchBar } from './ListSearch'

export type CheckablePickItem = {
  key: string
  title: ReactNode
  description?: ReactNode
  searchText: string
  extra?: ReactNode
  avatar?: ReactNode
}

export function CheckablePickList({
  items,
  selectedKeys,
  onChange,
  placeholder,
  emptyText,
  maxHeight = 380,
  filterLabels,
}: {
  items: CheckablePickItem[]
  selectedKeys: string[]
  onChange: (keys: string[]) => void
  placeholder?: string
  emptyText?: string
  maxHeight?: number | string
  filterLabels?: { all: string; on: string; off: string }
}) {
  const selected = useMemo(() => new Set(selectedKeys), [selectedKeys])
  const [membership, setMembership] = useState<'all' | 'on' | 'off'>('all')
  const [query, setQuery] = useState('')

  const scoped = useMemo(() => {
    if (membership === 'on') return items.filter((it) => selected.has(it.key))
    if (membership === 'off') return items.filter((it) => !selected.has(it.key))
    return items
  }, [items, membership, selected])

  const visible = useMemo(
    () => scoped.filter((it) => matchesQuery(query, it.searchText)),
    [scoped, query],
  )

  const visibleKeys = visible.map((it) => it.key)
  const allVisibleSelected =
    visibleKeys.length > 0 && visibleKeys.every((k) => selected.has(k))

  function toggle(key: string, checked: boolean) {
    if (checked) {
      if (selected.has(key)) return
      onChange([...selectedKeys, key])
      return
    }
    onChange(selectedKeys.filter((k) => k !== key))
  }

  function selectVisible(on: boolean) {
    if (on) {
      const next = new Set(selectedKeys)
      for (const k of visibleKeys) next.add(k)
      onChange([...next])
      return
    }
    const drop = new Set(visibleKeys)
    onChange(selectedKeys.filter((k) => !drop.has(k)))
  }

  const labels = filterLabels || {
    all: he.pickFilterAll,
    on: he.pickFilterOn,
    off: he.pickFilterOff,
  }

  const selectedItems = items.filter((it) => selected.has(it.key))

  return (
    <div className="pick-list">
      <ListSearchBar
        value={query}
        onChange={setQuery}
        placeholder={placeholder}
        total={scoped.length}
        shown={visible.length}
      />
      <Flex justify="space-between" align="center" gap={8} wrap="wrap" className="pick-list-toolbar">
        <Segmented
          size="small"
          value={membership}
          onChange={(v) => setMembership(v as 'all' | 'on' | 'off')}
          options={[
            { value: 'all', label: `${labels.all} (${items.length})` },
            { value: 'on', label: `${labels.on} (${selected.size})` },
            { value: 'off', label: `${labels.off} (${items.length - selected.size})` },
          ]}
        />
        <Space size={4} wrap>
          <Button
            size="small"
            type="link"
            disabled={!visible.length || allVisibleSelected}
            onClick={() => selectVisible(true)}
          >
            {he.pickAllVisible}
          </Button>
          <Button
            size="small"
            type="link"
            disabled={!visible.length || !visibleKeys.some((k) => selected.has(k))}
            onClick={() => selectVisible(false)}
          >
            {he.clearVisible}
          </Button>
        </Space>
      </Flex>

      {selectedItems.length > 0 && membership !== 'on' ? (
        <div className="pick-list-chips">
          {selectedItems.slice(0, 8).map((it) => (
            <Tag
              key={it.key}
              closable
              onClose={(e) => {
                e.preventDefault()
                toggle(it.key, false)
              }}
            >
              {typeof it.title === 'string' ? it.title : it.key}
            </Tag>
          ))}
          {selectedItems.length > 8 ? (
            <Tag>
              {he.selectedDevices}: {selectedItems.length}
            </Tag>
          ) : null}
        </div>
      ) : null}

      {!items.length ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyText} />
      ) : !visible.length ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={he.noMatches} />
      ) : (
        <div className="pick-list-scroll" style={{ maxHeight }}>
          {visible.map((it) => {
            const checked = selected.has(it.key)
            return (
              <label
                key={it.key}
                className={checked ? 'pick-list-row is-checked' : 'pick-list-row'}
              >
                <Checkbox
                  checked={checked}
                  onChange={(e) => toggle(it.key, e.target.checked)}
                />
                <span className="pick-list-avatar">{it.avatar ?? <MobileOutlined />}</span>
                <span className="pick-list-meta">
                  <span className="pick-list-title">{it.title}</span>
                  {it.description ? (
                    <span className="pick-list-desc">{it.description}</span>
                  ) : null}
                </span>
                {it.extra ? <span className="pick-list-extra">{it.extra}</span> : null}
              </label>
            )
          })}
        </div>
      )}
    </div>
  )
}

export function DevicePickList({
  devices,
  selectedKeys,
  onChange,
  groupNameById,
  placeholder,
}: {
  devices: Array<{
    enrollment_id: string
    name: string
    serial_number?: string
    last_seen_at?: string
    mdm?: boolean
    group_ids?: string[]
  }>
  selectedKeys: string[]
  onChange: (keys: string[]) => void
  groupNameById?: (id: string) => string | undefined
  placeholder?: string
}) {
  const items: CheckablePickItem[] = devices.map((d) => {
    const title = d.name || d.serial_number || d.enrollment_id
    const groups = (d.group_ids || [])
      .map((id) => groupNameById?.(id) || '')
      .filter(Boolean)
    return {
      key: d.enrollment_id,
      title,
      searchText: [d.name, d.serial_number, d.enrollment_id, ...groups].filter(Boolean).join(' '),
      description: (
        <Typography.Text type="secondary" className="pick-list-desc-text">
          {[
            d.serial_number,
            d.mdm ? he.managed : '',
            d.last_seen_at ? formatRelativeHe(d.last_seen_at) : '',
          ]
            .filter(Boolean)
            .join(' · ')}
        </Typography.Text>
      ),
      extra: groups.length ? (
        <Space size={[4, 4]} wrap>
          {groups.slice(0, 3).map((name) => (
            <Tag key={name}>{name}</Tag>
          ))}
        </Space>
      ) : undefined,
    }
  })

  return (
    <CheckablePickList
      items={items}
      selectedKeys={selectedKeys}
      onChange={onChange}
      placeholder={placeholder || he.searchDevices}
      emptyText={he.emptyDevices}
      filterLabels={{
        all: he.pickFilterAll,
        on: he.pickFilterInGroup,
        off: he.pickFilterOutGroup,
      }}
    />
  )
}
