import type { Identity } from '../transactions/model'

export type { Identity, Page } from '../transactions/model'

export type Counterparty = { label: string; identity: Identity | null }

export type ConversationSummary = {
  conversationId: string
  viewerRole: 'BUYER' | 'SELLER'
  product: { id: string; title: string }
  counterparty: Counterparty
  lastMessage: { body: string; createdAt: string; fromSelf: boolean } | null
  hasUnread: boolean
}

export type Sender = { kind: 'SELF' | 'COUNTERPARTY'; label: string; identity: Identity | null }

export type Message = { id: string; body: string; createdAt: string; readAt: string | null; sender: Sender }

/** Message length limit owned by #19 (docs/decisions/anonymous-messages.md); the server enforces the same value. */
export const MAX_MESSAGE_LENGTH = 2000

/** Counts Unicode code points, the way the server counts characters. */
export const codePointLength = (text: string) => [...text].length

export const describeCounterparty = ({ label, identity }: Counterparty) =>
  identity ? `${label} · ${identity.realName} (${identity.department} · ${identity.jobLevel})` : label
