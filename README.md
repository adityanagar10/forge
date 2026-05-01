# Forge

A PostgreSQL-backed job queue system with workers, retry logic, dead-letter queue, and a real-time dashboard.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Architecture                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌──────────┐         ┌──────────────┐         ┌──────────────────┐       │
│   │  Client  │ ──────► │   REST API   │ ──────► │    PostgreSQL    │       │
│   │          │  HTTP   │   :8080      │   SQL   │    jobs table    │       │
│   └──────────┘         └──────────────┘         └────────┬─────────┘       │
│                                                          │                  │
│   ┌──────────────────────────────────────────────────────┼─────────────┐   │
│   │                      Worker Pool                     │             │   │
│   │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐ │             │   │
│   │  │Worker 1 │  │Worker 2 │  │Worker 3 │  │Worker N │◄┘  Dequeue   │   │
│   │  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘   (advisory  │   │
│   │       │            │            │            │         locks)    │   │
│   │       ▼            ▼            ▼            ▼                   │   │
│   │   ┌─────────────────────────────────────────────┐                │   │
│   │   │              Job Handlers                   │                │   │
│   │   │   email  │  webhook  │  report  │  ...     │                │   │
│   │   └─────────────────────────────────────────────┘                │   │
│   └──────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│   ┌──────────────┐                                                         │
│   │  Dashboard   │  Next.js + Tailwind                                     │
│   │   :3000      │  Real-time stats, DLQ management, job queueing          │
│   └──────────────┘                                                         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

forge/
├── cmd/
│   └── server/
│       └── main.go          # Entry point
├── internal/
│   ├── api/
│   │   └── server.go        # REST API handlers
│   ├── job/
│   │   └── job.go           # Job struct and status types
│   ├── store/
│   │   ├── store.go         # JobStore interface
│   │   ├── postgre.go       # PostgreSQL implementation
│   │   └── migrations/
│   │       └── 001_create_jobs_table.sql
│   └── worker/
│       └── worker.go        # Worker pool
├── dashboard/               # Next.js dashboard
├── docker-compose.yml
├── go.mod
└── README.md
```

## Features

- **PostgreSQL Backend** - Durable job storage with ACID guarantees
- **Advisory Locks** - Safe concurrent dequeuing with `pg_try_advisory_xact_lock`
- **Worker Pool** - Configurable concurrency with graceful shutdown
- **Retry with Exponential Backoff** - Failed jobs retry with increasing delays
- **Dead Letter Queue** - Jobs that exhaust retries move to DLQ for inspection
- **Priority Queues** - Jobs processed by priority (higher first)
- **Scheduled Jobs** - Queue jobs to run at a future time
- **REST API** - Enqueue, inspect, and manage jobs via HTTP
- **Real-time Dashboard** - Monitor queue health, retry failed jobs

## Quick Start

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- Node.js 18+ (for dashboard)

### 1. Start PostgreSQL

```bash
docker-compose up -d
```

### 2. Run Migrations

```bash
psql postgres://jobqueue:jobqueue@localhost:5432/jobqueue \
  -f internal/store/migrations/001_create_jobs_table.sql
```

### 3. Start the Server

```bash
go run ./cmd/server
```

### 4. Start the Dashboard

```bash
cd dashboard
npm install
npm run dev
```

Open http://localhost:3000 to see the dashboard.

## API Reference

### Enqueue a Job

```bash
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "payload": {"to": "user@example.com", "subject": "Hello"},
    "priority": 10,
    "max_retries": 5
  }'
```

### Get Job Status

```bash
curl http://localhost:8080/jobs/{id}
```

### Get Queue Stats

```bash
curl http://localhost:8080/stats
```

### List Dead Jobs

```bash
curl http://localhost:8080/dlq
```

### Retry Dead Job

```bash
curl -X POST http://localhost:8080/dlq/{id}/retry
```

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://jobqueue:jobqueue@localhost:5432/jobqueue` | PostgreSQL connection string |
| `PORT` | `:8080` | HTTP server port |

Worker pool config (in code):

```go
pool := worker.New(db, worker.Config{
    Concurrency:  5,           // Number of concurrent workers
    PollInterval: time.Second, // How often to poll for jobs
})
```

## Registering Job Handlers

```go
pool.Register("email", func(ctx context.Context, j *job.Job) error {
    var payload EmailPayload
    json.Unmarshal(j.Payload, &payload)

    // Process the job...
    return sendEmail(payload)
})

pool.Register("webhook", func(ctx context.Context, j *job.Job) error {
    // ...
})
```

## Job Lifecycle

```
PENDING ──► PROCESSING ──► COMPLETED
    ▲            │
    │            ▼ (on failure)
    └─── PENDING (retry with backoff)
              │
              ▼ (max retries exceeded)
            DEAD
```


## Key Implementation Details

### Safe Concurrent Dequeuing

The `Dequeue` method uses PostgreSQL advisory locks combined with `FOR UPDATE SKIP LOCKED` to ensure no two workers process the same job:

```sql
SELECT * FROM jobs
WHERE status = 'PENDING'
  AND type = $1
  AND scheduled_at <= $2
  AND pg_try_advisory_xact_lock(hashtext(id))
ORDER BY priority DESC, scheduled_at ASC
LIMIT 1
FOR UPDATE SKIP LOCKED
```

### Exponential Backoff

Failed jobs are rescheduled with exponential backoff:

```go
func (j *Job) NextRetryDelay() time.Duration {
    base := 5 * time.Second
    return base * (1 << j.Attempts)  // 5s, 10s, 20s, 40s...
}
```

### Graceful Shutdown

Workers finish processing current jobs before shutting down on `SIGTERM`:

```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
<-sigCh

pool.Stop()  // Waits for workers to finish
```

## License

MIT
