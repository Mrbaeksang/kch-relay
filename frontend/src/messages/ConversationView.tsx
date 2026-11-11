import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useSession } from '../auth/session'
import { APIError } from '../lib/api'
import { describe, when } from '../transactions/model'
import { codePointLength, describeCounterparty, MAX_MESSAGE_LENGTH, type ConversationSummary, type Message, type Page } from './model'

type MessagePage = Page<Message> & { conversation: Omit<ConversationSummary, 'lastMessage' | 'hasUnread'> }
type Draft = { body: string; key: string }

const draftKey = (id: string) => `kch-relay:message-draft:${id}`
const newKey = () => crypto.randomUUID()

// The draft and its idempotency key survive an expired session (the screen is
// replaced by the login notice) and a reload, so a retry never duplicates.
function loadDraft(id: string): Draft {
  try {
    const saved = JSON.parse(sessionStorage.getItem(draftKey(id)) ?? 'null') as Draft | null
    if (saved && typeof saved.body === 'string' && typeof saved.key === 'string') return saved
  } catch { /* storage may be unavailable */ }
  return { body: '', key: newKey() }
}
function saveDraft(id: string, draft: Draft | null) {
  try { if (draft && draft.body) sessionStorage.setItem(draftKey(id), JSON.stringify(draft)); else sessionStorage.removeItem(draftKey(id)) } catch { /* ignore */ }
}

/** One conversation. Loaded on entry and on refresh; received messages are marked read only after they are shown. */
export function ConversationView({ conversationId }: { conversationId: string }) {
  const { api, state } = useSession()
  const storageId = `${state.status === "authenticated" ? state.user.id : "anonymous"}:${conversationId}`
  const [header, setHeader] = useState<MessagePage['conversation'] | null>(null)
  const [items, setItems] = useState<Message[]>([]) // oldest first
  const [older, setOlder] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [draft, setDraft] = useState<Draft>(() => loadDraft(storageId))
  const [sending, setSending] = useState(false)
  const [sendError, setSendError] = useState<string | null>(null)
  const readThrough = useRef<string | null>(null)

  async function load(cursor?: string) {
    setLoading(true); setError(null)
    try {
      const page = await api<MessagePage>(`/conversations/${conversationId}/messages${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''}`)
      const chronological = [...page.items].reverse()
      setHeader(page.conversation)
      setItems(previous => cursor ? [...chronological, ...previous] : chronological)
      setOlder(page.nextCursor)
    } catch (e) { setError(describe(e)) } finally { setLoading(false) }
  }
  useEffect(() => { void load() }, [conversationId])

  useEffect(() => {
    const received = items.filter(m => m.sender.kind === 'COUNTERPARTY')
    const latest = received[received.length - 1]
    if (!latest || !received.some(m => m.readAt === null) || readThrough.current === latest.id) return
    readThrough.current = latest.id
    api(`/conversations/${conversationId}/read`, { method: 'POST', body: { throughMessageId: latest.id } }).catch(() => { readThrough.current = null })
  }, [items])

  function edit(body: string) {
    const next = { ...draft, body, ...(sendError ? {key:newKey()}: {}) }
    setSendError(null)
    setDraft(next); saveDraft(storageId, next)
  }
  const length = codePointLength(draft.body)
  const blank = draft.body.trim() === ''
  const tooLong = length > MAX_MESSAGE_LENGTH

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (sending || blank || tooLong) return
    setSending(true); setSendError(null)
    try {
      const sent = await api<Message>(`/conversations/${conversationId}/messages`, { method: 'POST', body: { body: draft.body }, headers: { 'Idempotency-Key': draft.key } })
      setItems(previous => previous.some(m => m.id === sent.id) ? previous : [...previous, sent])
      const fresh = { body: '', key: newKey() }
      setDraft(fresh); saveDraft(storageId, null)
    } catch (e) {
      setSendError(e instanceof APIError && e.status === 401 ? '세션이 만료되었습니다. 다시 로그인해 주세요. 작성한 내용은 보관됩니다.' : describe(e))
    } finally { setSending(false) }
  }

  return <section aria-labelledby="conversation" className="conversation">
    <h1 id="conversation">{header ? `${header.product.title} · ${describeCounterparty(header.counterparty)}` : '쪽지'}</h1>
    <p><a href="/messages">대화 목록</a> · <Button type="button" disabled={loading} onClick={() => void load()}>새로고침</Button></p>
    {error ? <p role="alert">{error}</p> : null}
    {older ? <Button type="button" disabled={loading} onClick={() => void load(older)}>이전 쪽지 더 보기</Button> : null}
    {header && items.length === 0 ? <p>아직 쪽지가 없습니다. 첫 쪽지를 보내 보세요.</p> : null}
    <ol className="messages">{items.map(m => <li key={m.id} className={m.sender.kind === 'SELF' ? 'mine' : 'theirs'}>
      <strong>{m.sender.kind === 'SELF' ? m.sender.label : describeCounterparty(m.sender)}</strong>
      <p className="message-body">{m.body}</p>
      <small>{when(m.createdAt)}{m.sender.kind === 'SELF' ? ` · ${m.readAt ? '읽음' : '안 읽음'}` : ''}</small>
    </li>)}</ol>
    <form onSubmit={submit}>
      <label>쪽지 내용<Textarea disabled={sending} value={draft.body} onChange={e => edit(e.target.value)} rows={4} /></label>
      <p className={length > MAX_MESSAGE_LENGTH * 0.95 ? 'counter near-limit' : 'counter'} aria-live="polite">{length.toLocaleString('ko-KR')} / {MAX_MESSAGE_LENGTH.toLocaleString('ko-KR')}</p>
      {tooLong ? <p role="alert">쪽지는 {MAX_MESSAGE_LENGTH.toLocaleString('ko-KR')}자까지 보낼 수 있습니다.</p> : null}
      {sendError ? <p role="alert">{sendError}</p> : null}
      <Button type="submit" disabled={sending || blank || tooLong}>{sendError ? '다시 보내기' : '보내기'}</Button>
    </form>
  </section>
}
