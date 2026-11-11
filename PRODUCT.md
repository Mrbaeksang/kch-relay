# Product

<!-- impeccable:product-schema 1 -->

## Platform

web — PC·모바일 반응형 웹.

## Users

약 100명의 KCH 구성원. 직원은 판매자이자 구매자이며 인사팀은 상품 승인·이용 정지·기간별 완료 거래 통계를 담당한다.

## Product Purpose

사내에서 물품·서비스·나눔을 가볍게 거래한다. 모두 상품이고 가격과 필수 수량을 입력하며 수량 단위는 본문에 적는다.

## Capabilities and Constraints

상품은 인사팀 승인 후 공개. 거래 수락 전 상품별 익명, 수락 시 당사자 신원 공개·수량 차감·단가와 수량 확정. 수락 후 취소 없음, 판매자가 거래 완료 처리. 수량 변경은 대기 신청을 거절하지 않고 내용·가격 변경은 대기 신청 거절·알림과 원자적으로 처리한다. 결제·에스크로·배송·별점·태그·카테고리는 제공하지 않는다. 상세 정책은 docs/prd.md가 기준이다.

React·Vite·TypeScript, Go·chi, SQLite. 단일 Docker·Railway 서비스·영구 볼륨. 별도 프론트 런타임·실시간 서버·고정 폴링을 추가하지 않는다. 구글 인증은 추후 구현하며 현재 테스트 계정을 제공한다.

## Brand Commitments

사용자가 요청한 방향은 깔끔하고 직관적인 KCH그룹의 사내 장터다. 서비스 이름은 현재 KCH Relay이며 제안한 새 이름은 미확정이다. 공식 로고는 frontend/public/brand/kch에 원본·출처를 보관한다. 사용자가 제공한 공식 CI 가이드의 브랜드 규정 발췌본은 docs/brand/kch-ci-guidelines.pdf에 보관하며 메인 로고·Sky Blue/Ocean Blue/Gray·한글 Pretendard를 확인했다. 첨부 Mobbin 문서는 디자인 규약의 구성 예시이며 해당 회사의 글꼴·에셋 사용 허가를 의미하지 않는다.

## Evidence on Hand

CONTEXT.md · docs/prd.md · docs/erd.md · docs/api-spec.md · docs/brand-assets.md.

## Open Decisions

새 서비스 이름, 구글 인증 자격 증명, Gotham 사용권 확인이 필요한 경우 별도 검토. 미확정 내용을 디자인 작업에서 사실로 만들지 않는다.
