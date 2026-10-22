# Capability matrix

This matrix maps the requested framework capabilities to the current source and
direct verification. The status describes this repository on 22 October 2026;
it is not a claim about an external release.

The generated project template includes the SQLite and bbolt settings stores,
memory and MyGo secret adapters, outbound HTTP client, and settings migration.
The final fresh-project acceptance compiles and exercises those copied packages
so their availability does not depend on the framework checkout.

| Capability | Implementation | Direct evidence | Status and limits |
| --- | --- | --- | --- |
| Persistent configuration | Typed keys and uncached values in [`settings`](../settings/settings.go), flag/environment overlays in [`settings/overlay.go`](../settings/overlay.go), default SQLite storage, and optional bbolt storage | Contract, restart, invalid-data, precedence, concurrency, and process-lock tests in [`settings`](../settings/acceptance_test.go), [`settings/sqlite`](../settings/sqlite/sqlite_test.go), and [`settings/bbolt`](../settings/bbolt/bbolt_test.go) | Implemented and verified. SQLite is the reference-app default; bbolt is opt-in. |
| OS keystore | Framework-neutral [`SecretStore`](../secrets/secrets.go), MyGo OS adapter, and in-memory test adapter | Shared contract tests plus missing, namespace, cancellation, and platform-error checks in [`secrets/mygo`](../secrets/mygo/mygo_test.go) and [`secrets/memory`](../secrets/memory/memory_test.go) | Implemented without plaintext fallback. The non-interactive Linux real-store probe reached GNOME Keyring but could not unlock it; macOS and Windows real-store runs were unavailable. |
| ORM and local database | One owned `database/sql` pool shared with GORM through ncruces gormlite in [`internal/database`](../internal/database/database.go) | Pool identity, WAL, foreign keys, busy timeout, migration, restart, and one-time close tests in [`internal/database`](../internal/database/database_test.go) | Implemented and verified with `CGO_ENABLED=0`. |
| Migrations | Embedded, ordered Goose SQL files in [`database/migrations`](../database/migrations) plus offline status and up commands in [`internal/appctl`](../internal/appctl/database.go) | Migration persistence/failure tests and read-only status/up tests in [`internal/database`](../internal/database/database_test.go) and [`internal/appctl`](../internal/appctl/database_test.go) | Implemented. Applied migrations are immutable; only forward `up` is exposed. |
| Dependency injection and services | samber/do composition in [`app/bootstrap`](../app/bootstrap/bootstrap.go) with constructor-injected ordinary services such as [`Notes`](../app/services/notes.go) | Eager-resolution, missing-provider, disposal-order, transaction, cancellation, and rollback tests in [`app/bootstrap`](../app/bootstrap/bootstrap_test.go) and [`app/services`](../app/services/notes_test.go) | Implemented. Container lookup stays at composition and adapter boundaries. |
| Logging | Contextual `slog`, development stderr, packaged rotating files, redaction, and GORM adapter in [`logging`](../logging) | Sink, rotation, cancellation, redaction, and payload-free ORM tests in [`logging`](../logging/logging_test.go) and [`logging/gorm_test.go`](../logging/gorm_test.go) | Implemented and verified. |
| Forms | Framework-neutral observable form controller in [`src/framework/form.ts`](../src/framework/form.ts) | State, reset, validation, duplicate-submit, exception, and obsolete-completion tests in [`tests/web/form.test.ts`](../tests/web/form.test.ts) | Implemented and used by the Notes UI. Ignoring stale completion does not cancel the underlying Go call. |
| Frontend request helper | Typed wire-response unwrapping and safe transport-error normalization in [`src/framework/request.ts`](../src/framework/request.ts) | Success, falsy values, backend problem, invalid envelope, and thrown transport tests in [`tests/web/request.test.ts`](../tests/web/request.test.ts) | Implemented and used by the Notes UI. |
| Outbound HTTP | Configured Resty v2 client with contexts, JSON defaults, safe metrics, bounded opt-in retries, and underlying-client access in [`httpclient`](../httpclient/httpclient.go) | JSON, headers, status, timeout, caller cancellation, retry, and write-gate tests in [`httpclient`](../httpclient/httpclient_test.go) | Implemented. Retries default off; writes require explicit retry opt-in. |
| Validation and application errors | validator v10 JSON-path mapping in [`validation`](../validation/validation.go) and cause-preserving safe problems in [`problem`](../problem/problem.go) | Nested fields, stable codes, cause identity, cancellation, and internal-message tests in [`validation`](../validation/validation_test.go) and [`problem`](../problem/problem_test.go) | Implemented across desktop and MCP adapters. |
| Generators | Source inspection, deterministic owned adapters, focused scaffolds, vertical features, and embedded project template in [`internal/generator`](../internal/generator) | Preservation, refusal, determinism, rollback, stale-check, and disposable fresh-project acceptance tests in [`internal/generator`](../internal/generator/acceptance_test.go) | Implemented. Hand-written files are never overwritten by regeneration. |
| CLI and developer workflow | Cobra `framework` commands in [`internal/cli`](../internal/cli) and GUI-independent `appctl` commands in [`internal/appctl`](../internal/appctl) | Help/version/completion, context, generation, MyGo delegation, scaffold, new-project, and offline database tests in the corresponding packages | Implemented. MyGo owns watch, build, package, and native diagnostics. |
| MCP generation and runtime | Explicit service annotations generate official SDK tools; an optional authenticated loopback runtime shares the desktop graph in [`app/mcp`](../app/mcp) | Real SDK list/call, schema, validation, bearer, Host, origin, shared-service, UI-change, and shutdown-order tests in [`app/bootstrap/mcp_acceptance_test.go`](../app/bootstrap/mcp_acceptance_test.go) and [`app/mcp`](../app/mcp/runtime_test.go) | Implemented. Disabled by default; each enabled launch uses a new bearer token and `127.0.0.1` listener. |
| Startup and shutdown | Synchronized readiness, rollback, app-lifetime context, and bounded MyGo quit stage in [`runtime`](../runtime) and [`app/desktop`](../app/desktop/quit.go) | Readiness, worker, quit re-entry, idempotence, deadline, and disposal-order tests plus packaged Linux smoke | Implemented and verified on Linux/amd64. |
| Cross-platform build and desktop runtime | MyGo packaging with a CGO-free Go package graph | All packages compiled for Linux, macOS, and Windows on amd64 and arm64; Linux packaged startup/create/restart/shutdown was exercised through WebKitGTK | Compile-verified for six targets. Runtime-verified only on Linux/amd64; no macOS, Windows, or Linux/arm64 runner was available. |

## Definition-of-done audit

| Required outcome | Final evidence | Result |
| --- | --- | --- |
| A fresh application builds through MyGo | The compiled CLI created a 55-file app, added Bookmark, installed the frozen Bun graph, regenerated both adapter layers, passed `go build ./...`, and produced the Linux executable, Debian package, and tarball through `framework build` | Pass |
| The application opens a usable UI | The fresh packaged app opened a 960×640 WebKitGTK window under Xvfb, accepted keyboard input through its real form, rendered `fresh audit persists`, and kept the form usable | Pass on Linux/amd64 |
| Application records and settings persist | The UI-created note rendered again after process restart. Fresh-project acceptance stored typed settings through the shared SQLite pool and the optional bbolt adapter | Pass |
| The application can use OS-protected secrets | The generated project compiles the MyGo adapter and exercises the same `SecretStore` contract through its memory adapter; root adapter tests preserve missing and unavailable errors and forbid plaintext fallback | Implemented with real-store limitation below |
| Forms validate and recover correctly | All 16 fresh-project request/form tests passed, including field mapping, duplicate submission, thrown transport errors, reset, and stale completion | Pass |
| Services can make cancellable HTTP requests | Fresh-project acceptance called a local JSON server through the copied configured client; root tests cover timeout, caller cancellation, safe metrics, and retry policy | Pass |
| Shutdown is clean and bounded | The fresh packaged app received SIGTERM after its window appeared and exited with status 0. Root lifecycle and MCP tests cover re-entry, deadlines, listener ordering, active-handler drain, and one-time database disposal | Pass |
| Explicit services can become MCP tools | Bookmark gained an explicit MCP marker; generation exposed `bookmarks_create` beside the two Notes tools, and an official SDK client created `MCP bookmark` through the fresh app's authenticated loopback endpoint | Pass |
| UI and MCP share ordinary application services | Notes UI/MCP acceptance asserts pointer identity and UI-visible MCP mutation; generated Bookmark uses the same provider/resolver path | Pass |
| No Goravel dependency or mandatory UI server exists | Dependency inspection finds neither Goravel nor generator/CLI/`x/tools` packages in the runtime graph. The normal UI uses MyGo bindings; MCP remains disabled by default | Pass |
| Generators make a second feature without losing hand-written code | Fresh-project acceptance scaffolds Bookmark, preserves an appended implementation marker across repeated generation, detects deliberate stale output without writing, then repairs it | Pass |

The fresh runtime graph contains 365 packages and no generator, CLI, or
`golang.org/x/tools` package. Normal, race, and CGO-free repository suites,
static analysis, 32 web tests, production web build, generated-file freshness,
module tidiness, and a 100-run CGO-free MCP shutdown stress check all pass.

## Recorded platform limitations

- Linux Secret Service activated GNOME Keyring, but the non-interactive runner
  did not complete credential-store unlock within the 20-second bound. The probe
  exited with status 124, so no real secret round trip is claimed.
- macOS and Windows binaries compiled with `CGO_ENABLED=0`, but this Linux host
  could not execute their desktop, shutdown, or credential-store smoke checks.
- Linux/arm64 also has compile evidence only because no arm64 runtime was
  available.

See the [`TODO.md` verification log](../TODO.md#verification-log) for the exact
commands and results. See [foundation.md](foundation.md) for the contracts,
security boundaries, cross-platform table, and measured performance comparison.
