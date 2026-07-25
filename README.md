> ## 📘 **For deep technical details, data flow, and trade-offs, please read the [Architecture Documentation](./ARCHITECTURE.md).**


# Graph Task Management API

REST API for managing tasks (do-to). Built with **Go**, **Gin**, **PostgreSQL**, and **Redis**.

---

## Architecture

![Architecture Diagram](docs/architecture.png)

> For interactive version, open `docs/architecture.html` in a browser. To regenerate from source, install [mermaid-cli](https://github.com/mermaid-js/mermaid-cli) and run:
> ```bash
> npx @mermaid-js/mermaid-cli -i docs/architecture.mmd -o docs/architecture.png -b white -w 1200
> ```

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
| `GET` | `/tasks/:id` | Get task by ID |
| `PATCH` | `/tasks/:id` | Update task |
| `DELETE` | `/tasks/:id` | Delete task |
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

### List Tasks (with Pagination & Filters)
```bash
curl "http://localhost:8080/tasks?page_number=1&page_size=10&status=TODO&assignee=Abolfazl"
```

**Response (200):**
```json
{
  "status": "success",
  "data": {
    "pagination": {
      "page_number": 1,
      "page_size": 10,
      "total": 5
    },
    "tasks": [...]
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
# If version is outdated:
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

Coverage: **~88%** (service: 93.6%, postgres: 81.4%, redis: 77.3%)

---

## API Documentation

Swagger UI: `http://localhost:8080/swagger/index.html`

Raw specs: `docs/swagger.json`, `docs/swagger.yaml`

---

## Observability

- **Metrics:** `http://localhost:8080/metrics` (Prometheus format)
- **Prometheus:** `http://localhost:9090` (via Docker)
- **Tracing:** OpenTelemetry spans in logs (trace_id, span_id)

Key metrics: `tasks_count`, `request_latency_histogram`, `requests_total`

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

Full performance test suite using [k6](https://k6.io/). Install with `brew install k6`.

| Test | VUs | Duration | Goal |
|------|-----|----------|------|
| **Load** | 0→50→0 | ~130s | Normal production traffic within SLA |
| **Stress** | 0→5→20→50→100→200→500→0 | ~4min | Find breaking point |
| **Spike** | 5→300→5→0 | ~2min | Flash sale / viral burst recovery |
| **Soak** | 0→20 (10min sustained) | ~11min | Memory leaks, connection pool exhaustion |

### Run

```bash
make k6-load
make k6-stress
make k6-spike
make k6-soak
make k6-all
```

### Results

| Test | Total Requests | Throughput | Avg | p50 | p90 | p95 | p99 | Max | Error Rate | Verdict |
|------|---------------|-----------|-----|-----|-----|-----|-----|-----|-----------|---------|
| **Load** (50 VUs) | 15,925 | 120.43 req/s | 4.7ms | <1ms | 8.5ms | 10.8ms | 15.2ms | 66.1ms | 0.00%* | PASS |
| **Stress** (500 VUs) | 82,157 | 409.61 req/s | 12.9ms | <1ms | 33.8ms | 52.8ms | 108ms | 368.4ms | 0.00% | PASS |

> \* Load test 409 Conflict responses from optimistic locking are expected, not failures.

### Metric Glossary

| Metric | What It Means |
|--------|--------------|
| **VUs (Virtual Users)** | تعداد کاربران همزمان مجازی که درخواست ارسال می‌کنند. هر VU یک loop متوالی از درخواست‌ها اجرا می‌کند |
| **Throughput (req/s)** | تعداد درخواست‌های موفق در ثانیه. نشان‌دهنده ظرفیت واقعی سرویس است |
| **Avg Latency** | میانگین زمان پاسخ‌دهی. شامل سریع‌ترین و کندترین درخواست‌ها |
| **p50 (Median)** | ۵۰٪ درخواست‌ها زیر این مقدار پاسخ می‌دهند. نشان‌دهنده رفتار «عادی» سرویس |
| **p90** | ۹۰٪ درخواست‌ها زیر این مقدار پاسخ می‌دهند |
| **p95** | ۹۵٪ درخواست‌ها زیر این مقدار پاسخ می‌دهند. مهم‌ترین معیار SLA |
| **p99** | ۹۹٪ درخواست‌ها زیر این مقدار پاسخ می‌دهند. نشان‌دهنده «بدترین حالت معقول» |
| **Max** | کندترین درخواست در کل تست. ممکن است outlier باشد (مثلاً cold start) |
| **Error Rate** | درصد درخواست‌های ناموفق (غیر 2xx). در این پروژه، 409 Conflict از optimistic locking خطا نیست |

### Detailed Analysis

#### Load Test — 50 VUs (ترافیک نرمال)

```
Phase:    Ramp-up (30s)  →  Sustain (90s)  →  Ramp-down (10s)
VUs:      0 → 50          →  50 constant     →  50 → 0
```

**نتیجه:** با ۵۰ کاربر همزمان، سرویس **۱۲۰ درخواست در ثانیه** با **p95 زیر ۱۱ میلی‌ثانیه** پاسخ می‌دهد.

- **p50 < 1ms:** نیمی از درخواست‌ها تقریباً آنی هستند — این نشان‌دهنده کارایی Redis cache است
- **p95 = 10.8ms:** ۹۵٪ درخواست‌ها زیر ۱۱ms پاسخ می‌دهند — بسیار بهتر از SLA معمول (<200ms)
- **p99 = 15.2ms:** حتی ۹۹٪ درخواست‌ها هم زیر ۱۵ms هستند — gap بین p95 و p99 کم است یعنی latency distribution یکنواخت است
- **Max = 66.1ms:** کندترین درخواست — احتمالاً cold cache یا اولین درخواست بعد از ramp-up
- **Error Rate = 0%:** تمام درخواست‌ها موفق بوده‌اند (409 Conflict ها محاسبه نشده‌اند)

#### Stress Test — 500 VUs (فشار حداکثری)

```
Phase:    Warm → Normal → Heavy → Stress → Extreme → Recovery → Down
VUs:      5  →  20    →  50   →  100  →  200    →  500     →  5  →  0
```

**نتیجه:** با **۵۰۰ کاربر همزمان**، سرویس **۴۱۰ درخواست در ثانیه** با **p95 زیر ۵۳ms** پاسخ می‌دهد و **صفر خطا** دارد.

- **p50 < 1ms:** حتی با ۵۰۰ VU، نیمی از درخواست‌ها همچنان زیر ۱ms هستند — cache hit rate بالا
- **p90 = 33.8ms:** ۹۰٪ درخواست‌ها زیر ۳۴ms — قابل قبول برای ۵۰۰ کاربر
- **p95 = 52.8ms:** با ۱۰ برابر load نسبت به تست قبل، p95 فقط ۵ برابر شده (۱۰ms → ۵۳ms) — خطی و قابل پیش‌بینی
- **p99 = 108ms:** حتی بدترین ۱٪ درخواست‌ها هم زیر ۱۱۰ms هستند — زیر SLA ۵۰۰ms
- **Max = 368ms:** کندترین درخواست در اوج فشار — احتمالاً مربوط به connection pool waiting
- **Error Rate = 0%:** صفر خطا حتی در ۵۰۰ VU — connection pool (25 conns) کافی بوده

#### مقایسه Load vs Stress

```
VUs:        50        500      (۱۰ برابر)
Throughput: 120       410      (۳.۴ برابر)
p95:        11ms      53ms     (۴.۸ برابر)
Error:      0%        0%       (بدون تغییر)
```

**تحلیل:** وقتی load ۱۰ برابر می‌شود، throughput فقط ۳.۴ برابر شده — این نشان‌دهنده bottleneck در connection pool PostgreSQL (25 connection) است. اگر pool را به ۵۰-۱۰۰ افزایش دهیم، throughput تقریباً خطی رشد می‌کند. اما p99 همچنان زیر SLA هست و error rate صفر است.

#### پیش‌بینی ظرفیت

بر اساس نتایج:
- **نرمال:** تا ۱۰۰ VU / ۲۰۰ req/s — بدون هیچ مشکلی
- **Heavy:** تا ۳۰۰ VU / ۳۵۰ req/s — p95 زیر ۴۰ms
- **Extreme:** ۵۰۰ VU / ۴۱۰ req/s — p95 زیر ۵۳ms
- **Breaking Point:** پیدا نشد — سرویس حتی با ۵۰۰ VU پایدار است

---

## Benchmark & pprof

### Run Benchmarks

```bash
make test-bench
```

### Benchmark Results

```
Benchmark           Iterations    ns/op        B/op      allocs/op
───────────────────────────────────────────────────────────────────
BenchmarkCreateTask    64,928     52,146      28,904       309
BenchmarkGetTask      147,530     23,345      12,554       147
BenchmarkUpdateTask    65,786     48,733      28,137       307
BenchmarkDeleteTask    79,022     44,716      22,590       244
BenchmarkListTask      57,543     61,552      33,504       380
```

### pprof Analysis

```bash
# Generate profiles
make test-bench

# CPU profile
go tool pprof cpu.prof

# Memory profile
go tool pprof mem.prof

# Top functions
go tool pprof -top cpu.prof
go tool pprof -top mem.prof
```
