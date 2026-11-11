import { createContext, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { APIError, createAPI, type API, type Meta } from '../lib/api'

export type User = { id: string; realName: string; department: string; jobLevel: string; role: 'MEMBER' | 'HR_ADMIN'; status: 'ACTIVE' | 'SUSPENDED' }
type SessionData = { user: User; csrfToken: string }
type AuthState = { status: 'loading' } | { status: 'anonymous' } | ({ status: 'authenticated' } & SessionData)
type AuthConfig = { authMode: string; developmentLogin: boolean; accounts: { key: string; label: string; role: string }[] }
type SessionValue = { state: AuthState; config: AuthConfig | null; meta: Meta; error: string | null; busy: boolean; api: API; login: (key: string) => Promise<void>; logout: () => Promise<void> }
const Context = createContext<SessionValue | null>(null)
const noUnread: Meta = { hasUnreadMessages: false, hasUnreadNotifications: false }

export function SessionProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'loading' })
  const [config, setConfig] = useState<AuthConfig | null>(null)
  const [meta, setMeta] = useState<Meta>(noUnread)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const csrf = useRef<string | null>(null)
  const api = useMemo(() => createAPI({
    getCSRF: () => csrf.current,
    onMeta: setMeta,
    onUnauthorized: () => {
      const hadSession = csrf.current !== null
      csrf.current = null; setState({ status: 'anonymous' }); setMeta(noUnread)
      if (hadSession) setError('로그인이 만료되었습니다. 다시 로그인해 주세요.')
    },
  }), [])
  function accept(data: SessionData) { csrf.current = data.csrfToken; setState({ status: 'authenticated', ...data }) }
  function describe(error: unknown) { return error instanceof APIError ? error.message : '연결을 확인하고 다시 시도해 주세요.' }
  useEffect(() => {
    const controller = new AbortController()
    void Promise.all([
      api<AuthConfig>('/auth/config', { signal: controller.signal }),
      api<SessionData>('/auth/me', { signal: controller.signal }).catch((error: unknown) => { if (error instanceof APIError && (error.status === 401 || error.status === 403)) { if (error.status === 403) setError(error.message); return null }; throw error }),
    ]).then(([configuration, session]) => {
      if (controller.signal.aborted) return
      setConfig(configuration)
      if (session) accept(session); else setState({ status: 'anonymous' })
    }).catch((error: unknown) => {
      if (!controller.signal.aborted) { setError(describe(error)); setState({ status: 'anonymous' }) }
    })
    return () => controller.abort()
  }, [api])
  async function login(key: string) {
    setBusy(true); setError(null)
    try { accept(await api<SessionData>('/auth/development-login', { method: 'POST', body: { accountKey: key } })) }
    catch (error) { setError(describe(error)) }
    finally { setBusy(false) }
  }
  async function logout() {
    setBusy(true); setError(null)
    try { await api('/auth/logout', { method: 'POST', body: {} }); csrf.current = null; setState({ status: 'anonymous' }); setMeta(noUnread) }
    catch (error) { if (!(error instanceof APIError && error.status === 401)) setError(describe(error)) }
    finally { setBusy(false) }
  }
  return <Context.Provider value={{ state, config, meta, error, busy, api, login, logout }}>{children}</Context.Provider>
}
export function useSession() { const session = useContext(Context); if (!session) throw new Error('SessionProvider가 필요합니다.'); return session }
export function RequireRole({ role, children }: { role?: User['role']; children: ReactNode }) {
  const { state } = useSession()
  if (state.status === 'loading') return <p>로그인 상태를 확인하고 있습니다.</p>
  if (state.status === 'anonymous') return <p role="alert">로그인이 필요합니다.</p>
  if (role && state.user.role !== role) return <p role="alert">관리자만 이용할 수 있습니다.</p>
  return <>{children}</>
}
