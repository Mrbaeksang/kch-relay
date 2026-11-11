import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { SessionProvider } from '@/auth/session'
import { CatalogList, ProductDetail } from './Catalog'

const product = {
  id: 'p1',
  title: '동료의 사과 한 박스',
  description: '직접 재배한 사과입니다.',
  unitPrice: 0,
  remainingQuantity: 3,
  recruitmentClosed: false,
  canApply: true,
  seller: { label: '판매자' },
  images: [],
  createdAt: '2026-11-05T03:00:00.000Z',
}
function transport(handler: (path: string) => Response | Promise<Response>) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      const path = String(input)
      if (path.endsWith('/auth/config'))
        return Response.json({
          data: { authMode: 'disabled', developmentLogin: false, accounts: [] },
        })
      if (path.endsWith('/auth/me'))
        return Response.json({
          data: {
            user: {
              id: 'viewer',
              realName: '가상 직원',
              role: 'MEMBER',
              status: 'ACTIVE',
            },
            csrfToken: 'csrf',
          },
        })
      return handler(path)
    }),
  )
}
beforeEach(() => {
  window.history.replaceState(null, '', '/')
})
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})
test('상품 조회 후 검색하고 다음 페이지로 이동한다', async () => {
  transport((path) =>
    Response.json({
      data: path.includes('cursor=next')
        ? {
            items: [
              {
                ...product,
                id: 'p2',
                title: '영어 회화',
                unitPrice: 100000000,
              },
            ],
            nextCursor: null,
          }
        : { items: [product], nextCursor: 'next' },
    }),
  )
  render(
    <SessionProvider>
      <CatalogList />
    </SessionProvider>,
  )
  await screen.findByRole('link', { name: /동료의 사과 한 박스/ })
  expect(screen.getByText('나눔 · 0원')).toBeTruthy()
  fireEvent.change(screen.getByRole('searchbox'), { target: { value: '영어' } })
  fireEvent.submit(screen.getByRole('search'))
  await waitFor(() =>
    expect(
      vi
        .mocked(fetch)
        .mock.calls.some(([url]) =>
          String(url).includes('q=%EC%98%81%EC%96%B4'),
        ),
    ).toBe(true),
  )
  await screen.findByRole('link', { name: /동료의 사과 한 박스/ })
  fireEvent.click(screen.getByRole('button', { name: '다음 상품' }))
  await screen.findByRole('link', { name: /영어 회화/ })
  expect(screen.queryByRole('link', { name: /동료의 사과/ })).toBeNull()
  expect(screen.getByText('100,000,000원')).toBeTruthy()
})

test('통신 오류는 재시도할 수 있고 빈 검색 결과는 별도 안내한다', async () => {
  let failed = true
  transport(() =>
    failed
      ? Promise.reject(new Error('offline'))
      : Response.json({ data: { items: [], nextCursor: null } }),
  )
  render(
    <SessionProvider>
      <CatalogList />
    </SessionProvider>,
  )
  await screen.findByRole('alert')
  expect(screen.getByText('불러오지 못했습니다')).toBeTruthy()
  failed = false
  fireEvent.click(screen.getByRole('button', { name: '다시 시도' }))
  await screen.findByText('아직 공개된 상품이 없습니다')
  expect(screen.queryByRole('alert')).toBeNull()
})

test('상세는 판매자 별칭과 소진 상태를 표시하고 숨겨진 상품은 찾을 수 없다', async () => {
  transport(() =>
    Response.json({
      data: { ...product, remainingQuantity: 0, canApply: false },
    }),
  )
  const view = render(
    <SessionProvider>
      <ProductDetail productId="p1" />
    </SessionProvider>,
  )
  await screen.findByRole('heading', { name: product.title })
  expect(screen.getByText('수량 소진')).toBeTruthy()
  expect(screen.getByText('판매자', { selector: 'dd' })).toBeTruthy()
  expect(screen.getByText('현재 거래 신청할 수 없는 상품입니다.')).toBeTruthy()
  expect(screen.queryByRole('button', { name: /신청/ })).toBeNull()
  view.unmount()
  transport(() =>
    Response.json(
      { error: { code: 'NOT_FOUND', message: '상품을 찾을 수 없습니다.' } },
      { status: 404 },
    ),
  )
  render(
    <SessionProvider>
      <ProductDetail productId="private" />
    </SessionProvider>,
  )
  await screen.findByRole('heading', { name: '상품을 찾을 수 없습니다' })
  expect(screen.queryByText(product.title)).toBeNull()
})

test('이전 검색의 늦은 응답이 새 검색을 덮지 않는다', async () => {
  let finishOld: ((r: Response) => void) | undefined
  transport((path) =>
    path.includes('q=')
      ? Response.json({
          data: {
            items: [{ ...product, title: '새 검색 결과' }],
            nextCursor: null,
          },
        })
      : new Promise<Response>((resolve) => {
          finishOld = resolve
        }),
  )
  render(
    <SessionProvider>
      <CatalogList />
    </SessionProvider>,
  )
  await waitFor(() => expect(finishOld).toBeDefined())
  fireEvent.change(screen.getByRole('searchbox'), {
    target: { value: '새 검색' },
  })
  fireEvent.submit(screen.getByRole('search'))
  await screen.findByRole('link', { name: /새 검색 결과/ })
  finishOld!(Response.json({ data: { items: [product], nextCursor: null } }))
  await waitFor(() =>
    expect(screen.queryByRole('link', { name: /동료의 사과/ })).toBeNull(),
  )
  expect(screen.getByRole('link', { name: /새 검색 결과/ })).toBeTruthy()
})
