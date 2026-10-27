# Nexus Applications

This repository contains two intentionally small deployable applications:

- `api` is a Go web application that serves the browser page and REST API,
  stores items in PostgreSQL, caches reads in Valkey and publishes SQS jobs.
- `worker` is a Java 21 Spring Boot application that long-polls SQS and processes
  notification jobs.

Two services are enough to demonstrate HTTP traffic, persistence, caching,
asynchronous work, separate container builds and a polyglot CI/CD pipeline.

## Structure

```text
.
├── .github/workflows/ci.yaml       Tests, scans, ECR publishing and DEV promotion
├── local/elasticmq.conf            Local SQS queues and dead-letter queue
├── services/
│   ├── api/                        Go web page and API
│   ├── observability/              Go metrics, tracing and structured logging
│   └── worker/
│       ├── src/main/java/          Java SQS worker
│       ├── src/main/resources/     Spring Boot configuration
│       ├── src/test/java/          Java unit tests
│       ├── pom.xml                 Java dependencies and build
│       └── Dockerfile              Maven build and distroless runtime
├── compose.yaml                    Complete local environment
├── go.mod / go.sum                 Go dependencies
├── Makefile                        Local commands
└── sonar-project.properties        Optional Go and Java analysis
```

## Go API

The API exposes:

- `GET /` for the browser page;
- `GET /healthz` and `GET /readyz` for Kubernetes probes;
- `GET /metrics` for Prometheus;
- `GET /api/items` and `POST /api/items` for PostgreSQL-backed items;
- `POST /api/notify` for SQS notifications.

The API is compiled as a static binary and runs as UID `65532` in a scratch
container. The HTML page is embedded into that binary, so it does not require a
separate frontend deployment.

At startup the API creates its table only when PostgreSQL accepts writes. In
the disaster-recovery standby region it connects to a read replica, skips the
schema step, and serves reads until the replica is promoted.

## Java Worker

The Java worker uses Spring Boot, AWS SDK for Java v2 and Java 21. It long-polls
SQS, validates the JSON notification and deletes a message only after successful
processing. Invalid or failed messages remain available for retry and eventually
move to the dead-letter queue.

Spring Boot Actuator exposes:

- `GET /livez` for the liveness probe;
- `GET /readyz` for the readiness probe;
- `GET /metrics` in Prometheus format.

The worker is built with Maven and runs as a non-root user in a Java 21
distroless container.

## Local Development

Run both test suites in pinned containers:

```bash
make test
```

Build both application images:

```bash
make build
```

Start PostgreSQL, Valkey, ElasticMQ, the Go API and the Java worker:

```bash
make local-up
```

Open <http://localhost:8081>. Port `8081` avoids the Jenkins service already
using port `8080` on the host.

Inspect or stop the environment:

```bash
make local-status
make local-logs
make local-down
```

`local-down` keeps the PostgreSQL development volume. Use Docker Compose's
explicit volume-removal option only when you intentionally want to delete local
data.

## Configuration

The Go API uses the following groups of variables:

- `DB_*` for PostgreSQL;
- `VALKEY_*` for cache access;
- `AWS_REGION`, `SQS_QUEUE_URL` and optional `SQS_ENDPOINT` for SQS;
- `OTEL_*` for tracing.

The Java worker uses:

- `AWS_REGION`;
- `SQS_QUEUE_URL`;
- optional `SQS_ENDPOINT` for ElasticMQ;
- `SQS_WAIT_TIME_SECONDS` and `SQS_VISIBILITY_TIMEOUT_SECONDS`.

AWS credentials are not stored in the repository. Local Compose supplies test
credentials for ElasticMQ. EKS uses Pod Identity.

## CI and Image Deployment

`.github/workflows/ci.yaml` performs these steps:

1. select each service whose files differ from the commit that DEV runs, read
   from `nexus-gitops` when `GITOPS_TOKEN` is set, or from the previous commit
   otherwise;
2. test the affected service, while a configured SonarQube scan analyzes both languages;
3. build and scan only the affected `api` or `worker` image;
4. authenticate to AWS through GitHub OIDC;
5. push an immutable commit-SHA tag to ECR in the primary region, which
   replicates the image to the standby region;
6. update only the affected DEV image tag in `nexus-gitops`, which both
   regions deploy.

Manual workflow runs intentionally select both services. Changes to shared
container, Compose, analysis or workflow files also validate both services.

Runs on `main` queue instead of cancelling each other. Because every run
compares with what DEV runs, a superseded or failed run never drops a change,
and while DEV still uses the initial `0.1.0` tags the first run builds both
services. Actions are pinned to commit SHAs, and Dependabot opens grouped
weekly updates for them.

After the infrastructure exists, configure these repository settings:

- variable `AWS_ROLE_ARN` from the infrastructure `ci-identity` output;
- variable `AWS_REGION`, currently `us-east-1`;
- secret `GITOPS_TOKEN`, restricted to write access on `nexus-gitops`;
- optional variable `SONAR_HOST_URL` and secret `SONAR_TOKEN`.

If AWS settings are absent, CI still tests, builds and scans the affected images
but does not publish them. Staging and production promotion remains a deliberate
GitOps change of an already-tested image SHA.
