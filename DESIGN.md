---
version: "1.0"
name: KCH Relay
description: "한국어 사내 장터를 위한 밝고 단정한 UI 규약. 콘텐츠와 다음 행동을 우선하고 공통 컴포넌트를 재사용한다."
colors:
  primary: "#286FB7"
  brand-sky: "#69B3E3"
  brand-ocean: "#286FB7"
  brand-gray: "#BCBEC0"
  on-primary: "#ffffff"
  ink: "#20252b"
  text-muted: "#5c6672"
  canvas: "#ffffff"
  canvas-soft: "#f2f4f6"
  hairline: "#dce1e6"
  field-border: "#84909d"
  accent: "#286FB7"
  accent-soft: "#edf3ff"
  error: "#b42318"
  success: "#17663f"
  success-soft: "#edf7f0"
  warning: "#855100"
  warning-soft: "#fff5df"
typography:
  heading-1: { fontFamily: Pretendard Variable, fontSize: 36px, fontWeight: 700, lineHeight: 1.3, letterSpacing: -0.02em }
  heading-2: { fontFamily: Pretendard Variable, fontSize: 24px, fontWeight: 600, lineHeight: 1.4, letterSpacing: 0 }
  title: { fontFamily: Pretendard Variable, fontSize: 18px, fontWeight: 600, lineHeight: 1.5, letterSpacing: 0 }
  body: { fontFamily: Pretendard Variable, fontSize: 16px, fontWeight: 400, lineHeight: 1.6, letterSpacing: 0 }
  body-sm: { fontFamily: Pretendard Variable, fontSize: 14px, fontWeight: 400, lineHeight: 1.5, letterSpacing: 0 }
  label: { fontFamily: Pretendard Variable, fontSize: 14px, fontWeight: 500, lineHeight: 1.5, letterSpacing: 0 }
  caption: { fontFamily: Pretendard Variable, fontSize: 12px, fontWeight: 500, lineHeight: 1.5, letterSpacing: 0 }
rounded:
  sm: 6px
  md: 8px
  lg: 10px
  xl: 16px
  full: 9999px
spacing:
  xxs: 4px
  xs: 8px
  sm: 12px
  md: 16px
  lg: 24px
  xl: 32px
  xxl: 48px
components:
  button-primary: { backgroundColor: "{colors.primary}", textColor: "{colors.on-primary}", rounded: "{rounded.md}", typography: "{typography.label}" }
  button-outline: { backgroundColor: "{colors.canvas}", textColor: "{colors.ink}", borderColor: "{colors.hairline}", rounded: "{rounded.md}" }
  text-input: { backgroundColor: "{colors.canvas}", textColor: "{colors.ink}", borderColor: "{colors.field-border}", rounded: "{rounded.md}" }
  badge-neutral: { backgroundColor: "{colors.canvas-soft}", textColor: "{colors.ink}", rounded: "{rounded.full}" }
  badge-success: { backgroundColor: "{colors.success-soft}", textColor: "{colors.success}", rounded: "{rounded.full}" }
  table: { backgroundColor: "{colors.canvas}", textColor: "{colors.ink}", borderColor: "{colors.hairline}" }
---

## Overview

**KCH 구성원이 물건과 서비스를 쉽고 가볍게 거래하는 장터.** 사진·제목·가격과 다음 행동이 먼저 보이는 밝고 단정한 업무 화면을 만든다. 제품 맥락은 PRODUCT.md, 거래 정책은 docs/prd.md, 공식 에셋 출처는 docs/brand-assets.md, 공식 CI 규정 발췌본은 docs/brand/kch-ci-guidelines.pdf를 따른다.

첨부 Mobbin 분석의 토큰·사용 규칙 문서 구조를 참고했다. Mobbin의 상업용 글꼴, 마케팅용 대형 타이포, 모든 버튼을 둥근 pill로 만드는 표현은 복제하지 않는다. 이 문서는 KCH 법인의 공식 CI 매뉴얼이 아닌 **이 저장소의 앱 디자인 규약**이다. 새 이름은 미확정이며 현재 서비스 이름 KCH Relay를 유지한다.

**판단 순서:** 사용자 정책과 접근성 → DESIGN.md의 공통 규칙 → 공통 컴포넌트 → 개별 화면. 시각 규칙이 거래 정책을 바꾸지 않는다.

## Colors

위 frontmatter는 디자인 토큰 목록, 실행 코드는 frontend/src/app/styles.css의 CSS 변수와 Tailwind 매핑이다. 변경 시 둘을 같은 PR에서 갱신한다. 추가 런타임 테마 엔진은 두지 않는다.

- Primary: 주요 행동은 공식 Ocean Blue. 제목·본문은 ink를 사용한다. 이하 PDF 쪽수는 사용자 제공 원본 기준이다. 공식 색상은 PDF 20쪽(인쇄18)의 RGB/HEX 기준이다. Sky Blue·Gray는 로고와 보조 표현에 쓰고 작은 본문이나 흰 글자의 버튼 배경으로 사용하지 않는다.
- Canvas/soft: 흰 화면과 절제된 보조 배경. 상품 사진이 화면의 주요 색을 만든다.
- Accent: 공식 Ocean Blue로 링크·키보드 포커스·선택 상태를 표시한다. 밝은 배경·상태색은 앱 전용 접근성 토큰이며 공식 CI 색상으로 표기하지 않는다.
- Success/warning/error: 의미가 있는 상태에만 쓰고 항상 글자로 상태를 함께 적는다.
- Hairline은 구획 구분용, field-border는 입력 영역 인지용이다. 입력 경계에 더 흐린 hairline을 대신 쓰지 않는다.

일반 글자·placeholder 대비 4.5:1 이상, 큰 글자와 의미 있는 UI 경계 3:1 이상을 확인한다. 이번 범위는 라이트 모드다. 라이브러리에 dark 변형이 있어도 다크 테마를 제공한다고 간주하지 않는다.

## Typography

공식 가이드 PDF 19쪽(인쇄17)의 한글 서체 Pretendard를 앱 기본 글꼴로 사용한다. v1.3.9 가변 WOFF2 한 파일을 자체 호스팅하고 font-display: swap으로 본문 표시를 막지 않는다. 외부 CDN 요청은 없다. 시스템 글꼴은 로딩 실패 시 fallback이다. PDF 17쪽(인쇄15)의 Gotham Medium은 별도 사용권 확인 없이 배포하지 않으며, 로고 글자를 웹폰트로 재작성하지 않는다.

- 화면 제목은 하나의 h1, 하위 섹션은 h2 순서. 모바일 h1은 28px까지 줄인다.
- 본문은 body, 도움말은 body-sm. caption을 본문·입력 오류에 사용하지 않는다.
- 금액·수량·날짜 비교에는 tabular-nums. 가격은 `10,000원`, 0원은 `나눔 · 0원`처럼 의미를 보존한다.
- 제목의 긴 한국어·URL을 확인한다. 목록 제목은 최대 두 줄, 상세 본문은 자르지 않는다.

## Layout

- 기준은 반응형 웹. 360px·768px·1280px에서 확인하고 320px에서도 페이지 가로 넘침을 피한다.
- 최대 너비 1200px, 좌우 여백은 모바일 16px·640px 이상 24px. 간격은 frontmatter의 4px 기반 단계.
- 직원 화면: 장터·내 거래·쪽지·알림 진입점을 일관되게 유지한다. 관리자 도구는 별도 영역이며 UI 숨김과 서버 권한 검사를 함께 적용한다.
- 상품 목록은 모바일 2열을 기본 제안으로 하되 사진·가격·긴 제목이 읽히지 않으면 1열로 조정한다. 640px 이상 3열, 1024px 이상 4열을 출발점으로 실측한다.
- 상세는 작은 화면에서 세로 흐름, 넓은 화면에서 사진과 정보를 나란히 배치한다. 사진 없는 서비스도 빈 프레임 대신 제목·본문으로 충분히 이해할 수 있어야 한다.
- 표는 필요한 영역 안에서만 가로 스크롤. 핵심 값·행동을 숨겨 억지로 맞추지 않는다.
- 고정 하단 행동을 도입할 때 safe-area와 본문 여백을 확보하여 마지막 내용·포커스를 가리지 않는다.

## Elevation & Depth

기본 표면은 평면이다. 상품마다 두꺼운 테두리·그림자·카드를 중첩하지 않는다. 팝오버와 Dialog처럼 겹침이 필요한 요소에만 라이브러리의 절제된 그림자를 사용한다. 장식용 그라데이션·배경 패턴·지속 애니메이션은 사용하지 않는다.

움직임은 hover/상태 변경의 짧은 전환으로 제한한다. prefers-reduced-motion을 존중한다. 상품 목록 진입 시 항목 전체를 순차 애니메이션하지 않는다.

## Shapes

버튼·입력은 md, 일반 표면은 lg, 사진은 xl. pill은 상태 배지처럼 작은 요소에만 사용한다. 클릭 가능한 버튼과 입력 높이는 최소 44px, 아이콘 버튼도 44px 정사각형 이상이다. 공식 로고는 PDF 3쪽의 원본 비율·도형과 20쪽 RGB 색상을 보존하고 이름은 별도 텍스트로 배치한다. 인쇄 최소 높이는 3mm이며 임의의 웹 px 규정으로 바꾸지 않는다. 앱에서는 너비72px로 표시한다. PDF 21쪽의 금지 사례처럼 글꼴 대체·재색칠·그라데이션·외곽선·순서/비율 변형을 하지 않는다.

## Components

**단일 진입점:** frontend/src/components/ui의 공통 컴포넌트를 import한다. 현재 Button·Input·Textarea·Label·Badge·Dialog·Select·Table·Skeleton을 제공한다. shadcn CLI로 가져온 코드이며 팀 규칙에 맞게 수정했다. 설치·추가·예제는 docs/agents/frontend-ui.md를 따른다.

| 구성 | 규칙 |
| --- | --- |
| Button | 주요 행동 default, 보조 outline, 덜 중요한 행동 ghost, 실제 파괴 행동 destructive. 한 구역에서 주요 행동 하나. type을 명시하고 서버 처리 중 disabled와 진행 문구를 제공한다. |
| Input / Textarea | Label과 id 연결. placeholder는 Label 대체가 아니다. 오류는 aria-invalid·aria-describedby와 복구 방법을 함께 제공한다. |
| Select | 정해진 enum 선택에 사용. 간단한 선택은 native select도 허용하되 크기·포커스·레이블 규칙을 따른다. |
| Badge | 상태 텍스트용. 승인 대기 secondary, 판매 중 success, 모집 종료 warning, 완료 outline, 반려 destructive. 태그 기능을 추가하지 않는다. |
| Dialog | 확인과 집중이 필요한 짧은 작업에만 사용. 제목·설명, Escape, 포커스 복귀를 유지한다. 전체 상품 등록을 작은 모달에 몰아넣지 않는다. |
| Table | 인사팀 승인·통계. 헤더와 숫자 정렬을 명확히 한다. 실제 집계가 없는 곳에 샘플 통계를 표시하지 않는다. |
| Skeleton | 실제 조회 중에만 사용. 빈 결과와 로딩을 구분한다. |

Lucide 아이콘은 16–20px, strokeWidth 기본값 2를 유지한다. 텍스트와 중복인 아이콘은 aria-hidden, 아이콘 단독 버튼은 aria-label을 제공한다. 이모지를 조작 아이콘 대신 쓰지 않는다.

**도메인 표현:** 수락 전 판매자는 `판매자`, 신청자는 서버가 발급한 상품별 익명 별칭이다. 수락 후 허용된 당사자에게만 실명·부서·직급을 보여준다. 이메일·전역 사용자 ID를 화면 데이터에 섞지 않는다. 승인·신청·수락·완료를 서로 다른 상태와 행동으로 명확하게 표현한다. 수락 후 취소 버튼은 없다.

장터 목록은 src/features/catalog/Catalog.tsx의 CatalogList, 상세는 ProductDetail에 구현되어 있다. 공통 RouteSlot에서 연결하고 사진 없는 상품은 설명 미리보기를 제공한다. 실제 업무 기능을 대신하는 가짜 신청 버튼을 만들지 않는다.

**상태:** 모든 조회 화면은 로딩·빈 결과·오류/재시도·권한/세션 만료를 갖춘다. 알림은 처리 결과를 설명하고 키보드·스크린리더로 읽을 수 있게 한다. 고정 폴링이나 새 서버를 UI 편의 때문에 추가하지 않는다.

## Do's and Don'ts

- **Do** 공통 토큰과 컴포넌트를 먼저 사용하고 중복 패턴은 공통화한다.
- **Do** 버튼이 실제 수행하는 동사를 쓰고 거래 수락의 결과를 확인 전에 설명한다.
- **Do** 모바일·키보드·긴 한국어·0원·사진 없음·처리 중 상태를 검증한다.
- **Do** 공통 변경은 DESIGN.md·사용 가이드·개발 예시를 같은 PR에서 갱신한다.
- **Don't** 화면마다 hex 색상·border radius·버튼을 새로 정의하지 않는다.
- **Don't** 카테고리·별점·태그·결제·실시간 채팅을 템플릿에 있다는 이유로 추가하지 않는다.
- **Don't** 새 UI 프레임워크·아이콘 라이브러리·토스트 라이브러리를 담당자 판단만으로 추가하지 않는다.
- **Don't** 클라이언트 역할 표시만으로 개인정보 공개·관리자 접근을 허용하지 않는다.

본문은 line-height 1.6, letter-spacing -0.01em, 한국어 어절 단위 줄바꿈을 사용한다. 설명 문단은 line-height 1.75와 문단 간 12–16px 여백을 두고, 업무 화면 제목·필터·빈 상태는 각각 24px 이상 간격으로 구분한다. 상품 사진은 고정 4:3 프레임 안에 object-contain으로 전체를 보여 주며 상세 프레임은 뷰포트 높이의 70%를 넘지 않는다.

업무 상세는 `components/layout/DetailSection` 공통 구획으로 작성한다. 작성자·가격/수량·설명, 심사 이력, 참여자 신원을 별도 그룹으로 나누고 데스크톱 2열·모바일 1열로 배치한다. 표는 공통 Table의 스크롤 컨테이너, 상태는 Badge를 사용한다. h3·h4의 기본 굵기/행간도 글로벌로 제공하되 문단·구획 간격은 공통 조합이 책임진다.
