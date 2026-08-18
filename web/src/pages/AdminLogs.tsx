import { LinkOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  Button,
  Card,
  DatePicker,
  Drawer,
  Empty,
  Flex,
  Input,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useQuery } from '@tanstack/react-query'
import type { Dayjs } from 'dayjs'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getAdminToken, type ActivityEvent } from '../api'
import { ListSearchBar } from '../components/ListSearch'
import { he } from '../he'
import { useIsMobile } from '../hooks/useIsMobile'
import { deviceLabel, deviceOptions, searchableSelect } from '../labels'
import { formatAbsoluteHe, formatRelativeHe } from '../time'
import { useDebounced } from '../ui'

const categoryOptions = [
  { value: 'mdm', label: he.activityCatMdm },
  { value: 'policy', label: he.activityCatPolicy },
  { value: 'groups', label: he.activityCatGroups },
  { value: 'requests', label: he.activityCatRequests },
  { value: 'credits', label: he.activityCatCredits },
  { value: 'devices', label: he.activityCatDevices },
  { value: 'abm', label: he.activityCatAbm },
  { value: 'system', label: he.activityCatSystem },
]

const resultOptions = [
  { value: 'ok', label: he.activityResultOk },
  { value: 'error', label: he.activityResultError },
  { value: 'info', label: he.activityResultInfo },
]

const actorOptions = [
  { value: 'admin', label: he.activityActorAdmin },
  { value: 'device', label: he.activityActorDevice },
  { value: 'system', label: he.activityActorSystem },
  { value: 'webhook', label: he.activityActorWebhook },
]

function resultColor(result: string) {
  if (result === 'error') return 'error'
  if (result === 'info') return 'processing'
  return 'success'
}

export default function AdminLogs() {
  const isMobile = useIsMobile()
  const [category, setCategory] = useState<string | undefined>()
  const [action, setAction] = useState('')
  const [enrollmentId, setEnrollmentId] = useState<string | undefined>()
  const [actorType, setActorType] = useState<string | undefined>()
  const [result, setResult] = useState<string | undefined>()
  const [q, setQ] = useState('')
  const debouncedQ = useDebounced(q, 300)
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null)
  const [offset, setOffset] = useState(0)
  const [selected, setSelected] = useState<ActivityEvent | null>(null)
  const limit = 50

  const devicesQuery = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.devices(),
    enabled: !!getAdminToken(),
  })

  const filter = useMemo(
    () => ({
      category,
      action: action.trim() || undefined,
      enrollment_id: enrollmentId,
      actor_type: actorType,
      result,
      q: debouncedQ.trim() || undefined,
      from: range?.[0]?.startOf('day').toISOString(),
      to: range?.[1]?.endOf('day').toISOString(),
      limit,
      offset,
    }),
    [category, action, enrollmentId, actorType, result, debouncedQ, range, offset],
  )

  const logsQuery = useQuery({
    queryKey: ['admin-activity', filter],
    queryFn: () => api.adminActivity(filter),
    enabled: !!getAdminToken(),
    refetchInterval: 15_000,
  })

  const events = logsQuery.data?.events ?? []

  const columns: ColumnsType<ActivityEvent> = [
    {
      title: he.activityWhen,
      dataIndex: 'at',
      width: 140,
      render: (at: string) => (
        <Typography.Text title={formatAbsoluteHe(at)}>{formatRelativeHe(at)}</Typography.Text>
      ),
    },
    {
      title: he.activityCategory,
      dataIndex: 'category',
      width: 110,
      render: (c: string) => (
        <Tag>{categoryOptions.find((o) => o.value === c)?.label || c}</Tag>
      ),
    },
    {
      title: he.activityResult,
      dataIndex: 'result',
      width: 90,
      render: (r: string) => (
        <Tag color={resultColor(r)}>
          {resultOptions.find((o) => o.value === r)?.label || r}
        </Tag>
      ),
    },
    {
      title: he.activitySummary,
      dataIndex: 'summary',
      ellipsis: true,
      render: (s: string, row) => (
        <Button type="link" style={{ paddingInline: 0, height: 'auto' }} onClick={() => setSelected(row)}>
          {s || row.action}
        </Button>
      ),
    },
    {
      title: he.activityActor,
      dataIndex: 'actor',
      width: 120,
      ellipsis: true,
      render: (actor: string, row) => (
        <Typography.Text type="secondary">
          {actorOptions.find((o) => o.value === row.actor_type)?.label || row.actor_type}
          {actor ? ` · ${actor}` : ''}
        </Typography.Text>
      ),
    },
    {
      title: he.device,
      dataIndex: 'enrollment_id',
      width: 140,
      render: (id?: string) =>
        id ? (
          <Link to={`/admin/devices/${encodeURIComponent(id)}`}>
            {deviceLabel(id, devicesQuery.data)}
            <LinkOutlined style={{ marginInlineStart: 4 }} />
          </Link>
        ) : (
          '—'
        ),
    },
  ]

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
        {he.activityLead}
      </Typography.Paragraph>
      <Card size="small">
        <Flex gap={8} wrap="wrap" align="center">
          <DatePicker.RangePicker
            className="filter-field"
            value={range}
            onChange={(v) => {
              setOffset(0)
              setRange(v)
            }}
            allowClear
          />
          <Select
            allowClear
            placeholder={he.activityCategory}
            className="filter-field"
            options={categoryOptions}
            value={category}
            onChange={(v) => {
              setOffset(0)
              setCategory(v)
            }}
          />
          <Select
            allowClear
            placeholder={he.activityResult}
            className="filter-field"
            options={resultOptions}
            value={result}
            onChange={(v) => {
              setOffset(0)
              setResult(v)
            }}
          />
          <Select
            allowClear
            placeholder={he.activityActor}
            className="filter-field"
            options={actorOptions}
            value={actorType}
            onChange={(v) => {
              setOffset(0)
              setActorType(v)
            }}
          />
          <Select
            allowClear
            placeholder={he.device}
            className="filter-field"
            value={enrollmentId}
            onChange={(v) => {
              setOffset(0)
              setEnrollmentId(v)
            }}
            options={deviceOptions(devicesQuery.data ?? [])}
            {...searchableSelect}
          />
          <Input
            allowClear
            placeholder={he.activityAction}
            className="filter-field"
            value={action}
            onChange={(e) => {
              setOffset(0)
              setAction(e.target.value)
            }}
          />
          <ListSearchBar
            placeholder={he.activitySearch}
            className="filter-field grow"
            value={q}
            onChange={(v) => {
              setOffset(0)
              setQ(v)
            }}
          />
          <Button
            icon={<ReloadOutlined />}
            loading={logsQuery.isFetching}
            onClick={() => void logsQuery.refetch()}
            block={isMobile}
          >
            {he.refreshStatus}
          </Button>
        </Flex>
      </Card>

      <Table
        size="small"
        rowKey="id"
        loading={logsQuery.isLoading}
        dataSource={events}
        columns={columns}
        scroll={{ x: 720 }}
        pagination={false}
        locale={{ emptyText: <Empty description={he.activityEmpty} /> }}
        onRow={(row) => ({
          onClick: () => setSelected(row),
          style: { cursor: 'pointer' },
        })}
      />

      <Flex justify="space-between" align="center" wrap="wrap" gap={8}>
        <Typography.Text type="secondary">
          {events.length ? `${offset + 1}–${offset + events.length}` : '0'}
        </Typography.Text>
        <Space>
          <Button disabled={offset <= 0} onClick={() => setOffset((o) => Math.max(0, o - limit))}>
            {he.activityPrev}
          </Button>
          <Button
            disabled={events.length < limit}
            onClick={() => setOffset((o) => o + limit)}
          >
            {he.activityNext}
          </Button>
        </Space>
      </Flex>

      <Drawer
        title={selected?.summary || he.activityDetail}
        open={!!selected}
        onClose={() => setSelected(null)}
        placement={isMobile ? 'bottom' : 'right'}
        width={isMobile ? '100%' : 420}
        height={isMobile ? '92%' : undefined}
        destroyOnHidden
      >
        {selected ? (
          <Space direction="vertical" size="small" style={{ width: '100%' }}>
            <Typography.Text type="secondary">
              {formatAbsoluteHe(selected.at)} · {formatRelativeHe(selected.at)}
            </Typography.Text>
            <div>
              <Tag>{selected.category}</Tag>
              <Tag color={resultColor(selected.result)}>{selected.result}</Tag>
              <Tag>{selected.action}</Tag>
            </div>
            <Typography.Text>
              {he.activityActor}: {selected.actor_type}
              {selected.actor ? ` · ${selected.actor}` : ''}
            </Typography.Text>
            {selected.enrollment_id ? (
              <Link to={`/admin/devices/${encodeURIComponent(selected.enrollment_id)}`}>
                {he.openDevicePage}
              </Link>
            ) : null}
            {selected.request_id ? (
              <Typography.Text type="secondary">
                {he.activityRequestId}: {selected.request_id}
              </Typography.Text>
            ) : null}
            {selected.command_uuid ? (
              <Typography.Text type="secondary">
                {he.activityCommandId}: {selected.command_uuid}
              </Typography.Text>
            ) : null}
            <Typography.Paragraph>
              <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', margin: 0 }}>
                {JSON.stringify(selected.detail ?? {}, null, 2)}
              </pre>
            </Typography.Paragraph>
          </Space>
        ) : null}
      </Drawer>
    </Space>
  )
}
