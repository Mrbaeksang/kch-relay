import { Button } from "@/components/ui/button"
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { DetailSection } from '@/components/layout/DetailSection'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table'
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
    <h1 id="review-title">상품 심사</h1>
    <div role="group" aria-label="심사 상태">
      {(['PENDING', 'REJECTED', 'APPROVED'] as const).map(value => <Button variant={status === value ? "secondary" : "ghost"} key={value} type="button" aria-pressed={status === value} onClick={() => { setStatus(value); setSelected(null) }}>{statusLabel[value]}</Button>)}
    </div>
    {error ? <p role="alert">{error}</p> : null}
    {loading ? <p role="status">심사 목록을 불러오고 있습니다.</p> : null}
    {error ? <Button type="button" variant="outline" onClick={()=>void load(status)}>다시 시도</Button> : null}
    {!loading && !error && list.length === 0 ? <p>{statusLabel[status]} 상품이 없습니다.</p> : <Table className="min-w-[720px]">
      <TableHeader><TableRow><TableHead>제목</TableHead><TableHead>단가</TableHead><TableHead>수량</TableHead><TableHead>작성자</TableHead><TableHead>요청 시각</TableHead><TableHead></TableHead></TableRow></TableHeader>
      <TableBody>{list.map(item => <TableRow key={item.id}>
        <TableCell className="whitespace-normal">{item.title}</TableCell><TableCell className="whitespace-normal">{won(item.unitPrice)}</TableCell><TableCell className="whitespace-normal">{item.remainingQuantity}</TableCell>
        <TableCell className="whitespace-normal">{item.seller.realName} · {item.seller.department} · {item.seller.jobLevel}</TableCell>
        <TableCell className="whitespace-normal">{when(item.updatedAt)}</TableCell>
        <TableCell className="whitespace-normal"><Button type="button" onClick={() => setSelected(item.id)}>{item.title} 상세</Button></TableCell>
      </TableRow>)}</TableBody>
    </Table>}
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
  return <article aria-label={`${detail.title} 심사`} className="review-detail space-y-6">
    <header className="flex flex-wrap items-start justify-between gap-4">
      <div className="space-y-2"><p className="text-sm text-muted-foreground">선택한 상품</p><h2 className="text-2xl font-semibold leading-snug">{detail.title}</h2></div>
      <Badge variant={detail.approvalStatus === 'REJECTED' ? 'destructive' : 'secondary'}>{statusLabel[detail.approvalStatus]}</Badge>
    </header>
    <div className="grid items-start gap-6 lg:grid-cols-2">
      <DetailSection title="상품 정보">
        <dl className="grid grid-cols-[72px_minmax(0,1fr)] gap-x-4 gap-y-3">
          <dt className="text-muted-foreground">작성자</dt><dd><span className="block font-medium">{detail.seller.realName}</span><span className="text-muted-foreground">{detail.seller.department} · {detail.seller.jobLevel}</span></dd>
          <dt className="text-muted-foreground">단가</dt><dd className="font-semibold tabular-nums">{won(detail.unitPrice)}</dd>
          <dt className="text-muted-foreground">남은 수량</dt><dd className="tabular-nums">{detail.remainingQuantity}</dd>
        </dl>
        <div className="border-t pt-4"><h4 className="mb-2 font-semibold">상품 설명</h4><p className="whitespace-pre-wrap">{detail.description}</p></div>
        {detail.images.length ? <div className="grid grid-cols-2 gap-3">{detail.images.map((image,index) => <img className="aspect-square w-full rounded-md bg-muted object-contain p-2" key={image.id} src={image.url} alt={`${detail.title} 사진 ${index+1}`} />)}</div> : <p className="rounded-md bg-muted px-4 py-3 text-muted-foreground">등록된 사진이 없습니다.</p>}
      </DetailSection>
      <div className="space-y-6">
        <DetailSection title="심사 이력">
          {detail.reviews.length === 0 ? <p className="text-muted-foreground">아직 심사 이력이 없습니다.</p> : <ol className="divide-y">{detail.reviews.map((review,index)=><li key={index} className="space-y-2 py-3 first:pt-0 last:pb-0"><div className="flex flex-wrap items-center gap-2"><Badge variant={review.decision === 'APPROVED' ? 'secondary' : 'destructive'}>{review.decision === 'APPROVED' ? '승인' : '반려'}</Badge><span className="font-medium">{review.reviewer}</span></div><time className="block text-xs text-muted-foreground" dateTime={review.createdAt}>{when(review.createdAt)}</time>{review.reason ? <p className="whitespace-pre-wrap">{review.reason}</p>:null}</li>)}</ol>}
        </DetailSection>
        <DetailSection title="참여자 신원">
          <p className="text-muted-foreground">인사팀만 확인할 수 있는 정보입니다.</p>
          {participants === null ? <Button type="button" variant="outline" onClick={() => api<{items:Participant[]}>(`${path}/participants`).then(data=>setParticipants(data.items)).catch((failure:unknown)=>setError(failure instanceof Error?failure.message:'참여자를 불러오지 못했습니다.'))}>참여자 보기</Button> : participants.length === 0 ? <p className="text-muted-foreground">문의·신청한 사람이 없습니다.</p> : <ul className="divide-y">{participants.map(person=><li className="space-y-1 py-3 first:pt-0 last:pb-0" key={person.label}><p className="font-medium">{person.label} · {person.realName}</p><p className="text-muted-foreground">{person.department} · {person.jobLevel}</p></li>)}</ul>}
        </DetailSection>
      </div>
    </div>
    {detail.approvalStatus === 'PENDING' ? <DetailSection title="심사 처리"><div className="space-y-5"><Button type="button" disabled={busy} onClick={()=>void decide('approve')}>승인</Button><form className="space-y-3 border-t pt-5" onSubmit={event=>void decide('reject',event)}><Label htmlFor="review-reason">반려 사유</Label><Textarea id="review-reason" value={reason} rows={3} maxLength={limits?.maxReasonLength} onChange={event=>setReason(event.target.value)}/><Button variant="destructive" type="submit" disabled={busy}>반려</Button></form></div></DetailSection>:null}
    {error ? <p role="alert" className="text-destructive">{error}</p>:null}
  </article>
}
