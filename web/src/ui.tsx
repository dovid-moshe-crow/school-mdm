import { useEffect, useRef, useState } from 'react'

export function useDebounced<T>(value: T, ms = 300): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}

const nearCallbacks = new WeakMap<Element, () => void>()
let sharedObserver: IntersectionObserver | null = null

function observer() {
  if (!sharedObserver) {
    sharedObserver = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue
          nearCallbacks.get(entry.target)?.()
          sharedObserver?.unobserve(entry.target)
        }
      },
      { root: null, rootMargin: '120px 0px', threshold: 0 },
    )
  }
  return sharedObserver
}

/** True once the element is on screen or just outside it (then stays true). */
export function useNearViewport(enabled = true) {
  const ref = useRef<HTMLSpanElement | null>(null)
  const [near, setNear] = useState(!enabled)

  useEffect(() => {
    if (!enabled || near) return
    const el = ref.current
    if (!el) return
    if (typeof IntersectionObserver === 'undefined') {
      setNear(true)
      return
    }
    const onHit = () => setNear(true)
    nearCallbacks.set(el, onHit)
    observer().observe(el)
    return () => {
      observer().unobserve(el)
      nearCallbacks.delete(el)
    }
  }, [enabled, near])

  return { ref, near }
}

export function AppThumb({
  name,
  url,
  size = 40,
  load,
}: {
  name?: string
  url?: string
  size?: number
  load?: boolean
}) {
  const { ref, near } = useNearViewport(load === undefined)
  const show = load ?? near
  const letter = (name || '?').slice(0, 1).toUpperCase()
  return (
    <span ref={load === undefined ? ref : undefined} className="app-thumb" style={{ width: size, height: size }}>
      {show && url ? (
        <img
          className="app-thumb-img"
          src={url}
          alt=""
          width={size}
          height={size}
          decoding="async"
          loading="lazy"
          draggable={false}
        />
      ) : (
        <span className="app-thumb-letter" style={{ width: size, height: size }}>
          {letter}
        </span>
      )}
    </span>
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
