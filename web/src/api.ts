export type AppMeta = {
  bundle_id: string
  app_name: string
  developer: string
  artwork_url?: string
  access_status?: AccessStatus
}

export type AccessStatus = 'allowed' | 'pending' | 'denied' | 'none'

export type Request = {
  id: string
  type: string
  kind?: string
  value: string
  enrollment_id: string
  reason: string
  status: string
  duration?: string
  created_at: string
  app?: AppMeta
}

export type Group = {
  id: string
  name: string
  description: string
  created_at: string
}

export type Device = {
  enrollment_id: string
  name: string
}

export type Allowance = {
  kind: string
  value: string
  source: string
  target_type?: string
  target_id?: string
  enrollment_id?: string
  group_id?: string
  expires_at?: string
  app?: AppMeta
}

async function json<T>(res: Response): Promise<T> {
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error((data as { error?: string }).error || res.statusText || 'שגיאה')
  return data as T
}

export const api = {
  searchApps(q: string, enrollmentID?: string) {
    const p = new URLSearchParams({ q })
    if (enrollmentID) p.set('enrollment_id', enrollmentID)
    return fetch(`/api/apps/search?${p}`).then((r) => json<AppMeta[]>(r))
  },
  accessStatus(enrollmentID: string, kind: string, value: string) {
    const p = new URLSearchParams({ enrollment_id: enrollmentID, kind, value })
    return fetch(`/api/access-status?${p}`).then((r) => json<{ status: AccessStatus }>(r))
  },
  createRequest(body: Record<string, string>) {
    return fetch('/api/requests', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<Request>(r))
  },
  myRequests(enrollmentID: string) {
    return fetch(`/api/device/${encodeURIComponent(enrollmentID)}/requests`).then((r) =>
      json<Request[]>(r),
    )
  },
  requests(params: URLSearchParams) {
    return fetch(`/api/requests?${params}`).then((r) => json<Request[]>(r))
  },
  decide(id: string, approve: boolean, body: Record<string, string>) {
    return fetch(`/api/requests/${id}/${approve ? 'approve' : 'deny'}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<Request>(r))
  },
  devices() {
    return fetch('/api/devices').then((r) => json<Device[]>(r))
  },
  setDeviceName(id: string, name: string) {
    return fetch(`/api/devices/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    }).then((r) => json<Device>(r))
  },
  groups() {
    return fetch('/api/groups').then((r) => json<Group[]>(r))
  },
  createGroup(name: string, description: string) {
    return fetch('/api/groups', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, description }),
    }).then((r) => json<Group>(r))
  },
  updateGroup(id: string, name: string, description: string) {
    return fetch(`/api/groups/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, description }),
    }).then((r) => json<Group>(r))
  },
  deleteGroup(id: string) {
    return fetch(`/api/groups/${id}`, { method: 'DELETE' }).then((r) => json<{ ok: string }>(r))
  },
  members(id: string) {
    return fetch(`/api/groups/${id}/members`).then((r) => json<string[]>(r))
  },
  setMembers(id: string, enrollment_ids: string[]) {
    return fetch(`/api/groups/${id}/members`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enrollment_ids }),
    }).then((r) => json<string[]>(r))
  },
  allowances(params: URLSearchParams) {
    return fetch(`/api/allowances?${params}`).then((r) => json<Allowance[]>(r))
  },
  createAllowance(body: Record<string, string>) {
    return fetch('/api/allowances', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<unknown>(r))
  },
  deleteAllowance(row: Allowance) {
    const p = new URLSearchParams({
      kind: row.kind,
      value: row.value,
      target_type: row.target_type || 'global',
      target_id: row.target_id || '',
    })
    return fetch(`/api/allowances?${p}`, { method: 'DELETE' }).then((r) => json<{ ok: string }>(r))
  },
}
