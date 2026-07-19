import { Avatar } from 'antd'
import { useEffect, useState } from 'react'

export function useDebounced<T>(value: T, ms = 300): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}

export function AppThumb({ name, url, size = 40 }: { name?: string; url?: string; size?: number }) {
  return (
    <Avatar
      src={url || undefined}
      shape="square"
      size={size}
      style={{ flexShrink: 0, background: '#d9e7e1', color: '#0b6e4f', fontWeight: 600 }}
    >
      {(name || '?').slice(0, 1).toUpperCase()}
    </Avatar>
  )
}

export function normalizeHostPreview(raw: string): string {
  const t = raw.trim()
  if (!t) return ''
  try {
    const withProto = t.includes('://') ? t : `https://${t}`
    const u = new URL(withProto)
    let host = u.host.toLowerCase().replace(/^www\./, '')
    const path = u.pathname.replace(/\/$/, '')
    return host + (path === '/' ? '' : path)
  } catch {
    return t.toLowerCase().replace(/^www\./, '').replace(/\/$/, '')
  }
}
