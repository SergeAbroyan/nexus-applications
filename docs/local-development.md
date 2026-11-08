# Local development

Start PostgreSQL, Valkey, ElasticMQ, the API, and the worker:

```bash
docker compose up --build --wait
```

Open <http://localhost:8081>; port `8081` avoids the Jenkins service on `8080`.
Follow the worker with `docker compose logs --follow worker` and stop everything
with `docker compose down`, which keeps the PostgreSQL volume. A volume created
by an earlier PostgreSQL major version cannot be opened by PostgreSQL 18; remove
it with `docker compose down --volumes` and start again.

To see traces, logs, metrics, and profiles locally, add the observability
overlay. It runs Grafana's all-in-one OpenTelemetry image and points both
services at it, so Grafana at <http://localhost:3000> shows each API request
and the worker span it caused in Tempo, and both services in Pyroscope:

```bash
docker compose -f compose.yaml -f compose.observability.yaml up --build --wait
```

Run the tests. The API's integration tests, tagged `integration`, start
PostgreSQL 18, Valkey 9.1, and ElasticMQ with Testcontainers and need Docker;
the worker's `mvn verify` runs its unit tests and then `NotificationWorkerIT`
against ElasticMQ:

```bash
cd services/api && go test -race ./... && go test -race -tags integration ./...
cd services/worker && mvn verify
```

Without a local Go or Java toolchain, run them in the build images instead.
The Docker socket lets Testcontainers start its containers from inside:

```bash
docker run --rm -v "$PWD/services/api:/src" -w /src \
  -v /var/run/docker.sock:/var/run/docker.sock -e TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal \
  golang:1.27.3-alpine go test -tags integration ./...
docker run --rm -v "$PWD/services/worker:/src" -w /src \
  -v /var/run/docker.sock:/var/run/docker.sock -e TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal \
  maven:3.9.16-eclipse-temurin-25-alpine mvn -B verify
```

Load the running stack with k6. `VUS`, `RAMP_UP`, and `DURATION` size the run,
and the thresholds fail it when more than 0.5% of requests fail or the 99th
percentile exceeds 500 ms, the API's latency objective:

```bash
docker run --rm -i --network nexus_default -e BASE_URL=http://api:8080 -e VUS=10 grafana/k6:2.3.0 run - < load/api.js
```
