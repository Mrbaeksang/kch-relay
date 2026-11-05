import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { SessionProvider, useSession, RequireRole } from './session'

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

test('선택 로그인·관리자 차단·로그아웃을 공통 상태로 처리한다', async () => {
  let loggedIn = false
  const transport = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input)
    if (path.endsWith('/auth/config')) return Response.json({ data: { authMode: 'development', developmentLogin: true, accounts: [] } })
    if (path.endsWith('/auth/logout')) {
      expect(new Headers(init?.headers).get('X-CSRF-Token')).toBe('csrf')
      loggedIn = false; return Response.json({ data: { loggedOut: true } })
    }
    if (path.endsWith('/auth/development-login')) loggedIn = true
    if (!loggedIn) return Response.json({ error: { code: 'UNAUTHENTICATED', message: '로그인 필요' } }, { status: 401 })
    return Response.json({ data: { user: { id: 'dev-employee-a', realName: '테스트 직원 A', department: 'TEST_DEPT', jobLevel: 'TEST_LEVEL', role: 'MEMBER', status: 'ACTIVE' }, csrfToken: 'csrf' }, meta: { hasUnreadMessages: true, hasUnreadNotifications: false } })
  })
  vi.stubGlobal('fetch', transport)
  function Screen() {
    const s = useSession()
    return <><p>{s.state.status === 'authenticated' ? s.state.user.realName : s.state.status}</p><button onClick={() => void s.login('dev-employee-a')}>선택 로그인</button><button onClick={() => void s.logout()}>로그아웃</button><RequireRole role="HR_ADMIN"><p>관리자 영역</p></RequireRole><span>{s.meta.hasUnreadMessages ? '새 쪽지' : '없음'}</span></>
  }
  render(<SessionProvider><Screen /></SessionProvider>)
  await screen.findByText('anonymous')
  fireEvent.click(screen.getByText('선택 로그인'))
  await screen.findByText('테스트 직원 A')
  expect(screen.queryByText('관리자 영역')).toBeNull()
  expect(screen.getByText('관리자만 이용할 수 있습니다.')).toBeTruthy()
  expect(screen.getByText('새 쪽지')).toBeTruthy()
  fireEvent.click(screen.getByText('로그아웃'))
  await screen.findByText('anonymous')
  await waitFor(() => expect(screen.getByText('없음')).toBeTruthy())
  expect(transport.mock.calls.filter(([url]) => String(url).endsWith('/auth/me'))).toHaveLength(1)
})
