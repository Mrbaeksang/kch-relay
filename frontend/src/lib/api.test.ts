import { expect, test, vi } from 'vitest'
import { createAPI } from './api'

test('쿠키와 CSRF를 연결하고 미확인 표시를 전달한다', async () => {
  const onMeta = vi.fn()
  const transport = vi.fn(async () => new Response(JSON.stringify({ data: { ok: true }, meta: { hasUnreadMessages: true, hasUnreadNotifications: false } }), { status: 200 }))
  const api = createAPI({ getCSRF: () => 'csrf', onMeta, onUnauthorized: vi.fn() }, transport)
  expect(await api('/auth/logout', { method: 'POST', body: {} })).toEqual({ ok: true })
  const [path, options] = transport.mock.calls[0] as unknown as [string, RequestInit]
  expect(path).toBe('/api/v1/auth/logout')
  expect(options.credentials).toBe('same-origin')
  expect(new Headers(options.headers).get('X-CSRF-Token')).toBe('csrf')
  expect(onMeta).toHaveBeenCalledWith({ hasUnreadMessages: true, hasUnreadNotifications: false })
})

test('401은 로그인 상태를 정리하고 403은 권한 오류만 전달한다', async () => {
  const onUnauthorized = vi.fn()
  const api = createAPI({ getCSRF: () => null, onMeta: vi.fn(), onUnauthorized }, async input => Response.json({ error: { code: 'FORBIDDEN', message: '권한 없음' } }, { status: String(input).endsWith('/expired') ? 401 : 403 }))
  await expect(api('/forbidden')).rejects.toMatchObject({ status: 403 })
  expect(onUnauthorized).not.toHaveBeenCalled()
  await expect(api('/expired')).rejects.toMatchObject({ status: 401 })
  expect(onUnauthorized).toHaveBeenCalledTimes(1)
})

test('사진 업로드는 FormData를 그대로 보내고 브라우저가 multipart 경계를 정하게 한다', async () => {
  const transport = vi.fn(async () => Response.json({ data: { id: 'img', url: '/api/v1/images/img' } }, { status: 201 }))
  const api = createAPI({ getCSRF: () => 'csrf', onMeta: vi.fn(), onUnauthorized: vi.fn() }, transport)
  const form = new FormData()
  form.append('file', new Blob(['x'], { type: 'image/jpeg' }), 'photo.jpg')
  await api('/images', { method: 'POST', body: form })
  const [, options] = transport.mock.calls[0] as unknown as [string, RequestInit]
  expect(options.body).toBe(form)
  expect(new Headers(options.headers).get('Content-Type')).toBeNull()
  expect(new Headers(options.headers).get('X-CSRF-Token')).toBe('csrf')
})

test('추가 헤더를 보내되 CSRF 값은 세션 값을 유지한다', async () => {
  const transport = vi.fn(async () => Response.json({ data: { ok: true } }))
  const api = createAPI({ getCSRF: () => 'csrf', onMeta: vi.fn(), onUnauthorized: vi.fn() }, transport)
  await api('/conversations/c1/messages', { method: 'POST', body: { body: '안녕' }, headers: { 'Idempotency-Key': 'k1', 'X-CSRF-Token': 'evil' } })
  const [, options] = transport.mock.calls[0] as unknown as [string, RequestInit]
  const headers = new Headers(options.headers)
  expect(headers.get('Idempotency-Key')).toBe('k1')
  expect(headers.get('X-CSRF-Token')).toBe('csrf')
  expect(headers.get('Content-Type')).toBe('application/json')
})
