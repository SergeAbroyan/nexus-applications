# 0007. Releases are versioned by Release Please, images by commit SHA

Status: accepted

## Context

Deployments need immutable image references, while people need readable
versions and changelogs.

## Decision

Images are tagged with the commit SHA that built them, and environments point
at those tags. Release Please turns conventional commit messages into a
release pull request per service; merging it creates a version tag, a
changelog, and a GitHub release for that commit.

## Consequences

A version always maps to exactly one tested image. Commit messages must follow
the conventional format, which the existing history already does.
