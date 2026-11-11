import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { RequireRole, SessionProvider, useSession } from '../auth/session'
import { NotificationInbox } from './NotificationInbox'

type Notice = { id: string; type: string; dealId: string; productId: string; createdAt: string; readAt: string | null }

// 서버 계약(GET /me/notifications, POST /me/notifications/{id}/read)을 흉내 내는 test-only 가짜 서버.
let notices: Notice[]
let mode: 'ok' | 'offline' | 'expired' | 'readFails' | 'readMissing'
let calls: { method: string; path: string; csrf: string | null }[]

function notice(n: number, readAt: string | null = null): Notice {
  return { id: `n${n}`, type: 'APPLICATION_REJECTED', dealId: `deal-${n}`, productId: `p${n}`, createdAt: `2026-11-0${n}T03:00:00.000Z`, readAt }
}
const metaNow = () => ({ hasUnreadMessages: false, hasUnreadNotifications: notices.some(n => n.readAt === null) })
const user = { id: 'dev-employee-b', realName: '테스트 직원 B', department: 'TEST_DEPT', jobLevel: 'TEST_LEVEL', role: 'MEMBER', status: 'ACTIVE' }

beforeEach(() => {
  notices = [notice(3), notice(2, '2026-11-02T05:00:00.000Z'), notice(1)]
  mode = 'ok'
  calls = []
  vi.stubGlobal('fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input), 'http://localhost')
    const method = init?.method ?? 'GET'
    calls.push({ method, path: url.pathname + url.search, csrf: new Headers(init?.headers).get('X-CSRF-Token') })
    if (url.pathname === '/api/v1/auth/config') return Response.json({ data: { authMode: 'development', developmentLogin: true, accounts: [] } })
    if (url.pathname === '/api/v1/auth/me') return Response.json({ data: { user, csrfToken: 'csrf' }, meta: metaNow() })
    if (mode === 'offline') throw new TypeError('Failed to fetch')
    if (mode === 'expired') return Response.json({ error: { code: 'SESSION_EXPIRED', message: '로그인이 만료되었습니다.' } }, { status: 401 })
    if (url.pathname === '/api/v1/me/notifications') {
      const limit = Number(url.searchParams.get('limit') ?? 20)
      const start = url.searchParams.get('cursor') ? Number(url.searchParams.get('cursor')) : 0
      const items = notices.slice(start, start + limit)
      const nextCursor = start + limit < notices.length ? String(start + limit) : null
      return Response.json({ data: { items, nextCursor }, meta: metaNow() })
    }
    const read = url.pathname.match(/^\/api\/v1\/me\/notifications\/([^/]+)\/read$/)
    if (read && method === 'POST') {
      if (mode === 'readFails') return Response.json({ error: { code: 'INTERNAL_ERROR', message: '처리 중 오류가 발생했습니다.' } }, { status: 500 })
      const target = notices.find(n => n.id === decodeURIComponent(read[1]))
      if (!target || mode === 'readMissing') return Response.json({ error: { code: 'NOT_FOUND', message: '알림을 찾을 수 없습니다.' } }, { status: 404, })
      target.readAt ??= '2026-11-11T09:00:00.000Z'
      return Response.json({ data: target, meta: metaNow() })
    }
    return Response.json({ error: { code: 'NOT_FOUND' } }, { status: 404 })
  })
})
afterEach(() => { cleanup(); vi.unstubAllGlobals() })

function UnreadProbe() {
  const { meta } = useSession()
  return <p data-testid="probe">{meta.hasUnreadNotifications ? '새 알림 있음' : '새 알림 없음'}</p>
}
function renderInbox(pageSize?: number) {
  const navigate = vi.fn()
  render(<SessionProvider><UnreadProbe /><RequireRole><NotificationInbox navigate={navigate} pageSize={pageSize} /></RequireRole></SessionProvider>)
  return navigate
}
const items = () => screen.getAllByRole('listitem')

test('최신 알림부터 거절 안내를 보여 주고 목록만 열어서는 읽음 처리하지 않는다', async () => {
  renderInbox()
  await screen.findByRole('heading', { name: '알림함' })
  await waitFor(() => expect(items()).toHaveLength(3))
  expect(items()[0].textContent).toContain('판매자가 거래 신청을 거절했어요')
  expect(items()[0].textContent).toContain('11월 3일')
  expect(within(items()[0]).getByText('안 읽음')).toBeTruthy()
  expect(within(items()[1]).queryByText('안 읽음')).toBeNull()
  expect(screen.getByText(/상품이 숨겨져도/)).toBeTruthy()
  expect(calls.some(c => c.method === 'POST')).toBe(false)
  expect(screen.getByTestId('probe').textContent).toBe('새 알림 있음')
})

test('안 읽은 알림을 누르면 읽음 처리 후 해당 거래로 이동하고 미확인 표시가 갱신된다', async () => {
  notices = [notice(3)]
  const navigate = renderInbox()
  fireEvent.click(await screen.findByRole('link', { name: /판매자가 거래 신청을 거절했어요/ }))
  await waitFor(() => expect(navigate).toHaveBeenCalledWith('/market/deals/deal-3'))
  const read = calls.find(c => c.method === 'POST')
  expect(read).toMatchObject({ path: '/api/v1/me/notifications/n3/read', csrf: 'csrf' })
  expect(screen.getByTestId('probe').textContent).toBe('새 알림 없음')
  expect(within(items()[0]).queryByText('안 읽음')).toBeNull()
})

test('이미 읽은 알림은 다시 읽음 요청 없이 거래로 이동한다', async () => {
  const navigate = renderInbox()
  await waitFor(() => expect(items()).toHaveLength(3))
  fireEvent.click(within(items()[1]).getByRole('link'))
  expect(navigate).toHaveBeenCalledWith('/market/deals/deal-2')
  expect(calls.some(c => c.method === 'POST')).toBe(false)
})

test('더 보기로 다음 페이지를 이어 붙이고 마지막 페이지에서는 버튼을 숨긴다', async () => {
  renderInbox(2)
  await waitFor(() => expect(items()).toHaveLength(2))
  expect(calls.find(c => c.path.startsWith('/api/v1/me/notifications'))?.path).toBe('/api/v1/me/notifications?limit=2')
  fireEvent.click(screen.getByRole('button', { name: '더 보기' }))
  await waitFor(() => expect(items()).toHaveLength(3))
  expect(calls.some(c => c.path === '/api/v1/me/notifications?limit=2&cursor=2')).toBe(true)
  expect(screen.queryByRole('button', { name: '더 보기' })).toBeNull()
})

test('받은 알림이 없으면 빈 목록 안내를 보여 준다', async () => {
  notices = []
  renderInbox()
  expect(await screen.findByText('아직 받은 알림이 없어요.')).toBeTruthy()
  expect(screen.queryByRole('list')).toBeNull()
})

test('통신이 실패하면 안내하고 다시 시도할 수 있다', async () => {
  mode = 'offline'
  renderInbox()
  expect((await screen.findByRole('alert')).textContent).toContain('알림을 불러오지 못했어요')
  mode = 'ok'
  fireEvent.click(screen.getByRole('button', { name: '다시 시도' }))
  await waitFor(() => expect(items()).toHaveLength(3))
  expect(screen.queryByRole('alert')).toBeNull()
})

test('세션이 만료되면 로그인 안내로 바뀐다', async () => {
  mode = 'expired'
  renderInbox()
  expect(await screen.findByText('로그인이 필요합니다.')).toBeTruthy()
  expect(screen.queryByRole('heading', { name: '알림함' })).toBeNull()
})

test('읽음 처리에 실패해도 거래로 이동하고 알림은 안 읽음으로 남는다', async () => {
  notices = [notice(3)]
  const navigate = renderInbox()
  await waitFor(() => expect(items()).toHaveLength(1))
  mode = 'readFails'
  fireEvent.click(within(items()[0]).getByRole('link'))
  await waitFor(() => expect(navigate).toHaveBeenCalledWith('/market/deals/deal-3'))
  expect(within(items()[0]).getByText('안 읽음')).toBeTruthy()
})

test('사라진 알림은 이동하지 않고 새로고침을 안내한다', async () => {
  notices = [notice(3)]
  const navigate = renderInbox()
  await waitFor(() => expect(items()).toHaveLength(1))
  mode = 'readMissing'
  fireEvent.click(within(items()[0]).getByRole('link'))
  expect((await screen.findByRole('alert')).textContent).toContain('이 알림을 찾을 수 없어요')
  expect(navigate).not.toHaveBeenCalled()
})
