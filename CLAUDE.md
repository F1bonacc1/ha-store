# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Quick Reference

See `AGENTS.md` for detailed coding guidelines, style conventions, and project structure.

## Build & Test Commands

```bash
make build                  # Build ha-store server → bin/ha-store
make build-stctl            # Build CLI client → bin/stctl
make test                   # go test -v ./...
make test-integration       # Integration tests (build tag: integration)
make coverage               # Coverage report
make docker-up / docker-down  # Start/stop NATS via Docker

# Single test
go test -v -run TestFileLifecycle ./...
go test -v ./handlers -run TestHandlePutFile
```

## Architecture

ha-store is a REST API that wraps NATS JetStream ObjectStore for file storage with optional mTLS.

**Request flow**: `stctl (CLI)` → HTTP/HTTPS → `main.go (Gin router)` → `handlers/` → `store/` → NATS JetStream ObjectStore

- **main.go** — Server setup, Gin routing, optional mTLS configuration, graceful shutdown
- **config/** — Loads configuration from CLI flags with env var fallbacks
- **handlers/** — Gin HTTP handlers for file and directory CRUD. Includes upload throttling (`throttle.go`) and archive extraction (zip, tgz, zst, 7z)
- **store/** — Thin wrapper around NATS JetStream ObjectStore. Single shared bucket `ha-store` with configurable replicas
- **stctl/** — Cobra-based CLI client. `stctl/client/` is the HTTP client library (with optional mTLS); `stctl/cmd/` has the CLI commands
- **testhelpers/** — `StartEmbeddedNATS(t)` spins up an in-process NATS server for tests

**Key patterns**:
- File deletion is async: handler returns 202 Accepted, deletes in a goroutine
- Uploads are throttled (default 150 MB/s) to avoid overwhelming JetStream replication
- Tests use embedded NATS — no external dependencies needed for `make test`

## Local Development

`process-compose.yaml` defines a 3-node NATS cluster for local HA testing (ports 4220-4222).
