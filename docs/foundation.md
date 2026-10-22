# Foundation verification

This page records the tested foundation behind [the implementation plan](../PLAN.md).
It describes unreleased repository behavior, not a published compatibility promise.

## Pinned baseline

| Component | Version |
| --- | --- |
| Go toolchain | 1.27.3 locally; module minimum 1.27.1 |
| MyGo Go module | 0.5.5 |
| MyGo CLI and runtime packages | 0.5.5 |
| ncruces SQLite driver | 0.35.6 |
| ncruces gormlite | 0.34.0 |
| GORM | 1.31.2 |
| Goose | 3.29.0 |

These were tagged releases on 22 October 2026. The MyGo packages use the same
release to keep the generated client and Go runtime compatible.

## Shared SQLite pool

[`internal/database`](../internal/database/database.go) owns one `sql.DB`.
GORM receives that handle through `gormlite.OpenDB`. Goose borrows the same
handle while it applies the embedded migrations. The migration provider is not
closed because its `Close` method would close the borrowed pool. Only
`Database.Close` releases the pool, and repeated calls are safe.

The initial connection policy is intentionally conservative:

| Setting | Initial value | Verified behavior |
| --- | --- | --- |
| Maximum open connections | 1 | `sql.DB.Stats` reports one |
| Maximum idle connections | 1 | The single connection remains available for local work |
| Busy timeout | 5 seconds | Every opened connection receives `busy_timeout(5000)` |
| Foreign keys | Enabled | Every opened connection receives `foreign_keys(1)` |
| Journal mode | WAL | Startup requests WAL and fails if SQLite returns another mode |

The one-connection limit avoids multiplying the Wasm-backed driver's per-connection
memory cost before the application has measured concurrency needs. It also keeps
the first transaction model predictable. A later change should raise the limit
only with a workload that demonstrates the need.

## Executed checks

The following checks passed on Linux/amd64 on 22 October 2026:

```text
go test ./...
go test -race ./internal/database
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go build -o /tmp/mygo-framework-app .
CGO_ENABLED=0 go build -o /tmp/mygo-framework-dbprobe ./cmd/dbprobe
/tmp/mygo-framework-dbprobe -database "<temp>/notes database.db" -note "WAL and CGO-free"
go vet ./...
golangci-lint run ./...
```

The database integration test asserts the pool identity, connection limit,
busy timeout, foreign-key mode, WAL mode, idempotent close, migration, and
close/reopen persistence. The probe also stored and reloaded a note from a path
containing a space while running from a `CGO_ENABLED=0` build.

These checks establish compilation and behavior on this Linux runner. They do
not establish runtime behavior on macOS or Windows; those checks remain in
[milestone 7](../PLAN.md#milestone-7-cross-platform-verification-and-performance).
