# 0001. A Go API and a Java worker communicate through SQS

Status: accepted

## Context

The platform needs workloads that exercise HTTP traffic, a database, a cache,
and asynchronous work, in the two languages most common on Kubernetes.

## Decision

The API is a small Go service with no framework. The worker is a Spring Boot
service in Java. The API queues notifications in SQS and the worker
long-polls them, deleting a message only after processing it; failures return
to the queue and reach the dead-letter queue after repeated attempts.

## Consequences

The services deploy and scale independently, and a worker outage only delays
notifications. The message format becomes a contract between two codebases,
which [0005](0005-contracts-tested-on-both-sides.md) protects.
