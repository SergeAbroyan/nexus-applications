# Configuration

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
| `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_TRACES_SAMPLER`, `OTEL_TRACES_SAMPLER_ARG`, `OTEL_RESOURCE_ATTRIBUTES` | both | tracing off | Standard OpenTelemetry settings |
| `PYROSCOPE_SERVER_ADDRESS` | both | profiling off | Pyroscope server that receives continuous profiles |
| `SQS_WAIT_TIME_SECONDS` | worker | `20` | Long-poll duration |
| `SQS_VISIBILITY_TIMEOUT_SECONDS` | worker | `60` | Time to process a message before redelivery |
| `SQS_POLL_DELAY_MS` | worker | `1000` | Pause between polls |

In Kubernetes these come from `nexus-gitops`. AWS credentials come from EKS Pod
Identity and are never stored. Locally, Compose points both services at
ElasticMQ with the standard `AWS_ENDPOINT_URL_SQS` variable.
