# 0002. One trace follows a request through the queue

Status: accepted

## Context

Tracing usually stops at a queue, so the work a request causes appears as an
unrelated trace or not at all.

## Decision

Both services use OpenTelemetry with the standard `OTEL_*` settings. The API
injects the W3C trace context into SQS message attributes, and the worker
extracts it and processes each message in a consumer span whose parent is that
request. Logs on both sides carry the same `trace_id`.

## Consequences

Tempo shows a request and the worker's processing as one trace, and logs link
to it. With tracing off, the worker still logs the `trace_id` it receives.
