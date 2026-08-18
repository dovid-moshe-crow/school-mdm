import { useMemo, useState } from 'react'
import { matchesQuery } from '../search'

export function useListSearch<T>(
  items: readonly T[],
  text: (item: T) => string,
): {
  query: string
  setQuery: (q: string) => void
  visible: T[]
  total: number
} {
  const [query, setQuery] = useState('')
  const visible = useMemo(() => {
    if (!query.trim()) return items as T[]
    return items.filter((item) => matchesQuery(query, text(item)))
  }, [items, query, text])
  return { query, setQuery, visible, total: items.length }
}
