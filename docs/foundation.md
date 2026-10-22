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

## Secret storage

`secrets.SecretStore` is a framework-neutral, injectable get/set/delete
boundary with a stable missing-secret sentinel and caller-owned byte slices.
The concurrency-safe memory implementation exists only as an explicit test
adapter; it is never selected as a fallback.

The MyGo adapter delegates exclusively to the operating-system credential store
under the stable `<application>/<environment>` service namespace. Namespace
parts must be non-empty and cannot contain the reserved separator or NUL. The
adapter maps missing values to the framework sentinel while preserving MyGo's
original missing error, and wrapping preserves `errors.ErrUnsupported` when an
OS credential store is unavailable. A shared contract verifies empty secrets,
overwrite/delete behavior, byte ownership, cancellation, and concurrent use.

## Milestone 3 acceptance

One typed acceptance suite runs against both SQLite and bbolt. It persists and
reloads `false`, zero, and an empty string through a real close/reopen; proves a
flag beats an environment value without replacing the persisted zero; rejects
malformed JSON and wrong persisted types; and confirms failed validation leaves
the last good value intact. The bbolt process-lock test returned its actionable
timeout after 0.95 seconds, within the fixed one-second bound.

The opt-in MyGo credential-store smoke reached the real Linux backend, which
reported `errors.ErrUnsupported`: this runner has no session bus or Secret
Service and cannot autolaunch D-Bus without an X11 display. The test therefore
skipped without storing a fallback. No real OS keyring round trip is claimed on
this host; milestone 7 repeats the probe on available platform runners.

## Frontend request normalization

The framework-neutral `runRequest` helper accepts a closure around an existing
generated MyGo method; it does not know transport names or issue RPC calls of
its own. A valid success becomes `{ ok: true, data }`, including `false`, zero,
and empty strings. A valid backend failure becomes `{ ok: false, problem }`
without changing its code, message, or field errors.

Impossible data/problem combinations produce a safe internal problem. A thrown
bridge or transport value produces a generic unavailable problem without
copying the rejection text into user-visible state. Focused Bun tests cover all
of these cases, and regeneration confirms the helper remains compatible with
the generated response types.

The dependency-free `createForm` controller publishes copied state snapshots
for values, dirty and touched fields, submission, field errors, and a form
error. `setField` updates typed values and clears errors for the edited field;
`reset` can restore or replace the baseline. Subscriptions receive the initial
state immediately and return an unsubscribe function.

Submission captures the current values, returns the same active promise to
duplicate callers, and clears pending state for success, backend failure, or an
unexpected callback throw. Backend errors map only to known fields, with
unmapped context retained as the form error. Reset and dispose advance a
generation guard so late Promise completion cannot mutate obsolete state; this
does not claim to cancel the underlying Go request.

The vanilla notes screen uses these helpers directly. Its subscription keeps
the input, pending label, disabled state, field alert, and form alert in sync;
successful submission resets the controller before reloading notes.
Availability failures remain a single global alert, while backend validation
stays attached to the input with `aria-invalid`. A controlled browser pass held
one create request while dispatching two submissions, observed one transport
call, then verified the successful reset and safe transport-failure state.

## Outbound HTTP client

[`httpclient`](../httpclient/httpclient.go) configures the pinned stable Resty
v2.17.2 client without copying its API. A validated optional base URL, a
30-second default or caller-supplied timeout, JSON accept/content headers,
caller headers, and an optional `http.RoundTripper` are applied once. Retries
are explicitly zero by default. `Resty` returns the underlying client, while
`Request` is the narrow framework entry point that attaches the caller's
context and a metric operation.

Every received response returns an error for status 400 or above while keeping
the Resty response available for typed error decoding. The error includes only
operation, method, host, and status. Response and transport outcomes record
operation, host, status, and duration through `slog`; authorization values,
URLs, and request or response bodies are never added. Focused tests cover a
real JSON `httptest` round trip, status errors and log redaction, caller-supplied
transport behavior, invalid configuration, native client access, and the
zero-retry default.

Retries require an explicit `RetryOptions` value and are capped at five after
the initial attempt. Only transport failures, HTTP 429, and HTTP 5xx responses
qualify. GET, HEAD, and OPTIONS may retry; POST, PUT, PATCH, and DELETE remain
single-attempt unless `Writes` is explicitly true. Caller cancellation reaches
active requests and interrupts retry waits; the client timeout reaches each
active request. Resty's own raw retry logger is disabled so full URLs and
transport details cannot bypass the structured metric policy.

Local server tests observe the request context closing for both the configured
30-millisecond timeout and direct caller cancellation. They also prove a read
can succeed on its third configured attempt, a write remains at one attempt
under the same retry policy, and the same write retries only after the explicit
write opt-in.

## Framework CLI root

[`internal/cli`](../internal/cli/root.go) owns a Cobra v1.10.2 `framework`
root with stable help and version flags plus stdout shell completion for Bash,
Fish, PowerShell, and Zsh. The constructor accepts future leaf commands as
explicit extensions and passes the execution context through Cobra; it imports
no desktop bootstrap, storage, logging, keyring, or MyGo application package.

[`cmd/framework`](../cmd/framework/main.go) is limited to signal-aware context,
command execution, one stderr error, and the process exit code. Focused tests
exercise help, version, completion, invalid shell input, context propagation,
and an injected generation command from an empty working directory. The real
binary produced 16,333 bytes of Bash completion without creating a database,
log, or any other file in that directory.

## Service discovery

[`internal/generator`](../internal/generator/discovery.go) uses
`go/packages` from x/tools v0.52.0 with syntax and type information; it never
imports or executes application constructors. Only methods with an explicit
`framework:command` or `framework:mcp` doc marker are returned. The supported
signature is deliberately one shape: a non-generic service method accepting
`context.Context` and one named struct value, then returning one named struct
value and `error`.

Discovery records package, service, method, description, input/output identity,
source position, command name, and optional MCP name plus explicit readonly and
destructive booleans. Contradictory effects, malformed markers, unsupported
signatures, missing descriptions, and duplicate command or tool names fail with
file/line diagnostics. Results sort by package, service, and method regardless
of source traversal order. The reference note Create/List methods now opt into
both desktop and MCP generation; unmarked exported methods remain private to
the Go application and outside generated transports.

## Generated adapters

`framework generate` discovers annotated services without importing application
bootstrap code, renders all output in memory, formats Go source, and then owns
exactly four files: `app/bootstrap/framework_generated.go`,
`app/desktop/framework_generated.go`, `framework_bindings_generated.go`, and
`app/mcp/framework_generated.go`. Desktop output contains only opted-in
commands, concrete wire responses, and readiness-gated service calls. Bootstrap
output explicitly registers named hand-written providers and supplies adapter
resolvers. The root registration stays explicit, while MCP output is limited to
selected tool metadata and declared effects.

Generation refuses to replace a file without its generated header and
preflights the complete set before writing. `framework generate --check`
reports stale or missing output without writes. A repeated run is byte-identical;
hand-written transport support belongs outside the three generated files.

## Focused scaffolds

`framework make service|request|model|migration|command NAME` creates one
formatted file in the matching application package. Names are restricted to
ASCII letters and digits with optional `-` or `_` separators, then normalized
to a Go identifier and snake-case filename. Files are created exclusively, so
an existing hand-written file is never truncated or replaced.

Migration scaffolds use the next five-digit Goose sequence. Asking for the same
migration again resolves to its original filename and fails as an existing file
instead of assigning a different schema identity. Focused scaffolds do not run
application bootstrap or framework generation; the feature workflow composes
those steps when it owns the whole change.

## Feature scaffold

`framework make feature NAME` is the primary vertical workflow. It preflights
all destinations, then creates a model, validated create request, persistent
service, explicit bootstrap provider, and forward/down Goose migration. The
service method opts into desktop generation; MCP exposure remains a separate,
deliberate annotation. If any write or discovery step fails, newly created
hand-written files are removed.

After the hand-written slice exists, the command runs framework generation once
before reporting success. Regeneration replaces only the four owned files, so
developers can implement or extend the service and provider normally. Tests also
remain feature-extensible: they continue to assert the Notes contract and core
runtime graph without assuming Notes is the only generated feature.

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
