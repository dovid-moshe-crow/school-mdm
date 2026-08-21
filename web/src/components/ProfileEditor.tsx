import { DownloadOutlined, InboxOutlined } from '@ant-design/icons'
import { App, Button, Card, Input, List, Segmented, Select, Space, Spin, Tag, Typography, Upload } from 'antd'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import {
  api,
  type CustomProfile,
  type CustomProfileAssignment,
  type Device,
  type Group,
} from '../api'
import { he } from '../he'
import { deviceLabel, deviceOptions, groupOptions, searchableSelect } from '../labels'
import { SearchableCollection } from './ListSearch'

export function ProfileEditor({
  profileId,
  profile,
  assignments,
  groups,
  devices,
}: {
  profileId: string
  profile: CustomProfile
  assignments: CustomProfileAssignment[]
  groups: Group[]
  devices: Device[]
}) {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [name, setName] = useState(profile.name)
  const [description, setDescription] = useState(profile.description || '')
  const [assignScope, setAssignScope] = useState<'global' | 'group' | 'device'>('group')
  const [assignTarget, setAssignTarget] = useState('')
  const [downloading, setDownloading] = useState(false)

  const saveMeta = useMutation({
    mutationFn: () => api.updateProfile(profileId, { name: name.trim(), description: description.trim() }),
    onSuccess: async () => {
      message.success(he.ok)
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['profile', profileId] }),
        qc.invalidateQueries({ queryKey: ['profiles'] }),
      ])
    },
    onError: (err) => message.error((err as Error).message),
  })

  const replaceFile = useMutation({
    mutationFn: (file: File) => api.replaceProfilePayload(profileId, file),
    onSuccess: async () => {
      message.success(he.profileReplaced)
      await Promise.all([
        qc.invalidateQueries({ queryKey: ['profile', profileId] }),
        qc.invalidateQueries({ queryKey: ['profiles'] }),
      ])
    },
    onError: (err) => message.error((err as Error).message),
  })

  const addAssignment = useMutation({
    mutationFn: () =>
      api.addProfileAssignment(profileId, {
        scope: assignScope,
        group_id: assignScope === 'group' ? assignTarget : undefined,
        enrollment_id: assignScope === 'device' ? assignTarget : undefined,
      }),
    onSuccess: async () => {
      message.success(he.ok)
      setAssignTarget('')
      await qc.invalidateQueries({ queryKey: ['profile', profileId] })
      await qc.invalidateQueries({ queryKey: ['profiles'] })
    },
    onError: (err) => message.error((err as Error).message),
  })

  const removeAssignment = useMutation({
    mutationFn: (as: CustomProfileAssignment) =>
      api.removeProfileAssignment(profileId, as.target_type, as.target_id),
    onSuccess: async () => {
      message.success(he.ok)
      await qc.invalidateQueries({ queryKey: ['profile', profileId] })
      await qc.invalidateQueries({ queryKey: ['profiles'] })
    },
    onError: (err) => message.error((err as Error).message),
  })

  function assignmentLabel(as: CustomProfileAssignment) {
    if (as.target_type === 'group') {
      return groups.find((g) => g.id === as.target_id)?.name || as.target_id
    }
    if (as.target_type === 'device') return deviceLabel(as.target_id, devices)
    return he.everyone
  }

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
        {he.profileItemsLead.replace('{name}', profile.name)}
      </Typography.Paragraph>

      <Card size="small" title={he.profileFile}>
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <div>
            <Typography.Text type="secondary">{he.profileName}</Typography.Text>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <Typography.Text type="secondary">{he.profileDescription}</Typography.Text>
            <Input.TextArea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              autoSize={{ minRows: 2, maxRows: 4 }}
            />
          </div>
          <Button
            type="primary"
            disabled={!name.trim() || saveMeta.isPending}
            loading={saveMeta.isPending}
            onClick={() => saveMeta.mutate()}
          >
            {he.save}
          </Button>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {he.profileIdentifier}: <Typography.Text copyable>{profile.payload_identifier}</Typography.Text>
          </Typography.Paragraph>
          <Typography.Text type="secondary">
            {profile.filename || 'profile.mobileconfig'}
            {profile.size_bytes ? ` · ${Math.max(1, Math.round(profile.size_bytes / 1024))} KB` : ''}
          </Typography.Text>
          <Button
            icon={<DownloadOutlined />}
            loading={downloading}
            onClick={() => {
              setDownloading(true)
              void api
                .downloadProfile(profileId, profile.filename || `${profile.payload_identifier}.mobileconfig`)
                .catch((err: Error) => message.error(err.message))
                .finally(() => setDownloading(false))
            }}
          >
            {he.profileDownload}
          </Button>
          <Upload.Dragger
            accept=".mobileconfig,.plist"
            maxCount={1}
            showUploadList={false}
            disabled={replaceFile.isPending}
            beforeUpload={(file) => {
              replaceFile.mutate(file)
              return false
            }}
          >
            <p className="ant-upload-drag-icon">
              {replaceFile.isPending ? <Spin /> : <InboxOutlined />}
            </p>
            <p className="ant-upload-text">{replaceFile.isPending ? he.loading : he.profileReplace}</p>
            <p className="ant-upload-hint">{he.profileUploadHint}</p>
          </Upload.Dragger>
        </Space>
      </Card>

      <Card size="small" title={he.packAssign}>
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {he.profileAssignLead}
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
                  header={<Typography.Text type="secondary">{he.packAssignedTo}</Typography.Text>}
                  dataSource={rows}
                  renderItem={(as) => (
                    <List.Item
                      actions={[
                        <Button
                          key="rm"
                          type="link"
                          danger
                          size="small"
                          loading={
                            removeAssignment.isPending &&
                            removeAssignment.variables?.target_type === as.target_type &&
                            removeAssignment.variables?.target_id === as.target_id
                          }
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
            <Typography.Text type="secondary">{he.packAssignEmpty}</Typography.Text>
          )}
        </Space>
      </Card>
    </Space>
  )
}
