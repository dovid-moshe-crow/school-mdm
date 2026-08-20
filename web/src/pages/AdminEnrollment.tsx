import {
  Alert,
  App,
  Button,
  Card,
  Descriptions,
  Empty,
  Input,
  Space,
  Skeleton,
  Switch,
  Table,
  Tag,
  Typography,
  Upload,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { api, type AbmDepDevice } from '../api'
import { ListSearchBar, SearchableEmpty } from '../components/ListSearch'
import { useIsMobile } from '../hooks/useIsMobile'
import { he } from '../he'
import { matchesQuery } from '../search'
import { formatRelativeHe } from '../time'

export default function AdminEnrollment() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const isMobile = useIsMobile()
  const [busy, setBusy] = useState('')
  const [abmProfileUUID, setAbmProfileUUID] = useState('')
  const [abmDepNameDraft, setAbmDepNameDraft] = useState('nanok')
  const [companionItunesDraft, setCompanionItunesDraft] = useState('')
  const [companionBundleDraft, setCompanionBundleDraft] = useState('com.kfilter.portal')
  const [companionEnabledDraft, setCompanionEnabledDraft] = useState(true)
  const [abmDeviceQ, setAbmDeviceQ] = useState('')
  const autoSyncedRef = useRef(false)

  const mdmStatusQuery = useQuery({
    queryKey: ['mdm-status'],
    queryFn: () => api.mdmStatus(),
    enabled: true,
  })
  const abmSettingsQuery = useQuery({
    queryKey: ['abm-settings'],
    queryFn: () => api.abmSettings(),
    enabled: true,
    retry: false,
  })
  const abmNamesQuery = useQuery({
    queryKey: ['abm-names'],
    queryFn: () => api.abmDepNames(),
    enabled: true,
    retry: false,
  })
  const abmAccountQuery = useQuery({
    queryKey: ['abm-account'],
    queryFn: () => api.abmAccount(),
    enabled: true,
    retry: false,
  })
  const abmDevicesQuery = useQuery({
    queryKey: ['abm-devices'],
    queryFn: () => api.abmDevices(),
    enabled: true,
    retry: false,
    staleTime: 60_000,
  })

  // When Apple is connected and the local cache is empty, sync once automatically.
  useEffect(() => {
    if (!abmAccountQuery.isSuccess) return
    if (autoSyncedRef.current) return
    if (abmDevicesQuery.isLoading) return
    const cached = abmDevicesQuery.data?.devices ?? []
    if (cached.length > 0) {
      autoSyncedRef.current = true
      return
    }
    autoSyncedRef.current = true
    let cancelled = false
    ;(async () => {
      setBusy('abm-sync')
      try {
        const data = await api.abmSync()
        if (cancelled) return
        qc.setQueryData(['abm-devices'], data)
      } catch {
        // Keep empty cache UI; user can press Sync manually.
      } finally {
        if (!cancelled) setBusy('')
      }
    })()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [abmAccountQuery.isSuccess, abmDevicesQuery.isLoading, abmDevicesQuery.data?.devices?.length])

  useEffect(() => {
    const name = abmSettingsQuery.data?.dep_name || abmNamesQuery.data?.dep_name
    if (name) setAbmDepNameDraft(name)
  }, [abmSettingsQuery.data?.dep_name, abmNamesQuery.data?.dep_name])

  useEffect(() => {
    const s = abmSettingsQuery.data
    if (!s) return
    if (s.dep_profile_uuid) setAbmProfileUUID(s.dep_profile_uuid)
    if (s.companion_itunes_id != null) setCompanionItunesDraft(String(s.companion_itunes_id || ''))
    if (s.companion_bundle_id) setCompanionBundleDraft(s.companion_bundle_id)
    if (typeof s.companion_enabled === 'boolean') setCompanionEnabledDraft(s.companion_enabled)
  }, [abmSettingsQuery.data])

  const activeDepName =
    abmSettingsQuery.data?.dep_name || abmNamesQuery.data?.dep_name || abmDepNameDraft || 'nanok'
  const abmConnected = abmAccountQuery.isSuccess
  const abmAccount = abmAccountQuery.data
  const abmDepDevices = abmDevicesQuery.data?.devices ?? []
  const visibleAbmDevices = useMemo(() => {
    return abmDepDevices.filter((d) =>
      matchesQuery(
        abmDeviceQ,
        d.serial_number,
        d.model,
        d.description,
        d.profile_status,
        d.os,
        d.device_family,
      ),
    )
  }, [abmDepDevices, abmDeviceQ])

  async function syncDevices() {
    setBusy('abm-sync')
    try {
      const data = await api.abmSync()
      qc.setQueryData(['abm-devices'], data)
      const n = data.devices?.length ?? 0
      const assigned = data.assigned ?? 0
      if (data.assign_error) {
        message.error(data.assign_error)
      } else if (!abmProfileUUID.trim()) {
        message.success(`${he.ok} — ${n} ${he.abmDevicesCount}`)
        if (n > 0) message.warning(he.abmAssignNeedProfile)
      } else if (assigned > 0) {
        message.success(
          he.abmSyncAssigned.replace('{n}', String(n)).replace('{m}', String(assigned)),
        )
      } else {
        message.success(he.abmSyncNoneAssigned.replace('{n}', String(n)))
      }
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setBusy('')
    }
  }

  const abmDeviceColumns: ColumnsType<AbmDepDevice> = useMemo(
    () => [
      {
        title: he.abmColSerial,
        dataIndex: 'serial_number',
        key: 'serial_number',
        render: (v: string) => (
          <Typography.Text code copyable>
            {v}
          </Typography.Text>
        ),
      },
      {
        title: he.abmColModel,
        dataIndex: 'model',
        key: 'model',
        render: (_: unknown, row) => row.description || row.model || '—',
      },
      { title: he.abmColFamily, dataIndex: 'device_family', key: 'device_family', width: 90 },
      { title: he.abmColOs, dataIndex: 'os', key: 'os', width: 90 },
      {
        title: he.abmColProfile,
        dataIndex: 'profile_status',
        key: 'profile_status',
        width: 120,
        render: (status?: string) => {
          if (status === 'assigned') return <Tag color="blue">{he.abmProfileAssigned}</Tag>
          if (status === 'pushed') return <Tag color="green">{he.abmProfilePushed}</Tag>
          if (status === 'removed') return <Tag>{he.abmProfileRemoved}</Tag>
          return <Tag>{he.abmProfileEmpty}</Tag>
        },
      },
      {
        title: he.abmColAssigned,
        dataIndex: 'device_assigned_date',
        key: 'device_assigned_date',
        width: 140,
        render: (v?: string) => (v ? formatRelativeHe(v) : '—'),
      },
    ],
    [],
  )

  const enrollUrl = mdmStatusQuery.data?.public_url
    ? `${mdmStatusQuery.data.public_url.replace(/\/$/, '')}/enroll`
    : ''

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
        {he.abmLead}
      </Typography.Paragraph>

      <Card size="small" title={he.mdmLead} loading={mdmStatusQuery.isLoading}>
        {mdmStatusQuery.data ? (
          <Space direction="vertical" size="small" style={{ width: '100%' }}>
            <Alert
              type={mdmStatusQuery.data.live && mdmStatusQuery.data.public_url ? 'success' : 'warning'}
              showIcon
              message={mdmStatusQuery.data.live ? he.mdmLive : he.mdmStub}
              description={
                enrollUrl
                  ? `${he.mdmEnrollHint}: ${enrollUrl}`
                  : 'MDM_PUBLIC_URL חסר — לא ניתן ליצור פרופיל הרשמה תקין.'
              }
            />
            {enrollUrl ? <Typography.Text copyable>{enrollUrl}</Typography.Text> : null}
            {mdmStatusQuery.data.topic ? (
              <Typography.Text type="secondary" copyable>
                {mdmStatusQuery.data.topic}
              </Typography.Text>
            ) : null}
            {mdmStatusQuery.data.push_cert ? (
              <Typography.Text type="secondary">APNs: {he.ok}</Typography.Text>
            ) : (
              <Typography.Text type="danger">תעודת APNs חסרה — פקודות לא יישלחו למכשירים.</Typography.Text>
            )}
          </Space>
        ) : (
          <Empty description={he.mdmStub} />
        )}
      </Card>

      <Card size="small" title={he.abmTitle}>
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Alert
            type={
              abmAccountQuery.isLoading
                ? 'info'
                : abmConnected
                  ? 'success'
                  : activeDepName
                    ? 'warning'
                    : 'info'
            }
            showIcon
            message={
              abmAccountQuery.isLoading
                ? he.loading
                : abmConnected
                  ? `${he.abmConnected}${abmAccount?.server_name ? `: ${abmAccount.server_name}` : ''}`
                  : activeDepName
                    ? he.abmConfiguredOnly
                    : he.abmNotConnected
            }
            description={
              <Space direction="vertical" size={4} style={{ width: '100%' }}>
                <Typography.Text type="secondary">{he.abmDepNameHint}</Typography.Text>
                <Space wrap style={{ width: isMobile ? '100%' : undefined }}>
                  <Input
                    style={{ width: isMobile ? '100%' : 220 }}
                    value={abmDepNameDraft}
                    onChange={(e) => setAbmDepNameDraft(e.target.value)}
                    placeholder="nanok"
                  />
                  <Button
                    type="primary"
                    size="small"
                    block={isMobile}
                    disabled={
                      !abmDepNameDraft.trim() ||
                      abmDepNameDraft.trim() === (abmSettingsQuery.data?.dep_name || '')
                    }
                    loading={busy === 'abm-settings'}
                    onClick={async () => {
                      setBusy('abm-settings')
                      try {
                        await api.abmPutSettings({ dep_name: abmDepNameDraft.trim() })
                        message.success(he.abmDepNameSaved)
                        void qc.invalidateQueries({ queryKey: ['abm-settings'] })
                        void qc.invalidateQueries({ queryKey: ['abm-names'] })
                        void qc.invalidateQueries({ queryKey: ['abm-account'] })
                      } catch (err) {
                        message.error((err as Error).message)
                      } finally {
                        setBusy('')
                      }
                    }}
                  >
                    {he.abmDepNameSave}
                  </Button>
                </Space>
                {abmAccountQuery.isError ? (
                  <Typography.Text type="danger">
                    {(abmAccountQuery.error as Error)?.message || he.abmNotConnected}
                  </Typography.Text>
                ) : null}
                <Button
                  size="small"
                  loading={abmAccountQuery.isFetching}
                  onClick={() => {
                    void qc.invalidateQueries({ queryKey: ['abm-account'] })
                    void qc.invalidateQueries({ queryKey: ['abm-names'] })
                    void qc.invalidateQueries({ queryKey: ['abm-settings'] })
                  }}
                >
                  {he.abmRefreshAccount}
                </Button>
              </Space>
            }
          />

          {abmConnected && abmAccount ? (
            <Card size="small" type="inner" title={he.abmAccountTitle}>
              <Descriptions size="small" column={1} bordered>
                {abmAccount.org_name ? (
                  <Descriptions.Item label={he.abmAccountOrg}>{abmAccount.org_name}</Descriptions.Item>
                ) : null}
                {abmAccount.server_name ? (
                  <Descriptions.Item label={he.abmAccountServer}>
                    {abmAccount.server_name}
                  </Descriptions.Item>
                ) : null}
                {abmAccount.admin_id || abmAccount.facilitator_id ? (
                  <Descriptions.Item label={he.abmAccountAdmin}>
                    {abmAccount.admin_id || abmAccount.facilitator_id}
                  </Descriptions.Item>
                ) : null}
                {abmAccount.org_email ? (
                  <Descriptions.Item label={he.abmAccountEmail}>{abmAccount.org_email}</Descriptions.Item>
                ) : null}
                {abmAccount.org_type ? (
                  <Descriptions.Item label={he.abmAccountType}>
                    {abmAccount.org_type === 'edu'
                      ? he.abmAccountTypeEdu
                      : abmAccount.org_type === 'org'
                        ? he.abmAccountTypeOrg
                        : abmAccount.org_type}
                  </Descriptions.Item>
                ) : null}
              </Descriptions>
            </Card>
          ) : null}

          <div>
            <Typography.Text strong>{he.abmLinksTitle}</Typography.Text>
            <Space direction="vertical" size={2} style={{ display: 'flex', marginTop: 6 }}>
              <Typography.Link href="https://school.apple.com" target="_blank" rel="noreferrer">
                {he.abmLinkSchool}
              </Typography.Link>
              <Typography.Link href="https://business.apple.com" target="_blank" rel="noreferrer">
                {he.abmLinkBusiness}
              </Typography.Link>
              <Typography.Link
                href="https://support.apple.com/guide/deployment/automated-device-enrollment-dep00a2e1/web"
                target="_blank"
                rel="noreferrer"
              >
                {he.abmLinkAde}
              </Typography.Link>
            </Space>
          </div>

          <Card size="small" type="inner" title={he.abmStep1Title}>
            <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
              {he.abmStep1Body}
            </Typography.Paragraph>
            <Button
              type="primary"
              block={isMobile}
              loading={busy === 'abm-cert'}
              onClick={async () => {
                setBusy('abm-cert')
                try {
                  await api.abmDownloadPublicKey(activeDepName)
                  message.success(he.ok)
                } catch (err) {
                  message.error((err as Error).message)
                } finally {
                  setBusy('')
                }
              }}
            >
              {he.abmDownloadCert}
            </Button>
          </Card>

          <Card size="small" type="inner" title={he.abmStep2Title}>
            <Typography.Paragraph type="secondary" style={{ marginTop: 0, marginBottom: 0 }}>
              {he.abmStep2Body}
            </Typography.Paragraph>
          </Card>

          <Card size="small" type="inner" title={he.abmStep3Title}>
            <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
              {he.abmStep3Body}
            </Typography.Paragraph>
            <Typography.Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
              {he.abmUploadTokenHint}
            </Typography.Text>
            <Input
              type="file"
              accept=".p7m,.txt,.pem,application/octet-stream"
              disabled={busy === 'abm-token'}
              onChange={async (e) => {
                const file = e.target.files?.[0]
                e.target.value = ''
                if (!file) return
                setBusy('abm-token')
                try {
                  await api.abmUploadToken(activeDepName, file)
                  message.success(he.abmUploadTokenOk)
                  void qc.invalidateQueries({ queryKey: ['abm-names'] })
                  void qc.invalidateQueries({ queryKey: ['abm-settings'] })
                  void qc.invalidateQueries({ queryKey: ['abm-account'] })
                } catch (err) {
                  message.error((err as Error).message)
                } finally {
                  setBusy('')
                }
              }}
            />
          </Card>

          <Card size="small" type="inner" title={he.abmStep4Title}>
            <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
              {he.abmStep4Body}
            </Typography.Paragraph>
            <Space direction="vertical" style={{ width: '100%' }} size="small">
              <div>
                <Typography.Text type="secondary">{he.abmDefineProfileHint}</Typography.Text>
                {abmProfileUUID ? (
                  <div style={{ marginTop: 4 }}>
                    <Tag color="green">{he.abmProfileReady}</Tag>{' '}
                    <Typography.Text code copyable>
                      {abmProfileUUID}
                    </Typography.Text>
                  </div>
                ) : (
                  <div style={{ marginTop: 4 }}>
                    <Typography.Text type="danger">{he.abmAssignNeedProfile}</Typography.Text>
                  </div>
                )}
                <div style={{ marginTop: 6 }}>
                  <Button
                    size="small"
                    type="primary"
                    block={isMobile}
                    loading={busy === 'abm-profile'}
                    onClick={async () => {
                      setBusy('abm-profile')
                      try {
                        const resp = await api.abmDefineProfile({ profile_name: 'School MDM' })
                        if (resp.profile_uuid) {
                          setAbmProfileUUID(resp.profile_uuid)
                          void qc.invalidateQueries({ queryKey: ['abm-settings'] })
                        }
                        message.success(he.ok)
                      } catch (err) {
                        message.error((err as Error).message)
                      } finally {
                        setBusy('')
                      }
                    }}
                  >
                    {he.abmDefineProfile}
                  </Button>
                </div>
              </div>
              <Typography.Text type="secondary">{he.abmSyncHint}</Typography.Text>
              <Typography.Text type="secondary">{he.abmAfterAssign}</Typography.Text>
            </Space>
          </Card>

          {abmConnected ? (
            <Card
              size="small"
              type="inner"
              title={`${he.abmDevicesTitle}${abmDepDevices.length ? ` (${abmDepDevices.length})` : ''}`}
              extra={
                <Space wrap>
                  {abmDevicesQuery.data?.synced_at ? (
                    <Typography.Text type="secondary">
                      {he.abmDevicesLastSync}: {formatRelativeHe(abmDevicesQuery.data.synced_at)}
                    </Typography.Text>
                  ) : null}
                  <Button size="small" type="primary" loading={busy === 'abm-sync'} onClick={() => void syncDevices()}>
                    {he.abmSync}
                  </Button>
                </Space>
              }
            >
              <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
                {he.abmSyncHint}
              </Typography.Paragraph>
              {abmDevicesQuery.isLoading && !abmDepDevices.length ? (
                <Skeleton active paragraph={{ rows: 3 }} />
              ) : abmDevicesQuery.isFetching && !abmDepDevices.length && busy === 'abm-sync' ? (
                <Skeleton active paragraph={{ rows: 3 }} />
              ) : !abmDepDevices.length ? (
                <Empty description={he.abmDevicesEmpty} />
              ) : (
                <>
                <ListSearchBar
                  placeholder={he.searchDevices}
                  value={abmDeviceQ}
                  onChange={setAbmDeviceQ}
                  total={abmDepDevices.length}
                  shown={visibleAbmDevices.length}
                  style={{ marginBottom: 12 }}
                />
                <SearchableEmpty
                  total={abmDepDevices.length}
                  shown={visibleAbmDevices.length}
                  emptyText={he.abmDevicesEmpty}
                />
                {visibleAbmDevices.length ? (
                <Table
                  size="small"
                  rowKey="serial_number"
                  columns={abmDeviceColumns}
                  dataSource={visibleAbmDevices}
                  pagination={{ pageSize: 10, hideOnSinglePage: true }}
                  scroll={{ x: 820 }}
                />
                ) : null}
                </>
              )}
            </Card>
          ) : null}
        </Space>
      </Card>

      <Card size="small" title={he.companionTitle}>
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {he.companionLead}
          </Typography.Paragraph>
          <Space wrap style={{ width: isMobile ? '100%' : undefined }}>
            <Input
              style={{ width: isMobile ? '100%' : 220 }}
              value={companionItunesDraft}
              onChange={(e) => setCompanionItunesDraft(e.target.value)}
              placeholder={he.companionItunesId}
            />
            <Input
              style={{ width: isMobile ? '100%' : 240 }}
              value={companionBundleDraft}
              onChange={(e) => setCompanionBundleDraft(e.target.value)}
              placeholder={he.companionBundleId}
            />
            <Switch
              checked={companionEnabledDraft}
              onChange={setCompanionEnabledDraft}
              aria-label={he.companionEnabled}
              checkedChildren={isMobile ? '✓' : he.companionEnabled}
              unCheckedChildren={isMobile ? '—' : he.companionEnabled}
            />
            <Button
              type="primary"
              size="small"
              block={isMobile}
              loading={busy === 'companion-settings'}
              onClick={async () => {
                setBusy('companion-settings')
                try {
                  const itunes = Number(companionItunesDraft.trim() || '0')
                  await api.abmPutSettings({
                    companion_itunes_id: Number.isFinite(itunes) ? itunes : 0,
                    companion_bundle_id: companionBundleDraft.trim() || 'com.kfilter.portal',
                    companion_enabled: companionEnabledDraft,
                  })
                  message.success(he.companionSaved)
                  void qc.invalidateQueries({ queryKey: ['abm-settings'] })
                } catch (err) {
                  message.error((err as Error).message)
                } finally {
                  setBusy('')
                }
              }}
            >
              {he.companionSave}
            </Button>
          </Space>
          <Typography.Text type="secondary">{he.vppTokenTitle}</Typography.Text>
          <Typography.Text type="secondary">{he.vppTokenHint}</Typography.Text>
          <Space wrap>
            <Tag color={abmSettingsQuery.data?.has_vpp_token ? 'green' : 'default'}>
              {abmSettingsQuery.data?.has_vpp_token ? he.vppTokenPresent : he.vppTokenMissing}
              {abmSettingsQuery.data?.vpp_token_filename
                ? ` · ${abmSettingsQuery.data.vpp_token_filename}`
                : ''}
            </Tag>
            <Upload
              accept=".vpptoken,.txt,.json,.p7m,*"
              showUploadList={false}
              beforeUpload={async (file) => {
                try {
                  const text = await file.text()
                  await api.uploadVppToken(text, file.name)
                  message.success(he.vppTokenUploaded)
                  void qc.invalidateQueries({ queryKey: ['abm-settings'] })
                } catch (err) {
                  message.error((err as Error).message)
                }
                return false
              }}
            >
              <Button size="small">{he.vppTokenUpload}</Button>
            </Upload>
            <Button
              size="small"
              danger
              disabled={!abmSettingsQuery.data?.has_vpp_token}
              onClick={async () => {
                try {
                  await api.deleteVppToken()
                  message.success(he.ok)
                  void qc.invalidateQueries({ queryKey: ['abm-settings'] })
                } catch (err) {
                  message.error((err as Error).message)
                }
              }}
            >
              {he.vppTokenClear}
            </Button>
          </Space>
        </Space>
      </Card>
    </Space>
  )
}
