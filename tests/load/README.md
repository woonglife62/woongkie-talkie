# Load Tests

k6-based load tests for woongkie-talkie.

## Install k6

**macOS:**
```bash
brew install k6
```

**Linux (Debian/Ubuntu):**
```bash
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update
sudo apt-get install k6
```

**Windows:**
```powershell
winget install k6 --source winget
```

**Docker:**
```bash
docker pull grafana/k6
```

## Prerequisites

The load tests expect a user `loadtest` / `loadtest123` to exist. Create it first:

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"loadtest","password":"loadtest123"}'
```

Also ensure at least one chat room exists so WebSocket tests can connect to it.

## Running Tests

Set `BASE_URL` to point at your running server (default: `http://localhost:8080`).

### HTTP API load test (50 VUs, 2 minutes)
```bash
k6 run tests/load/http-api.js
# or with custom URL:
BASE_URL=http://your-server:8080 k6 run tests/load/http-api.js
```

### WebSocket concurrent connections (100 VUs, 2 minutes)
```bash
k6 run tests/load/websocket.js
# or:
BASE_URL=http://your-server:8080 k6 run tests/load/websocket.js
```

### Burst test (50 users, 500 messages in ~1 second)
```bash
k6 run tests/load/burst.js
# or:
BASE_URL=http://your-server:8080 k6 run tests/load/burst.js
```

### Run with Docker
```bash
docker run --rm -i grafana/k6 run - < tests/load/http-api.js
# With env var:
docker run --rm -i -e BASE_URL=http://host.docker.internal:8080 grafana/k6 run - < tests/load/http-api.js
```

## pprof Profiling (Dev Mode)

When `IS_DEV=dev`, the server exposes a pprof endpoint on port 6060.

```bash
# CPU profile (30 seconds)
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Memory/heap profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine profile
go tool pprof http://localhost:6060/debug/pprof/goroutine

# Web UI (requires Graphviz)
go tool pprof -http=:8090 http://localhost:6060/debug/pprof/profile?seconds=10
```

Run load tests while profiling to identify bottlenecks:

```bash
# Terminal 1: start server in dev mode
IS_DEV=dev ./woongkie-talkie serve

# Terminal 2: run load test
k6 run tests/load/websocket.js

# Terminal 3: capture CPU profile during load
go tool pprof -http=:8090 http://localhost:6060/debug/pprof/profile?seconds=30
```
