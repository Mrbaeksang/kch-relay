# KCH Relay 기술 스택

상태: 사용자 선택 반영. 비용 최소화뿐 아니라 개발·배포·유지보수 시간을 함께 줄입니다.

| 영역 | 선택 |
|---|---|
| 구조 | 모노레포 frontend/ + backend/ |
| 화면 | Vite + React + TypeScript, 정적 빌드 |
| API | Go + chi, 정적 파일과 같은 프로세스·주소 |
| DB | SQLite + modernc 드라이버 + goose 마이그레이션 |
| 사진 | /data/uploads, 별도 S3 없음 |
| 실행 | 루트 Dockerfile의 단일 이미지·Go 프로세스 |
| 배포 | Railway 단일 서비스·영구 볼륨 |
| 쪽지·알림 | HTTP 요청 및 앱 내 알림함, 고정 폴링·실시간 서버 없음 |
| 통계 | 완료일 인덱스로 SQL 합산, 별도 집계 서버 없음 |

프론트 빌드에만 Node가 필요하며 운영 이미지에는 Node·Vite 서버가 없습니다. Go 서버 하나가 정적 화면과 API를 제공합니다. SQLite·사진은 영구 볼륨으로 유지합니다.

서버는 메모리·CPU·저장·전송량에 따라 과금됩니다. 실제 메모리·CPU는 구현 후 측정하며 특정 월 요금을 보장하지 않습니다. 사진 압축·불필요한 요청 제거·인덱스·단일 프로세스로 사용량을 줄입니다. 운영 백업은 별도로 마련합니다.

앞선 Cloudflare Workers·D1·R2 제안은 채택하지 않습니다. 플랫폼 별도 학습·연동보다 사용자가 제안한 단일 Docker 구성을 우선합니다.

[로컬 개발](local-development.md) · [배포·백업](deployment.md) · [결정 기록](adr/0002-single-container-on-railway.md)
