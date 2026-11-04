# KCH Relay 팀 개발 지침

## 프로젝트와 기준 문서

- 약 100명 규모의 사내 물품·서비스 거래 마켓이다. React·Vite·TypeScript, Go·chi, SQLite를 사용한다.
- 모노레포의 frontend·backend를 하나의 Docker 이미지로 빌드한다. Go가 정적 화면과 API를 제공하고 Railway 단일 서비스·영구 볼륨을 사용한다.
- 용어는 CONTEXT.md, 요구사항은 docs/prd.md, 모델은 docs/erd.md, API는 docs/api-spec.md, 중요한 결정은 docs/adr/를 따른다.
- 이슈는 GitHub Mrbaeksang/kch-relay에서 관리한다. 상세 운영은 docs/agents/를 읽는다.

## 공통 원칙

- 변경 전 관련 이슈·코드·문서를 확인하고 타인의 미완료 변경을 보존한다.
- 요청을 만족하는 가장 단순한 구현을 선택한다. 운영비·메모리·개발 시간을 우선하며 별도 서버·실시간 연결·불필요한 추상화를 추가하지 않는다.
- 거래 수락 전 익명, 인사팀 승인 후 공개, 수락 시 수량 차감·단가/수량 확정, 수락 후 취소 없음 정책을 지킨다.
- 수량 변경은 대기 신청을 거절하지 않는다. 상품 내용·가격 수정은 대기 신청 거절·앱 내 알림과 함께 처리한다.

## 작업별 지침

| 작업 | 스킬 |
|---|---|
| 기능·버그 | .agents/skills/kch-implement/SKILL.md |
| 리뷰 | .agents/skills/kch-review/SKILL.md |
| 커밋 | .agents/skills/kch-commit/SKILL.md |
| PR | .agents/skills/kch-pr/SKILL.md |

해당 작업의 스킬만 읽는다. 개인 PC 절대 경로·개인 스킬 설치를 전제하지 않는다.

## 실행·검사

```sh
npm --prefix frontend ci
npm --prefix frontend test
npm --prefix frontend run build
cd backend
go test -race ./...
go vet ./...
go build ./...
```

저장소 루트에서 docker compose up --build -d로 단일 컨테이너를 시작한다. 화면·API는 http://localhost:8080이다. 분리 개발은 docs/local-development.md를 따른다.

- 관련 검사와 빌드를 완료하고 실행하지 못한 검사는 이유를 남긴다.
- SQLite·사진은 영구 볼륨에 저장한다. 보존할 볼륨·DB를 임의 삭제하지 않는다.
- 가상 정보로 검증한다. 개발 로그인·실제 직원 정보·비밀키를 운영·공개 저장소에 노출하지 않는다.

## 협업

- 이슈 1개 = 작업 브랜치 1개 = PR 1개. 같은 이슈의 기존 열린 PR은 갱신한다.
- 브랜치는 feature/설명, fix/설명, docs/설명, setup/설명. 상위 사용자 지침에서 별도 접두어를 지정하면 따른다.
- 커밋·PR 제목은 비개발자가 이해할 한국어 한 문장으로 쓴다. PR 안에서는 목적별 커밋, main에는 Squash로 남긴다.
- 최초 빈 저장소의 부트스트랩 이후 main 직접 push·강제 push 금지. 필수 CI 통과 후 관리자만 PR을 병합한다.
- 커밋·push·PR·병합·배포는 사용자가 요청한 범위에서만 수행한다.
- 완료 보고는 변경·검증·남은 일 순서로 적고 로컬·CI·main 반영·배포를 구분한다.

## 리뷰 우선순위

신원·권한 노출, 인사팀 승인 우회, 초과 수락, 거래 합의값 변경, 수정/거절/알림 원자성, SQLite 영속성, 기간별 완료 금액의 날짜 경계를 우선 검토한다.
