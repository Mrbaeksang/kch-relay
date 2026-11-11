import { useState, type FormEvent } from 'react'
import { useSession } from '../auth/session'
import { APIError } from '../lib/api'
import { prepareImage, validateProduct, type Limits, type ProductImage, type ProductInput, type SellerProduct } from './types'

type Props = {
  limits: Limits
  initial?: SellerProduct
  submitLabel: string
  onSubmit: (input: ProductInput) => Promise<void>
  onCancel?: () => void
}

export function ProductForm({ limits, initial, submitLabel, onSubmit, onCancel }: Props) {
  const { api } = useSession()
  const [title, setTitle] = useState(initial?.title ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [unitPrice, setUnitPrice] = useState(initial ? String(initial.unitPrice) : '')
  const [quantity, setQuantity] = useState(initial ? String(initial.remainingQuantity) : '1')
  const [images, setImages] = useState<ProductImage[]>(initial?.images ?? [])
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const megabytes = Math.floor(limits.maxImageBytes / (1 << 20))

  async function addPhotos(files: FileList | null) {
    if (!files?.length) return
    setError(null)
    if (images.length + files.length > limits.maxImagesPerProduct) { setError(`사진은 최대 ${limits.maxImagesPerProduct}장까지 등록할 수 있습니다.`); return }
    setBusy(true)
    try {
      const added: ProductImage[] = []
      for (const file of Array.from(files)) {
        if (!limits.imageTypes.includes(file.type)) throw new Error('JPEG 또는 PNG 사진만 올릴 수 있습니다.')
        const prepared = await prepareImage(file)
        if (prepared.size > limits.maxImageBytes) throw new Error(`사진은 ${megabytes}MB 이하만 올릴 수 있습니다.`)
        const form = new FormData()
        form.append('file', prepared, 'photo')
        added.push(await api<ProductImage>('/images', { method: 'POST', body: form }))
      }
      setImages(current => [...current, ...added])
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : '사진을 올리지 못했습니다.')
    } finally { setBusy(false) }
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    const problem = validateProduct({ title, description, unitPrice, quantity, imageCount: images.length }, limits)
    if (problem) { setError(problem); return }
    setBusy(true); setError(null)
    try {
      await onSubmit({ title: title.trim(), description: description.trim(), unitPrice: Number(unitPrice), quantity: Number(quantity), imageIds: images.map(image => image.id) })
    } catch (failure) {
      setError(failure instanceof APIError ? failure.message : '저장하지 못했습니다. 다시 시도해 주세요.')
    } finally { setBusy(false) }
  }

  return <form className="product-form" onSubmit={submit} noValidate>
    <label>제목 <input value={title} maxLength={limits.maxTitleLength} onChange={event => setTitle(event.target.value)} /></label>
    <small>{[...title].length}/{limits.maxTitleLength}자</small>
    <label>본문 <textarea value={description} rows={6} maxLength={limits.maxDescriptionLength} onChange={event => setDescription(event.target.value)} /></label>
    <small>수량 단위(예: 한 박스, 1회 1시간)를 본문에 적어 주세요. {[...description].length}/{limits.maxDescriptionLength}자</small>
    <label>단가 (원, 1개 기준) <input inputMode="numeric" value={unitPrice} onChange={event => setUnitPrice(event.target.value)} /></label>
    <small>0원이면 나눔입니다. 최대 {limits.maxUnitPrice.toLocaleString('ko-KR')}원</small>
    <label>수량 <input inputMode="numeric" value={quantity} onChange={event => setQuantity(event.target.value)} /></label>
    <small>{limits.minQuantity}~{limits.maxQuantity.toLocaleString('ko-KR')}개</small>
    <fieldset>
      <legend>사진 (선택, 최대 {limits.maxImagesPerProduct}장 · JPEG/PNG · 장당 {megabytes}MB 이하)</legend>
      <ul className="photo-list">
        {images.map(image => <li key={image.id}>
          <img src={image.url} alt="상품 사진" width={96} height={96} />
          <button type="button" onClick={() => setImages(current => current.filter(item => item.id !== image.id))}>사진 빼기</button>
        </li>)}
      </ul>
      <label>사진 추가 <input type="file" accept={limits.imageTypes.join(',')} multiple disabled={busy || images.length >= limits.maxImagesPerProduct} onChange={event => { void addPhotos(event.target.files); event.target.value = '' }} /></label>
    </fieldset>
    {error ? <p role="alert">{error}</p> : null}
    <div className="actions">
      <button type="submit" disabled={busy}>{submitLabel}</button>
      {onCancel ? <button type="button" onClick={onCancel} disabled={busy}>취소</button> : null}
    </div>
  </form>
}
