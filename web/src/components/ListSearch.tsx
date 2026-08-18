import { SearchOutlined } from '@ant-design/icons'
import { Empty, Input, Typography } from 'antd'
import type { CSSProperties, ReactNode } from 'react'
import { he } from '../he'
import { useListSearch } from '../hooks/useListSearch'

export function ListSearchBar({
  value,
  onChange,
  placeholder,
  total,
  shown,
  autoFocus,
  className,
  style,
}: {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  total?: number
  shown?: number
  autoFocus?: boolean
  className?: string
  style?: CSSProperties
}) {
  const counting = total != null && total > 0
  const suffix =
    counting && value.trim() ? (
      <Typography.Text type="secondary" style={{ fontSize: 12, whiteSpace: 'nowrap' }}>
        {he.searchCount
          .replace('{shown}', String(shown ?? 0))
          .replace('{total}', String(total))}
      </Typography.Text>
    ) : counting ? (
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        {total}
      </Typography.Text>
    ) : undefined

  return (
    <Input
      allowClear
      autoFocus={autoFocus}
      className={className}
      style={style}
      prefix={<SearchOutlined style={{ color: 'rgba(0,0,0,0.35)' }} />}
      placeholder={placeholder || he.searchPlaceholder}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      suffix={suffix}
    />
  )
}

export function SearchableEmpty({
  total,
  shown,
  emptyText,
}: {
  total: number
  shown: number
  emptyText?: string
}) {
  if (!total) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyText} />
  if (!shown) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={he.noMatches} />
  return null
}

/** Search box + empty states around any filtered list. */
export function SearchableCollection<T>({
  items,
  text,
  placeholder,
  emptyText,
  hideSearchWhenEmpty = false,
  children,
}: {
  items: readonly T[]
  text: (item: T) => string
  placeholder?: string
  emptyText?: string
  hideSearchWhenEmpty?: boolean
  children: (visible: T[]) => ReactNode
}) {
  const search = useListSearch(items, text)
  if (hideSearchWhenEmpty && !items.length) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyText} />
  }
  return (
    <div className="searchable-collection">
      {items.length ? (
        <ListSearchBar
          value={search.query}
          onChange={search.setQuery}
          placeholder={placeholder}
          total={search.total}
          shown={search.visible.length}
          style={{ marginBottom: 8 }}
        />
      ) : null}
      <SearchableEmpty total={items.length} shown={search.visible.length} emptyText={emptyText} />
      {search.visible.length ? children(search.visible) : null}
    </div>
  )
}
