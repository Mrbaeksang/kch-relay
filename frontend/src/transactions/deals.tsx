import { useEffect, useState, type FormEvent } from 'react'
import { useSession } from '../auth/session'
import { actionLabels, describe, statusLabels, when, won, type Deal, type DealAction, type DealStatus, type Page } from './model'

export function DealSummary({ deal }: { deal: Deal }) {
  const { identity } = deal.counterparty
  return <dl className="deal-summary">
    <dt>상태</dt><dd>{statusLabels[deal.status]}</dd>
    <dt>상대</dt><dd>{deal.counterparty.label}{identity ? ` · ${identity.realName} (${identity.department} · ${identity.jobLevel})` : ''}</dd>
    <dt>신청 수량</dt><dd>{deal.requestedQuantity}개</dd>
    <dt>신청 시각</dt><dd>{when(deal.requestedAt)}{deal.quantityUpdatedAt ? ` (수량 수정 ${when(deal.quantityUpdatedAt)})` : ''}</dd>
    {deal.totalPrice !== null ? <><dt>확정 내용</dt><dd>{won(deal.agreedUnitPrice!)} × {deal.agreedQuantity}개 = {won(deal.totalPrice)}</dd></> : <><dt>현재 단가</dt><dd>{won(deal.product.unitPrice)}</dd></>}
    {deal.acceptedAt ? <><dt>성사 시각</dt><dd>{when(deal.acceptedAt)}</dd></> : null}
    {deal.completedAt ? <><dt>완료 시각</dt><dd>{when(deal.completedAt)}</dd></> : null}
  </dl>
}

/** Buttons for the transitions the server says this viewer may perform now. */
export function DealActions({ deal, onChange }: { deal: Deal; onChange: (deal: Deal) => void }) {
  const { api } = useSession()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [quantity, setQuantity] = useState(String(deal.requestedQuantity))
  useEffect(() => setQuantity(String(deal.requestedQuantity)), [deal.requestedQuantity])

  async function run(request: () => Promise<Deal>) {
    setBusy(true); setError(null)
    try { onChange(await request()) } catch (e) { setError(describe(e)) } finally { setBusy(false) }
  }
  function act(action: Exclude<DealAction, 'updateQuantity'>) {
    if (action === 'accept' && !window.confirm('수락하면 취소할 수 없고 서로의 실명·부서·직급이 공개됩니다. 수락할까요?')) return
    void run(() => api<Deal>(`/deals/${deal.id}/${action}`, { method: 'POST' }))
  }
  function changeQuantity(event: FormEvent) {
    event.preventDefault()
    void run(() => api<Deal>(`/deals/${deal.id}/quantity`, { method: 'PATCH', body: { quantity: Number(quantity) } }))
  }
  const buttons = deal.actions.filter((a): a is Exclude<DealAction, 'updateQuantity'> => a !== 'updateQuantity')
  return <div className="deal-actions">
    {deal.actions.includes('updateQuantity') ? <form onSubmit={changeQuantity}>
      <label>신청 수량 <input type="number" min={1} step={1} required value={quantity} onChange={e => setQuantity(e.target.value)} /></label>
      <button type="submit" disabled={busy}>수량 변경</button>
    </form> : null}
    {buttons.map(action => <button key={action} type="button" disabled={busy} onClick={() => act(action)}>{actionLabels[action]}</button>)}
    {error ? <p role="alert">{error}</p> : null}
  </div>
}

export function DealDetail({ dealId }: { dealId: string }) {
  const { api } = useSession()
  const [deal, setDeal] = useState<Deal | null>(null)
  const [error, setError] = useState<string | null>(null)
  useEffect(() => {
    const controller = new AbortController()
    api<Deal>(`/deals/${dealId}`, { signal: controller.signal }).then(setDeal, e => { if (!controller.signal.aborted) setError(describe(e)) })
    return () => controller.abort()
  }, [api, dealId])
  if (error) return <p role="alert">{error}</p>
  if (!deal) return <p>거래를 불러오고 있습니다.</p>
  return <section aria-labelledby="deal-title" className="deal-detail">
    <h2 id="deal-title">{deal.product.title}</h2>
    {deal.product.hidden ? <p>판매자가 숨긴 상품입니다. 거래 기록은 그대로 확인할 수 있습니다.</p> : null}
    <DealSummary deal={deal} />
    <DealActions deal={deal} onChange={setDeal} />
    <nav><a href={deal.viewerRole === 'SELLER' ? `/market/products/${deal.productId}/manage` : '/market/deals'}>{deal.viewerRole === 'SELLER' ? '상품 관리로' : '내 신청·거래로'}</a></nav>
  </section>
}

export function MyDeals() {
  const { api } = useSession()
  const [status, setStatus] = useState<DealStatus | ''>('')
  const [page, setPage] = useState<Page<Deal> | null>(null)
  const [error, setError] = useState<string | null>(null)
  async function load(cursor?: string) {
    const query = new URLSearchParams()
    if (status) query.set('status', status)
    if (cursor) query.set('cursor', cursor)
    setError(null)
    try {
      const next = await api<Page<Deal>>(`/me/deals?${query}`)
      setPage(previous => cursor && previous ? { items: [...previous.items, ...next.items], nextCursor: next.nextCursor } : next)
    } catch (e) { setError(describe(e)) }
  }
  useEffect(() => { void load() }, [status])
  return <section aria-labelledby="my-deals" className="my-deals">
    <h2 id="my-deals">내 신청·거래</h2>
    <label>상태 <select value={status} onChange={e => setStatus(e.target.value as DealStatus | '')}>
      <option value="">전체</option>
      {Object.entries(statusLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
    </select></label>
    {error ? <p role="alert">{error}</p> : null}
    {page && page.items.length === 0 ? <p>신청한 거래가 없습니다.</p> : null}
    <ul>{page?.items.map(deal => <li key={deal.id}>
      <a href={`/market/deals/${deal.id}`}>{deal.product.title}</a> · {statusLabels[deal.status]} · {deal.requestedQuantity}개
    </li>)}</ul>
    {page?.nextCursor ? <button type="button" onClick={() => void load(page.nextCursor!)}>더 보기</button> : null}
  </section>
}

export function ApplyForm({ productId }: { productId: string }) {
  const { api } = useSession()
  const [quantity, setQuantity] = useState('1')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [deal, setDeal] = useState<Deal | null>(null)
  async function submit(event: FormEvent) {
    event.preventDefault()
    setBusy(true); setError(null)
    try { setDeal(await api<Deal>(`/products/${productId}/deals`, { method: 'POST', body: { quantity: Number(quantity) } })) }
    catch (e) { setError(describe(e)) }
    finally { setBusy(false) }
  }
  if (deal) return <p role="status">신청했습니다. 판매자가 수락하기 전까지 익명으로 표시됩니다. <a href={`/market/deals/${deal.id}`}>신청 확인</a></p>
  return <form onSubmit={submit} className="apply-form" aria-label="거래 신청">
    <label>신청 수량 <input type="number" min={1} step={1} required value={quantity} onChange={e => setQuantity(e.target.value)} /></label>
    <button type="submit" disabled={busy}>거래 신청</button>
    <p>신청만으로는 수량이 확보되지 않습니다. 판매자가 수락하면 거래가 성사됩니다.</p>
    {error ? <p role="alert">{error}</p> : null}
  </form>
}
