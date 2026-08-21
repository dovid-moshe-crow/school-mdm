import { Button, Space, Typography } from 'antd'
import { api, type Device } from '../api'
import { he } from '../he'

type QueueAction = (
  id: string,
  key: string,
  fn: () => Promise<unknown>,
  confirm?: { title: string; content?: string },
) => Promise<void>

type QueuePoll = (
  id: string,
  label: string,
  title: string,
  enqueue: () => Promise<{ command_uuid: string }>,
) => Promise<unknown>

export function DeviceMdmActions({
  device,
  variant,
  mdmBusy,
  queueDeviceAction,
  queueAndPollResult,
  onOpenLostMode,
  onOpenErase,
}: {
  device: Device
  variant: 'full' | 'quick'
  mdmBusy: string
  queueDeviceAction: QueueAction
  queueAndPollResult: QueuePoll
  onOpenLostMode: () => void
  onOpenErase: () => void
}) {
  const id = device.enrollment_id
  const title = device.serial_number || device.name || device.enrollment_id
  const mdm = !!device.mdm
  const deviceBusy = mdmBusy.startsWith(id + ':')
  const busy = (key: string) => mdmBusy === id + ':' + key
  const propsFor = (key: string) => ({
    disabled: !mdm || (deviceBusy && !busy(key)),
    loading: busy(key),
  })

  return (
    <Space direction="vertical" size="small" style={{ width: '100%' }}>
      <div className="action-btn-grid">
        <Button
          size="small"
          danger
          {...propsFor('lock')}
          onClick={() =>
            void queueDeviceAction(
              id,
              'lock',
              () => api.mdmLock(id, { message: 'School MDM' }),
              { title: he.lockConfirm },
            )
          }
        >
          {he.lockDevice}
        </Button>
        <Button
          size="small"
          {...propsFor('pass')}
          onClick={() =>
            void queueDeviceAction(id, 'pass', () => api.mdmClearPasscode(id), {
              title: he.clearPasscodeConfirm,
            })
          }
        >
          {he.clearPasscode}
        </Button>
        <Button
          size="small"
          {...propsFor('restart')}
          onClick={() =>
            void queueDeviceAction(id, 'restart', () => api.mdmRestart(id), {
              title: he.restartConfirm,
            })
          }
        >
          {he.restartDevice}
        </Button>
        <Button
          size="small"
          {...propsFor('off')}
          onClick={() =>
            void queueDeviceAction(id, 'off', () => api.mdmShutDown(id), {
              title: he.shutDownConfirm,
            })
          }
        >
          {he.shutDownDevice}
        </Button>
      </div>

      <Typography.Text type="secondary">{he.lostMode}</Typography.Text>
      <div className="action-btn-grid">
        <Button
          size="small"
          disabled={!mdm || deviceBusy}
          loading={busy('lost-on')}
          onClick={onOpenLostMode}
        >
          {he.enableLostMode}
        </Button>
        <Button
          size="small"
          {...propsFor('lost-off')}
          onClick={() =>
            void queueDeviceAction(id, 'lost-off', () => api.mdmDisableLostMode(id))
          }
        >
          {he.disableLostMode}
        </Button>
        {variant === 'full' ? (
          <Button
            size="small"
            {...propsFor('sound')}
            onClick={() =>
              void queueDeviceAction(id, 'sound', () => api.mdmPlayLostModeSound(id))
            }
          >
            {he.playLostSound}
          </Button>
        ) : null}
        <Button
          size="small"
          {...propsFor('loc')}
          onClick={() =>
            void queueAndPollResult(id, 'loc', title, () => api.mdmDeviceLocation(id))
          }
        >
          {he.locateDevice}
        </Button>
      </div>

      {variant === 'full' ? (
        <>
          <Typography.Text type="secondary">{he.deviceInfoSection}</Typography.Text>
          <div className="action-btn-grid">
            <Button
              size="small"
              {...propsFor('info')}
              onClick={() =>
                void queueAndPollResult(id, 'info', title, () => api.mdmDeviceInformation(id))
              }
            >
              {he.mdmDeviceInfo}
            </Button>
            <Button
              size="small"
              {...propsFor('sec')}
              onClick={() =>
                void queueAndPollResult(id, 'sec', title, () => api.mdmSecurityInfo(id))
              }
            >
              {he.securityInfo}
            </Button>
            <Button
              size="small"
              {...propsFor('apps')}
              onClick={() =>
                void queueAndPollResult(id, 'apps', title, () => api.mdmInstalledApps(id))
              }
            >
              {he.installedApps}
            </Button>
            <Button
              size="small"
              {...propsFor('profiles')}
              onClick={() =>
                void queueAndPollResult(id, 'profiles', title, () => api.mdmProfileList(id))
              }
            >
              {he.profileList}
            </Button>
            <Button
              size="small"
              {...propsFor('companion-cfg')}
              onClick={() =>
                void queueDeviceAction(id, 'companion-cfg', () => api.mdmConfigureCompanion(id))
              }
            >
              {he.companionConfig}
            </Button>
            <Button
              size="small"
              {...propsFor('companion')}
              onClick={() =>
                void queueDeviceAction(id, 'companion', () => api.mdmInstallCompanion(id))
              }
            >
              {he.companionPush}
            </Button>
          </div>
          <Button
            size="small"
            danger
            block
            disabled={!mdm || deviceBusy}
            loading={busy('erase')}
            onClick={onOpenErase}
          >
            {he.eraseDevice}
          </Button>
        </>
      ) : (
        <Button
          size="small"
          danger
          block
          disabled={!mdm || deviceBusy}
          loading={busy('erase')}
          onClick={onOpenErase}
        >
          {he.eraseDevice}
        </Button>
      )}
    </Space>
  )
}
