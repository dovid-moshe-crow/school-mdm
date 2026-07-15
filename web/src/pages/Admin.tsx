import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, type Allowance, type AppMeta, type Device, type Group, type Request } from '../api'
import { he } from '../he'

type Tab = 'requests' | 'groups' | 'allowances'

function labelDevice(d: Device | string, devices: Device[]) {
  if (typeof d === 'string') {
    const found = devices.find((x) => x.enrollment_id === d)
    return found?.name ? `${found.name} (${d})` : d
  }
  return d.name ? `${d.name} (${d.enrollment_id})` : d.enrollment_id
}

function sourceLabel(src: string) {
  if (src === 'essential') return he.sourceEssential
  if (src === 'global') return he.sourceGlobal
  if (src === 'group') return he.sourceGroup
  if (src === 'device') return he.sourceDevice
  if (src === 'grant') return he.sourceGrant
  return src
}

export default function Admin() {
  const [tab, setTab] = useState<Tab>('requests')
  const [error, setError] = useState('')
  const [ok, setOk] = useState('')
  const [devices, setDevices] = useState<Device[]>([])
  const [groups, setGroups] = useState<Group[]>([])
  const [memberCounts, setMemberCounts] = useState<Record<string, number>>({})
  const [requests, setRequests] = useState<Request[]>([])
  const [allowances, setAllowances] = useState<Allowance[]>([])
  const [busy, setBusy] = useState(false)

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
  const [memberFilter, setMemberFilter] = useState('')
  const [renameDraft, setRenameDraft] = useState('')

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

  const flash = (msg: string, isErr = false) => {
    if (isErr) { setError(msg); setOk('') } else { setOk(msg); setError('') }
  }

  const refreshMeta = useCallback(async () => {
    const [d, g] = await Promise.all([api.devices(), api.groups()])
    setDevices(d)
    setGroups(g)
    const counts: Record<string, number> = {}
    await Promise.all(
      g.map(async (group) => {
        const m = await api.members(group.id)
        counts[group.id] = m.length
      }),
    )
    setMemberCounts(counts)
  }, [])

  const loadRequests = useCallback(async () => {
    setBusy(true)
    try {
      const p = new URLSearchParams({ status: reqStatus, type: reqType, sort: 'created_desc' })
      if (reqDevice) p.set('enrollment_id', reqDevice)
      if (reqQ.trim()) p.set('q', reqQ.trim())
      const list = await api.requests(p)
      setRequests(list)
      setApproveScope((prev) => {
        const next = { ...prev }
        for (const r of list) if (!next[r.id]) next[r.id] = 'device'
        return next
      })
    } catch (e) {
      flash((e as Error).message, true)
    } finally {
      setBusy(false)
    }
  }, [reqStatus, reqType, reqDevice, reqQ])

  const loadAllowances = useCallback(async () => {
    if (allowScope === 'device' && !allowDevice) return setAllowances([])
    if (allowScope === 'group' && !allowGroup) return setAllowances([])
    setBusy(true)
    try {
      const p = new URLSearchParams({ scope: allowScope, kind: allowKind })
      if (allowScope === 'device') p.set('enrollment_id', allowDevice)
      if (allowScope === 'group') p.set('group_id', allowGroup)
      if (allowQ.trim()) p.set('q', allowQ.trim())
      setAllowances(await api.allowances(p))
    } catch (e) {
      flash((e as Error).message, true)
    } finally {
      setBusy(false)
    }
  }, [allowScope, allowKind, allowDevice, allowGroup, allowQ])

  useEffect(() => { refreshMeta().catch((e) => flash((e as Error).message, true)) }, [refreshMeta])
  useEffect(() => {
    if (tab === 'requests') loadRequests()
    if (tab === 'allowances') loadAllowances()
    if (tab === 'groups') refreshMeta()
  }, [tab, loadRequests, loadAllowances, refreshMeta])

  const filteredDevices = useMemo(() => {
    const q = memberFilter.trim().toLowerCase()
    if (!q) return devices
    return devices.filter(
      (d) => d.enrollment_id.toLowerCase().includes(q) || d.name.toLowerCase().includes(q),
    )
  }, [devices, memberFilter])

  async function decide(id: string, approve: boolean, duration = '') {
    const prev = requests
    setRequests((list) => list.filter((r) => r.id !== id))
    try {
      const body: Record<string, string> = { duration }
      if (approve) {
        body.scope = approveScope[id] || 'device'
        if (body.scope === 'group') body.group_id = approveGroup[id] || ''
      }
      await api.decide(id, approve, body)
      flash(he.ok)
      refreshMeta()
    } catch (e) {
      setRequests(prev)
      flash((e as Error).message, true)
    }
  }

  return (
    <main className="shell wide">
      <h1 className="brand">{he.admin}</h1>
      <p className="lede">{he.adminLead}</p>

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
      {busy && <p className="count">{he.loading}</p>}

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
                  <option value="access">{he.typeLabel.access}</option>
                  <option value="general">{he.typeLabel.general}</option>
                  <option value="bug">{he.typeLabel.bug}</option>
                </select>
              </div>
              <div>
                <label>{he.device}</label>
                <select value={reqDevice} onChange={(e) => setReqDevice(e.target.value)}>
                  <option value="">{he.all}</option>
                  {devices.map((d) => (
                    <option key={d.enrollment_id} value={d.enrollment_id}>{labelDevice(d, devices)}</option>
                  ))}
                </select>
              </div>
            </div>
            <input value={reqQ} onChange={(e) => setReqQ(e.target.value)} placeholder={he.searchPlaceholder} />
          </div>
          {!requests.length && !busy && <div className="panel">{he.emptyRequests}</div>}
          {requests.map((r) => (
            <div className="card" key={r.id}>
              <span className="badge">{he.statusLabel[r.status] || r.status}</span>
              <span className="badge">{he.typeLabel[r.type] || r.type}</span>
              {r.kind && <span className="badge">{r.kind === 'app' ? 'אפליקציה' : 'אתר'}</span>}
              <div style={{ marginTop: '0.55rem', display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
                {r.app?.artwork_url && <img src={r.app.artwork_url} alt="" width={44} height={44} style={{ borderRadius: 10 }} />}
                <div>
                  <strong>{r.app?.app_name || r.value}</strong>
                  <div className="meta"><code>{r.value}</code></div>
                </div>
              </div>
              <div className="meta">{he.device}: {labelDevice(r.enrollment_id, devices)}</div>
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
              {r.status === 'pending' && r.type !== 'access' && (
                <div className="actions">
                  <button type="button" className="approve" onClick={() => decide(r.id, true)}>
                    {r.type === 'bug' ? he.resolve : he.approveForever}
                  </button>
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
                  await api.createGroup(newName.trim(), newDesc.trim())
                  setNewName('')
                  setNewDesc('')
                  flash(he.ok)
                  refreshMeta()
                } catch (e) {
                  flash((e as Error).message, true)
                }
              }}
            >
              {he.createGroup}
            </button>
          </div>

          <div className="panel" style={{ marginBottom: '1rem' }}>
            <strong>{he.nickname}</strong>
            <p className="meta">כינוי ידידותי למכשירים ברשימות.</p>
            {devices.map((d) => (
              <div key={d.enrollment_id} className="row" style={{ marginTop: '0.5rem' }}>
                <input
                  defaultValue={d.name}
                  placeholder={d.enrollment_id}
                  onBlur={async (e) => {
                    const name = e.target.value.trim()
                    if (name === d.name) return
                    try {
                      await api.setDeviceName(d.enrollment_id, name)
                      flash(he.ok)
                      refreshMeta()
                    } catch (err) {
                      flash((err as Error).message, true)
                    }
                  }}
                />
                <code className="meta" style={{ alignSelf: 'center' }}>{d.enrollment_id}</code>
              </div>
            ))}
          </div>

          {!groups.length && <div className="panel">{he.noGroups}</div>}
          {groups.map((g) => (
            <div className="card" key={g.id}>
              <div style={{ display: 'flex', justifyContent: 'space-between', gap: '0.75rem', flexWrap: 'wrap' }}>
                <div>
                  <strong>{g.name}</strong>
                  <span className="badge">{he.memberCount}: {memberCounts[g.id] ?? 0}</span>
                  <div className="meta">{g.description}</div>
                </div>
                <div className="actions">
                  <button
                    type="button"
                    className="tiny secondary"
                    onClick={async () => {
                      setSelectedGroup(g)
                      setRenameDraft(g.name)
                      setMembers(await api.members(g.id))
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
                      await api.deleteGroup(g.id)
                      refreshMeta()
                    }}
                  >
                    {he.delete}
                  </button>
                </div>
              </div>
              {selectedGroup?.id === g.id && (
                <div style={{ marginTop: '1rem', borderTop: '1px solid var(--line)', paddingTop: '0.85rem' }}>
                  <label>{he.rename}</label>
                  <div className="row">
                    <input value={renameDraft} onChange={(e) => setRenameDraft(e.target.value)} />
                    <button
                      type="button"
                      className="tiny approve"
                      onClick={async () => {
                        await api.updateGroup(g.id, renameDraft.trim(), g.description)
                        flash(he.ok)
                        refreshMeta()
                      }}
                    >
                      {he.save}
                    </button>
                  </div>
                  <label>{he.members}</label>
                  <input
                    value={memberFilter}
                    onChange={(e) => setMemberFilter(e.target.value)}
                    placeholder={he.filterDevices}
                  />
                  <div className="members">
                    {filteredDevices.map((d) => (
                      <label key={d.enrollment_id}>
                        <input
                          type="checkbox"
                          checked={members.includes(d.enrollment_id)}
                          onChange={(e) =>
                            setMembers((m) =>
                              e.target.checked
                                ? [...m, d.enrollment_id]
                                : m.filter((x) => x !== d.enrollment_id),
                            )
                          }
                        />
                        <span>{labelDevice(d, devices)}</span>
                      </label>
                    ))}
                  </div>
                  <div className="actions">
                    <button
                      type="button"
                      className="tiny approve"
                      onClick={async () => {
                        setMembers(await api.setMembers(g.id, members))
                        flash(he.ok)
                        refreshMeta()
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
                    if (e.target.value === 'device' && !allowDevice && devices[0]) setAllowDevice(devices[0].enrollment_id)
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
                      <option key={d.enrollment_id} value={d.enrollment_id}>{labelDevice(d, devices)}</option>
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
                  <option value="url">אתר</option>
                  <option value="app">אפליקציה</option>
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
                  <option key={d.enrollment_id} value={d.enrollment_id}>{labelDevice(d, devices)}</option>
                ))}
              </select>
            )}
            {addKind === 'url' ? (
              <input value={addValue} onChange={(e) => setAddValue(e.target.value)} placeholder="khanacademy.org" />
            ) : (
              <>
                <div className="row">
                  <input value={addAppQ} onChange={(e) => setAddAppQ(e.target.value)} />
                  <button type="button" className="tiny secondary" onClick={async () => setAddResults(await api.searchApps(addAppQ.trim()))}>
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
              </>
            )}
            <button
              type="button"
              className="approve"
              style={{ width: '100%' }}
              onClick={async () => {
                try {
                  await api.createAllowance({
                    kind: addKind,
                    value: addKind === 'app' ? addApp?.bundle_id || addValue : addValue,
                    scope: addScope,
                    duration: addDuration,
                    group_id: addGroup,
                    enrollment_id: addDevice,
                  })
                  flash(he.ok)
                  setAddValue('')
                  setAddApp(null)
                  loadAllowances()
                } catch (e) {
                  flash((e as Error).message, true)
                }
              }}
            >
              {he.addToAllow}
            </button>
          </div>

          {!allowances.length && !busy && <div className="panel">{he.emptyAllow}</div>}
          {allowances.map((row, i) => (
            <div className="card" key={`${row.kind}-${row.value}-${row.source}-${i}`}>
              <span className="badge">{row.kind === 'app' ? 'אפליקציה' : 'אתר'}</span>
              <span className="badge">{sourceLabel(row.source)}</span>
              <div style={{ marginTop: '0.55rem', display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
                {row.kind === 'app' && row.app?.artwork_url ? (
                  <img
                    src={row.app.artwork_url}
                    alt=""
                    width={44}
                    height={44}
                    style={{ borderRadius: 10, objectFit: 'cover', background: '#eee', flexShrink: 0 }}
                  />
                ) : row.kind === 'app' ? (
                  <div
                    aria-hidden
                    style={{
                      width: 44,
                      height: 44,
                      borderRadius: 10,
                      background: '#e8eef2',
                      display: 'grid',
                      placeItems: 'center',
                      fontWeight: 700,
                      color: 'var(--muted)',
                      flexShrink: 0,
                    }}
                  >
                    {(row.app?.app_name || row.value || '?').slice(0, 1).toUpperCase()}
                  </div>
                ) : null}
                <div>
                  <strong>{row.app?.app_name || row.value}</strong>
                  <div className="meta"><code>{row.value}</code></div>
                </div>
              </div>
              {row.enrollment_id && <div className="meta">{he.device}: {labelDevice(row.enrollment_id, devices)}</div>}
              {row.group_id && <div className="meta">{he.group}: {groups.find((g) => g.id === row.group_id)?.name || row.group_id}</div>}
              {row.source !== 'essential' && (
                <button
                  type="button"
                  className="tiny deny"
                  onClick={async () => {
                    try {
                      await api.deleteAllowance(row)
                      flash(he.ok)
                      loadAllowances()
                    } catch (e) {
                      flash((e as Error).message, true)
                    }
                  }}
                >
                  {he.revoke}
                </button>
              )}
            </div>
          ))}
        </>
      )}
    </main>
  )
}
