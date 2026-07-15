import { useCallback, useEffect, useMemo, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { api, type AccessStatus, type AppMeta, type Request } from '../api'
import { he } from '../he'

type Category = 'access-url' | 'access-app' | 'general' | 'bug'

function statusBadge(s?: AccessStatus) {
  if (s === 'allowed') return <span className="badge ok">{he.alreadyAllowed}</span>
  if (s === 'pending') return <span className="badge warn">{he.pendingRequest}</span>
  if (s === 'denied') return <span className="badge bad">{he.deniedBefore}</span>
  return null
}

export default function Portal() {
  const { deviceId = '' } = useParams()
  const [params] = useSearchParams()
  const [category, setCategory] = useState<Category>(params.get('url') ? 'access-url' : 'access-app')
  const [url, setUrl] = useState(params.get('url') || '')
  const [subject, setSubject] = useState('')
  const [reason, setReason] = useState('')
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<AppMeta[]>([])
  const [selected, setSelected] = useState<AppMeta | null>(null)
  const [searching, setSearching] = useState(false)
  const [searched, setSearched] = useState(false)
  const [urlStatus, setUrlStatus] = useState<AccessStatus | null>(null)
  const [checking, setChecking] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [msg, setMsg] = useState('')
  const [msgOk, setMsgOk] = useState(true)
  const [mine, setMine] = useState<Request[]>([])

  const reasonLabel = useMemo(() => {
    if (category === 'access-url') return he.reasonUrl
    if (category === 'access-app') return he.reasonApp
    if (category === 'bug') return he.reasonBug
    return he.reasonGeneral
  }, [category])

  const loadMine = useCallback(() => {
    if (!deviceId) return
    api.myRequests(deviceId).then(setMine).catch(() => setMine([]))
  }, [deviceId])

  useEffect(() => { loadMine() }, [loadMine])

  useEffect(() => {
    if (category !== 'access-url' || !url.trim() || !deviceId) {
      setUrlStatus(null)
      return
    }
    const t = setTimeout(() => {
      setChecking(true)
      api.accessStatus(deviceId, 'url', url.trim())
        .then((r) => setUrlStatus(r.status))
        .catch(() => setUrlStatus(null))
        .finally(() => setChecking(false))
    }, 350)
    return () => clearTimeout(t)
  }, [url, category, deviceId])

  async function search() {
    if (!query.trim()) return
    setSearching(true)
    setSearched(true)
    try {
      setResults(await api.searchApps(query.trim(), deviceId))
    } catch (e) {
      setMsg((e as Error).message)
      setMsgOk(false)
      setResults([])
    } finally {
      setSearching(false)
    }
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setMsg('')
    try {
      const body: Record<string, string> = { enrollment_id: deviceId, reason }
      if (category === 'access-url') {
        Object.assign(body, { type: 'access', kind: 'url', value: url })
      } else if (category === 'access-app') {
        if (!selected) throw new Error('יש לבחור אפליקציה')
        Object.assign(body, { type: 'access', kind: 'app', value: selected.bundle_id })
      } else if (category === 'general') {
        Object.assign(body, { type: 'general', value: subject })
      } else {
        Object.assign(body, { type: 'bug', value: subject })
      }
      await api.createRequest(body)
      setMsgOk(true)
      setMsg('הבקשה נשלחה')
      setReason('')
      setSubject('')
      setSelected(null)
      setResults([])
      loadMine()
    } catch (err) {
      setMsgOk(false)
      setMsg((err as Error).message)
    } finally {
      setSubmitting(false)
    }
  }

  const blocked =
    (category === 'access-url' && urlStatus === 'allowed') ||
    (category === 'access-app' && selected?.access_status === 'allowed')

  return (
    <main className="shell">
      <h1 className="brand">{he.portalTitle}</h1>
      <p className="lede">{he.portalLead}</p>
      <div className="chip">{he.device} <code>{deviceId}</code></div>

      <form className="panel" onSubmit={submit}>
        <label>{he.category}</label>
        <select
          value={category}
          onChange={(e) => {
            setCategory(e.target.value as Category)
            setMsg('')
            setSelected(null)
            setResults([])
          }}
        >
          <option value="access-url">{he.catUrl}</option>
          <option value="access-app">{he.catApp}</option>
          <option value="general">{he.catGeneral}</option>
          <option value="bug">{he.catBug}</option>
        </select>

        {category === 'access-url' && (
          <>
            <label>{he.url}</label>
            <input value={url} onChange={(e) => setUrl(e.target.value)} required placeholder="https://" />
            {checking && <p className="meta">{he.checkStatus}</p>}
            {statusBadge(urlStatus || undefined)}
          </>
        )}

        {category === 'access-app' && !selected && (
          <>
            <label>{he.searchApp}</label>
            <div className="row">
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && (e.preventDefault(), search())}
                placeholder="YouTube"
                autoComplete="off"
              />
              <button type="button" className="secondary tiny" onClick={search} disabled={searching}>
                {searching ? he.searching : he.search}
              </button>
            </div>
            {searched && !searching && results.length === 0 && <p className="meta">{he.noApps}</p>}
            <div className="results">
              {results.map((item) => (
                <button key={item.bundle_id} type="button" className="app-row" onClick={() => setSelected(item)}>
                  <img src={item.artwork_url || ''} alt="" />
                  <div>
                    <strong>{item.app_name}</strong>
                    <span>{item.developer ? `${he.by} ${item.developer}` : ''}</span>
                    {statusBadge(item.access_status)}
                  </div>
                  <div className="meta">{he.pick}</div>
                </button>
              ))}
            </div>
          </>
        )}

        {category === 'access-app' && selected && (
          <>
            <label>{he.selectedApp}</label>
            <div className="picked">
              <img src={selected.artwork_url || ''} alt="" />
              <div>
                <strong>{selected.app_name}</strong>
                <div className="meta">{selected.developer ? `${he.by} ${selected.developer}` : ''}</div>
                <code>{selected.bundle_id}</code>
                <div>{statusBadge(selected.access_status)}</div>
              </div>
              <div className="grow">
                <button type="button" className="secondary tiny" onClick={() => setSelected(null)}>{he.change}</button>
              </div>
            </div>
          </>
        )}

        {(category === 'general' || category === 'bug') && (
          <>
            <label>{category === 'bug' ? he.bugTitle : he.subject}</label>
            <input value={subject} onChange={(e) => setSubject(e.target.value)} required />
          </>
        )}

        <label>{reasonLabel}</label>
        <textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={3} />

        <button type="submit" disabled={submitting || blocked}>
          {submitting ? he.sending : he.submit}
        </button>
        {blocked && <p className="meta">{he.alreadyAllowed}</p>}
        {msg && <div className={`msg ${msgOk ? 'ok' : 'err'}`}>{msg}</div>}
      </form>

      <h2 style={{ marginTop: '2rem', fontSize: '1.15rem' }}>{he.myRequests}</h2>
      {!mine.length && <p className="meta">{he.noRequests}</p>}
      {mine.map((r) => (
        <div className="card" key={r.id}>
          <span className="badge">{he.statusLabel[r.status] || r.status}</span>
          <span className="badge">{he.typeLabel[r.type] || r.type}</span>
          <div style={{ marginTop: '0.5rem' }}><strong>{r.app?.app_name || r.value}</strong></div>
          <div className="meta">{r.reason}</div>
        </div>
      ))}
    </main>
  )
}
