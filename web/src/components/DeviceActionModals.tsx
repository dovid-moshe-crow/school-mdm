import { App, Button, Empty, Flex, Input, Modal, Space, Spin, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { api, type MdmCommandResult } from '../api'
import { he } from '../he'
import { useIsMobile } from '../hooks/useIsMobile'
import { MdmCommandResultView } from './MdmCommandResultView'

export function DeviceActionModals({
  enrollmentId,
  mdmBusy,
  setMdmBusy,
  lostModeOpen,
  setLostModeOpen,
  eraseOpen,
  setEraseOpen,
  mdmInfoOpen,
  setMdmInfoOpen,
  mdmInfoWaiting,
  mdmInfoTitle,
  mdmInfoResult,
}: {
  enrollmentId: string
  mdmBusy: string
  setMdmBusy: (v: string) => void
  lostModeOpen: boolean
  setLostModeOpen: (v: boolean) => void
  eraseOpen: boolean
  setEraseOpen: (v: boolean) => void
  mdmInfoOpen: boolean
  setMdmInfoOpen: (v: boolean) => void
  mdmInfoWaiting: boolean
  mdmInfoTitle: string
  mdmInfoResult: MdmCommandResult | null
}) {
  const { message } = App.useApp()
  const isMobile = useIsMobile()
  const [lostMsg, setLostMsg] = useState('אבד מכשיר בית הספר')
  const [lostPhone, setLostPhone] = useState('')
  const [eraseTyped, setEraseTyped] = useState('')

  useEffect(() => {
    if (eraseOpen) setEraseTyped('')
  }, [eraseOpen])

  return (
    <>
      <Modal
        open={lostModeOpen}
        title={he.enableLostMode}
        onCancel={() => setLostModeOpen(false)}
        okText={he.ok}
        cancelText={he.close}
        confirmLoading={mdmBusy === enrollmentId + ':lost-on'}
        onOk={async () => {
          setMdmBusy(enrollmentId + ':lost-on')
          try {
            await api.mdmEnableLostMode(enrollmentId, {
              message: lostMsg,
              phone: lostPhone || undefined,
            })
            message.success(he.ok)
            setLostModeOpen(false)
          } catch (err) {
            message.error((err as Error).message)
            throw err
          } finally {
            setMdmBusy('')
          }
        }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <div>
            <Typography.Text type="secondary">{he.lostModeMessage}</Typography.Text>
            <Input
              style={{ marginTop: 4 }}
              value={lostMsg}
              onChange={(e) => setLostMsg(e.target.value)}
            />
          </div>
          <div>
            <Typography.Text type="secondary">{he.lostModePhone}</Typography.Text>
            <Input
              style={{ marginTop: 4 }}
              value={lostPhone}
              onChange={(e) => setLostPhone(e.target.value)}
            />
          </div>
        </Space>
      </Modal>
      <Modal
        open={eraseOpen}
        title={he.eraseDevice}
        onCancel={() => setEraseOpen(false)}
        okText={he.ok}
        cancelText={he.close}
        okButtonProps={{ danger: true, disabled: eraseTyped !== 'ERASE' }}
        confirmLoading={mdmBusy === enrollmentId + ':erase'}
        onOk={async () => {
          setMdmBusy(enrollmentId + ':erase')
          try {
            await api.mdmErase(enrollmentId)
            message.success(he.ok)
            setEraseOpen(false)
          } catch (err) {
            message.error((err as Error).message)
            throw err
          } finally {
            setMdmBusy('')
          }
        }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Typography.Text type="danger">{he.eraseConfirm}</Typography.Text>
          <Input
            value={eraseTyped}
            onChange={(e) => setEraseTyped(e.target.value)}
            placeholder={he.eraseTypeHint}
          />
        </Space>
      </Modal>
      <Modal
        className={isMobile ? 'mdm-result-modal' : undefined}
        rootClassName={isMobile ? 'mdm-result-modal-root' : undefined}
        open={mdmInfoOpen}
        title={`${he.mdmDeviceInfoTitle}${mdmInfoTitle ? ` · ${mdmInfoTitle}` : ''}`}
        onCancel={() => setMdmInfoOpen(false)}
        footer={[
          <Button key="close" type="primary" block={isMobile} onClick={() => setMdmInfoOpen(false)}>
            {he.close}
          </Button>,
        ]}
        width={isMobile ? '100%' : 640}
        centered={!isMobile}
        styles={
          isMobile
            ? {
                body: { padding: '12px 12px 8px', maxHeight: '70vh', overflow: 'auto' },
                wrapper: { alignItems: 'flex-end' },
              }
            : undefined
        }
      >
        {mdmInfoWaiting ? (
          <Flex vertical align="center" gap={12} style={{ padding: 24 }}>
            <Spin />
            <Typography.Text type="secondary">{he.mdmWaitingDevice}</Typography.Text>
          </Flex>
        ) : mdmInfoResult ? (
          <MdmCommandResultView result={mdmInfoResult} />
        ) : (
          <Empty />
        )}
      </Modal>
    </>
  )
}
