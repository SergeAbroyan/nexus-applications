# 0003. The API applies versioned migrations at startup

Status: accepted

## Context

The schema was a `CREATE TABLE IF NOT EXISTS` statement in code, which cannot
change an existing table or record what has been applied. Atlas and a separate
Kubernetes Job are alternatives, but each adds a tool or a deployment step.

## Decision

Migrations are numbered SQL files embedded in the API binary and applied with
golang-migrate at startup, under a PostgreSQL advisory lock. The first
migration matches the old statement, so existing databases adopt it without
changes. Standby replicas skip migrations.

## Consequences

The schema version lives in the database and every pod agrees on it. The API's
database user needs DDL rights, and every migration must stay compatible with
the previous release, because a canary runs both versions at once.
