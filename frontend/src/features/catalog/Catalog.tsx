import { ApplyForm } from "@/transactions/deals"
import { useEffect, useState } from 'react'
import { ArrowLeft, ArrowRight, Search, RefreshCw } from 'lucide-react'
import { useSession } from '@/auth/session'
import { APIError } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'

export type Product = {
  id: string
  isOwn?: boolean
  approvalStatus?: 'PENDING' | 'APPROVED' | 'REJECTED'
  title: string
  description: string
  unitPrice: number
  remainingQuantity: number
  recruitmentClosed: boolean
  canApply: boolean
  seller: { label: string }
  images: { id: string; url: string; sortOrder: number }[]
  createdAt: string
}
type Page = { items: Product[]; nextCursor: string | null }
type Resource<T> =
  | { status: 'loading' }
  | { status: 'ready'; data: T }
  | { status: 'error'; message: string; notFound: boolean }
const money = new Intl.NumberFormat('ko-KR')
export function price(value: number) {
  return value === 0 ? '나눔 · 0원' : `${money.format(value)}원`
}
function status(p: Product) {
  if (p.approvalStatus === 'PENDING') return '승인 대기'
  if (p.approvalStatus === 'REJECTED') return '반려'
  return p.recruitmentClosed
    ? '모집 종료'
    : p.remainingQuantity === 0
      ? '수량 소진'
      : '판매 중'
}
function ProductStatus({ product }: { product: Product }) {
  return (
    <Badge
      variant="secondary"
      className={
        product.approvalStatus === 'PENDING' || product.approvalStatus === 'REJECTED' || product.recruitmentClosed || product.remainingQuantity === 0
          ? ''
          : 'bg-success-surface text-success'
      }
    >
      {status(product)}
    </Badge>
  )
}
function useResource<T>(path: string, revision: number) {
  const { api } = useSession()
  const [resource, setResource] = useState<Resource<T>>({ status: 'loading' })
  useEffect(() => {
    const controller = new AbortController()
    setResource({ status: 'loading' })
    void api<T>(path, { signal: controller.signal })
      .then((data) => {
        if (!controller.signal.aborted) setResource({ status: 'ready', data })
      })
      .catch((error: unknown) => {
        if (!controller.signal.aborted)
          setResource({
            status: 'error',
            message:
              error instanceof APIError
                ? error.message
                : '연결을 확인하고 다시 시도해 주세요.',
            notFound: error instanceof APIError && error.status === 404,
          })
      })
    return () => controller.abort()
  }, [api, path, revision])
  return resource
}
function ErrorState({
  message,
  retry,
}: {
  message: string
  retry: () => void
}) {
  return (
    <div className="py-12" role="alert">
      <p className="font-medium">불러오지 못했습니다</p>
      <p className="mt-2 text-muted-foreground">{message}</p>
      <Button type="button" variant="outline" className="mt-4" onClick={retry}>
        <RefreshCw aria-hidden="true" />
        다시 시도
      </Button>
    </div>
  )
}
function Photo({
  image,
  title,
  className = '',
}: {
  image: Product['images'][number]
  title: string
  className?: string
}) {
  const [failed, setFailed] = useState(false)
  if (failed)
    return (
      <div
        className={`flex items-center justify-center bg-muted p-4 text-sm text-muted-foreground ${className}`}
      >
        사진을 불러올 수 없습니다
      </div>
    )
  return (
    <div className={`relative overflow-hidden ${className}`}>
      <img
        src={image.url}
        alt={title}
        className="absolute inset-0 h-full w-full object-contain p-3"
        loading="lazy"
        onError={() => setFailed(true)}
      />
    </div>
  )
}

export function CatalogList() {
  const initial = new URLSearchParams(window.location.search).get('q') ?? ''
  const [draft, setDraft] = useState(initial)
  const [query, setQuery] = useState(initial)
  const [cursors, setCursors] = useState<(string | null)[]>([null])
  const [revision, setRevision] = useState(0)
  const params = new URLSearchParams({ limit: '20' })
  if (query) params.set('q', query)
  const cursor = cursors[cursors.length - 1]
  if (cursor) params.set('cursor', cursor)
  const resource = useResource<Page>(`/products?${params}`, revision)
  function search(event: React.FormEvent) {
    event.preventDefault()
    setQuery(draft.trim())
    setCursors([null])
    setRevision((v) => v + 1)
    const url = new URL(window.location.href)
    if (draft.trim()) url.searchParams.set('q', draft.trim())
    else url.searchParams.delete('q')
    window.history.replaceState(null, '', url)
  }
  return (
    <section aria-labelledby="market-title" className="py-8 sm:py-10">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 id="market-title">우리 사이의 다음 쓰임.</h1>
          <p className="mt-3 text-muted-foreground">
            물건도, 재능도. 동료에게 이어지는 사내 장터.
          </p>
        </div>
        <p className="text-sm text-muted-foreground">
          수락 전까지 서로 익명으로 거래해요.
        </p>
      </div>
      <form
        role="search"
        onSubmit={search}
        className="my-8 flex max-w-xl items-end gap-2"
      >
        <div className="min-w-0 flex-1 space-y-2">
          <Label htmlFor="market-search">상품 검색</Label>
          <Input
            id="market-search"
            type="search"
            maxLength={200}
            placeholder="제목이나 설명으로 찾아보세요"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
          />
        </div>
        <Button type="submit">
          <Search aria-hidden="true" />
          검색
        </Button>
      </form>
      {resource.status === 'loading' ? (
        <div role="status">
          <p className="mb-4 text-sm text-muted-foreground">
            상품을 불러오고 있습니다.
          </p>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="aspect-[4/3] rounded-xl" />
            ))}
          </div>
        </div>
      ) : null}
      {resource.status === 'error' ? (
        <ErrorState
          message={resource.message}
          retry={() => setRevision((v) => v + 1)}
        />
      ) : null}
      {resource.status === 'ready' ? (
        <>
          <p role="status" className="mb-4 text-sm text-muted-foreground">
            {query ? `“${query}” 검색 결과` : '최근 등록된 상품'} ·{' '}
            {resource.data.items.length}개
            {cursors.length > 1 ? ` · ${cursors.length}페이지` : ''}
          </p>
          {resource.data.items.length === 0 ? (
            <div className="py-12">
              <h2>
                {query
                  ? '검색한 상품이 없습니다'
                  : '아직 공개된 상품이 없습니다'}
              </h2>
              <p className="mt-2 text-muted-foreground">
                {query
                  ? '다른 검색어로 다시 찾아보세요.'
                  : '인사팀 승인을 받은 상품이 여기에 표시됩니다.'}
              </p>
            </div>
          ) : (
            <ul className="grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 sm:gap-x-6 lg:grid-cols-4">
              {resource.data.items.map((product) => (
                <li key={product.id} className="min-w-0">
                  <a
                    href={`/products/${encodeURIComponent(product.id)}`}
                    className="group block rounded-xl focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring"
                  >
                    {product.images[0] ? (
                      <Photo
                        key={product.images[0].id}
                        image={product.images[0]}
                        title=""
                        className="aspect-[4/3] w-full rounded-xl bg-muted"
                      />
                    ) : (
                      <div className="flex aspect-[4/3] flex-col justify-center gap-2 rounded-xl bg-muted p-4">
                        <span className="text-xs text-muted-foreground">
                          사진 없이 등록된 상품
                        </span>
                        <p className="line-clamp-3 text-sm text-muted-foreground">
                          {product.description}
                        </p>
                      </div>
                    )}
                    <div className="mt-3">
                      <ProductStatus product={product} />
                      <h2 className="mt-2 line-clamp-2 text-base font-medium leading-relaxed group-hover:underline">
                        {product.title}
                      </h2>
                      <p className="mt-1 font-semibold tabular-nums">
                        {price(product.unitPrice)}
                      </p>
                      <p className="mt-1 text-sm text-muted-foreground">
                        {product.isOwn ? '내 상품' : product.seller.label} · 남은 수량{' '}
                        {money.format(product.remainingQuantity)}
                      </p>
                    </div>
                  </a>
                </li>
              ))}
            </ul>
          )}
          <nav
            aria-label="상품 페이지"
            className="mt-10 flex justify-between gap-4"
          >
            <Button
              type="button"
              variant="outline"
              disabled={cursors.length === 1}
              onClick={() => setCursors((c) => c.slice(0, -1))}
            >
              <ArrowLeft aria-hidden="true" />
              이전 상품
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={!resource.data.nextCursor}
              onClick={() =>
                setCursors((c) => [...c, resource.data.nextCursor])
              }
            >
              다음 상품
              <ArrowRight aria-hidden="true" />
            </Button>
          </nav>
        </>
      ) : null}
    </section>
  )
}

export function ProductDetail({ productId }: { productId: string }) {
  const [revision, setRevision] = useState(0)
  const resource = useResource<Product>(
    `/products/${encodeURIComponent(productId)}`,
    revision,
  )
  return (
    <section className="py-8 sm:py-10">
      <a
        href="/market"
        className="mb-8 inline-flex min-h-11 items-center gap-2 text-sm underline"
      >
        <ArrowLeft aria-hidden="true" className="size-4" />
        장터로 돌아가기
      </a>
      {resource.status === 'loading' ? (
        <div role="status">
          <p>상품을 불러오고 있습니다.</p>
          <Skeleton className="mt-4 h-48 w-full rounded-xl" />
        </div>
      ) : null}
      {resource.status === 'error' ? (
        resource.notFound ? (
          <div>
            <h1>상품을 찾을 수 없습니다</h1>
            <p className="mt-3 text-muted-foreground">
              공개되지 않았거나 숨겨진 상품일 수 있습니다.
            </p>
          </div>
        ) : (
          <ErrorState
            message={resource.message}
            retry={() => setRevision((v) => v + 1)}
          />
        )
      ) : null}
      {resource.status === 'ready' ? (
        <article
          className={
            resource.data.images.length
              ? 'grid gap-8 lg:grid-cols-2 lg:gap-12'
              : 'max-w-3xl'
          }
        >
          {resource.data.images.length ? (
            <div className="space-y-4">
              {resource.data.images.map((image, i) => (
                <Photo
                  key={image.id}
                  image={image}
                  title={`${resource.data.title} 사진 ${i + 1}`}
                  className="aspect-[4/3] max-h-[70vh] w-full rounded-xl bg-muted"
                />
              ))}
            </div>
          ) : null}
          <div>
            <ProductStatus product={resource.data} />
            <h1 className="mt-4">{resource.data.title}</h1>
            <p className="mt-4 text-2xl font-semibold tabular-nums">
              {price(resource.data.unitPrice)}
            </p>
            <dl className="mt-6 grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-sm">
              <dt className="text-muted-foreground">판매자</dt>
              <dd>{resource.data.isOwn ? '내 상품' : resource.data.seller.label}</dd>
              <dt className="text-muted-foreground">남은 수량</dt>
              <dd className="tabular-nums">
                {money.format(resource.data.remainingQuantity)}
              </dd>
              <dt className="text-muted-foreground">등록일</dt>
              <dd>
                <time dateTime={resource.data.createdAt}>
                  {new Intl.DateTimeFormat('ko-KR', {
                    timeZone: 'Asia/Seoul',
                    dateStyle: 'medium',
                  }).format(new Date(resource.data.createdAt))}
                </time>
              </dd>
            </dl>
            <div className="mt-8 border-t pt-6">
              <h2 className="text-lg">상품 설명</h2>
              <p className="mt-3 whitespace-pre-wrap leading-7">
                {resource.data.description}
              </p>
            </div>
            {resource.data.isOwn ? <div className="mt-6 space-y-3"><Button asChild variant="outline"><a href={`/market/products/${encodeURIComponent(resource.data.id)}/manage`}>내 상품 관리</a></Button>{resource.data.approvalStatus !== 'APPROVED' ? <p className="text-sm text-muted-foreground">인사팀 승인 전에는 본인에게만 보입니다.</p> : null}</div> : null}
            {resource.data.canApply ? <div className="mt-8"><ApplyForm productId={resource.data.id} /></div> : null}
              <div className="mt-8 rounded-lg bg-muted p-4 text-sm text-muted-foreground">
              <p>
                {resource.data.canApply
                  ? '신청만으로는 수량이 확보되지 않습니다.'
                  : '현재 거래 신청할 수 없는 상품입니다.'}
              </p>
              <p className="mt-2">
                거래 수락 전에는 서로의 신원이 공개되지 않습니다.
              </p>
            </div>
          </div>
        </article>
      ) : null}
    </section>
  )
}
