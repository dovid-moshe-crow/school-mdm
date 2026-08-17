import { App, Modal } from 'antd'
import { useCallback, useState } from 'react'
import { api, type MdmCommandResult } from '../api'
import { he } from '../he'

export function useMdmDeviceActions() {
  const { message } = App.useApp()
  const [mdmBusy, setMdmBusy] = useState('')
  const [mdmInfoOpen, setMdmInfoOpen] = useState(false)
  const [mdmInfoWaiting, setMdmInfoWaiting] = useState(false)
  const [mdmInfoTitle, setMdmInfoTitle] = useState('')
  const [mdmInfoResult, setMdmInfoResult] = useState<MdmCommandResult | null>(null)

  const queueDeviceAction = useCallback(
    async (
      id: string,
      key: string,
      fn: () => Promise<unknown>,
      confirm?: { title: string; content?: string },
    ) => {
      const run = async () => {
        setMdmBusy(id + ':' + key)
        try {
          await fn()
          message.success(he.ok)
        } catch (err) {
          message.error((err as Error).message)
        } finally {
          setMdmBusy('')
        }
      }
      if (confirm) {
        Modal.confirm({
          title: confirm.title,
          content: confirm.content,
          okText: he.ok,
          cancelText: he.close,
          onOk: run,
        })
        return
      }
      await run()
    },
    [message],
  )

  const queueAndPollResult = useCallback(
    async (
      id: string,
      label: string,
      title: string,
      enqueue: () => Promise<{ command_uuid: string }>,
      opts?: { silent?: boolean },
    ) => {
      setMdmBusy(id + ':' + label)
      setMdmInfoTitle(title)
      setMdmInfoResult(null)
      setMdmInfoWaiting(true)
      if (!opts?.silent) setMdmInfoOpen(true)
      try {
        const queued = await enqueue()
        const deadline = Date.now() + 90_000
        while (Date.now() < deadline) {
          await new Promise((r) => setTimeout(r, 2000))
          const res = await api.mdmCommandResult(id, queued.command_uuid)
          if (!res.pending) {
            setMdmInfoResult(res)
            setMdmInfoWaiting(false)
            if (opts?.silent) setMdmInfoOpen(false)
            return res
          }
        }
        message.warning(he.mdmWaitingDevice)
        setMdmInfoWaiting(false)
        return null
      } catch (err) {
        if (!opts?.silent) setMdmInfoOpen(false)
        message.error((err as Error).message)
        return null
      } finally {
        setMdmBusy('')
      }
    },
    [message],
  )

  /** Poll DeviceInformation without opening the result modal (for status chips). */
  const fetchDeviceInformation = useCallback(
    async (id: string) => {
      return queueAndPollResult(
        id,
        'info-status',
        he.mdmDeviceInfo,
        () => api.mdmDeviceInformation(id),
        { silent: true },
      )
    },
    [queueAndPollResult],
  )

  return {
    mdmBusy,
    setMdmBusy,
    mdmInfoOpen,
    setMdmInfoOpen,
    mdmInfoWaiting,
    mdmInfoTitle,
    mdmInfoResult,
    setMdmInfoResult,
    queueDeviceAction,
    queueAndPollResult,
    fetchDeviceInformation,
  }
}
