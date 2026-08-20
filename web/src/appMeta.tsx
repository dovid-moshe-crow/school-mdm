import {
  createContext,
  memo,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { Flex, Typography } from 'antd'
import { api, type AppMeta } from './api'
import { appTitle, knownAppName } from './knownApps'
import { AppThumb, useNearViewport } from './ui'

type AppMetaCtx = {
  get: (bundleId?: string, fallback?: AppMeta | null) => AppMeta | undefined
  ensure: (ids: string[]) => void
  subscribe: (bundleId: string, onChange: () => void) => () => void
}

const Ctx = createContext<AppMetaCtx>({
  get: (_id, fallback) => fallback || undefined,
  ensure: () => {},
  subscribe: () => () => {},
})

export function AppMetaProvider({ children }: { children: ReactNode }) {
  const cacheRef = useRef<Record<string, AppMeta>>({})
  const inflight = useRef(new Set<string>())
  const queued = useRef(new Set<string>())
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const listeners = useRef(new Map<string, Set<() => void>>())

  const notify = useCallback((key: string) => {
    listeners.current.get(key)?.forEach((fn) => fn())
  }, [])

  const flush = useCallback(async () => {
    const BATCH = 20
    while (queued.current.size) {
      const ids = [...queued.current].filter((id) => {
        const key = id.toLowerCase()
        return !cacheRef.current[key] && !inflight.current.has(key)
      })
      queued.current.clear()
      if (!ids.length) break
      for (let i = 0; i < ids.length; i += BATCH) {
        const batch = ids.slice(i, i + BATCH)
        batch.forEach((id) => inflight.current.add(id.toLowerCase()))
        try {
          const list = await api.lookupApps(batch, { remote: false })
          const changed: string[] = []
          for (const m of list) {
            if (!m?.bundle_id) continue
            const key = m.bundle_id.toLowerCase()
            cacheRef.current[key] = m
            changed.push(key)
          }
          for (const id of batch) {
            const key = id.toLowerCase()
            if (!cacheRef.current[key]) {
              cacheRef.current[key] = {
                bundle_id: id,
                app_name: knownAppName(id) || id,
                developer: '',
                source: 'local',
              }
              changed.push(key)
            }
            inflight.current.delete(key)
          }
          changed.forEach(notify)
        } catch {
          batch.forEach((id) => inflight.current.delete(id.toLowerCase()))
        }
      }
    }
  }, [notify])

  const ensure = useCallback(
    (ids: string[]) => {
      let added = false
      for (const raw of ids) {
        const id = (raw || '').trim()
        if (!id) continue
        const key = id.toLowerCase()
        if (cacheRef.current[key] || inflight.current.has(key) || queued.current.has(key)) continue
        queued.current.add(key)
        added = true
      }
      if (!added) return
      if (timer.current) clearTimeout(timer.current)
      timer.current = setTimeout(() => {
        void flush()
      }, 80)
    },
    [flush],
  )

  const get = useCallback((bundleId?: string, fallback?: AppMeta | null) => {
    const id = (bundleId || fallback?.bundle_id || '').trim()
    if (!id) return fallback || undefined
    return cacheRef.current[id.toLowerCase()] || fallback || undefined
  }, [])

  const subscribe = useCallback((bundleId: string, onChange: () => void) => {
    const key = bundleId.trim().toLowerCase()
    if (!key) return () => {}
    let set = listeners.current.get(key)
    if (!set) {
      set = new Set()
      listeners.current.set(key, set)
    }
    set.add(onChange)
    return () => {
      set?.delete(onChange)
      if (set && set.size === 0) listeners.current.delete(key)
    }
  }, [])

  const value = useMemo(() => ({ get, ensure, subscribe }), [get, ensure, subscribe])
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useAppMetaStore() {
  return useContext(Ctx)
}

export function useAppMeta(bundleId?: string, fallback?: AppMeta | null, enabled = true) {
  const { get, ensure, subscribe } = useContext(Ctx)
  const id = (bundleId || fallback?.bundle_id || '').trim()
  const [, bump] = useState(0)
  useEffect(() => {
    if (!id) return
    return subscribe(id, () => bump((n) => n + 1))
  }, [id, subscribe])
  useEffect(() => {
    if (enabled && id) ensure([id])
  }, [id, ensure, enabled])
  return get(id, fallback)
}

export const AppIdentity = memo(function AppIdentity({
  bundleId,
  meta,
  size = 36,
  compact,
  extra,
}: {
  bundleId?: string
  meta?: AppMeta | null
  size?: number
  compact?: boolean
  extra?: ReactNode
}) {
  const { ref, near } = useNearViewport()
  const resolved = useAppMeta(bundleId || meta?.bundle_id, meta, near)
  const id = (bundleId || meta?.bundle_id || resolved?.bundle_id || '').trim()
  const name = appTitle(resolved || meta, id)
  const url = resolved?.artwork_url || meta?.artwork_url
  if (compact) {
    return (
      <span ref={ref} className="app-identity is-compact" title={id}>
        <AppThumb name={name} url={url} size={size} load={near} />
        <span className="app-identity-name">{name}</span>
      </span>
    )
  }
  return (
    <span ref={ref} className="app-identity-hit">
      <Flex gap={8} align="center" className="app-identity" title={id || undefined}>
        <AppThumb name={name} url={url} size={size} load={near} />
        <span style={{ minWidth: 0 }}>
          <Typography.Text strong ellipsis>
            {name}
          </Typography.Text>
          {extra}
        </span>
      </Flex>
    </span>
  )
})
