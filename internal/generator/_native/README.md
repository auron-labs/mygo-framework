# __PROJECT_TITLE__

A native MyGo application with Onevra components and typed SQLite preferences.
Requires Go 1.27.3 and the native libraries required by MyGo for your platform.

MyGo MCP is included as a pinned dependency.

```sh
go mod download
framework dev
framework doctor
framework build
```

The framework delegates to the pinned `go tool mygo`. You can also use
`go run .`, `go tool mygo dev`, and `go tool mygo build` directly.
No JavaScript runtime or frontend is required.

```sh
go test ./...
CGO_ENABLED=0 go test ./...
go tool mygo vet .
```

Toggle **Compact spacing**, choose units and a theme, close the window, and
reopen it. Preferences live in `__PROJECT_SLUG__/app.db` under
`os.UserConfigDir()`; run with `-data-dir DIR` to use another directory.
Startup applies the migrations in `database/migrations` (add yours after
`00001`). `preferences.go` defines the typed settings: compact spacing, time
zone, units, theme, profile name, and last import folder. A save failure
keeps the window open with a visible error. The database is closed after
the native event loop stops. The shell remembers window bounds under `main`.
It opens after its first frame is drawn, with the close handler already installed.

Drop a folder on the import area, or choose one with **Choose folder...**, to
run a sample background job from `importjob.go` with a progress bar and a
**Cancel import** button. The last folder is remembered; **Start import** runs
it again. The job service in `jobs/` runs
one job per name, recovers panics as job errors, and stops running jobs when
the app quits. Replace `sampleImport` with your own work.

`pdfreport/` writes report PDFs with text, tables, simple line charts and a page
header and footer, since a native window cannot be exported to PDF. It is pure Go.

## Inspect and interact through MCP

Start the headless UI harness instead of a desktop window:

```sh
go run . -mcp 127.0.0.1:8080
```

Connect an MCP client using Streamable HTTP to `http://127.0.0.1:8080/mcp`.
Ask it to list sessions, inspect the view, click **Toggle compact spacing**,
and capture a screenshot. The harness reuses the application's view and Onevra
theme with fresh in-memory state. It never loads or saves your preferences.
Stop it with Ctrl+C; restart it to reset the fixture.

The listener requires an explicit loopback IP and port. It has no authentication;
keep it local and use test state and services when extending `nativeFixture` in
`mcp.go`. Headless fixtures cannot control a running desktop window, OS dialogs,
or webviews. See the [MyGo MCP agent guide](https://github.com/auron-labs/mygo-mcp/blob/9f095775017aafe64c22e934d2178fa7b6657209/docs/AGENT_GUIDE.md)
for tool inputs and observation rules.

User-visible strings live in `locales/en.json`; views call the generated
`locales` functions, and `go tool mygo generate` (run by `framework dev` and
`framework build`) refreshes `locales/catalog_gen.go`. To add a language, copy
`en.json` to `<tag>.json` and translate it.

Signed updates are off until `mygo.json` has an `updates` section (public key and
release location); then the app checks once at launch and offers **Install
update** when a newer version exists. Keep the private signing key out of the
repository. `profiles/` keeps one SQLite file per named profile.

`framework make feature NAME` adds a native view, store, migration and test.

Set `SENTRY_DSN` to send unexpected errors and panics to Sentry; without it
nothing is initialised or sent. The home directory is replaced with `~` in error text.

Extend the retained screen state and compose Onevra components in its view.
Keep disk and network work outside builders. Web/MCP adapter generation is
intended for the web starter.
