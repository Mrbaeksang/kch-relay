import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { useCallback, useEffect, useState } from 'react'
import { useSession } from '../auth/session'
import { ProductForm } from './ProductForm'
import { statusLabel, when, won, type Limits, type Page, type ProductInput, type SellerProduct } from './types'

/** 판매자 화면: 상품 등록, 내 상품 상태 확인, 반려 상품 수정·재심사 요청. */
export function MyProducts() {
  const { api } = useSession()
  const [limits, setLimits] = useState<Limits | null>(null)
  const [products, setProducts] = useState<SellerProduct[]>([])
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const [editing, setEditing] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [notice, setNotice] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async (cursor?: string) => {
    setLoading(true); setError(null)
    try {
      const page = await api<Page<SellerProduct>>(`/me/products${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''}`)
      setProducts(current => cursor ? [...current, ...page.items] : page.items)
      setNextCursor(page.nextCursor)
    } catch (failure) { setError(failure instanceof Error ? failure.message : '내 상품을 불러오지 못했습니다.') } finally { setLoading(false) }
  }, [api])
  useEffect(() => {
    api<Limits>('/product-limits').then(setLimits).catch(() => setError('입력 기준을 불러오지 못했습니다.'))
    void load()
  }, [api, load])

  async function create(input: ProductInput) {
    await api<SellerProduct>('/products', { method: 'POST', body: input })
    setCreating(false); setNotice('등록했습니다. 인사팀 승인 후 장터에 공개됩니다.')
    await load()
  }
  async function resubmit(id: string, input: ProductInput) {
    await api<SellerProduct>(`/products/${encodeURIComponent(id)}/submit-review`, { method: 'POST', body: input })
    setEditing(null); setNotice('수정 내용으로 다시 심사를 요청했습니다.')
    await load()
  }

  return <section aria-labelledby="my-products-title" className="products-page">
    <h2 id="my-products-title">내 상품</h2>
    {error ? <p role="alert">{error}</p> : null}
    <p aria-live="polite">{notice}</p>
    {limits && !creating ? <Button type="button" onClick={() => { setCreating(true); setNotice('') }}>상품 등록</Button> : null}
    {limits && creating ? <section aria-label="상품 등록">
      <h3>새 상품</h3>
      <p>등록하면 인사팀 심사를 거쳐 장터에 공개됩니다. 심사 전에는 다른 직원에게 보이지 않습니다.</p>
      <ProductForm limits={limits} submitLabel="등록하고 심사 요청" onSubmit={create} onCancel={() => setCreating(false)} />
    </section> : null}
    {loading ? <p role="status">상품을 불러오고 있습니다.</p> : null}
    {error ? <Button type="button" variant="outline" onClick={()=>void load()}>다시 시도</Button> : null}
    {!loading && !error && products.length === 0 ? <p>등록한 상품이 없습니다.</p> : <ul className="product-list">
      {products.map(product => <li key={product.id}>
        <article aria-label={product.title}>
          <h3>{product.title} <span className="status">{statusLabel[product.approvalStatus]}</span></h3>
          <p>{won(product.unitPrice)} · 수량 {product.remainingQuantity} · 등록 {when(product.createdAt)}</p>
          {product.images.length ? <p className="photos">{product.images.map(image => <img key={image.id} src={image.url} alt="상품 사진" width={72} height={72} />)}</p> : null}
          {product.approvalStatus === 'REJECTED' ? <>
            <p role="note"><strong>반려 사유:</strong> {product.rejectionReason}</p>
            {editing === product.id && limits
              ? <ProductForm limits={limits} initial={product} submitLabel="수정하고 다시 심사 요청" onSubmit={input => resubmit(product.id, input)} onCancel={() => setEditing(null)} />
              : <Button type="button" onClick={() => { setEditing(product.id); setNotice('') }}>수정 후 다시 요청</Button>}
          </> : null}
          {product.approvalStatus === 'PENDING' ? <p>인사팀 심사를 기다리고 있습니다.</p> : null}
        </article>
      </li>)}
    </ul>}
    {nextCursor ? <Button type="button" disabled={loading} onClick={() => void load(nextCursor)}>더 보기</Button> : null}
  </section>
}
