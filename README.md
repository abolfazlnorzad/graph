> **For deep technical details, data flow, and trade-offs, see the [Architecture Documentation](./ARCHITECTURE.md).**

# Graph Task Management API

A REST API for managing tasks (to-do). Built with **Go**, **Gin**, **PostgreSQL**, **Redis**, and **OpenTelemetry**.

---

## Architecture

![Architecture Diagram](docs/architecture.png)


---

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Make (optional)

### Run with Docker
```bash
git clone https://github.com/abolfazlnorzad/graph.git
cd graph
make docker-up
```

API available at `http://localhost:8080`

### Run locally
```bash
make run
```

### Stop
```bash
make docker-down
```

---

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/tasks` | Create a task |
| `GET` | `/tasks/:id` | Get task by ID (with audit logs) |
| `PATCH` | `/tasks/:id` | Update task (optimistic locking) |
| `DELETE` | `/tasks/:id` | Delete task (soft delete) |
| `GET` | `/tasks` | List tasks (pagination + filters) |

---

## cURL Examples

### Create Task
```bash
curl -X POST http://localhost:8080/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Setup CI/CD Pipeline",
    "description": "Configure GitHub actions",
    "status": "TODO",
    "assignee": "Abolfazl"
  }'
```

**Response (201):**
```json
{
  "status": "success",
  "data": {
    "result": {
      "id": 1,
      "title": "Setup CI/CD Pipeline",
      "status": "TODO",
      "assignee": "Abolfazl",
      "version": 1,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  }
}
```

### Get Task with Audit Logs
```bash
curl "http://localhost:8080/tasks/1?audit_page=1&audit_page_size=10"
```

**Response (200):**
```json
{
  "status": "success",
  "data": {
    "result": {
      "id": 1,
      "title": "Setup CI/CD Pipeline",
      "status": "TODO",
      "audit_logs": [
        {
          "id": 1,
          "task_id": 1,
          "action": "CREATE",
          "previous_state": null,
          "new_state": {"title": "Setup CI/CD Pipeline", "status": "TODO"},
          "created_at": "2024-01-01T00:00:00Z"
        }
      ],
      "audit_pagination": {
        "page_size": 10,
        "page_number": 1,
        "total": 1
      }
    }
  }
}
```

### List Tasks (with Pagination & Filters)
```bash
curl "http://localhost:8080/tasks?page_number=1&page_size=10&status=TODO&assignee=Abolfazl"
```

**Response (200):**
```json
{
  "status": "success",
  "data": {
    "result": {
      "pagination": {
        "page_number": 1,
        "page_size": 10,
        "total": 5
      },
      "tasks": [...]
    }
  }
}
```

### Update Task (Optimistic Locking)
```bash
curl -X PATCH http://localhost:8080/tasks/1 \
  -H "Content-Type: application/json" \
  -d '{"title": "Updated Title", "status": "IN_PROGRESS", "version": 1}'
```

**Response (200):**
```json
{
  "status": "success",
  "data": {
    "result": {
      "id": 1,
      "title": "Updated Title",
      "status": "IN_PROGRESS",
      "version": 2
    }
  }
}
```

### Version Conflict (409)
```bash
curl -X PATCH http://localhost:8080/tasks/1 \
  -H "Content-Type: application/json" \
  -d '{"title": "Fail", "version": 1}'
```

**Response (409):**
```json
{
  "status": "error",
  "message": "error.conflict",
  "code": "error.conflict"
}
```

### Delete Task
```bash
curl -X DELETE http://localhost:8080/tasks/1
# Response: 204 No Content
```

---

## Testing

```bash
make test              # All tests
make test-unit         # Unit tests only
make test-integration  # Integration tests (requires Docker)
make test-coverage     # Coverage report (HTML)
make test-race         # Race detector
```

**Coverage:** ~85% (service: 91.9%, postgres: 77.5%, redis: 77.3%, pkg: 87-94%)

---

## API Documentation

- Swagger UI: `http://localhost:8080/swagger/index.html`
- Raw specs: `docs/swagger.json`, `docs/swagger.yaml`

---

## Observability

> All observability signals — metrics, traces, and logs — can be viewed and correlated in **Grafana** dashboards. Access at `http://localhost:3000` (admin/admin).

### Metrics (OTLP via OTel Collector)
- **Prometheus:** `http://localhost:9090`
- **Metrics endpoint:** `http://localhost:8080/metrics`

Key metrics: `requests_total`, `request_latency_histogram`, `tasks_count`, `task_created_total`, `task_fetched_total`

### Tracing (OpenTelemetry → Tempo)
- **Tempo:** `http://localhost:3200`
- All HTTP requests and DB operations are traced
- Click `trace_id` in logs → navigate to Tempo

### Logging (Loki + Promtail)
- **Loki:** `http://localhost:3100`
- Structured JSON logs with `trace_id`, `span_id`, `op`, `level`

### Grafana Dashboards

All observability data is available through pre-configured Grafana dashboards:

- **Graph - Application:** Request rate, latency (p50/p95/p99), business metrics, traces
- **Graph - Infrastructure:** Go runtime, PostgreSQL, Redis, Service Map
- **Graph - Logs:** Log volume, error logs, live stream, trace correlation

---

## Make Commands

```bash
make help             # Show all commands
make build            # Build binary
make run              # Run locally
make docker-up        # Start all services
make docker-down      # Stop all services
make swagger          # Regenerate swagger docs
make mocks            # Regenerate mocks
make clean            # Clean build artifacts
make k6-load          # Run k6 load test
make k6-stress        # Run k6 stress test
make k6-spike         # Run k6 spike test
make k6-all           # Run all k6 tests
```

---

## k6 Performance Tests

Full performance test suite using [k6](https://k6.io/)

| Test | VUs | Duration | Goal |
|------|-----|----------|------|
| **Load** | 0→50→0 | ~130s | Normal production traffic within SLA |
| **Stress** | 0→5→20→50→100→200→500→0 | ~4min | Find breaking point |
| **Spike** | 5→1000→5→0 | ~2min | Flash sale / viral burst recovery |

### Results

| Test | Requests | Throughput | p95 | Error Rate |
|------|----------|------------|-----|------------|
| **Load** (50 VUs) | 15,988 | 121.19 req/s | 10.2ms | 0.00% |
| **Stress** (500 VUs) | — | 375.01 req/s | 226.3ms | 0.00% |
| **Spike** (1000 VUs) | — | 395.63 req/s | 2138.0ms | 0.00% |

### Metric Glossary

| Metric | Description |
|--------|-------------|
| **VUs** | Virtual Users — concurrent users sending requests |
| **Throughput** | Successful requests per second |
| **p95** | 95th percentile latency — the most important SLA metric |
| **Error Rate** | Percentage of failed requests (409 Conflict is not an error) |

### Understanding VUs and Ramp Phases

**VUs (Virtual Users)** simulate concurrent users. Each VU runs a loop of requests independently.

**Ramp-up** gradually increases VUs to avoid shocking the system:
```
0 → 50 VUs over 30s = ~1.7 VUs added per second
```

**Sustain** holds steady VUs to measure stable performance:
```
50 VUs for 90s = sustained load
```

**Ramp-down** gradually decreases VUs:
```
50 → 0 VUs over 10s = ~5 VUs removed per second
```

**Why ramp?** Sudden spikes can cause connection pool exhaustion, cache stampedes, and cascading failures. Ramp-up mimics real-world traffic patterns.

---

## Benchmark & pprof

Benchmarks measure service layer performance with mock dependencies — no network or DB overhead.

### Run Benchmarks

```bash
make test-bench
```

### Benchmark Results

**Environment:** Apple M2 Pro, 12 cores, macOS Darwin (arm64)

```
Benchmark              Iterations    ns/op        B/op      allocs/op
──────────────────────────────────────────────────────────────────────
BenchmarkGetTask         26,546     45,416      18,989       221
BenchmarkCreateTask      20,664     57,185      29,327       328
BenchmarkUpdateTask      21,500     64,051      29,668       341
BenchmarkDeleteTask      23,926     46,487      22,436       244
BenchmarkListTask        18,474     64,172      33,397       380
```

### Benchmark Glossary

| Metric | Description |
|--------|-------------|
| **Iterations** | Number of times the test was executed |
| **ns/op** | Nanoseconds per operation |
| **B/op** | Bytes allocated per operation |
| **allocs/op** | Memory allocations per operation |

### pprof Analysis

```bash
# Generate profiles
make test-bench

# CPU profile
go tool pprof cpu.prof

# Memory profile
go tool pprof mem.prof

# Top functions by CPU time
go tool pprof -top cpu.prof

# Visual web UI (requires graphviz)
go tool pprof -http=:8081 cpu.prof
```

### Benchmark vs k6 Load Test

| Metric | Benchmark (service layer) | k6 Load Test (Docker) |
|--------|--------------------------|----------------------|
| GetTask | 45μs | ~5ms (p50) |
| CreateTask | 57μs | ~10ms |
| ListTask | 64μs | ~15ms |
| **Overhead** | — | **100-200x** |

The overhead is expected: k6 tests the full stack including HTTP, Gin routing, JSON serialization, network latency, Redis, and PostgreSQL.

### What Benchmarks Measure

Benchmarks test the **service layer in isolation** with mock dependencies:
- No network latency
- No actual database queries
- No JSON serialization/deserialization
- No HTTP middleware

This tells you the **theoretical minimum latency** of your business logic. The gap between benchmark and k6 shows the real-world overhead of the full stack.

### What k6 Measures

k6 tests the **entire system** under realistic conditions:
- HTTP requests over the network
- Gin routing and middleware
- JSON serialization/deserialization
- PostgreSQL queries with real data
- Redis cache operations
- Connection pool management
- Concurrent access patterns

This is what your users actually experience.

### pprof Analysis

pprof generates CPU and memory profiles to identify real bottlenecks.

```bash
# Generate profiles
go test -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof ./service/

# CPU profile — where CPU time is spent
go tool pprof -top cpu.prof

# Memory profile — where memory is allocated
go tool pprof -top -alloc_space mem.prof

# Visual web UI (requires graphviz)
go tool pprof -http=:8081 cpu.prof
```

#### CPU Profile — Top Functions

```
flat%   cum%   function
────────────────────────────────────────────────────
11.76%  11.76%  runtime.pthread_cond_signal    (goroutine scheduling)
 9.90%  21.66%  runtime.madvise                (memory allocation from OS)
 8.21%  29.86%  runtime.pthread_cond_wait      (goroutine waiting)
 6.94%  36.80%  runtime.pcvalue                (stack scanning)
 5.75%  42.55%  runtime.pthread_kill           (goroutine management)
 4.31%  46.87%  runtime.scanobject             (GC object scanning)
 3.89%  54.91%  runtime.step                   (stack unwinding)
 2.45%  57.36%  internal/bytealg.IndexByteString (string search in mock)
```

**Analysis:**
- **Top 8 functions are all Go runtime** — not application code. Business logic is optimized.
- **pthread_cond_signal/wait (20%)** — goroutine scheduling overhead from testify mock calls
- **runtime.madvise (9.9%)** — memory from OS. Normal for benchmarks with mocks
- **runtime.scanobject (4.3%)** — GC scanning objects. Indicates allocation pressure
- **IndexByteString (2.5%)** — mock reflection. Not present in production

#### Memory Profile — Top Allocators

```
flat      flat%    function
─────────────────────────────────────────────────────
901MB    22.46%   strings.genSplit
852MB    21.24%   fmt.Sprintf
770MB    19.18%   stretchr/testify/mock.(*Mock).MethodCalled
503MB    12.54%   stretchr/testify/assert.CallerInfo
206MB     5.14%   stretchr/testify/mock.(*Mock).Called
104MB     2.58%   github.com/go-ozzo/ozzo-validation/v4.findStructField
 83MB     2.07%   stretchr/testify/mock.Arguments.Diff
 71MB     1.77%   context.(*valueCtx).String
```

**Analysis:**
- **88% of memory is consumed by test framework** — not production code
- **strings.genSplit (22%)** — string splitting in mock assertions
- **fmt.Sprintf (21%)** — formatting error messages in mocks
- **testify/mock (37%)** — mock method call overhead
- **In production** (without mocks), memory usage would be 10-20x lower

---

