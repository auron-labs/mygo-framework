# Project guidance

## Use the project workflows

- Install pinned frontend tools with `bun install --frozen-lockfile`.
- Run `framework dev` for development and `framework build` for packaging.
- Run `framework doctor` after setup or when native builds stop working.
- Before committing, run `framework generate --check`, `go test ./...`,
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

## Generate only owned adapters

- Prefer `framework make feature NAME` for a vertical feature. Use focused
  `make service|request|model|migration|command` commands only when needed.
- After changing an exposed signature or `framework:` annotation, run
  `framework generate` and then `bun run generate`.
- Never hand-edit `app/bootstrap/framework_generated.go`,
  `app/desktop/framework_generated.go`, `app/mcp/framework_generated.go`,
  `framework_bindings_generated.go`, or `src/mygo.ts` by hand.
- Keep MCP exposure explicit. A `framework:command` marker alone must not create
  an MCP tool; every MCP marker declares readonly and destructive effects.

## Preserve trust boundaries

- Return safe application problems to UI and MCP callers. Log unexpected causes
  once without request bodies, authorization values, secrets, or database paths.
- Store secrets only through the OS credential adapter. Do not add a plaintext
  fallback.
- Keep UI calls on MyGo bindings. Do not add a local web server for ordinary UI
  requests or a second file watcher around MyGo.
- Close the desktop app before running `go run ./cmd/appctl db status|up`.
