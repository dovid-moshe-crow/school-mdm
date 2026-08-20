import { ArrowRightOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  App,
  Button,
  Card,
  Empty,
  Flex,
  Input,
  List,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  api,
  type Allowance,
  type AppMeta,
  type Device,
  type Group,
  type MdmCommandResult,
} from '../api'
import { AppSearchPicker } from '../components/AppSearchPicker'
import { EffectivePolicyView } from '../components/EffectivePolicyView'
import { DeviceActionModals } from '../components/DeviceActionModals'
import { DeviceMdmActions } from '../components/DeviceMdmActions'
import { SearchableCollection } from '../components/ListSearch'
import { deviceStatusFromInfo } from '../components/MdmCommandResultView'
import { useIsMobile } from '../hooks/useIsMobile'
import { useMdmDeviceActions } from '../hooks/useMdmDeviceActions'
import { he } from '../he'
import { formatRelativeHe } from '../time'
import { AppIdentity } from '../appMeta'
import { appTitle } from '../knownApps'

const EMPTY_DEVICES: Device[] = []
const EMPTY_GROUPS: Group[] = []
const EMPTY_ALLOWANCES: Allowance[] = []

function allowanceSearchText(row: Allowance) {
  return `${appTitle(row.app, row.value)} ${row.value} ${row.kind}`
}

export default function DeviceAdmin() {
  const { deviceId = '' } = useParams<{ deviceId: string }>()
  const navigate = useNavigate()
  const { message } = App.useApp()
  const qc = useQueryClient()
  const isMobile = useIsMobile()

  const [lostModeOpen, setLostModeOpen] = useState(false)
  const [eraseOpen, setEraseOpen] = useState(false)
  const [statusResult, setStatusResult] = useState<MdmCommandResult | null>(null)
  const [statusLoading, setStatusLoading] = useState(false)
  const [urlDraft, setUrlDraft] = useState('')
  const [groupBusy, setGroupBusy] = useState('')

  const mdm = useMdmDeviceActions()

  const devicesQuery = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.devices(),
  })
  const groupsQuery = useQuery({
    queryKey: ['groups'],
    queryFn: () => api.groups(),
  })
  const devices = devicesQuery.data ?? EMPTY_DEVICES
  const groups = groupsQuery.data ?? EMPTY_GROUPS
  const device = devices.find((d) => d.enrollment_id === deviceId)

  const mdmDetailQuery = useQuery({
    queryKey: ['mdm-device', deviceId],
    queryFn: () => api.mdmGetDevice(deviceId),
    enabled: !!deviceId && !!device?.mdm,
    retry: false,
  })

  const deviceAllowQuery = useQuery({
    queryKey: ['allowances', 'device', deviceId],
    queryFn: () => {
      const p = new URLSearchParams({ scope: 'device', enrollment_id: deviceId, kind: 'all' })
      return api.allowances(p)
    },
    enabled: !!deviceId,
  })
  const deviceProfilesQuery = useQuery({
    queryKey: ['profiles', 'device', deviceId],
    queryFn: () => api.profiles(deviceId),
    enabled: !!deviceId,
  })
  const deviceAllowances = deviceAllowQuery.data ?? EMPTY_ALLOWANCES
  const deviceProfiles = deviceProfilesQuery.data ?? []

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
    onSettled: (_data, _err, vars) => {
      void qc.invalidateQueries({ queryKey: ['devices'] })
      void qc.invalidateQueries({ queryKey: ['effective-allowlist', vars.id] })
    },
  })

  const statusChips = useMemo(() => deviceStatusFromInfo(statusResult), [statusResult])
  const displayName = device?.name || device?.serial_number || deviceId

  async function refreshStatus() {
    if (!deviceId || !device?.mdm) return
    setStatusLoading(true)
    try {
      const res = await mdm.fetchDeviceInformation(deviceId)
      if (res) setStatusResult(res)
    } finally {
      setStatusLoading(false)
    }
  }

  useEffect(() => {
    if (!deviceId || !device?.mdm) return
    void refreshStatus()
    // eslint-disable-next-line react-hooks/exhaustive-deps -- refresh once when device/mdm ready
  }, [deviceId, device?.mdm])

  async function addUrl() {
    const lines = urlDraft
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    if (!lines.length) return
    try {
      for (const line of lines) {
        await api.createAllowance({
          kind: 'url',
          value: line,
          scope: 'device',
          enrollment_id: deviceId,
          duration: 'permanent',
        })
      }
      setUrlDraft('')
      message.success(he.ok)
      void qc.invalidateQueries({ queryKey: ['allowances', 'device', deviceId] })
      void qc.invalidateQueries({ queryKey: ['effective-allowlist', deviceId] })
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  async function addApp(app: AppMeta) {
    try {
      await api.createAllowance({
        kind: 'app',
        value: app.bundle_id,
        scope: 'device',
        enrollment_id: deviceId,
        duration: 'permanent',
      })
      message.success(he.ok)
      void qc.invalidateQueries({ queryKey: ['allowances', 'device', deviceId] })
      void qc.invalidateQueries({ queryKey: ['effective-allowlist', deviceId] })
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  async function addToGroup(groupId: string) {
    setGroupBusy(groupId)
    try {
      await api.addDeviceGroup(deviceId, groupId)
      message.success(he.ok)
      void qc.invalidateQueries({ queryKey: ['devices'] })
      void qc.invalidateQueries({ queryKey: ['groups'] })
      void qc.invalidateQueries({ queryKey: ['effective-allowlist', deviceId] })
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setGroupBusy('')
    }
  }

  async function removeFromGroup(groupId: string) {
    setGroupBusy(groupId)
    try {
      await api.removeDeviceGroup(deviceId, groupId)
      message.success(he.ok)
      void qc.invalidateQueries({ queryKey: ['devices'] })
      void qc.invalidateQueries({ queryKey: ['groups'] })
      void qc.invalidateQueries({ queryKey: ['effective-allowlist', deviceId] })
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setGroupBusy('')
    }
  }

  async function revokeRow(row: Allowance) {
    try {
      await api.deleteAllowance(row)
      message.success(he.ok)
      void qc.invalidateQueries({ queryKey: ['allowances', 'device', deviceId] })
      void qc.invalidateQueries({ queryKey: ['effective-allowlist', deviceId] })
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  if (devicesQuery.isLoading) {
    return (
      <div className="page-shell wide device-admin-page">
        <Spin />
      </div>
    )
  }

  if (!device) {
    return (
      <div className="page-shell wide device-admin-page">
        <Empty description={he.deviceNotFound}>
          <Button type="primary" onClick={() => navigate('/admin?tab=devices')}>
            {he.backToDevices}
          </Button>
        </Empty>
      </div>
    )
  }

  const groupNames = (device.group_ids || [])
    .map((gid) => groups.find((g) => g.id === gid)?.name || gid)
    .filter(Boolean)

  const deviceApps = deviceAllowances.filter((a) => a.kind === 'app' && a.source === 'device')
  const deviceUrls = deviceAllowances.filter((a) => a.kind === 'url' && a.source === 'device')

  return (
    <div className={isMobile ? 'page-shell wide device-admin-page admin-mobile-shell' : 'page-shell wide device-admin-page'}>
      <div className="device-admin-topbar">
        <Button
          type="text"
          icon={<ArrowRightOutlined />}
          onClick={() => navigate('/admin?tab=devices')}
        >
          {he.backToDevices}
        </Button>
      </div>

      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <Card size="small" className="device-hero-card">
          <Flex justify="space-between" align="flex-start" gap={12} wrap="wrap">
            <div style={{ minWidth: 0, flex: 1 }}>
              <Typography.Text type="secondary">{he.nickname}</Typography.Text>
              <Input
                style={{ marginTop: 4 }}
                size="large"
                defaultValue={device.name}
                key={device.enrollment_id + '-name'}
                placeholder={he.nickname}
                onBlur={async (e) => {
                  const name = e.target.value.trim()
                  if (name === device.name) return
                  try {
                    await api.setDeviceName(device.enrollment_id, name)
                    message.success(he.ok)
                    void qc.invalidateQueries({ queryKey: ['devices'] })
                  } catch (err) {
                    message.error((err as Error).message)
                  }
                }}
              />
              <Typography.Paragraph type="secondary" style={{ margin: '6px 0 0' }}>
                {he.nicknameHint}
              </Typography.Paragraph>
            </div>
            <Space wrap>
              {device.unrestricted ? <Tag color="orange">{he.allowAll}</Tag> : null}
              {device.mdm ? (
                <Tag color="green">{he.managed}</Tag>
              ) : (
                <Tag>{he.notManaged}</Tag>
              )}
              {device.mdm ? (
                <Tag color={mdmDetailQuery.data?.has_push_token ? 'blue' : 'default'}>
                  {mdmDetailQuery.data?.has_push_token ? he.companionHasPush : he.companionNoPush}
                </Tag>
              ) : null}
            </Space>
          </Flex>
          <Space direction="vertical" size={2} style={{ marginTop: 8 }}>
            {device.serial_number ? (
              <Typography.Text type="secondary">
                {he.serial}: {device.serial_number}
              </Typography.Text>
            ) : null}
            {device.last_seen_at ? (
              <Typography.Text type="secondary" title={device.last_seen_at}>
                {he.lastSeen}: {formatRelativeHe(device.last_seen_at)}
              </Typography.Text>
            ) : null}
          </Space>
        </Card>

        <Card size="small" title={he.deviceGroups}>
          <Space direction="vertical" style={{ width: '100%' }} size="small">
            {groupNames.length ? (
              <Flex wrap="wrap" gap={8}>
                {(device.group_ids || []).map((gid) => {
                  const name = groups.find((g) => g.id === gid)?.name || gid
                  return (
                    <Tag
                      key={gid}
                      closable
                      color="blue"
                      onClose={(e) => {
                        e.preventDefault()
                        void removeFromGroup(gid)
                      }}
                    >
                      {name}
                    </Tag>
                  )
                })}
              </Flex>
            ) : (
              <Typography.Text type="secondary">{he.noDeviceGroups}</Typography.Text>
            )}
            <Select
              showSearch
              optionFilterProp="label"
              placeholder={he.addToGroup}
              value={null}
              loading={!!groupBusy}
              style={{ width: '100%', maxWidth: 360 }}
              options={groups
                .filter((g) => !(device.group_ids || []).includes(g.id))
                .map((g) => ({ value: g.id, label: g.name }))}
              onChange={(gid) => void addToGroup(gid)}
            />
          </Space>
        </Card>

        <Card
          size="small"
          className="device-status-card"
          title={he.deviceStatus}
          extra={
            <Button
              size="small"
              icon={<ReloadOutlined />}
              loading={statusLoading || mdm.mdmBusy === deviceId + ':info-status'}
              disabled={!device.mdm}
              onClick={() => void refreshStatus()}
            >
              {he.refreshStatus}
            </Button>
          }
        >
          {!device.mdm ? (
            <Typography.Text type="secondary">{he.statusNeedsMdm}</Typography.Text>
          ) : statusLoading && !statusResult ? (
            <Flex align="center" gap={10}>
              <Spin size="small" />
              <Typography.Text type="secondary">{he.mdmWaitingDevice}</Typography.Text>
            </Flex>
          ) : statusResult ? (
            <div className="device-status-chips">
              <div className="device-status-chip">
                <span className="device-status-chip-label">{he.battery}</span>
                <span className="device-status-chip-value">{statusChips.battery || '—'}</span>
              </div>
              <div className="device-status-chip">
                <span className="device-status-chip-label">{he.model}</span>
                <span className="device-status-chip-value">{statusChips.model || '—'}</span>
              </div>
              <div className="device-status-chip">
                <span className="device-status-chip-label">{he.osVersion}</span>
                <span className="device-status-chip-value">{statusChips.os || '—'}</span>
              </div>
              <div className="device-status-chip">
                <span className="device-status-chip-label">{he.storage}</span>
                <span className="device-status-chip-value">
                  {statusChips.available && statusChips.capacity
                    ? `${statusChips.available} / ${statusChips.capacity}`
                    : statusChips.capacity || '—'}
                </span>
              </div>
            </div>
          ) : (
            <Typography.Text type="secondary">{he.statusEmpty}</Typography.Text>
          )}
        </Card>

        <Card size="small" title={he.effectivePolicy}>
          <Flex justify="space-between" align="center" wrap="wrap" gap={8} style={{ marginBottom: 12 }}>
            <div style={{ minWidth: 0, flex: '1 1 220px' }}>
              <Typography.Text strong>{he.allowAll}</Typography.Text>
              <div>
                <Typography.Text type="secondary">{he.allowAllHint}</Typography.Text>
              </div>
            </div>
            <Switch
              checked={!!device.unrestricted}
              onChange={(on) => unrestrictedMutation.mutate({ id: device.enrollment_id, on })}
            />
          </Flex>
          {device.unrestricted ? (
            <Typography.Text type="secondary">{he.allowAll}</Typography.Text>
          ) : (
            <Space direction="vertical" size="small" style={{ width: '100%' }}>
              {deviceAllowQuery.isLoading ? <Spin /> : (
                <EffectivePolicyView rows={deviceAllowances} groups={groups} />
              )}
            </Space>
          )}
        </Card>

        <Card size="small" title={he.profileOnDevice}>
          <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
            {he.profileOnDeviceHint}
          </Typography.Paragraph>
          {deviceProfilesQuery.isLoading ? (
            <Spin />
          ) : !deviceProfiles.length ? (
            <Typography.Text type="secondary">{he.profileOnDeviceEmpty}</Typography.Text>
          ) : (
            <List
              size="small"
              dataSource={deviceProfiles}
              renderItem={(p) => (
                <List.Item>
                  <List.Item.Meta
                    title={p.name}
                    description={p.payload_identifier}
                  />
                </List.Item>
              )}
            />
          )}
        </Card>

        <Card size="small" title={he.deviceAllowEdit}>
          <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
            {he.deviceAllowEditHint}
          </Typography.Paragraph>
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <div>
              <Typography.Text strong>{he.whitelistApps}</Typography.Text>
              <div style={{ marginTop: 8 }}>
                <SearchableCollection
                  items={deviceApps}
                  text={allowanceSearchText}
                  emptyText={he.emptyAllow}
                >
                  {(rows) => (
                    <List
                      size="small"
                      dataSource={rows}
                      renderItem={(row) => (
                  <List.Item
                    actions={[
                      <Button key="rev" type="link" danger size="small" onClick={() => void revokeRow(row)}>
                        {he.revoke}
                      </Button>,
                    ]}
                  >
                    <List.Item.Meta
                      title={<AppIdentity bundleId={row.value} meta={row.app} size={36} />}
                    />
                  </List.Item>
                      )}
                    />
                  )}
                </SearchableCollection>
              </div>
              <div style={{ marginTop: 8 }}>
                <AppSearchPicker pickLabel={he.addToAllow} onPick={(app) => void addApp(app)} />
              </div>
            </div>
            <div>
              <Typography.Text strong>{he.whitelistWeb}</Typography.Text>
              <div style={{ marginTop: 8 }}>
                <SearchableCollection
                  items={deviceUrls}
                  text={allowanceSearchText}
                  emptyText={he.emptyAllow}
                >
                  {(rows) => (
                    <List
                      size="small"
                      dataSource={rows}
                      renderItem={(row) => (
                  <List.Item
                    actions={[
                      <Button key="rev" type="link" danger size="small" onClick={() => void revokeRow(row)}>
                        {he.revoke}
                      </Button>,
                    ]}
                  >
                    <Typography.Text>{row.value}</Typography.Text>
                  </List.Item>
                      )}
                    />
                  )}
                </SearchableCollection>
              </div>
              <Input.TextArea
                style={{ marginTop: 8 }}
                rows={3}
                value={urlDraft}
                onChange={(e) => setUrlDraft(e.target.value)}
                placeholder={he.pasteUrlsHint}
              />
              <Button type="primary" style={{ marginTop: 8 }} block={isMobile} onClick={() => void addUrl()}>
                {he.addToAllow}
              </Button>
            </div>
            <Link
              to={`/admin?tab=whitelists&wmode=oneoffs&ascope=device&adevice=${encodeURIComponent(deviceId)}`}
            >
              {he.openWhitelistsForDevice}
            </Link>
          </Space>
        </Card>

        <Card size="small" title={he.deviceActions}>
          <DeviceMdmActions
            device={device}
            variant="full"
            mdmBusy={mdm.mdmBusy}
            queueDeviceAction={mdm.queueDeviceAction}
            queueAndPollResult={mdm.queueAndPollResult}
            onOpenLostMode={() => setLostModeOpen(true)}
            onOpenErase={() => setEraseOpen(true)}
          />
          <div className="action-btn-grid" style={{ marginTop: 12 }}>
            <Button size="small" onClick={() => navigate('/admin?tab=credits')}>
              {he.goToCredits}
            </Button>
          </div>
        </Card>
      </Space>

      <DeviceActionModals
        enrollmentId={device.enrollment_id}
        mdmBusy={mdm.mdmBusy}
        setMdmBusy={mdm.setMdmBusy}
        lostModeOpen={lostModeOpen}
        setLostModeOpen={setLostModeOpen}
        eraseOpen={eraseOpen}
        setEraseOpen={setEraseOpen}
        mdmInfoOpen={mdm.mdmInfoOpen}
        setMdmInfoOpen={mdm.setMdmInfoOpen}
        mdmInfoWaiting={mdm.mdmInfoWaiting}
        mdmInfoTitle={mdm.mdmInfoTitle || displayName}
        mdmInfoResult={mdm.mdmInfoResult}
      />
    </div>
  )
}
