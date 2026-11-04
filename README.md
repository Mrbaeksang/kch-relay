# KCH Relay

**동료 사이에서 물건과 재능을 잇는 사내 마켓.**

약 100명의 같은 회사 재직자가 물품·서비스를 판매하거나 나눔하는 반응형 웹입니다. 인사팀 승인 후 상품 공개, 성사 전 상품별 익명, 판매자 수락 시 신원 공개·수량 차감, 판매자 완료 처리를 기준으로 합니다.

## 현재 상태

개발 기반 설정이 완료된 단계입니다. 준비 화면, Go 정적 파일·API 서버, SQLite 초기 마이그레이션, 상태 확인, 단일 Docker 빌드·볼륨 검증, CI가 포함됩니다. 실제 로그인·상품·쪽지·거래·알림·통계 업무 기능은 아직 구현하지 않았습니다.

## 구성

- frontend/: Vite + React + TypeScript. 빌드 후 별도 Node 서버 없이 Go가 제공합니다.
- backend/: Go + chi + SQLite(modernc) + goose 마이그레이션.
- Dockerfile: 프론트·백 빌드 후 Go 프로세스 하나로 실행.
- /data 영구 볼륨: market.db와 uploads/. 별도 S3·DB 서버 없음.
- Railway: 서비스 하나·복제본 하나·볼륨 하나.

## 시작하기

```sh
docker compose up --build -d
```

http://localhost:8080 에서 준비 화면을 확인합니다. http://localhost:8080/api/readyz 는 SQLite 연결 상태를 확인합니다.

- [로컬 개발](docs/local-development.md)
- [Railway 배포·백업](docs/deployment.md)
- [기술 스택과 비용](docs/tech-stack.md)
- [PRD](docs/prd.md) · [ERD](docs/erd.md) · [API 명세](docs/api-spec.md)
- [비개발자용 인포그래픽](docs/infographics/README.md)
- [팀 협업](CONTRIBUTING.md) · [에이전트 지침](AGENTS.md)

## 자동 검사

PR·main 반영 시 Frontend build and test, Backend build and test, Single container build and smoke test를 실행합니다. main은 필수 CI·Squash·대화 해결·관리자 병합 규칙으로 보호합니다.

개인정보·실제 사내 이메일·운영 DB·사진·비밀키는 이 공개 저장소에 커밋하지 않습니다.
