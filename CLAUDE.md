# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Kuberpult

Kuberpult is a Kubernetes deployment management system that manages *what gets deployed next* across cluster environments (dev, staging, production). It complements ArgoCD (which applies current versions). Think of it as a "catapult" for rolling out microservices.

## Architecture

Five Go microservices plus shared packages:

- **cd-service** — Core logic: release management, deployment, locks, release trains. Exposes gRPC only (HTTP port serves only a `/health` check).
- **manifest-repo-export-service** — Reads DB state, pushes manifest changes to the git repository that ArgoCD watches.
- **frontend-service** — REST/gRPC-web adapter backing the React web UI.
- **rollout-service** — Coordinates direct connection to Argo CD via grpc.
- **reposerver-service** — Manages repository synchronization, implements Argo CD grpc endpoints.
- **pkg/** — Shared packages: `api/` (protobuf), `db/`, `auth/`, `logger/`, `metrics/`, `tracing/`, `testutil/`, `migrations/`.

Services communicate via gRPC (port 8443 internal, 8080 HTTP). All service config comes from environment variables. PostgreSQL is the only supported database; migrations run automatically on startup.

## Build & Run

All build targets use Docker + the builder image. The builder image must be built locally before anything else:

```bash
make builder          # Build local builder image (required first time)
make kuberpult        # Build everything and start all services via docker-compose
make kuberpult-freshdb  # Same but with a clean database
make reset-db         # Delete the postgres volume
```

Build a single service Docker image:
```bash
IMAGE_TAG=local make -C services/cd-service docker
```

## Docker Concept
Our "builder" Dockerfile contains the go.mod and runs go mod download.
Other Dockerfiles that depend on it, should not use go mod download again. They just copy from the builder.

## Testing

Tests run inside Docker using the builder image and connect to a test PostgreSQL instance.

```bash
make test                          # Run all tests (all services + pkg)
make -C services/cd-service test   # Test a single service
make -C pkg test                   # Test shared packages
```

For IDE/local test runs (without Docker), the test database must be reachable:
1. Start the test database: `make unit-test-db`
2. Run tests directly: `go test ./... -v` inside the service directory

Run a single Go test:
```bash
go test -run TestFunctionName ./path/to/package -v
```

Frontend tests (inside `services/frontend-service`):
```bash
pnpm test              # Watch mode
pnpm test-ci           # CI mode (no watch)
```


## Linting

Go linting runs via golangci-lint inside Docker:
```bash
make -C services/cd-service lint      # Lint a service
make -C services/cd-service lint-fix  # Auto-fix
```

Enabled linters: `errcheck`, `govet`, `ineffassign`, `unused`, `asciicheck`, `bodyclose`, `copyloopvar`, `staticcheck`, `unconvert`, `gocritic` (importShadow check), `revive` (redundant-import-alias). Generated code in `pkg/publicapi/` is excluded.

Frontend linting (inside `services/frontend-service`):
```bash
pnpm lint
pnpm lint-fix
```

## Code Coverage Thresholds

Coverage is enforced at test time and the build fails if thresholds are not met.
The coverage thresholds are defined in each service's `services/<service-name>/Makefile`, look for `MIN_COVERAGE`.

## Go Test Patterns

All Go tests must follow these conventions:

- Even for the simplest test, immediately create a "table" (go slice) so that testing different variations is easy in the future.
- In a table-driven test, only put the really relevant parts into the table data. Data that is identical for all cases should not be part of the table.
- Don't use function-typed fields in the table struct (e.g. `fn func() error`). Keep all logic inline in the test loop body.
- Omit the line `tc := tc` at the beginning of test loops — it is outdated (Go 1.22+ handles loop variable capture correctly).

**Table-driven tests:**
```go
tcs := []struct {
    Name string
    // ...
}{
    {Name: "happy path", ...},
}
for _, tc := range tcs {
    t.Run(tc.Name, func(t *testing.T) { ... })
}
```

**Assertions with `cmpDiff`:**
```go
if diff := cmpDiff(expected, actual); diff != "" {
    t.Errorf("mismatch (-want, +got):\n%s", diff)
}
```
Always use cmpDiff, never use cmp.Diff, as it is not type-safe.

**Proto message comparison:**
```go
if diff := cmpDiff(expected, actual, protocmp.Transform()); diff != "" {
    t.Errorf("mismatch (-want, +got):\n%s", diff)
}
```

Do not compare raw JSON strings; always compare Go objects.

## Commit Conventions

This repo uses conventional commits enforced by commitlint:
- `fix:` → PATCH bump
- `feat:` → MINOR bump
- `feat!:` / `fix!:` → MAJOR bump (breaking change)

Types `revert`, `perf`, `docs`, `test`, `refactor`, `style`, `chore`, `build`, `ci` are restricted (not allowed).

## Protobuf / API Generation

Proto definitions live in `pkg/api/`. Regenerate after changes:
```bash
make -C pkg gen
```

For frontend TypeScript types:
```bash
make -C services/frontend-service gen-api
```

## Helm Chart

The Helm chart is in `charts/kuberpult/`. Critical values:
- `git.url` — manifest repository URL (required)
- `ssh.identity` — SSH private key for git access (required)
- `pgp.keyring` — PGP keyring for signature verification (recommended)


## Types
When introducing new fields into structs, consider defining a new custom type as in `pkg/types/types.go`.
This is especially important for unique concepts, that cannot mix with anything else.
For example, there is no point in comparing an envName to an appName, so they should be separate types.

## Sleep
Invoking functions like `time.Sleep` in Go is generally an antipattern.
It should be avoided in all code, including setup, request handlers, and test code.
Always replace it with a concrete signal: a channel receive, `sync.WaitGroup.Wait()`, context cancellation, or restructured code that only proceeds when the awaited event actually occurs. The right mechanism is case-by-case.

## Database Approach
In Kuberpult we never want to lose data. Most data is relevant to keep forever.
This includes deployment data, as well as metadata. But also lock information should never be lost.
Therefore, we rarely use `DELETE` SQL statements.
Currently most database entities like apps and releases have two tables: A current version, and a history version.
The current version only stores data that is needed, but the history keeps everything.
Never delete anything from a history table!


## TypeScript
When calling a grpc API, always supply the authHeader parameter:
```typescript
const subscription = api
    .overviewService()
    .StreamOverview({}, authHeader)
```

The only exceptions are early calls that happen before the Authentication happens, for example the GetConfig call:
```typescript
api.configService()
    .GetConfig({}) // the config service does not require authorisation
```

## Nil Checks in DB code
Do not add nil checks for DBHandler (h) or sql.Tx (tx/transaction) in DB package functions.
**Why:** They are bloating the code too much. In the unlikely case that one is nil, we can deal with the panic.
**How to apply:** When writing any new function in `pkg/db/`,
skip the `if h == nil { return nil }` and `if tx == nil { return fmt.Errorf(...) }` guard clauses entirely.

## Database Queries
Format database queries in go code like this:
```go
	selectQuery := h.AdaptQuery(`
		SELECT created, name, json, applications
		FROM environments
		LIMIT 1;
	`)
```
Each main sql keyword gets its own line.

There are 2 kinds of select queries:
1) Those with exactly 1 or 0 results. If we filter for the primary key, then we do not need an ORDER BY.
If we filter by something else, an ORDER BY and LIMIT 1 is required.
2) Those with potentially a lot of results. These must have an ORDER BY and LIMIT N.

The goal is to make all queries deterministic, including the order of the result.


## Kind tests
The tests in test/kind-brackets are using the cd-, frontend-, reposerver-,
and rollout-service.
They do NOT use the manifest-repo-export.
This means the only service communicating with Argo CD in any way is the
rollout-service.

## Rollout Service
Details about the rollout-service are in a package-level comment in argo.go.
