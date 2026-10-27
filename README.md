# Nexus Applications

Two small services that together exercise everything the Nexus platform
provides: HTTP traffic, PostgreSQL, a Valkey cache, an SQS queue, tracing,
metrics, and structured logs.

- `api` is a Go service that serves the web page and REST API, stores items in
  PostgreSQL, caches reads in Valkey, and queues notifications in SQS.
- `worker` is a Java 21 Spring Boot service that long-polls SQS and processes
  the notifications.

Each service folder is self-contained: its own dependencies, tests, and
Dockerfile, built with the folder as the Docker context.

## Structure

```text
services/
├── api/
│   ├── main.go                 Wiring: configuration, telemetry, dependencies, HTTP server lifecycle
│   ├── internal/
│   │   ├── config/             Environment variables
│   │   ├── items/              Item rules and the read-through cache
│   │   ├── notifications/      The notification message sent to the worker
│   │   ├── postgres/           Item storage and schema
│   │   ├── valkey/             Cache client and ElastiCache IAM authentication
│   │   ├── sqs/                Queue client that forwards the trace context
│   │   ├── server/             Routes, handlers, and the embedded web page
│   │   └── telemetry/          Tracing, JSON logs, and Prometheus metrics
│   ├── go.mod
│   └── Dockerfile              Static binary in a scratch image
└── worker/
    ├── src/main/java/          SQS polling, message model, configuration
    ├── src/test/java/          Unit tests
    ├── pom.xml
    └── Dockerfile              Fat jar in a distroless Java 21 image
local/elasticmq.conf            Local SQS queue and dead-letter queue
compose.yaml                    Complete local environment
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

Item lists are read through Valkey: a miss reads PostgreSQL and fills the
cache, and creating an item clears it. Valkey is an optimization, so when it
fails the API logs a warning and serves from PostgreSQL, and readiness ignores
it.

At startup the API creates its table only when PostgreSQL accepts writes. In
the disaster-recovery standby region the database is a read replica, so the API
skips the schema step, serves reads, and answers item creation with `503` until
the replica is promoted.

Requests are traced with OpenTelemetry, and the trace context continues through
PostgreSQL, Valkey, and SQS. Logs are JSON and carry the `trace_id`. The
metrics cover the application routes (`nexus_http_requests_total`,
`nexus_http_requests_in_flight`) plus the Go runtime and process; probes and
`/metrics` are not counted or traced.

## Worker

The worker long-polls SQS, validates each notification, and deletes a message
only after it has been processed. A message that fails stays in the queue, is
delivered again after the visibility timeout, and moves to the dead-letter
queue after repeated failures. On shutdown it finishes the batch in progress.

Logs are JSON in the Elastic Common Schema format, and each processed message
logs the `trace_id` of the API request that queued it. Spring Boot Actuator
serves `GET /livez`, `GET /readyz`, and `GET /metrics`, including
`nexus_worker_messages_total{outcome}` and
`nexus_worker_receive_failures_total`.

## Message contract

The API sends, and the worker accepts, this JSON body with a `traceparent`
message attribute:

```json
{"type": "notification.requested", "message": "hello", "createdAt": "2026-10-27T09:30:00Z"}
```

The contract is defined in `internal/notifications` for the API and in
`Notification.java` for the worker. The worker ignores fields it does not know,
so the API can add fields before the worker uses them.

## Configuration

| Variable | Service | Default | Purpose |
| --- | --- | --- | --- |
| `PORT` | both | `8080` | HTTP port |
| `AWS_REGION` | both | required | Region of SQS and ElastiCache |
| `SQS_QUEUE_URL` | both | required | Notification queue |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER` | api | `localhost`, `5432`, `nexus`, `nexus` | PostgreSQL |
| `DB_PASSWORD` | api | required | PostgreSQL password |
| `DB_SSLMODE` | api | `prefer` | PostgreSQL TLS mode |
| `VALKEY_HOST`, `VALKEY_PORT` | api | `localhost`, `6379` | Valkey |
| `VALKEY_TLS` | api | `false` | Connect to Valkey over TLS |
| `VALKEY_IAM_AUTH`, `VALKEY_USERNAME`, `VALKEY_CACHE_NAME` | api | `false` | ElastiCache IAM authentication |
| `VALKEY_CACHE_TTL` | api | `30s` | Lifetime of the cached item list |
| `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_TRACES_SAMPLER`, `OTEL_TRACES_SAMPLER_ARG` | api | tracing off | Standard OpenTelemetry settings |
| `SQS_WAIT_TIME_SECONDS` | worker | `20` | Long-poll duration |
| `SQS_VISIBILITY_TIMEOUT_SECONDS` | worker | `60` | Time to process a message before redelivery |
| `SQS_POLL_DELAY_MS` | worker | `1000` | Pause between polls |

In Kubernetes these come from `nexus-gitops`. AWS credentials come from EKS Pod
Identity and are never stored. Locally, Compose points both services at
ElasticMQ with the standard `AWS_ENDPOINT_URL_SQS` variable.

## Local development

Start PostgreSQL, Valkey, ElasticMQ, the API, and the worker:

```bash
docker compose up --build --wait
```

Open <http://localhost:8081>; port `8081` avoids the Jenkins service on `8080`.
Follow the worker with `docker compose logs --follow worker` and stop everything
with `docker compose down`, which keeps the PostgreSQL volume.

Run the tests:

```bash
cd services/api && go test -race ./...
cd services/worker && mvn verify
```

Without a local Go or Java toolchain, run them in the build images instead:

```bash
docker run --rm -v "$PWD/services/api:/src" -w /src golang:1.27.3-alpine go test ./...
docker run --rm -v "$PWD/services/worker:/src" -w /src maven:3.9.12-eclipse-temurin-21-alpine mvn -B verify
```

## CI and delivery

`.github/workflows/ci.yaml` runs on pull requests, on pushes to `main`, and on
demand:

1. **Detect changed services.** A pull request tests what it changes. A push
   compares each service with the commit that DEV runs, read from
   `nexus-gitops`, so a failed or superseded run never drops a change. A
   manual run selects both services, and a change to the workflow selects both.
2. **Test.** The API job checks formatting, module files, and `go vet`, and
   runs the tests with the race detector; the worker job runs `mvn verify`.
   When `SONAR_HOST_URL` is set, SonarQube analyzes both services and enforces
   its quality gate.
3. **Build and scan** each changed image and fail on fixable high or critical
   vulnerabilities.
4. **Publish** on `main`: sign in to AWS with GitHub OIDC and push an immutable
   commit-SHA tag to ECR in the primary region, which replicates it to the
   standby region. An image that already exists is not rebuilt.
5. **Promote to DEV** by updating the image tag of each published service in
   `nexus-gitops`; Argo CD rolls it out in both regions.

Runs on `main` queue instead of cancelling each other. Staging and production
change only through a deliberate GitOps commit that points them at an image SHA
already tested in DEV. Dependabot keeps GitHub Actions, Go modules, Maven
dependencies, and base images current.

Repository settings, once the infrastructure exists:

- variable `AWS_ROLE_ARN`, the `application_ci_role_arn` output of the `global`
  unit in `nexus-platform`;
- variable `AWS_REGION`, currently `us-east-1`;
- secret `GITOPS_TOKEN` with write access to `nexus-gitops` only;
- optionally variable `SONAR_HOST_URL` and secret `SONAR_TOKEN`.

Without the AWS settings, CI still tests, builds, and scans, but does not
publish.
