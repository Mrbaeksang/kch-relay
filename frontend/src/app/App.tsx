import { messageRoute } from "../messages/routes"
import { Onboarding } from '../auth/Onboarding'
import { MemberAdmin, DepartmentAdmin } from '../auth/Administration'
import { CompletedDealStatistics } from "../statistics/CompletedDealStatistics"
import { NotificationInbox } from "../notifications/NotificationInbox"
import { transactionRoute } from "../transactions/routes"
import { MyProducts } from "../products/MyProducts"
import { AdminReview } from "../products/AdminReview"
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { DesignSystem } from './DesignSystem'
import { CatalogList, ProductDetail } from '@/features/catalog/Catalog'
import { useState } from 'react'
import { SessionProvider, useSession, RequireRole } from '../auth/session'

export function App() {
  return (
    <SessionProvider>
      <Page />
    </SessionProvider>
  )
}

function Page() {
  const session = useSession()
  const [navigationOpen,setNavigationOpen]=useState(false)
  if (import.meta.env.DEV && window.location.pathname === '/design-system')
    return <DesignSystem />
  const authenticated = session.state.status === 'authenticated' && session.state.user.status === 'ACTIVE'
  return (
    <main className="page-shell flex flex-col">
      <a
        href="#content"
        className="sr-only focus:not-sr-only focus:mb-4 focus:underline"
      >
        본문으로 바로가기
      </a>
      <header className="app-header flex-wrap">
        <a
          href="/market"
          className="brand rounded-md"
          aria-label="KCH Relay 장터"
        >
          <img src="/brand/kch/kch-logo.svg" alt="KCH" width="72" height="34" />
          <span className="wordmark">Relay</span>
        </a>
        {authenticated ? (
          <>
          <Button type="button" variant="outline" className="md:hidden" aria-controls="main-navigation" aria-expanded={navigationOpen} onClick={()=>setNavigationOpen(open=>!open)}>메뉴</Button>
          <nav
            id="main-navigation"
            aria-label="주 메뉴"
            className={`${navigationOpen ? "flex" : "hidden"} w-full flex-wrap items-center gap-2 md:flex md:w-auto`}
          >
            <Button variant="ghost" asChild>
              <a href="/market">장터</a>
            </Button>
            <Button variant="ghost" asChild><a href="/me/products">내 상품</a></Button>
          <Button variant="ghost" asChild><a href="/market/deals">내 신청·거래</a></Button>
          <Button variant="ghost" asChild><a href="/notifications">알림함{session.meta.hasUnreadNotifications?<span aria-label="새 알림 있음" className="size-2 rounded-full bg-destructive"/>:null}</a></Button>
          <Button variant="ghost" asChild><a href="/messages">쪽지{session.meta.hasUnreadMessages?<span aria-label="안 읽은 쪽지 있음" className="size-2 rounded-full bg-destructive"/>:null}</a></Button>
          {session.state.status === 'authenticated' &&
            session.state.user.role === 'HR_ADMIN' ? (
              <Button variant="ghost" asChild>
                <a href="/admin/products">상품 심사</a>
              </Button>
            ) : null}
            <Button
              type="button"
              variant="outline"
              disabled={session.busy}
              onClick={() => void session.logout()}
            >
              로그아웃
            </Button>
          {session.state.status === "authenticated" && session.state.user.role === "HR_ADMIN" ? <Button variant="ghost" asChild><a href="/admin/statistics">거래 통계</a></Button>:null}{session.state.status === "authenticated" && session.state.user.role === "HR_ADMIN" ? <><Button variant="ghost" asChild><a href="/admin/members">구성원 관리</a></Button><Button variant="ghost" asChild><a href="/admin/departments">부서 관리</a></Button></>:null}</nav>
          </>
        ) : (
          <Badge variant="secondary">구성원 전용</Badge>
        )}
      </header>
      {authenticated &&
      (session.meta.hasUnreadMessages ||
        session.meta.hasUnreadNotifications) ? (
        <p aria-live="polite" className="mt-4 text-sm text-muted-foreground">
          {session.meta.hasUnreadMessages ? '안 읽은 쪽지가 있습니다. ' : ''}
          {session.meta.hasUnreadNotifications ? '새 알림이 있습니다.' : ''}
        </p>
      ) : null}
      {authenticated && session.error ? (
        <p role="alert" className="mt-4 text-destructive">
          {session.error}
        </p>
      ) : null}
      <div id="content" tabIndex={-1} className="flex-1">
        {session.state.status === 'loading' ? (
          <p role="status" className="py-12">
            로그인 상태를 확인하고 있습니다.
          </p>
        ) : session.state.status === 'authenticated' && session.state.user.status !== 'ACTIVE' ? <Onboarding /> : authenticated ? (
          <RouteSlot
            key={
              session.state.status === 'authenticated'
                ? session.state.user.id
                : ''
            }
          />
        ) : (
          <section className="welcome" aria-labelledby="title">
            <h1 id="title">
              우리 사이의
              <br />
              다음 쓰임.
            </h1>
            <p className="intro">
              사과 한 박스부터 영어 과외까지.
              <br />
              동료끼리 가볍게 나누고 거래하는 사내 마켓입니다.
            </p>
            <div className="facts">
              <p>
                <strong>물품과 서비스</strong>
                <span>모두 하나의 상품으로</span>
              </p>
              <p>
                <strong>인사팀 승인</strong>
                <span>승인 후 장터에 공개</span>
              </p>
              <p>
                <strong>성사 전 익명</strong>
                <span>수락한 당사자끼리 신원 공개</span>
              </p>
            </div>
            {session.config?.googleLogin ? <div className="mt-8 space-y-3">{session.error ? <p role="alert" className="text-destructive">{session.error}</p> : null}{new URLSearchParams(window.location.search).has('loginError') ? <p role="alert" className="text-destructive">구글 로그인을 완료하지 못했습니다. 다시 시도해 주세요.</p> : null}<Button asChild><a href="/api/v1/auth/google/start">Google 계정으로 로그인</a></Button><p className="text-sm text-muted-foreground">처음 이용하면 가입 신청 후 인사팀 승인이 필요합니다.</p></div> : <DevelopmentSession />}
          </section>
        )}
      </div>
      <footer className="app-footer mt-12 pb-4">
        동료 사이에서, KCH Relay
      </footer>
    </main>
  )
}

function DevelopmentSession() {
  const session = useSession()
  return (
    <section aria-label="개발 로그인" className="development-session">
      <h2>개발 계정</h2>
      {session.state.status === 'loading' ? (
        <p>로그인 상태를 확인하고 있습니다.</p>
      ) : null}
      {session.error ? <p role="alert">{session.error}</p> : null}
      {session.state.status === 'authenticated' ? (
        <>
          <p>
            {session.state.user.realName} · {session.state.user.role}
          </p>
          <p>
            {session.meta.hasUnreadMessages
              ? '안 읽은 쪽지 있음'
              : '안 읽은 쪽지 없음'}{' '}
            /{' '}
            {session.meta.hasUnreadNotifications
              ? '새 알림 있음'
              : '새 알림 없음'}
          </p>
          <Button
            type="button"
            disabled={session.busy}
            onClick={() => void session.logout()}
          >
            로그아웃
          </Button>
          <nav aria-label="개발 확인">
            <a href="/">처음으로</a> · <a href="/market">직원 영역</a> ·{' '}
            <a href="/admin/products">상품 심사 영역</a>
          </nav>
        </>
      ) : null}
      {session.config?.developmentLogin ? (
        <fieldset
          className="flex flex-wrap gap-2"
          disabled={session.busy || session.state.status === 'loading'}
        >
          <legend>테스트 계정 선택</legend>
          {session.config.accounts.map((account) => (
            <Button
              key={account.key}
              type="button"
              onClick={() => void session.login(account.key)}
            >
              {account.label}로 로그인
            </Button>
          ))}
          <p>같은 브라우저의 탭은 로그인 계정을 공유합니다.</p>
        </fieldset>
      ) : session.config ? (
        <p>
          개발 로그인이 비활성화되어 있습니다. 실제 구글 로그인은 준비 중입니다.
        </p>
      ) : null}
    </section>
  )
}

// Future feature tickets add their screen entry points here; auth stays in the shared provider.
function RouteSlot() {
  const path = window.location.pathname
  const message = messageRoute(path)
  if (message) return <RequireRole>{message}</RequireRole>
 if(path === "/admin/members") return <RequireRole role="HR_ADMIN"><MemberAdmin /></RequireRole>
 if(path === "/admin/departments") return <RequireRole role="HR_ADMIN"><DepartmentAdmin /></RequireRole>
  if (path === "/notifications") return <RequireRole><NotificationInbox /></RequireRole>
  if(path === "/admin/statistics") return <RequireRole role="HR_ADMIN"><CompletedDealStatistics /></RequireRole>
  const transaction = transactionRoute(path)
  if (transaction) return <RequireRole>{transaction}</RequireRole>
  if (path === '/' || path === '/market')
    return (
      <RequireRole>
        <CatalogList />
      </RequireRole>
    )
  const product = /^\/products\/([^/]+)\/?$/.exec(path)
  if (product) {
    let id: string
    try {
      id = decodeURIComponent(product[1])
    } catch {
      return <p role="alert">상품 주소를 확인해 주세요.</p>
    }
    return (
      <RequireRole>
        <ProductDetail productId={id} />
      </RequireRole>
    )
  }
  if (path === '/me/products') return <RequireRole><MyProducts /></RequireRole>
  if (path === '/admin/products') return <RequireRole role="HR_ADMIN"><AdminReview /></RequireRole>
  if (path === '/admin')
    return (
      <RequireRole role="HR_ADMIN">
        <AccessCheck admin />
      </RequireRole>
    )
  return (
    <section className="py-12">
      <h1>아직 제공하지 않는 화면입니다</h1>
      <a
        className="mt-4 inline-flex min-h-11 items-center underline"
        href="/market"
      >
        장터로 돌아가기
      </a>
    </section>
  )
}

function AccessCheck({ admin = false }: { admin?: boolean }) {
  const { api } = useSession()
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  async function check() {
    setBusy(true)
    try {
      await api(admin ? '/admin/access' : '/access')
      setMessage('서버 권한 확인을 통과했습니다.')
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : '확인하지 못했습니다.',
      )
    } finally {
      setBusy(false)
    }
  }
  return (
    <section
      aria-label={admin ? '관리자 확인' : '직원 확인'}
      className="access-check"
    >
      <h2>{admin ? '관리자' : '직원'} 기능 연결 영역</h2>
      <p>업무 화면은 후속 티켓에서 연결합니다.</p>
      <Button type="button" disabled={busy} onClick={() => void check()}>
        서버 권한 확인
      </Button>
      <p aria-live="polite">{message}</p>
    </section>
  )
}
