import { App, Button, Card, Empty, Input, List, Segmented, Select, Space, Tag, Typography } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import {
  api,
  type AppMeta,
  type Device,
  type Group,
  type WhitelistPack,
  type WhitelistPackAssignment,
  type WhitelistPackItem,
} from '../api'
import { he } from '../he'
import { deviceLabel, deviceOptions, groupOptions, searchableSelect } from '../labels'
import { AppThumb } from '../ui'
import { AppSearchPicker } from './AppSearchPicker'
import { SearchableCollection } from './ListSearch'

export function PackEditor({
  packId,
  pack,
  items,
  assignments,
  groups,
  devices,
}: {
  packId: string
  pack: WhitelistPack
  items: WhitelistPackItem[]
  assignments: WhitelistPackAssignment[]
  groups: Group[]
  devices: Device[]
}) {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [kind, setKind] = useState<'app' | 'url'>('app')
  const [urlDraft, setUrlDraft] = useState('')
  const [assignScope, setAssignScope] = useState<'global' | 'group' | 'device'>('group')
  const [assignTarget, setAssignTarget] = useState('')

  const apps = items.filter((it) => it.kind === 'app')
  const urls = items.filter((it) => it.kind === 'url')
  const inPack = useMemo(() => new Set(apps.map((it) => it.value)), [apps])

  const metaQuery = useQuery({
    queryKey: ['pack-item-meta', packId, apps.map((a) => a.value).sort().join('|')],
    queryFn: async () => {
      const out: Record<string, AppMeta> = {}
      await Promise.all(
        apps.slice(0, 50).map(async (it) => {
          try {
            out[it.value] = await api.lookupApp(it.value)
          } catch {
            /* unknown bundle */
          }
        }),
      )
      return out
    },
    enabled: apps.length > 0,
  })
  const meta = metaQuery.data ?? {}

  const addItem = useMutation({
    mutationFn: ({ itemKind, value }: { itemKind: string; value: string }) =>
      api.addPackItem(packId, itemKind, value),
    onSuccess: async () => {
      message.success(he.ok)
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['pack', packId] }),
        qc.invalidateQueries({ queryKey: ['packs'] }),
      ])
    },
    onError: (err) => message.error((err as Error).message),
  })

  const removeItem = useMutation({
    mutationFn: ({ itemKind, value }: { itemKind: string; value: string }) =>
      api.removePackItem(packId, itemKind, value),
    onSuccess: async () => {
      message.success(he.ok)
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['pack', packId] }),
        qc.invalidateQueries({ queryKey: ['packs'] }),
      ])
    },
    onError: (err) => message.error((err as Error).message),
  })

  const addAssignment = useMutation({
    mutationFn: () =>
      api.addPackAssignment(packId, {
        scope: assignScope,
        group_id: assignScope === 'group' ? assignTarget : undefined,
        enrollment_id: assignScope === 'device' ? assignTarget : undefined,
      }),
    onSuccess: async () => {
      message.success(he.ok)
      setAssignTarget('')
      await qc.invalidateQueries({ queryKey: ['pack', packId] })
    },
    onError: (err) => message.error((err as Error).message),
  })

  const removeAssignment = useMutation({
    mutationFn: (as: WhitelistPackAssignment) =>
      api.removePackAssignment(packId, as.target_type, as.target_id),
    onSuccess: async () => {
      message.success(he.ok)
      await qc.invalidateQueries({ queryKey: ['pack', packId] })
    },
    onError: (err) => message.error((err as Error).message),
  })

  function assignmentLabel(as: WhitelistPackAssignment) {
    if (as.target_type === 'group') {
      return groups.find((g) => g.id === as.target_id)?.name || as.target_id
    }
    if (as.target_type === 'device') return deviceLabel(as.target_id, devices)
    return he.everyone
  }

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
        {he.packItemsLead.replace('{name}', pack.name)}
      </Typography.Paragraph>

      <Card size="small" title={he.packItems}>
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Segmented
            block
            value={kind}
            onChange={(v) => setKind(v as 'app' | 'url')}
            options={[
              { value: 'app', label: `${he.whitelistApps} (${apps.length})` },
              { value: 'url', label: `${he.whitelistWeb} (${urls.length})` },
            ]}
          />

          {kind === 'app' ? (
            <AppSearchPicker
              pickLabel={he.packAddApp}
              placeholder={he.packSearchStore}
              hint={he.packSearchHint}
              excludeBundles={inPack}
              onPick={async (app) => {
                await addItem.mutateAsync({ itemKind: 'app', value: app.bundle_id })
              }}
            />
          ) : (
            <div>
              <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
                {he.pasteUrlsHint}
              </Typography.Paragraph>
              <Space.Compact style={{ width: '100%' }}>
                <Input
                  value={urlDraft}
                  onChange={(e) => setUrlDraft(e.target.value)}
                  placeholder="khanacademy.org"
                  onPressEnter={() => {
                    const v = urlDraft.trim()
                    if (!v) return
                    addItem.mutate(
                      { itemKind: 'url', value: v },
                      { onSuccess: () => setUrlDraft('') },
                    )
                  }}
                />
                <Button
                  type="primary"
                  disabled={!urlDraft.trim()}
                  loading={addItem.isPending && addItem.variables?.itemKind === 'url'}
                  onClick={() => {
                    const v = urlDraft.trim()
                    if (!v) return
                    addItem.mutate(
                      { itemKind: 'url', value: v },
                      { onSuccess: () => setUrlDraft('') },
                    )
                  }}
                >
                  {he.packAddUrl}
                </Button>
              </Space.Compact>
            </div>
          )}

          {kind === 'app' ? (
            apps.length ? (
              <SearchableCollection
                items={apps}
                text={(it) => `${meta[it.value]?.app_name || ''} ${it.value}`}
                placeholder={he.searchPlaceholder}
              >
                {(rows) => (
                  <List
                    size="small"
                    header={
                      <Typography.Text type="secondary">
                        {he.packAppsInPack} · {apps.length}
                      </Typography.Text>
                    }
                    dataSource={rows}
                    renderItem={(it) => {
                      const app = meta[it.value]
                      return (
                        <List.Item
                          actions={[
                            <Button
                              key="rm"
                              type="link"
                              danger
                              size="small"
                              loading={
                                removeItem.isPending &&
                                removeItem.variables?.value === it.value
                              }
                              onClick={() =>
                                removeItem.mutate({ itemKind: 'app', value: it.value })
                              }
                            >
                              {he.removeOverride}
                            </Button>,
                          ]}
                        >
                          <List.Item.Meta
                            avatar={
                              <AppThumb name={app?.app_name || it.value} url={app?.artwork_url} />
                            }
                            title={app?.app_name || it.value}
                            description={app?.developer || it.value}
                          />
                        </List.Item>
                      )
                    }}
                  />
                )}
              </SearchableCollection>
            ) : (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={he.packEmptyApps}
              />
            )
          ) : urls.length ? (
            <SearchableCollection items={urls} text={(it) => it.value} placeholder={he.searchPlaceholder}>
              {(rows) => (
                <List
                  size="small"
                  header={
                    <Typography.Text type="secondary">
                      {he.packUrlsInPack} · {urls.length}
                    </Typography.Text>
                  }
                  dataSource={rows}
                  renderItem={(it) => (
                    <List.Item
                      actions={[
                        <Button
                          key="rm"
                          type="link"
                          danger
                          size="small"
                          onClick={() => removeItem.mutate({ itemKind: 'url', value: it.value })}
                        >
                          {he.removeOverride}
                        </Button>,
                      ]}
                    >
                      <Typography.Text>{it.value}</Typography.Text>
                    </List.Item>
                  )}
                />
              )}
            </SearchableCollection>
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={he.packEmptyUrls} />
          )}
        </Space>
      </Card>

      <Card size="small" title={he.packAssign}>
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {he.packAssignLead}
          </Typography.Paragraph>
          <Segmented
            block
            value={assignScope}
            onChange={(v) => {
              setAssignScope(v as 'global' | 'group' | 'device')
              setAssignTarget('')
            }}
            options={[
              { value: 'global', label: he.everyone },
              { value: 'group', label: he.group },
              { value: 'device', label: he.deviceEffective },
            ]}
          />
          {assignScope === 'group' ? (
            <Select
              style={{ width: '100%' }}
              placeholder={he.searchGroups}
              value={assignTarget || undefined}
              onChange={setAssignTarget}
              options={groupOptions(groups)}
              {...searchableSelect}
            />
          ) : null}
          {assignScope === 'device' ? (
            <Select
              style={{ width: '100%' }}
              placeholder={he.searchDevices}
              value={assignTarget || undefined}
              onChange={setAssignTarget}
              options={deviceOptions(devices)}
              {...searchableSelect}
            />
          ) : null}
          <Button
            type="primary"
            block
            disabled={assignScope !== 'global' && !assignTarget.trim()}
            loading={addAssignment.isPending}
            onClick={() => addAssignment.mutate()}
          >
            {he.packAssign}
          </Button>
          {assignments.length ? (
            <SearchableCollection
              items={assignments}
              text={(as) => `${as.target_type} ${assignmentLabel(as)} ${as.target_id}`}
              placeholder={he.searchPlaceholder}
            >
              {(rows) => (
                <List
                  size="small"
                  header={
                    <Typography.Text type="secondary">{he.packAssignedTo}</Typography.Text>
                  }
                  dataSource={rows}
                  renderItem={(as) => (
                    <List.Item
                      actions={[
                        <Button
                          key="rm"
                          type="link"
                          danger
                          size="small"
                          onClick={() => removeAssignment.mutate(as)}
                        >
                          {he.removeOverride}
                        </Button>,
                      ]}
                    >
                      <Space>
                        <Tag>
                          {as.target_type === 'group'
                            ? he.group
                            : as.target_type === 'device'
                              ? he.device
                              : he.everyone}
                        </Tag>
                        {assignmentLabel(as)}
                      </Space>
                    </List.Item>
                  )}
                />
              )}
            </SearchableCollection>
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={he.packAssignEmpty} />
          )}
        </Space>
      </Card>
    </Space>
  )
}
