import {
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
  List,
  Row,
  Segmented,
  Select,
  Skeleton,
  Space,
  Spin,
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
import { useEffect, useMemo, useState } from 'react'
import { api, type Allowance, type AppMeta, type Device, type Group, type Request } from '../api'
import { RequestThread } from '../components/RequestThread'
import { useIsMobile } from '../hooks/useIsMobile'
import { he, statusClass, adminNextAction } from '../he'
import { AppThumb, useDebounced } from '../ui'

const tabKeys = ['requests', 'groups', 'allowances', 'devices'] as const
type TabKey = (typeof tabKeys)[number]

const allowScopes = ['global', 'group', 'device', 'all'] as const

function labelDevice(d: Device | string, devices: Device[]) {
  if (typeof d === 'string') {
    const found = devices.find((x) => x.enrollment_id === d)
    if (!found) return d
    return found.name || d
  }
  return d.name || d.enrollment_id
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
    akind: parseAsString.withDefault('all'),
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
  const [addAppQ, setAddAppQ] = useState('')
  const debouncedAddAppQ = useDebounced(addAppQ, 320)
  const [addApp, setAddApp] = useState<AppMeta | null>(null)

  const devicesQuery = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.devices(),
  })
  const groupsQuery = useQuery({
    queryKey: ['groups'],
    queryFn: () => api.groups(),
  })
  const devices = devicesQuery.data ?? []
  const groups = groupsQuery.data ?? []

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
  const requests = requestsQuery.data ?? []

  const allowancesEnabled =
    tab === 'allowances' &&
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
    tab === 'allowances' &&
    ((allowFilters.ascope === 'device' && !allowFilters.adevice) ||
      (allowFilters.ascope === 'group' && !allowFilters.agroup))
      ? []
      : (allowancesQuery.data ?? [])

  const membersQuery = useQuery({
    queryKey: ['group-members', selectedGroupId],
    queryFn: () => api.members(selectedGroupId!),
    enabled: tab === 'groups' && !!selectedGroupId,
  })

  const addAppsQuery = useQuery({
    queryKey: ['app-search', 'admin-add', debouncedAddAppQ],
    queryFn: () => api.searchApps(debouncedAddAppQ.trim()),
    enabled: addOpen && addKind === 'app' && !!debouncedAddAppQ.trim(),
  })
  const addResults = addAppsQuery.data ?? []

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
      const next = { ...prev }
      for (const r of requests) {
        if (!next[r.id]) next[r.id] = 'device'
      }
      return next
    })
  }, [requests])

  useEffect(() => {
    if (groups.length !== 1) return
    setApproveGroup((prev) => {
      const next = { ...prev }
      for (const r of requests) {
        if (!next[r.id]) next[r.id] = groups[0].id
      }
      return next
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

  const filteredDevices = useMemo(() => {
    const q = memberFilter.trim().toLowerCase()
    if (!q) return devices
    return devices.filter(
      (d) => d.enrollment_id.toLowerCase().includes(q) || d.name.toLowerCase().includes(q),
    )
  }, [devices, memberFilter])

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

  async function openGroup(g: Group) {
    await setSelectedGroupId(g.id)
  }

  const metaLoading =
    (tab === 'groups' || tab === 'devices') &&
    (devicesQuery.isLoading || groupsQuery.isLoading)
  const requestsLoading = tab === 'requests' && requestsQuery.isLoading && !requestsQuery.data
  const requestsFetching = tab === 'requests' && requestsQuery.isFetching
  const allowancesLoading =
    tab === 'allowances' && allowancesEnabled && allowancesQuery.isLoading && !allowancesQuery.data
  const allowancesFetching = tab === 'allowances' && allowancesEnabled && allowancesQuery.isFetching
  const showQueue = !isMobile || !selectedReqId
  const showTicket = !isMobile || !!selectedReqId

  return (
    <div className="page-shell wide">
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <div>
          <Typography.Title level={2} className="page-title" style={{ marginBottom: 8 }}>
            {he.admin}
          </Typography.Title>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {he.adminLead}
          </Typography.Paragraph>
        </div>

        <Tabs
          activeKey={tab}
          onChange={(k) => void setTab(k as TabKey)}
          size={isMobile ? 'small' : 'middle'}
          tabBarGutter={isMobile ? 8 : undefined}
          moreIcon={null}
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
                          options={devices.map((d) => ({
                            value: d.enrollment_id,
                            label: labelDevice(d, devices),
                          }))}
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
                                          value={
                                            approveGroup[r.id] ||
                                            (groups.length === 1 ? groups[0].id : undefined)
                                          }
                                          onChange={(v) =>
                                            setApproveGroup((s) => ({ ...s, [r.id]: v }))
                                          }
                                          options={groups.map((g) => ({
                                            value: g.id,
                                            label: g.name,
                                          }))}
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
                      {!groups.length && <Empty description={he.noGroups} />}
                      {groups.map((g) => (
                        <Card
                          key={g.id}
                          size="small"
                          style={
                            selectedGroup?.id === g.id
                              ? { outline: '2px solid #0b6e4f', outlineOffset: 2 }
                              : undefined
                          }
                        >
                          <Flex justify="space-between" gap={12} align="start">
                            <div>
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
                            <Space>
                              <Button size="small" onClick={() => void openGroup(g)}>
                                {he.openGroup}
                              </Button>
                              <Button
                                size="small"
                                onClick={() => {
                                  void setTab('allowances')
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
              key: 'allowances',
              label: he.tabAllow,
              children: (
                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                  <Card size="small">
                    <Space direction="vertical" style={{ width: '100%' }} size="middle">
                      <Segmented
                        block
                        size={isMobile ? 'small' : 'middle'}
                        value={allowFilters.ascope}
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
                          { value: 'all', label: he.allSources },
                        ]}
                      />
                      <Flex wrap="wrap" gap={12}>
                        <Select
                          className="filter-field"
                          style={{ minWidth: isMobile ? '100%' : 140, flex: 1 }}
                          value={allowFilters.akind}
                          onChange={(v) => void setAllowFilters({ akind: v })}
                          options={[
                            { value: 'all', label: he.appsAndUrls },
                            { value: 'app', label: he.appsOnly },
                            { value: 'url', label: he.urlsOnly },
                          ]}
                        />
                        {allowFilters.ascope === 'group' && (
                          <Select
                            style={{ minWidth: isMobile ? '100%' : 160, flex: 1 }}
                            value={allowFilters.agroup || undefined}
                            onChange={(v) => void setAllowFilters({ agroup: v })}
                            options={groups.map((g) => ({ value: g.id, label: g.name }))}
                          />
                        )}
                        {allowFilters.ascope === 'device' && (
                          <Select
                            style={{ minWidth: isMobile ? '100%' : 160, flex: 1 }}
                            value={allowFilters.adevice || undefined}
                            onChange={(v) => void setAllowFilters({ adevice: v })}
                            options={devices.map((d) => ({
                              value: d.enrollment_id,
                              label: labelDevice(d, devices),
                            }))}
                          />
                        )}
                        <Input
                          style={{ flex: 1, minWidth: isMobile ? '100%' : 180 }}
                          value={allowFilters.aq}
                          onChange={(e) => void setAllowFilters({ aq: e.target.value })}
                          placeholder={he.searchPlaceholder}
                          allowClear
                        />
                        <Button type="primary" block={isMobile} onClick={() => setAddOpen(true)}>
                          {he.addAllow}
                        </Button>
                      </Flex>
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
                          placeholder="…"
                          value={addGroup || undefined}
                          onChange={setAddGroup}
                          options={groups.map((g) => ({ value: g.id, label: g.name }))}
                        />
                      )}
                      {addScope === 'device' && (
                        <Select
                          style={{ width: '100%' }}
                          placeholder="…"
                          value={addDevice || undefined}
                          onChange={setAddDevice}
                          options={devices.map((d) => ({
                            value: d.enrollment_id,
                            label: labelDevice(d, devices),
                          }))}
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
                          <Input
                            value={addAppQ}
                            onChange={(e) => setAddAppQ(e.target.value)}
                            placeholder="YouTube"
                            allowClear
                          />
                          {addAppsQuery.isFetching && (
                            <Typography.Text type="secondary">{he.searching}</Typography.Text>
                          )}
                          <List
                            size="small"
                            dataSource={addResults}
                            renderItem={(app) => (
                              <List.Item
                                actions={[
                                  <Button
                                    key="pick"
                                    type="link"
                                    onClick={() => {
                                      setAddApp(app)
                                      setAddValue(app.bundle_id)
                                    }}
                                  >
                                    {he.pick}
                                  </Button>,
                                ]}
                              >
                                <List.Item.Meta
                                  avatar={<AppThumb name={app.app_name} url={app.artwork_url} />}
                                  title={app.app_name}
                                  description={app.bundle_id}
                                />
                              </List.Item>
                            )}
                          />
                          {addApp && (
                            <Typography.Text type="secondary">
                              {addApp.app_name} ·{' '}
                              <Typography.Text code>{addApp.bundle_id}</Typography.Text>
                            </Typography.Text>
                          )}
                        </>
                      )}
                      <Button
                        type="primary"
                        block
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
              ),
            },
            {
              key: 'devices',
              label: he.tabDevices,
              children: metaLoading ? (
                <LoadingBlock />
              ) : (
                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                  <Card size="small">
                    <Typography.Text strong>{he.nickname}</Typography.Text>
                    <div>
                      <Typography.Text type="secondary">{he.nicknameHint}</Typography.Text>
                    </div>
                  </Card>
                  {!devices.length && <Empty description={he.emptyDevices} />}
                  {devices.map((d) => (
                    <Card key={d.enrollment_id} size="small">
                      <Typography.Text type="secondary">{he.nickname}</Typography.Text>
                      <Input
                        style={{ marginTop: 4 }}
                        defaultValue={d.name}
                        placeholder={d.enrollment_id}
                        onBlur={async (e) => {
                          const name = e.target.value.trim()
                          if (name === d.name) return
                          try {
                            await api.setDeviceName(d.enrollment_id, name)
                            message.success(he.ok)
                            void refreshMeta()
                          } catch (err) {
                            message.error((err as Error).message)
                          }
                        }}
                      />
                      <Typography.Text code style={{ marginTop: 8, display: 'block' }}>
                        {d.enrollment_id}
                      </Typography.Text>
                    </Card>
                  ))}
                </Space>
              ),
            },
          ]}
        />
      </Space>
    </div>
  )
}
