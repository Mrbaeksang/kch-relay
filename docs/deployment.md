# Railway 단일 서비스 배포

## 구성

GitHub 저장소 Mrbaeksang/kch-relay를 연결하고 루트 Dockerfile을 빌드합니다. railway.toml은 Docker 빌드와 /api/readyz 상태 확인을 지정합니다. 운영 프로세스는 Go 하나이며 프론트는 정적 파일입니다.

## 최초 설정

- Railway 서비스 하나, 복제본 1개.
- 서비스에 영구 볼륨을 추가하고 /data에 마운트.
- DATABASE_PATH=/data/market.db, WEB_DIR=/app/frontend/dist. PORT는 Railway 주입값을 사용합니다.
- 시작 스크립트가 마운트된 /data와 /data/uploads의 소유권을 설정하고 UID 10001로 권한을 낮춘 뒤 Go를 실행합니다. 기존 DB를 가져올 때 파일 소유권도 맞춥니다. 권한 오류를 임시 디스크 저장으로 우회하지 않습니다.
- Railway 제공 HTTPS 도메인 연결. Google 인증 구현 시 승인된 리다이렉트 URL·회사 도메인·자격 증명을 Railway 변수로 설정합니다.

현재 저장소는 개발 계정 로그인과 승인 상품 목록·상세 조회를 제공합니다. 운영 구글 인증·상품 등록/승인·거래 기능은 후속 구현입니다. 운영 기본값 AUTH_MODE=disabled에서는 직원 장터 API에 접근할 수 없습니다. APP_ORIGIN은 실제 HTTPS origin으로 설정해야 합니다. 본 설정 작업에서 Railway 프로젝트 생성·배포·결제 설정은 수행하지 않습니다.

## 영속성과 백업

컨테이너 재배포·재시작 후에도 /data를 유지해야 합니다. SQLite는 한 Go 프로세스에서 사용하며 외부 DB 서버나 S3는 필수가 아닙니다. 사진은 향후 /data/uploads에 저장하고 인증된 API로만 제공해야 합니다.

영구 볼륨은 백업을 대신하지 않습니다. 운영 개시 전 SQLite 온라인 백업 API나 VACUUM INTO로 일관된 사본을 만들고 사진과 함께 별도 위치에 보관·복원 검증을 합니다. 실행 중 DB 파일만 단순 복사하지 않습니다. Railway 볼륨 백업의 이용 플랜·보존 조건을 계정에서 확인합니다.

볼륨 서비스는 재배포 시 잠깐 중단될 수 있습니다. 다중 복제본·무중단·고가용성은 현재 범위에 없습니다. 수동으로 볼륨을 삭제하지 않습니다.

공식 참고: https://docs.railway.com/volumes/reference · https://docs.railway.com/volumes/backups · https://docs.railway.com/pricing
