import { ReloadOutlined } from '@ant-design/icons'
import {
  Button,
  Card,
  Empty,
  Flex,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, getAdminToken, type CreditPurchase } from '../api'
import { he } from '../he'
import { deviceLabel, deviceOptions, searchableSelect } from '../labels'
import { formatAbsoluteHe, formatRelativeHe } from '../time'

function statusTag(status: string) {
  switch (status) {
    case 'paid':
      return <Tag color="success">{he.purchaseStatusPaid}</Tag>
    case 'pending':
      return <Tag color="processing">{he.purchaseStatusPending}</Tag>
    case 'failed':
      return <Tag color="error">{he.purchaseStatusFailed}</Tag>
    case 'expired':
      return <Tag>{he.purchaseStatusExpired}</Tag>
    default:
      return <Tag>{status}</Tag>
  }
}

function fmtILS(agorot: number) {
  return `₪${(agorot / 100).toFixed(agorot % 100 === 0 ? 0 : 2)}`
}

export default function AdminCreditPurchases() {
  const [status, setStatus] = useState<string | undefined>('paid')
  const [enrollmentId, setEnrollmentId] = useState<string | undefined>()
  const [offset, setOffset] = useState(0)
  const limit = 40

  const devicesQuery = useQuery({
    queryKey: ['devices'],
    queryFn: () => api.devices(),
    enabled: !!getAdminToken(),
  })

  const filter = useMemo(
    () => ({
      status,
      enrollment_id: enrollmentId,
      limit,
      offset,
    }),
    [status, enrollmentId, offset],
  )

  const purchasesQuery = useQuery({
    queryKey: ['admin-credit-purchases', filter],
    queryFn: () => api.adminCreditPurchases(filter),
    enabled: !!getAdminToken(),
    refetchInterval: 20_000,
  })

  const rows = purchasesQuery.data?.purchases ?? []

  const columns: ColumnsType<CreditPurchase> = [
    {
      title: he.activityWhen,
      key: 'when',
      width: 140,
      render: (_, row) => {
        const at = row.paid_at || row.created_at
        return (
          <Typography.Text title={formatAbsoluteHe(at)}>{formatRelativeHe(at)}</Typography.Text>
        )
      },
    },
    {
      title: he.device,
      dataIndex: 'enrollment_id',
      ellipsis: true,
      render: (id: string, row) => (
        <Link to={`/admin/devices/${encodeURIComponent(id)}`}>
          {row.device_name || deviceLabel(id, devicesQuery.data)}
        </Link>
      ),
    },
    {
      title: he.purchasePackage,
      key: 'pkg',
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <Typography.Text>{row.package_name || he.packageCredits.replace('{n}', String(row.credits))}</Typography.Text>
          <Typography.Text type="secondary">
            {he.packageCredits.replace('{n}', String(row.credits))} · {fmtILS(row.amount_agorot)}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: he.activityResult,
      dataIndex: 'status',
      width: 110,
      render: (s: string) => statusTag(s),
    },
    {
      title: he.purchaseProvider,
      dataIndex: 'provider',
      width: 100,
      render: (p: string) => (p === 'nedarim' ? he.purchaseProviderNedarim : he.purchaseProviderFake),
    },
  ]

  return (
    <Card
      size="small"
      title={he.creditPurchases}
      extra={
        <Button
          size="small"
          icon={<ReloadOutlined />}
          loading={purchasesQuery.isFetching}
          onClick={() => void purchasesQuery.refetch()}
        >
          {he.refreshStatus}
        </Button>
      }
    >
      <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
        {he.creditPurchasesLead}
      </Typography.Paragraph>
      <Flex gap={8} wrap="wrap" style={{ marginBottom: 12 }}>
        <Select
          allowClear
          placeholder={he.activityResult}
          className="filter-field"
          value={status}
          onChange={(v) => {
            setOffset(0)
            setStatus(v)
          }}
          options={[
            { value: 'paid', label: he.purchaseStatusPaid },
            { value: 'pending', label: he.purchaseStatusPending },
            { value: 'failed', label: he.purchaseStatusFailed },
            { value: 'expired', label: he.purchaseStatusExpired },
          ]}
        />
        <Select
          allowClear
          placeholder={he.device}
          className="filter-field grow"
          value={enrollmentId}
          onChange={(v) => {
            setOffset(0)
            setEnrollmentId(v)
          }}
          options={deviceOptions(devicesQuery.data ?? [])}
          {...searchableSelect}
        />
      </Flex>
      <Table
        size="small"
        rowKey="id"
        loading={purchasesQuery.isLoading}
        dataSource={rows}
        columns={columns}
        scroll={{ x: 650 }}
        pagination={false}
        locale={{ emptyText: <Empty description={he.creditPurchasesEmpty} /> }}
      />
      <Flex justify="space-between" align="center" wrap="wrap" gap={8} style={{ marginTop: 12 }}>
        <Typography.Text type="secondary">
          {rows.length ? `${offset + 1}–${offset + rows.length}` : '0'}
        </Typography.Text>
        <Space>
          <Button disabled={offset <= 0} onClick={() => setOffset((o) => Math.max(0, o - limit))}>
            {he.activityPrev}
          </Button>
          <Button disabled={rows.length < limit} onClick={() => setOffset((o) => o + limit)}>
            {he.activityNext}
          </Button>
        </Space>
      </Flex>
    </Card>
  )
}
