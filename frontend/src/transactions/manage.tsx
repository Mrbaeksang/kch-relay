import { PhotoEditor } from "./PhotoEditor"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { useSession } from '../auth/session'
import { DealActions, DealSummary } from './deals'
import { describe, won, type Deal, type ManagedProduct, type Page } from './model'

type SellerPage = Page<Deal> & { product: ManagedProduct }

export function ProductManage({ productId }: { productId: string }) {
  const { api } = useSession()
  const [data, setData] = useState<SellerPage | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const load = useCallback(async (cursor?:string) => {
    try { const next = await api<SellerPage>(`/products/${productId}/deals${cursor?`?cursor=${encodeURIComponent(cursor)}`:""}`);setData(old=>cursor&&old?{...next,items:[...old.items,...next.items]}:next) } catch (e) { setError(describe(e)) }
  }, [api, productId])
  useEffect(() => { void load() }, [load])

  async function change(request: () => Promise<unknown>, done: string) {
    setBusy(true); setError(null); setNotice(null)
    try { await request(); await load(); setNotice(done);return true } catch (e) { setError(describe(e));return false } finally { setBusy(false) }
  }
  function replaceDeal(deal: Deal) { void load(); setData(d => d && { ...d, items: d.items.map(x => x.id === deal.id ? deal : x) }) }

  if (!data) return error ? <div><p role="alert">{error}</p><Button type="button" onClick={()=>void load()}>다시 시도</Button></div> : <p>상품을 불러오고 있습니다.</p>
  const { product } = data
  const closed = product.recruitmentClosed
  return <section aria-labelledby="manage-title" className="product-manage">
    <h1 id="manage-title">상품 관리 · {product.title}</h1>
    <p>{won(product.unitPrice)} · 잔여 {product.remainingQuantity}개 · {closed ? '모집 종료' : '모집 중'}{product.hidden ? ' · 숨김' : ''} · 대기 신청 {product.pendingApplicationCount}건</p>
    {notice ? <p role="status">{notice}</p> : null}
    {error ? <p role="alert">{error}</p> : null}
    {product.hidden ? <p>숨긴 상품은 더 이상 수정할 수 없습니다. 성사된 거래의 완료 처리는 할 수 있습니다.</p> : <>
      <EditForm product={product} busy={busy} onSubmit={body => change(() => api(`/products/${productId}`, { method: 'PATCH', body }), '상품을 수정했습니다.')} />
      <QuantityForm product={product} busy={busy} onSubmit={remainingQuantity => change(() => api(`/products/${productId}/quantity`, { method: 'PATCH', body: { remainingQuantity } }), '잔여 수량을 바꿨습니다. 대기 중인 신청은 그대로 유지됩니다.')} />
      <div className="product-controls">
        <Button type="button" disabled={busy} onClick={() => void change(() => api(`/products/${productId}/${closed ? 'reopen' : 'close'}`, { method: 'POST' }), closed ? '모집을 다시 시작했습니다.' : '모집을 종료했습니다. 기존 신청과 거래는 유지됩니다.')}>{closed ? '모집 재개' : '모집 종료'}</Button>
        <Button type="button" disabled={busy} onClick={() => {
          if (window.confirm('상품을 숨기면 장터에서 사라지고 대기 중인 신청은 모두 거절됩니다. 숨길까요?')) void change(() => api(`/products/${productId}`, { method: 'DELETE' }), '상품을 숨겼습니다.')
        }}>상품 숨기기</Button>
      </div>
    </>}
    <h3>신청·거래</h3>
    {data.items.length === 0 ? <p>아직 신청이 없습니다.</p> : null}
    <ul className="seller-deals">{data.items.map(deal => <li key={deal.id}>
      <DealSummary deal={deal} />
      <DealActions deal={deal} onChange={replaceDeal} />
      <a href={`/market/deals/${deal.id}`}>상세 보기</a>
    </li>)}</ul>
  </section>
}

type Edit = { title: string; description: string; unitPrice: number; rejectPendingApplications: boolean; imageIds?:string[] }

function EditForm({ product, busy, onSubmit }: { product: ManagedProduct; busy: boolean; onSubmit: (body: Edit) => Promise<unknown> }) {
  const [title, setTitle] = useState(product.title)
  const [description, setDescription] = useState(product.description)
  const [unitPrice, setUnitPrice] = useState(String(product.unitPrice))
  const [imageIds,setImageIds]=useState(product.imageIds)
  const [uploading,setUploading]=useState(false)
  const [reject, setReject] = useState(false)
  function submit(event: FormEvent) {
    event.preventDefault()
    void onSubmit({ title, description, unitPrice: Number(unitPrice), rejectPendingApplications: reject, ...(JSON.stringify(imageIds)!==JSON.stringify(product.imageIds)?{imageIds}: {}) }).then(saved => {if(saved)setReject(false)})
  }
  return <form onSubmit={submit} aria-label="상품 내용 수정">
    <label>제목 <Input required value={title} onChange={e => setTitle(e.target.value)} /></label>
    <label>내용 <Textarea required value={description} onChange={e => setDescription(e.target.value)} /></label>
    <label>단가(원) <Input type="number" min={0} step={1} required value={unitPrice} onChange={e => setUnitPrice(e.target.value)} /></label>
    <PhotoEditor ids={imageIds} onChange={setImageIds} busy={busy||uploading} onBusy={setUploading} />
    {product.pendingApplicationCount > 0 ? <label>
      <input className="size-5 accent-primary" type="checkbox" checked={reject} onChange={e => setReject(e.target.checked)} />
      대기 중인 신청 {product.pendingApplicationCount}건을 모두 거절하고 수정합니다. 신청자에게 거절 알림이 갑니다.
    </label> : null}
    <p>이미 성사된 거래의 확정 단가·수량은 바뀌지 않습니다.</p>
    <Button type="submit" disabled={busy||uploading}>내용 저장</Button>
  </form>
}

function QuantityForm({ product, busy, onSubmit }: { product: ManagedProduct; busy: boolean; onSubmit: (quantity: number) => Promise<unknown> }) {
  const [quantity, setQuantity] = useState(String(product.remainingQuantity))
  useEffect(() => setQuantity(String(product.remainingQuantity)), [product.remainingQuantity])
  return <form onSubmit={e => { e.preventDefault(); void onSubmit(Number(quantity)) }} aria-label="잔여 수량 변경">
    <label>잔여 수량 <Input type="number" min={0} step={1} required value={quantity} onChange={e => setQuantity(e.target.value)} /></label>
    <Button type="submit" disabled={busy}>수량 저장</Button>
  </form>
}
