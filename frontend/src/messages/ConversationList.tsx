import { Button } from "@/components/ui/button"
import { useEffect, useState } from 'react'
import { useSession } from '../auth/session'
import { describe, when } from '../transactions/model'
import { describeCounterparty, type ConversationSummary, type Page } from './model'

/** The viewer's conversations. Loaded on entry and on the refresh button only. */
export function ConversationList() {
  const { api } = useSession()
  const [page, setPage] = useState<Page<ConversationSummary> | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  async function load(cursor?: string) {
    setBusy(true); setError(null)
    try {
      const next = await api<Page<ConversationSummary>>(`/me/conversations${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''}`)
      setPage(previous => cursor && previous ? { items: [...previous.items, ...next.items], nextCursor: next.nextCursor } : next)
    } catch (e) { setError(describe(e)) } finally { setBusy(false) }
  }
  useEffect(() => { void load() }, [])
  return <section aria-labelledby="conversations" className="conversation-list">
    <h1 id="conversations">쪽지</h1>
    <Button type="button" disabled={busy} onClick={() => void load()}>새로고침</Button>
    {error ? <p role="alert">{error}</p> : null}
    {page && page.items.length === 0 ? <p>주고받은 쪽지가 없습니다.</p> : null}
    <ul>{page?.items.map(c => <li key={c.conversationId}>
      <a href={`/messages/${c.conversationId}`}>{c.product.title}</a> · {describeCounterparty(c.counterparty)}
      {c.hasUnread ? <strong> · 새 쪽지</strong> : null}
      {c.lastMessage ? <p className="message-preview">{c.lastMessage.fromSelf ? '나: ' : ''}{c.lastMessage.body} <small>{when(c.lastMessage.createdAt)}</small></p> : <p className="message-preview">아직 보낸 쪽지가 없습니다.</p>}
    </li>)}</ul>
    {page?.nextCursor ? <Button type="button" disabled={busy} onClick={() => void load(page.nextCursor!)}>더 보기</Button> : null}
  </section>
}
