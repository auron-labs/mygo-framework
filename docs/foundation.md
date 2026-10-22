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
| validator | 10.30.5 |
| samber/do | 2.1.0 |
| Lumberjack | 2.2.1 |
| bbolt (optional) | 1.5.0 |

These were tagged releases on 22 October 2026. The MyGo packages use the same
release to keep the generated client and Go runtime compatible.

## Compatibility corrections

The MyGo 0.5.5 initializer selected the latest Go module, which was 0.6.2.
The generated npm packages remained on 0.5.5. The repository now pins the Go
module, CLI, and runtime packages to 0.5.5 instead of accepting that mismatch.

The current gormlite release is a separate Go module at
`github.com/ncruces/go-sqlite3/gormlite`. It is pinned to 0.34.0 alongside the
0.35.6 SQLite driver. Its `OpenDB` API accepts the existing `sql.DB`, so the
plan's shared-pool design works without an adapter or a second connection.

Validator 10.30.5 requested retracted mimetype 1.4.15, whose module notice
reports quadratic work for crafted CDF files. The dependency graph pins the
compatible 1.4.16 patch instead. `go list -m -retracted all` reports no
retracted selected module after that correction.

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

## Structured logging

[`logging`](../logging/logging.go) constructs the `slog.Logger` injected into
application components. Development output goes to stderr. Packaged output is
mirrored to stderr and `<log directory>/app.log`, where Lumberjack rotates files
with exposed size, backup-count, age, and compression settings. The default
policy is 10 MB per file, three backups, 28 days, and compression enabled.

The handler redacts common authorization, cookie, password, payload, body,
secret, settings, and token fields, including attributes nested under a
sensitive group. `ReportUnexpected` is the single adapter-boundary helper for
unexpected failures. It records operation and component fields while omitting
normal cancellation and deadline errors.

## Runtime resource ownership

[`app/bootstrap`](../app/bootstrap/bootstrap.go) registers providers without
opening resources. Its `Start` method receives paths resolved at the desktop or
CLI edge, opens logging first, opens the one shared database pool, applies the
embedded forward migrations, and only then publishes readiness. Database and
logger resources participate in dependency-ordered disposal.

A failed startup publishes one cause-preserving `unavailable` result and rolls
back every resource that opened. Migration errors name the failing SQL file,
while operational logs replace the complete database path with a placeholder.
Runtime shutdown and failed-start rollback each have one-shot guards; repeated
shutdown does not close the shared pool twice.

## Note service path

[`app/services`](../app/services/notes.go) contains the transport-independent
note feature. Its constructor receives GORM and the shared validator directly.
Create and list accept concrete inputs from `app/requests`, validate at the
public service entry point, and use the caller's context for every database
operation. Create owns its GORM transaction; an error after insertion rolls the
write back before it reaches an adapter.

The runtime installs a GORM handler backed by the configured `slog.Logger`.
It records safe component, operation, duration, and row-count diagnostics but
never SQL text or parameter values. Query errors continue to the adapter for
the single unexpected-failure record.

## Desktop adapter

[`app/desktop`](../app/desktop/notes.go) is the only note object bound to MyGo.
It exports `Create` and `List` and no lifecycle or provider methods. Each call
waits on runtime readiness with MyGo's request context, resolves the shared
service only after readiness, and returns a concrete nullable `Data`/`Problem`
response. Validation, startup, and cancellation failures keep their stable
codes; only unexpected internal failures produce one desktop-boundary log.

The executable registers that adapter before `App.Run` without resolving a
provider. Once MyGo is ready, it resolves `PathUserData` and `PathLogs`, starts
the runtime in a background goroutine, and opens the window immediately. MyGo
generation therefore sees the production binding set without opening logs,
storage, or migrations.

## Notes UI states

The vanilla frontend starts with a live runtime status and a disabled composer.
Successful list loading hides the status, enables input, and renders either the
empty message or persisted notes with an accessible count. Startup and request
availability failures keep the composer disabled and show a visible alert.

The form sends every submission through backend validation. A body field error
sets `aria-invalid`, shows an associated alert, and returns focus to the input.
Successful creation reloads the persisted list and announces completion through
the polite live region. Reduced-motion preferences disable the startup spinner,
and the composer stacks at narrow widths.

## Application lifetime and quit

The runtime starts one `LifetimeWorker` to prove app-scoped cancellation without
introducing a scheduler. It depends on the database in the provider graph, so
dependency-ordered shutdown waits for the worker before releasing storage.
Request contexts remain independent and are never used as the worker lifetime.

The desktop registers a MyGo `OnBeforeQuit` stage. The first quit is canceled,
the application context is canceled once, and runtime cleanup runs away from the
UI thread with a five-second deadline. Repeated quit attempts remain canceled
while cleanup is active. Completion or deadline expiry triggers one guarded
re-entry into MyGo quit, allowing `App.Run` to return only after cleanup finishes
or reaches its bound.

## Milestone 2 acceptance

The packaged Linux app was exercised in a real WebKitGTK window under an
isolated Xvfb/DBus session. The empty state rendered, an empty submission showed
the field error, and a valid submission persisted `persists after restart`.
SIGTERM completed through the registered quit stage with exit status 0 in 110
milliseconds. Relaunching against the same user-data directory reopened the
database and rendered the saved note.

Launching the packaged app with an invalid configuration root kept the window
open, displayed the unavailable state, and disabled submission; its later
SIGTERM also exited 0. Request-cancellation checks remain in the GUI-independent
service and desktop-adapter suites, while the packaged flow verifies that the
real bindings, storage lifecycle, and visible states work together.

## Typed settings contract

The framework-neutral `settings` package registers heterogeneous `Key[T]`
definitions against a small JSON `Store` contract. `Get`, `Set`, and `Delete`
accept only the original registered key, so unknown or same-name incompatible
keys fail before storage access. Keys carry a default and optional validation;
defaults are JSON-round-tripped so callers receive fresh maps, slices, and
pointers rather than shared mutable state.

A missing row returns the declared default, while stored `false`, zero, and
empty strings remain distinct values. Reads reject malformed JSON, wrong types,
non-nullable `null`, and values that fail validation with the key name and cause
preserved. Writes validate and encode before calling the store. The registry is
immutable after construction, adds no cache, and requires stores and validation
functions to support concurrent calls.

The default SQLite adapter stores values in the versioned
`mygo_framework_settings` table. It borrows the application's migrated
`sql.DB`, opens no connection of its own, and has no close method. A shared
adapter contract verifies exact JSON persistence through close/reopen,
overwrite/delete behavior, byte ownership, invalid-JSON rejection, context
cancellation, and concurrent use; the optional bbolt adapter will run the same
contract.

Transient settings use an immutable store overlay. Callers explicitly map typed
flag and environment values to registered keys; the overlay never scans the
process environment. Reads resolve flags, then environment values, then the
persistent store, with the typed layer supplying the default when all are
missing. Construction and reads never save an override. Explicit writes and
deletes pass through to persistence while the transient value remains in force.

The optional bbolt adapter owns one `0600` database and one
`mygo_framework_settings` bucket. It copies values before read transactions
end, validates JSON before writes, and closes its database once. Opening uses a
fixed one-second lock timeout; contention preserves bbolt's timeout sentinel and
adds an actionable busy message. It passes the shared persistence contract and
remains absent from the default application's dependency graph.

## Executed checks

The following checks passed on Linux/amd64 on 22 October 2026:

```text
go test ./...
go test -race ./...
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go build -o /tmp/mygo-framework-app .
CGO_ENABLED=0 go build -o /tmp/mygo-framework-dbprobe ./cmd/dbprobe
/tmp/mygo-framework-dbprobe -database "<temp>/notes database.db" -note "WAL and CGO-free"
go vet ./...
golangci-lint run ./...
bun run typecheck
bun run build:web
bun run generate # repeated with an identical generated-client hash
```

The database integration test asserts the pool identity, connection limit,
busy timeout, foreign-key mode, WAL mode, idempotent close, migration, and
close/reopen persistence. The probe also stored and reloaded a note from a path
containing a space while running from a `CGO_ENABLED=0` build.

## Baseline measurements

The baseline was measured on Ubuntu 25.10 with Linux 6.17.0-41-generic. The
runner exposed seven logical Intel Core i7-9700K CPUs and 28,194,418,688 bytes
of memory. It used Go 1.27.3, Bun 1.4.2, and the pinned dependencies above.

| Metric | Result | Method |
| --- | ---: | --- |
| Dependency-warm clean Go app build | 8.65 seconds | `CGO_ENABLED=0 go build .` with downloaded modules and a new empty `GOCACHE` |
| Incremental build after a bound-service edit | 0.43 seconds | Rebuilt with the same cache after an ephemeral source comment changed `main.go` |
| Startup to ready window | 374 milliseconds | Launched the packaged binary under Xvfb and measured until the `WhenReady` window title appeared in the X11 tree |
| Plain unstripped Go app binary | 13,938,697 bytes | Output of the clean `go build` above |
| Packaged Linux app binary | 9,425,159 bytes | `bun run build` output in `build/linux-amd64` |
| Linux tarball | 4,052,537 bytes | `bun run build` output |
| Debian package | 4,031,784 bytes | `bun run build` output |

The full MyGo package build completed in 11.8 seconds with the existing Go
cache and transformed seven frontend modules. Each number is one observation,
so it is a reproducible baseline rather than a statistical benchmark.
Milestone 7 repeats the same method for the default framework and MCP-enabled
variants.

The startup measurement proves that MyGo reached the readiness callback and
created the 960×640 window. It does not prove interaction inside WebKitGTK;
the milestone 2 UI flow and milestone 7 OS smoke checks cover that behavior.

These checks establish compilation and behavior on this Linux runner. They do
not establish runtime behavior on macOS or Windows; those checks remain in
[milestone 7](../PLAN.md#milestone-7-cross-platform-verification-and-performance).
