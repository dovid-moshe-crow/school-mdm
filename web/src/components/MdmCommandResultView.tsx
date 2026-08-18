import {
  Alert,
  Button,
  Card,
  Descriptions,
  Flex,
  List,
  Tag,
  Typography,
} from 'antd'
import { useState, type ReactNode } from 'react'
import type { MdmCommandResult } from '../api'
import { useIsMobile } from '../hooks/useIsMobile'
import { he } from '../he'
import { matchesQuery } from '../search'
import { ListSearchBar } from './ListSearch'

export function formatMdmQueryValue(key: string, value: unknown): string {
  if (value == null) return '—'
  if (typeof value === 'boolean') return value ? 'true' : 'false'
  if (typeof value === 'number') {
    if (key === 'BatteryLevel') {
      if (value < 0) return 'unknown'
      return `${Math.round(value * 100)}%`
    }
    if (key === 'DeviceCapacity' || key === 'AvailableDeviceCapacity') {
      return `${value.toFixed(1)} GB`
    }
    return String(value)
  }
  if (Array.isArray(value)) return value.map(String).join(', ')
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

export function asRecordList(v: unknown): Record<string, unknown>[] {
  if (!Array.isArray(v)) return []
  return v.filter(
    (x): x is Record<string, unknown> => !!x && typeof x === 'object' && !Array.isArray(x),
  )
}

function dictEntries(v: unknown): [string, unknown][] {
  if (!v || typeof v !== 'object' || Array.isArray(v)) return []
  return Object.entries(v as Record<string, unknown>)
}

function mdmErrorMessages(parsed: Record<string, unknown>): string[] {
  const chain = asRecordList(parsed.ErrorChain)
  const msgs: string[] = []
  for (const e of chain) {
    const text = e.LocalizedDescription || e.USEnglishDescription || e.ErrorCode
    if (text != null && String(text).trim()) msgs.push(String(text))
  }
  return msgs
}

function friendlyMdmError(
  requestType: string,
  parsed: Record<string, unknown>,
  status: string,
): string | null {
  const msgs = mdmErrorMessages(parsed)
  const joined = msgs.join(' ').toLowerCase()
  const code = asRecordList(parsed.ErrorChain)[0]?.ErrorCode
  if (
    requestType === 'DeviceLocation' &&
    (code === 12067 || joined.includes('not in mdm lost mode'))
  ) {
    return he.mdmLocationNeedsLostMode
  }
  if (status === 'Error' || msgs.length) {
    return msgs[0] || he.mdmCommandFailed
  }
  return null
}

export type DeviceStatusChips = {
  battery?: string
  model?: string
  os?: string
  capacity?: string
  available?: string
  deviceName?: string
}

export function deviceStatusFromInfo(result: MdmCommandResult | null | undefined): DeviceStatusChips {
  const q = result?.query_responses
  if (!q) return {}
  const battery =
    q.BatteryLevel != null ? formatMdmQueryValue('BatteryLevel', q.BatteryLevel) : undefined
  const model =
    q.ProductName != null
      ? String(q.ProductName)
      : q.Model != null
        ? String(q.Model)
        : undefined
  const os = q.OSVersion != null ? String(q.OSVersion) : undefined
  const capacity =
    q.DeviceCapacity != null
      ? formatMdmQueryValue('DeviceCapacity', q.DeviceCapacity)
      : undefined
  const available =
    q.AvailableDeviceCapacity != null
      ? formatMdmQueryValue('AvailableDeviceCapacity', q.AvailableDeviceCapacity)
      : undefined
  const deviceName = q.DeviceName != null ? String(q.DeviceName) : undefined
  return { battery, model, os, capacity, available, deviceName }
}

function MdmResultToolbar({
  title,
  onShowRaw,
}: {
  title?: ReactNode
  onShowRaw: () => void
}) {
  return (
    <Flex className="mdm-result-toolbar" justify="space-between" align="center" gap={8} wrap="wrap">
      <div className="mdm-result-toolbar-title">{title}</div>
      <Button size="small" type="link" onClick={onShowRaw} style={{ paddingInline: 0 }}>
        {he.mdmShowRaw}
      </Button>
    </Flex>
  )
}

function MdmKeyValueList({
  entries,
}: {
  entries: { key: string; label: string; value: ReactNode }[]
}) {
  return (
    <div className="mdm-result-kv">
      {entries.map((row) => (
        <div key={row.key} className="mdm-result-kv-row">
          <div className="mdm-result-kv-label">{row.label}</div>
          <div className="mdm-result-kv-value">{row.value}</div>
        </div>
      ))}
    </div>
  )
}

export function MdmCommandResultView({ result }: { result: MdmCommandResult }) {
  const isMobile = useIsMobile()
  const [showRaw, setShowRaw] = useState(false)
  const [listFilter, setListFilter] = useState('')
  const parsed = result.parsed || {}
  const requestType = result.request_type || ''
  const errorText = friendlyMdmError(requestType, parsed, result.status || '')
  const scrollMax = isMobile ? 'min(52vh, 420px)' : 420

  if (showRaw) {
    return (
      <div className="mdm-result-view">
        <Flex justify="flex-end" style={{ marginBottom: 8 }}>
          <Button size="small" type="link" onClick={() => setShowRaw(false)} style={{ paddingInline: 0 }}>
            {he.close}
          </Button>
        </Flex>
        <Typography.Paragraph
          copyable
          className="mdm-result-raw"
          style={{
            marginBottom: 0,
            whiteSpace: 'pre-wrap',
            fontFamily: 'IBM Plex Mono, monospace',
            fontSize: 12,
            maxHeight: scrollMax,
            overflow: 'auto',
          }}
        >
          {result.result || JSON.stringify(parsed, null, 2)}
        </Typography.Paragraph>
      </div>
    )
  }

  if (requestType === 'InstalledApplicationList') {
    const apps = asRecordList(parsed.InstalledApplicationList)
      .slice()
      .sort((a, b) =>
        String(a.Name || a.Identifier || '').localeCompare(
          String(b.Name || b.Identifier || ''),
          'he',
        ),
      )
    const filtered = apps.filter((app) =>
      matchesQuery(
        listFilter,
        String(app.Name ?? ''),
        String(app.Identifier ?? ''),
        String(app.ShortVersion ?? ''),
        String(app.Version ?? ''),
      ),
    )
    return (
      <div className="mdm-result-view">
        <MdmResultToolbar
          onShowRaw={() => setShowRaw(true)}
          title={he.mdmAppsCount.replace('{n}', String(apps.length))}
        />
        {apps.length ? (
          <ListSearchBar
            value={listFilter}
            onChange={setListFilter}
            placeholder={he.search}
            total={apps.length}
            shown={filtered.length}
            style={{ marginBottom: 8 }}
          />
        ) : null}
        <List
          className="mdm-result-list"
          size="small"
          bordered={!isMobile}
          style={{ maxHeight: scrollMax, overflow: 'auto' }}
          dataSource={filtered}
          locale={{ emptyText: he.emptyAllow }}
          renderItem={(app) => (
            <List.Item className="mdm-result-list-item">
              <div className="mdm-result-list-body">
                <Flex justify="space-between" align="flex-start" gap={8} wrap="wrap">
                  <Typography.Text strong className="mdm-result-list-title">
                    {String(app.Name || app.Identifier || '—')}
                  </Typography.Text>
                  {app.HasUpdateAvailable ? (
                    <Tag color="orange" style={{ marginInlineEnd: 0 }}>
                      {he.mdmUpdateAvailable}
                    </Tag>
                  ) : null}
                </Flex>
                <Typography.Text
                  type="secondary"
                  className="mdm-result-list-sub"
                  copyable={
                    app.Identifier
                      ? { text: String(app.Identifier), tooltips: false }
                      : false
                  }
                >
                  {String(app.Identifier || '—')}
                </Typography.Text>
                <Typography.Text type="secondary" className="mdm-result-list-sub">
                  {he.mdmAppVersion}: {String(app.ShortVersion || app.Version || '—')}
                </Typography.Text>
              </div>
            </List.Item>
          )}
        />
      </div>
    )
  }

  if (requestType === 'ProfileList') {
    const profiles = asRecordList(parsed.ProfileList)
      .slice()
      .sort((a, b) =>
        String(a.PayloadDisplayName || a.PayloadIdentifier || '').localeCompare(
          String(b.PayloadDisplayName || b.PayloadIdentifier || ''),
          'he',
        ),
      )
    const filtered = profiles.filter((p) =>
      matchesQuery(
        listFilter,
        String(p.PayloadDisplayName ?? ''),
        String(p.PayloadIdentifier ?? ''),
        String(p.PayloadOrganization ?? ''),
      ),
    )
    return (
      <div className="mdm-result-view">
        <MdmResultToolbar
          onShowRaw={() => setShowRaw(true)}
          title={he.mdmProfilesCount.replace('{n}', String(profiles.length))}
        />
        {profiles.length ? (
          <ListSearchBar
            value={listFilter}
            onChange={setListFilter}
            placeholder={he.search}
            total={profiles.length}
            shown={filtered.length}
            style={{ marginBottom: 8 }}
          />
        ) : null}
        <List
          className="mdm-result-list"
          size="small"
          bordered={!isMobile}
          style={{ maxHeight: scrollMax, overflow: 'auto' }}
          dataSource={filtered}
          locale={{ emptyText: he.emptyAllow }}
          renderItem={(p) => (
            <List.Item className="mdm-result-list-item">
              <div className="mdm-result-list-body">
                <Typography.Text strong className="mdm-result-list-title">
                  {String(p.PayloadDisplayName || p.PayloadIdentifier || '—')}
                </Typography.Text>
                <Typography.Text
                  type="secondary"
                  className="mdm-result-list-sub"
                  copyable={
                    p.PayloadIdentifier
                      ? { text: String(p.PayloadIdentifier), tooltips: false }
                      : false
                  }
                >
                  {String(p.PayloadIdentifier || '')}
                  {p.PayloadOrganization ? ` · ${String(p.PayloadOrganization)}` : ''}
                </Typography.Text>
              </div>
            </List.Item>
          )}
        />
      </div>
    )
  }

  if (errorText) {
    return (
      <div className="mdm-result-view">
        <MdmResultToolbar
          onShowRaw={() => setShowRaw(true)}
          title={<Typography.Text type="secondary">{result.status || 'Error'}</Typography.Text>}
        />
        <Alert type="warning" showIcon message={errorText} />
      </div>
    )
  }

  if (requestType === 'DeviceLocation') {
    const lat = Number(parsed.Latitude)
    const lon = Number(parsed.Longitude)
    const hasCoords = Number.isFinite(lat) && Number.isFinite(lon)
    const pad = 0.012
    const osmEmbed = hasCoords
      ? `https://www.openstreetmap.org/export/embed.html?bbox=${encodeURIComponent(
          `${lon - pad},${lat - pad},${lon + pad},${lat + pad}`,
        )}&layer=mapnik&marker=${encodeURIComponent(`${lat},${lon}`)}`
      : ''
    const osmOpen = hasCoords
      ? `https://www.openstreetmap.org/?mlat=${lat}&mlon=${lon}#map=16/${lat}/${lon}`
      : ''
    return (
      <div className="mdm-result-view">
        <MdmResultToolbar
          onShowRaw={() => setShowRaw(true)}
          title={<Typography.Text strong>{he.mdmLocation}</Typography.Text>}
        />
        {hasCoords ? (
          <Flex vertical gap={8} style={{ width: '100%' }}>
            <Typography.Text type="secondary" copyable>
              {lat.toFixed(5)}, {lon.toFixed(5)}
            </Typography.Text>
            <iframe
              title={he.mdmLocation}
              src={osmEmbed}
              className="mdm-result-map"
              style={{
                width: '100%',
                height: isMobile ? 200 : 280,
                border: '1px solid #d9d9d9',
                borderRadius: 8,
              }}
              loading="lazy"
              referrerPolicy="no-referrer-when-downgrade"
            />
            <Button type="link" href={osmOpen} target="_blank" rel="noreferrer" block={isMobile}>
              {he.mdmOpenMaps}
            </Button>
          </Flex>
        ) : (
          <Alert type="info" showIcon message={he.mdmLocationNeedsLostMode} />
        )}
      </div>
    )
  }

  const descSource =
    result.query_responses ||
    (requestType === 'SecurityInfo' && parsed.SecurityInfo && typeof parsed.SecurityInfo === 'object'
      ? (parsed.SecurityInfo as Record<string, unknown>)
      : null) ||
    (dictEntries(parsed).length
      ? Object.fromEntries(
          dictEntries(parsed).filter(
            ([k]) => !['CommandUUID', 'Status', 'UDID', 'EnrollmentID'].includes(k),
          ),
        )
      : null)

  if (descSource && Object.keys(descSource).length) {
    const flat = Object.entries(descSource)
      .filter(([, v]) => v == null || typeof v !== 'object' || Array.isArray(v))
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([k, v]) => ({
        key: k,
        label: k,
        value: formatMdmQueryValue(k, v),
      }))
    const nested = Object.entries(descSource).filter(
      ([, v]) => v && typeof v === 'object' && !Array.isArray(v),
    )
    return (
      <div className="mdm-result-view">
        <MdmResultToolbar onShowRaw={() => setShowRaw(true)} />
        <div className="mdm-result-scroll" style={{ maxHeight: scrollMax, overflow: 'auto' }}>
          {flat.length ? (
            isMobile ? (
              <MdmKeyValueList entries={flat} />
            ) : (
              <Descriptions
                size="small"
                column={1}
                bordered
                items={flat.map((row) => ({
                  key: row.key,
                  label: row.label,
                  children: row.value,
                }))}
              />
            )
          ) : null}
          {nested.map(([k, v]) => {
            const nestedEntries = Object.entries(v as Record<string, unknown>).map(([ik, iv]) => ({
              key: ik,
              label: ik,
              value: formatMdmQueryValue(ik, iv),
            }))
            return (
              <Card key={k} size="small" title={k} style={{ marginTop: 8 }}>
                {isMobile ? (
                  <MdmKeyValueList entries={nestedEntries} />
                ) : (
                  <Descriptions
                    size="small"
                    column={1}
                    items={nestedEntries.map((row) => ({
                      key: row.key,
                      label: row.label,
                      children: row.value,
                    }))}
                  />
                )}
              </Card>
            )
          })}
        </div>
      </div>
    )
  }

  return (
    <div className="mdm-result-view">
      <MdmResultToolbar
        onShowRaw={() => setShowRaw(true)}
        title={
          <Typography.Text>
            {he.status}: {result.status || '—'}
          </Typography.Text>
        }
      />
    </div>
  )
}
