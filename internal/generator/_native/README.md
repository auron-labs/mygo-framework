# __PROJECT_TITLE__

A native MyGo application with Onevra components and typed bbolt preferences.
Requires Go 1.27.3 and the native libraries required by MyGo for your platform.

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

Extend the retained screen state and compose Onevra components in its view.
Keep disk and network work outside builders. Framework feature scaffolding and
web/MCP adapter generation are intended for the web starter.
