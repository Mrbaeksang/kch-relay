import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useSession } from '../auth/session'
import { APIError } from '../lib/api'

type CompletedDeals = { from: string; to: string; timezone: string; currency: string; completedDealCount: number; totalCompletedAmount: number }
type View = { status: 'loading' } | { status: 'loaded'; result: CompletedDeals } | { status: 'error'; message: string }

const number = new Intl.NumberFormat('ko-KR')
const unsafeAmount = '합계 금액을 정확히 표시할 수 없습니다. 기간을 나누어 조회해 주세요.'

// seoulDate is the Korean calendar date of an instant, whatever time zone the browser uses.
function seoulDate(now: Date) {
  return new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Seoul', year: 'numeric', month: '2-digit', day: '2-digit' }).format(now)
}
function describe(error: unknown) {
  if (error instanceof RangeError) return unsafeAmount
  if (error instanceof APIError) return error.message
  return '서버와 통신하지 못했습니다. 연결을 확인하고 다시 시도해 주세요.'
}

export function CompletedDealStatistics() {
  const { api } = useSession()
  const [range, setRange] = useState(() => { const today = seoulDate(new Date()); return { from: `${today.slice(0, 8)}01`, to: today } })
  const [view, setView] = useState<View>({ status: 'loading' })
  const inFlight = useRef<AbortController | null>(null)

  async function load(from: string, to: string) {
    inFlight.current?.abort()
    const controller = new AbortController()
    inFlight.current = controller
    setView({ status: 'loading' })
    try {
      const result = await api<CompletedDeals>(`/admin/statistics/completed-deals?${new URLSearchParams({ from, to })}`, { signal: controller.signal })
      if(controller.signal.aborted)return
      if (!Number.isSafeInteger(result.totalCompletedAmount) || !Number.isSafeInteger(result.completedDealCount)) throw new RangeError(unsafeAmount)
      setView({ status: 'loaded', result })
    } catch (error) {
      if (!controller.signal.aborted) setView({ status: 'error', message: describe(error) })
    }
  }
  useEffect(() => {
    void load(range.from, range.to)
    return () => inFlight.current?.abort()
  }, []) // 화면을 열 때 기본 기간으로 한 번만 조회한다.

  function submit(event: FormEvent) {
    event.preventDefault()
    if (!range.from || !range.to || range.to < range.from) {
      inFlight.current?.abort()
      setView({ status: 'error', message: !range.from || !range.to ? '시작일과 종료일을 모두 선택해 주세요.' : '종료일은 시작일과 같거나 이후여야 합니다.' })
      return
    }
    void load(range.from, range.to)
  }

  return <section aria-labelledby="statistics-title" className="statistics">
    <h1 id="statistics-title">기간별 거래 완료 금액</h1>
    <p>이 금액은 판매자가 거래 완료로 표시한 장터 거래액입니다. 회사 매출이나 실제 송금·정산 금액이 아닙니다.</p>
    <form onSubmit={submit} noValidate>
      <label>시작일<Input type="date" value={range.from} onChange={event => setRange({ ...range, from: event.target.value })} /></label>
      <label>종료일<Input type="date" value={range.to} onChange={event => setRange({ ...range, to: event.target.value })} /></label>
      <Button type="submit">조회</Button>
    </form>
    {view.status === 'loading' ? <p role="status">불러오는 중입니다.</p> : null}
    {view.status === 'error' ? <p role="alert">{view.message}</p> : null}
    {view.status === 'loaded' ? <div aria-live="polite">
      <p>조회 기간: {view.result.from} ~ {view.result.to} (한국 시간, 시작일·종료일 포함)</p>
      {view.result.completedDealCount === 0 ? <p>이 기간에 완료된 거래가 없습니다.</p> : null}
      <dl>
        <div><dt>완료 거래 건수</dt><dd>{`${number.format(view.result.completedDealCount)}건`}</dd></div>
        <div><dt>총 거래 완료 금액</dt><dd>{`${number.format(view.result.totalCompletedAmount)}원`}</dd></div>
      </dl>
    </div> : null}
  </section>
}
