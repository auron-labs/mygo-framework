# __PROJECT_TITLE__

A native MyGo application on Onevra's sidebar shell, with page routing, an
icon-only rail for narrow windows and typed SQLite preferences.

Onevra is maintained in the framework module and uses its release version.
Import UI packages from `github.com/auron-labs/mygo-framework/onevra/...`; see
[the Onevra guide](https://github.com/auron-labs/mygo-framework/blob/main/onevra/README.md).

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
No JavaScript runtime or frontend is required. When `framework dev` restarts
the app after a code change, it returns to the page that was showing.

```sh
go test ./...
CGO_ENABLED=0 go test ./...
go tool mygo vet .
golangci-lint run
```

`.github/workflows/ci.yml` runs these on Linux and Windows for every push and
pull request, and fails when `go tool mygo generate` would change a file.

Open **Settings** in the sidebar footer, toggle **Compact spacing**, choose units
and a theme, close the window, and reopen it. Preferences live in
`__PROJECT_SLUG__/app.db`, under `~/Library/Application Support` on macOS,
`%AppData%` on Windows and `$XDG_CONFIG_HOME` (usually `~/.config`) on Linux.
On iOS and Android it is in MyGo's directory for the app inside the sandbox.
Run with `-data-dir DIR` on desktop to use another directory.
Startup applies the migrations in `database/migrations` (add yours after
`00001`). `preferences.go` defines the typed settings: compact spacing, time
zone, units, theme, profile name, and last import folder. A save failure
keeps the window open with a visible error. The database is closed after
the native event loop stops. The shell remembers window size and position under `main`; the first launch
opens at 960x640 with a minimum of 520x480.
It opens after its first frame is drawn, with the close handler already installed.

On the **Import** page, drop a folder on the import area, or choose one with
**Choose folder...**, to
run a sample background job from `importjob.go` with a progress bar and a
**Cancel import** button. The last folder is remembered; **Start import** runs
it again. On phones, where MyGo's folder picker does not work on Android, the
drop zone and the remembered folder are hidden and **Choose files...** imports
files from the document picker instead; the picker's temporary copies are
removed when the import ends. The job service in `jobs/` runs
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
Ask it to list sessions, inspect the view, click **Go to Settings**, toggle
**Toggle compact spacing** and capture a screenshot. The harness reuses the application's view and Onevra
theme with fresh in-memory state. It never loads or saves your preferences.
Stop it with Ctrl+C; restart it to reset the fixture.

The listener requires an explicit loopback IP and port. It has no authentication;
keep it local and use test state and services when extending `nativeFixture` in
`mcp.go`. Headless fixtures cannot control a running desktop window, OS dialogs,
or webviews. See the [MyGo MCP agent guide](https://github.com/auron-labs/mygo-framework/blob/main/mygomcp/docs/AGENT_GUIDE.md)
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
nothing is initialised or sent. The home directory is replaced with `~` in
error text; on iOS and Android the app data directory becomes `<app data>`.
Phones have no environment, so compile the DSN into phone builds with
`MYGO_LDFLAGS`, as the mobile guide describes.

## Pages, sidebar and rail

`sidebar.go` builds the shell's regions: the navigation (`left`), the footer
(`footer`: update prompt, account card, **Settings**, version), the top bar
title (`contentTitle`) and the page area (`content`). `pages.go` holds the route
table. A route is a navigation id, a title, an icon, an optional count badge
and a page view. `page_home.go`, `page_import.go` and `page_settings.go` are
the pages; each is `func(*ui.Context, *screen)`, reads the shared state and does
not call another page. The selected id is `screen.kept.page`, kept with
`mygo.KeepState`, so a `framework dev` restart or a phone's relaunch returns to
it. An unknown id shows Home.

To add a page: add an id and a `route` in `pages.go`, a `page_NAME.go` with its
view, and its strings in `locales/en.json`. Set `inNav` to list it in the
navigation; otherwise link to it with `screen.navigate`.

Onevra's sidebar shell has no breakpoint, so `layout.go` adds one. Below
`railBreakpoint` (720) `syncRail` shrinks the sidebar to `railWidth` with
`SetLeftWidth` and the navigation shows icons with tooltips. It remembers the
width the user dragged the sidebar to and restores it when the window widens.
The window minimum is narrower than the breakpoint so the rail is reachable.

`screen.view` lays out the same regions without a window; the tests and the MCP
fixture use it with a stand-in for the shell's sidebar width.

### On phones

Run it with `framework dev -platform ios|android`; [the mobile guide](https://github.com/auron-labs/mygo-framework/blob/main/docs/mobile.md)
covers prerequisites, signing and publishing.

The same route table drives the phone layout: `inNav` routes become a bottom tab
bar, and the other routes (Settings) open as pushed screens from a top-bar
button, with a back button. The sidebar, rail and its update prompt and account
card are desktop-only. Pages move through a `ui.Router` (`screen.router`):
`navigate` replaces the page for a tab and pushes any other route, so Android's
back and iOS's swipe from the left edge return to the page before. The router's
page is the one MyGo keeps. Tests run the phone layout with `screen.onPhone` and
`Tester.SetInsets`; see `TestPhoneNavigationByFingerAndBack`.

Compose Onevra components in the page views. Keep disk and network work
outside builders. Web/MCP adapter generation is
intended for the web starter.
