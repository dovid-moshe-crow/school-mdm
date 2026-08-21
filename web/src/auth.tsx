import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Alert, Button, Card, Flex, Input, Space, Spin, Typography } from 'antd'
import { useLocation, useNavigate } from 'react-router-dom'
import { api, getAdminToken, setAdminToken, type AdminAuthUser } from './api'
import { he } from './he'

type AdminAuth = {
  loading: boolean
  authed: boolean
  user: AdminAuthUser | null
  google: boolean
  tokenLogin: boolean
  logout: () => Promise<void>
  saveToken: (token: string) => void
}

const Ctx = createContext<AdminAuth>({
  loading: true,
  authed: false,
  user: null,
  google: false,
  tokenLogin: true,
  logout: async () => {},
  saveToken: () => {},
})

export function useAdminAuth() {
  return useContext(Ctx)
}

export function AdminAuthProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient()
  const token = getAdminToken()
  const meQuery = useQuery({
    queryKey: ['auth-me', token],
    queryFn: () => api.authMe(),
    retry: false,
    staleTime: 30_000,
  })
  const cfgQuery = useQuery({
    queryKey: ['auth-config'],
    queryFn: () => api.authConfig(),
    staleTime: 60_000,
  })
  const user = meQuery.data ?? null
  const google = !!cfgQuery.data?.google
  const authed = google ? user?.method === 'google' : !!user
  const loading = meQuery.isLoading || cfgQuery.isLoading

  useEffect(() => {
    if (!google) return
    setAdminToken('')
    void qc.invalidateQueries({ queryKey: ['auth-me'] })
  }, [google, qc])

  async function logout() {
    setAdminToken('')
    try {
      await api.authLogout()
    } catch {
      /* ignore */
    }
    await qc.invalidateQueries({ queryKey: ['auth-me'] })
  }

  function saveToken(next: string) {
    setAdminToken(next.trim())
    void qc.invalidateQueries({ queryKey: ['auth-me'] })
  }

  return (
    <Ctx.Provider
      value={{
        loading,
        authed,
        user,
        google,
        tokenLogin: !google && cfgQuery.data?.token_login !== false,
        logout,
        saveToken,
      }}
    >
      {children}
    </Ctx.Provider>
  )
}

export function RequireAdmin({ children }: { children: ReactNode }) {
  const auth = useAdminAuth()
  if (auth.loading) {
    return (
      <div className="page-shell">
        <Flex justify="center" style={{ padding: 48 }}>
          <Spin />
        </Flex>
      </div>
    )
  }
  if (!auth.authed) return <AdminLoginPage />
  return children
}

function AdminLoginPage() {
  const auth = useAdminAuth()
  const loc = useLocation()
  const navigate = useNavigate()
  const [googleBusy, setGoogleBusy] = useState(false)
  const params = new URLSearchParams(loc.search)
  const err = params.get('error')
  const next = loc.pathname.startsWith('/admin') || loc.pathname.startsWith('/api-docs')
    ? loc.pathname + loc.search.replace(/[?&]error=[^&]*/g, '')
    : '/admin'

  function googleStart() {
    setGoogleBusy(true)
    window.location.href = `/api/auth/google/start?next=${encodeURIComponent(next || '/admin')}`
  }

  return (
    <div className="page-shell">
      <Card>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div>
            <Typography.Title level={3} style={{ marginBottom: 8 }}>
              {he.adminLoginTitle}
            </Typography.Title>
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              {he.adminLoginLead}
            </Typography.Paragraph>
          </div>
          {err ? (
            <Alert type="error" showIcon message={loginErrorText(err)} />
          ) : null}
          {auth.google ? (
            <Button type="primary" size="large" block loading={googleBusy} onClick={googleStart}>
              {he.adminLoginGoogle}
            </Button>
          ) : (
            <TokenLoginForm
              onSave={(token) => {
                auth.saveToken(token)
                navigate(next || '/admin', { replace: true })
              }}
            />
          )}
        </Space>
      </Card>
    </div>
  )
}

function TokenLoginForm({ onSave }: { onSave: (token: string) => void }) {
  const [token, setToken] = useState('')
  const [busy, setBusy] = useState(false)
  function save() {
    if (!token.trim() || busy) return
    setBusy(true)
    onSave(token)
  }
  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      <Typography.Text type="secondary">{he.adminLoginTokenHint}</Typography.Text>
      <Input.Password
        value={token}
        placeholder="dev-admin-token"
        onChange={(e) => setToken(e.target.value)}
        onPressEnter={save}
      />
      <Button type="primary" block disabled={!token.trim()} loading={busy} onClick={save}>
        {he.adminLoginToken}
      </Button>
    </Space>
  )
}

function loginErrorText(code: string) {
  switch (code) {
    case 'not-allowed':
      return he.adminLoginNotAllowed
    case 'unverified-email':
      return he.adminLoginUnverified
    default:
      return he.adminLoginFailed
  }
}
