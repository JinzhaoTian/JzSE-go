# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

JzSE (Jinzhao's Storage Engine) is a distributed, multi-region file service system written in Go. It uses a two-layer architecture: **Region Layer** (per-geography, serves local reads/writes) and **Global Coordination Layer** (centralized, manages cross-region sync and conflict resolution).

Go module path: `asisaid.cn/JzSE`

## Build & Development Commands

```bash
make build                # Build both binaries → bin/region, bin/coordinator
make build-region         # Build region service only
make build-coordinator    # Build coordinator service only
make test                 # Run all tests with -race and -cover
make test-coverage        # Generate HTML coverage report (coverage.html)
make lint                 # Run golangci-lint
make fmt                  # Format all code
make vet                  # Run go vet
make proto                # Generate protobuf code
make deps                 # go mod download && go mod tidy
```

Run a single test:
```bash
go test -v -run TestFunctionName ./internal/region/service/...
```

Run services locally:
```bash
make run-region           # Uses configs/region.yaml
make run-coordinator      # Uses configs/coordinator.yaml
make dev-region           # Development mode (console logging)
```

## Architecture

### Two-Layer Design

**Region Layer** (`internal/region/`) — deployed per geographic region:
- `service/` — FileService: core business logic (upload, download, delete, metadata, list)
- `storage/` — Pluggable storage backends via `Backend` interface; current impl: `LocalFSBackend` (hash-based 2-level directory structure, atomic writes via temp files)
- `metadata/` — Local metadata store using BadgerDB; `FileMetadata` model includes vector clocks, sync state, local state
- `sync/` — SyncAgent pushes local changes to coordinator; supports push/batch/pull modes with retry logic

**Coordination Layer** (`internal/coordinator/`) — centralized:
- `metadata/` — Global metadata manager (designed for etcd, currently in-memory fallback); extends file metadata with region locations and replica info
- `conflict/` — Conflict detection via vector clock comparison; resolution strategies: LWW (default), Fork, Manual
- `sync/` — SyncEngine processes change events (CREATE/UPDATE/DELETE), updates global metadata, broadcasts to other regions
- `registry/` — Region registry with heartbeat-based health checking (healthy → degraded → offline)

**Shared** (`internal/common/`):
- `errors/` — Sentinel errors (ErrNotFound, ErrConflict, etc.) and `JzSEError` type with Op/Kind/Err/Details
- `logger/` — Zap-based structured logging with component context
- `config/` — Viper-based YAML config with env var overrides (prefix: `JZSE_`)

### HTTP API Entry Points

- `pkg/api/http/` — Gin-based HTTP handlers and routers
- Region API on `:8080` — file CRUD, directory listing, health, region status
- Coordinator API on `:8081` — metadata queries, region management, heartbeats, sync

### Data Flow

Write path: Client → RegionService → LocalStorage + LocalMetadata → async SyncAgent → Coordinator → broadcast to other regions

Read path: Client → RegionService → check local metadata → serve from LocalStorage (or fetch from origin region via Coordinator if not local)

### Consistency Model

Local strong consistency within a region. Global eventual consistency across regions. Conflicts detected via vector clocks and resolved by configurable strategy.

## Code Conventions

- Follow [Effective Go](https://golang.org/doc/effective_go); lint with `golangci-lint`
- Interfaces in dedicated files, implementations in same package
- Error construction: `errors.E(op, kind, underlying_err, details)`
- Logging: `logger.WithComponent("Name").Info("msg", zap.String("key", val))`
- Table-driven tests; unit tests in `*_test.go` alongside source; integration tests in `test/integration/`

## Configuration

Config files in `configs/` (region.yaml, coordinator.yaml). Environment variable overrides use `JZSE_` prefix (e.g., `JZSE_LOGGER_DEVELOPMENT=true`).

## Current Status

Core region service (storage, metadata, HTTP API) is functional. Coordinator components exist as frameworks with in-memory fallbacks — etcd client, NATS integration, gRPC services, and actual cross-region communication are not yet wired up.
