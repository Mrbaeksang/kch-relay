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
