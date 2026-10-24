# Native Go applications

The framework's packages are usable from MyGo's native `ui.View` as well as
from a webview. The native example uses typed settings and the owning bbolt
store, without starting the reference app's SQLite database, MCP server, or
TypeScript frontend.

From this checkout:

```sh
go run ./examples/native
go test ./examples/native
```

Toggle **Compact spacing**, close the window and reopen it. The preference
is stored in `mygo-framework-native/preferences.db` under `os.UserConfigDir()`.
The bbolt store holds a process lock and is closed after the native event loop
stops. UI builders only change retained Go state; disk writes happen outside
the builder. Real apps should save documents during their pre-quit flow,
keeping the app open if a save fails, rather than adopting this small
preference-only example's after-exit save.

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
configuration for packaging. Onevra can supply native components and shells;
keep the application's MyGo version compatible with the selected Onevra
version. Go selects the higher MyGo requirement, so verify framework packages
against that version when upgrading.

## Current CLI boundary

`framework new` creates the web starter. `framework dev`, `build`, and `doctor`
run the framework generator and delegate through `bun run mygo`. These commands
are not a native-project workflow yet. Run native apps with `go run` and use
the matching MyGo Go CLI directly for native packaging; do not add a dummy
TypeScript frontend just to satisfy the framework CLI.

Native dependencies still need their target-platform review. In particular,
MyGo's packager builds with cgo disabled; an application using a cgo-backed
storage engine must use a cgo-enabled Go build or provide a separate packaging
step. The example itself adds no cgo dependency.
