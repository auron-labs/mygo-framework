# Project guidance

## Use the project workflows

- Install pinned frontend tools with `bun install --frozen-lockfile`.
- Run `go tool framework dev` for development and `go tool framework build` for packaging.
- Run `go tool framework doctor` after setup or when native builds stop working.
- Before committing, run `go tool framework generate --check`, `go test ./...`,
  `CGO_ENABLED=0 go test ./...`, `bun run test:web`, and `bun run typecheck`.

## Keep feature ownership clear

- Put transport-independent behavior in `app/services`. Validate typed request
  structs at each public service entry point.
- Keep persistence models in `app/models`, request types in `app/requests`, and
  explicit provider wiring in `app/bootstrap`.
- Use GORM through the one injected database. Add schema changes as new Goose
  files in `database/migrations`; never rewrite an applied migration.
- Give each closable resource one owner. Preserve startup rollback, readiness
  gating, request cancellation, and dependency-ordered shutdown.
- Phones may end an app in the background without a quit. When the system
  suspends the app, `main.go` checkpoints the SQLite write-ahead log within a
  bounded time, then flushes queued error reports with their own bound. Nothing
  is released, so the app keeps working on resume.
- The page hands the note body the user has typed but not submitted to the
  bound `NoteDraft`; `App.OnSaveState` keeps it for the next launch, which seeds
  the form from it. `App.OnSaveState` holds one function: an app that adds
  `mygo.KeepState` must fold the draft into what it keeps.

## Generate only owned adapters

- Prefer `go tool framework make feature NAME` for a vertical feature. Use focused
  `make service|request|model|migration|command` commands only when needed.
- After changing an exposed signature or `framework:` annotation, run
  `go tool framework generate` and then `bun run generate`.
- Never hand-edit `app/bootstrap/framework_generated.go`,
  `app/desktop/framework_generated.go`, `app/mcp/framework_generated.go`,
  `framework_bindings_generated.go`, or `src/mygo.ts` by hand.
- Keep MCP exposure explicit. A `framework:command` marker alone must not create
  an MCP tool; every MCP marker declares readonly and destructive effects.

## Preserve trust boundaries

- Errors reach Sentry only through `errorreport` and only when `SENTRY_DSN` is
  set. Report at boundaries (`logging.ReportUnexpected`, jobs, startup); call
  `errorreport.Flush()` before any new exit path. Do not add tracing or replay.

- Return safe application problems to UI and MCP callers. Log unexpected causes
  once without request bodies, authorization values, secrets, or database paths.
- Store secrets only through the OS credential adapter. Do not add a plaintext
  fallback.
- Keep UI calls on MyGo bindings. Do not add a local web server for ordinary UI
  requests or a second file watcher around MyGo.
- Close the desktop app before running `go run ./cmd/appctl db status|up`.
