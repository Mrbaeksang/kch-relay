import { useState } from 'react'
import { SessionProvider, useSession, RequireRole } from '../auth/session'
import { NotificationInbox } from '../notifications/NotificationInbox'
import { CompletedDealStatistics } from '../statistics/CompletedDealStatistics'

export function App() { return <SessionProvider><Page /></SessionProvider> }

function Page() {
  return (
    <main>
      <header><span className="wordmark">KCH / RELAY</span><span className="badge">준비 중</span></header>
      <section aria-labelledby="title">
        <p className="eyebrow">동료에게 이어지는 물건과 재능</p>
        <h1 id="title">우리 사이의<br />다음 쓰임.</h1>
        <p className="intro">사과 한 박스부터 영어 과외까지.<br />동료끼리 가볍게 나누고 거래하는 사내 마켓을 준비하고 있습니다.</p>
        <div className="facts"><p><strong>물품과 서비스</strong><span>모두 하나의 상품으로</span></p><p><strong>인사팀 승인</strong><span>승인 후 장터에 공개</span></p><p><strong>성사 전 익명</strong><span>수락한 당사자끼리 신원 공개</span></p></div>
        <DevelopmentSession />
        <RouteSlot />
        <p className="notice" role="status">아직 상품 조회·가입·거래 기능은 제공하지 않습니다.</p>
      </section>
      <footer>동료 사이에서, KCH Relay</footer>
    </main>
  )
}

function DevelopmentSession() {
  const session = useSession()
  return <section aria-label="개발 로그인" className="development-session">
    <h2>개발 계정</h2>
    {session.state.status === 'loading' ? <p>로그인 상태를 확인하고 있습니다.</p> : null}
    {session.error ? <p role="alert">{session.error}</p> : null}
    {session.state.status === 'authenticated' ? <>
      <p>{session.state.user.realName} · {session.state.user.role}</p>
      <p>{session.meta.hasUnreadMessages ? '안 읽은 쪽지 있음' : '안 읽은 쪽지 없음'} / {session.meta.hasUnreadNotifications ? '새 알림 있음' : '새 알림 없음'}</p>
      <button type="button" disabled={session.busy} onClick={() => void session.logout()}>로그아웃</button>
      <nav aria-label="개발 확인"><a href="/">처음으로</a> · <a href="/market">직원 영역</a> · <a href="/admin">관리자 영역</a> · <a href="/admin/statistics">거래 완료 통계</a> · <a href="/notifications">알림함{session.meta.hasUnreadNotifications ? <span className="unread-dot" aria-label="새 알림 있음" /> : null}</a></nav>
    </> : null}
    {session.config?.developmentLogin ? <fieldset disabled={session.busy || session.state.status === 'loading'}><legend>테스트 계정 선택</legend>
      {session.config.accounts.map(account => <button key={account.key} type="button" onClick={() => void session.login(account.key)}>{account.label}로 로그인</button>)}
      <p>같은 브라우저의 탭은 로그인 계정을 공유합니다.</p>
    </fieldset> : session.config ? <p>개발 로그인이 비활성화되어 있습니다. 실제 구글 로그인은 준비 중입니다.</p> : null}
  </section>
}

function RouteSlot() {
  const path = window.location.pathname
  if (path === '/admin') return <RequireRole role="HR_ADMIN"><AccessCheck admin /></RequireRole>
  if (path === '/admin/statistics') return <RequireRole role="HR_ADMIN"><CompletedDealStatistics /></RequireRole>
  if (path === '/market') return <RequireRole><AccessCheck /></RequireRole>
  if (path === '/notifications') return <RequireRole><NotificationInbox /></RequireRole>
  if (path !== '/') return <p role="alert">아직 제공하지 않는 화면입니다.</p>
  return null
}
function AccessCheck({ admin = false }: { admin?: boolean }) {
  const { api } = useSession()
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  async function check() {
    setBusy(true)
    try { await api(admin ? '/admin/access' : '/access'); setMessage('서버 권한 확인을 통과했습니다.') }
    catch (error) { setMessage(error instanceof Error ? error.message : '확인하지 못했습니다.') }
    finally { setBusy(false) }
  }
  return <section aria-label={admin ? '관리자 확인' : '직원 확인'} className="access-check"><h2>{admin ? '관리자' : '직원'} 기능 연결 영역</h2><p>업무 화면은 후속 티켓에서 연결합니다.</p><button type="button" disabled={busy} onClick={() => void check()}>서버 권한 확인</button><p aria-live="polite">{message}</p></section>
}
