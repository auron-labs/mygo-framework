# Native Go applications

The framework's packages are usable from MyGo's native `ui.View` as well as
from a webview. The native example stores typed settings in the same SQLite database
package the web starter uses (WAL, foreign keys, embedded Goose migrations),
without the reference app's service MCP server or TypeScript frontend.

The native starter includes a pinned `auron-labs/mygo-mcp` dependency.

Create a native project with Go 1.27.3 or newer and the framework CLI installed:

```sh
framework new my-native-app --native
cd my-native-app
go mod download
framework doctor
framework dev
```

The starter pins MyGo v0.8.3 and Onevra v0.2026.18. It includes a Onevra plain
shell, retained Go view state, and typed settings in SQLite.
The shell stays hidden until its first frame is drawn, so it opens with its
content ready while the preference-saving close handler is already installed.
`framework build` packages it through the pinned `go tool mygo`; no Bun or
TypeScript frontend is required. `go run .` also runs the application.

Toggle **Compact spacing**, choose units and a theme, close the window, and
reopen it. The preferences are stored in `my-native-app/app.db` under
`os.UserConfigDir()`; pass `-data-dir DIR` to use another directory. Startup
opens the database and applies pending migrations from `database/migrations`
before the window is created. Add your own migrations there, numbered after
`00001`. The database is closed after the event loop stops.

The typed keys in `preferences.go` cover compact spacing, time zone, units,
theme, profile name, and last import folder. The window edits compact spacing,
units, and theme; the theme applies immediately. Time zone, profile name, and
the import folder are validated and persisted, ready for your own views.
UI builders only change retained Go state. The close handler saves preferences
before allowing the window to close, including during application quit. If a
save fails, it keeps the window open and displays an error so the user can retry.

```sh
go test ./...
CGO_ENABLED=0 go test ./...
go tool mygo vet .
```

The headless tests cover the preference controls, database creation and
migration, persistence across restarts, validation, and visible save errors. Check resizing, focus, keyboard traversal, and light/dark appearance in
the actual window on your target platform.

The same example is available from this checkout:

```sh
go run ./examples/native
go test ./examples/native
```

Its preference directory is `mygo-framework-native` under `os.UserConfigDir()`.

## Background jobs

`mygo-framework/jobs` runs named, cancellable jobs with progress. It is copied
into native projects and has no UI dependency, so the web starter can import it
too.

```go
manager := jobs.New(ctx) // jobs stop when ctx is canceled
err := manager.Start("export", func(ctx context.Context, report jobs.Report) error {
	for i := 1; i <= total; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		report(i, total, "Exported "+name(i))
	}
	return nil
})
manager.Cancel("export")
status, ok := manager.Status("export")
off := manager.Subscribe(func(s jobs.Status) { /* every change, including the final one */ })
err = manager.Shutdown(shutdownCtx)
```

- Only one job per name runs at a time. Starting a name that is still running
  returns `jobs.ErrRunning` and leaves the running job untouched; cancel it or
  wait for it first. A finished job's name can be started again.
- A job ends as `Done` (returned nil), `Canceled` (returned an error wrapping
  `context.Canceled`), or `Failed` (any other error, or a panic, which is
  recovered and reported in `Status.Err`). A job that returns nil after being
  canceled is `Done`.
- `Subscribe` callbacks run on the goroutine that changed the status and must
  not block. The first event for a job is always `Running`, and the last is its
  terminal state. MyGo's `Event` and `Channel` deliver to web pages only, so a
  native view subscribes with a callback: the starter calls `Window.Invalidate`
  (safe from any goroutine) and reads `Status` while building the view. A web
  page can forward the same callback to a `mygo.Event` or a bound method's
  `*mygo.Channel`.
- `Shutdown` rejects new jobs, cancels running ones, and waits for them within
  its context. The native starter calls it with a five-second deadline after
  the event loop stops and before the database closes. Jobs must return
  promptly when their context is canceled.

The starter's import flow runs `sampleImport` in `importjob.go`, renders
`components.Progress` from `Status.Fraction()` and offers **Cancel import**;
replace the function with real work.

## Inspect native views through MCP

Generated native projects include a headless MCP harness for their real view:

```sh
go run . -mcp 127.0.0.1:8080
```

From this checkout, use `go run ./examples/native -mcp 127.0.0.1:8080`.
Connect an MCP client using Streamable HTTP to `http://127.0.0.1:8080/mcp`.
Ask it to list sessions, inspect the view, click **Toggle compact spacing**,
and capture a screenshot. Stop the harness with Ctrl+C.

The harness reuses the desktop view and Onevra theme with independent in-memory
state; it does not open a window or load or save preferences. Restart to reset it.
Extend `nativeFixture` in `mcp.go` with test state and services as your view grows.
Only an explicit loopback IP and port are accepted. The endpoint has no
authentication, so keep it local.

MyGo MCP operates on isolated native fixtures. It cannot control running windows,
OS dialogs, or webviews. The web starter's authenticated service MCP server is
separate. See the [MyGo MCP agent guide](https://github.com/auron-labs/mygo-mcp/blob/9f095775017aafe64c22e934d2178fa7b6657209/docs/AGENT_GUIDE.md)
for tools and observation rules. The included MCP integration test covers
discovery, inspection, checkbox interaction, screenshots, fixture isolation,
and shutdown over a real loopback connection.

## Reusing the packages in a separate application

This repository is source-first and currently declares `module mygo-framework`.
It is not directly importable with `go get github.com/auron-labs/mygo-framework`.
An external native consumer can use an authenticated sibling checkout:

```sh
gh repo clone auron-labs/mygo-framework ../mygo-framework
go mod edit -require=mygo-framework@v0.0.0
go mod edit -replace=mygo-framework=../mygo-framework
```

The replacement path is relative to the consumer's `go.mod`; adjust it for
nested `source/` layouts. Pin the framework checkout to a reviewed commit
for reproducible builds. Import `mygo-framework/settings` and a store such as
`mygo-framework/settings/sqlite` directly. Alternatively,
copy the source-first packages into the application and change import paths
as with a generated application.

Use MyGo's native `ui.View` for window content and native `mygo.json`
configuration for packaging. Onevra supplies native components and shells;
keep the application's MyGo version compatible with the selected Onevra
version. Go selects the higher MyGo requirement, so verify framework packages
against that version when upgrading.

## Current CLI boundary

`framework new` creates the web starter; `framework new NAME --native` creates
the native starter. For native projects identified by `mygo.json`, `framework
dev`, `build`, and `doctor` skip web adapter generation and delegate through
`go tool mygo`. Web projects retain generation and `bun run mygo` delegation.

The native starter intentionally uses direct Go calls and typed settings.
`framework make feature` and framework web/MCP adapter generation target the
web starter; they are not part of the native starter workflow.

Native dependencies still need their target-platform review. In particular,
MyGo's packager builds with cgo disabled; an application using a cgo-backed
storage engine must use a cgo-enabled Go build or provide a separate packaging
step. The example itself adds no cgo dependency.
