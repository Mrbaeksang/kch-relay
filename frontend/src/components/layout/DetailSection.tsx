import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

/** A shared information group: heading, consistent padding, and reading rhythm. */
export function DetailSection({ title, children, className }: { title: string; children: ReactNode; className?: string }) {
  return <section aria-label={title} className={cn('rounded-lg border bg-card p-5 sm:p-6', className)}>
    <h3 className="text-base font-semibold leading-6">{title}</h3>
    <div className="mt-4 space-y-4 text-sm leading-7">{children}</div>
  </section>
}
