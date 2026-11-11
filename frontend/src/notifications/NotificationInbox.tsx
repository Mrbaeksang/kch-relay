import { Button } from "@/components/ui/button"
import { useCallback, useEffect, useRef, useState, type MouseEvent } from 'react'
import { useSession } from '../auth/session'
import { APIError } from '../lib/api'

export type Notification = { id: string; type: string; dealId: string; productId: string; createdAt: string; readAt: string | null }
type NotificationPage = { items: Notification[]; nextCursor: string | null }
type Props = { navigate?: (path: string) => void; pageSize?: number }

// 거래 상세 화면(기능 3 담당)의 경로. 알림은 상품이 아니라 본인 거래 기록으로 연결한다.
export const dealPath = (dealId: string) => `/market/deals/${encodeURIComponent(dealId)}`

const messages: Record<string, string> = { APPLICATION_REJECTED: '판매자가 거래 신청을 거절했어요.' }
const timeFormat = new Intl.DateTimeFormat('ko-KR', { timeZone: 'Asia/Seoul', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit' })
function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : timeFormat.format(date)
}

export function NotificationInbox({ navigate = path => window.location.assign(path), pageSize = 20 }: Props) {
  const { api } = useSession()
  const [items, setItems] = useState<Notification[]>([])
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
  const [loadingMore, setLoadingMore] = useState(false)
  const [moreError, setMoreError] = useState(false)
  const [itemError, setItemError] = useState<string | null>(null)
  const opening = useRef<string | null>(null)

  const path = useCallback((cursor: string | null) => `/me/notifications?limit=${pageSize}${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`, [pageSize])

  const loadFirst = useCallback((signal?: AbortSignal) => {
    setStatus('loading')
    setItemError(null)
    api<NotificationPage>(path(null), { signal })
      .then(page => { setItems(page.items); setNextCursor(page.nextCursor); setMoreError(false); setStatus('ready') })
      .catch((error: unknown) => {
        if (signal?.aborted || (error instanceof APIError && error.status === 401)) return
        setStatus('error')
      })
  }, [api, path])

  useEffect(() => {
    const controller = new AbortController()
    loadFirst(controller.signal)
    return () => controller.abort()
  }, [loadFirst])

  async function loadMore() {
    if (!nextCursor || loadingMore) return
    setLoadingMore(true)
    setMoreError(false)
    try {
      const page = await api<NotificationPage>(path(nextCursor))
      setItems(current => {
        const known = new Set(current.map(item => item.id))
        return [...current, ...page.items.filter(item => !known.has(item.id))]
      })
      setNextCursor(page.nextCursor)
    } catch (error) {
      if (!(error instanceof APIError && error.status === 401)) setMoreError(true)
    } finally {
      setLoadingMore(false)
    }
  }

  async function open(event: MouseEvent<HTMLAnchorElement>, item: Notification) {
    event.preventDefault()
    if (opening.current) return
    opening.current = item.id
    setItemError(null)
    try {
      if (item.readAt === null) {
        try {
          const updated = await api<Notification>(`/me/notifications/${encodeURIComponent(item.id)}/read`, { method: 'POST' })
          setItems(current => current.map(existing => existing.id === updated.id ? updated : existing))
        } catch (error) {
          if (error instanceof APIError && error.status === 401) return
          if (error instanceof APIError && error.status === 404) {
            setItemError('이 알림을 찾을 수 없어요. 목록을 새로고침해 주세요.')
            return
          }
          // 읽음 표시에 실패해도 거래 확인은 막지 않는다. 알림은 안 읽음으로 남아 다음에 다시 처리된다.
        }
      }
      navigate(dealPath(item.dealId))
    } finally {
      opening.current = null
    }
  }

  return <section aria-labelledby="notification-inbox-title" className="notification-inbox">
    <h1 id="notification-inbox-title">알림함</h1>
    <p className="notification-hint">알림을 누르면 읽음으로 바뀌고 해당 거래로 이동해요. 상품이 숨겨져도 내 거래 내역에서 거절된 신청을 확인할 수 있어요.</p>
    {status === 'loading' ? <p aria-live="polite">알림을 불러오고 있어요.</p> : null}
    {status === 'error' ? <div role="alert">
      <p>알림을 불러오지 못했어요. 연결을 확인하고 다시 시도해 주세요.</p>
      <Button type="button" onClick={() => loadFirst()}>다시 시도</Button>
    </div> : null}
    {itemError ? <div role="alert">
      <p>{itemError}</p>
      <Button type="button" onClick={() => loadFirst()}>새로고침</Button>
    </div> : null}
    {status === 'ready' && items.length === 0 ? <p>아직 받은 알림이 없어요.</p> : null}
    {status === 'ready' && items.length > 0 ? <ul className="notification-list">
      {items.map(item => <li key={item.id} className={item.readAt === null ? 'unread' : undefined}>
        <a href={dealPath(item.dealId)} onClick={event => void open(event, item)}>
          <span className="notification-message">{messages[item.type] ?? '새 알림이 있어요.'}</span>
          <time dateTime={item.createdAt}>{formatTime(item.createdAt)}</time>
          {item.readAt === null ? <span className="notification-unread">안 읽음</span> : null}
        </a>
      </li>)}
    </ul> : null}
    {status === 'ready' && nextCursor ? <Button type="button" disabled={loadingMore} onClick={() => void loadMore()}>{loadingMore ? '불러오는 중…' : '더 보기'}</Button> : null}
    {moreError ? <p role="alert">다음 알림을 불러오지 못했어요. 더 보기를 다시 눌러 주세요.</p> : null}
  </section>
}
