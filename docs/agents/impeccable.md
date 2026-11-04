# Impeccable 디자인 스킬

팀원이 같은 디자인 지침을 사용하도록 공식 Impeccable 4.8.0을 저장소 안에 설치했다. 앱 실행 의존성이 아니라 코딩 에이전트용 도구이며 운영 Docker 이미지에 포함되지 않는다.

## 설치 위치

- Codex: `.agents/skills/impeccable/`
- Claude Code: `.claude/skills/impeccable/`

각 도구의 공식 배포본은 지침·경로·에이전트 정의가 달라 각각 원본을 유지한다. 기존 kch-* 팀 스킬과 달리 두 폴더를 같은 심볼릭 링크로 합치지 않는다. 최초 복사 후 Claude 실행 스크립트의 실행 권한만 추가했다.

다음 턴에 스킬이 표시되는지 확인한다. 기존 세션에서 목록이 갱신되지 않으면 코딩 도구를 다시 연다. 개인 전역 스킬 설치는 필요 없다.

## 사용 예

Codex:

```text
$impeccable init
$impeccable audit 상품 등록 화면
$impeccable critique 거래 신청 흐름
$impeccable polish 상품 카드
$impeccable adapt 모바일 장터
```

Claude Code에서는 `$impeccable` 대신 `/impeccable`을 사용한다.

첫 디자인 작업의 init은 기존 [PRD](../prd.md)·[용어집](../../CONTEXT.md)·[기술 스택](../tech-stack.md)을 먼저 읽고 이미 확정된 내용을 재질문하지 않는다. 필요한 제품 맥락을 PRODUCT.md에 정리하는 흐름이며 이번 설치에서는 UI 변경·디자인 방향 선택·PRODUCT.md 초기화까지 수행하지 않았다.

## 설치 범위

스킬·명령 참조·런처·Codex 에이전트 정의를 포함한다. 자동 훅과 live 서버는 추가하지 않았다. 자동 검사·디자인 변경은 사용자가 요청한 작업에서 실행한다. 전체 개발·커밋·PR 규칙은 AGENTS.md와 기존 kch-* 스킬을 계속 따른다.

런처는 운영체제별 엔진을 처음 실행할 때 공식 GitHub 릴리스에서 다운로드할 수 있고 동봉된 체크섬으로 검증한다. 버전·플랫폼 캐시는 개발자 PC에 저장되며 Git에 올리지 않는다. 스킬이 필요할 때 실행하는 개발 도구이며 사내 서비스의 서버 메모리·실행 프로세스를 늘리지 않는다.

## 출처와 업데이트

- [공식 저장소](https://github.com/pbakaus/impeccable)
- 고정 커밋: `695cea5c6d1cbd420fc331375c7ef126a774e5f7`
- 스킬 버전: `4.8.0`, 엔진 버전: `0.1.18`
- [원본 정보·파일 해시](../third-party/impeccable/upstream.json)
- [Apache 2.0 라이선스](../third-party/impeccable/LICENSE)

갱신할 때는 새 이슈·브랜치에서 공식 저장소의 두 도구용 배포 폴더를 같은 커밋 기준으로 함께 교체하고 출처·파일 해시·실행 권한을 갱신한다. 공식 설치기를 사용한다면 `npx impeccable@<version> install --providers=codex,claude --scope=project`이며, 자동 훅·설정 등 추가 변경을 확인한다. 기존 팀 스킬을 덮어쓰지 않는다. PR로 검토하고 CI 통과 후 관리자만 병합한다.
