import { Alert, Button, Input, Spin, Typography } from 'antd'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { api } from '../api'
import { he } from '../he'

function fmtTime(v?: string) {
  if (!v) return ''
  try {
    return new Date(v).toLocaleString('he-IL', { dateStyle: 'short', timeStyle: 'short' })
  } catch {
    return v
  }
}

type Props = {
  requestId: string
  role: 'student' | 'admin'
  deviceId?: string
  closed?: boolean
  onPosted?: () => void
}

export function RequestThread({ requestId, role, deviceId, closed, onPosted }: Props) {
  const qc = useQueryClient()
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)

  const messagesQuery = useQuery({
    queryKey: ['messages', requestId, role, deviceId],
    queryFn: () => api.messages(requestId, role === 'student' ? deviceId : undefined),
    refetchInterval: 10_000,
  })
  const msgs = messagesQuery.data ?? []

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }, [msgs.length])

  useEffect(() => {
    if (messagesQuery.isError) {
      setErr(messagesQuery.error instanceof Error ? messagesQuery.error.message : 'שגיאה')
    }
  }, [messagesQuery.isError, messagesQuery.error])

  async function send() {
    const body = draft.trim()
    if (!body || busy) return
    setBusy(true)
    setErr('')
    try {
      if (role === 'admin') {
        await api.postAdminMessage(requestId, body)
      } else {
        if (!deviceId) throw new Error('חסר מזהה מכשיר')
        await api.postStudentMessage(deviceId, requestId, body)
      }
      setDraft('')
      await qc.invalidateQueries({ queryKey: ['messages', requestId] })
      onPosted?.()
    } catch (e) {
      setErr(e instanceof Error ? e.message : 'שגיאה')
    } finally {
      setBusy(false)
    }
  }

  const placeholder = role === 'admin' ? he.replyPlaceholderAdmin : he.replyPlaceholder

  return (
    <div className="thread">
      <div className="thread-scroll">
        {messagesQuery.isLoading && !msgs.length && (
          <div style={{ textAlign: 'center', padding: 12 }}>
            <Spin size="small" />
          </div>
        )}
        {!messagesQuery.isLoading && !msgs.length && (
          <Typography.Text type="secondary">{he.noMessages}</Typography.Text>
        )}
        {msgs.map((m) => {
          const fromAdmin = m.author_role === 'admin'
          const label = fromAdmin
            ? he.adminRole
            : role === 'student'
              ? he.you
              : he.studentRole
          return (
            <div key={m.id} className={`bubble ${fromAdmin ? 'admin' : 'student'}`}>
              <div className="bubble-meta">
                <strong>{label}</strong>
                <span>{fmtTime(m.created_at)}</span>
              </div>
              <div className="bubble-body">{m.body}</div>
            </div>
          )
        })}
        <div ref={bottomRef} />
      </div>
      {closed && role === 'student' && (
        <Typography.Text type="secondary">{he.reopenHint}</Typography.Text>
      )}
      <div className="thread-compose">
        <Input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder={placeholder}
          disabled={busy}
          onPressEnter={send}
        />
        <Button type="primary" onClick={send} loading={busy} disabled={!draft.trim()}>
          {he.sendReply}
        </Button>
      </div>
      {err && <Alert type="error" showIcon message={err} />}
    </div>
  )
}
