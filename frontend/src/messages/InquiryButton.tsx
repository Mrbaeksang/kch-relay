import { Button } from "@/components/ui/button"
import { useState } from 'react'
import { useSession } from '../auth/session'
import { describe } from '../transactions/model'

/** Opens (or reuses) the viewer's anonymous conversation on a product. The product screen (#16) places it. */
export function InquiryButton({ productId }: { productId: string }) {
  const { api } = useSession()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  async function start() {
    setBusy(true); setError(null)
    try {
      const { conversationId } = await api<{ conversationId: string }>(`/products/${productId}/conversations`, { method: 'POST' })
      window.location.assign(`/messages/${conversationId}`)
    } catch (e) { setError(describe(e)); setBusy(false) }
  }
  return <div className="inquiry">
    <Button type="button" disabled={busy} onClick={() => void start()}>익명으로 문의하기</Button>
    {error ? <p role="alert">{error}</p> : null}
  </div>
}
