import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, type Allowance, type AppMeta, type Group, type Request } from '../api'
import { he } from '../he'

type Tab = 'requests' | 'groups' | 'allowances'

function useToken() {
  const [params] = useSearchParams()
  const initial = params.get('token') || sessionStorage.getItem('adminToken') || ''
  const [token, setToken] = useState(initial)
  useEffect(() => {
    if (params.get('token')) sessionStorage.setItem('adminToken', params.get('token')!)
  }, [params])
  return {
    token,
    setToken: (t: string) => {
      sessionStorage.setItem('adminToken', t)
      setToken(t)
    },
  }
}

export default function Admin() {
  const { token, setToken } = useToken()
  const [tab, setTab] = useState<Tab>('requests')
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const [devices, setDevices] = useState<string[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [requests, setRequests] = useState<Request[]>([])
  const [allowances, setAllowances] = useState<Allowance[]>([])
  const [loading, setLoading] = useState(false)

  const [reqStatus, setReqStatus] = useState('pending')
  const [reqType, setReqType] = useState('all')
  const [reqDevice, setReqDevice] = useState('')
  const [reqQ, setReqQ] = useState('')
  const [approveScope, setApproveScope] = useState<Record<string, string>>({})
  const [approveGroup, setApproveGroup] = useState<Record<string, string>>({})

  const [newName, setNewName] = useState('')
  const [newDesc, setNewDesc] = useState('')
  const [selectedGroup, setSelectedGroup] = useState<Group | null>(null)
  const [members, setMembers] = useState<string[]>([])

  const [allowScope, setAllowScope] = useState('global')
  const [allowKind, setAllowKind] = useState('all')
  const [allowDevice, setAllowDevice] = useState('')
  const [allowGroup, setAllowGroup] = useState('')
  const [allowQ, setAllowQ] = useState('')
  const [addKind, setAddKind] = useState('url')
  const [addScope, setAddScope] = useState('global')
  const [addGroup, setAddGroup] = useState('')
  const [addDevice, setAddDevice] = useState('')
  const [addDuration, setAddDuration] = useState('permanent')
  const [addValue, setAddValue] = useState('')
  const [addAppQ, setAddAppQ] = useState('')
  const [addResults, setAddResults] = useState<AppMeta[]>([])
  const [addApp, setAddApp] = useState<AppMeta | null>(null)

  const groupName = useCallback(
    (id?: string) => groups.find((g) => g.id === id)?.name || id || '',
    [groups],
  )

  const refreshMeta = useCallback(async () => {
    if (!token) return
    const [d, g] = await Promise.all([api.devices(token), api.groups(token)])
    setDevices(d)
    setGroups(g)
  }, [token])

  const loadRequests = useCallback(async () => {
    if (!token) return
    setLoading(true)
    setError('')
    try {
      const p = new URLSearchParams({ status: reqStatus, type: reqType, sort: 'created_desc' })
      if (reqDevice) p.set('enrollment_id', reqDevice)
      if (reqQ.trim()) p.set('q', reqQ.trim())
      const list = await api.requests(token, p)
      setRequests(list)
      setApproveScope((prev) => {
        const next = { ...prev }
        for (const r of list) if (!next[r.id]) next[r.id] = 'device'
        return next
      })
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }, [token, reqStatus, reqType, reqDevice, reqQ])

  const loadAllowances = useCallback(async () => {
    if (!token) return
    if (allowScope === 'device' && !allowDevice) return setAllowances([])
    if (allowScope === 'group' && !allowGroup) return setAllowances([])
    setLoading(true)
    setError('')
    try {
      const p = new URLSearchParams({ scope: allowScope, kind: allowKind })
      if (allowScope === 'device') p.set('enrollment_id', allowDevice)
      if (allowScope === 'group') p.set('group_id', allowGroup)
      if (allowQ.trim()) p.set('q', allowQ.trim())
      setAllowances(await api.allowances(token, p))
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }, [token, allowScope, allowKind, allowDevice, allowGroup, allowQ])

  useEffect(() => {
    if (!token) return
    refreshMeta().catch((e) => setError((e as Error).message))
  }, [token, refreshMeta])

  useEffect(() => {
    if (tab === 'requests') loadRequests()
    if (tab === 'allowances') loadAllowances()
    if (tab === 'groups') refreshMeta()
  }, [tab, loadRequests, loadAllowances, refreshMeta])

  const tokenGate = useMemo(() => {
    if (token) return null
    return (
      <div className="panel">
        <label>{he.token}</label>
        <div className="row">
          <input
            id="tok"
            placeholder="dev-admin-token"
            onKeyDown={(e) => {
              if (e.key === 'Enter') setToken((e.target as HTMLInputElement).value.trim())
            }}
          />
          <button
            type="button"
            className="tiny"
            onClick={() => setToken((document.getElementById('tok') as HTMLInputElement).value.trim())}
          >
            {he.saveToken}
          </button>
        </div>
      </div>
    )
  }, [token, setToken])

  async function decide(id: string, approve: boolean, duration = '') {
    setError('')
    setOk('')
    try {
      const body: Record<string, string> = { duration }
      if (approve) {
        body.scope = approveScope[id] || 'device'
        if (body.scope === 'group') body.group_id = approveGroup[id] || ''
      }
      await api.decide(token, id, approve, body)
      setOk(he.ok)
      await Promise.all([loadRequests(), refreshMeta()])
    } catch (e) {
      setError((e as Error).message)
    }
  }

  return (
    <main className="shell wide">
      <h1 className="brand">{he.admin}</h1>
      <p className="lede">{he.adminLead}</p>
      {tokenGate}
      {token && (
        <>
          <div className="tabs">
            {([
              ['requests', he.tabRequests],
              ['groups', he.tabGroups],
              ['allowances', he.tabAllow],
            ] as const).map(([k, label]) => (
              <button key={k} type="button" className={`tab ${tab === k ? 'active' : ''}`} onClick={() => setTab(k)}>
                {label}
              </button>
            ))}
          </div>
          {error && <div className="msg err">{error}</div>}
          {ok && <div className="msg ok">{ok}</div>}
          <p className="count">{loading ? he.loading : null}</p>

          {tab === 'requests' && (
            <>
              <div className="panel filters">
                <div className="filters-row">
                  <div>
                    <label>{he.status}</label>
                    <select value={reqStatus} onChange={(e) => setReqStatus(e.target.value)}>
                      <option value="all">{he.all}</option>
                      <option value="pending">{he.pending}</option>
                      <option value="open">{he.open}</option>
                      <option value="closed">{he.closed}</option>
                      <option value="approved">{he.approved}</option>
                      <option value="denied">{he.denied}</option>
                      <option value="resolved">{he.resolved}</option>
                    </select>
                  </div>
                  <div>
                    <label>{he.type}</label>
                    <select value={reqType} onChange={(e) => setReqType(e.target.value)}>
                      <option value="all">{he.all}</option>
                      <option value="access">access</option>
                      <option value="general">general</option>
                      <option value="bug">bug</option>
                    </select>
                  </div>
                  <div>
                    <label>{he.device}</label>
                    <select value={reqDevice} onChange={(e) => setReqDevice(e.target.value)}>
                      <option value="">{he.all}</option>
                      {devices.map((d) => (
                        <option key={d} value={d}>{d}</option>
                      ))}
                    </select>
                  </div>
                </div>
                <label>{he.searchPlaceholder}</label>
                <input value={reqQ} onChange={(e) => setReqQ(e.target.value)} placeholder={he.searchPlaceholder} />
              </div>
              {!requests.length && !loading && <div className="panel">{he.emptyRequests}</div>}
              {requests.map((r) => (
                <div className="card" key={r.id}>
                  <span className="badge">{he.statusLabel[r.status] || r.status}</span>
                  <span className="badge">{he.typeLabel[r.type] || r.type}</span>
                  {r.kind && <span className="badge">{r.kind === 'app' ? 'אפליקציה' : r.kind === 'url' ? 'אתר' : r.kind}</span>}
                  <div style={{ marginTop: '0.55rem', display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
                    {r.app?.artwork_url && <img src={r.app.artwork_url} alt="" width={44} height={44} style={{ borderRadius: 10 }} />}
                    <div>
                      <strong>{r.app?.app_name || r.value}</strong>
                      <div className="meta"><code>{r.value}</code></div>
                    </div>
                  </div>
                  <div className="meta">{he.device} <code>{r.enrollment_id || '—'}</code></div>
                  <div className="meta">{r.reason}</div>
                  {r.status === 'pending' && r.type === 'access' && (
                    <>
                      <div className="filters-row" style={{ marginTop: '0.5rem' }}>
                        <div>
                          <label>{he.grantScope}</label>
                          <select
                            value={approveScope[r.id] || 'device'}
                            onChange={(e) => setApproveScope((s) => ({ ...s, [r.id]: e.target.value }))}
                          >
                            <option value="device">{he.thisDevice}</option>
                            <option value="group">{he.aGroup}</option>
                            <option value="global">{he.everyone}</option>
                          </select>
                        </div>
                        {(approveScope[r.id] || 'device') === 'group' && (
                          <div>
                            <label>{he.group}</label>
                            <select
                              value={approveGroup[r.id] || ''}
                              onChange={(e) => setApproveGroup((s) => ({ ...s, [r.id]: e.target.value }))}
                            >
                              <option value="">…</option>
                              {groups.map((g) => (
                                <option key={g.id} value={g.id}>{g.name}</option>
                              ))}
                            </select>
                          </div>
                        )}
                      </div>
                      <div className="actions">
                        <button type="button" className="approve" onClick={() => decide(r.id, true, '1h')}>{he.approve1h}</button>
                        <button type="button" className="approve" onClick={() => decide(r.id, true, 'permanent')}>{he.approveForever}</button>
                        <button type="button" className="deny" onClick={() => decide(r.id, false)}>{he.deny}</button>
                      </div>
                    </>
                  )}
                  {r.status === 'pending' && r.type === 'bug' && (
                    <div className="actions">
                      <button type="button" className="approve" onClick={() => decide(r.id, true)}>{he.resolve}</button>
                      <button type="button" className="deny" onClick={() => decide(r.id, false)}>{he.deny}</button>
                    </div>
                  )}
                  {r.status === 'pending' && r.type === 'general' && (
                    <div className="actions">
                      <button type="button" className="approve" onClick={() => decide(r.id, true)}>{he.approveForever}</button>
                      <button type="button" className="deny" onClick={() => decide(r.id, false)}>{he.deny}</button>
                    </div>
                  )}
                </div>
              ))}
            </>
          )}

          {tab === 'groups' && (
            <>
              <div className="panel" style={{ marginBottom: '1rem' }}>
                <strong>{he.createGroup}</strong>
                <label>{he.groupName}</label>
                <input value={newName} onChange={(e) => setNewName(e.target.value)} />
                <label>{he.groupDesc}</label>
                <input value={newDesc} onChange={(e) => setNewDesc(e.target.value)} />
                <button
                  type="button"
                  className="approve"
                  style={{ width: '100%' }}
                  disabled={!newName.trim()}
                  onClick={async () => {
                    try {
                      await api.createGroup(token, newName.trim(), newDesc.trim())
                      setNewName('')
                      setNewDesc('')
                      setOk(he.ok)
                      refreshMeta()
                    } catch (e) {
                      setError((e as Error).message)
                    }
                  }}
                >
                  {he.createGroup}
                </button>
              </div>
              {!groups.length && <div className="panel">{he.noGroups}</div>}
              {groups.map((g) => (
                <div className="card" key={g.id}>
                  <strong>{g.name}</strong>
                  <div className="meta">{g.description}</div>
                  <div className="actions">
                    <button
                      type="button"
                      className="tiny secondary"
                      onClick={async () => {
                        setSelectedGroup(g)
                        setMembers(await api.members(token, g.id))
                      }}
                    >
                      {he.members}
                    </button>
                    <button
                      type="button"
                      className="tiny secondary"
                      onClick={() => {
                        setTab('allowances')
                        setAllowScope('group')
                        setAllowGroup(g.id)
                      }}
                    >
                      {he.viewAllow}
                    </button>
                    <button
                      type="button"
                      className="tiny deny"
                      onClick={async () => {
                        if (!confirm(he.delete + '?')) return
                        await api.deleteGroup(token, g.id)
                        refreshMeta()
                      }}
                    >
                      {he.delete}
                    </button>
                  </div>
                  {selectedGroup?.id === g.id && (
                    <div style={{ marginTop: '1rem', borderTop: '1px solid var(--line)', paddingTop: '0.85rem' }}>
                      <label>{he.members}</label>
                      <div className="members">
                        {devices.map((d) => (
                          <label key={d}>
                            <input
                              type="checkbox"
                              checked={members.includes(d)}
                              onChange={(e) =>
                                setMembers((m) => (e.target.checked ? [...m, d] : m.filter((x) => x !== d)))
                              }
                            />
                            <span>{d}</span>
                          </label>
                        ))}
                      </div>
                      <div className="actions">
                        <button
                          type="button"
                          className="tiny approve"
                          onClick={async () => {
                            setMembers(await api.setMembers(token, g.id, members))
                            setOk(he.ok)
                          }}
                        >
                          {he.saveMembers}
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </>
          )}

          {tab === 'allowances' && (
            <>
              <div className="panel filters">
                <div className="filters-row">
                  <div>
                    <label>{he.scope}</label>
                    <select
                      value={allowScope}
                      onChange={(e) => {
                        setAllowScope(e.target.value)
                        if (e.target.value === 'device' && !allowDevice && devices[0]) setAllowDevice(devices[0])
                        if (e.target.value === 'group' && !allowGroup && groups[0]) setAllowGroup(groups[0].id)
                      }}
                    >
                      <option value="global">{he.global}</option>
                      <option value="group">{he.group}</option>
                      <option value="device">{he.deviceEffective}</option>
                      <option value="all">{he.allSources}</option>
                    </select>
                  </div>
                  <div>
                    <label>{he.kind}</label>
                    <select value={allowKind} onChange={(e) => setAllowKind(e.target.value)}>
                      <option value="all">{he.appsAndUrls}</option>
                      <option value="app">{he.appsOnly}</option>
                      <option value="url">{he.urlsOnly}</option>
                    </select>
                  </div>
                  {allowScope === 'group' && (
                    <div>
                      <label>{he.group}</label>
                      <select value={allowGroup} onChange={(e) => setAllowGroup(e.target.value)}>
                        {groups.map((g) => (
                          <option key={g.id} value={g.id}>{g.name}</option>
                        ))}
                      </select>
                    </div>
                  )}
                  {allowScope === 'device' && (
                    <div>
                      <label>{he.device}</label>
                      <select value={allowDevice} onChange={(e) => setAllowDevice(e.target.value)}>
                        {devices.map((d) => (
                          <option key={d} value={d}>{d}</option>
                        ))}
                      </select>
                    </div>
                  )}
                </div>
                <input value={allowQ} onChange={(e) => setAllowQ(e.target.value)} placeholder={he.searchPlaceholder} />
              </div>

              <div className="panel" style={{ marginBottom: '1rem' }}>
                <strong>{he.addAllow}</strong>
                <div className="filters-row">
                  <div>
                    <label>{he.kind}</label>
                    <select value={addKind} onChange={(e) => { setAddKind(e.target.value); setAddApp(null); setAddValue('') }}>
                      <option value="url">url</option>
                      <option value="app">app</option>
                    </select>
                  </div>
                  <div>
                    <label>{he.scope}</label>
                    <select value={addScope} onChange={(e) => setAddScope(e.target.value)}>
                      <option value="global">{he.everyone}</option>
                      <option value="group">{he.group}</option>
                      <option value="device">{he.device}</option>
                    </select>
                  </div>
                  <div>
                    <label>{he.duration}</label>
                    <select value={addDuration} onChange={(e) => setAddDuration(e.target.value)}>
                      <option value="permanent">{he.permanent}</option>
                      <option value="1h">{he.hour}</option>
                      <option value="24h">{he.day}</option>
                      <option value="15m">{he.minutes15}</option>
                      <option value="today">{he.today}</option>
                    </select>
                  </div>
                </div>
                {addScope === 'group' && (
                  <select value={addGroup} onChange={(e) => setAddGroup(e.target.value)}>
                    <option value="">…</option>
                    {groups.map((g) => (
                      <option key={g.id} value={g.id}>{g.name}</option>
                    ))}
                  </select>
                )}
                {addScope === 'device' && (
                  <select value={addDevice} onChange={(e) => setAddDevice(e.target.value)}>
                    <option value="">…</option>
                    {devices.map((d) => (
                      <option key={d} value={d}>{d}</option>
                    ))}
                  </select>
                )}
                {addKind === 'url' ? (
                  <input value={addValue} onChange={(e) => setAddValue(e.target.value)} placeholder="khanacademy.org" />
                ) : (
                  <>
                    <div className="row">
                      <input value={addAppQ} onChange={(e) => setAddAppQ(e.target.value)} />
                      <button
                        type="button"
                        className="tiny secondary"
                        onClick={async () => setAddResults(await api.searchApps(addAppQ.trim()))}
                      >
                        {he.search}
                      </button>
                    </div>
                    <div className="results">
                      {addResults.map((app) => (
                        <button key={app.bundle_id} type="button" className="app-row" onClick={() => { setAddApp(app); setAddValue(app.bundle_id); setAddResults([]) }}>
                          <img src={app.artwork_url || ''} alt="" />
                          <div><strong>{app.app_name}</strong><span>{app.bundle_id}</span></div>
                        </button>
                      ))}
                    </div>
                    {addApp && <div className="meta"><code>{addApp.bundle_id}</code></div>}
                  </>
                )}
                <button
                  type="button"
                  className="approve"
                  style={{ width: '100%' }}
                  onClick={async () => {
                    try {
                      await api.createAllowance(token, {
                        kind: addKind,
                        value: addKind === 'app' ? addApp?.bundle_id || addValue : addValue,
                        scope: addScope,
                        duration: addDuration,
                        group_id: addGroup,
                        enrollment_id: addDevice,
                      })
                      setOk(he.ok)
                      setAddValue('')
                      setAddApp(null)
                      loadAllowances()
                    } catch (e) {
                      setError((e as Error).message)
                    }
                  }}
                >
                  {he.addToAllow}
                </button>
              </div>

              {!allowances.length && !loading && <div className="panel">{he.emptyAllow}</div>}
              {allowances.map((row, i) => (
                <div className="card" key={`${row.kind}-${row.value}-${row.source}-${i}`}>
                  <span className="badge">{row.kind}</span>
                  <span className="badge">{row.source}</span>
                  <div style={{ marginTop: '0.55rem' }}>
                    <strong>{row.app?.app_name || row.value}</strong>
                    <div className="meta"><code>{row.value}</code></div>
                  </div>
                  {row.enrollment_id && <div className="meta">{he.device} <code>{row.enrollment_id}</code></div>}
                  {row.group_id && <div className="meta">{he.group} <code>{groupName(row.group_id)}</code></div>}
                </div>
              ))}
            </>
          )}
        </>
      )}
    </main>
  )
}
