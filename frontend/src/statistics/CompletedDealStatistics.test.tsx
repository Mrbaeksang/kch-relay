import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { App } from '../app/App'

const meta = { hasUnreadMessages: false, hasUnreadNotifications: false }
type Reply = (url: URL) => Response | Promise<Response>

// serve answers the session APIs as the given role and hands statistics requests to the replies in order.
function serve(role: 'HR_ADMIN' | 'MEMBER', ...replies: Reply[]) {
  const calls: URL[] = []
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
    const url = new URL(String(input), 'http://localhost')
    if (url.pathname === '/api/v1/auth/config') return Response.json({ data: { authMode: 'development', developmentLogin: true, accounts: [] } })
    if (url.pathname === '/api/v1/auth/me') return Response.json({ data: { user: { id: 'u', realName: '테스트 사용자', department: 'TEST_DEPT', jobLevel: 'TEST_LEVEL', role, status: 'ACTIVE' }, csrfToken: 'csrf' }, meta })
    if (url.pathname === '/api/v1/admin/statistics/completed-deals') {
      calls.push(url)
      const reply = replies.shift()
      if (!reply) throw new Error('예상하지 못한 통계 요청')
      return reply(url)
    }
    return Response.json({ error: { code: 'NOT_FOUND', message: '없음' } }, { status: 404 })
  }))
  return calls
}
const totals = (count: number, amount: number): Reply => url => Response.json({ data: { from: url.searchParams.get('from'), to: url.searchParams.get('to'), timezone: 'Asia/Seoul', currency: 'KRW', completedDealCount: count, totalCompletedAmount: amount }, meta })
const failure = (status: number, code: string, message: string): Reply => () => Response.json({ error: { code, message, requestId: 'r' } }, { status })
const period = (url: URL) => `${url.searchParams.get('from')}~${url.searchParams.get('to')}`
function choose(from: string, to: string) {
  fireEvent.change(screen.getByLabelText('시작일'), { target: { value: from } })
  fireEvent.change(screen.getByLabelText('종료일'), { target: { value: to } })
  fireEvent.click(screen.getByRole('button', { name: '조회' }))
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-11-30T15:30:00Z')) // 한국 시간 12월 1일 00:30
  window.history.pushState({}, '', '/admin/statistics')
})
afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.useRealTimers(); window.history.pushState({}, '', '/') })

test('관리자는 화면을 열면 한국 날짜 기준 이번 달 완료 거래 건수와 금액을 본다', async () => {
  let release!: () => void
  const calls = serve('HR_ADMIN', url => new Promise(resolve => { release = () => resolve(totals(3, 51000)(url) as Response) }))
  render(<App />)
  expect(await screen.findByText('불러오는 중입니다.')).toBeTruthy()
  release()
  expect(await screen.findByText('51,000원')).toBeTruthy()
  expect(screen.getByText('3건')).toBeTruthy()
  expect(period(calls[0])).toBe('2026-12-01~2026-12-01')
  expect(screen.getByText(/회사 매출이나 실제 송금·정산 금액이 아닙니다/)).toBeTruthy()
})

test('빈 기간은 0원·0건으로 안내하고 고른 기간으로 다시 조회한다', async () => {
  const calls = serve('HR_ADMIN', totals(0, 0), totals(2, 1314197523))
  render(<App />)
  expect(await screen.findByText('이 기간에 완료된 거래가 없습니다.')).toBeTruthy()
  expect(screen.getByText('0원')).toBeTruthy()
  choose('2026-01-01', '2026-06-30')
  expect(await screen.findByText('1,314,197,523원')).toBeTruthy()
  expect(screen.getByText('2건')).toBeTruthy()
  expect(screen.queryByText('이 기간에 완료된 거래가 없습니다.')).toBeNull()
  expect(period(calls[1])).toBe('2026-01-01~2026-06-30')
})

test('잘못 입력한 기간은 서버에 묻지 않고 바로 안내한다', async () => {
  const calls = serve('HR_ADMIN', totals(1, 1000))
  render(<App />)
  await screen.findByText('1,000원')
  choose('2026-11-30', '2026-11-01')
  expect(screen.getByText('종료일은 시작일과 같거나 이후여야 합니다.')).toBeTruthy()
  choose('', '2026-11-01')
  expect(screen.getByText('시작일과 종료일을 모두 선택해 주세요.')).toBeTruthy()
  expect(calls).toHaveLength(1)
})

test('서버 오류·권한 오류·통신 실패·표시할 수 없는 금액을 구분해 안내한다', async () => {
  serve('HR_ADMIN',
    failure(409, 'AMOUNT_OUT_OF_RANGE', '합계 금액이 정확히 표시할 수 있는 범위를 넘었습니다. 기간을 나누어 조회해 주세요.'),
    failure(403, 'FORBIDDEN', '관리자만 이용할 수 있습니다.'),
    () => { throw new TypeError('Failed to fetch') },
    totals(1, 2 ** 53),
  )
  render(<App />)
  expect(await screen.findByText(/기간을 나누어 조회해 주세요/)).toBeTruthy()
  choose('2026-11-01', '2026-11-30')
  expect(await screen.findByText('관리자만 이용할 수 있습니다.')).toBeTruthy()
  choose('2026-11-01', '2026-11-30')
  expect(await screen.findByText('서버와 통신하지 못했습니다. 연결을 확인하고 다시 시도해 주세요.')).toBeTruthy()
  choose('2026-11-01', '2026-11-30')
  expect(await screen.findByText('합계 금액을 정확히 표시할 수 없습니다. 기간을 나누어 조회해 주세요.')).toBeTruthy()
  expect(screen.queryByText(/원$/)).toBeNull()
})

test('일반 직원에게는 통계 화면을 보여주지 않고 요청도 보내지 않는다', async () => {
  const calls = serve('MEMBER')
  render(<App />)
  expect(await screen.findByText('관리자만 이용할 수 있습니다.')).toBeTruthy()
  expect(screen.queryByLabelText('시작일')).toBeNull()
  await waitFor(() => expect(calls).toHaveLength(0))
})
