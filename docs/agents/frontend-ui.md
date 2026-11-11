# 공통 프론트 UI 사용법

화면 작업 전 루트 DESIGN.md와 PRODUCT.md, 해당 기능 이슈·API 계약을 읽는다. 사용자 제공 Mobbin 문서를 다시 다운로드할 필요 없이 저장소 안의 기준만으로 작업한다.

## 설치와 확인

```sh
npm --prefix frontend ci
npm --prefix frontend run dev
```

http://localhost:5173/design-system 에서 공통 컴포넌트를 확인한다. 개발 모드 전용이며 서버 쓰기·실제 데이터·폴링이 없다. production 빌드에서는 이 경로를 UI 가이드로 제공하지 않는다. 업무 기능은 별도 티켓에서 구현한다.

## 선택한 라이브러리

- Tailwind CSS 4 + 공식 Vite 플러그인: CSS 빌드 도구. 별도 운영 프로세스 없음.
- shadcn/ui new-york 계열의 Radix 기반 컴포넌트: 소스는 src/components/ui에서 직접 관리한다.
- Lucide React: 아이콘. 필요한 아이콘만 이름으로 import한다.
- clsx + tailwind-merge: src/lib/utils의 cn으로 클래스 조합.
- 의존성 버전은 frontend/package.json·package-lock.json을 커밋하며 팀원은 npm ci로 동일하게 설치한다.

공식 참고: https://ui.shadcn.com/docs/installation/vite · https://tailwindcss.com/docs/installation/using-vite

## 재사용 예시

```tsx
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

<div className="space-y-2">
  <Label htmlFor="title">상품 제목</Label>
  <Input id="title" name="title" required />
</div>
<Button type="submit" disabled={saving}>
  {saving ? '저장 중…' : '상품 등록'}
</Button>
```

구체적인 saving 상태·폼 처리·API는 각 기능에서 구현한다. 공통 컴포넌트에 업무 API·권한·도메인 상태 로직을 넣지 않는다. 입력 오류는 Label 유지 + aria-invalid + aria-describedby + 오류 텍스트로 연결한다. Select의 항목 값은 서버 enum을 따른다.

색상은 bg-background/text-foreground/text-muted-foreground/bg-primary, 상태는 text-destructive/text-success/text-warning 등 의미 토큰을 쓴다. 금액은 tabular-nums, 긴 텍스트는 min-w-0·필요 시 줄바꿈으로 처리한다. hex 값·임의 크기·인라인 스타일은 공유 토큰으로 표현할 수 없는 이유가 있을 때만 검토한다.

## 컴포넌트 추가·수정

기존 9종이 부족할 때만 추가한다. frontend 디렉터리에서 아래 고정 CLI를 사용한다.

```sh
npx shadcn@4.23.0 add <component>
```

components.json에 CSS 경로·별칭·스타일이 들어 있다. 생성 결과는 반드시 검토한다. 이미 변경한 컴포넌트를 --overwrite로 덮지 않는다. CLI가 외부 cn 패키지 import를 생성하면 @/lib/utils로 수정하며 불필요한 패키지는 제거한다. 기본 높이가 작게 생성되면 44px 터치 영역에 맞춘다. 별칭은 Vite·Vitest·TypeScript 모두 src를 가리키도록 설정되어 있다(@/components는 src/components).

코드를 복사해 화면 전용 두 번째 Button을 만들지 않는다. 업무별 조합은 src/features/<feature>에서 공통 UI를 조합한다. 공통 변경은 사용 중인 모든 화면을 확인하고 DESIGN.md·/design-system을 함께 갱신한다.

## PR 확인

- npm --prefix frontend test 및 npm --prefix frontend run build.
- 360px/768px/1280px 화면, 키보드 포커스·Dialog 닫기/복귀, 긴 한국어·오류·빈 상태.
- 수락 전 익명성·승인 전 비공개·취소 없음 등 업무 정책.
- 공통 UI/토큰을 재사용했는지, 새 라이브러리나 네트워크 요청이 불필요하게 추가되지 않았는지.
- 도구 사용 시 AGENTS.md와 해당 스킬을 따른다. 디자인 문서는 권한 검사나 테스트를 대체하지 않는다.

## 장터 화면과 후속 기능 연결

장터 목록은 src/features/catalog/Catalog.tsx의 CatalogList, 상세는 ProductDetail이다. App.tsx의 RouteSlot에서 /·/market·/products/:id를 조립한다. 후속 티켓은 자기 기능 화면을 export하고 RouteSlot에 진입점을 추가한다. SessionProvider·전역 인증/로그아웃 처리를 복제하지 않는다. 일반 직원/관리자 가드는 RequireRole을 사용하되 최종 권한은 서버에서 검사한다.

목록은 검색 제출·페이지 이동·수동 재시도 때만 조회한다. 이전 요청은 AbortController로 취소하고 늦은 응답을 무시한다. 상품 사진 실패는 이미지 대체 안내를 표시한다. 상품 DTO에는 공개 판매자 별칭만 사용하며 프론트에서 사용자 정보를 조합하지 않는다. canApply는 표시용이며 실제 거래 API의 권한·재고 검사를 대체하지 않는다.

#17의 실제 상품 등록·승인·이미지 API가 연결된 후 등록→승인→목록/상세 사진 표시를 추가 검증한다. 현재 SQLite fixture 검증을 해당 기능과의 통합 완료라고 보고하지 않는다.
