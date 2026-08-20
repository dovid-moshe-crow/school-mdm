import {
  ApiOutlined,
  CopyOutlined,
  DeleteOutlined,
  ExperimentOutlined,
  LockOutlined,
  PlusOutlined,
  RobotOutlined,
} from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Alert,
  App,
  Button,
  Card,
  Collapse,
  ConfigProvider,
  Drawer,
  Flex,
  Form,
  Input,
  Segmented,
  Select,
  Space,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
} from 'antd'
import enUS from 'antd/locale/en_US'
import heIL from 'antd/locale/he_IL'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  api,
  getAdminToken,
  setAdminToken,
  type OpenAPIOperation,
  type OpenAPIParameter,
  type OpenAPISpec,
  type WebhookDelivery,
  type WebhookEndpoint,
} from '../api'
import { buildAiInstructions, listOperations } from '../apiDocs/aiInstructions'
import { useListSearch } from '../hooks/useListSearch'
import { theme } from '../theme'

type Lang = 'he' | 'en'

const METHOD_COLOR: Record<string, string> = {
  get: 'blue',
  post: 'green',
  put: 'orange',
  patch: 'gold',
  delete: 'red',
}

const t = {
  he: {
    title: 'ממשק API',
    brand: 'ניהול בית ספר',
    lead: 'כל מה שיש במסך הניהול זמין גם מכאן: מכשירים, קבוצות, רשימות מותרים, בקשות, קרדיטים, MDM, ABM — ואירועים בזמן אמת דרך webhooks.',
    copyAi: 'העתקת הוראות ל־AI',
    copiedAi: 'ההוראות הועתקו — הדביקו בצ׳אט של העוזר',
    copyFail: 'ההעתקה נכשלה',
    token: 'אסימון ניהול',
    saveToken: 'שמירת אסימון',
    tokenSaved: 'האסימון נשמר בדפדפן זה',
    admin: 'חזרה לניהול',
    tabStart: 'איך מתחילים',
    tabRef: 'נתיבים',
    tabHooks: 'Webhooks',
    tabTokens: 'אסימונים',
    stepAuthTitle: '1. אסימון',
    stepAuth: 'הממשק משתמש ב-Google. לסקריפטים צרו אסימון בכרטיסייה «אסימונים» ושולחים Authorization: Bearer …',
    stepTryTitle: '2. ניסוי נתיב',
    stepTry: 'בכרטיסייה «נתיבים» חפשו פעולה, מלאו מזהה מכשיר אם צריך, ולחצו ניסיון. נתיבים עם מנעול דורשים אסימון.',
    stepHookTitle: '3. Webhooks',
    stepHook: 'כל פעולה ביומן נשלחת החוצה כ־POST JSON. חתימה: HMAC-SHA256 של גוף הבקשה. סינון: * או mdm.* או שם מדויק.',
    specUrl: 'מפרט OpenAPI',
    searchOps: 'חיפוש נתיב, פעולה או תיאור…',
    tryIt: 'ניסיון',
    copyCurl: 'העתקת curl',
    copiedCurl: 'curl הועתק',
    jsonBody: 'גוף JSON',
    adminLock: 'דורש אסימון',
    public: 'פתוח',
    noOps: 'אין נתיבים שתואמים לחיפוש.',
    hooksLead:
      'השרת שולח כל אירוע פעילות (אישור בקשה, נעילת מכשיר, שינוי קבוצה וכו׳) לכתובת HTTPS שלכם. אשרו את החתימה לפני שסומכים על הבקשה.',
    hooksVerify: 'אימות חתימה',
    addEndpoint: 'הוספת כתובת',
    hooksList: 'כתובות קצה',
    tokenNeeded: 'צריך להיות מחוברים לניהול כדי לנהל webhooks.',
    events: 'אירועים',
    enabled: 'פעיל',
    test: 'בדיקה',
    deliveries: 'משלוחים',
    edit: 'עריכה',
    del: 'מחיקה',
    catalog: 'קטלוג אירועים',
    newHook: 'Webhook חדש',
    editHook: 'עריכת webhook',
    save: 'שמירה',
    hookUrl: 'כתובת הקצה',
    hookDesc: 'תיאור',
    hookSecret: 'סוד חתימה',
    hookSecretHint: 'ריק = יישמר הקיים, או יווצר אוטומטית ביצירה.',
    created: 'נוצר — העתיקו את סוד החתימה',
    updated: 'עודכן',
    deleted: 'נמחק',
    delConfirm: 'למחוק את ה־webhook?',
    tokensLead: 'אסימון לסקריפטים בלבד. הממשק עצמו מחובר עם Google. האסימון מוצג פעם אחת.',
    tokenName: 'שם',
    tokenCreate: 'יצירת אסימון',
    tokenCreated: 'העתיקו עכשיו — לא יוצג שוב',
    tokenRevoke: 'ביטול',
    tokenEmpty: 'אין אסימונים עדיין.',
    tokenLastUsed: 'שימוש אחרון',
    tokenPrefix: 'קידומת',
    tokenNever: 'עדיין לא',
  },
  en: {
    title: 'API',
    brand: 'School MDM',
    lead: 'Everything in the admin panel is available over REST: devices, groups, allowlists, requests, credits, MDM, ABM — plus live activity webhooks.',
    copyAi: 'Copy instructions for AI',
    copiedAi: 'Copied — paste into your assistant chat',
    copyFail: 'Could not copy',
    token: 'Admin token',
    saveToken: 'Save token',
    tokenSaved: 'Token saved in this browser',
    admin: 'Back to admin',
    tabStart: 'Start here',
    tabRef: 'Endpoints',
    tabHooks: 'Webhooks',
    tabTokens: 'Tokens',
    stepAuthTitle: '1. Token',
    stepAuth: 'The admin UI uses Google. For scripts, create a token on the Tokens tab and send Authorization: Bearer …',
    stepTryTitle: '2. Try a route',
    stepTry: 'On Endpoints, search an action, fill a device id if needed, and hit Try. Padlock routes need the token.',
    stepHookTitle: '3. Webhooks',
    stepHook: 'Every activity-log event is POSTed as JSON. Verify HMAC-SHA256 of the raw body. Filters: *, mdm.*, or an exact name.',
    specUrl: 'OpenAPI spec',
    searchOps: 'Search path, action, or summary…',
    tryIt: 'Try it',
    copyCurl: 'Copy curl',
    copiedCurl: 'curl copied',
    jsonBody: 'JSON body',
    adminLock: 'Needs token',
    public: 'Open',
    noOps: 'No endpoints match that search.',
    hooksLead:
      'The server POSTs every activity event (request approved, device locked, group changed, …) to your HTTPS URL. Verify the signature before trusting the body.',
    hooksVerify: 'Verify signature',
    addEndpoint: 'Add endpoint',
    hooksList: 'Endpoints',
    tokenNeeded: 'Sign in to admin to manage webhooks.',
    events: 'Events',
    enabled: 'On',
    test: 'Test',
    deliveries: 'Deliveries',
    edit: 'Edit',
    del: 'Delete',
    catalog: 'Event catalog',
    newHook: 'New webhook',
    editHook: 'Edit webhook',
    save: 'Save',
    hookUrl: 'Endpoint URL',
    hookDesc: 'Description',
    hookSecret: 'Signing secret',
    hookSecretHint: 'Leave blank to keep the current secret, or to generate one on create.',
    created: 'Created — copy the signing secret',
    updated: 'Updated',
    deleted: 'Deleted',
    delConfirm: 'Delete this webhook?',
    tokensLead: 'Tokens are for scripts. The admin UI uses Google. The secret is shown once.',
    tokenName: 'Name',
    tokenCreate: 'Create token',
    tokenCreated: 'Copy it now — it is not shown again',
    tokenRevoke: 'Revoke',
    tokenEmpty: 'No tokens yet.',
    tokenLastUsed: 'Last used',
    tokenPrefix: 'Prefix',
    tokenNever: 'Never',
  },
}

type OpRow = {
  method: string
  path: string
  tag: string
  op: OpenAPIOperation
  admin: boolean
}

function isAdminOp(op: OpenAPIOperation): boolean {
  const sec = op.security
  if (!Array.isArray(sec) || sec.length === 0) return false
  return sec.some((item) => item && typeof item === 'object' && 'bearerAuth' in item)
}

function flattenSpec(spec: OpenAPISpec | undefined): OpRow[] {
  if (!spec?.paths) return []
  const out: OpRow[] = []
  for (const [path, item] of Object.entries(spec.paths)) {
    for (const [method, op] of Object.entries(item)) {
      if (!op || typeof op !== 'object') continue
      out.push({
        method: method.toLowerCase(),
        path,
        tag: op.tags?.[0] || 'Other',
        op,
        admin: isAdminOp(op),
      })
    }
  }
  return out
}

function exampleBody(op: OpenAPIOperation): string {
  const schema = op.requestBody?.content?.['application/json']?.schema
  const props = schema?.properties
  if (!props) return op.requestBody ? '{\n  \n}' : ''
  const sample: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(props)) {
    const meta = (v as { type?: string; example?: unknown; enum?: unknown[] }) || {}
    if (meta.example !== undefined) sample[k] = meta.example
    else if (meta.enum?.[0] !== undefined) sample[k] = meta.enum[0]
    else if (meta.type === 'boolean') sample[k] = true
    else if (meta.type === 'array') sample[k] = []
    else if (meta.type === 'object') sample[k] = {}
    else if (meta.type === 'integer' || meta.type === 'number') sample[k] = 0
    else sample[k] = ''
  }
  return JSON.stringify(sample, null, 2)
}

function curlFor(row: OpRow, origin: string, values: Record<string, string>, body: string): string {
  let path = row.path
  const q = new URLSearchParams()
  for (const p of row.op.parameters || []) {
    const v = (values[`${p.in}:${p.name}`] || '').trim()
    if (p.in === 'path') path = path.replace(`{${p.name}}`, encodeURIComponent(v || p.name))
    if (p.in === 'query' && v) q.set(p.name, v)
  }
  const url = `${origin}${q.toString() ? `${path}?${q}` : path}`
  const parts = [`curl -sS -X ${row.method.toUpperCase()}`]
  if (row.admin) parts.push('-H "Authorization: Bearer $TOKEN"')
  if (row.op.requestBody && row.method !== 'get') {
    parts.push('-H "Content-Type: application/json"')
    parts.push(`-d '${body.replace(/'/g, `'\\''`)}'`)
  }
  parts.push(`"${url}"`)
  return parts.join(' \\\n  ')
}

function MethodTag({ method }: { method: string }) {
  return (
    <Tag color={METHOD_COLOR[method] || 'default'} className="api-method-tag">
      {method.toUpperCase()}
    </Tag>
  )
}

function OperationCard({
  row,
  lang,
  origin,
}: {
  row: OpRow
  lang: Lang
  origin: string
}) {
  const ui = t[lang]
  const { message } = App.useApp()
  const [values, setValues] = useState<Record<string, string>>({})
  const [body, setBody] = useState(() => exampleBody(row.op))
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState('')
  const params = row.op.parameters || []
  const hasBody = !!row.op.requestBody

  async function send() {
    setBusy(true)
    setResult('')
    try {
      let path = row.path
      const q = new URLSearchParams()
      for (const p of params) {
        const v = (values[`${p.in}:${p.name}`] || '').trim()
        if (p.in === 'path') path = path.replace(`{${p.name}}`, encodeURIComponent(v || p.name))
        if (p.in === 'query' && v) q.set(p.name, v)
      }
      const url = q.toString() ? `${path}?${q}` : path
      const headers: Record<string, string> = {}
      const token = getAdminToken()
      if (token) headers.Authorization = `Bearer ${token}`
      const init: RequestInit = { method: row.method.toUpperCase(), headers, credentials: 'include' }
      if (hasBody && row.method !== 'get') {
        headers['Content-Type'] = 'application/json'
        init.body = body
      }
      const res = await fetch(url, init)
      const text = await res.text()
      let pretty = text
      try {
        pretty = JSON.stringify(JSON.parse(text), null, 2)
      } catch {
        /* keep */
      }
      setResult(`${res.status} ${res.statusText}\n\n${pretty}`)
    } catch (err) {
      message.error(err instanceof Error ? err.message : ui.copyFail)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="api-op">
      <Flex align="center" gap={8} wrap="wrap">
        <MethodTag method={row.method} />
        <Typography.Text code className="api-op-path" dir="ltr">
          {row.path}
        </Typography.Text>
        {row.admin ? (
          <Tag icon={<LockOutlined />}>{ui.adminLock}</Tag>
        ) : (
          <Tag>{ui.public}</Tag>
        )}
      </Flex>
      <Typography.Paragraph className="api-op-summary">
        {row.op.summary || row.op.operationId}
      </Typography.Paragraph>
      {row.op.description ? (
        <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
          {row.op.description}
        </Typography.Paragraph>
      ) : null}
      {params.map((p) => (
        <ParamField
          key={`${p.in}:${p.name}`}
          param={p}
          value={values[`${p.in}:${p.name}`] || ''}
          onChange={(v) => setValues((prev) => ({ ...prev, [`${p.in}:${p.name}`]: v }))}
        />
      ))}
      {hasBody ? (
        <div style={{ marginBottom: 12 }}>
          <Typography.Text type="secondary">{ui.jsonBody}</Typography.Text>
          <Input.TextArea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            autoSize={{ minRows: 3, maxRows: 12 }}
            spellCheck={false}
            dir="ltr"
            className="api-code-input"
          />
        </div>
      ) : null}
      <Space wrap>
        <Button type="primary" loading={busy} onClick={() => void send()}>
          {ui.tryIt}
        </Button>
        <Button
          icon={<CopyOutlined />}
          onClick={() => {
            void navigator.clipboard.writeText(curlFor(row, origin, values, body))
            message.success(ui.copiedCurl)
          }}
        >
          {ui.copyCurl}
        </Button>
      </Space>
      {result ? <pre className="api-result" dir="ltr">{result}</pre> : null}
    </div>
  )
}

function ParamField({
  param,
  value,
  onChange,
}: {
  param: OpenAPIParameter
  value: string
  onChange: (v: string) => void
}) {
  return (
    <div className="api-param">
      <Flex gap={8} align="baseline" wrap="wrap">
        <Typography.Text code dir="ltr">{param.name}</Typography.Text>
        <Tag>{param.in}</Tag>
        {param.required ? <Tag color="red">required</Tag> : null}
        {param.description ? (
          <Typography.Text type="secondary">{param.description}</Typography.Text>
        ) : null}
      </Flex>
      <Input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={param.name}
        allowClear
        dir="ltr"
        style={{ marginTop: 6 }}
      />
    </div>
  )
}

function StartTab({
  lang,
  origin,
  spec,
  onCopyAi,
}: {
  lang: Lang
  origin: string
  spec?: OpenAPISpec
  onCopyAi: () => void
}) {
  const ui = t[lang]
  const n = listOperations(spec).length
  return (
    <div className="api-start">
      <div className="api-step-grid">
        <Card className="api-step-card" title={ui.stepAuthTitle}>
          <Typography.Paragraph>{ui.stepAuth}</Typography.Paragraph>
        </Card>
        <Card className="api-step-card" title={ui.stepTryTitle}>
          <Typography.Paragraph>{ui.stepTry}</Typography.Paragraph>
        </Card>
        <Card className="api-step-card" title={ui.stepHookTitle}>
          <Typography.Paragraph>{ui.stepHook}</Typography.Paragraph>
        </Card>
      </div>
      <Card className="api-ai-card">
        <Flex justify="space-between" align="center" wrap="wrap" gap={12}>
          <div>
            <Typography.Title level={4} style={{ margin: '0 0 6px' }}>
              {lang === 'he' ? 'עבודה עם עוזר AI' : 'Working with an AI assistant'}
            </Typography.Title>
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              {lang === 'he'
                ? 'הכפתור מעתיק מדריך מלא (עברית + אנגלית): כתובת בסיס, אימות, דוגמאות curl, פורמט webhook, וכל הנתיבים. מדביקים בצ׳אט.'
                : 'Copies a full brief (Hebrew + English): base URL, auth, curl examples, webhook format, and every endpoint. Paste it into chat.'}
            </Typography.Paragraph>
          </div>
          <Button type="primary" size="large" icon={<RobotOutlined />} onClick={onCopyAi}>
            {ui.copyAi}
          </Button>
        </Flex>
      </Card>
      <Card size="small">
        <Space direction="vertical" size="small" style={{ width: '100%' }}>
          <Typography.Text type="secondary">{ui.specUrl}</Typography.Text>
          <Typography.Text copyable code dir="ltr">
            {origin}/api/openapi.json
          </Typography.Text>
          <Typography.Text type="secondary">
            {n} {lang === 'he' ? 'נתיבים במפרט' : 'endpoints in the spec'}
          </Typography.Text>
        </Space>
      </Card>
    </div>
  )
}

function ReferenceTab({
  spec,
  lang,
  origin,
}: {
  spec: OpenAPISpec | undefined
  lang: Lang
  origin: string
}) {
  const ui = t[lang]
  const rows = useMemo(() => flattenSpec(spec), [spec])
  const search = useListSearch(rows, (r) =>
    `${r.method} ${r.path} ${r.tag} ${r.op.summary || ''} ${r.op.description || ''}`,
  )
  const tags = spec?.tags?.map((t0) => t0.name) || [...new Set(rows.map((r) => r.tag))]
  const visibleTags = tags.filter((tag) => search.visible.some((r) => r.tag === tag))

  const items = visibleTags.map((tag) => {
    const ops = search.visible.filter((r) => r.tag === tag)
    const desc = spec?.tags?.find((x) => x.name === tag)?.description
    return {
      key: tag,
      label: `${tag} (${ops.length})`,
      children: (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          {desc ? <Typography.Paragraph type="secondary">{desc}</Typography.Paragraph> : null}
          <Collapse
            bordered={false}
            items={ops.map((row) => ({
              key: `${row.method}:${row.path}`,
              label: (
                <Flex align="center" gap={8} wrap="wrap">
                  <MethodTag method={row.method} />
                  <span className="api-op-path" dir="ltr">
                    {row.path}
                  </span>
                  <Typography.Text type="secondary">{row.op.summary}</Typography.Text>
                </Flex>
              ),
              children: <OperationCard row={row} lang={lang} origin={origin} />,
            }))}
          />
        </Space>
      ),
    }
  })

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Input.Search
        allowClear
        value={search.query}
        onChange={(e) => search.setQuery(e.target.value)}
        placeholder={ui.searchOps}
      />
      {items.length === 0 ? (
        <Alert type="info" message={ui.noOps} />
      ) : (
        <Collapse items={items} accordion defaultActiveKey={visibleTags[0]} />
      )}
    </Space>
  )
}

function WebhooksTab({ lang }: { lang: Lang }) {
  const ui = t[lang]
  const { message, modal } = App.useApp()
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<WebhookEndpoint | null>(null)
  const [deliveriesFor, setDeliveriesFor] = useState<WebhookEndpoint | null>(null)
  const [form] = Form.useForm()

  const eventsQuery = useQuery({ queryKey: ['webhook-events'], queryFn: () => api.webhookEvents() })
  const listQuery = useQuery({
    queryKey: ['webhooks'],
    queryFn: () => api.webhooks(),
    enabled: true,
  })
  const deliveriesQuery = useQuery({
    queryKey: ['webhook-deliveries', deliveriesFor?.id],
    queryFn: () => api.webhookDeliveries(deliveriesFor!.id),
    enabled: !!deliveriesFor,
  })

  const saveMut = useMutation({
    mutationFn: async (values: {
      url: string
      description?: string
      events?: string[]
      enabled?: boolean
      secret?: string
    }) => {
      if (editing) return api.updateWebhook(editing.id, values)
      return api.createWebhook(values)
    },
    onSuccess: (ep) => {
      void qc.invalidateQueries({ queryKey: ['webhooks'] })
      setOpen(false)
      setEditing(null)
      form.resetFields()
      message.success(editing ? ui.updated : ui.created)
      if (!editing && ep.secret) {
        modal.info({
          title: ui.hookSecret,
          content: (
            <Typography.Paragraph copyable style={{ marginBottom: 0 }} dir="ltr">
              {ep.secret}
            </Typography.Paragraph>
          ),
        })
      }
    },
    onError: (err: Error) => message.error(err.message),
  })

  const delMut = useMutation({
    mutationFn: (id: string) => api.deleteWebhook(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['webhooks'] })
      message.success(ui.deleted)
    },
    onError: (err: Error) => message.error(err.message),
  })

  const testMut = useMutation({
    mutationFn: (id: string) => api.testWebhook(id),
    onSuccess: (d) => {
      void qc.invalidateQueries({ queryKey: ['webhook-deliveries'] })
      message.success(d.status === 'success' ? `HTTP ${d.http_status}` : d.error || d.status)
    },
    onError: (err: Error) => message.error(err.message),
  })

  const options = [
    { value: '*', label: '* — *' },
    ...[...new Set((eventsQuery.data?.events || []).map((e) => e.category))].map((c) => ({
      value: `${c}.*`,
      label: `${c}.*`,
    })),
    ...(eventsQuery.data?.events || []).map((e) => ({
      value: e.name,
      label: `${e.name} — ${e.description}`,
    })),
  ]

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <Alert
        type="info"
        showIcon
        message={ui.tabHooks}
        description={
          <div>
            <Typography.Paragraph style={{ marginBottom: 8 }}>{ui.hooksLead}</Typography.Paragraph>
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              {ui.hooksVerify}: <Typography.Text code dir="ltr">X-SchoolMDM-Signature: sha256=…</Typography.Text>
              {' — HMAC-SHA256. '}
              <Typography.Text code>*</Typography.Text>
              {' / '}
              <Typography.Text code>mdm.*</Typography.Text>
              {' / '}
              <Typography.Text code>requests.request_approve</Typography.Text>
            </Typography.Paragraph>
          </div>
        }
      />
      <Flex justify="space-between" align="center" wrap="wrap" gap={8}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          {ui.hooksList}
        </Typography.Title>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => {
            setEditing(null)
            form.resetFields()
            form.setFieldsValue({ events: ['*'], enabled: true })
            setOpen(true)
          }}
        >
          {ui.addEndpoint}
        </Button>
      </Flex>
      <Table
        rowKey="id"
        loading={listQuery.isLoading}
        dataSource={listQuery.data?.endpoints || []}
        pagination={false}
        columns={[
          {
            title: ui.hookUrl,
            dataIndex: 'url',
            render: (url: string, row: WebhookEndpoint) => (
              <Space direction="vertical" size={0}>
                <Typography.Text dir="ltr">{url}</Typography.Text>
                {row.description ? (
                  <Typography.Text type="secondary">{row.description}</Typography.Text>
                ) : null}
              </Space>
            ),
          },
          {
            title: ui.events,
            dataIndex: 'events',
            render: (events: string[]) => (
              <Space size={[4, 4]} wrap>
                {(events || []).map((e) => (
                  <Tag key={e}>{e}</Tag>
                ))}
              </Space>
            ),
          },
          {
            title: ui.enabled,
            dataIndex: 'enabled',
            width: 90,
            render: (on: boolean, row: WebhookEndpoint) => (
              <Switch
                checked={on}
                onChange={(enabled) => {
                  api.updateWebhook(row.id, { enabled }).then(
                    () => void qc.invalidateQueries({ queryKey: ['webhooks'] }),
                    (err: Error) => message.error(err.message),
                  )
                }}
              />
            ),
          },
          {
            title: '',
            key: 'actions',
            width: 260,
            render: (_: unknown, row: WebhookEndpoint) => (
              <Space wrap>
                <Button size="small" icon={<ExperimentOutlined />} loading={testMut.isPending} onClick={() => testMut.mutate(row.id)}>
                  {ui.test}
                </Button>
                <Button size="small" onClick={() => setDeliveriesFor(row)}>
                  {ui.deliveries}
                </Button>
                <Button
                  size="small"
                  onClick={() => {
                    setEditing(row)
                    form.setFieldsValue(row)
                    setOpen(true)
                  }}
                >
                  {ui.edit}
                </Button>
                <Button
                  size="small"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={() =>
                    modal.confirm({
                      title: ui.delConfirm,
                      onOk: () => delMut.mutateAsync(row.id),
                    })
                  }
                />
              </Space>
            ),
          },
        ]}
      />

      <Card size="small" title={ui.catalog}>
        <Table
          size="small"
          rowKey="name"
          pagination={false}
          dataSource={eventsQuery.data?.events || []}
          columns={[
            {
              title: ui.events,
              dataIndex: 'name',
              render: (v: string) => (
                <Typography.Text copyable={{ text: v }} code dir="ltr">
                  {v}
                </Typography.Text>
              ),
            },
            { title: lang === 'he' ? 'מה קורה' : 'What it means', dataIndex: 'description' },
          ]}
        />
      </Card>

      <Drawer
        title={editing ? ui.editHook : ui.newHook}
        open={open}
        onClose={() => setOpen(false)}
        width={480}
        extra={
          <Button type="primary" loading={saveMut.isPending} onClick={() => form.submit()}>
            {ui.save}
          </Button>
        }
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMut.mutate(values)}>
          <Form.Item name="url" label={ui.hookUrl} rules={[{ required: true }]}>
            <Input placeholder="https://example.com/webhooks/school-mdm" dir="ltr" />
          </Form.Item>
          <Form.Item name="description" label={ui.hookDesc}>
            <Input />
          </Form.Item>
          <Form.Item name="events" label={ui.events}>
            <Select mode="tags" options={options} placeholder="*  mdm.*  requests.request_approve" />
          </Form.Item>
          <Form.Item name="secret" label={ui.hookSecret} extra={ui.hookSecretHint}>
            <Input.Password placeholder={editing ? '…' : ''} dir="ltr" />
          </Form.Item>
          <Form.Item name="enabled" label={ui.enabled} valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Drawer>

      <Drawer title={`${ui.deliveries} — ${deliveriesFor?.url || ''}`} open={!!deliveriesFor} onClose={() => setDeliveriesFor(null)} width={640}>
        <Table
          size="small"
          rowKey="id"
          loading={deliveriesQuery.isLoading}
          dataSource={deliveriesQuery.data?.deliveries || []}
          pagination={false}
          columns={[
            {
              title: lang === 'he' ? 'מתי' : 'When',
              dataIndex: 'created_at',
              render: (v: string) => (v ? new Date(v).toLocaleString() : ''),
            },
            { title: lang === 'he' ? 'אירוע' : 'Event', dataIndex: 'event_name' },
            {
              title: lang === 'he' ? 'סטטוס' : 'Status',
              dataIndex: 'status',
              render: (s: string, row: WebhookDelivery) => (
                <Tag color={s === 'success' ? 'green' : 'red'}>
                  {s}
                  {row.http_status ? ` ${row.http_status}` : ''}
                </Tag>
              ),
            },
            { title: lang === 'he' ? 'ניסיון' : 'Try', dataIndex: 'attempt', width: 80 },
            { title: lang === 'he' ? 'שגיאה' : 'Error', dataIndex: 'error' },
          ]}
        />
      </Drawer>
    </Space>
  )
}

function ApiDocsInner() {
  const specQuery = useQuery({ queryKey: ['openapi'], queryFn: () => api.openapi() })
  const eventsQuery = useQuery({ queryKey: ['webhook-events'], queryFn: () => api.webhookEvents() })
  const [tokenDraft, setTokenDraft] = useState(() => getAdminToken())
  const [lang, setLang] = useState<Lang>('he')
  const { message } = App.useApp()
  const ui = t[lang]
  const origin = typeof window !== 'undefined' ? window.location.origin : ''

  function copyAi() {
    const text = buildAiInstructions({
      origin,
      spec: specQuery.data,
      events: eventsQuery.data?.events,
    })
    void navigator.clipboard.writeText(text).then(
      () => message.success(ui.copiedAi),
      () => message.error(ui.copyFail),
    )
  }

  return (
    <ConfigProvider direction={lang === 'he' ? 'rtl' : 'ltr'} locale={lang === 'he' ? heIL : enUS} theme={theme}>
      <div className={`api-docs-page ${lang === 'he' ? 'is-rtl' : 'is-ltr'}`}>
        <header className="api-docs-hero">
          <div className="api-docs-hero-text">
            <Flex align="center" gap={10} wrap="wrap">
              <ApiOutlined className="api-docs-hero-icon" />
              <Typography.Title level={2} className="api-docs-hero-title">
                {ui.brand} · {ui.title}
              </Typography.Title>
            </Flex>
            <Typography.Paragraph className="api-docs-hero-lead">{ui.lead}</Typography.Paragraph>
          </div>
          <div className="api-docs-hero-actions">
            <Segmented
              value={lang}
              onChange={(v) => setLang(v as Lang)}
              options={[
                { label: 'עברית', value: 'he' },
                { label: 'English', value: 'en' },
              ]}
            />
            <Button type="primary" size="large" icon={<RobotOutlined />} onClick={copyAi}>
              {ui.copyAi}
            </Button>
          </div>
        </header>

        <Card className="api-docs-toolbar" size="small">
          <Flex gap={8} wrap="wrap" align="center">
            <Input.Password
              style={{ width: 260 }}
              value={tokenDraft}
              onChange={(e) => setTokenDraft(e.target.value)}
              placeholder={ui.token}
            />
            <Button
              onClick={() => {
                setAdminToken(tokenDraft.trim())
                message.success(ui.tokenSaved)
              }}
            >
              {ui.saveToken}
            </Button>
            <Link to="/admin">
              <Button>{ui.admin}</Button>
            </Link>
          </Flex>
        </Card>

        <Tabs
          className="api-docs-tabs"
          defaultActiveKey="start"
          items={[
            {
              key: 'start',
              label: ui.tabStart,
              children: (
                <StartTab lang={lang} origin={origin} spec={specQuery.data} onCopyAi={copyAi} />
              ),
            },
            {
              key: 'reference',
              label: ui.tabRef,
              children: specQuery.isError ? (
                <Alert type="error" message="/api/openapi.json" />
              ) : (
                <ReferenceTab spec={specQuery.data} lang={lang} origin={origin} />
              ),
            },
            { key: 'webhooks', label: ui.tabHooks, children: <WebhooksTab lang={lang} /> },
          ]}
        />
      </div>
    </ConfigProvider>
  )
}

export default function ApiDocs() {
  return (
    <App>
      <ApiDocsInner />
    </App>
  )
}
