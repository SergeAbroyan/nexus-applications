# 0006. No feature flag service

Status: accepted

## Context

OpenFeature with flagd would let features be turned on without a deploy.

## Decision

Do not add a flag service. The canary release already limits the impact of new
code, and the two services have no features that need to be enabled per user
or per environment.

## Consequences

Turning a feature off means a revert and a release, which takes minutes here.
Revisit when a feature must ship dark or roll out to a subset of users.
