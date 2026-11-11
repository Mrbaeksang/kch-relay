import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { useSession } from '../auth/session'
import { APIError } from '../lib/api'
import { statusLabel, when, won, type AdminProduct, type AdminProductDetail, type ApprovalStatus, type Limits, type Page, type Participant } from './types'

/** 인사팀 화면: 심사 목록, 상세(작성자 신원·이력), 승인·반려, 참여자 신원. */
export function AdminReview() {
  const { api } = useSession()
  const [status, setStatus] = useState<ApprovalStatus>('PENDING')
  const [list, setList] = useState<AdminProduct[]>([])
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [selected, setSelected] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async (filter: ApprovalStatus, cursor?: string) => {
    setError(null); setLoading(true)
    try {
      const page = await api<Page<AdminProduct>>(`/admin/products?approvalStatus=${filter}${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`)
      setList(current => cursor ? [...current, ...page.items] : page.items)
      setNextCursor(page.nextCursor)
    } catch (failure) { setError(failure instanceof Error ? failure.message : '심사 목록을 불러오지 못했습니다.') } finally {setLoading(false)}
  }, [api])
  useEffect(() => { void load(status) }, [load, status])

  return <section aria-labelledby="review-title" className="products-page">
    <h2 id="review-title">상품 심사</h2>
    <div role="group" aria-label="심사 상태">
      {(['PENDING', 'REJECTED', 'APPROVED'] as const).map(value => <Button key={value} type="button" aria-pressed={status === value} onClick={() => { setStatus(value); setSelected(null) }}>{statusLabel[value]}</Button>)}
    </div>
    {error ? <p role="alert">{error}</p> : null}
    {loading ? <p role="status">심사 목록을 불러오고 있습니다.</p> : null}
    {error ? <Button type="button" variant="outline" onClick={()=>void load(status)}>다시 시도</Button> : null}
    {!loading && !error && list.length === 0 ? <p>{statusLabel[status]} 상품이 없습니다.</p> : <table>
      <thead><tr><th>제목</th><th>단가</th><th>수량</th><th>작성자</th><th>요청 시각</th><th></th></tr></thead>
      <tbody>{list.map(item => <tr key={item.id}>
        <td>{item.title}</td><td>{won(item.unitPrice)}</td><td>{item.remainingQuantity}</td>
        <td>{item.seller.realName} · {item.seller.department} · {item.seller.jobLevel}</td>
        <td>{when(item.updatedAt)}</td>
        <td><Button type="button" onClick={() => setSelected(item.id)}>{item.title} 상세</Button></td>
      </tr>)}</tbody>
    </table>}
    {nextCursor ? <Button type="button" disabled={loading} onClick={() => void load(status, nextCursor)}>더 보기</Button> : null}
    {selected ? <ReviewDetail key={selected} id={selected} onDecided={() => { setSelected(null); void load(status) }} /> : null}
  </section>
}

function ReviewDetail({ id, onDecided }: { id: string; onDecided: () => void }) {
  const { api } = useSession()
  const [detail, setDetail] = useState<AdminProductDetail | null>(null)
  const [participants, setParticipants] = useState<Participant[] | null>(null)
  const [limits, setLimits] = useState<Limits | null>(null)
  const [reason, setReason] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const path = `/admin/products/${encodeURIComponent(id)}`

  useEffect(() => {
    api<AdminProductDetail>(path).then(setDetail).catch((failure: unknown) => setError(failure instanceof Error ? failure.message : '상품을 불러오지 못했습니다.'))
    api<Limits>('/product-limits').then(setLimits).catch(() => undefined)
  }, [api, path])

  async function decide(action: 'approve' | 'reject', event?: FormEvent) {
    event?.preventDefault()
    if (action === 'reject') {
      if (!reason.trim()) { setError('반려 사유을(를) 입력해 주세요.'); return }
      if (limits && [...reason.trim()].length > limits.maxReasonLength) { setError(`반려 사유은(는) ${limits.maxReasonLength}자 이하로 입력해 주세요.`); return }
    }
    setBusy(true); setError(null)
    try {
      await api(`${path}/${action}`, { method: 'POST', body: action === 'reject' ? { reason: reason.trim() } : {} })
      onDecided()
    } catch (failure) { setError(failure instanceof APIError ? failure.message : '처리하지 못했습니다.') }
    finally { setBusy(false) }
  }

  if (!detail) return error ? <p role="alert">{error}</p> : <p>불러오는 중입니다.</p>
  return <article aria-label={`${detail.title} 심사`} className="review-detail">
    <h3>{detail.title} <span className="status">{statusLabel[detail.approvalStatus]}</span></h3>
    <p>작성자: {detail.seller.realName} · {detail.seller.department} · {detail.seller.jobLevel}</p>
    <p>{won(detail.unitPrice)} · 수량 {detail.remainingQuantity}</p>
    <p className="description">{detail.description}</p>
    {detail.images.length ? <p className="photos">{detail.images.map(image => <img key={image.id} src={image.url} alt="상품 사진" width={160} height={160} />)}</p> : <p>사진 없음</p>}
    <h4>심사 이력</h4>
    {detail.reviews.length === 0 ? <p>아직 심사 이력이 없습니다.</p> : <ol>
      {detail.reviews.map((review, index) => <li key={index}>{when(review.createdAt)} · {review.decision === 'APPROVED' ? '승인' : '반려'} · {review.reviewer}{review.reason ? ` — ${review.reason}` : ''}</li>)}
    </ol>}
    {detail.approvalStatus === 'PENDING' ? <form onSubmit={event => void decide('reject', event)}>
      <Button type="button" disabled={busy} onClick={() => void decide('approve')}>승인</Button>
      <label>반려 사유 <Textarea value={reason} rows={3} maxLength={limits?.maxReasonLength} onChange={event => setReason(event.target.value)} /></label>
      <Button type="submit" disabled={busy}>반려</Button>
    </form> : null}
    {error ? <p role="alert">{error}</p> : null}
    <h4>참여자 신원</h4>
    {participants === null
      ? <Button type="button" onClick={() => api<{ items: Participant[] }>(`${path}/participants`).then(data => setParticipants(data.items)).catch((failure: unknown) => setError(failure instanceof Error ? failure.message : '참여자를 불러오지 못했습니다.'))}>참여자 보기</Button>
      : participants.length === 0 ? <p>문의·신청한 사람이 없습니다.</p>
        : <ul>{participants.map(person => <li key={person.label}>{person.label}: {person.realName} · {person.department} · {person.jobLevel}</li>)}</ul>}
  </article>
}
