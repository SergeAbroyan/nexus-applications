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
