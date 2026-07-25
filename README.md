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

| Test | Throughput | p95 Latency | Error Rate |
|------|-----------|-------------|-----------|
| Load (50 VUs) | 121.40 req/s | 9.0ms | 0.00%* |
| Stress (500 VUs) | 416.74 req/s | 27.0ms | 0.00% |
| Spike (300 VUs) | 267.52 req/s | 39.7ms | 0.00% |

> \* Load test 409 Conflict responses from optimistic locking are expected, not failures.

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
