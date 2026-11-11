import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { App } from './App'

beforeEach(() => { vi.stubGlobal('fetch', async (input: RequestInfo | URL) => String(input).endsWith('/auth/config') ? Response.json({ data: { authMode: 'disabled', developmentLogin: false, accounts: [] } }) : Response.json({ error: { code: 'UNAUTHENTICATED' } }, { status: 401 })) })
afterEach(() => { cleanup(); vi.unstubAllGlobals() })
test('개발 준비 화면은 거래 기능을 제공한다고 오해시키지 않는다', () => {
  render(<App />)
  expect(screen.getByRole('heading', { level: 1 }).textContent).toContain('다음 쓰임')
  expect(screen.getByRole('status').textContent).toContain('아직 상품 조회·가입·거래 기능은 제공하지 않습니다')
  expect(screen.queryByRole('button', { name: /로그인|신청/ })).toBeNull()
})

test('로그인한 직원은 메뉴에서 알림함으로 가고 새 알림 표시를 본다', async () => {
  vi.stubGlobal('fetch', async (input: RequestInfo | URL) => {
    const path = String(input)
    if (path.endsWith('/auth/config')) return Response.json({ data: { authMode: 'development', developmentLogin: true, accounts: [] } })
    if (path.endsWith('/auth/me')) return Response.json({ data: { user: { id: 'dev-employee-b', realName: '테스트 직원 B', department: 'TEST_DEPT', jobLevel: 'TEST_LEVEL', role: 'MEMBER', status: 'ACTIVE' }, csrfToken: 'csrf' }, meta: { hasUnreadMessages: false, hasUnreadNotifications: true } })
    return Response.json({ error: { code: 'NOT_FOUND' } }, { status: 404 })
  })
  render(<App />)
  const link = await screen.findByRole('link', { name: /알림함/ })
  expect(link.getAttribute('href')).toBe('/notifications')
  expect(screen.getByLabelText('새 알림 있음')).toBeTruthy()
})
