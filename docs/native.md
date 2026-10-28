# Native Go applications

The framework's packages are usable from MyGo's native `ui.View` as well as
from a webview. The native example uses typed settings and the owning bbolt
store, without starting the reference app's SQLite database, MCP server, or
TypeScript frontend.

The native starter includes a pinned `auron-labs/mygo-mcp` dependency.

Create a native project with Go 1.27.3 or newer and the framework CLI installed:

```sh
framework new my-native-app --native
cd my-native-app
go mod download
framework doctor
framework dev
```

The starter pins MyGo v0.7.1 and Onevra v0.2026.16. It includes a Onevra plain
shell, retained Go view state, typed settings, and a bbolt preferences store.
`framework build` packages it through the pinned `go tool mygo`; no Bun or
TypeScript frontend is required. `go run .` also runs the application.

Toggle **Compact spacing**, close the window, and reopen it. The preference
is stored in `my-native-app/preferences.db` under `os.UserConfigDir()`.
The bbolt store holds a process lock and is closed after the event loop stops.
UI builders only change retained Go state. The close handler saves preferences
before allowing the window to close, including during application quit. If a
save fails, it keeps the window open and displays an error so the user can retry.

```sh
go test ./...
CGO_ENABLED=0 go test ./...
go tool mygo vet .
```

The headless tests cover the preference control, persistence, and visible save
errors. Check resizing, focus, keyboard traversal, and light/dark appearance in
the actual window on your target platform.

The same example is available from this checkout:

```sh
go run ./examples/native
go test ./examples/native
```

Its preference directory is `mygo-framework-native` under `os.UserConfigDir()`.

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
for reproducible builds. Import `mygo-framework/settings` and
`mygo-framework/settings/bbolt` directly, as the example does. Alternatively,
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
