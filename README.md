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
frontend, API, and worker containers. A successful `main` build publishes all
three images to ECR with the full Git commit SHA as their immutable tag.

Publishing uses GitHub OIDC, so the repository does not store AWS access keys.
After applying the infrastructure, add these GitHub repository variables:

- `AWS_ROLE_ARN`: the `application_ci_iam_role_arn` output from the
  `ci-identity` Terragrunt unit;
- `AWS_REGION`: the deployed AWS region, currently `us-east-1`.

Until both variables exist, GitHub still runs all tests and container builds but
skips the ECR publishing job. This lets the workflow operate before the AWS
infrastructure is deployed.

Publishing does not modify the GitOps repository. To promote a tested build,
set `image.tag` in the target `nexus-gitops/environments/<environment>/values.yaml`
file to the published commit SHA and merge that GitOps change.
