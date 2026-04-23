# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Kuberpult

Kuberpult is a CD tool triggered by REST API (not git push). When `/release` or `/api/release` is called with manifest files, it saves the manifest to a database. The `manifest-repo-export-service` then commits and pushes those manifests to a git repository watched by ArgoCD.

## Build & Run

```bash
# Build the builder image first (required before building services)
make builder

# Build all services
make all

# Build a single service's Docker image
IMAGE_TAG=local make -C services/cd-service docker

# Run all services locally via docker-compose
make kuberpult

# Reset the database volume
make reset-db
```

## Testing

```bash
# Run all tests (starts a Postgres container automatically)
make test

# Run tests for a single service
make -C services/cd-service test
make -C services/frontend-service test

# Verbose Go tests (from within a service directory)
cd services/cd-service && go test ./... -v

# For local IDE test runs (requires /etc/hosts entry and running DB):
# 1. Add to /etc/hosts:  127.0.0.1 kuberpult-test-postgres
# 2. Start test DB:
make unit-test-db
# 3. Run tests from IDE or terminal

# Run integration tests (requires k3s cluster setup)
make integration-test
```

## Linting

```bash
make lint        # lint all services
make lint-fix    # auto-fix linting issues
```

## Code Generation

```bash
# Generate protobuf / gRPC code (from pkg/)
# On Apple Silicon (arm64) the builder Docker image won't run — use PKG_WITHOUT_DOCKER=1 instead:
PKG_WITHOUT_DOCKER=1 make -C pkg gen

# Generate frontend API client
make -C services/frontend-service gen-api
```

The generated files in `pkg/api/v1/` and `pkg/publicapi/` are required to compile and test
services locally (e.g. `go test ./...` in `services/frontend-service`). Always run
`PKG_WITHOUT_DOCKER=1 make -C pkg gen` once after a fresh clone or after `pkg/api/v1/api.proto`
changes before running tests.

**Always run the relevant service tests locally before pushing or opening a PR:**

```bash
cd services/frontend-service && go test ./...
cd services/cd-service && go test ./...
```

## Architecture

Kuberpult is a Go microservices backend with a React frontend.

### Services

| Service | Role |
|---|---|
| `cd-service` | Core backend: release ingestion, deploy decisions, database writes, gRPC server |
| `frontend-service` | REST API gateway (`/api/*`) and React UI host; sole point that handles author headers |
| `manifest-repo-export-service` | Watches the DB, exports manifests to the git repository (ArgoCD source of truth) |
| `rollout-service` | Manages release train operations |
| `reposerver-service` | Git repository server |
| `cli/` | CLI client for the Kuberpult API |
| `pkg/` | Shared Go packages (protos, domain types, utilities) |
| `charts/kuberpult/` | Helm chart for Kubernetes deployment |

### Communication

- **gRPC** — service-to-service (e.g., `cd-service` ↔ `manifest-repo-export-service`); gRPC-Web for browser calls
- **REST** — public-facing endpoints served by `frontend-service`
- **PostgreSQL** — shared database; migrations live in `database/migrations/postgres/` and are applied by golang-migrate

### Key Patterns

- **Git as source of truth**: manifests are committed to a git repo by `manifest-repo-export-service`, which ArgoCD then syncs
- **Author headers required**: all gRPC and HTTP calls (except `/release` and `/health`) must supply `author-name` and `author-email` headers; `frontend-service` injects defaults from Helm config
- **Locks**: environment/app/team/group locks gate deployments
- **Release trains**: coordinated multi-service deployments handled by `rollout-service`
- **libgit2**: used for performance (go-git was too slow); requires libgit2 1.5.0 via Nix

### Development Environment

The repo uses **Nix Flakes** + **direnv** to provide a reproducible environment with the correct libgit2, Go, Node, pnpm, and protoc versions. Run `direnv allow` once in the project root.

Verify the environment is set up:
```bash
pkg-config --modversion libgit2  # should print 1.5.0
```

When running `manifest-repo-export-service` tests from an IDE, set `LD_LIBRARY_PATH="$NIX_LD_LIBRARY_PATH"` before starting the IDE.

## Test Writing Conventions

- Use **table-driven tests** for all new test cases.
- Always compare with `cmp.Diff` and print the diff on failure:
  ```go
  if diff := cmp.Diff(tc.want, got, protocmp.Transform()); diff != "" {
      t.Errorf("mismatch (-want +got):\n%s", diff)
  }
  ```
- Use `protocmp.Transform()` for proto-message comparisons and `cmpopts.EquateErrors()` for errors.
- Do **not** compare verbatim JSON strings — protojson output is non-deterministic and produces flaky tests. Compare structured objects instead.

## Commit Message Convention

Only `fix` and `feat` types are permitted (other types like `chore`, `refactor`, `docs` are not allowed in this repo):
- `fix: …` → patch release
- `feat: …` → minor release
- `feat!: …` / `fix!: …` → breaking change, major release
