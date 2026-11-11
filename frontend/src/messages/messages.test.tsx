import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import { RequireRole, SessionProvider } from '../auth/session'
import { ConversationList } from './ConversationList'
import { ConversationView } from './ConversationView'
import { InquiryButton } from './InquiryButton'
import type { ConversationSummary, Message } from './model'

type Call = { method: string; path: string; body: unknown; key: string | null }
type Reply = { status?: number; data?: unknown; error?: { code: string; message: string } }

/** Stubs the API: auth endpoints log in employee A, other paths go to route. */
function server(route: (call: Call) => Reply, userId = 'dev-employee-a') {
  const calls: Call[] = []
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input).replace('/api/v1', '')
    const meta = { hasUnreadMessages: false, hasUnreadNotifications: false }
    if (path === '/auth/config') return Response.json({ data: { authMode: 'development', developmentLogin: true, accounts: [] } })
    if (path === '/auth/me') return Response.json({ data: { user: { id: userId, realName: '테스트 직원 A', department: 'TEST_DEPT', jobLevel: 'TEST_LEVEL', role: 'MEMBER', status: 'ACTIVE' }, csrfToken: 'csrf' }, meta })
    const call = { method: init?.method ?? 'GET', path, body: init?.body ? JSON.parse(String(init.body)) : undefined, key: new Headers(init?.headers).get('Idempotency-Key') }
    calls.push(call)
    const reply = route(call)
    return Response.json(reply.error ? { error: reply.error, meta } : { data: reply.data, meta }, { status: reply.status ?? 200 })
  }))
  return calls
}

const mount = (node: React.ReactNode) => render(<SessionProvider><RequireRole>{node}</RequireRole></SessionProvider>)

const summary = (overrides: Partial<ConversationSummary> = {}): ConversationSummary => ({
  conversationId: 'c1', viewerRole: 'SELLER', product: { id: 'p1', title: '사과 한 박스' },
  counterparty: { label: '문의자 A', identity: null }, lastMessage: { body: '아직 있나요?', createdAt: '2026-11-11T01:00:00.000Z', fromSelf: false }, hasUnread: true, ...overrides,
})
const message = (id: string, kind: 'SELF' | 'COUNTERPARTY', overrides: Partial<Message> = {}): Message => ({
  id, body: `쪽지 ${id}`, createdAt: '2026-11-11T01:00:00.000Z', readAt: null,
  sender: kind === 'SELF' ? { kind, label: '나', identity: null } : { kind, label: '판매자', identity: null }, ...overrides,
})
const header = { conversationId: 'c1', viewerRole: 'BUYER', product: { id: 'p1', title: '사과 한 박스' }, counterparty: { label: '판매자', identity: null } }
const page = (items: Message[], nextCursor: string | null = null) => ({ data: { conversation: header, items, nextCursor } })

afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.restoreAllMocks(); vi.useRealTimers(); sessionStorage.clear() })

test('대화 목록은 진입 시 한 번 불러오고 새로고침 버튼으로만 다시 부른다', async () => {
  const calls = server(() => ({ data: { items: [summary()], nextCursor: null } }))
  mount(<ConversationList />)
  await screen.findByText('사과 한 박스')
  expect(screen.getByText(/문의자 A/)).toBeTruthy()
  expect(screen.getByText(/새 쪽지/)).toBeTruthy()
  vi.useFakeTimers()
  await act(async () => { vi.advanceTimersByTime(120_000) })
  vi.useRealTimers()
  expect(calls.filter(c => c.path.startsWith('/me/conversations'))).toHaveLength(1)
  fireEvent.click(screen.getByRole('button', { name: '새로고침' }))
  await waitFor(() => expect(calls.filter(c => c.path.startsWith('/me/conversations'))).toHaveLength(2))
})

test('대화가 없으면 빈 상태를 안내한다', async () => {
  server(() => ({ data: { items: [], nextCursor: null } }))
  mount(<ConversationList />)
  await screen.findByText('주고받은 쪽지가 없습니다.')
})

test('문의 버튼은 대화를 만들고 그 대화로 이동하며 실패 이유를 보여준다', async () => {
  const assign = vi.fn()
  vi.stubGlobal('location', { ...window.location, assign })
  let attempts = 0
  const calls = server(() => ++attempts === 1
    ? { status: 409, error: { code: 'INVALID_STATE', message: '승인되어 공개 중인 상품에만 새로 문의할 수 있습니다.' } }
    : { status: 201, data: { conversationId: 'c9' } })
  mount(<InquiryButton productId="p1" />)
  fireEvent.click(await screen.findByRole('button', { name: '익명으로 문의하기' }))
  expect((await screen.findByRole('alert')).textContent).toContain('승인되어 공개 중인 상품')
  fireEvent.click(screen.getByRole('button', { name: '익명으로 문의하기' }))
  await waitFor(() => expect(assign).toHaveBeenCalledWith('/messages/c9'))
  expect(calls.map(c => [c.method, c.path])).toEqual([['POST', '/products/p1/conversations'], ['POST', '/products/p1/conversations']])
})

test('오래된 쪽지가 위에 오고 이전 쪽지 더 보기로 cursor를 넘긴다', async () => {
  const calls = server(call => call.path.includes('cursor=') ? page([message('m0', 'COUNTERPARTY', { readAt: '2026-11-11T00:00:00.000Z' })]) : page([message('m2', 'SELF'), message('m1', 'COUNTERPARTY', { readAt: '2026-11-11T00:00:00.000Z' })], 'older'))
  mount(<ConversationView conversationId="c1" />)
  await screen.findByText('쪽지 m2')
  const bodies = () => screen.getAllByText(/^쪽지 m\d$/).map(e => e.textContent)
  expect(bodies()).toEqual(['쪽지 m1', '쪽지 m2'])
  fireEvent.click(screen.getByRole('button', { name: '이전 쪽지 더 보기' }))
  await screen.findByText('쪽지 m0')
  expect(bodies()).toEqual(['쪽지 m0', '쪽지 m1', '쪽지 m2'])
  expect(calls.some(c => c.path === '/conversations/c1/messages?cursor=older')).toBe(true)
  expect(calls.some(c => c.path.endsWith('/read'))).toBe(false) // nothing unread
})

test('화면에 그린 뒤 최신 수신 쪽지까지 읽음 처리하고 실패해도 쪽지는 그대로 보인다', async () => {
  const calls = server(call => call.path.endsWith('/read') ? { status: 500, error: { code: 'INTERNAL', message: '오류' } } : page([message('m3', 'SELF'), message('m2', 'COUNTERPARTY'), message('m1', 'COUNTERPARTY')]))
  mount(<ConversationView conversationId="c1" />)
  await screen.findByText('쪽지 m2')
  await waitFor(() => expect(calls.find(c => c.path === '/conversations/c1/read')?.body).toEqual({ throughMessageId: 'm2' }))
  expect(screen.getByText('쪽지 m1')).toBeTruthy()
  expect(screen.queryByRole('alert')).toBeNull()
})

test('보낸 쪽지는 상대가 읽었는지 보여준다', async () => {
  server(() => page([message('m2', 'SELF'), message('m1', 'SELF', { readAt: '2026-11-11T02:00:00.000Z' })]))
  mount(<ConversationView conversationId="c1" />)
  await screen.findByText('쪽지 m1')
  expect(screen.getByText(/· 읽음/)).toBeTruthy()
  expect(screen.getByText(/· 안 읽음/)).toBeTruthy()
})

test('글자 수는 서버처럼 문자 단위로 세고 공백만 있거나 넘치면 보낼 수 없다', async () => {
  server(() => page([]))
  mount(<ConversationView conversationId="c1" />)
  const box = await screen.findByLabelText('쪽지 내용')
  const send = () => screen.getByRole('button', { name: '보내기' }) as HTMLButtonElement
  fireEvent.change(box, { target: { value: '   \n' } })
  expect(send().disabled).toBe(true)
  fireEvent.change(box, { target: { value: '😀'.repeat(2000) } })
  expect(screen.getByText('2,000 / 2,000')).toBeTruthy()
  expect(send().disabled).toBe(false)
  fireEvent.change(box, { target: { value: '😀'.repeat(2001) } })
  expect(send().disabled).toBe(true)
  expect(screen.getByRole('alert').textContent).toContain('2,000자까지')
})

test('전송이 실패하면 내용을 지우지 않고 같은 키로 다시 보내며 성공 후에는 새 키를 쓴다', async () => {
  let posts = 0
  const calls = server(call => {
    if (call.method === 'POST' && call.path.endsWith('/messages')) {
      posts++
      if (posts === 1) return { status: 500, error: { code: 'INTERNAL', message: '잠시 후 다시 시도해 주세요.' } }
      return { status: 201, data: message(`s${posts}`, 'SELF', { body: (call.body as { body: string }).body }) }
    }
    return page([])
  })
  mount(<ConversationView conversationId="c1" />)
  const box = await screen.findByLabelText('쪽지 내용') as HTMLTextAreaElement
  fireEvent.change(box, { target: { value: '긴 한국어 문의입니다.\n두 번째 줄' } })
  fireEvent.click(screen.getByRole('button', { name: '보내기' }))
  fireEvent.click(screen.getByRole('button', { name: '보내기' })) // a double click sends once
  expect((await screen.findByRole('alert')).textContent).toBe('잠시 후 다시 시도해 주세요.')
  expect(box.value).toBe('긴 한국어 문의입니다.\n두 번째 줄')
  fireEvent.click(screen.getByRole('button', { name: '다시 보내기' }))
  await screen.findByText(/두 번째 줄/, { selector: '.message-body' })
  expect(box.value).toBe('')
  fireEvent.change(box, { target: { value: '다음 쪽지' } })
  fireEvent.click(screen.getByRole('button', { name: '보내기' }))
  await screen.findByText('다음 쪽지', { selector: '.message-body' })
  const keys = calls.filter(c => c.method === 'POST' && c.path.endsWith('/messages')).map(c => c.key)
  expect(keys).toHaveLength(3)
  expect(keys[0]).toBeTruthy()
  expect(keys[1]).toBe(keys[0])
  expect(keys[2]).not.toBe(keys[0])
})

test('세션이 만료되어도 작성한 내용과 키를 보관했다가 다시 들어오면 복원한다', async () => {
  const calls = server(call => call.method === 'POST' ? { status: 401, error: { code: 'UNAUTHORIZED', message: '로그인이 필요합니다.' } } : page([]))
  mount(<ConversationView conversationId="c1" />)
  fireEvent.change(await screen.findByLabelText('쪽지 내용'), { target: { value: '보관할 문의' } })
  fireEvent.click(screen.getByRole('button', { name: '보내기' }))
  await screen.findByText('로그인이 필요합니다.')
  cleanup()
  mount(<ConversationView conversationId="c1" />)
  expect(((await screen.findByLabelText('쪽지 내용')) as HTMLTextAreaElement).value).toBe('보관할 문의')
  const key = calls.find(c => c.method === 'POST')?.key
  expect(JSON.parse(sessionStorage.getItem('kch-relay:message-draft:dev-employee-a:c1')!).key).toBe(key)
})

test('서버 검증 오류 문구를 그대로 보여준다', async () => {
  server(call => call.method === 'POST' ? { status: 400, error: { code: 'VALIDATION_ERROR', message: '쪽지 내용을 입력해 주세요.' } } : page([]))
  mount(<ConversationView conversationId="c1" />)
  fireEvent.change(await screen.findByLabelText('쪽지 내용'), { target: { value: '내용' } })
  fireEvent.click(screen.getByRole('button', { name: '보내기' }))
  expect((await screen.findByRole('alert')).textContent).toBe('쪽지 내용을 입력해 주세요.')
})

test('같은 대화라도 다른 계정에서는 이전 작성자의 초안을 보지 않는다',async()=>{
 server(()=>page([]))
 const first=mount(<ConversationView conversationId="c1" />)
 await screen.findByText(/아직 쪽지가 없습니다/)
 fireEvent.change(screen.getByLabelText('쪽지 내용'),{target:{value:'A 계정만 볼 초안'}})
 first.unmount()
 server(()=>page([]),'dev-employee-b')
 mount(<ConversationView conversationId="c1" />)
 await screen.findByText(/아직 쪽지가 없습니다/)
 expect((screen.getByLabelText('쪽지 내용') as HTMLTextAreaElement).value).toBe('')
})
