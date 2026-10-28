# __PROJECT_TITLE__

A native MyGo application with Onevra components and typed bbolt preferences.
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

Toggle **Compact spacing**, close the window, and reopen it. Preferences live
in `__PROJECT_SLUG__/preferences.db` under `os.UserConfigDir()`. A save failure
keeps the window open with a visible error. The settings store is closed after
the native event loop stops. The shell remembers window bounds under `main`.

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

Extend the retained screen state and compose Onevra components in its view.
Keep disk and network work outside builders. Framework feature scaffolding and
web/MCP adapter generation are intended for the web starter.
