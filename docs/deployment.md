# Railway 단일 서비스 배포

## 운영 구성

- 서비스: [kch-relay production](https://railway.com/project/c2673811-428a-473f-9447-bea1aab47eb7/service/5f842e1b-a85a-48a9-a165-bc6521da9e96?environmentId=030a33b8-31c9-4176-95c0-2159999ceed2)
- 접속: https://kch-relay-production.up.railway.app
- 소스: `Mrbaeksang/kch-relay`의 `main`, 루트 `Dockerfile` / `railway.toml`.
- 싱가포르 `asia-southeast1-eqsg3a`, 복제본 1개. CPU 상한 1개·메모리 상한 500 MB이며 예약 사용량이나 월 요금 보장이 아니다.
- 영구 볼륨 `kch-relay-data` 1000 MB를 `/data`에 마운트한다. DB와 사진을 함께 저장한다.

Docker 다단계 빌드에서 React/Vite 화면과 Go 실행 파일을 만든다. 최종 이미지에는 빌드 도구·Node·Vite 개발 서버가 없고 Go 하나가 화면과 API를 모두 `0.0.0.0:$PORT`로 제공한다. SQLite는 Go에 포함된 라이브러리이므로 별도 DB 컨테이너가 없다. 공개 도메인은 컨테이너 포트 8080으로 연결된다.

## 환경 변수

| 변수 | 운영 값 |
| --- | --- |
| APP_ENV | production |
| AUTH_MODE | google (자격 증명 등록 및 해당 구현 배포 후 전환) |
| APP_ORIGIN | https://kch-relay-production.up.railway.app |
| PORT | 8080 |
| DATABASE_PATH | /data/market.db |
| UPLOAD_DIR | /data/uploads |
| WEB_DIR | /app/frontend/dist |

구글 로그인 운영에는 `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URI`가 필요하다. 콜백은 `APP_ORIGIN + /api/v1/auth/google/callback`과 정확히 같아야 한다. 실제 시크릿은 Railway 변수에만 보관한다. 이전 이미지에서는 google 모드로 시작할 수 없으므로 최초 전환은 구현 이미지 배포 성공 후 수행한다. 개발 로그인은 운영에서 허용하지 않는다.

최초 관리자는 실제 Google 로그인 → 인사팀 선택 → 가입 신청까지 완료한 뒤 운영자가 컨테이너에서 `./bootstrap-admin --email '정확한 구글 이메일'`을 실행한다. 활성 관리자가 이미 있거나 해당 이메일의 인사팀 승인 대기 신청이 없으면 실패한다. 이 명령은 HTTP로 노출되지 않는다. 이후에는 관리자 화면에서 승인한다. 관리자 부서는 이름이 아닌 고정 지정으로 구분한다.

## GitHub CI/CD

1. PR에서 프론트 테스트/빌드, Go 정적 검사/경쟁 조건 테스트/빌드, 실제 단일 컨테이너 빌드/영속성 검사가 모두 성공해야 main에 병합할 수 있다.
2. 관리자가 Squash 병합하면 `.github/workflows/ci.yml`이 main의 최종 커밋을 다시 검사한다.
3. Railway의 GitHub source는 main을 추적하며 **Wait for CI**가 켜져 있다. 해당 커밋의 GitHub Actions 워크플로가 성공한 뒤 Docker 이미지를 빌드하고 배포한다. 실패한 워크플로는 배포를 건너뛴다.
4. `/api/readyz`가 60초 안에 성공해야 배포가 준비된다. 이 경로는 실제 SQLite 연결을 검사한다.

별도 GitHub 배포 토큰·개인 Railway 토큰을 저장소나 GitHub secrets에 넣지 않는다. GitHub 저장소 연결과 Wait for CI는 Railway 서비스 설정이며 `railway.toml`만으로 새 프로젝트에 복제되지 않는다. 새 환경을 만들 때 두 설정을 다시 확인한다. Railway GitHub App은 저장소 접근 권한과 checks 권한이 필요하다.

전체 main 변경을 배포 대상으로 유지하므로 watch path 필터는 두지 않는다. 마이그레이션은 볼륨이 마운트된 Go 시작 시 실행하며 준비 확인 전에 끝낸다. Railway pre-deploy 단계에는 볼륨이 없으므로 SQLite 마이그레이션을 그 단계로 옮기지 않는다.

## 권한·종료·영속성

시작 스크립트는 Railway가 마운트한 `/data`와 `/data/uploads` 소유권을 맞춘 뒤 UID 10001의 app 사용자로 권한을 낮춰 Go를 실행한다. 기존 DB를 수동 복원할 때에는 DB·보조 파일과 사진도 UID 10001이 읽고 쓸 수 있도록 관리자가 소유권을 맞춘다. 권한 오류를 임시 디스크 저장으로 우회하지 않는다.

Go는 SIGTERM을 받으면 최대 10초 동안 요청을 정리한다. Railway의 drainingSeconds는 20초다. 같은 SQLite 볼륨으로 복제본을 늘리거나 여러 리전에 동시에 띄우지 않는다. 볼륨 서비스는 재배포 시 잠깐 중단될 수 있으며 무중단·고가용성을 제공하지 않는다. 재배포·재시작 때 기존 볼륨을 유지하고 볼륨·DB를 임의로 삭제하지 않는다.

## 백업과 복구

영구 볼륨은 백업을 대신하지 않는다. 운영 데이터 사용 전 SQLite 온라인 백업 API나 `VACUUM INTO`로 일관된 사본을 만들고 사진과 함께 별도 위치에 보관·복원 검증을 한다. 실행 중 DB 파일만 단순 복사하지 않는다. Railway 볼륨 백업의 플랜·보존 조건은 계정에서 확인한다. 코드 rollback은 DB와 사진을 과거 상태로 돌리지 않으므로 하위 호환 마이그레이션을 유지한다.

## 공식 문서

- [Docker 다단계 빌드](https://docs.docker.com/build/building/multi-stage/)
- [Railway Dockerfile 감지](https://docs.railway.com/builds/dockerfiles)
- [GitHub 자동 배포와 Wait for CI](https://docs.railway.com/deployments/github-autodeploys)
- [설정 파일·상태 확인·종료 대기](https://docs.railway.com/config-as-code/reference)
- [볼륨과 pre-deploy 제약](https://docs.railway.com/volumes)
- [볼륨 복제본·배포 중단 제약](https://docs.railway.com/volumes/reference)
- [지원 리전](https://docs.railway.com/deployments/regions)


### 익명 쪽지 마이그레이션

익명 쪽지의 idempotency_key와 고유 인덱스는 `00005_message_idempotency.sql`로 추가한다. 기존 운영 가입 마이그레이션 00004 다음에 실행하며 과거 00003 번호를 운영 DB에 뒤늦게 추가하지 않는다. 기존 쪽지와 계정·상품·사진은 보존한다. 기존 쪽지는 idempotency_key=NULL로 유지된다.
