import {
  InboxOutlined,
  MobileOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  TeamOutlined,
  UnorderedListOutlined,
  WalletOutlined,
} from '@ant-design/icons'
import {
  Alert,
  App,
  Badge,
  Button,
  Card,
  Checkbox,
  Col,
  Drawer,
  Empty,
  Flex,
  Input,
  InputNumber,
  List,
  Modal,
  Row,
  Segmented,
  Select,
  Skeleton,
  Space,
  Spin,
  Switch,
  Tabs,
  Tag,
  Typography,
} from 'antd'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  parseAsString,
  parseAsStringLiteral,
  useQueryState,
  useQueryStates,
} from 'nuqs'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  api,
  getAdminToken,
  setAdminToken,
  type Allowance,
  type AllotmentInterval,
  type AllotmentTargetType,
  type AppMeta,
  type CreditAllotmentRule,
  type CreditLedgerEntry,
  type CreditPackage,
  type Device,
  type Group,
  type Request,
  type WhitelistPack,
} from '../api'
import { AppSearchPicker } from '../components/AppSearchPicker'
import { DeviceActionModals } from '../components/DeviceActionModals'
import { DeviceMdmActions } from '../components/DeviceMdmActions'
import { RequestThread } from '../components/RequestThread'
import { useIsMobile } from '../hooks/useIsMobile'
import { useMdmDeviceActions } from '../hooks/useMdmDeviceActions'
import { he, statusClass, adminNextAction } from '../he'
import {
  deviceLabel,
  deviceMatches,
  deviceOptions,
  groupMatches,
  groupOptions,
  searchableSelect,
} from '../labels'
import { formatRelativeHe } from '../time'
import AdminCreditPurchases from './AdminCreditPurchases'
import AdminEnrollment from './AdminEnrollment'
import AdminLogs from './AdminLogs'
import { AppThumb, useDebounced } from '../ui'

const tabKeys = ['requests', 'devices', 'whitelists', 'groups', 'credits', 'logs', 'enrollment', 'settings'] as const
type TabKey = (typeof tabKeys)[number]

const allowScopes = ['global', 'group', 'device', 'all'] as const

const EMPTY_REQUESTS: Request[] = []
const EMPTY_GROUPS: Group[] = []
const EMPTY_DEVICES: Device[] = []
const EMPTY_ALLOWANCES: Allowance[] = []

function labelDevice(d: Device | string, devices: Device[]) {
  return deviceLabel(d, devices)
}

function deviceSub(d: Device | string, devices: Device[]) {
  if (typeof d === 'string') {
    const found = devices.find((x) => x.enrollment_id === d)
    return found?.name ? d : ''
  }
  return d.name ? d.enrollment_id : ''
}

function sourceLabel(src: string) {
  if (src === 'essential') return he.sourceEssential
  if (src === 'global') return he.sourceGlobal
  if (src === 'group') return he.sourceGroup
  if (src === 'device') return he.sourceDevice
  if (src === 'grant') return he.sourceGrant
  return src
}

function whyLine(row: Allowance, groups: Group[], devices: Device[]) {
  const src = sourceLabel(row.source)
  if (row.source === 'essential') return src
  if (row.group_id) {
    const g = groups.find((x) => x.id === row.group_id)
    return `${src}${g ? ` · ${g.name}` : ''}`
  }
  if (row.enrollment_id) return `${src} · ${labelDevice(row.enrollment_id, devices)}`
  if (row.expires_at) {
    try {
      return `${src} · עד ${new Date(row.expires_at).toLocaleString('he-IL')}`
    } catch {
      return src
    }
  }
  return src
}

function nextTagColor(kind: string, status: string) {
  if (kind === 'act') return 'processing'
  if (kind === 'wait') return 'warning'
  if (statusClass(status) === 'bad') return 'error'
  return 'success'
}

function sortRequests(list: Request[]) {
  const next = [...list]
  next.sort((a, b) => {
    const score = (r: Request) => {
      if (r.status !== 'pending') return 2
      if (r.last_message?.author_role === 'admin') return 1
      return 0
    }
    const d = score(a) - score(b)
    if (d !== 0) return d
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  })
  return next
}

function LoadingBlock({ tip }: { tip?: string }) {
  return (
    <div style={{ textAlign: 'center', padding: 32 }}>
      <Spin tip={tip} />
    </div>
  )
}

export default function Admin() {
  const { message, modal } = App.useApp()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const mdm = useMdmDeviceActions()

  const [tab, setTab] = useQueryState(
    'tab',
    parseAsStringLiteral(tabKeys).withDefault('requests'),
  )
  const [reqFilters, setReqFilters] = useQueryStates({
    status: parseAsString.withDefault('pending'),
    type: parseAsString.withDefault('all'),
    device: parseAsString.withDefault(''),
    q: parseAsString.withDefault(''),
  })
  const [selectedReqId, setSelectedReqId] = useQueryState('request', parseAsString)
  const [selectedGroupId, setSelectedGroupId] = useQueryState('group', parseAsString)
  const [allowFilters, setAllowFilters] = useQueryStates({
    ascope: parseAsStringLiteral(allowScopes).withDefault('global'),
    akind: parseAsString.withDefault('app'),
    adevice: parseAsString.withDefault(''),
    agroup: parseAsString.withDefault(''),
    aq: parseAsString.withDefault(''),
  })

  const debouncedReqQ = useDebounced(reqFilters.q, 300)
  const debouncedAllowQ = useDebounced(allowFilters.aq, 300)

  const [approveScope, setApproveScope] = useState<Record<string, string>>({})
  const [approveGroup, setApproveGroup] = useState<Record<string, string>>({})

  const [newName, setNewName] = useState('')
  const [newDesc, setNewDesc] = useState('')
  const [members, setMembers] = useState<string[]>([])
  const [memberFilter, setMemberFilter] = useState('')
  const [renameDraft, setRenameDraft] = useState('')

  const [addOpen, setAddOpen] = useState(false)
  const [addKind, setAddKind] = useState('url')
  const [addScope, setAddScope] = useState('global')
  const [addGroup, setAddGroup] = useState('')
  const [addDevice, setAddDevice] = useState('')
  const [addDuration, setAddDuration] = useState('permanent')
  const [addValue, setAddValue] = useState('')
  const [addApp, setAddApp] = useState<AppMeta | null>(null)
  const [giftDevice, setGiftDevice] = useState('')
  const [giftAmount, setGiftAmount] = useState('10')
  const [giftNote, setGiftNote] = useState('')
  const [gifting, setGifting] = useState(false)
  const [accessCostDraft, setAccessCostDraft] = useState<number | null>(null)
  const [savingSettings, setSavingSettings] = useState(false)
  const [pkgModalOpen, setPkgModalOpen] = useState(false)
  const [editingPkg, setEditingPkg] = useState<CreditPackage | null>(null)
  const [pkgName, setPkgName] = useState('')
  const [pkgCredits, setPkgCredits] = useState(10)
  const [pkgPriceIls, setPkgPriceIls] = useState(10)
  const [pkgSort, setPkgSort] = useState(10)
  const [pkgActive, setPkgActive] = useState(true)
  const [savingPkg, setSavingPkg] = useState(false)
  const [allotModalOpen, setAllotModalOpen] = useState(false)
  const [editingAllot, setEditingAllot] = useState<CreditAllotmentRule | null>(null)
  const [allotName, setAllotName] = useState('')
  const [allotNote, setAllotNote] = useState('')
  const [allotAmount, setAllotAmount] = useState(5)
  const [allotInterval, setAllotInterval] = useState<AllotmentInterval>('daily')
  const [allotTargetType, setAllotTargetType] = useState<AllotmentTargetType>('everyone')
  const [allotTargetID, setAllotTargetID] = useState('')
  const [allotEnabled, setAllotEnabled] = useState(true)
  const [savingAllot, setSavingAllot] = useState(false)
  const [runningAllot, setRunningAllot] = useState(false)
  const [adminTokenDraft, setAdminTokenDraft] = useState(() => getAdminToken())
  const [lostModeOpen, setLostModeOpen] = useState(false)
  const [eraseOpen, setEraseOpen] = useState(false)
  const [deviceDrawerId, setDeviceDrawerId] = useState('')
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [bulkGroupId, setBulkGroupId] = useState('')
  const [urlPaste, setUrlPaste] = useState('')
  const [lockScreenEnabledDraft, setLockScreenEnabledDraft] = useState(true)
  const [lockScreenFootnoteDraft, setLockScreenFootnoteDraft] = useState('מכשיר בית ספר · KFilter')
  const [whitelistMode, setWhitelistMode] = useQueryState(
    'wmode',
    parseAsStringLiteral(['packs', 'oneoffs'] as const).withDefault('packs'),
  )
  const [deviceSearch, setDeviceSearch] = useState('')
  const [groupSearch, setGroupSearch] = useState('')
  const [packSearch, setPackSearch] = useState('')
  const [selectedPackId, setSelectedPackId] = useState('')
  const [newPackName, setNewPackName] = useState('')
  const [packAssignScope, setPackAssignScope] = useState<'global' | 'group' | 'device'>('group')
  const [packAssignTarget, setPackAssignTarget] = useState('')
  const [packItemKind, setPackItemKind] = useState<'app' | 'url'>('app')
  const [packItemValue, setPackItemValue] = useState('')

  const devicesQuery = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.devices(),
  })
  const groupsQuery = useQuery({
    queryKey: ['groups'],
    queryFn: () => api.groups(),
  })
  const packsQuery = useQuery({
    queryKey: ['packs'],
    queryFn: () => api.packs(),
    enabled: tab === 'whitelists',
  })
  const packDetailQuery = useQuery({
    queryKey: ['pack', selectedPackId],
    queryFn: () => api.getPack(selectedPackId),
    enabled: tab === 'whitelists' && whitelistMode === 'packs' && !!selectedPackId,
  })
  const creditSettingsQuery = useQuery({
    queryKey: ['admin-credit-settings'],
    queryFn: () => api.adminCreditSettings(),
    enabled: tab === 'credits',
  })
  const creditPackagesQuery = useQuery({
    queryKey: ['admin-credit-packages'],
    queryFn: () => api.adminCreditPackages(),
    enabled: tab === 'credits',
  })
  const allotmentsQuery = useQuery({
    queryKey: ['admin-credit-allotments'],
    queryFn: () => api.adminAllotments(),
    enabled: tab === 'credits',
  })
  const creditDeviceQuery = useQuery({
    queryKey: ['admin-credit-device', giftDevice],
    queryFn: () => api.adminCreditDevice(giftDevice),
    enabled: tab === 'credits' && !!giftDevice,
  })
  const abmSettingsQuery = useQuery({
    queryKey: ['abm-settings', getAdminToken()],
    queryFn: () => api.abmSettings(),
    enabled: tab === 'settings' && !!getAdminToken(),
    retry: false,
  })
  useEffect(() => {
    const s = abmSettingsQuery.data
    if (!s) return
    if (typeof s.lock_screen_enabled === 'boolean') setLockScreenEnabledDraft(s.lock_screen_enabled)
    if (typeof s.lock_screen_footnote === 'string') setLockScreenFootnoteDraft(s.lock_screen_footnote)
  }, [abmSettingsQuery.data])
  const devices = devicesQuery.data ?? EMPTY_DEVICES
  const groups = groupsQuery.data ?? EMPTY_GROUPS
  const drawerDevice = devices.find((d) => d.enrollment_id === deviceDrawerId)

  async function runBulk(op: string, extra?: { group_id?: string; message?: string }) {
    if (!selectedIds.length) return
    try {
      const res = await api.mdmBulk({
        enrollment_ids: selectedIds,
        op,
        ...extra,
      })
      const failed = res.results.filter((r) => !r.ok)
      if (failed.length) message.error(failed.map((f) => `${f.id}: ${f.error}`).join('; '))
      else message.success(he.ok)
      void qc.invalidateQueries({ queryKey: ['devices'] })
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  useEffect(() => {
    if (creditSettingsQuery.data && accessCostDraft === null) {
      setAccessCostDraft(creditSettingsQuery.data.access_request_cost)
    }
  }, [creditSettingsQuery.data, accessCostDraft])

  function openPackageModal(pkg?: CreditPackage) {
    if (pkg) {
      setEditingPkg(pkg)
      setPkgName(pkg.name_he)
      setPkgCredits(pkg.credits)
      setPkgPriceIls(pkg.price_agorot / 100)
      setPkgSort(pkg.sort_order)
      setPkgActive(pkg.active)
    } else {
      setEditingPkg(null)
      setPkgName('')
      setPkgCredits(10)
      setPkgPriceIls(10)
      setPkgSort(40)
      setPkgActive(true)
    }
    setPkgModalOpen(true)
  }

  function openAllotmentModal(rule?: CreditAllotmentRule) {
    if (rule) {
      setEditingAllot(rule)
      setAllotName(rule.name || '')
      setAllotNote(rule.note || '')
      setAllotAmount(rule.amount)
      setAllotInterval(rule.interval)
      setAllotTargetType(rule.target_type)
      setAllotTargetID(rule.target_id || '')
      setAllotEnabled(rule.enabled)
    } else {
      setEditingAllot(null)
      setAllotName('')
      setAllotNote('')
      setAllotAmount(5)
      setAllotInterval('daily')
      setAllotTargetType('everyone')
      setAllotTargetID('')
      setAllotEnabled(true)
    }
    setAllotModalOpen(true)
  }

  function allotmentTargetLabel(rule: CreditAllotmentRule) {
    if (rule.target_type === 'everyone') return he.allotmentTargetEveryone
    if (rule.target_type === 'group') {
      const g = groups.find((x) => x.id === rule.target_id)
      return `${he.allotmentTargetGroup}: ${g?.name || rule.target_id}`
    }
    const d = devices.find((x) => x.enrollment_id === rule.target_id)
    return `${he.allotmentTargetIndividual}: ${d ? labelDevice(d, devices) : rule.target_id}`
  }

  function allotmentIntervalLabel(interval: string) {
    if (interval === 'weekly') return he.allotmentIntervalWeekly
    if (interval === 'monthly') return he.allotmentIntervalMonthly
    return he.allotmentIntervalDaily
  }

  function ledgerReasonLabel(reason: string) {
    const map: Record<string, string> = {
      gift: 'הענקה',
      adjust: 'התאמה',
      purchase: 'רכישה',
      spend: 'חיוב',
      refund: 'החזר',
      allotment: 'הקצאה תקופתית',
      allotment_expire: 'פקיעת הקצאה',
    }
    return map[reason] || reason
  }

  const pendingQuery = useQuery({
    queryKey: ['requests', 'pending-count'],
    queryFn: async () => {
      const list = await api.requests(new URLSearchParams({ status: 'pending' }))
      return list.length
    },
  })
  const pendingCount = pendingQuery.data ?? 0

  const requestsQuery = useQuery({
    queryKey: ['requests', reqFilters.status, reqFilters.type, reqFilters.device, debouncedReqQ],
    queryFn: async () => {
      const p = new URLSearchParams({
        status: reqFilters.status,
        type: reqFilters.type,
        sort: 'created_desc',
      })
      if (reqFilters.device) p.set('enrollment_id', reqFilters.device)
      if (debouncedReqQ.trim()) p.set('q', debouncedReqQ.trim())
      return sortRequests(await api.requests(p))
    },
    enabled: tab === 'requests',
    placeholderData: keepPreviousData,
  })
  const requests = requestsQuery.data ?? EMPTY_REQUESTS

  const allowancesEnabled =
    tab === 'whitelists' &&
    !(allowFilters.ascope === 'device' && !allowFilters.adevice) &&
    !(allowFilters.ascope === 'group' && !allowFilters.agroup)

  const allowancesQuery = useQuery({
    queryKey: [
      'allowances',
      allowFilters.ascope,
      allowFilters.akind,
      allowFilters.adevice,
      allowFilters.agroup,
      debouncedAllowQ,
    ],
    queryFn: async () => {
      const p = new URLSearchParams({ scope: allowFilters.ascope, kind: allowFilters.akind })
      if (allowFilters.ascope === 'device') p.set('enrollment_id', allowFilters.adevice)
      if (allowFilters.ascope === 'group') p.set('group_id', allowFilters.agroup)
      if (debouncedAllowQ.trim()) p.set('q', debouncedAllowQ.trim())
      return api.allowances(p)
    },
    enabled: allowancesEnabled,
    placeholderData: keepPreviousData,
  })
  const allowances =
    tab === 'whitelists' &&
    ((allowFilters.ascope === 'device' && !allowFilters.adevice) ||
      (allowFilters.ascope === 'group' && !allowFilters.agroup))
      ? EMPTY_ALLOWANCES
      : (allowancesQuery.data ?? EMPTY_ALLOWANCES)

  const membersQuery = useQuery({
    queryKey: ['group-members', selectedGroupId],
    queryFn: () => api.members(selectedGroupId!),
    enabled: tab === 'groups' && !!selectedGroupId,
  })

  useEffect(() => {
    if (requestsQuery.isError) message.error((requestsQuery.error as Error).message)
  }, [requestsQuery.isError, requestsQuery.error, message])

  useEffect(() => {
    if (allowancesQuery.isError) message.error((allowancesQuery.error as Error).message)
  }, [allowancesQuery.isError, allowancesQuery.error, message])

  useEffect(() => {
    if (devicesQuery.isError) message.error((devicesQuery.error as Error).message)
  }, [devicesQuery.isError, devicesQuery.error, message])

  useEffect(() => {
    if (groupsQuery.isError) message.error((groupsQuery.error as Error).message)
  }, [groupsQuery.isError, groupsQuery.error, message])

  useEffect(() => {
    setApproveScope((prev) => {
      let changed = false
      const next = { ...prev }
      for (const r of requests) {
        if (!next[r.id]) {
          next[r.id] = 'device'
          changed = true
        }
      }
      return changed ? next : prev
    })
  }, [requests])

  useEffect(() => {
    if (groups.length !== 1) return
    const onlyGroupId = groups[0].id
    setApproveGroup((prev) => {
      let changed = false
      const next = { ...prev }
      for (const r of requests) {
        if (!next[r.id]) {
          next[r.id] = onlyGroupId
          changed = true
        }
      }
      return changed ? next : prev
    })
  }, [groups, requests])

  useEffect(() => {
    if (!selectedReqId || !requestsQuery.isSuccess) return
    if (!requests.some((r) => r.id === selectedReqId)) {
      void setSelectedReqId(null)
    }
  }, [selectedReqId, requests, requestsQuery.isSuccess, setSelectedReqId])

  const selectedGroup = useMemo(
    () => groups.find((g) => g.id === selectedGroupId) || null,
    [groups, selectedGroupId],
  )

  useEffect(() => {
    if (!selectedGroup) return
    setRenameDraft(selectedGroup.name)
  }, [selectedGroup])

  useEffect(() => {
    if (membersQuery.data) setMembers(membersQuery.data)
  }, [membersQuery.data])

  const filteredDevices = useMemo(
    () => devices.filter((d) => deviceMatches(d, memberFilter)),
    [devices, memberFilter],
  )
  const visibleDevices = useMemo(
    () => devices.filter((d) => deviceMatches(d, deviceSearch)),
    [devices, deviceSearch],
  )
  const visibleGroups = useMemo(
    () => groups.filter((g) => groupMatches(g, groupSearch)),
    [groups, groupSearch],
  )
  const visiblePacks = useMemo(() => {
    const q = packSearch.trim().toLowerCase()
    const list = packsQuery.data ?? []
    if (!q) return list
    return list.filter(
      (p) =>
        p.name.toLowerCase().includes(q) ||
        (p.description || '').toLowerCase().includes(q),
    )
  }, [packsQuery.data, packSearch])

  const isMobile = useIsMobile()

  const selectedReq =
    requests.find((x) => x.id === selectedReqId) ||
    (!isMobile && !selectedReqId ? requests[0] ?? null : null)

  async function refreshMeta() {
    await Promise.all([
      qc.invalidateQueries({ queryKey: ['devices'] }),
      qc.invalidateQueries({ queryKey: ['groups'] }),
      qc.invalidateQueries({ queryKey: ['requests', 'pending-count'] }),
    ])
  }

  const decideMutation = useMutation({
    mutationFn: async ({
      id,
      approve,
      duration,
    }: {
      id: string
      approve: boolean
      duration?: string
    }) => {
      const body: Record<string, string> = { duration: duration || '' }
      if (approve) {
        body.scope = approveScope[id] || 'device'
        if (body.scope === 'group') body.group_id = approveGroup[id] || ''
      }
      return api.decide(id, approve, body)
    },
    onSuccess: async (_data, vars) => {
      message.success(he.ok)
      if (selectedReqId === vars.id) void setSelectedReqId(null)
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['requests'] }),
        refreshMeta(),
      ])
    },
    onError: (e) => message.error((e as Error).message),
  })

  const unrestrictedMutation = useMutation({
    mutationFn: ({ id, on }: { id: string; on: boolean }) => api.setDeviceUnrestricted(id, on),
    onMutate: async ({ id, on }) => {
      await qc.cancelQueries({ queryKey: ['devices'] })
      const prev = qc.getQueryData<Device[]>(['devices'])
      qc.setQueryData<Device[]>(['devices'], (old) =>
        (old ?? []).map((d) => (d.enrollment_id === id ? { ...d, unrestricted: on } : d)),
      )
      return { prev }
    },
    onError: (err, _vars, ctx) => {
      if (ctx?.prev) qc.setQueryData(['devices'], ctx.prev)
      message.error((err as Error).message)
    },
    onSuccess: (_data, vars) => {
      void qc.invalidateQueries({ queryKey: ['effective-allowlist', vars.id] })
    },
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: ['devices'] })
    },
  })

  async function openGroup(g: Group) {
    await setSelectedGroupId(g.id)
  }

  const metaLoading =
    (tab === 'groups' || tab === 'devices' || tab === 'credits') &&
    (devicesQuery.isLoading || groupsQuery.isLoading)
  const requestsLoading = tab === 'requests' && requestsQuery.isLoading && !requestsQuery.data
  const requestsFetching = tab === 'requests' && requestsQuery.isFetching
  const allowancesLoading =
    tab === 'whitelists' && allowancesEnabled && allowancesQuery.isLoading && !allowancesQuery.data
  const allowancesFetching = tab === 'whitelists' && allowancesEnabled && allowancesQuery.isFetching
  const showQueue = !isMobile || !selectedReqId
  const showTicket = !isMobile || !!selectedReqId

  const adminTokenPresent = !!getAdminToken()

  const mobileNavItems: { key: TabKey; label: string; icon: ReactNode }[] = [
    { key: 'requests', label: he.tabRequests, icon: <InboxOutlined /> },
    { key: 'groups', label: he.tabGroups, icon: <TeamOutlined /> },
    { key: 'whitelists', label: he.tabAllowShort, icon: <SafetyCertificateOutlined /> },
    { key: 'devices', label: he.tabDevices, icon: <MobileOutlined /> },
    { key: 'credits', label: he.tabCredits, icon: <WalletOutlined /> },
    { key: 'logs', label: he.tabLogs, icon: <UnorderedListOutlined /> },
    { key: 'enrollment', label: he.tabEnrollment, icon: <SafetyCertificateOutlined /> },
    { key: 'settings', label: he.tabSettings, icon: <SettingOutlined /> },
  ]

  return (
    <div className={isMobile ? 'page-shell wide admin-mobile-shell' : 'page-shell wide'}>
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <div>
          <Typography.Title level={2} className="page-title" style={{ marginBottom: 8 }}>
            {he.admin}
          </Typography.Title>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {he.adminLead}
          </Typography.Paragraph>
        </div>

        {!adminTokenPresent ? (
          <Alert
            type="warning"
            showIcon
            message={he.mdmAdminTokenMissing}
            description={he.mdmAdminTokenHint}
            action={
              <Button
                size="small"
                type="primary"
                onClick={() => {
                  setAdminToken('dev-admin-token')
                  setAdminTokenDraft('dev-admin-token')
                  message.success(he.mdmSaveToken)
                  void qc.invalidateQueries()
                }}
              >
                {he.mdmUseDefaultToken}
              </Button>
            }
          />
        ) : null}

        <Tabs
          className={isMobile ? 'admin-tabs admin-tabs-mobile' : 'admin-tabs'}
          activeKey={tab}
          onChange={(k) => void setTab(k as TabKey)}
          size={isMobile ? 'small' : 'middle'}
          tabBarGutter={isMobile ? 8 : undefined}
          moreIcon={null}
          renderTabBar={isMobile ? () => <div className="admin-tabs-mobile-spacer" /> : undefined}
          items={[
            {
              key: 'requests',
              label: (
                <Badge count={pendingCount} size="small" offset={[8, 0]}>
                  {he.tabRequests}
                </Badge>
              ),
              children: (
                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                  <Card size="small">
                    <Flex wrap="wrap" gap={12}>
                      <div className="filter-field">
                        <Typography.Text type="secondary">{he.status}</Typography.Text>
                        <Select
                          style={{ width: '100%', marginTop: 4 }}
                          value={reqFilters.status}
                          onChange={(v) => void setReqFilters({ status: v })}
                          options={[
                            { value: 'pending', label: he.pending },
                            { value: 'all', label: he.all },
                            { value: 'open', label: he.open },
                            { value: 'closed', label: he.closed },
                            { value: 'approved', label: he.approved },
                            { value: 'denied', label: he.denied },
                            { value: 'resolved', label: he.resolved },
                          ]}
                        />
                      </div>
                      <div className="filter-field">
                        <Typography.Text type="secondary">{he.type}</Typography.Text>
                        <Select
                          style={{ width: '100%', marginTop: 4 }}
                          value={reqFilters.type}
                          onChange={(v) => void setReqFilters({ type: v })}
                          options={[
                            { value: 'all', label: he.all },
                            { value: 'access', label: he.typeLabel.access },
                            { value: 'general', label: he.typeLabel.general },
                            { value: 'bug', label: he.typeLabel.bug },
                          ]}
                        />
                      </div>
                      <div className="filter-field">
                        <Typography.Text type="secondary">{he.device}</Typography.Text>
                        <Select
                          style={{ width: '100%', marginTop: 4 }}
                          value={reqFilters.device || undefined}
                          allowClear
                          placeholder={he.all}
                          onChange={(v) => void setReqFilters({ device: v || '' })}
                          options={deviceOptions(devices)}
                          {...searchableSelect}
                        />
                      </div>
                      <div className="filter-field grow">
                        <Typography.Text type="secondary">{he.search}</Typography.Text>
                        <Input
                          style={{ marginTop: 4 }}
                          value={reqFilters.q}
                          onChange={(e) => void setReqFilters({ q: e.target.value })}
                          placeholder={he.searchPlaceholder}
                          allowClear
                        />
                      </div>
                    </Flex>
                  </Card>

                  {requestsLoading && <LoadingBlock />}
                  {!requestsLoading && !requests.length && (
                    <Empty description={he.emptyRequests} />
                  )}
                  {!!requests.length && (
                    <Spin spinning={requestsFetching && !requestsLoading}>
                      <Row gutter={[16, 16]} className="inbox-grid">
                        {showQueue && (
                        <Col xs={24} md={9}>
                          <Card size="small" title={he.queue} styles={{ body: { padding: 0 } }}>
                            <div className="inbox-queue">
                              <List
                                dataSource={requests}
                                renderItem={(r) => {
                                  const next = adminNextAction(
                                    r.type,
                                    r.status,
                                    r.last_message?.author_role,
                                  )
                                  const snip = r.last_message?.body || r.reason || ''
                                  const active = selectedReq?.id === r.id
                                  return (
                                    <List.Item
                                      className="tap-row"
                                      onClick={() => void setSelectedReqId(r.id)}
                                      style={{
                                        paddingInline: 12,
                                        background: active ? '#eef7f2' : undefined,
                                      }}
                                    >
                                      <List.Item.Meta
                                        avatar={
                                          r.type === 'access' && r.kind === 'app' ? (
                                            <AppThumb
                                              name={r.app?.app_name || r.value}
                                              url={r.app?.artwork_url}
                                              size={36}
                                            />
                                          ) : undefined
                                        }
                                        title={
                                          <Flex justify="space-between" gap={8} align="center">
                                            <Typography.Text ellipsis style={{ flex: 1, minWidth: 0 }}>
                                              {r.app?.app_name || r.value}
                                            </Typography.Text>
                                            <Tag color={nextTagColor(next.kind, r.status)}>
                                              {next.label}
                                            </Tag>
                                          </Flex>
                                        }
                                        description={
                                          <>
                                            <div>
                                              {labelDevice(r.enrollment_id, devices)} ·{' '}
                                              {he.typeLabel[r.type] || r.type}
                                            </div>
                                            {snip && <div className="inbox-snip">{snip}</div>}
                                          </>
                                        }
                                      />
                                    </List.Item>
                                  )
                                }}
                              />
                            </div>
                          </Card>
                        </Col>
                        )}
                        {showTicket && (
                        <Col xs={24} md={15}>
                          <Card
                            size="small"
                            title={he.ticket}
                            extra={
                              isMobile && selectedReq ? (
                                <Button
                                  type="link"
                                  size="small"
                                  onClick={() => void setSelectedReqId(null)}
                                  style={{ paddingInline: 0 }}
                                >
                                  {he.back}
                                </Button>
                              ) : undefined
                            }
                          >
                            {!selectedReq && <Empty description={he.pickRequest} />}
                            {selectedReq && (() => {
                              const r = selectedReq
                              const scope = approveScope[r.id] || 'device'
                              const sub = deviceSub(r.enrollment_id, devices)
                              const next = adminNextAction(
                                r.type,
                                r.status,
                                r.last_message?.author_role,
                              )
                              return (
                                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                                  <Flex justify="space-between" gap={12} align="start" wrap="wrap">
                                    <Flex gap={10} align="center" style={{ minWidth: 0, flex: 1 }}>
                                      {r.type === 'access' && r.kind === 'app' && (
                                        <AppThumb
                                          name={r.app?.app_name || r.value}
                                          url={r.app?.artwork_url}
                                          size={40}
                                        />
                                      )}
                                      <div style={{ minWidth: 0 }}>
                                        <Typography.Text strong>
                                          {r.app?.app_name || r.value}
                                        </Typography.Text>
                                        <div>
                                          <Typography.Text type="secondary">
                                            {labelDevice(r.enrollment_id, devices)}
                                            {sub && ` · ${sub}`}
                                            {' · '}
                                            {he.typeLabel[r.type] || r.type}
                                          </Typography.Text>
                                        </div>
                                      </div>
                                    </Flex>
                                    <Tag color={nextTagColor(next.kind, r.status)}>{next.label}</Tag>
                                  </Flex>

                                  <RequestThread
                                    requestId={r.id}
                                    role="admin"
                                    closed={r.status !== 'pending'}
                                    onPosted={() => {
                                      void qc.invalidateQueries({ queryKey: ['requests'] })
                                      void refreshMeta()
                                    }}
                                  />

                                  {r.status === 'pending' && r.type === 'access' && (
                                    <Flex wrap="wrap" gap={8} className="decide-actions">
                                      <Select
                                        style={{ minWidth: isMobile ? undefined : 140 }}
                                        value={scope}
                                        onChange={(v) =>
                                          setApproveScope((s) => ({ ...s, [r.id]: v }))
                                        }
                                        options={[
                                          { value: 'device', label: he.thisDevice },
                                          { value: 'group', label: he.aGroup },
                                          { value: 'global', label: he.everyone },
                                        ]}
                                      />
                                      {scope === 'group' && (
                                        <Select
                                          style={{ minWidth: isMobile ? undefined : 140 }}
                                          placeholder="קבוצה…"
                                          {...searchableSelect}
                                          options={groupOptions(groups)}
                                          value={
                                            approveGroup[r.id] ||
                                            (groups.length === 1 ? groups[0].id : undefined)
                                          }
                                          onChange={(v) =>
                                            setApproveGroup((s) => ({ ...s, [r.id]: v }))
                                          }
                                        />
                                      )}
                                      <Button
                                        type="primary"
                                        loading={decideMutation.isPending}
                                        onClick={() =>
                                          decideMutation.mutate({
                                            id: r.id,
                                            approve: true,
                                            duration: '1h',
                                          })
                                        }
                                      >
                                        {he.approve1h}
                                      </Button>
                                      <Button
                                        title={he.approveForeverHint}
                                        loading={decideMutation.isPending}
                                        onClick={() =>
                                          decideMutation.mutate({
                                            id: r.id,
                                            approve: true,
                                            duration: 'permanent',
                                          })
                                        }
                                      >
                                        {he.approveForever}
                                      </Button>
                                      <Button
                                        danger
                                        loading={decideMutation.isPending}
                                        onClick={() =>
                                          decideMutation.mutate({ id: r.id, approve: false })
                                        }
                                      >
                                        {he.deny}
                                      </Button>
                                    </Flex>
                                  )}

                                  {r.status === 'pending' && r.type !== 'access' && (
                                    <Flex wrap="wrap" gap={8} className="decide-actions">
                                      <Button
                                        type="primary"
                                        loading={decideMutation.isPending}
                                        onClick={() =>
                                          decideMutation.mutate({ id: r.id, approve: true })
                                        }
                                      >
                                        {r.type === 'bug' ? he.resolveBug : he.resolveGeneral}
                                      </Button>
                                      <Button
                                        danger
                                        loading={decideMutation.isPending}
                                        onClick={() =>
                                          decideMutation.mutate({ id: r.id, approve: false })
                                        }
                                      >
                                        {he.denyGeneral}
                                      </Button>
                                    </Flex>
                                  )}
                                </Space>
                              )
                            })()}
                          </Card>
                        </Col>
                        )}
                      </Row>
                    </Spin>
                  )}
                </Space>
              ),
            },
            {
              key: 'groups',
              label: he.tabGroups,
              children: metaLoading ? (
                <LoadingBlock />
              ) : (
                <Row gutter={[16, 16]}>
                  {(!isMobile || !selectedGroup) && (
                  <Col xs={24} md={12}>
                    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                      <Card title={he.createGroup} size="small">
                        <Space direction="vertical" style={{ width: '100%' }}>
                          <Input
                            placeholder={he.groupName}
                            value={newName}
                            onChange={(e) => setNewName(e.target.value)}
                          />
                          <Input
                            placeholder={he.groupDesc}
                            value={newDesc}
                            onChange={(e) => setNewDesc(e.target.value)}
                          />
                          <Button
                            type="primary"
                            block
                            disabled={!newName.trim()}
                            onClick={async () => {
                              try {
                                const g = await api.createGroup(newName.trim(), newDesc.trim())
                                setNewName('')
                                setNewDesc('')
                                message.success(he.ok)
                                await refreshMeta()
                                await openGroup(g)
                              } catch (e) {
                                message.error((e as Error).message)
                              }
                            }}
                          >
                            {he.createGroup}
                          </Button>
                        </Space>
                      </Card>
                      <Input
                        allowClear
                        placeholder={he.searchGroups}
                        value={groupSearch}
                        onChange={(e) => setGroupSearch(e.target.value)}
                      />
                      {!groups.length && <Empty description={he.noGroups} />}
                      {!!groups.length && !visibleGroups.length && (
                        <Empty description={he.noMatches} />
                      )}
                      {visibleGroups.map((g) => (
                        <Card
                          key={g.id}
                          size="small"
                          style={
                            selectedGroup?.id === g.id
                              ? { outline: '2px solid #0b6e4f', outlineOffset: 2 }
                              : undefined
                          }
                        >
                          <Flex justify="space-between" gap={12} align="start" wrap="wrap">
                            <div style={{ minWidth: 0, flex: '1 1 180px' }}>
                              <Typography.Text strong>{g.name}</Typography.Text>{' '}
                              <Tag>
                                {g.member_count ?? 0} {he.memberCount}
                              </Tag>
                              {g.description && (
                                <div>
                                  <Typography.Text type="secondary">{g.description}</Typography.Text>
                                </div>
                              )}
                            </div>
                            <Space wrap style={{ width: isMobile ? '100%' : undefined }}>
                              <Button size="small" block={isMobile} onClick={() => void openGroup(g)}>
                                {he.openGroup}
                              </Button>
                              <Button
                                size="small"
                                block={isMobile}
                                onClick={() => {
                                  void setTab('whitelists')
                                  void setWhitelistMode('oneoffs')
                                  void setAllowFilters({
                                    ascope: 'group',
                                    agroup: g.id,
                                  })
                                }}
                              >
                                {he.viewAllow}
                              </Button>
                            </Space>
                          </Flex>
                        </Card>
                      ))}
                    </Space>
                  </Col>
                  )}
                  {(!isMobile || selectedGroup) && (
                  <Col xs={24} md={12}>
                    {!selectedGroup && (
                      <Empty description="בחרו קבוצה" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                    )}
                    {selectedGroup && (
                      <Card
                        size="small"
                        title={selectedGroup.name}
                        extra={
                          <Button type="link" onClick={() => void setSelectedGroupId(null)}>
                            {isMobile ? he.back : he.close}
                          </Button>
                        }
                      >
                        <Spin spinning={membersQuery.isFetching}>
                          <Space direction="vertical" style={{ width: '100%' }} size="middle">
                            {membersQuery.isLoading && !membersQuery.data ? (
                              <Skeleton active paragraph={{ rows: 4 }} />
                            ) : (
                              <>
                                <div>
                                  <Typography.Text type="secondary">{he.rename}</Typography.Text>
                                  <Flex gap={8} style={{ marginTop: 4 }}>
                                    <Input
                                      value={renameDraft}
                                      onChange={(e) => setRenameDraft(e.target.value)}
                                    />
                                    <Button
                                      type="primary"
                                      onClick={async () => {
                                        await api.updateGroup(
                                          selectedGroup.id,
                                          renameDraft.trim(),
                                          selectedGroup.description,
                                        )
                                        message.success(he.ok)
                                        void refreshMeta()
                                      }}
                                    >
                                      {he.save}
                                    </Button>
                                  </Flex>
                                </div>
                                <div>
                                  <Typography.Text type="secondary">{he.members}</Typography.Text>
                                  <Input
                                    style={{ marginTop: 4, marginBottom: 8 }}
                                    value={memberFilter}
                                    onChange={(e) => setMemberFilter(e.target.value)}
                                    placeholder={he.filterDevices}
                                    allowClear
                                  />
                                  <Checkbox.Group
                                    style={{ width: '100%' }}
                                    value={members}
                                    onChange={(vals) => setMembers(vals as string[])}
                                  >
                                    <Space direction="vertical">
                                      {filteredDevices.map((d) => (
                                        <Checkbox key={d.enrollment_id} value={d.enrollment_id}>
                                          {labelDevice(d, devices)}
                                        </Checkbox>
                                      ))}
                                    </Space>
                                  </Checkbox.Group>
                                  {!devices.length && (
                                    <Typography.Text type="secondary">
                                      {he.emptyDevices}
                                    </Typography.Text>
                                  )}
                                </div>
                                <Space>
                                  <Button
                                    type="primary"
                                    onClick={async () => {
                                      setMembers(await api.setMembers(selectedGroup.id, members))
                                      message.success(he.ok)
                                      void refreshMeta()
                                      void qc.invalidateQueries({
                                        queryKey: ['group-members', selectedGroup.id],
                                      })
                                    }}
                                  >
                                    {he.saveMembers}
                                  </Button>
                                  <Button
                                    danger
                                    onClick={() => {
                                      modal.confirm({
                                        title: he.delete + '?',
                                        onOk: async () => {
                                          await api.deleteGroup(selectedGroup.id)
                                          await setSelectedGroupId(null)
                                          void refreshMeta()
                                        },
                                      })
                                    }}
                                  >
                                    {he.delete}
                                  </Button>
                                </Space>
                              </>
                            )}
                          </Space>
                        </Spin>
                      </Card>
                    )}
                  </Col>
                  )}
                </Row>
              ),
            },
            {
              key: 'whitelists',
              label: he.tabAllow,
              children: (
                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                  <Segmented
                    block
                    value={whitelistMode}
                    onChange={(v) => setWhitelistMode(v as 'packs' | 'oneoffs')}
                    options={[
                      { value: 'packs', label: he.packsTab },
                      { value: 'oneoffs', label: he.oneOffsTab },
                    ]}
                  />
                  {whitelistMode === 'packs' ? (
                    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                      <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
                        {he.packsLead}
                      </Typography.Paragraph>
                      <Card size="small">
                        <Space.Compact style={{ width: '100%' }}>
                          <Input
                            placeholder={he.packName}
                            value={newPackName}
                            onChange={(e) => setNewPackName(e.target.value)}
                          />
                          <Button
                            type="primary"
                            disabled={!newPackName.trim()}
                            onClick={async () => {
                              try {
                                const p = await api.createPack(newPackName.trim())
                                setNewPackName('')
                                setSelectedPackId(p.id)
                                message.success(he.ok)
                                void qc.invalidateQueries({ queryKey: ['packs'] })
                              } catch (err) {
                                message.error((err as Error).message)
                              }
                            }}
                          >
                            {he.createPack}
                          </Button>
                        </Space.Compact>
                      </Card>
                      <Input
                        allowClear
                        placeholder={he.searchPacks}
                        value={packSearch}
                        onChange={(e) => setPackSearch(e.target.value)}
                      />
                      {packsQuery.isLoading ? <LoadingBlock /> : null}
                      {!packsQuery.isLoading && !(packsQuery.data?.length) ? (
                        <Empty description={he.emptyPacks} />
                      ) : null}
                      {!packsQuery.isLoading && !!(packsQuery.data?.length) && !visiblePacks.length ? (
                        <Empty description={he.noMatches} />
                      ) : null}
                      <List
                        dataSource={visiblePacks}
                        renderItem={(p) => (
                          <List.Item
                            actions={[
                              <Button
                                key="open"
                                type="link"
                                onClick={() => setSelectedPackId(p.id)}
                              >
                                {he.openPack}
                              </Button>,
                              <Button
                                key="del"
                                type="link"
                                danger
                                onClick={async () => {
                                  try {
                                    await api.deletePack(p.id)
                                    if (selectedPackId === p.id) setSelectedPackId('')
                                    message.success(he.ok)
                                    void qc.invalidateQueries({ queryKey: ['packs'] })
                                  } catch (err) {
                                    message.error((err as Error).message)
                                  }
                                }}
                              >
                                {he.delete}
                              </Button>,
                            ]}
                          >
                            <List.Item.Meta
                              title={p.name}
                              description={`${p.item_count ?? 0} · ${p.description || ''}`}
                            />
                          </List.Item>
                        )}
                      />
                      <Drawer
                        open={!!selectedPackId}
                        onClose={() => setSelectedPackId('')}
                        title={packDetailQuery.data?.pack.name || he.packsTab}
                        width={isMobile ? '100%' : 440}
                        placement={isMobile ? 'bottom' : 'right'}
                        height={isMobile ? '92%' : undefined}
                      >
                        {packDetailQuery.isLoading ? <LoadingBlock /> : null}
                        {packDetailQuery.data ? (
                          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                            <Card size="small" title={he.packItems}>
                              <Space direction="vertical" style={{ width: '100%' }}>
                                <Segmented
                                  block
                                  value={packItemKind}
                                  onChange={(v) => setPackItemKind(v as 'app' | 'url')}
                                  options={[
                                    { value: 'app', label: he.whitelistApps },
                                    { value: 'url', label: he.whitelistWeb },
                                  ]}
                                />
                                {packItemKind === 'app' ? (
                                  <AppSearchPicker
                                    pickLabel={he.addToAllow}
                                    onPick={async (app) => {
                                      try {
                                        await api.addPackItem(
                                          selectedPackId,
                                          'app',
                                          app.bundle_id,
                                        )
                                        message.success(he.ok)
                                        void qc.invalidateQueries({
                                          queryKey: ['pack', selectedPackId],
                                        })
                                        void qc.invalidateQueries({ queryKey: ['packs'] })
                                      } catch (err) {
                                        message.error((err as Error).message)
                                      }
                                    }}
                                  />
                                ) : (
                                  <Space.Compact style={{ width: '100%' }}>
                                    <Input
                                      value={packItemValue}
                                      onChange={(e) => setPackItemValue(e.target.value)}
                                      placeholder="example.com"
                                    />
                                    <Button
                                      type="primary"
                                      disabled={!packItemValue.trim()}
                                      onClick={async () => {
                                        try {
                                          await api.addPackItem(
                                            selectedPackId,
                                            'url',
                                            packItemValue.trim(),
                                          )
                                          setPackItemValue('')
                                          message.success(he.ok)
                                          void qc.invalidateQueries({
                                            queryKey: ['pack', selectedPackId],
                                          })
                                          void qc.invalidateQueries({ queryKey: ['packs'] })
                                        } catch (err) {
                                          message.error((err as Error).message)
                                        }
                                      }}
                                    >
                                      {he.addAllow}
                                    </Button>
                                  </Space.Compact>
                                )}
                                <List
                                  size="small"
                                  dataSource={packDetailQuery.data.items}
                                  locale={{ emptyText: he.emptyAllow }}
                                  renderItem={(it) => (
                                    <List.Item
                                      actions={[
                                        <Button
                                          key="rm"
                                          type="link"
                                          danger
                                          size="small"
                                          onClick={async () => {
                                            try {
                                              await api.removePackItem(
                                                selectedPackId,
                                                it.kind,
                                                it.value,
                                              )
                                              message.success(he.ok)
                                              void qc.invalidateQueries({
                                                queryKey: ['pack', selectedPackId],
                                              })
                                              void qc.invalidateQueries({ queryKey: ['packs'] })
                                            } catch (err) {
                                              message.error((err as Error).message)
                                            }
                                          }}
                                        >
                                          {he.removeOverride}
                                        </Button>,
                                      ]}
                                    >
                                      <Tag>{it.kind === 'app' ? 'app' : 'url'}</Tag> {it.value}
                                    </List.Item>
                                  )}
                                />
                              </Space>
                            </Card>
                            <Card size="small" title={he.packAssign}>
                              <Space direction="vertical" style={{ width: '100%' }}>
                                <Segmented
                                  block
                                  value={packAssignScope}
                                  onChange={(v) => {
                                    setPackAssignScope(v as 'global' | 'group' | 'device')
                                    setPackAssignTarget('')
                                  }}
                                  options={[
                                    { value: 'global', label: he.global },
                                    { value: 'group', label: he.group },
                                    { value: 'device', label: he.deviceEffective },
                                  ]}
                                />
                                {packAssignScope === 'group' ? (
                                  <Select
                                    style={{ width: '100%' }}
                                    placeholder={he.group}
                                    value={packAssignTarget || undefined}
                                    onChange={setPackAssignTarget}
                                    options={groupOptions(groups)}
                                    {...searchableSelect}
                                  />
                                ) : null}
                                {packAssignScope === 'device' ? (
                                  <Select
                                    style={{ width: '100%' }}
                                    placeholder={he.tabDevices}
                                    value={packAssignTarget || undefined}
                                    onChange={setPackAssignTarget}
                                    options={deviceOptions(devices)}
                                    {...searchableSelect}
                                  />
                                ) : null}
                                <Button
                                  type="primary"
                                  disabled={
                                    packAssignScope !== 'global' && !packAssignTarget.trim()
                                  }
                                  onClick={async () => {
                                    try {
                                      await api.addPackAssignment(selectedPackId, {
                                        scope: packAssignScope,
                                        group_id:
                                          packAssignScope === 'group'
                                            ? packAssignTarget
                                            : undefined,
                                        enrollment_id:
                                          packAssignScope === 'device'
                                            ? packAssignTarget
                                            : undefined,
                                      })
                                      message.success(he.ok)
                                      void qc.invalidateQueries({
                                        queryKey: ['pack', selectedPackId],
                                      })
                                    } catch (err) {
                                      message.error((err as Error).message)
                                    }
                                  }}
                                >
                                  {he.packAssign}
                                </Button>
                                <Typography.Text type="secondary">
                                  {he.packAssignedTo}
                                </Typography.Text>
                                <List
                                  size="small"
                                  dataSource={packDetailQuery.data.assignments}
                                  locale={{ emptyText: '—' }}
                                  renderItem={(as) => (
                                    <List.Item
                                      actions={[
                                        <Button
                                          key="rm"
                                          type="link"
                                          danger
                                          size="small"
                                          onClick={async () => {
                                            try {
                                              await api.removePackAssignment(
                                                selectedPackId,
                                                as.target_type,
                                                as.target_id,
                                              )
                                              message.success(he.ok)
                                              void qc.invalidateQueries({
                                                queryKey: ['pack', selectedPackId],
                                              })
                                            } catch (err) {
                                              message.error((err as Error).message)
                                            }
                                          }}
                                        >
                                          {he.removeOverride}
                                        </Button>,
                                      ]}
                                    >
                                      <Tag>{as.target_type}</Tag>{' '}
                                      {as.target_type === 'group'
                                        ? groups.find((g) => g.id === as.target_id)?.name ||
                                          as.target_id
                                        : as.target_type === 'device'
                                          ? labelDevice(as.target_id, devices)
                                          : he.global}
                                    </List.Item>
                                  )}
                                />
                              </Space>
                            </Card>
                          </Space>
                        ) : null}
                      </Drawer>
                    </Space>
                  ) : (
                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                  <Card size="small">
                    <Space direction="vertical" style={{ width: '100%' }} size="middle">
                      <Segmented
                        block
                        size={isMobile ? 'small' : 'middle'}
                        value={allowFilters.akind === 'url' ? 'url' : 'app'}
                        onChange={(v) =>
                          void setAllowFilters({ akind: v === 'url' ? 'url' : 'app' })
                        }
                        options={[
                          { value: 'app', label: he.whitelistApps },
                          { value: 'url', label: he.whitelistWeb },
                        ]}
                      />
                      <Segmented
                        block
                        size={isMobile ? 'small' : 'middle'}
                        value={
                          allowFilters.ascope === 'all' || allowFilters.ascope === 'group'
                            ? allowFilters.ascope
                            : allowFilters.ascope === 'device'
                              ? 'device'
                              : 'global'
                        }
                        onChange={(v) => {
                          const next = v as (typeof allowScopes)[number]
                          const patch: Partial<typeof allowFilters> = { ascope: next }
                          if (next === 'device' && !allowFilters.adevice && devices[0]) {
                            patch.adevice = devices[0].enrollment_id
                          }
                          if (next === 'group' && !allowFilters.agroup && groups[0]) {
                            patch.agroup = groups[0].id
                          }
                          void setAllowFilters(patch)
                        }}
                        options={[
                          { value: 'global', label: he.global },
                          { value: 'group', label: he.group },
                          { value: 'device', label: he.deviceEffective },
                        ]}
                      />
                      <Flex wrap="wrap" gap={12}>
                        {allowFilters.ascope === 'group' && (
                          <Select
                            style={{ minWidth: isMobile ? '100%' : 160, flex: 1 }}
                            value={allowFilters.agroup || undefined}
                            onChange={(v) => void setAllowFilters({ agroup: v })}
                            options={groupOptions(groups)}
                            {...searchableSelect}
                          />
                        )}
                        {allowFilters.ascope === 'device' && (
                          <Select
                            style={{ minWidth: isMobile ? '100%' : 160, flex: 1 }}
                            value={allowFilters.adevice || undefined}
                            onChange={(v) => void setAllowFilters({ adevice: v })}
                            options={deviceOptions(devices)}
                            {...searchableSelect}
                          />
                        )}
                        <Input
                          style={{ flex: 1, minWidth: isMobile ? '100%' : 180 }}
                          value={allowFilters.aq}
                          onChange={(e) => void setAllowFilters({ aq: e.target.value })}
                          placeholder={he.searchPlaceholder}
                          allowClear
                        />
                        <Button
                          type="primary"
                          block={isMobile}
                          onClick={() => {
                            setAddKind(allowFilters.akind === 'url' ? 'url' : 'app')
                            setAddScope(
                              allowFilters.ascope === 'group' || allowFilters.ascope === 'device'
                                ? allowFilters.ascope
                                : 'global',
                            )
                            setAddGroup(allowFilters.agroup)
                            setAddDevice(allowFilters.adevice)
                            setAddApp(null)
                            setAddValue('')
                            setAddOpen(true)
                          }}
                        >
                          {he.addAllow}
                        </Button>
                      </Flex>
                      {allowFilters.akind === 'url' ? (
                        <Space direction="vertical" style={{ width: '100%' }} size="small">
                          <Typography.Text type="secondary">{he.pasteUrlsHint}</Typography.Text>
                          <Input.TextArea
                            rows={3}
                            value={urlPaste}
                            onChange={(e) => setUrlPaste(e.target.value)}
                            placeholder="example.com"
                          />
                          <Button
                            type="primary"
                            disabled={!urlPaste.trim()}
                            onClick={async () => {
                              const lines = urlPaste
                                .split(/[\n,]+/)
                                .map((s) => s.trim())
                                .filter(Boolean)
                              const scope = allowFilters.ascope === 'all' ? 'global' : allowFilters.ascope
                              try {
                                for (const line of lines) {
                                  await api.createAllowance({
                                    kind: 'url',
                                    value: line,
                                    scope,
                                    group_id: scope === 'group' ? allowFilters.agroup : undefined,
                                    enrollment_id:
                                      scope === 'device' ? allowFilters.adevice : undefined,
                                    duration: 'permanent',
                                  })
                                }
                                setUrlPaste('')
                                message.success(he.ok)
                                void qc.invalidateQueries({ queryKey: ['allowances'] })
                              } catch (err) {
                                message.error((err as Error).message)
                              }
                            }}
                          >
                            {he.addAllow}
                          </Button>
                        </Space>
                      ) : null}
                    </Space>
                  </Card>

                  {allowancesLoading && <LoadingBlock />}
                  {!allowancesLoading && !allowances.length && (
                    <Empty description={he.emptyAllow} />
                  )}
                  <Spin spinning={allowancesFetching && !allowancesLoading}>
                    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                      {allowances.map((row, i) => (
                        <Card key={`${row.kind}-${row.value}-${row.source}-${i}`} size="small">
                          <Flex gap={12} align="start" justify="space-between" wrap="wrap">
                            <Flex gap={12} align="start" style={{ minWidth: 0, flex: 1 }}>
                              {row.kind === 'app' && (
                                <AppThumb
                                  name={row.app?.app_name || row.value}
                                  url={row.app?.artwork_url}
                                />
                              )}
                              <div style={{ minWidth: 0 }}>
                                <Space size={4} wrap>
                                  <Tag>{row.kind === 'app' ? 'אפליקציה' : 'אתר'}</Tag>
                                  <Tag>{sourceLabel(row.source)}</Tag>
                                </Space>
                                <div>
                                  <Typography.Text strong>
                                    {row.app?.app_name || row.value}
                                  </Typography.Text>
                                </div>
                                <Typography.Text type="secondary">
                                  {whyLine(row, groups, devices)}
                                </Typography.Text>
                                <div>
                                  <Typography.Text code copyable>
                                    {row.value}
                                  </Typography.Text>
                                </div>
                              </div>
                            </Flex>
                            {row.source === 'essential' ? (
                              <Typography.Text type="secondary">{he.essentialNote}</Typography.Text>
                            ) : (
                              <Button
                                danger
                                size="small"
                                onClick={() => {
                                  modal.confirm({
                                    title: he.revokeConfirm,
                                    content: row.app?.app_name || row.value,
                                    onOk: async () => {
                                      try {
                                        await api.deleteAllowance(row)
                                        message.success(he.ok)
                                        void qc.invalidateQueries({ queryKey: ['allowances'] })
                                      } catch (e) {
                                        message.error((e as Error).message)
                                      }
                                    },
                                  })
                                }}
                              >
                                {he.revoke}
                              </Button>
                            )}
                          </Flex>
                        </Card>
                      ))}
                    </Space>
                  </Spin>

                  <Drawer
                    open={addOpen}
                    title={he.addAllow}
                    onClose={() => setAddOpen(false)}
                    width={isMobile ? '100%' : 400}
                    placement={isMobile ? 'bottom' : 'right'}
                    height={isMobile ? '90%' : undefined}
                  >
                    <Space direction="vertical" style={{ width: '100%' }} size="middle">
                      <div>
                        <Typography.Text type="secondary">{he.kind}</Typography.Text>
                        <Segmented
                          block
                          size={isMobile ? 'small' : 'middle'}
                          style={{ marginTop: 4 }}
                          value={addKind}
                          onChange={(v) => {
                            setAddKind(String(v))
                            setAddApp(null)
                            setAddValue('')
                          }}
                          options={[
                            { value: 'url', label: 'אתר' },
                            { value: 'app', label: 'אפליקציה' },
                          ]}
                        />
                      </div>
                      <div>
                        <Typography.Text type="secondary">{he.scope}</Typography.Text>
                        <Segmented
                          block
                          size={isMobile ? 'small' : 'middle'}
                          style={{ marginTop: 4 }}
                          value={addScope}
                          onChange={(v) => setAddScope(String(v))}
                          options={[
                            { value: 'global', label: he.everyone },
                            { value: 'group', label: he.group },
                            { value: 'device', label: he.device },
                          ]}
                        />
                      </div>
                      {addScope === 'group' && (
                        <Select
                          style={{ width: '100%' }}
                          placeholder={he.group}
                          value={addGroup || undefined}
                          onChange={setAddGroup}
                          options={groupOptions(groups)}
                          {...searchableSelect}
                        />
                      )}
                      {addScope === 'device' && (
                        <Select
                          style={{ width: '100%' }}
                          placeholder={he.tabDevices}
                          value={addDevice || undefined}
                          onChange={setAddDevice}
                          options={deviceOptions(devices)}
                          {...searchableSelect}
                        />
                      )}
                      <div>
                        <Typography.Text type="secondary">{he.duration}</Typography.Text>
                        <Select
                          style={{ width: '100%', marginTop: 4 }}
                          value={addDuration}
                          onChange={setAddDuration}
                          options={[
                            { value: 'permanent', label: he.permanent },
                            { value: '1h', label: he.hour },
                            { value: '24h', label: he.day },
                            { value: '15m', label: he.minutes15 },
                            { value: 'today', label: he.today },
                          ]}
                        />
                      </div>
                      {addKind === 'url' ? (
                        <Input
                          value={addValue}
                          onChange={(e) => setAddValue(e.target.value)}
                          placeholder="khanacademy.org"
                        />
                      ) : (
                        <>
                          <AppSearchPicker
                            autoFocus
                            onPick={(app) => {
                              setAddApp(app)
                              setAddValue(app.bundle_id)
                            }}
                          />
                          {addApp && (
                            <Typography.Text type="secondary">
                              {he.selectedApp}: {addApp.app_name} ·{' '}
                              <Typography.Text code>{addApp.bundle_id}</Typography.Text>
                            </Typography.Text>
                          )}
                        </>
                      )}
                      <Button
                        type="primary"
                        block
                        disabled={addKind === 'app' ? !addApp && !addValue.trim() : !addValue.trim()}
                        onClick={async () => {
                          try {
                            await api.createAllowance({
                              kind: addKind,
                              value: addKind === 'app' ? addApp?.bundle_id || addValue : addValue,
                              scope: addScope,
                              duration: addDuration,
                              group_id: addScope === 'group' ? addGroup : '',
                              enrollment_id: addScope === 'device' ? addDevice : '',
                            })
                            message.success(he.ok)
                            setAddValue('')
                            setAddApp(null)
                            setAddOpen(false)
                            void qc.invalidateQueries({ queryKey: ['allowances'] })
                          } catch (e) {
                            message.error((e as Error).message)
                          }
                        }}
                      >
                        {he.addToAllow}
                      </Button>
                    </Space>
                  </Drawer>
                </Space>
                  )}
                </Space>
              ),
            },
            {
              key: 'devices',
              label: he.tabDevices,
              children: metaLoading ? (
                <LoadingBlock />
              ) : (
                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                  <Input
                    allowClear
                    placeholder={he.searchDevices}
                    value={deviceSearch}
                    onChange={(e) => setDeviceSearch(e.target.value)}
                  />
                  {!devices.length && <Empty description={he.mdmNoDevices} />}
                  {!!devices.length && !visibleDevices.length && (
                    <Empty description={he.noMatches} />
                  )}
                  {selectedIds.length > 0 ? (
                    <Card size="small">
                      <div className="bulk-bar">
                        <Typography.Text style={{ flex: '1 1 100%' }}>
                          {he.bulkSelected}: {selectedIds.length}
                        </Typography.Text>
                        <Button size="small" onClick={() => void runBulk('unrestricted')}>
                          {he.bulkAllowAll}
                        </Button>
                        <Button size="small" onClick={() => void runBulk('restrict')}>
                          {he.bulkRestrict}
                        </Button>
                        <Button size="small" danger onClick={() => void runBulk('lock')}>
                          {he.bulkLock}
                        </Button>
                        <Button
                          size="small"
                          onClick={() =>
                            Modal.confirm({
                              title: he.restartConfirm,
                              onOk: () => runBulk('restart'),
                            })
                          }
                        >
                          {he.bulkRestart}
                        </Button>
                        <Button
                          size="small"
                          onClick={() =>
                            Modal.confirm({
                              title: he.clearPasscodeConfirm,
                              onOk: () => runBulk('clear-passcode'),
                            })
                          }
                        >
                          {he.bulkClearPasscode}
                        </Button>
                        <Select
                          size="small"
                          placeholder={he.group}
                          value={bulkGroupId || undefined}
                          onChange={setBulkGroupId}
                          options={groupOptions(groups)}
                          {...searchableSelect}
                        />
                        <Button
                          size="small"
                          disabled={!bulkGroupId}
                          onClick={() => void runBulk('add-group', { group_id: bulkGroupId })}
                        >
                          {he.bulkAddGroup}
                        </Button>
                        <Button size="small" type="link" onClick={() => setSelectedIds([])}>
                          {he.close}
                        </Button>
                      </div>
                    </Card>
                  ) : null}
                  <List
                    dataSource={visibleDevices}
                    renderItem={(d) => (
                      <List.Item
                        className="device-list-item"
                        actions={[
                          <Switch
                            key="allowall"
                            aria-label={he.allowAll}
                            checkedChildren={isMobile ? '✓' : he.allowAll}
                            unCheckedChildren={isMobile ? '—' : he.allowAll}
                            checked={!!d.unrestricted}
                            onChange={(on) =>
                              unrestrictedMutation.mutate({ id: d.enrollment_id, on })
                            }
                          />,
                          <Button
                            key="quick"
                            type="link"
                            onClick={() => setDeviceDrawerId(d.enrollment_id)}
                          >
                            {he.quickActions}
                          </Button>,
                          <Button
                            key="open"
                            type="link"
                            onClick={() => navigate(`/admin/devices/${encodeURIComponent(d.enrollment_id)}`)}
                          >
                            {he.openDevice}
                          </Button>,
                        ]}
                      >
                        <Checkbox
                          style={{ marginInlineEnd: 12 }}
                          checked={selectedIds.includes(d.enrollment_id)}
                          onChange={(e) => {
                            setSelectedIds((prev) =>
                              e.target.checked
                                ? [...prev, d.enrollment_id]
                                : prev.filter((id) => id !== d.enrollment_id),
                            )
                          }}
                        />
                        <div
                          className="device-list-meta"
                          role="link"
                          tabIndex={0}
                          onClick={() => navigate(`/admin/devices/${encodeURIComponent(d.enrollment_id)}`)}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter' || e.key === ' ') {
                              e.preventDefault()
                              void navigate(`/admin/devices/${encodeURIComponent(d.enrollment_id)}`)
                            }
                          }}
                          style={{ cursor: 'pointer', minWidth: 0, flex: 1 }}
                        >
                          <List.Item.Meta
                            title={
                              <Space wrap>
                                <span>{d.name || d.serial_number || d.enrollment_id}</span>
                                {d.unrestricted ? <Tag color="orange">{he.allowAll}</Tag> : null}
                                {d.mdm ? <Tag color="green">{he.managed}</Tag> : null}
                              </Space>
                            }
                            description={
                              <Space direction="vertical" size={0} onClick={(e) => e.stopPropagation()}>
                                {d.serial_number ? (
                                  <Typography.Text type="secondary" title={d.last_seen_at || undefined}>
                                    {he.serial}: {d.serial_number}
                                    {d.last_seen_at ? ` · ${formatRelativeHe(d.last_seen_at)}` : ''}
                                  </Typography.Text>
                                ) : d.last_seen_at ? (
                                  <Typography.Text type="secondary" title={d.last_seen_at}>
                                    {he.lastSeen}: {formatRelativeHe(d.last_seen_at)}
                                  </Typography.Text>
                                ) : null}
                                {d.group_ids?.length ? (
                                  <Typography.Text type="secondary">
                                    {d.group_ids
                                      .map((gid) => groups.find((g) => g.id === gid)?.name || gid)
                                      .join(' · ')}
                                  </Typography.Text>
                                ) : null}
                              </Space>
                            }
                          />
                        </div>
                      </List.Item>
                    )}
                  />
                  <Drawer
                    open={!!deviceDrawerId}
                    onClose={() => setDeviceDrawerId('')}
                    title={
                      drawerDevice
                        ? drawerDevice.name ||
                          drawerDevice.serial_number ||
                          drawerDevice.enrollment_id
                        : he.quickActions
                    }
                    width={isMobile ? '100%' : 420}
                    placement={isMobile ? 'bottom' : 'right'}
                    height={isMobile ? '92%' : undefined}
                  >
                    {drawerDevice ? (
                      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                        <div>
                          <Typography.Text type="secondary">{he.nickname}</Typography.Text>
                          <Input
                            style={{ marginTop: 4 }}
                            defaultValue={drawerDevice.name}
                            key={drawerDevice.enrollment_id + '-name'}
                            placeholder={drawerDevice.enrollment_id}
                            onBlur={async (e) => {
                              const name = e.target.value.trim()
                              if (name === drawerDevice.name) return
                              try {
                                await api.setDeviceName(drawerDevice.enrollment_id, name)
                                message.success(he.ok)
                                void refreshMeta()
                              } catch (err) {
                                message.error((err as Error).message)
                              }
                            }}
                          />
                        </div>
                        <Flex justify="space-between" align="center">
                          <Typography.Text strong>{he.allowAll}</Typography.Text>
                          <Switch
                            checked={!!drawerDevice.unrestricted}
                            onChange={(on) =>
                              unrestrictedMutation.mutate({
                                id: drawerDevice.enrollment_id,
                                on,
                              })
                            }
                          />
                        </Flex>
                        <Card size="small" title={he.quickActions}>
                          <DeviceMdmActions
                            device={drawerDevice}
                            variant="quick"
                            mdmBusy={mdm.mdmBusy}
                            queueDeviceAction={mdm.queueDeviceAction}
                            queueAndPollResult={mdm.queueAndPollResult}
                            onOpenLostMode={() => setLostModeOpen(true)}
                            onOpenErase={() => setEraseOpen(true)}
                          />
                        </Card>
                        <Button
                          type="primary"
                          block
                          onClick={() => {
                            const id = drawerDevice.enrollment_id
                            setDeviceDrawerId('')
                            navigate(`/admin/devices/${encodeURIComponent(id)}`)
                          }}
                        >
                          {he.openDevicePage}
                        </Button>
                      </Space>
                    ) : null}
                  </Drawer>
                </Space>
              ),
            },
            {
              key: 'credits',
              label: he.tabCredits,
              children: metaLoading ? (
                <LoadingBlock />
              ) : (
                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                  <AdminCreditPurchases />

                  <Card size="small" title={he.creditSettings}>
                    <Space direction="vertical" style={{ width: '100%' }} size="small">
                      <Flex align="center" justify="space-between" gap={12} wrap="wrap">
                        <div>
                          <Typography.Text>{he.creditsSystemEnabled}</Typography.Text>
                          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
                            {he.creditsSystemDisabledHint}
                          </Typography.Paragraph>
                        </div>
                        <Switch
                          checked={creditSettingsQuery.data?.enabled !== false}
                          loading={savingSettings}
                          onChange={async (on) => {
                            setSavingSettings(true)
                            try {
                              const cost =
                                accessCostDraft ??
                                creditSettingsQuery.data?.access_request_cost ??
                                1
                              await api.adminUpdateCreditSettings(cost, on)
                              message.success(he.settingsSaved)
                              void qc.invalidateQueries({ queryKey: ['admin-credit-settings'] })
                            } catch (err) {
                              message.error((err as Error).message)
                            } finally {
                              setSavingSettings(false)
                            }
                          }}
                        />
                      </Flex>
                      <Typography.Text type="secondary">{he.accessRequestCostHint}</Typography.Text>
                      <Typography.Text type="secondary">{he.accessRequestCost}</Typography.Text>
                      <InputNumber
                        style={{ width: '100%', marginTop: 4 }}
                        min={1}
                        disabled={creditSettingsQuery.data?.enabled === false}
                        value={accessCostDraft ?? creditSettingsQuery.data?.access_request_cost ?? 1}
                        onChange={(v) => setAccessCostDraft(typeof v === 'number' ? v : 1)}
                      />
                      <Button
                        type="primary"
                        loading={savingSettings}
                        disabled={
                          creditSettingsQuery.data?.enabled === false ||
                          !accessCostDraft ||
                          accessCostDraft < 1
                        }
                        onClick={async () => {
                          if (!accessCostDraft || accessCostDraft < 1) return
                          setSavingSettings(true)
                          try {
                            await api.adminUpdateCreditSettings(accessCostDraft, true)
                            message.success(he.settingsSaved)
                            void qc.invalidateQueries({ queryKey: ['admin-credit-settings'] })
                          } catch (err) {
                            message.error((err as Error).message)
                          } finally {
                            setSavingSettings(false)
                          }
                        }}
                      >
                        {he.save}
                      </Button>
                    </Space>
                  </Card>

                  <Card
                    size="small"
                    title={he.allotmentRules}
                    extra={
                      <Space>
                        <Button
                          size="small"
                          loading={runningAllot}
                          onClick={async () => {
                            setRunningAllot(true)
                            try {
                              const res = await api.adminRunAllotments()
                              message.success(
                                `${he.allotmentRunOk} · ${res.grants_applied}/${res.grants_skipped + res.grants_applied}`,
                              )
                              void qc.invalidateQueries({ queryKey: ['admin-credit-allotments'] })
                              if (giftDevice) {
                                void qc.invalidateQueries({
                                  queryKey: ['admin-credit-device', giftDevice],
                                })
                              }
                            } catch (err) {
                              message.error((err as Error).message)
                            } finally {
                              setRunningAllot(false)
                            }
                          }}
                        >
                          {he.allotmentRunNow}
                        </Button>
                        <Button size="small" type="primary" onClick={() => openAllotmentModal()}>
                          {he.newAllotment}
                        </Button>
                      </Space>
                    }
                  >
                    <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
                      {he.allotmentRulesHint}
                    </Typography.Text>
                    {allotmentsQuery.isLoading ? (
                      <Skeleton active paragraph={{ rows: 3 }} />
                    ) : !(allotmentsQuery.data ?? []).length ? (
                      <Empty description={he.emptyAllow} />
                    ) : (
                      <Space direction="vertical" style={{ width: '100%' }} size="small">
                        {(allotmentsQuery.data ?? []).map((rule) => (
                          <Card key={rule.id} size="small" type="inner">
                            <Flex justify="space-between" align="flex-start" gap={12} wrap="wrap">
                              <div>
                                <Typography.Text strong>
                                  {rule.name || he.allotmentRules}
                                </Typography.Text>
                                <div>
                                  <Typography.Text type="secondary">
                                    {he.packageCredits.replace('{n}', String(rule.amount))} ·{' '}
                                    {allotmentIntervalLabel(rule.interval)} ·{' '}
                                    {allotmentTargetLabel(rule)}
                                  </Typography.Text>
                                </div>
                                <Space size={4} wrap style={{ marginTop: 6 }}>
                                  <Tag color={rule.enabled ? 'success' : 'default'}>
                                    {rule.enabled ? he.allotmentEnabled : he.allotmentDisabled}
                                  </Tag>
                                  {rule.period_key && (
                                    <Tag>
                                      {he.allotmentPeriodKey}: {rule.period_key}
                                    </Tag>
                                  )}
                                </Space>
                                <div style={{ marginTop: 6 }}>
                                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                                    {he.allotmentLastRun}:{' '}
                                    {rule.last_run_at
                                      ? new Date(rule.last_run_at).toLocaleString('he-IL')
                                      : '—'}
                                    {rule.next_period_at
                                      ? ` · ${he.allotmentNextPeriod}: ${new Date(rule.next_period_at).toLocaleString('he-IL')}`
                                      : ''}
                                  </Typography.Text>
                                </div>
                              </div>
                              <Space>
                                <Button size="small" onClick={() => openAllotmentModal(rule)}>
                                  {he.editAllotment}
                                </Button>
                                <Button
                                  size="small"
                                  onClick={async () => {
                                    try {
                                      await api.adminUpdateAllotment(rule.id, {
                                        enabled: !rule.enabled,
                                      })
                                      message.success(he.allotmentSaved)
                                      void qc.invalidateQueries({
                                        queryKey: ['admin-credit-allotments'],
                                      })
                                    } catch (err) {
                                      message.error((err as Error).message)
                                    }
                                  }}
                                >
                                  {rule.enabled ? he.allotmentDisabled : he.allotmentEnabled}
                                </Button>
                                <Button
                                  size="small"
                                  danger
                                  onClick={() => {
                                    modal.confirm({
                                      title: he.allotmentDeleteConfirm,
                                      okText: he.delete,
                                      cancelText: he.close,
                                      onOk: async () => {
                                        try {
                                          await api.adminDeleteAllotment(rule.id)
                                          message.success(he.allotmentDeleted)
                                          void qc.invalidateQueries({
                                            queryKey: ['admin-credit-allotments'],
                                          })
                                        } catch (err) {
                                          message.error((err as Error).message)
                                        }
                                      },
                                    })
                                  }}
                                >
                                  {he.delete}
                                </Button>
                              </Space>
                            </Flex>
                          </Card>
                        ))}
                      </Space>
                    )}
                  </Card>

                  <Card
                    size="small"
                    title={he.creditPackagesAdmin}
                    extra={
                      <Button size="small" type="primary" onClick={() => openPackageModal()}>
                        {he.newPackage}
                      </Button>
                    }
                  >
                    {creditPackagesQuery.isLoading ? (
                      <Skeleton active paragraph={{ rows: 3 }} />
                    ) : !(creditPackagesQuery.data ?? []).length ? (
                      <Empty description={he.emptyAllow} />
                    ) : (
                      <Space direction="vertical" style={{ width: '100%' }} size="small">
                        {(creditPackagesQuery.data ?? []).map((pkg) => (
                          <Card key={pkg.id} size="small" type="inner">
                            <Flex justify="space-between" align="flex-start" gap={12} wrap="wrap">
                              <div>
                                <Typography.Text strong>{pkg.name_he}</Typography.Text>
                                <div>
                                  <Typography.Text type="secondary">
                                    {he.packageCredits.replace('{n}', String(pkg.credits))} ·{' '}
                                    {he.priceILS.replace(
                                      '{n}',
                                      (pkg.price_agorot / 100).toFixed(
                                        pkg.price_agorot % 100 === 0 ? 0 : 2,
                                      ),
                                    )}{' '}
                                    · #{pkg.sort_order}
                                  </Typography.Text>
                                </div>
                                <Tag color={pkg.active ? 'success' : 'default'} style={{ marginTop: 6 }}>
                                  {pkg.active ? he.packageActive : he.packageInactive}
                                </Tag>
                              </div>
                              <Space>
                                <Button size="small" onClick={() => openPackageModal(pkg)}>
                                  {he.editPackage}
                                </Button>
                                {pkg.active && (
                                  <Button
                                    size="small"
                                    danger
                                    onClick={() => {
                                      modal.confirm({
                                        title: he.deactivatePackageConfirm,
                                        okText: he.deactivatePackage,
                                        cancelText: he.close,
                                        onOk: async () => {
                                          try {
                                            await api.adminDeactivateCreditPackage(pkg.id)
                                            message.success(he.packageSaved)
                                            void qc.invalidateQueries({
                                              queryKey: ['admin-credit-packages'],
                                            })
                                          } catch (err) {
                                            message.error((err as Error).message)
                                          }
                                        },
                                      })
                                    }}
                                  >
                                    {he.deactivatePackage}
                                  </Button>
                                )}
                              </Space>
                            </Flex>
                          </Card>
                        ))}
                      </Space>
                    )}
                  </Card>

                  <Card size="small" title={he.adjustCredits}>
                    <Space direction="vertical" style={{ width: '100%' }} size="small">
                      <Select
                        style={{ width: '100%' }}
                        placeholder={he.device}
                        value={giftDevice || undefined}
                        onChange={setGiftDevice}
                        options={deviceOptions(devices)}
                        {...searchableSelect}
                        allowClear
                      />
                      <Typography.Text type="secondary">{he.adjustAmountHint}</Typography.Text>
                      <Space.Compact style={{ width: '100%' }}>
                        <Input
                          style={{ width: 'auto' }}
                          value={he.adjustAmount}
                          disabled
                        />
                        <Input
                          type="number"
                          value={giftAmount}
                          onChange={(e) => setGiftAmount(e.target.value)}
                          style={{ width: '100%' }}
                        />
                      </Space.Compact>
                      <Input
                        value={giftNote}
                        onChange={(e) => setGiftNote(e.target.value)}
                        placeholder={he.giftNote}
                      />
                      <Button
                        type="primary"
                        loading={gifting}
                        disabled={!giftDevice || Number(giftAmount) === 0 || Number.isNaN(Number(giftAmount))}
                        onClick={async () => {
                          setGifting(true)
                          try {
                            const res = await api.adminAdjustCredits(
                              giftDevice,
                              Number(giftAmount),
                              giftNote,
                            )
                            message.success(
                              `${he.adjustOk} · ${he.availableBalance}: ${res.available ?? res.balance}`,
                            )
                            setGiftNote('')
                            void qc.invalidateQueries({
                              queryKey: ['admin-credit-device', giftDevice],
                            })
                          } catch (err) {
                            message.error((err as Error).message)
                          } finally {
                            setGifting(false)
                          }
                        }}
                      >
                        {he.adjustCredits}
                      </Button>
                      {giftDevice && creditDeviceQuery.isLoading && <Skeleton active paragraph={{ rows: 2 }} />}
                      {giftDevice && creditDeviceQuery.data && (
                        <Card size="small" type="inner" title={he.lookupDevice}>
                          <Space direction="vertical" size={4}>
                            <Typography.Text>
                              {he.availableBalance}:{' '}
                              <Typography.Text strong>
                                {creditDeviceQuery.data.available ??
                                  creditDeviceQuery.data.balance +
                                    (creditDeviceQuery.data.allotment_balance ?? 0)}
                              </Typography.Text>
                            </Typography.Text>
                            <Typography.Text type="secondary">
                              {he.permanentBalance}: {creditDeviceQuery.data.balance} ·{' '}
                              {he.allotmentBalance}:{' '}
                              {creditDeviceQuery.data.allotment_balance ?? 0}
                            </Typography.Text>
                          </Space>
                          <Typography.Paragraph strong style={{ marginTop: 12, marginBottom: 8 }}>
                            {he.recentLedger}
                          </Typography.Paragraph>
                          {!(creditDeviceQuery.data.ledger ?? []).length ? (
                            <Empty description={he.ledgerEmpty} />
                          ) : (
                            <List
                              size="small"
                              dataSource={creditDeviceQuery.data.ledger}
                              renderItem={(entry: CreditLedgerEntry) => (
                                <List.Item>
                                  <List.Item.Meta
                                    title={
                                      <Flex gap={8} wrap="wrap">
                                        <Tag>{ledgerReasonLabel(entry.reason)}</Tag>
                                        <Typography.Text
                                          type={entry.delta >= 0 ? 'success' : 'danger'}
                                        >
                                          {entry.delta >= 0 ? '+' : ''}
                                          {entry.delta}
                                        </Typography.Text>
                                        <Typography.Text type="secondary">
                                          → {entry.balance_after}
                                        </Typography.Text>
                                      </Flex>
                                    }
                                    description={
                                      <>
                                        {entry.note ? `${entry.note} · ` : ''}
                                        {new Date(entry.created_at).toLocaleString('he-IL')}
                                      </>
                                    }
                                  />
                                </List.Item>
                              )}
                            />
                          )}
                        </Card>
                      )}
                    </Space>
                  </Card>

                  <Modal
                    title={editingPkg ? he.editPackage : he.newPackage}
                    open={pkgModalOpen}
                    onCancel={() => setPkgModalOpen(false)}
                    confirmLoading={savingPkg}
                    okText={he.save}
                    cancelText={he.close}
                    width={isMobile ? '100%' : 520}
                    centered={!isMobile}
                    onOk={async () => {
                      const priceAgorot = Math.round(pkgPriceIls * 100)
                      if (!pkgName.trim() || pkgCredits < 1 || priceAgorot < 1) {
                        message.error(he.packageName)
                        return
                      }
                      setSavingPkg(true)
                      try {
                        if (editingPkg) {
                          await api.adminUpdateCreditPackage(editingPkg.id, {
                            name_he: pkgName.trim(),
                            credits: pkgCredits,
                            price_agorot: priceAgorot,
                            active: pkgActive,
                            sort_order: pkgSort,
                          })
                        } else {
                          await api.adminCreateCreditPackage({
                            name_he: pkgName.trim(),
                            credits: pkgCredits,
                            price_agorot: priceAgorot,
                            active: pkgActive,
                            sort_order: pkgSort,
                          })
                        }
                        message.success(he.packageSaved)
                        setPkgModalOpen(false)
                        void qc.invalidateQueries({ queryKey: ['admin-credit-packages'] })
                      } catch (err) {
                        message.error((err as Error).message)
                      } finally {
                        setSavingPkg(false)
                      }
                    }}
                  >
                    <Space direction="vertical" style={{ width: '100%' }} size="middle">
                      <div>
                        <Typography.Text type="secondary">{he.packageName}</Typography.Text>
                        <Input
                          style={{ marginTop: 4 }}
                          value={pkgName}
                          onChange={(e) => setPkgName(e.target.value)}
                        />
                      </div>
                      <div>
                        <Typography.Text type="secondary">{he.packageCreditsAmount}</Typography.Text>
                        <InputNumber
                          style={{ width: '100%', marginTop: 4 }}
                          min={1}
                          value={pkgCredits}
                          onChange={(v) => setPkgCredits(typeof v === 'number' ? v : 1)}
                        />
                      </div>
                      <div>
                        <Typography.Text type="secondary">{he.packagePriceILS}</Typography.Text>
                        <InputNumber
                          style={{ width: '100%', marginTop: 4 }}
                          min={0.01}
                          step={0.5}
                          value={pkgPriceIls}
                          onChange={(v) => setPkgPriceIls(typeof v === 'number' ? v : 1)}
                        />
                      </div>
                      <div>
                        <Typography.Text type="secondary">{he.packageSort}</Typography.Text>
                        <InputNumber
                          style={{ width: '100%', marginTop: 4 }}
                          value={pkgSort}
                          onChange={(v) => setPkgSort(typeof v === 'number' ? v : 0)}
                        />
                      </div>
                      <Flex align="center" gap={8}>
                        <Switch checked={pkgActive} onChange={setPkgActive} />
                        <Typography.Text>
                          {pkgActive ? he.packageActive : he.packageInactive}
                        </Typography.Text>
                      </Flex>
                    </Space>
                  </Modal>

                  <Modal
                    title={editingAllot ? he.editAllotment : he.newAllotment}
                    open={allotModalOpen}
                    onCancel={() => setAllotModalOpen(false)}
                    confirmLoading={savingAllot}
                    okText={he.save}
                    cancelText={he.close}
                    width={isMobile ? '100%' : 520}
                    centered={!isMobile}
                    onOk={async () => {
                      if (allotAmount < 1) {
                        message.error(he.allotmentAmount)
                        return
                      }
                      if (
                        (allotTargetType === 'group' || allotTargetType === 'individual') &&
                        !allotTargetID.trim()
                      ) {
                        message.error(he.allotmentTarget)
                        return
                      }
                      setSavingAllot(true)
                      try {
                        const payload = {
                          name: allotName.trim(),
                          note: allotNote.trim(),
                          amount: allotAmount,
                          interval: allotInterval,
                          target_type: allotTargetType,
                          target_id:
                            allotTargetType === 'everyone' ? '' : allotTargetID.trim(),
                          enabled: allotEnabled,
                        }
                        if (editingAllot) {
                          await api.adminUpdateAllotment(editingAllot.id, payload)
                        } else {
                          await api.adminCreateAllotment(payload)
                        }
                        message.success(he.allotmentSaved)
                        setAllotModalOpen(false)
                        void qc.invalidateQueries({ queryKey: ['admin-credit-allotments'] })
                      } catch (err) {
                        message.error((err as Error).message)
                      } finally {
                        setSavingAllot(false)
                      }
                    }}
                  >
                    <Space direction="vertical" style={{ width: '100%' }} size="middle">
                      <div>
                        <Typography.Text type="secondary">{he.allotmentName}</Typography.Text>
                        <Input
                          style={{ marginTop: 4 }}
                          value={allotName}
                          onChange={(e) => setAllotName(e.target.value)}
                        />
                      </div>
                      <div>
                        <Typography.Text type="secondary">{he.allotmentAmount}</Typography.Text>
                        <InputNumber
                          style={{ width: '100%', marginTop: 4 }}
                          min={1}
                          value={allotAmount}
                          onChange={(v) => setAllotAmount(typeof v === 'number' ? v : 1)}
                        />
                      </div>
                      <div>
                        <Typography.Text type="secondary">{he.allotmentInterval}</Typography.Text>
                        <Select
                          style={{ width: '100%', marginTop: 4 }}
                          value={allotInterval}
                          onChange={setAllotInterval}
                          options={[
                            { value: 'daily', label: he.allotmentIntervalDaily },
                            { value: 'weekly', label: he.allotmentIntervalWeekly },
                            { value: 'monthly', label: he.allotmentIntervalMonthly },
                          ]}
                        />
                      </div>
                      <div>
                        <Typography.Text type="secondary">{he.allotmentTarget}</Typography.Text>
                        <Select
                          style={{ width: '100%', marginTop: 4 }}
                          value={allotTargetType}
                          onChange={(v) => {
                            setAllotTargetType(v)
                            setAllotTargetID('')
                          }}
                          options={[
                            { value: 'everyone', label: he.allotmentTargetEveryone },
                            { value: 'group', label: he.allotmentTargetGroup },
                            { value: 'individual', label: he.allotmentTargetIndividual },
                          ]}
                        />
                      </div>
                      {allotTargetType === 'group' && (
                        <Select
                          style={{ width: '100%' }}
                          placeholder={he.group}
                          value={allotTargetID || undefined}
                          onChange={setAllotTargetID}
                          options={groupOptions(groups)}
                          {...searchableSelect}
                        />
                      )}
                      {allotTargetType === 'individual' && (
                        <Select
                          style={{ width: '100%' }}
                          placeholder={he.device}
                          value={allotTargetID || undefined}
                          onChange={setAllotTargetID}
                          options={deviceOptions(devices)}
                          {...searchableSelect}
                        />
                      )}
                      <div>
                        <Typography.Text type="secondary">{he.giftNote}</Typography.Text>
                        <Input
                          style={{ marginTop: 4 }}
                          value={allotNote}
                          onChange={(e) => setAllotNote(e.target.value)}
                          placeholder={he.giftNote}
                        />
                      </div>
                      <Flex align="center" gap={8}>
                        <Switch checked={allotEnabled} onChange={setAllotEnabled} />
                        <Typography.Text>
                          {allotEnabled ? he.allotmentEnabled : he.allotmentDisabled}
                        </Typography.Text>
                      </Flex>
                    </Space>
                  </Modal>
                </Space>
              ),
            },
            {
              key: 'logs',
              label: he.tabLogs,
              children: <AdminLogs />,
            },
            {
              key: 'enrollment',
              label: he.tabEnrollment,
              children: <AdminEnrollment />,
            },
            {
              key: 'settings',
              label: he.tabSettings,
              children: (
                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                  <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
                    {he.mdmAdminTokenHint}
                  </Typography.Paragraph>
                  <Card size="small" title={he.mdmAdminToken}>
                    <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
                      {he.mdmAdminTokenHint}
                    </Typography.Paragraph>
                    <Space.Compact style={{ width: '100%' }}>
                      <Input.Password
                        value={adminTokenDraft}
                        onChange={(e) => setAdminTokenDraft(e.target.value)}
                        placeholder="dev-admin-token"
                      />
                      <Button
                        type="primary"
                        onClick={() => {
                          setAdminToken(adminTokenDraft.trim())
                          message.success(he.mdmSaveToken)
                        }}
                      >
                        {he.mdmSaveToken}
                      </Button>
                    </Space.Compact>
                  </Card>
                  <Card size="small" title={he.lockScreenTitle}>
                    <Space direction="vertical" style={{ width: '100%' }} size="middle">
                      <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
                        {he.lockScreenLead}
                      </Typography.Paragraph>
                      <Switch
                        checked={lockScreenEnabledDraft}
                        onChange={setLockScreenEnabledDraft}
                        checkedChildren={he.lockScreenEnabled}
                        unCheckedChildren={he.lockScreenEnabled}
                      />
                      <Input
                        value={lockScreenFootnoteDraft}
                        onChange={(e) => setLockScreenFootnoteDraft(e.target.value)}
                        placeholder={he.lockScreenFootnoteHint}
                        maxLength={200}
                        disabled={!lockScreenEnabledDraft}
                      />
                      <Typography.Text type="secondary">{he.lockScreenFootnote}</Typography.Text>
                      <Button
                        type="primary"
                        size="small"
                        loading={mdm.mdmBusy === 'lock-screen-settings'}
                        onClick={async () => {
                          mdm.setMdmBusy('lock-screen-settings')
                          try {
                            await api.abmPutSettings({
                              lock_screen_enabled: lockScreenEnabledDraft,
                              lock_screen_footnote: lockScreenFootnoteDraft.trim(),
                            })
                            message.success(he.lockScreenSaved)
                            void qc.invalidateQueries({ queryKey: ['abm-settings'] })
                          } catch (err) {
                            message.error((err as Error).message)
                          } finally {
                            mdm.setMdmBusy('')
                          }
                        }}
                      >
                        {he.lockScreenSave}
                      </Button>
                    </Space>
                  </Card>
                </Space>
              ),
            },
          ]}
        />
        {(deviceDrawerId || mdm.mdmInfoOpen || lostModeOpen || eraseOpen) &&
        (deviceDrawerId || mdm.mdmInfoResult?.enrollment_id) ? (
          <DeviceActionModals
            enrollmentId={deviceDrawerId || mdm.mdmInfoResult!.enrollment_id}
            mdmBusy={mdm.mdmBusy}
            setMdmBusy={mdm.setMdmBusy}
            lostModeOpen={lostModeOpen}
            setLostModeOpen={setLostModeOpen}
            eraseOpen={eraseOpen}
            setEraseOpen={setEraseOpen}
            mdmInfoOpen={mdm.mdmInfoOpen}
            setMdmInfoOpen={mdm.setMdmInfoOpen}
            mdmInfoWaiting={mdm.mdmInfoWaiting}
            mdmInfoTitle={mdm.mdmInfoTitle}
            mdmInfoResult={mdm.mdmInfoResult}
          />
        ) : null}
      </Space>

      {isMobile ? (
        <nav className="admin-bottom-nav" aria-label={he.admin}>
          {mobileNavItems.map((item) => {
            const active = tab === item.key
            const icon =
              item.key === 'requests' ? (
                <Badge count={pendingCount} size="small" offset={[4, -2]}>
                  {item.icon}
                </Badge>
              ) : (
                item.icon
              )
            return (
              <button
                key={item.key}
                type="button"
                className={active ? 'active' : undefined}
                aria-current={active ? 'page' : undefined}
                onClick={() => void setTab(item.key)}
              >
                <span className="admin-bottom-nav-icon">{icon}</span>
                <span className="admin-bottom-nav-label">{item.label}</span>
              </button>
            )
          })}
        </nav>
      ) : null}
    </div>
  )
}
