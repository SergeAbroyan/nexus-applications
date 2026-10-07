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
