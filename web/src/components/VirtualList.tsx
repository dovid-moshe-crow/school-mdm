import { useRef, useState, type ReactNode, type UIEvent } from 'react'

export function VirtualList<T>({
  items,
  rowHeight = 56,
  height = 420,
  overscan = 6,
  itemKey,
  renderRow,
}: {
  items: readonly T[]
  rowHeight?: number
  height?: number
  overscan?: number
  itemKey: (item: T, index: number) => string
  renderRow: (item: T, index: number) => ReactNode
}) {
  const [start, setStart] = useState(0)
  const ref = useRef<HTMLDivElement>(null)

  if (items.length <= 24) {
    return (
      <div>
        {items.map((item, i) => (
          <div key={itemKey(item, i)}>{renderRow(item, i)}</div>
        ))}
      </div>
    )
  }

  const visible = Math.ceil(height / rowHeight)
  const windowSize = visible + overscan * 2
  const maxStart = Math.max(0, items.length - windowSize)
  const from = Math.min(start, maxStart)
  const to = Math.min(items.length, from + windowSize)
  const slice = items.slice(from, to)

  function onScroll(e: UIEvent<HTMLDivElement>) {
    const next = Math.min(maxStart, Math.max(0, Math.floor(e.currentTarget.scrollTop / rowHeight) - overscan))
    setStart((prev) => (prev === next ? prev : next))
  }

  return (
    <div ref={ref} className="virtual-list" style={{ height, overflow: 'auto' }} onScroll={onScroll}>
      <div style={{ height: items.length * rowHeight, position: 'relative' }}>
        <div style={{ position: 'absolute', top: from * rowHeight, left: 0, right: 0 }}>
          {slice.map((item, i) => {
            const index = from + i
            return (
              <div key={itemKey(item, index)} className="virtual-list-row" style={{ height: rowHeight }}>
                {renderRow(item, index)}
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
