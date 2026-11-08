# 0008. Any version deploys to any environment from this repository

Status: accepted. Supersedes `nexus-gitops` ADR 0008.

## Context

CI deployed every build to DEV, and a `Promote release` workflow in
`nexus-gitops` copied image tags from DEV to staging and from staging to
production. It could not deploy to DEV, could not choose a version, so a
rollback meant a revert, and it lived in a different repository from the code.
A branch per environment was also considered; it lets the code in each
environment drift apart and needs a separate build for each branch, so the
image in production would not be the one that was tested.

## Decision

Development is trunk-based, and environments are folders in `nexus-gitops`.
A manual `Deploy` workflow here takes an environment, the services, and an
optional commit SHA. A first job resolves the tags, from the input, from the
previous environment, or from the newest build, and verifies in ECR that CI
signed each image. A second job runs in the GitHub environment of the same
name, where only production has required reviewers, and commits the tags to
`nexus-gitops`.

The jobs are split because a job bound to a GitHub environment presents a
different OIDC subject, which the AWS role does not trust. Verification
therefore needs AWS and no approval; the commit needs approval and no AWS.

## Consequences

One place deploys, rolls back, and promotes, and a broken or unsigned image is
rejected before anyone approves it. Every deployment remains a commit in
`nexus-gitops`. The workflow refuses production until that GitHub environment
has reviewers.
