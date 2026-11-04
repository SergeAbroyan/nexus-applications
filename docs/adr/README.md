# Architecture decision records

Each record explains one decision: the situation, what was chosen, and what it
costs. Records are never rewritten; a changed decision gets a new record that
supersedes the old one.

| Record | Decision |
| --- | --- |
| [0001](0001-api-and-worker-over-a-queue.md) | A Go API and a Java worker communicate through SQS |
| [0002](0002-one-trace-across-the-queue.md) | One trace follows a request through the queue |
| [0003](0003-migrations-applied-by-the-api.md) | The API applies versioned migrations at startup |
| [0004](0004-signed-images-sboms-and-provenance.md) | Every image is signed and carries an SBOM |
| [0005](0005-contracts-tested-on-both-sides.md) | HTTP and queue contracts are documents that tests enforce |
| [0006](0006-no-feature-flags.md) | No feature flag service |
| [0007](0007-release-please-and-sha-tags.md) | Releases are versioned by Release Please, images by commit SHA |
