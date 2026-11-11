import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { SessionProvider } from '../auth/session'
import { ApplyForm, DealDetail } from './deals'
import { ProductManage } from './manage'
import type { Deal, ManagedProduct } from './model'

type Call = { method: string; path: string; body: unknown; csrf: string | null }
type Reply = { status?: number; data?: unknown; error?: { code: string; message: string } }

/** Stubs the API: auth endpoints log in employee A, other paths go to route. */
function server(route: (call: Call) => Reply) {
  const calls: Call[] = []
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input).replace('/api/v1', '')
    const meta = { hasUnreadMessages: false, hasUnreadNotifications: false }
    if (path === '/auth/config') return Response.json({ data: { authMode: 'development', developmentLogin: true, accounts: [] } })
    if (path === '/auth/me') return Response.json({ data: { user: { id: 'dev-employee-a', realName: '테스트 직원 A', department: 'TEST_DEPT', jobLevel: 'TEST_LEVEL', role: 'MEMBER', status: 'ACTIVE' }, csrfToken: 'csrf' }, meta })
    const call = { method: init?.method ?? 'GET', path, body: init?.body ? JSON.parse(String(init.body)) : undefined, csrf: new Headers(init?.headers).get('X-CSRF-Token') }
    calls.push(call)
    const reply = route(call)
    return Response.json(reply.error ? { error: reply.error, meta } : { data: reply.data, meta }, { status: reply.status ?? 200 })
  }))
  return calls
}

const deal = (overrides: Partial<Deal> = {}): Deal => ({
  id: 'deal-1', productId: 'product-1', conversationId: 'conversation-1', status: 'REQUESTED', viewerRole: 'BUYER',
  product: { id: 'product-1', title: '사과 한 박스', unitPrice: 10000, remainingQuantity: 5, recruitmentClosed: false, hidden: false },
  requestedQuantity: 2, agreedUnitPrice: null, agreedQuantity: null, totalPrice: null,
  requestedAt: '2026-11-11T01:00:00.000Z', quantityUpdatedAt: null, acceptedAt: null, completedAt: null, rejectedAt: null, withdrawnAt: null,
  counterparty: { label: '판매자', identity: null }, actions: ['updateQuantity', 'withdraw'], ...overrides,
})

const product: ManagedProduct = { id: 'product-1', title: '사과 한 박스', description: '직접 전달합니다.', unitPrice: 10000, remainingQuantity: 5, recruitmentClosed: false, approvalStatus: 'APPROVED', hidden: false, imageIds: [], pendingApplicationCount: 1 }

afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.restoreAllMocks() })

test('구매자는 수량을 정해 신청하고 실패 이유를 안내받는다', async () => {
  let attempts = 0
  const calls = server(() => ++attempts === 1
    ? { status: 409, error: { code: 'INSUFFICIENT_QUANTITY', message: '남은 수량이 부족합니다.' } }
    : { status: 201, data: deal() })
  render(<SessionProvider><ApplyForm productId="product-1" /></SessionProvider>)
  fireEvent.change(screen.getByLabelText('신청 수량'), { target: { value: '9' } })
  fireEvent.click(screen.getByRole('button', { name: '거래 신청' }))
  expect((await screen.findByRole('alert')).textContent).toBe('남은 수량이 부족합니다.')
  fireEvent.change(screen.getByLabelText('신청 수량'), { target: { value: '2' } })
  fireEvent.click(screen.getByRole('button', { name: '거래 신청' }))
  expect((await screen.findByRole('status')).textContent).toContain('신청했습니다')
  expect(calls.map(c => [c.method, c.path, c.body])).toEqual([
    ['POST', '/products/product-1/deals', { quantity: 9 }],
    ['POST', '/products/product-1/deals', { quantity: 2 }],
  ])
})

test('구매자는 익명의 판매자에게 보낸 신청의 수량을 바꾸고 철회한다', async () => {
  const calls = server(call => {
    if (call.path.endsWith('/quantity')) return { data: deal({ requestedQuantity: 3, quantityUpdatedAt: '2026-11-11T02:00:00.000Z' }) }
    if (call.path.endsWith('/withdraw')) return { data: deal({ requestedQuantity: 3, status: 'WITHDRAWN', actions: [] }) }
    return { data: deal() }
  })
  render(<SessionProvider><DealDetail dealId="deal-1" /></SessionProvider>)
  await screen.findByRole('heading', { name: '사과 한 박스' })
  expect(screen.getByText('판매자')).toBeTruthy()
  expect(screen.queryByRole('button', { name: '수락' })).toBeNull()

  fireEvent.change(screen.getByLabelText('신청 수량'), { target: { value: '3' } })
  fireEvent.click(screen.getByRole('button', { name: '수량 변경' }))
  await screen.findByText(/수량 수정/)
  fireEvent.click(screen.getByRole('button', { name: '신청 철회' }))
  await screen.findByText('철회함')
  expect(screen.queryByRole('button', { name: '신청 철회' })).toBeNull()
  expect(calls.slice(1).map(c => [c.method, c.path, c.body, c.csrf])).toEqual([
    ['PATCH', '/deals/deal-1/quantity', { quantity: 3 }, 'csrf'],
    ['POST', '/deals/deal-1/withdraw', undefined, 'csrf'],
  ])
})

test('판매자는 확인 후 수락하고 성사된 상대의 신원과 확정 금액을 본다', async () => {
  const seller = { viewerRole: 'SELLER' as const, counterparty: { label: '익명 1', identity: null }, actions: ['accept', 'reject'] as Deal['actions'] }
  const accepted = deal({ ...seller, status: 'ACCEPTED', agreedUnitPrice: 10000, agreedQuantity: 2, totalPrice: 20000, acceptedAt: '2026-11-11T03:00:00.000Z', counterparty: { label: '익명 1', identity: { realName: '테스트 직원 B', department: 'TEST_DEPT', jobLevel: 'TEST_LEVEL' } }, actions: ['complete'] })
  let current = deal(seller)
  const calls = server(call => {
    if (call.path.endsWith('/accept')) current = accepted
    return { data: call.path.endsWith('/deals') ? { items: [current], nextCursor: null, product } : current }
  })
  const confirm = vi.spyOn(window, 'confirm').mockReturnValueOnce(false).mockReturnValueOnce(true)
  render(<SessionProvider><ProductManage productId="product-1" /></SessionProvider>)
  const list = await screen.findByRole('list')
  expect(within(list).getByText('익명 1')).toBeTruthy()

  fireEvent.click(within(list).getByRole('button', { name: '수락' }))
  expect(calls.some(c => c.path.endsWith('/accept'))).toBe(false)
  fireEvent.click(within(list).getByRole('button', { name: '수락' }))
  expect(confirm).toHaveBeenCalledTimes(2)
  await within(list).findByText(/테스트 직원 B/)
  expect(within(list).getByText('10,000원 × 2개 = 20,000원')).toBeTruthy()
  expect(within(list).getByRole('button', { name: '거래 완료 처리' })).toBeTruthy()
})

test('판매자는 대기 신청이 있으면 명시적으로 거절을 선택해야 내용을 수정할 수 있다', async () => {
  const calls = server(call => {
    if (call.method === 'PATCH') {
      const body = call.body as { rejectPendingApplications: boolean }
      return body.rejectPendingApplications ? { data: { ...product, title: '배 한 박스', pendingApplicationCount: 0 } } : { status: 409, error: { code: 'PENDING_APPLICATIONS_EXIST', message: '대기 중인 신청을 거절한 후 수정할 수 있습니다.' } }
    }
    return { data: { items: [], nextCursor: null, product } }
  })
  render(<SessionProvider><ProductManage productId="product-1" /></SessionProvider>)
  fireEvent.change(await screen.findByLabelText('제목'), { target: { value: '배 한 박스' } })
  fireEvent.click(screen.getByRole('button', { name: '내용 저장' }))
  expect((await screen.findByRole('alert')).textContent).toBe('대기 중인 신청을 거절한 후 수정할 수 있습니다.')

  fireEvent.click(screen.getByLabelText(/대기 중인 신청 1건을 모두 거절하고 수정합니다/))
  fireEvent.click(screen.getByRole('button', { name: '내용 저장' }))
  expect((await screen.findByRole('status')).textContent).toBe('상품을 수정했습니다.')
  expect(calls.filter(c => c.method === 'PATCH').map(c => c.body)).toEqual([
    { title: '배 한 박스', description: '직접 전달합니다.', unitPrice: 10000, rejectPendingApplications: false },
    { title: '배 한 박스', description: '직접 전달합니다.', unitPrice: 10000, rejectPendingApplications: true },
  ])
})

test('판매자의 수량 변경과 모집 종료는 대기 신청을 유지한다고 안내한다', async () => {
  const calls = server(() => ({ data: { items: [], nextCursor: null, product } }))
  render(<SessionProvider><ProductManage productId="product-1" /></SessionProvider>)
  fireEvent.change(await screen.findByLabelText('잔여 수량'), { target: { value: '0' } })
  fireEvent.click(screen.getByRole('button', { name: '수량 저장' }))
  expect((await screen.findByRole('status')).textContent).toContain('대기 중인 신청은 그대로 유지됩니다')
  fireEvent.click(screen.getByRole('button', { name: '모집 종료' }))
  expect((await screen.findByText(/모집을 종료했습니다/)).textContent).toContain('기존 신청과 거래는 유지됩니다')
  expect(calls.filter(c => c.method !== 'GET').map(c => [c.method, c.path, c.body])).toEqual([
    ['PATCH', '/products/product-1/quantity', { remainingQuantity: 0 }],
    ['POST', '/products/product-1/close', undefined],
  ])
})
