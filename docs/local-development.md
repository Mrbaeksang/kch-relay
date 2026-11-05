# 로컬 개발

Node 24, Go 1.26, Docker를 사용합니다.

## 컨테이너 하나로 실행

저장소 루트에서 docker compose up --build -d를 실행하고 http://localhost:8080 을 확인합니다. docker compose down은 컨테이너를 종료하지만 볼륨은 보존합니다. 보존할 데이터가 있으면 down -v를 실행하지 않습니다.

## 빠른 수정

1. npm --prefix frontend ci
2. backend에서 APP_ENV=development AUTH_MODE=development go run ./cmd/server (8080)
3. 다른 터미널에서 npm --prefix frontend run dev (5173)

Vite는 /api 요청을 8080으로 프록시합니다. 프론트 빌드 결과를 Go로 확인하려면 npm --prefix frontend run build 후 backend에서 실행합니다. backend 기준 기본 DB 경로는 ./data/market.db입니다. DATABASE_PATH·PORT·WEB_DIR로 변경할 수 있고 dotenv 자동 로딩은 없습니다. 루트 .env는 Compose 보간용이며 비밀키 파일로 커밋하지 않습니다.

## 검사

- npm --prefix frontend test
- npm --prefix frontend run build
- backend에서 go test -race ./..., go vet ./..., go build ./...
- 루트에서 docker build -t kch-relay:local .
- bash scripts/container-smoke.sh kch-relay:local

컨테이너 스모크 테스트는 자신이 만든 임시 컨테이너·볼륨만 정리합니다. 정적 화면, 준비 API, 미구현 API 404, 재생성 후 파일 보존을 검사합니다. 실제 Google 로그인·사내 데이터 검증과는 별개입니다.

## 개발 로그인과 Origin

Compose는 APP_ENV=development·AUTH_MODE=development·APP_ORIGIN=http://localhost:8080을 사용합니다. 직접 분리 개발하면 APP_ORIGIN 기본값은 http://localhost:5173입니다. 브라우저에서 사용하는 주소와 정확히 맞추고 localhost와 127.0.0.1을 섞지 않습니다.

테스트 계정 키는 dev-admin·dev-employee-a·dev-employee-b입니다. 개발 모드로 시작하면 세 가상 계정만 멱등 생성됩니다. 상품·거래 데이터는 재시작 시 초기화하지 않습니다. 같은 브라우저 탭은 쿠키를 공유하므로 A/B 동시 확인은 별도 브라우저 프로필을 사용합니다.

운영 Docker 기본값은 production·disabled입니다. production에서 development 인증을 켜면 시작 실패합니다. 실제 구글 로그인은 아직 구현하지 않았으며 운영 인증을 개발 로그인으로 대체하지 않습니다.

## 공통 계약 검증

`python3 -m unittest discover -s scripts -p 'test_*.py' -v`는 계약 검사기가 잘못된 응답도 잡는지 확인합니다. `python3 scripts/verify_contract.py --base-url http://localhost:8080 --mode development`는 실행 중인 실제 개발 서버를 검사합니다. 분리 Vite 개발에서 백엔드를 직접 검사하려면 `--origin http://localhost:5173`을 추가합니다.

컨테이너 스모크는 실제 development/disabled 모드·CSRF·권한·로그아웃·재생성 후 DB/파일/세션 보존까지 검사합니다. 테스트 자체가 만든 임시 컨테이너·볼륨·쿠키 파일만 정리합니다.

## 새 업무 모듈 연결

서버 조립 함수의 선택적 Module 등록 지점을 사용하면 인증된 router와 DB·Config·MetaProvider를 전달받습니다. HTTP handler는 httpx.Adapt를 사용하고 관리자 기능만 RequireAdmin을 추가합니다. 일반 사용자 인증과 CSRF는 상위 API middleware가 처리합니다. 상품 소유권·대화 참여자 확인은 업무 모듈이 별도로 검증합니다.
