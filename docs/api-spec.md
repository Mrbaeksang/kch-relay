# API 명세 초안

작성일: 2026-11-04. Base path `/api/v1`. 검토용 JSON REST 명세이며 실행 가능한 OpenAPI는 아니다. [PRD](prd.md)·[ERD](erd.md)의 미확정 제안을 포함한다.

## 1. 공통

- 구글 인증 후 서버 세션 쿠키 사용을 제안한다. HttpOnly, 운영 환경 Secure, 적절한 SameSite와 상태 변경 요청의 CSRF 검증을 적용한다.
- 업무 API는 가입이 완료된 ACTIVE 사용자만, 관리자 API는 HR_ADMIN만 사용한다.
- ID는 불투명 문자열. 날짜는 UTC ISO 8601. 금액은 원화 정수, 수량은 정수다. 최대값은 안전한 정수 범위에서 확정한다.
- 목록은 cursor·limit(제안: 기본 20, 최대 100), 안정적인 시각+ID 정렬. 목록 data는 `{items: [], nextCursor: null}`.
- 인증된 업무 응답에 `meta.hasUnreadMessages`, `meta.hasUnreadNotifications`를 함께 제공한다. 인증 리다이렉트·이미지 바이너리는 제외한다. 204 대신 JSON 200을 사용한다.
- 요청이 없으면 미확인 표시도 갱신되지 않는다. 별도 타이머 조회는 사용하지 않는다.
- 익명 응답에는 상대의 전역 userId·구글 ID·이메일을 포함하지 않는다. 개인 응답은 공유 캐시하지 않는다.

```json
{
  "data": {},
  "meta": {"hasUnreadMessages": true, "hasUnreadNotifications": false}
}
```

```json
{
  "error": {
    "code": "PENDING_APPLICATIONS_EXIST",
    "message": "대기 중인 신청을 거절한 후 수정할 수 있습니다.",
    "details": {"pendingCount": 2}
  },
  "meta": {"hasUnreadMessages": false, "hasUnreadNotifications": false}
}
```

미인증 오류에는 meta를 생략한다. 서버는 모든 변경 요청에서 소유권·상태·수량을 재검증한다.

| HTTP | 주요 오류 코드 |
|---|---|
| 400 | VALIDATION_ERROR, INVALID_ENUM_VALUE |
| 401 | UNAUTHENTICATED, SESSION_EXPIRED |
| 403 | ACCOUNT_SUSPENDED, FORBIDDEN, COMPANY_ACCOUNT_REQUIRED |
| 404 | NOT_FOUND — 미공개 상품·타인 대화 등 비공개 리소스 포함 |
| 409 | INVALID_STATE, PENDING_APPLICATIONS_EXIST, INSUFFICIENT_QUANTITY, ACTIVE_DEAL_EXISTS, RECRUITMENT_CLOSED |

## 2. 인증·가입

| Method | 경로 | 요청·결과 |
|---|---|---|
| GET | `/auth/google/start` | 구글 인증으로 리다이렉트 |
| GET | `/auth/google/callback` | 회사 계정 검증, 기존 사용자 로그인 또는 가입용 세션 후 리다이렉트 |
| GET | `/registration-options` | 구글 인증된 가입자에게 department·jobLevel의 code·label 목록 |
| POST | `/auth/register` | 가입용 세션 + `{realName, department, jobLevel}` → 가입 완료, 201 |
| GET | `/auth/me` | 본인 프로필·역할·이용 상태 및 meta. 가입 전이면 등록 필요 상태 |
| POST | `/auth/refresh` | 세션 갱신 및 meta. 갱신 자체가 구글 계정 정지 확인을 보장하지 않음 |
| POST | `/auth/logout` | 세션 회수, 200 |

email·googleSubject는 인증 결과에서 얻는다. role·status는 서버 관리이며 가입 입력으로 받지 않는다. 개인정보 수정 API는 정책 확정 전 제외한다. 회사 도메인·enum 실제 값·계정 정지 연동은 미정이다.

## 3. 상품

| Method | 경로 | 권한·동작 |
|---|---|---|
| GET | `/products?q=&cursor=&limit=` | 승인·비삭제 상품 목록. 제목·본문 검색은 제안 |
| GET | `/products/{productId}` | 공개 상품. 판매자는 자기 미공개 상품도 조회 |
| GET | `/me/products` | 본인 상품·승인 상태·최근 반려 사유 |
| POST | `/products` | `{title, description, unitPrice, quantity, imageIds?}` → PENDING 생성, 201 |
| PATCH | `/products/{productId}` | 판매자. `{title?, description?, unitPrice?, imageIds?, rejectPendingApplications?}` |
| PATCH | `/products/{productId}/quantity` | 판매자. `{remainingQuantity}` 0 이상 |
| POST | `/products/{productId}/close` | 모집 종료. 기존 신청·쪽지·거래 유지 |
| POST | `/products/{productId}/reopen` | 모집 재개. 잔여 수량 0이면 신청 불가 유지 |
| POST | `/products/{productId}/submit-review` | 판매자. REJECTED → PENDING |
| DELETE | `/products/{productId}` | 판매자 숨김, 200 `{deleted:true}`. 기록 유지 |

- 가격 0 이상·등록 수량 1 이상·제목/본문 필수·사진 선택. 수량 단위 필드 없음.
- rejectPendingApplications 기본 false. 변경 필드가 있고 REQUESTED가 남아 있으면 409. true면 대기 신청 전부 거절·신청자별 알림·수정을 원자적으로 처리한다.
- ACCEPTED·COMPLETED는 수정 차단·일괄 거절 대상이 아니다. 승인 상품은 수정 후에도 승인 상태 유지.
- 동일 값만 제출한 무변경 요청은 거절을 발생시키지 않는다.
- 수량·모집 변경은 신청을 유지한 채 허용한다. 수량을 줄여 기존 신청 수량보다 적어져도 자동 거절하지 않으며 수락 시 재검증한다.

등록 예:

```json
{
  "title": "사과 판매",
  "description": "한 개는 사과 한 박스입니다. 전달은 직접 협의합니다.",
  "unitPrice": 10000,
  "quantity": 10,
  "imageIds": []
}
```

공개 상품 응답 예:

```json
{
  "data": {
    "id": "product-id",
    "title": "사과 판매",
    "description": "한 개는 사과 한 박스입니다.",
    "unitPrice": 10000,
    "remainingQuantity": 8,
    "recruitmentClosed": false,
    "canApply": true,
    "seller": {"label": "판매자"},
    "images": [],
    "createdAt": "2026-11-04T10:00:00Z"
  },
  "meta": {"hasUnreadMessages": false, "hasUnreadNotifications": false}
}
```

상품 seller에는 신원을 넣지 않는다. 성사 후 신원은 거래·대화 관계로 검사하여 반환한다. canApply는 본인 상품·진행 거래·공개·모집·수량 조건을 반영한다.

### 이미지 업로드 — 제안

`POST /images` multipart 파일 → 201 `{id, url}`. 본인 소유의 검증된 미연결 imageId를 상품에 연결한다. 교체 시 기존 이미지는 연결을 해제하고, 보존·삭제 시점은 추후 정한다. 파일명·EXIF·URL에 신원이 노출되지 않게 처리한다. 사진 크기·개수·형식 제한은 미정이다.

## 4. 거래 신청·수락·완료

| Method | 경로 | 권한·동작 |
|---|---|---|
| POST | `/products/{productId}/deals` | 구매자. `{quantity}` → REQUESTED, 201. 신청 별칭 배정 |
| GET | `/me/deals?status=&cursor=&limit=` | 본인 구매 신청·거래 목록 |
| GET | `/products/{productId}/deals?status=&cursor=&limit=` | 판매자만 해당 상품 신청·거래 목록 |
| GET | `/deals/{dealId}` | 해당 판매자·구매자만 상세 |
| PATCH | `/deals/{dealId}/quantity` | 구매자·REQUESTED만. `{quantity}`, 수정 시각 기록 |
| POST | `/deals/{dealId}/withdraw` | 구매자·REQUESTED → WITHDRAWN |
| POST | `/deals/{dealId}/reject` | 판매자·REQUESTED → REJECTED, 신청자 알림 |
| POST | `/deals/{dealId}/accept` | 판매자·REQUESTED → ACCEPTED, 수량 차감·단가/수량 확정·신원 공개 |
| POST | `/deals/{dealId}/complete` | 판매자·ACCEPTED → COMPLETED, 추가 차감 없음 |

- 자기 상품 신청 금지. quantity는 양의 정수. **제안:** 신청·수정도 현재 잔여 수량 이하, 진행 중 거래는 상품·구매자별 한 건.
- 모집 종료는 새 신청만 막으며 기존 신청 수락은 허용한다. 숨김 상품의 대기 신청 수락은 차단하되 이미 성사된 거래 완료는 허용하는 것을 제안한다.
- 수락은 서버가 현재 상품 단가와 최신 신청 수량을 확정한다. accept 요청에 단가·수량을 받지 않는다.
- 수락·거절·철회·완료의 반복 호출은 이미 같은 상태면 기존 결과 반환, 다른 불가능 상태면 409. 재고 차감·알림을 중복 수행하지 않는다.
- 수락 후 취소 API 없음. 상품 버전·신청 당시 상품 복사본 없음.
- **제안:** 재신청은 새 dealId를 발급하고 상품 내 기존 별칭·대화를 유지한다.

성사 거래 상세 예(판매자 관점):

```json
{
  "data": {
    "id": "deal-id",
    "productId": "product-id",
    "conversationId": "conversation-id",
    "status": "ACCEPTED",
    "requestedQuantity": 2,
    "agreedUnitPrice": 10000,
    "agreedQuantity": 2,
    "totalPrice": 20000,
    "requestedAt": "2026-11-04T10:10:00Z",
    "quantityUpdatedAt": null,
    "acceptedAt": "2026-11-04T10:20:00Z",
    "counterparty": {
      "label": "익명 1",
      "identity": {"realName": "홍길동", "department": "회사 정의 코드", "jobLevel": "회사 정의 코드"}
    }
  },
  "meta": {"hasUnreadMessages": false, "hasUnreadNotifications": false}
}
```

수락 전 합의 필드·totalPrice·acceptedAt은 null, counterparty.identity도 null이다. 상품 현재 정보와 거래 확정 단가·수량은 구분한다. **제안:** 같은 상품에서 이미 신원을 공개한 두 사람은 이후 대화·재신청에서도 공개 상태를 유지한다.

신청 생성·쪽지 발송에는 `Idempotency-Key`를 제안한다. 사용자·경로·키 기준 최초 요청 해시·결과를 저장하고 같은 키의 다른 본문은 409다. 그래야 완료·철회 직후 지연된 네트워크 재시도가 새 신청을 만들지 않는다. 저장소·키 보관 기간은 미정이다.

## 5. 쪽지

| Method | 경로 | 동작 |
|---|---|---|
| POST | `/products/{productId}/conversations` | 구매자 문의 시작. 최초 대화 201, 기존 대화 200 |
| GET | `/me/conversations?cursor=&limit=` | 본인 대화·별칭·미확인 여부 |
| GET | `/conversations/{conversationId}/messages?cursor=&limit=` | 해당 두 당사자만 메시지 조회 |
| POST | `/conversations/{conversationId}/messages` | `{body}` 쪽지 발송, 201. 송수신자는 서버 결정 |
| POST | `/conversations/{conversationId}/read` | `{throughMessageId}`까지 본인이 수신한 쪽지 읽음 처리 |

- 별도 거래 신청 없이 문의 가능. 신청하면 같은 대화에 ‘문의자 A’ → ‘익명 1’ 별칭 전환.
- **제안:** 새 대화는 공개·비삭제 상품에서만 생성하고 기존 대화는 모집 종료·숨김 후에도 유지한다. 메시지는 텍스트만 지원한다.
- 메시지 발신자는 SELF/COUNTERPARTY·별칭으로 표현한다. 전역 사용자 ID 없음. 성사 상대에게만 실명·부서·직급 제공.
- 조회만으로 읽음 처리하지 않는다. 화면 표시 후 read 요청을 보낸다. throughMessageId는 해당 대화 메시지여야 하며 그 이후 도착한 메시지는 읽음 처리하지 않는다.
- 진입·수동 새로고침 조회. 고정 폴링·웹소켓 없음. 기존 업무 요청의 meta로 미확인 표시 갱신.

## 6. 알림 — 앱 내 알림함

| Method | 경로 | 동작 |
|---|---|---|
| GET | `/me/notifications?cursor=&limit=` | 본인 알림 목록 |
| POST | `/me/notifications/{notificationId}/read` | 본인 알림 읽음 처리 |

APPLICATION_REJECTED 알림은 id·dealId·productId·createdAt·readAt을 반환한다. 거절과 알림은 같은 트랜잭션이며 거래당 한 건만 생성한다. 상품이 숨겨져도 본인 거래 기록으로 이동 가능하다. 외부 이메일·구글 챗 발송은 포함하지 않는다.

## 7. 인사팀

| Method | 경로 | 동작 |
|---|---|---|
| GET | `/admin/products?approvalStatus=PENDING&cursor=&limit=` | 심사 목록 |
| GET | `/admin/products/{productId}` | 상품·작성자 실명·부서·직급 |
| POST | `/admin/products/{productId}/approve` | PENDING → APPROVED, 심사 이력 |
| POST | `/admin/products/{productId}/reject` | `{reason}` 필수, PENDING → REJECTED, 심사 이력 |
| GET | `/admin/products/{productId}/participants` | 상품별 구매자 신원 열람 |

반려 사유는 판매자의 상품 상세에서 조회한다. 운영자의 쪽지 내용 열람·강제 거래 종료·사용자 제재는 미확정이므로 API에 포함하지 않는다. 운영자 역할 지정 방식과 승인·반려 알림은 미정이다.

## 8. 인사팀 거래 완료 금액

`GET /admin/statistics/completed-deals?from=2026-11-01&to=2026-11-30`

- HR_ADMIN 전용. from·to는 필수 YYYY-MM-DD, from ≤ to. 잘못된 날짜·역순은 400 VALIDATION_ERROR.
- 초안 기본 시간대 Asia/Seoul. 양 끝 날짜 포함. 내부적으로 `[시작일 00:00, 종료일 다음 날 00:00)` UTC 구간으로 completed_at을 필터링한다.
- COMPLETED만 조회해 agreed_unit_price × agreed_quantity 합산. 신청 시각·수락 시각·현재 상품 가격은 사용하지 않는다. 상품 숨김 여부와 무관하다.
- 별도 집계 테이블 없이 요청 시 SQL SUM·COUNT. 0원 거래는 건수에 포함. 빈 결과는 0이다.

```json
{
  "data": {
    "from": "2026-11-01",
    "to": "2026-11-30",
    "timezone": "Asia/Seoul",
    "currency": "KRW",
    "completedDealCount": 12,
    "totalCompletedAmount": 340000
  },
  "meta": {"hasUnreadMessages": false, "hasUnreadNotifications": false}
}
```

통계 금액·곱셈·합계의 안전한 숫자 범위를 구현 시 검증한다. 실제 결제 검증·정산 API는 아니다.

## 9. 구현 전 확정 사항

OpenAPI·DB 마이그레이션 작성 전 도메인·enum 값, 필드 길이·최댓값, 세션 회수, 개인정보 수정·보관, 이미지 제한, 멱등성 보관 정책 및 PRD의 제안 사항을 확정한다.
