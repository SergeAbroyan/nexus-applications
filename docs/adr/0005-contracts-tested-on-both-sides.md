# 0005. HTTP and queue contracts are documents that tests enforce

Status: accepted

## Context

Documentation that tests do not check drifts from the code, and two services
in different languages can disagree on a message format without noticing
until production.

## Decision

The API serves an OpenAPI 3 document, and a contract test validates a request
and response for every documented operation and status code, failing when an
operation is documented but untested. The queue message has a JSON Schema in
`contracts/`; the API's tests validate what it sends against it, and both
services' tests read the same example.

## Consequences

The documentation is always accurate, and a breaking message change fails CI
for both services. Changing an endpoint means changing the document in the
same commit.
