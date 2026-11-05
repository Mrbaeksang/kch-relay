# 6인 병렬 공통 기반 계약 v1

2026-11-04. 목적은 공통 셋업 6개 티켓의 개발 착수 대기를 없애는 것이다. 착수 의존과 실제 병합·통합 검증의 순서를 구분한다. 모든 담당자가 공통 설정을 중복 구현하지 않는다.

## 확정된 실행·범위

- 개발: Vite 5173, Go 8080, Vite의 /api 프록시. 운영: Go 프로세스 하나가 정적 화면·API를 제공하는 Docker 이미지, Railway 단일 서비스·영구 볼륨.
- 실제 구글 인증은 후속 작업. 개발은 관리자·직원 A·직원 B의 서버 세션으로 검증한다.
- 디자인 기조는 프로젝트 소유자가 담당한다. 공통 기능 화면은 시각 디자인 결정 없이 기본 마크업으로 검증한다.
- 역할 MEMBER·HR_ADMIN, 사용자 상태 ACTIVE·SUSPENDED. 실명·부서·직급은 본인·허용 상대·관리자 외에는 노출하지 않는다.
- 아래 계약은 이번 공통 기반의 구현 기준이다. 미정인 사업 정책·회사 enum을 임의 확정하는 문서는 아니다.

## C1. 설정·서버 조립 계약

Config의 필드명은 AppEnv, AuthMode, Port, DatabasePath, UploadDir, WebDir, AppOrigin, SessionTTL로 통일한다. 환경변수 대응은 APP_ENV, AUTH_MODE, PORT, DATABASE_PATH, UPLOAD_DIR, WEB_DIR, APP_ORIGIN, SESSION_TTL이다.

- AppEnv: development 또는 production. 생략 시 production.
- AuthMode: disabled 또는 development. 생략 시 disabled. google은 후속 예약값이며 현재 지정하면 명확히 시작 실패한다.
- production + development 인증은 시작 오류. 설정을 개발 인증으로 자동 보정하지 않는다.
- Port 기본 8080. 로컬 경로는 실행 위치에 맞춰 일관되게 계산하고, Docker는 DB /data/market.db·사진 /data/uploads·정적 파일 /app/frontend/dist를 사용한다.
- AppOrigin은 브라우저 기준 신뢰할 origin. development 기본 http://localhost:5173. production에서 외부 업무 기능을 열기 전 HTTPS origin을 명시한다. disabled 상태의 준비 서버는 공개 인증 없이 상태 확인·정적 준비 화면만 제공할 수 있다.
- SessionTTL은 duration 값, 기본 8h, 양수만 허용한다. 상시 만료 연장은 하지 않는다.
- LoadConfig는 설정 또는 오류를 반환한다. 프로세스 종료·로그 출력을 스스로 하지 않는다.
- 서버 조립 담당자가 설정 로딩·DB 열기·마이그레이션·조건부 개발 seed·모듈 등록·공통 미들웨어 순서를 책임진다. 다른 모듈은 main 또는 루트 라우터를 직접 바꾸지 않고 등록 함수를 제공한다.
- /healthz는 프로세스, /api/readyz는 초기화와 DB 준비 상태를 확인한다. 정상은 200, 준비 실패는 503. 운영 상태 확인 응답은 업무 meta 대상에서 제외한다.

## C2. 요청·사용자·에러 계약

공통 사용자 Actor의 필드: ID, RealName, Department, JobLevel, Role, Status. 문자열 값이며 ID는 불투명 식별자다. 구글 식별자·세션 토큰·이메일을 Actor의 외부 응답에 포함하지 않는다.

- WithActor: 검증된 Actor를 요청 context에 저장한 새 context 반환.
- CurrentUser: context에서 Actor와 존재 여부 반환. 쿠키 재조회·DB 재접속을 하지 않는다.
- HTTP 글로벌 담당자가 위 타입·context 함수를 소유한다. 인증 담당자는 이 계약으로 요청을 보강한다. 공통 HTTP 코드가 인증 구현을 import하는 순환 구조를 만들지 않는다.
- 공통 Handler는 요청을 받아 Result 또는 error를 반환한다. Result는 Status와 Data를 가진다. Adapt는 이를 표준 HTTP 응답으로 변환한다. 업무 모듈은 공통 어댑터 밖에서 임의 JSON 오류를 쓰지 않는다.
- AppError 필드: Status, Code, Message, Details, Cause. Cause는 내부 로깅 전용이며 직렬화하지 않는다.
- 오류: VALIDATION_ERROR 400, UNAUTHENTICATED/SESSION_EXPIRED 401, ACCOUNT_SUSPENDED/FORBIDDEN 403, NOT_FOUND 404, INVALID_STATE 등 업무 충돌 409, INTERNAL_ERROR 500.
- 성공 응답 최상위는 data, meta. 실패는 error, 필요한 meta. error는 code·message·선택 details·requestId. 미인증 오류에는 meta를 생략한다. 예상 못한 오류의 message는 일반 안내문이고 SQL·경로·원인 문자열을 노출하지 않는다.
- meta는 hasUnreadMessages·hasUnreadNotifications 두 boolean이다. 인증된 일반 JSON 업무 응답에만 붙인다. MetaProvider는 context와 검증된 사용자 ID를 받아 두 존재 여부 또는 오류를 반환한다. 기능 모듈 결합은 조립 지점에서만 한다.
- 인증된 업무 응답의 meta는 처리 후 조회한다. 이미 commit한 뒤 meta 조회가 실패하면 서버 오류일 수 있음을 숨기지 않는다. 상태 변경의 재시도 안전성은 기능 구현에서 보장하고 500이 항상 rollback을 뜻한다고 가정하지 않는다.
- 요청 ID를 서버가 생성하고 X-Request-ID 응답 헤더에 제공한다. 외부 요청 ID를 무제한 그대로 신뢰하지 않는다.
- 로그는 요청 ID·메서드·템플릿 경로·상태·소요시간. 쿠키·토큰·개인정보·본문·query 값을 그대로 기록하지 않는다.
- Panic recovery는 최후의 500 처리다. 정상 업무 오류는 반환값으로 처리한다. 이미 전송을 시작한 응답에 JSON을 두 번 쓰지 않는다.
- 일반 JSON 본문 1MiB 제한·단일 JSON 값·알 수 없는 필드 거절. 업로드는 별도 정책. JSON 업무 요청 15초 context 제한, 서버 transport timeout과 별도다. timeout도 중복 응답하지 않는다.
- 요청 처리 순서: 요청 ID/접근 로그 → panic recovery → 공통 제한 → 선택적 세션 조회 → 경로별 RequireUser/RequireAdmin·CSRF → 업무 handler → 공통 응답. 상태 확인·로그인은 공개 경로 예외다.
- DecodeJSON은 공통 파싱만 한다. 금액·수량·문자열 업무 제약은 기능 소유자가 검증한다. 페이지 기본20·최대100, 잘못된 cursor/limit는 400이다.

## C3. DB·세션 저장·공용 데이터 계약

- 기존 database.Open의 context·경로 입력과 sql.DB 반환 계약을 유지한다. foreign_keys ON, busy_timeout 5000, 최대 열린 연결 1. 신규 schema와 제약은 goose 추가 migration으로 적용한다.
- WithTx는 context·DB·tx를 받는 작업 함수를 받아 begin/commit/rollback을 관리하고 오류를 반환한다. tx 내부에서 바깥 DB를 재호출하지 않는다. nested transaction·외부 네트워크·이미지 처리 없음.
- 전체 업무 schema의 의미는 기존 ERD를 따른다. users·sessions·products·participants·deals·messages·notifications·images·reviews를 준비한다. 미정인 사업 제한은 확정된 DB 제약처럼 넣지 않는다.
- users.google_subject는 nullable UNIQUE로 전환한다. 개발 사용자는 NULL. 기존 사용자와 UNIQUE·CHECK·timestamp 데이터는 보존한다. 이미 적용한 migration을 수정하지 않는다.
- sessions 필드: id, user_id, token_hash, csrf_token, created_at, expires_at, revoked_at. token_hash는 UNIQUE, user_id FK. 시각은 UTC 고정 밀리초 ISO 8601 TEXT, revoked_at만 nullable. 원본 세션 토큰 저장 금지.
- 개발 사용자 키 dev-admin·dev-employee-a·dev-employee-b. 개발용 seed는 내부 사용자 ID를 고정하고 이메일은 example.invalid, 실명은 테스트 관리자/테스트 직원 A/테스트 직원 B, 부서·직급은 TEST_DEPT·TEST_LEVEL. 실제 회사 enum으로 오해시키지 않는다.
- EnsureDevelopmentUsers는 development 전용 호출. 동일 키로 중복 생성·기존 데이터 초기화 없이 세 명을 보장한다. 업무 상품·거래 샘플은 서버 재시작 시 자동 재생성하지 않는다.
- 테스트 데이터 도구는 실제 migrations를 적용한 임시 DB와 시각/ID가 통제된 사용자·상품·거래·쪽지·알림 fixture를 제공한다. production에서 fixture 생성 API를 열지 않는다.
- EnsureParticipantTx는 상품ID·구매자ID·요청 종류 문의/신청을 받아 같은 연결을 재사용하고 필요한 순번만 원자적으로 배정한다. 참가자·별칭 소유자는 DB 공통 담당자이며 재신청 별칭 정책은 미정 사항을 명시한다.
- AppendRejectionNotificationTx는 tx·거래ID·수신자ID·생성시각을 받아 알림ID를 반환한다. APPLICATION_REJECTED, 거래+유형 UNIQUE, 중복이면 기존 알림 재사용. 자체 commit 없음.
- UnreadMeta는 사용자ID별 recipient_id·read_at 인덱스 기반 EXISTS 두 개로 조회한다. 사용자 테이블의 중복 카운터 없음. 실제 SQL 공급자는 DB 공통 담당자가 제공하고 HTTP MetaProvider에 주입한다.

## C4. 인증 API 계약

버전 접두어 /api/v1. 아래는 테스트용 가짜 응답이 아니라 실제 서버 세션을 사용하는 정식 개발 API다.

- GET /auth/config: data.authMode, data.developmentLogin. development일 때만 data.accounts 배열(key,label,role)을 제공한다. disabled에서는 accounts를 빈 배열로 반환한다.
- POST /auth/development-login: 입력은 accountKey 하나. 허용 키는 위 세 개뿐이며 임의 사용자ID·role을 받지 않는다. development에서만 경로를 등록하고 production/disabled는 404.
- 로그인 성공200: data.user는 Actor의 camelCase 필드(id,realName,department,jobLevel,role,status), data.csrfToken은 CSRF 난수. meta 두 값 포함. 원본 세션 토큰은 Set-Cookie만 사용한다.
- GET /auth/me: 로그인 성공과 동일한 data.user·data.csrfToken·meta. 미인증401. 새로고침 시 복구 용도.
- POST /auth/logout: 세션·CSRF 검증 후 현재 세션 회수, 쿠키 삭제, 200 data.loggedOut=true. 응답 후 인증 상태는 익명이다.
- 쿠키 이름 relay_session, HttpOnly, SameSite=Lax, Path=/api, 운영 Secure. development의 HTTP에서만 Secure=false. 난수 토큰의 SHA-256 해시를 DB에 저장한다.
- 토큰과 CSRF 값은 암호학적 난수 최소32bytes. CSRF는 세션에 연결하고 변경 요청 X-CSRF-Token과 비교한다. 로그인 전 development-login은 신뢰 origin·JSON 검사로 보호하며 세션 token을 요구하지 않는다.
- Authentication middleware는 세션을 한 번 조회하고 ACTIVE·만료·회수 여부를 확인해 Actor를 설정한다. RequireUser는 로그인 여부, RequireAdmin은 HR_ADMIN 여부만 판단한다. 데이터 소유권은 기능에서 검증한다.
- 로그인 교체 시 현재 세션 회수 후 새 세션 발급. 다른 기기 세션은 유지한다. 여러 탭은 같은 쿠키를 공유하므로 A/B 동시 검증은 별도 브라우저 프로필 사용.
- 만료 시각이 현재 시각과 같으면 만료다. NowUTC와 ID 생성은 테스트에서 통제할 수 있도록 생성 시 주입한다. 전역 가변 clock을 두지 않는다.

## C5. 프론트 공통 계약

- apiRequest가 상대 /api/v1 주소·동일 origin 쿠키·JSON·변경 요청 CSRF 헤더를 처리한다. localStorage에 세션 토큰을 넣지 않는다. 미확인 표시는 응답 meta로 갱신한다.
- AuthState는 loading, anonymous, authenticated 중 하나이며 authenticated만 user·csrfToken을 가진다. user의 필드는 C4와 동일하다.
- 앱 시작은 auth/config 확인 및 auth/me 복구다. 정상 anonymous 401은 로그인 화면으로 전환하며 무한 재요청하지 않는다. authenticated 이후401은 상태·CSRF를 지우고 재로그인 안내한다. 403은 로그아웃시키지 않고 권한 오류로 표시한다.
- 최소 개발 로그인·현재 계정 표시·로그아웃·직원/관리자 접근 안내를 연결한다. 기능 화면의 route slot을 제공하지만 업무 화면을 대신 구현하지 않는다.
- 디자인 기조는 프로젝트 소유자가 정한다. 전역 색·글꼴·간격·최종 컴포넌트 props는 기능 명세로 임의 고정하지 않는다. 기본 마크업으로도 검증 가능해야 한다.
- 폴링·웹소켓·추가 상태관리 서버 없음. 프론트 공용 호출/인증 상태/라우트 소유권은 한 담당자에게 둔다.

## C6. 독립 착수·병합·검증 규칙

- 개발 착수 blocker는 모두 없다. 각 티켓은 위 확정 계약을 기준으로 작성한다. 다른 담당자 산출물이 없을 때 test-only 어댑터·HTTP reference fixture로 자체 경계를 검증할 수 있다.
- 테스트 대체 구현은 테스트 범위에만 두고 운영 코드에 임시 인증 우회·가짜 성공·기능별 별도 DB를 넣지 않는다. 공용 패키지를 각자 중복 구현하지 않는다.
- 공용 패키지를 import해야 해서 미완성 작업 브랜치가 compile되지 않는다면 draft PR로 공유한다. 계약만으로 해당 브랜치의 전체CI가 이미 통과한다고 주장하지 않는다. 필요한 기반이 main에 반영된 뒤 rebase하고 실제 공급자와 검사한다.
- 독립 착수는 무조건 독립 병합이라는 뜻이 아니다. 설정/HTTP/DB 제공 후 인증 연결, 그다음 프론트 실제 연결 순서가 존재한다. 이는 담당자 작업을 시작하지 못하게 하는 blocker가 아니라 merge/integration gate로 각 이슈에 적는다.
- 전체 검증 담당자는 API reference fixture로 계약 검증 도구·시나리오·CI 실행 방식을 먼저 작성하고, 공급자가 준비되면 실제 단일 컨테이너에 같은 검증을 적용한다. fixture만 통과한 상태를 전체 통합 완료로 표시하지 않는다.
- 공통 bootstrap·루트 라우터·Docker·Compose의 최종 조립은 실행/설정 담당자가 한다. 개별 모듈 담당자는 자기 등록 함수와 격리 검증을 제공하여 루트 파일 충돌을 줄인다. 추가 연결 변경은 해당 이슈 PR에서 조율한다.
- 1인 1이슈·1작업브랜치·1PR. 테스트·기능 연결은 각 PR의 범위에 포함한다. shared contract 변경은 소비 담당자 영향과 버전 변경을 명시하고 독단적으로 바꾸지 않는다.

## 발행된 공통 티켓

- [[공통 1/6] 같은 설정으로 서버를 실행하고 모듈을 연결한다](https://github.com/Mrbaeksang/kch-relay/issues/4)
- [[공통 2/6] 모든 요청의 인증 정보와 오류 처리를 같은 방식으로 통일한다](https://github.com/Mrbaeksang/kch-relay/issues/5)
- [[공통 3/6] 같은 SQLite 구조와 트랜잭션으로 데이터를 개발한다](https://github.com/Mrbaeksang/kch-relay/issues/6)
- [[공통 4/6] 구글 계정 없이 테스트 관리자와 직원으로 로그인한다](https://github.com/Mrbaeksang/kch-relay/issues/7)
- [[공통 5/6] 프론트에서 같은 로그인 상태와 API 오류를 사용한다](https://github.com/Mrbaeksang/kch-relay/issues/8)
- [[공통 6/6] 여섯 모듈의 계약과 단일 컨테이너 연결을 자동 검증한다](https://github.com/Mrbaeksang/kch-relay/issues/9)
