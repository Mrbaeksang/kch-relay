export type Limits = {
  maxTitleLength: number; maxDescriptionLength: number
  minUnitPrice: number; maxUnitPrice: number; minQuantity: number; maxQuantity: number
  maxImagesPerProduct: number; maxImageBytes: number; imageTypes: string[]; maxReasonLength: number
}
export type ApprovalStatus = 'PENDING' | 'APPROVED' | 'REJECTED'
export type ProductImage = { id: string; url: string }
export type SellerProduct = {
  id: string; title: string; description: string; unitPrice: number; remainingQuantity: number
  approvalStatus: ApprovalStatus; rejectionReason: string | null; recruitmentClosed: boolean
  images: ProductImage[]; createdAt: string; updatedAt: string
}
export type Identity = { realName: string; department: string; jobLevel: string }
export type AdminProduct = { id: string; title: string; unitPrice: number; remainingQuantity: number; approvalStatus: ApprovalStatus; seller: Identity; createdAt: string; updatedAt: string }
export type Review = { decision: 'APPROVED' | 'REJECTED'; reason: string | null; reviewer: string; createdAt: string }
export type AdminProductDetail = AdminProduct & { description: string; images: ProductImage[]; reviews: Review[] }
export type Participant = Identity & { label: string; inquirerNumber: number | null; applicantNumber: number | null; createdAt: string }
export type Page<T> = { items: T[]; nextCursor: string | null }
export type ProductInput = { title: string; description: string; unitPrice: number; quantity: number; imageIds: string[] }

export const statusLabel: Record<ApprovalStatus, string> = { PENDING: '심사 대기', APPROVED: '승인됨', REJECTED: '반려됨' }
export function won(value: number) { return value === 0 ? '나눔 (0원)' : `${value.toLocaleString('ko-KR')}원` }
export function when(iso: string) { return new Date(iso).toLocaleString('ko-KR', { dateStyle: 'medium', timeStyle: 'short' }) }

/** Mirrors the server rules so people see the same message before sending. */
export function validateProduct(input: { title: string; description: string; unitPrice: string; quantity: string; imageCount: number }, limits: Limits): string | null {
  const title = input.title.trim(), description = input.description.trim()
  if (!title) return '제목을(를) 입력해 주세요.'
  if ([...title].length > limits.maxTitleLength) return `제목은(는) ${limits.maxTitleLength}자 이하로 입력해 주세요.`
  if (!description) return '본문을(를) 입력해 주세요.'
  if ([...description].length > limits.maxDescriptionLength) return `본문은(는) ${limits.maxDescriptionLength}자 이하로 입력해 주세요.`
  const integer = (raw: string, label: string, min: number, max: number) => {
    if (raw.trim() === '') return `${label}을(를) 입력해 주세요.`
    if (!/^\d+$/.test(raw.trim()) || Number(raw) < min || Number(raw) > max) return `${label}은(는) ${min} 이상 ${max} 이하의 정수여야 합니다.`
    return null
  }
  return integer(input.unitPrice, '단가', limits.minUnitPrice, limits.maxUnitPrice)
    ?? integer(input.quantity, '수량', limits.minQuantity, limits.maxQuantity)
    ?? (input.imageCount > limits.maxImagesPerProduct ? `사진은 최대 ${limits.maxImagesPerProduct}장까지 등록할 수 있습니다.` : null)
}

/**
 * Applies camera rotation and shrinks large photos in the browser before upload.
 * The server still re-encodes every image; this only saves bandwidth and storage.
 */
export async function prepareImage(file: File): Promise<Blob> {
  if (typeof createImageBitmap !== 'function' || typeof document === 'undefined') return file
  try {
    const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' })
    const scale = Math.min(1, 2000 / Math.max(bitmap.width, bitmap.height))
    const canvas = document.createElement('canvas')
    canvas.width = Math.round(bitmap.width * scale)
    canvas.height = Math.round(bitmap.height * scale)
    const context = canvas.getContext('2d')
    if (!context) return file
    context.drawImage(bitmap, 0, 0, canvas.width, canvas.height)
    bitmap.close()
    const blob = await new Promise<Blob | null>(resolve => canvas.toBlob(resolve, 'image/jpeg', 0.85))
    return blob ?? file
  } catch { return file }
}
