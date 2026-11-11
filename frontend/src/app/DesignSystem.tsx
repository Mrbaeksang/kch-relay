import { useState } from "react"
import { Check, Search } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"

/** Development-only component workbench. All examples are illustrative, with no API writes. */
export function DesignSystem() {
  const [saved, setSaved] = useState(false)
  return <main className="page-shell space-y-10 pb-12">
    <header className="app-header"><div className="brand"><img src="/brand/kch/kch-logo.svg" alt="KCH" width="72" height="34" /><span className="wordmark">Relay</span></div><Badge variant="secondary">개발용 UI 가이드</Badge></header>
    <section className="space-y-3"><h1>같은 기준으로 만드는 장터</h1><p className="max-w-prose text-muted-foreground">버튼, 입력, 상태 안내를 확인하는 개발 전용 예시입니다. 실제 상품이나 거래 데이터는 저장하지 않습니다.</p></section>
    <section aria-labelledby="buttons" className="space-y-4"><h2 id="buttons">버튼과 확인창</h2><div className="flex flex-wrap items-center gap-3"><Button type="button" onClick={() => setSaved(true)}><Check aria-hidden="true" />예시 확인</Button><Button type="button" variant="outline" onClick={() => setSaved(false)}>초기화</Button><Button type="button" disabled>처리 중…</Button><Dialog><DialogTrigger asChild><Button type="button" variant="secondary">확인창 예시</Button></DialogTrigger><DialogContent><DialogHeader><DialogTitle>변경 내용을 확인해 주세요</DialogTitle><DialogDescription>키보드 Tab으로 이동하고 Escape로 닫을 수 있습니다. 실제 거래를 처리하지 않는 예시입니다.</DialogDescription></DialogHeader></DialogContent></Dialog></div><p role="status" className="text-sm text-muted-foreground">{saved ? '예시를 확인했습니다. 저장된 데이터는 없습니다.' : '주요 행동은 파란 버튼, 보조 행동은 테두리 버튼으로 구분합니다.'}</p></section>
    <section aria-labelledby="fields" className="space-y-4"><h2 id="fields">입력과 선택</h2><div className="grid max-w-3xl gap-6 sm:grid-cols-2"><div className="space-y-2"><Label htmlFor="example-title">상품 제목</Label><Input id="example-title" placeholder="예: 영어 회화 1회" /><p className="text-sm text-muted-foreground">물건과 서비스 모두 상품으로 등록합니다.</p></div><div className="space-y-2"><Label htmlFor="example-search">장터 검색</Label><div className="relative"><Search aria-hidden="true" className="pointer-events-none absolute top-3 left-3 size-5 text-muted-foreground" /><Input id="example-search" className="pl-10" placeholder="어떤 상품을 찾으세요?" /></div></div><div className="space-y-2"><Label htmlFor="example-select">선택 컴포넌트 예시</Label><Select><SelectTrigger id="example-select" className="w-full"><SelectValue placeholder="항목 선택" /></SelectTrigger><SelectContent><SelectItem value="one">첫 번째 예시</SelectItem><SelectItem value="two">두 번째 예시</SelectItem></SelectContent></Select></div><div className="space-y-2"><Label htmlFor="example-error">입력 오류</Label><Input id="example-error" aria-invalid="true" aria-describedby="example-error-help" defaultValue="-1" /><p id="example-error-help" className="text-sm text-destructive">가격은 0원 이상으로 입력해 주세요.</p></div><div className="space-y-2 sm:col-span-2"><Label htmlFor="example-description">상품 설명</Label><Textarea id="example-description" placeholder="제공 내용과 거래 방법을 적어 주세요." /></div></div></section>
    <section aria-labelledby="states" className="space-y-4"><h2 id="states">상태 표시</h2><div className="flex flex-wrap gap-2"><Badge variant="secondary">승인 대기</Badge><Badge className="bg-success-surface text-success">판매 중</Badge><Badge className="bg-warning-surface text-warning">모집 종료</Badge><Badge variant="outline">거래 완료</Badge><Badge variant="destructive">반려</Badge></div><p className="text-sm text-muted-foreground">색상에만 의존하지 않고 상태 이름을 함께 표시합니다. 분류·태그 기능이 아닙니다.</p><Skeleton className="h-6 w-48" aria-label="불러오는 중" /></section>
    <section aria-labelledby="rows" className="space-y-4"><h2 id="rows">표와 빈 상태</h2><Table><TableHeader><TableRow><TableHead>예시 항목</TableHead><TableHead>상태</TableHead><TableHead className="text-right">금액</TableHead></TableRow></TableHeader><TableBody><TableRow><TableCell>가상 상품</TableCell><TableCell><Badge variant="secondary">승인 대기</Badge></TableCell><TableCell className="text-right tabular-nums">10,000원</TableCell></TableRow></TableBody></Table><div className="py-6"><p className="font-medium">아직 알림이 없습니다</p><p className="text-sm text-muted-foreground">거래 신청 결과가 도착하면 여기에서 확인할 수 있습니다.</p></div></section>
    <footer className="app-footer">기준 문서: DESIGN.md · UI 추가 방법: docs/agents/frontend-ui.md</footer>
  </main>
}
