import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import type { ReactNode } from 'react'
import { ConversationList } from './ConversationList'
import { ConversationView } from './ConversationView'

/** Screens of the message feature, or null when the path belongs to another feature. */
export function messageRoute(path: string): ReactNode | null {
  if (path === '/messages') return <ConversationList />
  const match = path.match(/^\/messages\/([A-Za-z0-9_-]+)$/)
  if (match) return <ConversationView key={match[1]} conversationId={match[1]} />
  return null
}
