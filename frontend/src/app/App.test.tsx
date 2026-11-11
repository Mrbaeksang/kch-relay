import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { App } from './App'

beforeEach(() => {
  vi.stubGlobal('fetch', async (input: RequestInfo | URL) =>
    String(input).endsWith('/auth/config')
      ? Response.json({
          data: { authMode: 'disabled', developmentLogin: false, accounts: [] },
        })
      : Response.json({ error: { code: 'UNAUTHENTICATED' } }, { status: 401 }),
  )
})
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})
test('로그인 전에는 상품과 신원을 노출하지 않는다', async () => {
  render(<App />)
  expect(
    (await screen.findByRole('heading', { level: 1 })).textContent,
  ).toContain('다음 쓰임')
  expect(
    screen.getByText(
      '개발 로그인이 비활성화되어 있습니다. 실제 구글 로그인은 준비 중입니다.',
    ),
  ).toBeTruthy()
  expect(screen.queryByRole('button', { name: /로그인|신청/ })).toBeNull()
})

test('장터 조회 중 세션이 만료되면 내용을 감추고 재로그인을 안내한다', async () => {
  vi.stubGlobal('fetch', async (input: RequestInfo | URL) => {
    const path = String(input)
    if (path.endsWith('/auth/config'))
      return Response.json({
        data: {
          authMode: 'development',
          developmentLogin: true,
          accounts: [
            { key: 'dev-employee-b', label: '테스트 직원 B', role: 'MEMBER' },
          ],
        },
      })
    if (path.endsWith('/auth/development-login'))
      return Response.json({
        data: {
          user: {
            id: 'dev-employee-b',
            realName: '테스트 직원 B',
            role: 'MEMBER',
            status: 'ACTIVE',
          },
          csrfToken: 'csrf',
        },
      })
    return Response.json(
      { error: { code: 'UNAUTHENTICATED', message: '로그인이 필요합니다.' } },
      { status: 401 },
    )
  })
  render(<App />)
  const login = await screen.findByRole('button', {
    name: '테스트 직원 B로 로그인',
  })
  const { fireEvent } = await import('@testing-library/react')
  fireEvent.click(login)
  await screen.findByText('로그인이 만료되었습니다. 다시 로그인해 주세요.')
  expect(screen.queryByRole('searchbox')).toBeNull()
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
