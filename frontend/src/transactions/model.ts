import { APIError } from '../lib/api'

export type DealStatus = 'REQUESTED' | 'WITHDRAWN' | 'REJECTED' | 'ACCEPTED' | 'COMPLETED'
export type DealAction = 'updateQuantity' | 'withdraw' | 'accept' | 'reject' | 'complete'
export type Identity = { realName: string; department: string; jobLevel: string }

export type Deal = {
  id: string
  productId: string
  conversationId: string
  status: DealStatus
  viewerRole: 'BUYER' | 'SELLER'
  product: { id: string; title: string; unitPrice: number; remainingQuantity: number; recruitmentClosed: boolean; hidden: boolean }
  requestedQuantity: number
  agreedUnitPrice: number | null
  agreedQuantity: number | null
  totalPrice: number | null
  requestedAt: string
  quantityUpdatedAt: string | null
  acceptedAt: string | null
  completedAt: string | null
  rejectedAt: string | null
  withdrawnAt: string | null
  counterparty: { label: string; identity: Identity | null }
  actions: DealAction[]
}

export type ManagedProduct = {
  id: string
  title: string
  description: string
  unitPrice: number
  remainingQuantity: number
  recruitmentClosed: boolean
  approvalStatus: 'PENDING' | 'APPROVED' | 'REJECTED'
  hidden: boolean
  imageIds: string[]
  pendingApplicationCount: number
}

export type Page<T> = { items: T[]; nextCursor: string | null }

export const statusLabels: Record<DealStatus, string> = {
  REQUESTED: '대기 중',
  WITHDRAWN: '철회함',
  REJECTED: '거절됨',
  ACCEPTED: '거래 성사',
  COMPLETED: '거래 완료',
}

export const actionLabels: Record<Exclude<DealAction, 'updateQuantity'>, string> = {
  withdraw: '신청 철회',
  reject: '거절',
  accept: '수락',
  complete: '거래 완료 처리',
}

export const won = (value: number) => `${value.toLocaleString('ko-KR')}원`
export const when = (value: string) => new Date(value).toLocaleString('ko-KR')
export const describe = (error: unknown) => error instanceof APIError ? error.message : '연결을 확인하고 다시 시도해 주세요.'
