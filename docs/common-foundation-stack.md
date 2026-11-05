# 공통 기반 담당·PR 스택

2026-11-04. 프로젝트 소유자를 루트 담당자로 고정하고 나머지 5명은 무작위 배정했다.

| 이슈 | 담당자 | 작업 브랜치 | 최초 PR base |
|---|---|---|---|
| [#4](https://github.com/Mrbaeksang/kch-relay/issues/4) | @Mrbaeksang | setup/foundation-root | main |
| [#5](https://github.com/Mrbaeksang/kch-relay/issues/5) | @Seonwoo2231 | setup/global-http | setup/foundation-root |
| [#6](https://github.com/Mrbaeksang/kch-relay/issues/6) | @barsaa1989 | setup/shared-database | setup/global-http |
| [#7](https://github.com/Mrbaeksang/kch-relay/issues/7) | @kmb8696-hash | setup/development-auth | setup/shared-database |
| [#8](https://github.com/Mrbaeksang/kch-relay/issues/8) | @dlwotjd799-code | setup/frontend-foundation | setup/development-auth |
| [#9](https://github.com/Mrbaeksang/kch-relay/issues/9) | @kkh3558 | setup/foundation-verification | setup/frontend-foundation |

모두 계약 기준으로 즉시 착수한다. 부모가 아직 구현하지 않았으면 test-only fixture를 사용하고, 부모 변경이 올라오면 자기 브랜치를 restack한다. 구현 커밋이 생긴 후 최초 PR base를 사용해 PR을 만든다. 빈 PR은 만들지 않았다.

main 병합은 #4부터 순서대로, CI 통과 후 관리자 Squash 방식이다. 부모 Squash 후에는 이전 부모 tip 기준으로 자기 변경만 새 main 위로 옮기고 base를 main으로 바꾼다. 단순히 과거 부모 커밋을 다시 적용하지 않는다. 후속 자식도 restack한다. main 강제 push와 다른 사람의 브랜치 임의 변경은 금지한다.

현재 원격 브랜치 6개는 동일 main 기준으로 생성되었다. 업무 구현·새 PR·main 병합은 아직 수행하지 않았다. 상세 계약과 각 티켓의 실제 통합 조건을 함께 따른다.

## 2026-11-05 운영 변경

사용자가 공통 기반 6개를 먼저 모두 구현·검증·병합하도록 요청했다. 이번 셋업 구현은 한 작업자가 이슈별 PR을 순서대로 반영하며, 원래의 구성원별 배정은 후속 기능 티켓과 구분한다. main 보호·필수 CI·이슈별 범위는 유지한다.
