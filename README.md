# Nexus Applications

This repository contains the three intentionally small Nexus services:

- `frontend`: serves the web page and calls the API.
- `api`: will use PostgreSQL, Valkey, and SQS.
- `worker`: will consume jobs from SQS.

The application stays small because its purpose is to exercise the surrounding
AWS, Kubernetes, GitOps, security, scaling, and observability platform.

## Frontend

Run the tests in the pinned Go container:

```bash
make test
```

Build and run the minimal frontend container:

```bash
make run-frontend
```

Open <http://localhost:8081>. Port 8081 is used on the host because Jenkins is
already using port 8080. Inside Kubernetes, the service continues to listen on
port 8080.

## API

The API provides:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`
- `GET /api/items`
- `POST /api/items`
- `POST /api/notify`

Items are stored in PostgreSQL and cached briefly in Valkey. Notifications are
published to SQS for the worker. Runtime configuration is supplied through
environment variables; credentials are never stored in this repository.

Build the API container:

```bash
make build-api
```

## Worker

The worker long-polls SQS, validates each notification, logs the processed
message, and deletes it only after successful processing. Failed messages stay
in SQS for retry and eventually move to the dead-letter queue.

Build the worker container:

```bash
make build-worker
```

## Metrics

Every service exposes `GET /metrics` in Prometheus text format. The shared
`services/observability` package records service identity, process uptime,
total HTTP requests, and in-flight requests using only the Go standard library.
Metrics scrapes are excluded from the request count.

## Tracing

Each service initializes OpenTelemetry when `OTEL_EXPORTER_OTLP_ENDPOINT` is
set. Incoming HTTP requests create server spans, the frontend propagates trace
context to the API, and the worker traces each SQS polling cycle. JSON log
records written with a request or worker context include `trace_id` and
`span_id`, allowing Grafana to correlate Tempo traces with Loki logs. A
parent-based ratio sampler honors upstream Istio decisions and applies the
configured ratio to root spans such as worker polling cycles.

The API also creates child spans for PostgreSQL, Valkey, and SQS operations.
SQS messages carry standard W3C trace context in message attributes, allowing
the worker's consumer span and contextual logs to continue the originating API
trace. Trace attributes describe operations without recording SQL statements,
cache values, notification bodies, credentials, or SQS receipt handles.

Tracing stays disabled when the endpoint is unset, so the local Compose stack
does not require an observability backend. Kubernetes supplies the internal
collector endpoint through the Nexus Helm chart.

## Complete local environment

Start PostgreSQL, Valkey, the local SQS-compatible queue, and all three Nexus
services:

```bash
make local-up
```

Open <http://localhost:8081>. The frontend forwards `/api` requests to the API,
so the browser uses one origin just as it will behind the Kubernetes Gateway.

Inspect container status or follow logs:

```bash
make local-status
make local-logs
```

Stop the environment without deleting the PostgreSQL development volume:

```bash
make local-down
```

## Continuous integration

`.github/workflows/ci.yaml` runs for pull requests and pushes to `main`. It
checks Go formatting, runs `go vet` and race-enabled tests, and builds the
frontend, API, and worker containers. Trivy scans each built image and blocks
the pipeline when it finds a HIGH or CRITICAL vulnerability that has an
available fix. A successful `main` build publishes all three images to ECR with
the full Git commit SHA as their immutable tag.

Publishing uses GitHub OIDC, so the repository does not store AWS access keys.
After applying the infrastructure, add these GitHub repository variables:

- `AWS_ROLE_ARN`: the `application_ci_iam_role_arn` output from the
  `ci-identity` Terragrunt unit;
- `AWS_REGION`: the deployed AWS region, currently `us-east-1`.

Add the `GITOPS_TOKEN` repository secret to enable automatic DEV deployment.
Use a fine-grained GitHub personal access token restricted to the
`SergeAbroyan/nexus-gitops` repository with `Contents: Read and write`
permission. It does not need access to AWS or any other repository.

SonarQube analysis is enabled when both of these repository settings exist:

- variable `SONAR_HOST_URL`: the HTTPS URL of the SonarQube server;
- secret `SONAR_TOKEN`: a project analysis token created in SonarQube.

The scanner uploads Go coverage and waits up to five minutes for the Quality
Gate. A failed gate fails the quality job and prevents ECR publishing. Until
both settings exist, only the external SonarQube steps are skipped.

Until both AWS variables exist, GitHub still runs all tests and container
builds but skips the ECR publishing and DEV promotion jobs. After all three
images are published successfully, one promotion job changes
`nexus-gitops/environments/dev/values.yaml` to the same commit SHA. Argo CD then
reconciles that immutable release. If `GITOPS_TOKEN` is absent, publishing still
succeeds and the workflow records that promotion was skipped.

Staging and production promotions remain deliberate GitOps changes. Promote
the already-tested SHA by changing `image.tag` in the target environment values
file; never rebuild an image for promotion.
