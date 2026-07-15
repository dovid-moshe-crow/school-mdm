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

function authHeaders(token: string): HeadersInit {
  return token ? { Authorization: `Bearer ${token}` } : {}
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
    return fetch(`/api/access-status?${p}`).then((r) =>
      json<{ status: AccessStatus }>(r),
    )
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
  requests(token: string, params: URLSearchParams) {
    return fetch(`/api/requests?${params}`, { headers: authHeaders(token) }).then((r) =>
      json<Request[]>(r),
    )
  },
  decide(token: string, id: string, approve: boolean, body: Record<string, string>) {
    return fetch(`/api/requests/${id}/${approve ? 'approve' : 'deny'}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
      body: JSON.stringify(body),
    }).then((r) => json<Request>(r))
  },
  devices(token: string) {
    return fetch('/api/devices', { headers: authHeaders(token) }).then((r) => json<string[]>(r))
  },
  groups(token: string) {
    return fetch('/api/groups', { headers: authHeaders(token) }).then((r) => json<Group[]>(r))
  },
  createGroup(token: string, name: string, description: string) {
    return fetch('/api/groups', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
      body: JSON.stringify({ name, description }),
    }).then((r) => json<Group>(r))
  },
  deleteGroup(token: string, id: string) {
    return fetch(`/api/groups/${id}`, { method: 'DELETE', headers: authHeaders(token) }).then((r) =>
      json<{ ok: string }>(r),
    )
  },
  members(token: string, id: string) {
    return fetch(`/api/groups/${id}/members`, { headers: authHeaders(token) }).then((r) =>
      json<string[]>(r),
    )
  },
  setMembers(token: string, id: string, enrollment_ids: string[]) {
    return fetch(`/api/groups/${id}/members`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
      body: JSON.stringify({ enrollment_ids }),
    }).then((r) => json<string[]>(r))
  },
  allowances(token: string, params: URLSearchParams) {
    return fetch(`/api/allowances?${params}`, { headers: authHeaders(token) }).then((r) =>
      json<Allowance[]>(r),
    )
  },
  createAllowance(token: string, body: Record<string, string>) {
    return fetch('/api/allowances', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
      body: JSON.stringify(body),
    }).then((r) => json<unknown>(r))
  },
}
