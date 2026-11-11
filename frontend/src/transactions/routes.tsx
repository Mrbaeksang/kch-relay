import type { ReactNode } from 'react'
import { ApplyForm, DealDetail, MyDeals } from './deals'
import { ProductManage } from './manage'

const id = '([A-Za-z0-9_-]+)'

/** Screens of the deal feature, or null when the path belongs to another feature. */
export function transactionRoute(path: string): ReactNode | null {
  let match: RegExpMatchArray | null
  if (path === '/market/deals') return <MyDeals />
  if ((match = path.match(new RegExp(`^/market/deals/${id}$`)))) return <DealDetail key={match[1]} dealId={match[1]} />
  if ((match = path.match(new RegExp(`^/market/products/${id}/manage$`)))) return <ProductManage key={match[1]} productId={match[1]} />
  if ((match = path.match(new RegExp(`^/market/products/${id}/apply$`)))) return <ApplyForm key={match[1]} productId={match[1]} />
  return null
}
