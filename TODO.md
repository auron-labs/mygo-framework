# MyGo framework implementation checklist

This is the execution checklist for [`PLAN.md`](PLAN.md), which remains the
authoritative specification. Items are ordered by the plan's milestones and are
completed one at a time. Each checked item is committed with its implementation
and verification evidence before the next item starts.

## Progress

- Completed milestone: none
- Current item: M1.1 — scaffold the pinned plain MyGo web notes application
- Last verification: documentation-only checklist review; no runtime checks yet
- Next action: implement M1.1 and keep generation free of startup side effects

## Working rules

- Keep the reference application runnable at every milestone boundary.
- Treat every checkbox below as one commit-sized item; do not combine items.
- Update this file in the same commit that completes an item, including the
  actual focused command and result in the verification log.
- Follow the package choices, ownership rules, exclusions, and completion rules
  in [`PLAN.md`](PLAN.md); this checklist does not replace their detail.
- Keep generator/help paths independent of runtime startup and keep optional
  adapters out of the default runtime import graph.
- Never weaken trust-boundary validation, resource cleanup, cancellation,
  secret handling, generated-file safety, or the platform checks in the plan.

## Verification log

| Item | Command or check | Result |
| --- | --- | --- |
| Checklist | Compared every task group with `PLAN.md` sections and milestones | Pass |

## Milestone 1 — verify the foundation

Reference: [`PLAN.md` — Milestone 1](PLAN.md#milestone-1-verify-the-foundation),
[`PLAN.md` — selected packages](PLAN.md#selected-packages), and
[`PLAN.md` — MyGo registration and startup](PLAN.md#registration-and-startup).

- [ ] **M1.1** Scaffold a pinned plain MyGo web notes application with the
  supported entry point, frontend layout, checked-in lockfile, and explicit
  bindings that perform no storage, logging, migration, keyring, or listener
  work before `App.Run`.
- [ ] **M1.2** Add the non-GUI SQLite probe using one `database/sql` pool shared
  by ncruces gormlite and Goose, an embedded notes migration, and a real
  store/reload integration test.
- [ ] **M1.3** Add one concrete `Data`/`Problem` desktop response and prove MyGo
  Go-to-TypeScript generation succeeds without opening storage.
- [ ] **M1.4** Prove the probe and application build with `CGO_ENABLED=0`, record
  the pool configuration and WAL compatibility result, and verify the shared
  pool is closed by exactly one owner.
- [ ] **M1.5** Record toolchain versions plus comparable dependency-warm clean
  build, incremental service edit, startup-to-ready, and packaged-size baseline
  measurements using an isolated clean-build cache.
- [ ] **M1.6** Audit the milestone acceptance criteria: reload a note, regenerate
  bindings without side effects, preserve pinned compatible APIs, and record any
  measured API correction before moving to runtime work.

## Milestone 2 — runtime and one complete feature

Reference: [`PLAN.md` — Milestone 2](PLAN.md#milestone-2-runtime-and-one-complete-feature),
[`PLAN.md` — architecture](PLAN.md#architecture), and
[`PLAN.md` — shutdown and context ownership](PLAN.md#shutdown-and-context-ownership).

- [ ] **M2.1** Implement the core application error codes, cause-preserving
  `Problem` mapping, validator v10 setup, JSON/nested field-name mapping, and
  focused validation tests.
- [ ] **M2.2** Implement synchronized runtime readiness so callers can wait with
  their context, startup failure becomes `unavailable`, and new work is rejected
  after shutdown begins.
- [ ] **M2.3** Add explicit samber/do v2 provider composition, eagerly resolve the
  required graph, and keep container lookups out of business service methods.
- [ ] **M2.4** Add development `slog` output and packaged rotating-file logging
  with safe fields, one-boundary unexpected-error reporting, and no payload or
  secret logging.
- [ ] **M2.5** Move the proven database/migration path behind the runtime owner,
  apply embedded forward migrations before readiness, and test migration
  persistence, failure reporting, rollback cleanup, and one-time disposal.
- [ ] **M2.6** Implement ordinary create/list note service methods with direct
  constructor dependencies, context-aware GORM calls, service-entry validation,
  and service-owned transaction boundaries.
- [ ] **M2.7** Add the narrow MyGo notes adapter with concrete per-operation wire
  responses, readiness checks, and no exported lifecycle/provider methods.
- [ ] **M2.8** Connect the web UI to create/list notes and render starting,
  startup-failure, empty, validation-error, and persisted-note states.
- [ ] **M2.9** Add one cancellable app-lifetime background operation and bounded,
  idempotent quit-stage shutdown that stops work before releasing services and
  the database, without blocking the UI thread.
- [ ] **M2.10** Prove the complete feature: UI create/list, field errors, restart
  persistence, visible startup failure, request cancellation, and released
  resources; keep the core unit/integration suite GUI-independent.

## Milestone 3 — configuration and secrets

Reference: [`PLAN.md` — Milestone 3](PLAN.md#milestone-3-configuration-and-secrets),
[`PLAN.md` — persistent configuration](PLAN.md#persistent-configuration), and
[`PLAN.md` — OS keystore](PLAN.md#os-keystore).

- [ ] **M3.1** Define registered typed setting keys and the small JSON
  read/write/delete contract, preserving defaults, explicit zero values,
  validation, contextual decode/type errors, and goroutine safety without a
  mutable cache.
- [ ] **M3.2** Implement the SQLite settings adapter in a framework-namespaced
  versioned migration on the shared pool and run the common persistence tests.
- [ ] **M3.3** Implement explicit flag/environment overlays with
  flag → environment → persisted → default precedence and prove overrides never
  persist silently.
- [ ] **M3.4** Implement the optional bbolt adapter with one bucket, copied read
  bytes, bounded lock timeout, actionable busy errors, one close owner, and the
  same adapter contract tests without adding it to the default app import graph.
- [ ] **M3.5** Define the injectable `SecretStore`, add a memory test adapter, and
  add a stable app/environment-namespaced MyGo Secrets adapter that preserves
  missing and OS-unavailable errors without plaintext fallback.
- [ ] **M3.6** Prove both settings backends survive restart, reject invalid data,
  preserve zero values, keep overlays transient, fail a locked bbolt promptly,
  and report available real-keystore smoke coverage accurately.

## Milestone 4 — forms and HTTP helpers

Reference: [`PLAN.md` — Milestone 4](PLAN.md#milestone-4-forms-and-http-helpers),
[`PLAN.md` — forms and frontend requests](PLAN.md#forms-and-frontend-requests), and
[`PLAN.md` — outbound HTTP requests](PLAN.md#outbound-http-requests).

- [ ] **M4.1** Add a framework-neutral TypeScript request helper that consumes
  MyGo-generated types, unwraps `Data`/`Problem`, and normalizes unexpected
  transport failures without creating another RPC client.
- [ ] **M4.2** Add the dependency-free form state helper with values, dirty and
  touched state, submitting state, field/form errors, reset, setField,
  subscribe, submit, duplicate-submit prevention, and disposed-completion guards.
- [ ] **M4.3** Test form success, failure, duplicate submission, reset, disposal,
  and field-error mapping, then update the vanilla notes form to use the helpers.
- [ ] **M4.4** Add the configured Resty v2 client with context, timeout, base URL,
  headers, JSON behavior, consistent HTTP errors, safe request metrics, supplied
  test transport, underlying-client access, and retries disabled by default.
- [ ] **M4.5** Prove timeout/cancellation propagation, explicit bounded retry
  behavior, and that writes are never retried without explicit configuration by
  using a local `httptest` server.

## Milestone 5 — CLI and desktop generation

Reference: [`PLAN.md` — Milestone 5](PLAN.md#milestone-5-cli-and-desktop-generation),
[`PLAN.md` — commands to implement](PLAN.md#commands-to-implement), and
[`PLAN.md` — generation rules](PLAN.md#generation-rules).

- [ ] **M5.1** Add the Cobra `framework` root with stable help, flags, completion,
  and extension points while proving help and generator commands do not start
  runtime services or open application storage.
- [ ] **M5.2** Implement go/packages/type-based annotation discovery for the one
  supported service signature with command/MCP opt-in markers, descriptions,
  effect metadata, stable ordering, and file/line diagnostics for unsupported or
  duplicate declarations.
- [ ] **M5.3** Generate formatted, narrow desktop adapters, concrete responses,
  explicit registrations, and selected MCP registration stubs; support initial
  generation before generated imports exist, byte-identical repeat runs, and
  `generate --check` without writes.
- [ ] **M5.4** Add focused service, request, model, migration, and command
  scaffolds that validate names, refuse to overwrite hand-written files, and
  preserve stable migration identity.
- [ ] **M5.5** Add the primary feature scaffold for the approved request, service,
  model, migration, adapter/provider shape, followed by generation, while
  preserving existing feature implementation on regeneration.
- [ ] **M5.6** Implement `framework new NAME` for one supported MyGo web template
  with pinned dependencies, explicit bootstrap, the form helper, migrations,
  generated project guidance, and no speculative template variants.
- [ ] **M5.7** Implement `framework dev`, `build`, and `doctor` by running the
  framework generation/check exactly once and delegating to pinned MyGo tooling;
  document the manual adapter-regeneration rule and add no second watcher.
- [ ] **M5.8** Implement `appctl db status` and `db up` with the shared profile/path
  resolver, explicit data-directory override, and a minimal offline bootstrap;
  status must not migrate, and neither command starts MyGo, services, secrets,
  workers, or a local RPC bridge.
- [ ] **M5.9** Run the generator acceptance flow in a temporary directory: create
  a fresh project, generate a second feature, build it, invoke it through the real
  MyGo-generated client, prove a second run is clean, detect deliberate stale
  output, and preserve hand-written code.

## Milestone 6 — MCP generation and runtime

Reference: [`PLAN.md` — Milestone 6](PLAN.md#milestone-6-mcp-generation-and-runtime)
and [`PLAN.md` — MCP integration](PLAN.md#mcp-integration).

- [ ] **M6.1** Complete generated official-SDK typed MCP wrappers for explicitly
  annotated methods only, including descriptions, type-derived schemas, effect
  metadata, duplicate-name rejection, and ordinary service input/output calls.
- [ ] **M6.2** Add the optional in-process stateless Streamable HTTP server after
  runtime readiness, disabled by default, bound to loopback with configurable
  port, reusing the existing service instances and stores for every request.
- [ ] **M6.3** Enforce SDK localhost Host protection, Go cross-origin protection,
  and a per-launch bearer token; expose connection details only through an
  explicit developer action and never normal logs.
- [ ] **M6.4** Map domain failures to MCP error results with concise text and
  structured field details, propagate cancellation, and publish the example's
  note-change event through a small UI callback adapter rather than an event bus.
- [ ] **M6.5** Prove one real SDK client list/call round trip, annotated-only
  exposure, validation failures, UI-visible mutation, rejected Host/origin/token,
  no second store, and listener shutdown before shared service disposal.

## Milestone 7 — cross-platform verification and performance

Reference: [`PLAN.md` — Milestone 7](PLAN.md#milestone-7-cross-platform-verification-and-performance)
and [`PLAN.md` — testing and completion rules](PLAN.md#testing-and-completion-rules).

- [ ] **M7.1** Run supported Windows, macOS, and Linux build checks with
  `CGO_ENABLED=0`, keep race-test requirements separate, and distinguish compile
  evidence from runtime evidence on each platform.
- [ ] **M7.2** Run the startup/create/restart/shutdown desktop smoke flow on each
  available real OS runner, including permitted real-keystore checks, and record
  unavailable platforms without overstating coverage.
- [ ] **M7.3** Measure plain MyGo, default framework, and MCP-enabled variants on
  the same toolchain/hardware for dependency-warm clean build, incremental build,
  startup readiness, and package size; fix only clear accidental regressions and
  publish the measured comparison.

## Milestone 8 — handoff and bounded cleanup

Reference: [`PLAN.md` — Milestone 8](PLAN.md#milestone-8-handoff-and-bounded-cleanup)
and [`PLAN.md` — definition of done](PLAN.md#definition-of-done).

- [ ] **M8.1** Finish the runnable notes reference app and remove only scaffolding
  made obsolete by the completed implementation.
- [ ] **M8.2** Write the quick start and focused examples for adding a feature,
  saving a setting, reading a secret, calling HTTP, defining a migration, and
  exposing an MCP tool, including exact development/build/test commands.
- [ ] **M8.3** Generate concise project `AGENTS.md` guidance and a capability
  matrix mapping every requested feature to its implementation and verification,
  with evidence for any genuinely blocked platform check.
- [ ] **M8.4** Perform the final fresh-project audit: create a project, add and use
  a second feature through the documented path, build/open the usable UI, verify
  persistence/settings/forms/HTTP/shutdown/optional MCP, inspect the final diff
  and dependency graph, and confirm every `PLAN.md` definition-of-done statement.

## Explicitly out of scope

As specified by [`PLAN.md`](PLAN.md), do not add Goravel, automatic backend
conversion, settings sync or watchers, repositories/factories for every service,
another UI framework, a custom ORM/DI/MCP engine, a second development watcher,
ordinary-UI HTTP RPC, remote MCP hosting, stdio forwarding, arbitrary shell/SQL
tools, RBAC/OAuth, cloud sync, queues, plugins, SQL triggers, or an admin panel.
