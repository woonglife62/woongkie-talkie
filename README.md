# woongkie-talkie

실시간 다중 채팅방 웹 애플리케이션. WebSocket 기반 실시간 메시징, JWT 인증, Redis Pub/Sub을 통한 멀티 서버 브로드캐스트를 지원합니다.

## 기술 스택

| 분류 | 기술 |
|------|------|
| Language | Go 1.21 |
| Framework | Echo v4 |
| Database | MongoDB 7 |
| Cache/PubSub | Redis 7 |
| Auth | JWT (golang-jwt/v5) |
| Realtime | WebSocket (gorilla/websocket) |
| Logging | zap (structured logging) |
| Container | Docker, Docker Compose |
| Orchestration | Kubernetes |
| Load Test | k6 |

## 프로젝트 구조

```
woongkie-talkie/
├── cmd/                    # CLI 명령어 (cobra)
│   ├── root.go
│   └── serve.go
├── pkg/                    # 공유 패키지
│   ├── config/             # 환경 설정
│   ├── logger/             # 구조화 로깅 (zap)
│   ├── mongoDB/            # MongoDB 연결 및 스키마
│   └── redis/              # Redis 클라이언트
├── server/                 # HTTP/WS 서버
│   ├── handler/            # API 핸들러
│   ├── middleware/          # JWT 인증, Rate Limiting
│   └── router/             # 라우트 정의
├── view/                   # 프론트엔드 (HTML/CSS)
├── docs/                   # OpenAPI 스펙
├── k8s/                    # Kubernetes 매니페스트
├── tests/load/             # k6 부하 테스트
├── Dockerfile              # 멀티스테이지 빌드
├── docker-compose.yml      # MongoDB + Redis + App
└── Taskfile.yml            # go-task 자동화
```

## 빠른 시작

### 사전 요구사항

- [Go 1.21+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/) & Docker Compose
- [go-task](https://taskfile.dev/installation/) (선택, 권장)

### 환경 설정

```bash
# .env.example을 .env로 복사 (go-task 사용 시)
task env

# 또는 수동으로
cp .env.example .env
```

`.env` 파일을 열어 `JWT_SECRET` 등 값을 수정하세요.

### Docker로 실행 (권장)

```bash
# 전체 서비스 시작 (MongoDB + Redis + App)
task docker:up

# 로그 확인
task docker:logs

# 종료
task docker:down
```

브라우저에서 http://localhost:8080 접속.

### 로컬 개발

MongoDB와 Redis가 별도로 실행 중이어야 합니다.

```bash
# 의존성 다운로드
task mod

# 서버 실행
task run
```

## Task 명령어 목록

| 명령어 | 설명 |
|--------|------|
| `task` | 기본 실행 (`task run`과 동일) |
| `task build` | Go 바이너리 빌드 (`./bin/woongkie-talkie`) |
| `task run` | 로컬 서버 실행 |
| `task test` | 전체 테스트 실행 |
| `task test:handler` | 핸들러 테스트만 실행 |
| `task vet` | `go vet` 정적 분석 |
| `task lint` | vet + build 검증 |
| `task clean` | 빌드 아티팩트 삭제 |
| `task docker:up` | Docker Compose로 전체 서비스 시작 |
| `task docker:down` | Docker Compose 종료 |
| `task docker:logs` | Docker Compose 로그 팔로우 |
| `task docker:build` | Docker 이미지만 빌드 |
| `task env` | `.env.example` → `.env` 복사 |
| `task mod` | `go mod tidy` + `go mod download` |

## API 엔드포인트

### 인증

| Method | Path | 설명 |
|--------|------|------|
| POST | `/auth/register` | 회원가입 |
| POST | `/auth/login` | 로그인 (JWT 발급) |
| GET | `/auth/me` | 현재 사용자 정보 |

### 채팅방

| Method | Path | 설명 |
|--------|------|------|
| GET | `/rooms` | 채팅방 목록 |
| POST | `/rooms` | 채팅방 생성 |
| GET | `/rooms/:id` | 채팅방 상세 |
| DELETE | `/rooms/:id` | 채팅방 삭제 |
| GET | `/rooms/default` | 기본 채팅방 조회 |
| POST | `/rooms/:id/join` | 채팅방 참가 |
| POST | `/rooms/:id/leave` | 채팅방 나가기 |

### 메시지

| Method | Path | 설명 |
|--------|------|------|
| GET | `/rooms/:id/messages` | 메시지 목록 (무한스크롤) |
| PUT | `/rooms/:id/messages/:msgId` | 메시지 편집 |
| DELETE | `/rooms/:id/messages/:msgId` | 메시지 삭제 |
| POST | `/rooms/:id/messages/:msgId/reply` | 메시지 답장 |

### WebSocket

| Method | Path | 설명 |
|--------|------|------|
| GET | `/rooms/:id/ws` | 실시간 채팅 연결 |

### 프로필

| Method | Path | 설명 |
|--------|------|------|
| GET | `/users/:username/profile` | 사용자 프로필 조회 |
| PUT | `/users/me/profile` | 내 프로필 수정 |

### 시스템

| Method | Path | 설명 |
|--------|------|------|
| GET | `/health` | 헬스체크 |
| GET | `/ready` | 레디니스 체크 |
| GET | `/docs` | Swagger UI |
| GET | `/docs/openapi.yaml` | OpenAPI 스펙 |

## 주요 기능

- **JWT 인증** - 회원가입/로그인, 토큰 기반 인증
- **다중 채팅방** - 채팅방 생성, 참가, 나가기
- **실시간 메시징** - WebSocket 기반 실시간 채팅
- **Redis Pub/Sub** - 멀티 서버 환경에서 메시지 브로드캐스트
- **메시지 편집/삭제** - 본인 메시지 수정 및 삭제
- **메시지 답장** - 특정 메시지에 대한 답장
- **타이핑 인디케이터** - 상대방 입력 중 표시
- **프로필 관리** - 사용자 프로필 조회 및 수정
- **Presence** - 사용자 온/오프라인 상태
- **무한 스크롤** - 메시지 히스토리 페이지네이션
- **Rate Limiting** - API 요청 제한
- **구조화 로깅** - zap 기반 JSON 로깅

## 환경 변수

| 변수 | 설명 | 기본값 |
|------|------|--------|
| `IS_DEV` | 실행 모드 (`dev` / `prod`) | `dev` |
| `MONGODB_URI` | MongoDB 연결 URI | `mongodb://mongodb:27017` |
| `MONGODB_USER` | MongoDB 사용자 | `root` |
| `MONGODB_PASSWORD` | MongoDB 비밀번호 | `1234` |
| `MONGODB_DATABASE` | 데이터베이스 이름 | `woongkietalkie` |
| `JWT_SECRET` | JWT 서명 키 (32자 이상) | - |
| `JWT_EXPIRY` | JWT 만료 시간 | `24h` |
| `REDIS_ADDR` | Redis 주소 | `localhost:6379` |
| `REDIS_PASSWORD` | Redis 비밀번호 | (빈 값) |
| `REDIS_DB` | Redis DB 번호 | `0` |
| `TLS_CERT_FILE` | TLS 인증서 경로 (선택) | - |
| `TLS_KEY_FILE` | TLS 키 경로 (선택) | - |

## 배포

### Docker Compose (기본)

```bash
task docker:up
```

MongoDB, Redis, 애플리케이션이 함께 시작됩니다. 리소스 제한과 헬스체크가 설정되어 있습니다.

### Kubernetes

`k8s/` 디렉토리에 매니페스트가 준비되어 있습니다:

- `namespace.yaml` - 네임스페이스
- `configmap.yaml` - 환경 설정
- `secret.yaml` - 시크릿
- `deployment.yaml` - 디플로이먼트
- `service.yaml` - 서비스
- `ingress.yaml` - 인그레스
- `hpa.yaml` - 오토스케일링

자세한 내용은 [k8s/README.md](k8s/README.md)를 참고하세요.

## 테스트

```bash
# 전체 테스트
task test

# 핸들러 테스트만
task test:handler
```

### 부하 테스트 (k6)

```bash
# HTTP API 부하 테스트
k6 run tests/load/http-api.js

# WebSocket 부하 테스트
k6 run tests/load/websocket.js

# Burst 테스트
k6 run tests/load/burst.js
```

자세한 내용은 [tests/load/README.md](tests/load/README.md)를 참고하세요.

## 라이선스

Copyright 2023 woonglife
