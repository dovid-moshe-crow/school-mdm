import { useCallback, useRef, useState } from 'react'

/** Tracks one in-flight UI action so buttons can spin and ignore double-clicks. */
export function useBusy() {
  const [busy, setBusy] = useState('')
  const lock = useRef('')

  const run = useCallback(async <T,>(key: string, fn: () => Promise<T>): Promise<T | undefined> => {
    if (lock.current) return
    lock.current = key
    setBusy(key)
    try {
      return await fn()
    } finally {
      lock.current = ''
      setBusy('')
    }
  }, [])

  const is = useCallback((key: string) => busy === key, [busy])

  return { busy, run, is, locked: !!busy }
}
