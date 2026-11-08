# Architecture

## Structure

```text
services/
├── api/
│   ├── main.go                 Wiring: configuration, telemetry, dependencies, HTTP server lifecycle
│   ├── internal/
│   │   ├── config/             Environment variables
│   │   ├── items/              Item rules and the read-through cache
│   │   ├── notifications/      The notification message sent to the worker
│   │   ├── postgres/           Item storage and versioned schema migrations
│   │   ├── valkey/             Cache client and ElastiCache IAM authentication
│   │   ├── sqs/                Queue client that forwards the trace context
│   │   ├── server/             Routes, handlers, the OpenAPI document, and the embedded web page
│   │   └── telemetry/          Tracing, profiling, JSON logs, and Prometheus metrics
│   ├── go.mod
│   └── Dockerfile              Static binary in a scratch image
└── worker/
    ├── src/main/java/          SQS polling, message model, tracing, profiling, configuration
    ├── src/test/java/          Unit tests and the ElasticMQ integration test
    ├── pom.xml
    └── Dockerfile              Fat jar in a distroless Java 25 image
contracts/                      JSON Schema and example of the queue message, shared by both services
load/api.js                     k6 load test of the API
.zap/rules.tsv                  Alerts the ZAP scan may ignore, with the reason
release-please-config.json      How Release Please versions each service
docs/                           Architecture, configuration, local development, CI and delivery
└── adr/                        Architecture decision records
local/elasticmq.conf            Local SQS queue and dead-letter queue
compose.yaml                    Complete local environment
compose.observability.yaml      Optional Grafana, Tempo, Loki, Prometheus, and Pyroscope
sonar-project.properties        Optional SonarQube analysis
```

Packages under `services/api/internal` depend inward: `server` calls the
`items` and `notifications` services, which define the interfaces that
`postgres`, `valkey`, and `sqs` implement. Business rules can therefore be
tested without any infrastructure.

## API

| Endpoint | Purpose |
| --- | --- |
| `GET /` | Web page |
| `GET /api/items`, `POST /api/items` | List and create items |
| `POST /api/notify` | Queue a notification for the worker |
| `GET /healthz` | Liveness: the process is running |
| `GET /readyz` | Readiness: PostgreSQL answers |
| `GET /metrics` | Prometheus metrics |
| `GET /api/openapi.yaml` | The OpenAPI 3 document of the endpoints above |

Item lists are read through Valkey: a miss reads PostgreSQL and fills the
cache, and creating an item clears it. Valkey is an optimization, so when it
fails the API logs a warning and serves from PostgreSQL, and readiness ignores
it.

At startup the API applies the versioned migrations in
`internal/postgres/migrations` with golang-migrate, which records the version
in `schema_migrations` and holds a PostgreSQL advisory lock, so pods that start
together apply each migration once. Because a canary runs the old and new
versions side by side, every migration must keep the previous version working:
add columns and tables first, remove them in a later release. In the
disaster-recovery standby region the database is a read replica, so the API
skips migrations, serves reads, and answers item creation with `503` until the
replica is promoted.

`internal/server/openapi.yaml` documents every application endpoint, and the
API serves it at `/api/openapi.yaml`. A contract test sends a request for each
documented operation and response code, validates both against the document,
and fails when an operation is documented but not tested. Every response
carries a strict Content Security Policy and cross-origin isolation headers,
and API responses are never cached.

Requests are traced with OpenTelemetry, and the trace context continues through
PostgreSQL, Valkey, and SQS. Logs are JSON and carry the `trace_id`. The
metrics cover the application routes (`nexus_http_requests_total`,
`nexus_http_requests_in_flight`) plus the Go runtime and process; probes and
`/metrics` are not counted or traced. When `PYROSCOPE_SERVER_ADDRESS` is set,
the API also sends CPU, allocation, heap, and goroutine profiles to Pyroscope.

## Worker

The worker long-polls SQS, validates each notification, and deletes a message
only after it has been processed. A message that fails stays in the queue, is
delivered again after the visibility timeout, and moves to the dead-letter
queue after repeated failures. On shutdown it finishes the batch in progress.

Each message is processed in an OpenTelemetry consumer span whose parent is
the API request that queued it, read from the `traceparent`, `tracestate`, and
`baggage` message attributes, so one trace in Tempo shows the request and the
work it caused. Logs are JSON in the Elastic Common Schema format and carry the
same `trace_id`. When `PYROSCOPE_SERVER_ADDRESS` is set, the worker sends CPU
and allocation profiles to Pyroscope. Spring Boot Actuator
serves `GET /livez`, `GET /readyz`, and `GET /metrics`, including
`nexus_worker_messages_total{outcome}` and
`nexus_worker_receive_failures_total`.

## Message contract

The API sends, and the worker accepts, this JSON body with a `traceparent`
message attribute:

```json
{"type": "notification.requested", "message": "hello", "createdAt": "2026-10-27T09:30:00Z"}
```

`contracts/notification.schema.json` is the single definition. The API's
tests validate every message it produces, and the shared example, against the
schema; the worker's tests read the same example. A change that breaks either
side fails CI for both services. The worker ignores fields it does not know, so
the API can add optional fields before the worker uses them.
