import {
  ClockCircleOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  PlusOutlined,
} from '@ant-design/icons'
import {
  Alert,
  App,
  Button,
  Card,
  Checkbox,
  DatePicker,
  Drawer,
  Empty,
  Flex,
  Input,
  List,
  Segmented,
  Space,
  Switch,
  Tag,
  TimePicker,
  Typography,
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import dayjs, { type Dayjs } from 'dayjs'
import customParseFormat from 'dayjs/plugin/customParseFormat'
import { useMemo, useState } from 'react'
import {
  api,
  type CustomProfile,
  type Device,
  type Group,
  type PolicyTimer,
  type PolicyTimerWrite,
  type WhitelistPack,
} from '../api'
import { DevicePickList, GroupPickList, PackPickList, ProfilePickList } from '../components/CheckablePickList'
import { useBusy } from '../hooks/useBusy'
import { useIsMobile } from '../hooks/useIsMobile'
import { he } from '../he'
import { deviceLabel } from '../labels'
import { formatAbsoluteHe, formatRelativeHe } from '../time'

dayjs.extend(customParseFormat)

const WEEKDAYS = [0, 1, 2, 3, 4, 5, 6] as const

type Draft = {
  name: string
  action: 'add' | 'remove'
  packIds: string[]
  profileIds: string[]
  deviceIds: string[]
  groupIds: string[]
  schedule: 'once' | 'weekly'
  runAt: Dayjs | null
  weekdays: number[]
  timeOfDay: Dayjs | null
  enabled: boolean
}

function emptyDraft(): Draft {
  return {
    name: '',
    action: 'add',
    packIds: [],
    profileIds: [],
    deviceIds: [],
    groupIds: [],
    schedule: 'weekly',
    runAt: dayjs().add(1, 'day').hour(8).minute(0).second(0).millisecond(0),
    weekdays: [0, 1, 2, 3, 4],
    timeOfDay: dayjs('08:00', 'HH:mm'),
    enabled: true,
  }
}

function draftFromTimer(t: PolicyTimer): Draft {
  return {
    name: t.name,
    action: t.action === 'remove' ? 'remove' : 'add',
    packIds: t.pack_ids || [],
    profileIds: t.profile_ids || [],
    deviceIds: t.device_ids || [],
    groupIds: t.group_ids || [],
    schedule: t.schedule === 'once' ? 'once' : 'weekly',
    runAt: t.run_at ? dayjs(t.run_at) : emptyDraft().runAt,
    weekdays: t.weekdays?.length ? t.weekdays : [0, 1, 2, 3, 4],
    timeOfDay: t.time_of_day ? dayjs(t.time_of_day, 'HH:mm') : dayjs('08:00', 'HH:mm'),
    enabled: t.enabled,
  }
}

function weekdayLabel(d: number) {
  return he.timerDays[d] || String(d)
}

export function AdminTimers({
  devices,
  groups,
  packs,
  profiles,
}: {
  devices: Device[]
  groups: Group[]
  packs: WhitelistPack[]
  profiles: CustomProfile[]
}) {
  const { message, modal } = App.useApp()
  const qc = useQueryClient()
  const isMobile = useIsMobile()
  const action = useBusy()
  const [editing, setEditing] = useState<PolicyTimer | 'new' | null>(null)
  const [draft, setDraft] = useState<Draft>(emptyDraft)

  const timersQuery = useQuery({
    queryKey: ['timers'],
    queryFn: () => api.timers(),
    enabled: true,
  })
  const timers = timersQuery.data ?? []

  const packName = useMemo(() => {
    const m = new Map(packs.map((p) => [p.id, p.name]))
    return (id: string) => m.get(id) || id
  }, [packs])
  const profileName = useMemo(() => {
    const m = new Map(profiles.map((p) => [p.id, p.name]))
    return (id: string) => m.get(id) || id
  }, [profiles])
  const groupName = useMemo(() => {
    const m = new Map(groups.map((g) => [g.id, g.name]))
    return (id: string) => m.get(id) || id
  }, [groups])

  const saveMut = useMutation({
    mutationFn: async () => {
      if (!draft.name.trim()) throw new Error(he.timerName)
      if (!draft.packIds.length && !draft.profileIds.length) throw new Error(he.timerNeedPacks)
      if (!draft.deviceIds.length && !draft.groupIds.length) throw new Error(he.timerNeedTargets)
      if (draft.schedule === 'once' && !draft.runAt?.isValid()) throw new Error(he.timerNeedWhen)
      if (draft.schedule === 'weekly' && !draft.weekdays.length) throw new Error(he.timerNeedWeekdays)
      if (draft.schedule === 'weekly' && !draft.timeOfDay?.isValid()) throw new Error(he.timerNeedWhen)
      const body: PolicyTimerWrite = {
        name: draft.name.trim(),
        action: draft.action,
        pack_ids: draft.packIds,
        profile_ids: draft.profileIds,
        device_ids: draft.deviceIds,
        group_ids: draft.groupIds,
        schedule: draft.schedule,
        enabled: draft.enabled,
      }
      if (draft.schedule === 'once' && draft.runAt) body.run_at = draft.runAt.toISOString()
      if (draft.schedule === 'weekly') {
        body.weekdays = draft.weekdays
        body.time_of_day = draft.timeOfDay!.format('HH:mm')
      }
      if (editing && editing !== 'new') return api.updateTimer(editing.id, body)
      return api.createTimer(body)
    },
    onSuccess: () => {
      message.success(he.timerSaved)
      setEditing(null)
      void qc.invalidateQueries({ queryKey: ['timers'] })
      void qc.invalidateQueries({ queryKey: ['packs'] })
      void qc.invalidateQueries({ queryKey: ['profiles'] })
    },
    onError: (err: Error) => message.error(err.message),
  })

  function openNew() {
    setDraft(emptyDraft())
    setEditing('new')
  }
  function openEdit(t: PolicyTimer) {
    setDraft(draftFromTimer(t))
    setEditing(t)
  }

  function scheduleText(t: PolicyTimer) {
    if (t.schedule === 'once') {
      return t.run_at ? formatAbsoluteHe(t.run_at) : he.timerOnce
    }
    const days = (t.weekdays || []).map(weekdayLabel).join(', ')
    return `${days || he.timerWeekly} · ${t.time_of_day || ''}`
  }

  function targetsText(t: PolicyTimer) {
    const g = (t.group_ids || []).map(groupName)
    const d = (t.device_ids || []).map((id) => deviceLabel(id, devices))
    return [...g, ...d].filter(Boolean).join(' · ')
  }

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
        {he.timersLead}
      </Typography.Paragraph>
      <Card size="small">
        <Flex justify="space-between" align="center" gap={8} wrap="wrap">
          <Typography.Text>{he.timerTzNote}</Typography.Text>
          <Button type="primary" icon={<PlusOutlined />} onClick={openNew}>
            {he.createTimer}
          </Button>
        </Flex>
      </Card>
      {timersQuery.isLoading ? <Typography.Text type="secondary">{he.loading}</Typography.Text> : null}
      {!timersQuery.isLoading && !timers.length ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={he.timerEmpty} />
      ) : (
        <List
          dataSource={timers}
          renderItem={(t) => {
            const overdue = !!(t.enabled && t.next_run_at && dayjs(t.next_run_at).isBefore(dayjs()))
            const done = t.schedule === 'once' && t.last_run_key === 'once'
            return (
              <List.Item>
                <Card size="small" style={{ width: '100%' }} onClick={() => openEdit(t)}>
                  <Flex justify="space-between" align="flex-start" gap={12} wrap="wrap">
                    <Space direction="vertical" size={4} style={{ flex: 1, minWidth: 180 }}>
                      <Space wrap>
                        <Typography.Text strong>{t.name}</Typography.Text>
                        <Tag color={t.action === 'remove' ? 'red' : 'green'}>
                          {t.action === 'remove' ? he.timerActionRemove : he.timerActionAdd}
                        </Tag>
                        {t.enabled ? <Tag color="blue">{he.timerEnabled}</Tag> : <Tag>{he.timerDisabled}</Tag>}
                        {done ? <Tag>{he.timerDone}</Tag> : null}
                        {overdue ? <Tag color="orange">{he.timerOverdue}</Tag> : null}
                      </Space>
                      <Typography.Text type="secondary">
                        {[
                          ...(t.pack_ids || []).map(packName),
                          ...(t.profile_ids || []).map(profileName),
                        ].join(', ') || he.timerPacks}
                      </Typography.Text>
                      <Typography.Text type="secondary">{targetsText(t)}</Typography.Text>
                      <Typography.Text type="secondary">
                        <ClockCircleOutlined /> {scheduleText(t)}
                      </Typography.Text>
                      {t.next_run_at && t.enabled && !done ? (
                        <Typography.Text type="secondary">
                          {he.timerNextRun}: {formatAbsoluteHe(t.next_run_at)}
                        </Typography.Text>
                      ) : null}
                      {t.last_run_at ? (
                        <Typography.Text type="secondary">
                          {he.timerLastRun}: {formatRelativeHe(t.last_run_at)}
                        </Typography.Text>
                      ) : null}
                    </Space>
                    <Space onClick={(e) => e.stopPropagation()}>
                      <Switch
                        checked={t.enabled}
                        loading={action.is('timer-en-' + t.id)}
                        onChange={(on) =>
                          void action.run('timer-en-' + t.id, async () => {
                            try {
                              await api.updateTimer(t.id, { enabled: on })
                              void qc.invalidateQueries({ queryKey: ['timers'] })
                            } catch (err) {
                              message.error((err as Error).message)
                            }
                          })
                        }
                      />
                      <Button
                        icon={<PlayCircleOutlined />}
                        loading={action.is('timer-run-' + t.id)}
                        onClick={() =>
                          void action.run('timer-run-' + t.id, async () => {
                            try {
                              const res = await api.runTimer(t.id)
                              message.success(
                                `${he.timerRan} · ${res.assignments}/${res.devices}${res.errors ? ` · ${res.errors}` : ''}`,
                              )
                              void qc.invalidateQueries({ queryKey: ['timers'] })
                              void qc.invalidateQueries({ queryKey: ['packs'] })
                              void qc.invalidateQueries({ queryKey: ['profiles'] })
                            } catch (err) {
                              message.error((err as Error).message)
                            }
                          })
                        }
                      >
                        {he.timerRunNow}
                      </Button>
                      <Button
                        danger
                        icon={<DeleteOutlined />}
                        loading={action.is('timer-del-' + t.id)}
                        onClick={() => {
                          modal.confirm({
                            title: he.timerDeleteConfirm,
                            okText: he.delete,
                            okType: 'danger',
                            onOk: () =>
                              action.run('timer-del-' + t.id, async () => {
                                await api.deleteTimer(t.id)
                                message.success(he.timerDeleted)
                                void qc.invalidateQueries({ queryKey: ['timers'] })
                              }),
                          })
                        }}
                      />
                    </Space>
                  </Flex>
                </Card>
              </List.Item>
            )
          }}
        />
      )}

      <Drawer
        open={!!editing}
        onClose={() => setEditing(null)}
        title={editing && editing !== 'new' ? he.timerEdit : he.createTimer}
        width={isMobile ? '100%' : 560}
        placement={isMobile ? 'bottom' : 'right'}
        height={isMobile ? '92%' : undefined}
        extra={
          <Button type="primary" loading={saveMut.isPending} onClick={() => saveMut.mutate()}>
            {he.save}
          </Button>
        }
      >
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div>
            <Typography.Text type="secondary">{he.timerName}</Typography.Text>
            <Input
              value={draft.name}
              onChange={(e) => setDraft({ ...draft, name: e.target.value })}
              placeholder={he.timerName}
            />
          </div>
          <div>
            <Typography.Text type="secondary">{he.timerAction}</Typography.Text>
            <Segmented
              block
              value={draft.action}
              onChange={(v) => setDraft({ ...draft, action: v === 'remove' ? 'remove' : 'add' })}
              options={[
                { value: 'add', label: he.timerActionAdd },
                { value: 'remove', label: he.timerActionRemove },
              ]}
            />
          </div>
          <div>
            <Typography.Text type="secondary">{he.timerSchedule}</Typography.Text>
            <Segmented
              block
              value={draft.schedule}
              onChange={(v) => setDraft({ ...draft, schedule: v === 'once' ? 'once' : 'weekly' })}
              options={[
                { value: 'weekly', label: he.timerWeekly },
                { value: 'once', label: he.timerOnce },
              ]}
            />
            <Typography.Paragraph type="secondary" style={{ margin: '8px 0 0' }}>
              {he.timerTzNote}
            </Typography.Paragraph>
            {draft.schedule === 'once' ? (
              <DatePicker
                showTime={{ format: 'HH:mm' }}
                format="D.M.YYYY HH:mm"
                value={draft.runAt}
                onChange={(v) => setDraft({ ...draft, runAt: v })}
                style={{ width: '100%' }}
              />
            ) : (
              <Space direction="vertical" style={{ width: '100%' }} size="small">
                <Typography.Text type="secondary">{he.timerWeekdays}</Typography.Text>
                <div className="timer-weekdays">
                  {WEEKDAYS.map((d) => (
                    <Checkbox
                      key={d}
                      checked={draft.weekdays.includes(d)}
                      onChange={(e) => {
                        const on = e.target.checked
                        setDraft({
                          ...draft,
                          weekdays: on
                            ? [...draft.weekdays, d].sort((a, b) => a - b)
                            : draft.weekdays.filter((x) => x !== d),
                        })
                      }}
                    >
                      {weekdayLabel(d)}
                    </Checkbox>
                  ))}
                </div>
                <Typography.Text type="secondary">{he.timerTimeOfDay}</Typography.Text>
                <TimePicker
                  format="HH:mm"
                  value={draft.timeOfDay}
                  onChange={(v) => setDraft({ ...draft, timeOfDay: v })}
                  style={{ width: '100%' }}
                  needConfirm={false}
                />
              </Space>
            )}
          </div>
          <Flex align="center" gap={8}>
            <Switch
              checked={draft.enabled}
              onChange={(on) => setDraft({ ...draft, enabled: on })}
            />
            <Typography.Text>{draft.enabled ? he.timerEnabled : he.timerDisabled}</Typography.Text>
          </Flex>
          {!packs.length && !profiles.length ? (
            <Alert type="warning" showIcon message={`${he.emptyPacks} ${he.emptyProfiles}`} />
          ) : null}
          <div>
            <Typography.Text type="secondary">{he.timerPacks}</Typography.Text>
            <PackPickList
              packs={packs}
              selectedKeys={draft.packIds}
              onChange={(packIds) => setDraft({ ...draft, packIds })}
            />
          </div>
          <div>
            <Typography.Text type="secondary">{he.timerProfiles}</Typography.Text>
            <ProfilePickList
              profiles={profiles}
              selectedKeys={draft.profileIds}
              onChange={(profileIds) => setDraft({ ...draft, profileIds })}
            />
          </div>
          <div>
            <Typography.Text type="secondary">{he.timerGroups}</Typography.Text>
            <GroupPickList
              groups={groups}
              selectedKeys={draft.groupIds}
              onChange={(groupIds) => setDraft({ ...draft, groupIds })}
            />
          </div>
          <div>
            <Typography.Text type="secondary">{he.timerDevices}</Typography.Text>
            <DevicePickList
              devices={devices}
              selectedKeys={draft.deviceIds}
              onChange={(deviceIds) => setDraft({ ...draft, deviceIds })}
              groupNameById={groupName}
            />
          </div>
        </Space>
      </Drawer>
    </Space>
  )
}
