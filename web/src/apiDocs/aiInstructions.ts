import type { OpenAPISpec, WebhookEventInfo } from '../api'

type Op = { method: string; path: string; summary: string; tag: string; admin: boolean }

function isAdminOp(security: unknown): boolean {
  if (!Array.isArray(security) || security.length === 0) return false
  return security.some((item) => item && typeof item === 'object' && 'bearerAuth' in item)
}

export function listOperations(spec: OpenAPISpec | undefined): Op[] {
  if (!spec?.paths) return []
  const out: Op[] = []
  for (const [path, item] of Object.entries(spec.paths)) {
    for (const [method, op] of Object.entries(item)) {
      if (!op || typeof op !== 'object') continue
      out.push({
        method: method.toLowerCase(),
        path,
        summary: op.summary || op.operationId || '',
        tag: op.tags?.[0] || 'Other',
        admin: isAdminOp(op.security),
      })
    }
  }
  return out
}

/** Markdown an AI (or a developer) can paste to use this School MDM API correctly. */
export function buildAiInstructions(opts: {
  origin: string
  spec?: OpenAPISpec
  events?: WebhookEventInfo[]
}): string {
  const origin = opts.origin.replace(/\/$/, '')
  const ops = listOperations(opts.spec)
  const byTag = new Map<string, Op[]>()
  for (const op of ops) {
    const list = byTag.get(op.tag) || []
    list.push(op)
    byTag.set(op.tag, list)
  }
  const tags = opts.spec?.tags?.map((t) => t.name) || [...byTag.keys()]
  const catalog = (opts.events || []).map((e) => `- \`${e.name}\` — ${e.description}`).join('\n')

  const endpointLines = tags
    .map((tag) => {
      const rows = byTag.get(tag) || []
      if (!rows.length) return ''
      const lines = rows
        .map((r) => `- \`${r.method.toUpperCase()} ${r.path}\`${r.admin ? ' (admin Bearer)' : ''} — ${r.summary}`)
        .join('\n')
      return `### ${tag}\n${lines}`
    })
    .filter(Boolean)
    .join('\n\n')

  return `# School MDM API — instructions for an AI assistant

You are working against a live School MDM server (school iOS MDM: devices, groups, allowlists, student requests, credits, Apple MDM, ABM/ADE, activity, webhooks).

Do not invent URLs. Use only the endpoints below (or refresh from OpenAPI). Do not print or log the admin token.

## Hebrew overview (for the operator)

זהו ממשק REST של ניהול בית הספר. כל פעולה שקיימת במסך הניהול זמינה גם ב־API.
אימות: התחברות Google בממשק, או אסימון מ־\`POST /api/admin/tokens\` בכותרת Bearer לסקריפטים.
מסמכים אינטראקטיביים: ${origin}/api-docs
מפרט OpenAPI: ${origin}/api/openapi.json
וובהוקים: כל אירוע פעילות נשלח ב־POST JSON עם חתימת HMAC.

## Base URL

${origin}

## Auth

- Header: \`Authorization: Bearer <token>\` from \`POST /api/admin/tokens\` **or** a Google admin session cookie
- Env \`ADMIN_TOKENS\` still works for local/scripts. Do not paste those into the browser.
- Or HTTP Basic with the token as the password
- Student portal routes are unauthenticated and scoped by enrollment id. Admin panel / MDM / activity / webhooks / purchases require auth.

## Discovery

- Human docs: ${origin}/api-docs
- OpenAPI 3.1: \`GET ${origin}/api/openapi.json\`
- Webhook event catalog: \`GET ${origin}/api/webhooks/events\`

## Conventions

- JSON request/response.
- Path params like \`{id}\` are enrollment IDs unless noted (packs/groups/requests have their own UUIDs).
- MDM command POSTs usually return \`202 { "status": "queued" }\` and sometimes \`command_uuid\` to poll.
- Poll command results: \`GET /api/mdm/devices/{id}/commands/{commandUUID}\`
- Prefer idempotent reads before destructive MDM actions (lock/erase/lost mode).
- After allowlist/group/pack/custom-profile changes the server reconciles policy to devices.

## Quick examples

List school devices:
\`\`\`bash
curl -sS -H "Authorization: Bearer $TOKEN" ${origin}/api/devices
\`\`\`

Lock a supervised device:
\`\`\`bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \\
  -d '{"pin":"123456","message":"נא לפנות לניהול"}' \\
  ${origin}/api/mdm/devices/ENROLLMENT_ID/lock
\`\`\`

Approve a student request:
\`\`\`bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \\
  -d '{}' ${origin}/api/requests/REQUEST_ID/approve
\`\`\`

Create a webhook (HTTPS, or http://127.0.0.1 for local tests):
\`\`\`bash
curl -sS -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \\
  -d '{"url":"https://example.com/hooks/school-mdm","events":["*"],"description":"automation"}' \\
  ${origin}/api/webhooks
\`\`\`

## Webhooks

Every activity event is POSTed to matching endpoints.

Body:
\`\`\`json
{
  "event": "requests.request_approve",
  "id": "event-uuid",
  "created_at": "2026-08-18T00:00:00Z",
  "data": {
    "id": "event-uuid",
    "category": "requests",
    "action": "request_approve",
    "actor_type": "admin",
    "result": "ok",
    "summary": "…",
    "enrollment_id": "optional",
    "detail": {}
  }
}
\`\`\`

Headers:
- \`X-SchoolMDM-Event\`: dotted name (\`category.action\`)
- \`X-SchoolMDM-Delivery\`: delivery id
- \`X-SchoolMDM-Event-Id\`: activity event id
- \`X-SchoolMDM-Signature\`: \`sha256=<hex>\` HMAC-SHA256 of the **raw body** with the endpoint secret
- \`User-Agent\`: SchoolMDM-Webhooks/1.0

Subscribe with \`*\`, \`mdm.*\`, or an exact name. Failed deliveries retry 3 times. Respond 2xx. HTTPS required except localhost.

### Event catalog
${catalog || '(fetch GET /api/webhooks/events)'}

## Full endpoint list

${endpointLines || '(fetch OpenAPI)'}
`
}
