export type AppMeta = {
  bundle_id: string
  app_name: string
  developer: string
  artwork_url?: string
  store_url?: string
  track_id?: number
  source?: string
  access_status?: AccessStatus
  description?: string
  genre?: string
  version?: string
  average_rating?: number
  rating_count?: number
  content_rating?: string
  release_date?: string
  formatted_price?: string
  file_size_bytes?: number
  seller_name?: string
  screenshots?: string[]
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
  decided_at?: string
  message_count?: number
  last_message?: RequestMessage
  app?: AppMeta
}

export type RequestMessage = {
  id: string
  request_id: string
  author_role: 'student' | 'admin'
  body: string
  created_at: string
}

export type Group = {
  id: string
  name: string
  description: string
  created_at: string
  member_count?: number
}

export type Device = {
  enrollment_id: string
  name: string
  unrestricted?: boolean
  serial_number?: string
  enabled?: boolean
  last_seen_at?: string
  group_ids?: string[]
  mdm?: boolean
}

export type ActivityEvent = {
  id: string
  at: string
  category: string
  action: string
  actor_type: string
  actor: string
  enrollment_id?: string
  group_id?: string
  request_id?: string
  command_uuid?: string
  result: string
  summary: string
  detail?: Record<string, unknown>
}

export type ActivityFilter = {
  from?: string
  to?: string
  category?: string
  action?: string
  enrollment_id?: string
  actor_type?: string
  result?: string
  q?: string
  limit?: number
  offset?: number
}

export type MdmSettings = {
  dep_name: string
  dep_profile_uuid?: string
  companion_bundle_id?: string
  companion_itunes_id?: number
  companion_enabled?: boolean
  lock_screen_enabled?: boolean
  lock_screen_footnote?: string
  has_vpp_token?: boolean
  vpp_token_filename?: string
  vpp_token_updated_at?: string
  updated_at?: string
}

/** Apple DEP/ABM account detail from GET /api/mdm/abm/account */
export type AbmAccount = {
  admin_id?: string
  facilitator_id?: string
  org_name?: string
  org_email?: string
  org_phone?: string
  org_address?: string
  org_id?: string
  org_type?: string
  org_version?: string
  server_name?: string
  server_uuid?: string
}

export type AbmDepDevice = {
  serial_number: string
  model?: string
  description?: string
  color?: string
  asset_tag?: string
  device_family?: string
  os?: string
  profile_status?: string
  profile_uuid?: string
  device_assigned_by?: string
  device_assigned_date?: string
  profile_assign_time?: string
  profile_push_time?: string
  op_type?: string
}

export type AbmDeviceSync = {
  status?: string
  cursor?: string
  devices?: AbmDepDevice[]
  fetched_until?: string
  more_to_follow?: boolean
  synced_at?: string
  cached?: boolean
  assigned?: number
  needed?: number
  assign_error?: string
}

export type APIToken = {
  id: string
  name: string
  prefix: string
  created_by?: string
  created_at: string
  last_used_at?: string
}

export type Allowance = {
  kind: string
  value: string
  source: string
  target_type?: string
  target_id?: string
  enrollment_id?: string
  group_id?: string
  pack_id?: string
  pack_name?: string
  expires_at?: string
  app?: AppMeta
}

export type SystemAllowlistItem = {
  kind: string
  value: string
  enabled: boolean
}

export type WhitelistPack = {
  id: string
  name: string
  description: string
  created_at: string
  item_count?: number
}

export type WhitelistPackItem = {
  pack_id: string
  kind: string
  value: string
}

export type WhitelistPackAssignment = {
  pack_id: string
  target_type: string
  target_id: string
}

export type CustomProfile = {
  id: string
  name: string
  description: string
  filename: string
  payload_identifier: string
  payload_uuid: string
  payload_display_name: string
  payload_type: string
  size_bytes: number
  assignment_count?: number
  created_at: string
  updated_at: string
}

export type CustomProfileAssignment = {
  profile_id: string
  target_type: string
  target_id: string
}

export type PolicyTimer = {
  id: string
  name: string
  action: 'add' | 'remove' | string
  pack_ids: string[]
  profile_ids: string[]
  device_ids: string[]
  group_ids: string[]
  schedule: 'once' | 'weekly' | string
  run_at?: string
  weekdays?: number[]
  time_of_day?: string
  enabled: boolean
  last_run_at?: string
  last_run_key?: string
  created_at: string
  updated_at: string
  next_run_at?: string
}

export type PolicyTimerWrite = {
  name: string
  action: 'add' | 'remove'
  pack_ids: string[]
  profile_ids: string[]
  device_ids: string[]
  group_ids: string[]
  schedule: 'once' | 'weekly'
  run_at?: string
  weekdays?: number[]
  time_of_day?: string
  enabled?: boolean
}

export type PolicyTimerApplyResult = {
  timer_id: string
  action: string
  assignments: number
  errors: number
  devices: number
}

async function json<T>(res: Response): Promise<T> {
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const raw = (data as { error?: string }).error || res.statusText || 'שגיאה'
    if (/admin authorization required/i.test(raw)) {
      throw new Error('נדרשת התחברות לממשק הניהול.')
    }
    throw new Error(raw)
  }
  return data as T
}

const ADMIN_TOKEN_KEY = 'school_mdm_admin_token'
/** Matches `.env.example` ADMIN_TOKENS — used so Vite/local admin works without hunting Settings. */
const DEV_DEFAULT_ADMIN_TOKEN = 'dev-admin-token'

export function getAdminToken(): string {
  try {
    const stored = localStorage.getItem(ADMIN_TOKEN_KEY) || ''
    if (stored) return stored
    // Dev UI (:5173) has its own localStorage; seed the common local token once.
    if (import.meta.env.DEV) {
      localStorage.setItem(ADMIN_TOKEN_KEY, DEV_DEFAULT_ADMIN_TOKEN)
      return DEV_DEFAULT_ADMIN_TOKEN
    }
    return ''
  } catch {
    return import.meta.env.DEV ? DEV_DEFAULT_ADMIN_TOKEN : ''
  }
}

export function setAdminToken(token: string) {
  try {
    if (token) localStorage.setItem(ADMIN_TOKEN_KEY, token)
    else localStorage.removeItem(ADMIN_TOKEN_KEY)
  } catch {
    /* ignore */
  }
}

function adminHeaders(extra?: HeadersInit): HeadersInit {
  const token = getAdminToken()
  return {
    ...(extra || {}),
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  }
}

function req(url: string, init: RequestInit = {}) {
  return fetch(url, {
    ...init,
    credentials: 'include',
    headers: adminHeaders(init.headers),
  })
}

export type AdminAuthUser = {
  email: string
  name: string
  method: 'google' | 'token'
}

export type AdminAuthConfig = {
  google: boolean
  token_login: boolean
  hosted_domain?: string
}

export type MdmEnrollment = {
  id: string
  device_id: string
  serial_number?: string
  type: string
  topic: string
  enabled: boolean
  last_seen_at: string
}

export type MdmStatus = {
  enqueue: string
  live: boolean
  public_url?: string
  topic?: string
  checkin?: boolean
  push_cert?: { topic: string; not_after?: string }
}

export type MdmCommandResult = {
  enrollment_id: string
  command_uuid: string
  request_type: string
  status: string
  result?: string
  query_responses?: Record<string, unknown>
  parsed?: Record<string, unknown>
  updated_at: string
  pending: boolean
}

export const api = {
  authConfig() {
    return req('/api/auth/config').then((r) => json<AdminAuthConfig>(r))
  },
  async authMe(): Promise<AdminAuthUser | null> {
    const r = await req('/api/auth/me')
    if (r.status === 401) return null
    return json<AdminAuthUser>(r)
  },
  authLogout() {
    return req('/api/auth/logout', { method: 'POST' }).then((r) => json<{ ok: boolean }>(r))
  },
  searchApps(q: string, enrollmentID?: string) {
    const p = new URLSearchParams({ q })
    if (enrollmentID) p.set('enrollment_id', enrollmentID)
    return req(`/api/apps/search?${p}`).then((r) => json<AppMeta[]>(r))
  },
  lookupApp(bundleID: string, opts?: { refresh?: boolean; enrollmentID?: string }) {
    const p = new URLSearchParams()
    if (opts?.refresh) p.set('full', '1')
    if (opts?.enrollmentID) p.set('enrollment_id', opts.enrollmentID)
    const q = p.toString()
    return req(`/api/apps/${encodeURIComponent(bundleID)}${q ? `?${q}` : ''}`).then((r) =>
      json<AppMeta>(r),
    )
  },
  lookupApps(ids: string[], opts?: { remote?: boolean }) {
    const clean = [...new Set(ids.map((s) => s.trim()).filter(Boolean))]
    const out: AppMeta[] = []
    const run = async () => {
      for (let i = 0; i < clean.length; i += 80) {
        const chunk = clean.slice(i, i + 80)
        const p = new URLSearchParams()
        for (const id of chunk) p.append('id', id)
        if (opts?.remote === false) p.set('fetch', '0')
        out.push(...(await req(`/api/apps/lookup?${p}`).then((r) => json<AppMeta[]>(r))))
      }
      return out
    }
    return run()
  },
  accessStatus(enrollmentID: string, kind: string, value: string) {
    const p = new URLSearchParams({ enrollment_id: enrollmentID, kind, value })
    return req(`/api/access-status?${p}`).then((r) => json<{ status: AccessStatus }>(r))
  },
  createRequest(body: Record<string, string>) {
    return req('/api/requests', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<Request>(r))
  },
  myRequests(enrollmentID: string) {
    return req(`/api/device/${encodeURIComponent(enrollmentID)}/requests`).then((r) =>
      json<Request[]>(r),
    )
  },
  getRequest(id: string) {
    return req(`/api/requests/${id}`).then((r) => json<Request>(r))
  },
  messages(id: string, enrollmentID?: string) {
    const p = new URLSearchParams()
    if (enrollmentID) p.set('enrollment_id', enrollmentID)
    const q = p.toString()
    return req(`/api/requests/${id}/messages${q ? `?${q}` : ''}`).then((r) =>
      json<RequestMessage[]>(r),
    )
  },
  postAdminMessage(id: string, body: string) {
    return req(`/api/requests/${id}/messages`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ body }),
    }).then((r) => json<RequestMessage>(r))
  },
  postStudentMessage(deviceID: string, id: string, body: string) {
    return req(`/api/device/${encodeURIComponent(deviceID)}/requests/${id}/messages`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ body }),
    }).then((r) => json<RequestMessage>(r))
  },
  requests(params: URLSearchParams) {
    return req(`/api/requests?${params}`).then((r) => json<Request[]>(r))
  },
  decide(id: string, approve: boolean, body: Record<string, string>) {
    return req(`/api/requests/${id}/${approve ? 'approve' : 'deny'}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<Request>(r))
  },
  packs() {
    return req('/api/packs').then((r) => json<WhitelistPack[]>(r))
  },
  createPack(name: string, description = '') {
    return req('/api/packs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, description }),
    }).then((r) => json<WhitelistPack>(r))
  },
  getPack(id: string) {
    return req(`/api/packs/${encodeURIComponent(id)}`).then((r) =>
      json<{
        pack: WhitelistPack
        items: WhitelistPackItem[]
        assignments: WhitelistPackAssignment[]
      }>(r),
    )
  },
  deletePack(id: string) {
    return req(`/api/packs/${encodeURIComponent(id)}`, { method: 'DELETE' }).then((r) =>
      json<{ ok: string }>(r),
    )
  },
  systemAllowlist() {
    return req('/api/system-allowlist', { headers: adminHeaders() }).then((r) =>
      json<SystemAllowlistItem[]>(r),
    )
  },
  setSystemAllowlistEnabled(kind: string, value: string, enabled: boolean) {
    const p = new URLSearchParams({ kind, value })
    return req(`/api/system-allowlist?${p}`, {
      method: 'PATCH',
      headers: adminHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify({ enabled }),
    }).then((r) => json<{ ok: boolean }>(r))
  },
  addSystemAllowlist(value: string, kind = 'app') {
    return req('/api/system-allowlist', {
      method: 'POST',
      headers: adminHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify({ kind, value, enabled: true }),
    }).then((r) => json<SystemAllowlistItem>(r))
  },
  addPackItem(id: string, kind: string, value: string) {
    return req(`/api/packs/${encodeURIComponent(id)}/items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ kind, value }),
    }).then((r) => json<{ ok: string }>(r))
  },
  removePackItem(id: string, kind: string, value: string) {
    const p = new URLSearchParams({ kind, value })
    return req(`/api/packs/${encodeURIComponent(id)}/items?${p}`, { method: 'DELETE' }).then(
      (r) => json<{ ok: string }>(r),
    )
  },
  addPackAssignment(
    id: string,
    body: { scope: string; group_id?: string; enrollment_id?: string },
  ) {
    return req(`/api/packs/${encodeURIComponent(id)}/assignments`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<{ ok: string }>(r))
  },
  removePackAssignment(id: string, targetType: string, targetId: string) {
    const p = new URLSearchParams({ target_type: targetType, target_id: targetId })
    return req(`/api/packs/${encodeURIComponent(id)}/assignments?${p}`, {
      method: 'DELETE',
    }).then((r) => json<{ ok: string }>(r))
  },
  profiles(enrollmentId?: string) {
    const q = enrollmentId ? `?enrollment_id=${encodeURIComponent(enrollmentId)}` : ''
    return req(`/api/profiles${q}`).then((r) => json<CustomProfile[]>(r))
  },
  createProfile(file: File, name = '', description = '') {
    const body = new FormData()
    body.append('file', file)
    if (name) body.append('name', name)
    if (description) body.append('description', description)
    return req('/api/profiles', {
      method: 'POST',
      headers: adminHeaders(),
      body,
    }).then((r) => json<CustomProfile>(r))
  },
  getProfile(id: string) {
    return req(`/api/profiles/${encodeURIComponent(id)}`).then((r) =>
      json<{
        profile: CustomProfile
        assignments: CustomProfileAssignment[]
      }>(r),
    )
  },
  updateProfile(id: string, body: { name: string; description?: string }) {
    return req(`/api/profiles/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<CustomProfile>(r))
  },
  replaceProfilePayload(id: string, file: File) {
    const body = new FormData()
    body.append('file', file)
    return req(`/api/profiles/${encodeURIComponent(id)}/payload`, {
      method: 'PUT',
      headers: adminHeaders(),
      body,
    }).then((r) => json<CustomProfile>(r))
  },
  deleteProfile(id: string) {
    return req(`/api/profiles/${encodeURIComponent(id)}`, { method: 'DELETE' }).then((r) =>
      json<{ ok: string }>(r),
    )
  },
  async downloadProfile(id: string, filename: string) {
    const res = await req(`/api/profiles/${encodeURIComponent(id)}/file`)
    if (!res.ok) {
      await json(res)
      return
    }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename || 'profile.mobileconfig'
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  },
  addProfileAssignment(
    id: string,
    body: { scope: string; group_id?: string; enrollment_id?: string },
  ) {
    return req(`/api/profiles/${encodeURIComponent(id)}/assignments`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<{ ok: string }>(r))
  },
  removeProfileAssignment(id: string, targetType: string, targetId: string) {
    const p = new URLSearchParams({ target_type: targetType, target_id: targetId })
    return req(`/api/profiles/${encodeURIComponent(id)}/assignments?${p}`, {
      method: 'DELETE',
    }).then((r) => json<{ ok: string }>(r))
  },
  timers() {
    return req('/api/timers', { headers: adminHeaders() }).then((r) => json<PolicyTimer[]>(r))
  },
  createTimer(body: PolicyTimerWrite) {
    return req('/api/timers', {
      method: 'POST',
      headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<PolicyTimer>(r))
  },
  updateTimer(id: string, body: Partial<PolicyTimerWrite>) {
    return req(`/api/timers/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<PolicyTimer>(r))
  },
  deleteTimer(id: string) {
    return req(`/api/timers/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      headers: adminHeaders(),
    }).then((r) => json<{ ok: boolean }>(r))
  },
  runTimer(id: string) {
    return req(`/api/timers/${encodeURIComponent(id)}/run`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<PolicyTimerApplyResult>(r))
  },
  devices() {
    return req('/api/devices').then((r) => json<Device[]>(r))
  },
  setDeviceName(id: string, name: string) {
    return req(`/api/devices/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    }).then((r) => json<Device>(r))
  },
  setDeviceUnrestricted(id: string, unrestricted: boolean) {
    return req(`/api/devices/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ unrestricted }),
    }).then((r) => json<Device>(r))
  },
  adminActivity(filter: ActivityFilter = {}) {
    const q = new URLSearchParams()
    if (filter.from) q.set('from', filter.from)
    if (filter.to) q.set('to', filter.to)
    if (filter.category) q.set('category', filter.category)
    if (filter.action) q.set('action', filter.action)
    if (filter.enrollment_id) q.set('enrollment_id', filter.enrollment_id)
    if (filter.actor_type) q.set('actor_type', filter.actor_type)
    if (filter.result) q.set('result', filter.result)
    if (filter.q) q.set('q', filter.q)
    if (filter.limit != null) q.set('limit', String(filter.limit))
    if (filter.offset != null) q.set('offset', String(filter.offset))
    const qs = q.toString()
    return req(`/api/admin/activity${qs ? `?${qs}` : ''}`, { headers: adminHeaders() }).then(
      (r) => json<{ events: ActivityEvent[]; limit: number; offset: number }>(r),
    )
  },
  groups() {
    return req('/api/groups').then((r) => json<Group[]>(r))
  },
  createGroup(name: string, description: string) {
    return req('/api/groups', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, description }),
    }).then((r) => json<Group>(r))
  },
  updateGroup(id: string, name: string, description: string) {
    return req(`/api/groups/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, description }),
    }).then((r) => json<Group>(r))
  },
  deleteGroup(id: string) {
    return req(`/api/groups/${id}`, { method: 'DELETE' }).then((r) => json<{ ok: string }>(r))
  },
  members(id: string) {
    return req(`/api/groups/${id}/members`).then((r) => json<string[]>(r))
  },
  setMembers(id: string, enrollment_ids: string[]) {
    return req(`/api/groups/${id}/members`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enrollment_ids }),
    }).then((r) => json<string[]>(r))
  },
  addDeviceGroup(enrollmentId: string, groupId: string) {
    return req(`/api/devices/${encodeURIComponent(enrollmentId)}/groups`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ group_id: groupId }),
    }).then((r) => json<{ ok: boolean; group_ids: string[] }>(r))
  },
  removeDeviceGroup(enrollmentId: string, groupId: string) {
    return req(
      `/api/devices/${encodeURIComponent(enrollmentId)}/groups/${encodeURIComponent(groupId)}`,
      { method: 'DELETE' },
    ).then((r) => json<{ ok: boolean; group_ids: string[] }>(r))
  },
  allowances(params: URLSearchParams) {
    return req(`/api/allowances?${params}`).then((r) => json<Allowance[]>(r))
  },
  createAllowance(body: Record<string, string>) {
    return req('/api/allowances', {
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
    return req(`/api/allowances?${p}`, { method: 'DELETE' }).then((r) => json<{ ok: string }>(r))
  },
  creditBalance(enrollmentID: string) {
    const p = new URLSearchParams({ enrollment_id: enrollmentID })
    return req(`/api/credits/balance?${p}`).then((r) =>
      json<{
        enrollment_id: string
        balance: number
        allotment_balance: number
        available: number
        access_cost: number
        enabled?: boolean
      }>(r),
    )
  },
  creditPackages() {
    return req('/api/credits/packages').then((r) => json<CreditPackage[]>(r))
  },
  creditSettings() {
    return req('/api/credits/settings').then((r) =>
      json<{ access_request_cost: number; enabled: boolean }>(r),
    )
  },
  creditCheckout(enrollmentID: string, packageID: string) {
    return req('/api/credits/checkout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enrollment_id: enrollmentID, package_id: packageID }),
    }).then((r) =>
      json<{
        purchase_id: string
        iframe_url: string
        mode: string
        credits: number
        amount_agorot: number
      }>(r),
    )
  },
  creditConfirm(enrollmentID: string, purchaseID: string) {
    return req('/api/credits/confirm', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enrollment_id: enrollmentID, purchase_id: purchaseID }),
    }).then((r) => json<{ purchase: unknown; balance: number }>(r))
  },
  adminGiftCredits(enrollmentID: string, amount: number, note?: string) {
    return req('/api/admin/credits/gift', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enrollment_id: enrollmentID, amount, note: note || '' }),
    }).then((r) =>
      json<{
        balance: number
        allotment_balance: number
        available: number
        applied: boolean
        entry: CreditLedgerEntry
      }>(r),
    )
  },
  adminAdjustCredits(enrollmentID: string, amount: number, note?: string) {
    return req('/api/admin/credits/adjust', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enrollment_id: enrollmentID, amount, note: note || '' }),
    }).then((r) =>
      json<{
        enrollment_id: string
        balance: number
        allotment_balance: number
        available: number
        applied: boolean
        entry: CreditLedgerEntry
        ledger: CreditLedgerEntry[]
      }>(r),
    )
  },
  adminCredits() {
    return req('/api/admin/credits').then((r) =>
      json<
        {
          enrollment_id: string
          balance: number
          allotment_balance?: number
          available?: number
          updated_at?: string
        }[]
      >(r),
    )
  },
  adminCreditDevice(enrollmentID: string, ledgerLimit = 20) {
    const p = new URLSearchParams({
      enrollment_id: enrollmentID,
      ledger_limit: String(ledgerLimit),
    })
    return req(`/api/admin/credits?${p}`).then((r) =>
      json<{
        enrollment_id: string
        balance: number
        allotment_balance: number
        available: number
        updated_at?: string
        ledger: CreditLedgerEntry[]
      }>(r),
    )
  },
  adminCreditSettings() {
    return req('/api/admin/credits/settings').then((r) => json<CreditSettings>(r))
  },
  adminUpdateCreditSettings(accessRequestCost: number, enabled?: boolean) {
    const body: { access_request_cost: number; enabled?: boolean } = {
      access_request_cost: accessRequestCost,
    }
    if (enabled !== undefined) body.enabled = enabled
    return req('/api/admin/credits/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<CreditSettings>(r))
  },
  adminCreditPurchases(filter: {
    enrollment_id?: string
    status?: string
    limit?: number
    offset?: number
  } = {}) {
    const q = new URLSearchParams()
    if (filter.enrollment_id) q.set('enrollment_id', filter.enrollment_id)
    if (filter.status) q.set('status', filter.status)
    if (filter.limit != null) q.set('limit', String(filter.limit))
    if (filter.offset != null) q.set('offset', String(filter.offset))
    const qs = q.toString()
    return req(`/api/admin/credits/purchases${qs ? `?${qs}` : ''}`, {
      headers: adminHeaders(),
    }).then((r) => json<{ purchases: CreditPurchase[]; limit: number; offset: number }>(r))
  },
  adminCreditPackages() {
    return req('/api/admin/credits/packages').then((r) => json<CreditPackage[]>(r))
  },
  adminCreateCreditPackage(pkg: {
    name_he: string
    credits: number
    price_agorot: number
    active?: boolean
    sort_order?: number
  }) {
    return req('/api/admin/credits/packages', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(pkg),
    }).then((r) => json<CreditPackage>(r))
  },
  adminUpdateCreditPackage(
    id: string,
    patch: {
      name_he?: string
      credits?: number
      price_agorot?: number
      active?: boolean
      sort_order?: number
    },
  ) {
    return req(`/api/admin/credits/packages/${encodeURIComponent(id)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(patch),
    }).then((r) => json<CreditPackage>(r))
  },
  adminDeactivateCreditPackage(id: string) {
    return req(`/api/admin/credits/packages/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    }).then((r) => json<CreditPackage>(r))
  },
  adminAllotments() {
    return req('/api/admin/credits/allotments').then((r) => json<CreditAllotmentRule[]>(r))
  },
  adminCreateAllotment(rule: {
    name?: string
    note?: string
    amount: number
    interval: AllotmentInterval
    target_type: AllotmentTargetType
    target_id?: string
    enabled?: boolean
  }) {
    return req('/api/admin/credits/allotments', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(rule),
    }).then((r) => json<CreditAllotmentRule>(r))
  },
  adminUpdateAllotment(
    id: string,
    patch: {
      name?: string
      note?: string
      amount?: number
      interval?: AllotmentInterval
      target_type?: AllotmentTargetType
      target_id?: string
      enabled?: boolean
    },
  ) {
    return req(`/api/admin/credits/allotments/${encodeURIComponent(id)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(patch),
    }).then((r) => json<CreditAllotmentRule>(r))
  },
  adminDeleteAllotment(id: string) {
    return req(`/api/admin/credits/allotments/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    }).then((r) => json<{ ok: string }>(r))
  },
  adminRunAllotments() {
    return req('/api/admin/credits/allotments/run', { method: 'POST' }).then((r) =>
      json<{
        rules_processed: number
        grants_applied: number
        grants_skipped: number
        errors: number
      }>(r),
    )
  },
  mdmStatus() {
    return req('/api/mdm/status', { headers: adminHeaders() }).then((r) => json<MdmStatus>(r))
  },
  mdmDevices() {
    return req('/api/mdm/devices', { headers: adminHeaders() }).then((r) =>
      json<MdmEnrollment[]>(r),
    )
  },
  mdmPush(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/push`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string }>(r))
  },
  mdmReconcile(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/reconcile`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string }>(r))
  },
  mdmClearAllowlist(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/clear-allowlist`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string }>(r))
  },
  mdmLock(id: string, body?: { pin?: string; message?: string }) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/lock`, {
      method: 'POST',
      headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    }).then((r) => json<{ status: string }>(r))
  },
  mdmClearPasscode(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/clear-passcode`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string }>(r))
  },
  mdmRestart(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/restart`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string }>(r))
  },
  mdmShutDown(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/shutdown`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string }>(r))
  },
  mdmErase(id: string, body?: { pin?: string }) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/erase`, {
      method: 'POST',
      headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    }).then((r) => json<{ status: string }>(r))
  },
  mdmEnableLostMode(id: string, body: { message: string; phone?: string; footnote?: string }) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/lost-mode/enable`, {
      method: 'POST',
      headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<{ status: string }>(r))
  },
  mdmDisableLostMode(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/lost-mode/disable`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string }>(r))
  },
  mdmPlayLostModeSound(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/lost-mode/play-sound`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string }>(r))
  },
  mdmDeviceLocation(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/lost-mode/location`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string; command_uuid: string }>(r))
  },
  mdmSecurityInfo(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/security-info`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string; command_uuid: string }>(r))
  },
  mdmProfileList(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/profile-list`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string; command_uuid: string }>(r))
  },
  mdmInstalledApps(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/installed-apps`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string; command_uuid: string }>(r))
  },
  mdmBulk(body: {
    enrollment_ids: string[]
    op: string
    pin?: string
    message?: string
    phone?: string
    footnote?: string
    group_id?: string
  }) {
    return req('/api/mdm/devices/bulk', {
      method: 'POST',
      headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<{ results: { id: string; ok: boolean; error?: string }[] }>(r))
  },
  effectiveAllowlist(enrollmentId: string) {
    return req(`/api/allowlist?enrollment_id=${encodeURIComponent(enrollmentId)}`).then((r) =>
      json<{ apps: string[]; urls: string[] }>(r),
    )
  },
  abmAccount() {
    return req('/api/mdm/abm/account', { headers: adminHeaders() }).then((r) =>
      json<AbmAccount>(r),
    )
  },
  abmSettings() {
    return req('/api/mdm/abm/settings', { headers: adminHeaders() }).then((r) =>
      json<MdmSettings>(r),
    )
  },
  abmPutSettings(body: Partial<MdmSettings> & { dep_name?: string }) {
    return req('/api/mdm/abm/settings', {
      method: 'PUT',
      headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<MdmSettings>(r))
  },
  uploadVppToken(tokenText: string, filename?: string) {
    return req('/api/mdm/vpp/token', {
      method: 'PUT',
      headers: adminHeaders({
        'Content-Type': 'text/plain',
        ...(filename ? { 'X-Filename': filename } : {}),
      }),
      body: tokenText,
    }).then((r) =>
      json<{ ok: boolean; has_vpp_token: boolean; vpp_token_filename?: string }>(r),
    )
  },
  deleteVppToken() {
    return req('/api/mdm/vpp/token', { method: 'DELETE', headers: adminHeaders() }).then((r) =>
      json<{ ok: boolean; has_vpp_token: boolean }>(r),
    )
  },
  mdmInstallCompanion(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/install-companion`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string }>(r))
  },
  mdmConfigureCompanion(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/configure-companion`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string }>(r))
  },
  mdmGetDevice(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}`, { headers: adminHeaders() }).then(
      (r) =>
        json<{
          id: string
          serial_number?: string
          has_push_token?: boolean
          enabled?: boolean
          last_seen_at?: string
        }>(r),
    )
  },
  abmDepNames() {
    return req('/api/mdm/abm/dep-names', { headers: adminHeaders() }).then((r) =>
      json<{ dep_name: string; dep_names: string[] }>(r),
    )
  },
  abmDevices() {
    return req('/api/mdm/abm/devices', { headers: adminHeaders() }).then((r) =>
      json<AbmDeviceSync>(r),
    )
  },
  abmSync() {
    return req('/api/mdm/abm/sync', { method: 'POST', headers: adminHeaders() }).then((r) =>
      json<AbmDeviceSync>(r),
    )
  },
  abmGetProfile(profileUUID?: string) {
    const q = profileUUID?.trim()
      ? `?profile_uuid=${encodeURIComponent(profileUUID.trim())}`
      : ''
    return req(`/api/mdm/abm/profile${q}`, { headers: adminHeaders() }).then((r) =>
      json<{ profile_uuid: string; profile: Record<string, unknown> }>(r),
    )
  },
  abmDefineProfile(body: { profile_name: string; url?: string }) {
    return req('/api/mdm/abm/profile', {
      method: 'POST',
      headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<{ profile_uuid?: string }>(r))
  },
  abmAssign(profileUUID: string, devices: string[]) {
    return req('/api/mdm/abm/assign', {
      method: 'POST',
      headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({ profile_uuid: profileUUID, devices }),
    }).then((r) => json<unknown>(r))
  },
  async abmDownloadPublicKey(depName: string) {
    const name = encodeURIComponent(depName || 'nanok')
    const res = await req(`/dep/v1/tokenpki/${name}`)
    if (!res.ok) {
      const text = await res.text().catch(() => '')
      throw new Error(text || `download cert failed (${res.status})`)
    }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${depName || 'nanok'}-dep.pem`
    a.click()
    URL.revokeObjectURL(url)
  },
  async abmUploadToken(depName: string, file: File) {
    const name = encodeURIComponent(depName || 'nanok')
    const body = await file.arrayBuffer()
    const res = await req(`/dep/v1/tokenpki/${name}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/octet-stream',
      },
      body,
    })
    if (!res.ok) {
      const text = await res.text().catch(() => '')
      throw new Error(text || `upload token failed (${res.status})`)
    }
    return res.json().catch(() => ({ ok: true }))
  },
  mdmDeviceInformation(id: string) {
    return req(`/api/mdm/devices/${encodeURIComponent(id)}/device-information`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<{ status: string; command_uuid: string }>(r))
  },
  mdmCommandResult(id: string, commandUUID: string) {
    return req(
      `/api/mdm/devices/${encodeURIComponent(id)}/commands/${encodeURIComponent(commandUUID)}`,
      { headers: adminHeaders() },
    ).then((r) => json<MdmCommandResult>(r))
  },
  openapi() {
    return req('/api/openapi.json').then((r) => json<OpenAPISpec>(r))
  },
  webhookEvents() {
    return req('/api/webhooks/events').then((r) =>
      json<{ events: WebhookEventInfo[]; filters: string[] }>(r),
    )
  },
  webhooks() {
    return req('/api/webhooks', { headers: adminHeaders() }).then((r) =>
      json<{ endpoints: WebhookEndpoint[] }>(r),
    )
  },
  createWebhook(body: {
    url: string
    secret?: string
    description?: string
    events?: string[]
    enabled?: boolean
  }) {
    return req('/api/webhooks', {
      method: 'POST',
      headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<WebhookEndpoint>(r))
  },
  updateWebhook(
    id: string,
    body: {
      url?: string
      secret?: string
      description?: string
      events?: string[]
      enabled?: boolean
    },
  ) {
    return req(`/api/webhooks/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      headers: { ...adminHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((r) => json<WebhookEndpoint>(r))
  },
  deleteWebhook(id: string) {
    return req(`/api/webhooks/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      headers: adminHeaders(),
    }).then((r) => json<{ ok: boolean }>(r))
  },
  webhookDeliveries(id: string, limit = 50) {
    return req(`/api/webhooks/${encodeURIComponent(id)}/deliveries?limit=${limit}`, {
      headers: adminHeaders(),
    }).then((r) => json<{ deliveries: WebhookDelivery[] }>(r))
  },
  testWebhook(id: string) {
    return req(`/api/webhooks/${encodeURIComponent(id)}/test`, {
      method: 'POST',
      headers: adminHeaders(),
    }).then((r) => json<WebhookDelivery>(r))
  },
  apiTokens() {
    return req('/api/admin/tokens').then((r) => json<{ tokens: APIToken[] }>(r))
  },
  createAPIToken(name: string) {
    return req('/api/admin/tokens', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    }).then((r) => json<APIToken & { token: string; token_hint?: string }>(r))
  },
  deleteAPIToken(id: string) {
    return req(`/api/admin/tokens/${encodeURIComponent(id)}`, { method: 'DELETE' }).then((r) =>
      json<{ ok: boolean }>(r),
    )
  },
}

export type CreditPackage = {
  id: string
  name_he: string
  credits: number
  price_agorot: number
  active: boolean
  sort_order: number
}

export type CreditPurchase = {
  id: string
  enrollment_id: string
  package_id: string
  credits: number
  amount_agorot: number
  status: string
  provider: string
  provider_tx_id?: string
  client_unique_id?: string
  created_at: string
  paid_at?: string
  device_name?: string
  package_name?: string
}

export type CreditSettings = {
  access_request_cost: number
  enabled: boolean
  updated_at?: string
}

export type AllotmentInterval = 'daily' | 'weekly' | 'monthly'
export type AllotmentTargetType = 'everyone' | 'group' | 'individual'

export type CreditAllotmentRule = {
  id: string
  name: string
  note?: string
  amount: number
  interval: AllotmentInterval
  target_type: AllotmentTargetType
  target_id: string
  enabled: boolean
  last_run_at?: string
  created_at: string
  updated_at: string
  period_key?: string
  next_period_at?: string
}

export type CreditLedgerEntry = {
  id: string
  enrollment_id: string
  delta: number
  balance_after: number
  reason: string
  ref_type?: string
  ref_id?: string
  note?: string
  created_at: string
}

export type WebhookEndpoint = {
  id: string
  url: string
  secret?: string
  description: string
  events: string[]
  enabled: boolean
  created_at: string
}

export type WebhookDelivery = {
  id: string
  endpoint_id: string
  event_id: string
  event_name: string
  status: string
  attempt: number
  http_status: number
  error?: string
  created_at: string
}

export type WebhookEventInfo = {
  name: string
  category: string
  action: string
  description: string
}

export type OpenAPIParameter = {
  name: string
  in: 'path' | 'query' | 'header'
  required?: boolean
  description?: string
  schema?: { type?: string }
}

export type OpenAPIOperation = {
  tags?: string[]
  summary?: string
  description?: string
  operationId?: string
  security?: unknown[]
  parameters?: OpenAPIParameter[]
  requestBody?: {
    required?: boolean
    content?: {
      'application/json'?: { schema?: { type?: string; properties?: Record<string, unknown> } }
    }
  }
}

export type OpenAPISpec = {
  openapi?: string
  info: { title: string; description?: string; version: string }
  tags?: { name: string; description?: string }[]
  paths: Record<string, Record<string, OpenAPIOperation>>
}
