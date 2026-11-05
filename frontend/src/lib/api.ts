export type Meta = { hasUnreadMessages: boolean; hasUnreadNotifications: boolean }
export type RequestOptions = { method?: string; body?: unknown; signal?: AbortSignal }
export type API = <T>(path: string, options?: RequestOptions) => Promise<T>
export class APIError extends Error {
  constructor(public status: number, public code: string, message: string, public requestId?: string) { super(message) }
}
type Hooks = { getCSRF: () => string | null; onUnauthorized: () => void; onMeta: (meta: Meta) => void }
export function createAPI(hooks: Hooks, transport: typeof fetch = fetch): API {
  return async <T>(path: string, options: RequestOptions = {}): Promise<T> => {
    if (!path.startsWith('/') || path.startsWith('//')) throw new Error('API 경로가 올바르지 않습니다.')
    const method = options.method ?? 'GET'
    const headers = new Headers({ Accept: 'application/json' })
    if (options.body !== undefined) headers.set('Content-Type', 'application/json')
    const csrf = hooks.getCSRF()
    if (!['GET', 'HEAD', 'OPTIONS'].includes(method.toUpperCase()) && csrf) headers.set('X-CSRF-Token', csrf)
    const response = await transport(`/api/v1${path}`, { method, headers, credentials: 'same-origin', body: options.body === undefined ? undefined : JSON.stringify(options.body), signal: options.signal })
    if (options.signal?.aborted) throw new DOMException('Aborted', 'AbortError')
    if (response.status === 401) hooks.onUnauthorized()
    let envelope
    try { envelope = await response.json() } catch { throw new APIError(response.status, 'INVALID_RESPONSE', '서버 응답을 확인할 수 없습니다.') }
    if (!response.ok) throw new APIError(response.status, envelope.error?.code ?? 'REQUEST_FAILED', envelope.error?.message ?? '요청을 처리하지 못했습니다.', envelope.error?.requestId)
    if (!envelope || !Object.hasOwn(envelope, 'data')) throw new APIError(500, 'INVALID_RESPONSE', '서버 응답을 확인할 수 없습니다.')
    if (envelope.meta !== undefined) {
      if (typeof envelope.meta?.hasUnreadMessages !== 'boolean' || typeof envelope.meta?.hasUnreadNotifications !== 'boolean') throw new APIError(500, 'INVALID_RESPONSE', '알림 정보를 확인할 수 없습니다.')
      hooks.onMeta(envelope.meta)
    }
    return envelope.data as T
  }
}
