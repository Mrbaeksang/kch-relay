import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { SessionProvider } from '../auth/session'
import { AdminReview } from './AdminReview'
import { MyProducts } from './MyProducts'

const limits = { maxTitleLength: 100, maxDescriptionLength: 5000, minUnitPrice: 0, maxUnitPrice: 10_000_000_000, minQuantity: 1, maxQuantity: 10_000, maxImagesPerProduct: 5, maxImageBytes: 5 << 20, imageTypes: ['image/jpeg', 'image/png'], maxReasonLength: 500 }
const meta = { hasUnreadMessages: false, hasUnreadNotifications: false }
const rejected = { id: 'p1', title: '사과 한 박스', description: '설명', unitPrice: 10000, remainingQuantity: 2, approvalStatus: 'REJECTED', rejectionReason: '가격을 다시 확인해 주세요.', recruitmentClosed: false, images: [], createdAt: '2026-11-11T00:00:00.000Z', updatedAt: '2026-11-11T00:00:00.000Z' }
type Call = { path: string; method: string; body: unknown }

function serve(role: 'MEMBER' | 'HR_ADMIN', routes: Record<string, (body: unknown) => Response>) {
  const calls: Call[] = []
  vi.stubGlobal('fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input).replace('/api/v1', '')
    const method = init?.method ?? 'GET'
    const body = typeof init?.body === 'string' ? JSON.parse(init.body) : init?.body
    calls.push({ path, method, body })
    if (path === '/auth/config') return Response.json({ data: { authMode: 'development', developmentLogin: true, accounts: [] } })
    if (path === '/auth/me') return Response.json({ data: { user: { id: 'u', realName: '테스트', department: 'D', jobLevel: 'L', role, status: 'ACTIVE' }, csrfToken: 'csrf' }, meta })
    if (path === '/product-limits') return Response.json({ data: limits, meta })
    const handler = routes[`${method} ${path}`]
    return handler ? handler(body) : Response.json({ error: { code: 'NOT_FOUND', message: '없음' } }, { status: 404 })
  })
  return calls
}
const ok = (data: unknown, status = 200) => () => Response.json({ data, meta }, { status })
afterEach(() => { cleanup(); vi.unstubAllGlobals() })

test('상품 등록은 입력 오류를 서버 전송 전에 안내하고 0원 나눔을 정수로 저장한다', async () => {
  const calls = serve('MEMBER', { 'GET /me/products': ok({ items: [], nextCursor: null }), 'POST /products': ok({ ...rejected, approvalStatus: 'PENDING' }, 201) })
  render(<SessionProvider><MyProducts /></SessionProvider>)
  fireEvent.click(await screen.findByRole('button', { name: '상품 등록' }))
  fireEvent.click(screen.getByRole('button', { name: '등록하고 심사 요청' }))
  expect((await screen.findByRole('alert')).textContent).toContain('제목')
  fireEvent.change(screen.getByLabelText('제목'), { target: { value: '  책 나눔  ' } })
  fireEvent.change(screen.getByLabelText('본문'), { target: { value: '한 권입니다.' } })
  fireEvent.change(screen.getByLabelText(/단가/), { target: { value: '1.5' } })
  fireEvent.click(screen.getByRole('button', { name: '등록하고 심사 요청' }))
  expect((await screen.findByRole('alert')).textContent).toContain('단가은(는) 0 이상')
  expect(calls.some(call => call.path === '/products')).toBe(false)
  fireEvent.change(screen.getByLabelText(/단가/), { target: { value: '0' } })
  fireEvent.click(screen.getByRole('button', { name: '등록하고 심사 요청' }))
  await screen.findByText(/인사팀 승인 후 장터에 공개/)
  expect(calls.find(call => call.method === 'POST' && call.path === '/products')?.body).toEqual({ title: '책 나눔', description: '한 권입니다.', unitPrice: 0, quantity: 1, imageIds: [] })
})

test('서버가 거절한 이유를 그대로 안내한다', async () => {
  serve('MEMBER', { 'GET /me/products': ok({ items: [], nextCursor: null }), 'POST /products': () => Response.json({ error: { code: 'VALIDATION_ERROR', message: '사용할 수 없는 사진이 포함되어 있습니다.' }, meta }, { status: 400 }) })
  render(<SessionProvider><MyProducts /></SessionProvider>)
  fireEvent.click(await screen.findByRole('button', { name: '상품 등록' }))
  fireEvent.change(screen.getByLabelText('제목'), { target: { value: '사과' } })
  fireEvent.change(screen.getByLabelText('본문'), { target: { value: '설명' } })
  fireEvent.change(screen.getByLabelText(/단가/), { target: { value: '100000000' } })
  fireEvent.click(screen.getByRole('button', { name: '등록하고 심사 요청' }))
  expect((await screen.findByRole('alert')).textContent).toBe('사용할 수 없는 사진이 포함되어 있습니다.')
})

test('반려 사유를 보여 주고 수정 내용으로 다시 심사를 요청한다', async () => {
  const calls = serve('MEMBER', { 'GET /me/products': ok({ items: [rejected], nextCursor: null }), 'POST /products/p1/submit-review': ok({ ...rejected, approvalStatus: 'PENDING' }) })
  render(<SessionProvider><MyProducts /></SessionProvider>)
  expect((await screen.findByRole('note')).textContent).toContain('가격을 다시 확인해 주세요.')
  fireEvent.click(screen.getByRole('button', { name: '수정 후 다시 요청' }))
  fireEvent.change(screen.getByLabelText(/단가/), { target: { value: '9000' } })
  fireEvent.click(screen.getByRole('button', { name: '수정하고 다시 심사 요청' }))
  await screen.findByText('수정 내용으로 다시 심사를 요청했습니다.')
  expect(calls.find(call => call.path === '/products/p1/submit-review')?.body).toMatchObject({ unitPrice: 9000, quantity: 2 })
})

test('인사팀은 작성자 신원을 보고 사유 없이 반려할 수 없다', async () => {
  const item = { id: 'p1', title: '사과 한 박스', unitPrice: 10000, remainingQuantity: 2, approvalStatus: 'PENDING', seller: { realName: '테스트 직원 A', department: 'TEST_DEPT', jobLevel: 'TEST_LEVEL' }, createdAt: rejected.createdAt, updatedAt: rejected.updatedAt }
  const calls = serve('HR_ADMIN', {
    'GET /admin/products?approvalStatus=PENDING': ok({ items: [item], nextCursor: null }),
    'GET /admin/products/p1': ok({ ...item, description: '설명', images: [], reviews: [] }),
    'POST /admin/products/p1/reject': ok({ ...item, approvalStatus: 'REJECTED' }),
  })
  render(<SessionProvider><AdminReview /></SessionProvider>)
  expect(await screen.findByText(/테스트 직원 A · TEST_DEPT/)).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: '사과 한 박스 상세' }))
  fireEvent.click(await screen.findByRole('button', { name: '반려' }))
  expect((await screen.findByRole('alert')).textContent).toContain('반려 사유')
  expect(calls.some(call => call.path.endsWith('/reject'))).toBe(false)
  fireEvent.change(screen.getByLabelText('반려 사유'), { target: { value: '사진이 필요합니다.' } })
  fireEvent.click(screen.getByRole('button', { name: '반려' }))
  await waitFor(() => expect(calls.find(call => call.path.endsWith('/reject'))?.body).toEqual({ reason: '사진이 필요합니다.' }))
})
