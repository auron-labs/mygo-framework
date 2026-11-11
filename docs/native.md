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
Preferences live in a [reactive cell](#reactive-state) and are saved 300 ms
after the last change. Closing the window, including during application quit,
first saves anything still pending. If a save fails, the window shows an error
and stays open so the user can try again.

```sh
go test ./...
CGO_ENABLED=0 go test ./...
go tool mygo vet .
```

The headless tests cover the preference controls, theme changes, database
creation and migration, saving without closing the window, persistence across
restarts, validation, and visible save errors. Check resizing, focus, keyboard
traversal, and light/dark appearance in the actual window on your target
platform.

The same example is available from this checkout:

```sh
go run ./examples/native
go test ./examples/native
```

Its preference directory is `mygo-framework-native` under `os.UserConfigDir()`.

## Background jobs

`github.com/auron-labs/mygo-framework/jobs` runs named, cancellable jobs with
progress. It has no UI dependency, so the web starter can import it too.

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

The starter's import flow runs `sampleImport` in `importjob.go` on a folder,
renders `components.Progress` from `Status.Fraction()` and offers **Cancel
import**; replace the function with real work. See
[Importing a folder](#importing-a-folder) for how the folder is chosen.

## Importing a folder

A native window accepts a folder dragged from Finder, Explorer or a Linux file
manager. MyGo reports the dropped paths to the element that asks for them:

```go
zone := ui.Column(c).FillWidth().Padding(16).Border(1, t.Border)
if zone.FileDragOver() {
	zone.Border(2, t.Accent) // files are over the zone and it would take them
}
if files := zone.DroppedFiles(); len(files) > 0 { // absolute paths, since the last frame
	startImport(files)
}
```

`Element.DroppedFiles` and `Element.FileDragOver` work on Linux, Windows and
macOS. A window can also take files anywhere with `Window.OnFileDrop`, which
gets the files no element took. A drop is a list of files and folders, so
check `os.Stat(path)` before treating one as a folder.

The starter wires both entry points to the same job and to the persisted last
import folder (`native.import_folder`, saved automatically):

- the drop zone in `importjob.go` takes the first dropped folder and says so
  when only files were dropped;
- **Choose folder...** calls `mygo.Dialog.Open(OpenDialogOptions{Directory:
  true})`. The call blocks until the dialog closes, so it runs in a goroutine
  and returns its result through `Window.Update`;
- **Start import** re-imports the remembered folder, or opens the picker when
  none was chosen.

In tests, `ui.Tester.DropFiles` delivers the same events as a real drop.

## PDF reports

A native `ui.View` cannot be exported to PDF: `Page.PrintToPDF` only prints web
pages. `github.com/auron-labs/mygo-framework/pdfreport` lays out report PDFs directly instead. It is
pure Go (builds with `CGO_ENABLED=0`), wraps
[`signintech/gopdf`](https://github.com/signintech/gopdf), and is imported by
native projects only. Its embedded Go fonts cover Latin scripts; embed another
font with gopdf directly for other scripts.

```go
r, err := pdfreport.New(pdfreport.Options{Title: "Weekly report", Footer: "Acme"})
r.Heading("Weekly report", 20)
r.Paragraph("Text wraps to the page width.")
r.Table([]pdfreport.Column{{Title: "Night", Width: 2}, {Title: "Hours", Width: 1, Right: true}}, rows)
r.LineChart(pdfreport.LineChart{Title: "Hours", Series: []pdfreport.Series{{Name: "Hours", Points: points}}})
data, err := r.Bytes() // write it with os.WriteFile or mygo.Dialog.Save's path
```

A4 portrait, a header with the title and a footer with the text and page number
on every page. Tables and paragraphs continue on the next page, and a table
repeats its header row there; a chart stays whole on one page. Content errors
(ragged table rows, an empty chart, non-finite values) are reported by `Bytes`.
Not included: images, cell wrapping inside tables, other page sizes, pie or bar
charts.

`go run ./examples/report -o report.pdf` writes a one-page report with a table
and a line chart.

## Large tables and long lists

Onevra's `components.TableVirtualized` and `components.List` build only the rows
on screen, so the cost of a frame does not grow with the number of rows. Run
`go test -bench . ./examples/tablebench` to measure them on your hardware.

- Your `cell` and `row` callbacks run during layout and must index into data
  already in memory. Sorting, filtering or querying 100,000 rows belongs in a
  job, with the view reading the result.
- A table with many columns or heavy cells (images, rich text) costs
  proportionally more per visible row.

## Time-series charts

`github.com/auron-labs/mygo-framework/timechart` draws continuous time series: a clock-aligned time
(or plain numeric) x-axis, line, step and min/max band series, point and
interval event markers, and a hover crosshair with a readout. Wheel or pinch
zooms around the cursor, dragging pans, and a double click resets; with the
plot focused, the arrow keys pan, `=` and `-` zoom and Home resets. It builds
on MyGo's `ui/chart` scales and `Draw` callbacks and depends on nothing else
in this framework.

```go
rng := timechart.NewRange(start, end) // Unix seconds; bounds the zoom and pan
flow := &timechart.Chart{
	Range: rng, Label: "Airflow", HideXAxis: true,
	Series: []timechart.Series{{ID: "flow", Label: "Airflow", Provider: provider}},
	Events: []timechart.Event{{Start: t0, End: t1, Label: "Apnea"}, {Start: t2, Label: "Arousal"}},
}
spo2 := &timechart.Chart{Range: rng, Label: "Oxygen", YMin: 80, YMax: 100, Series: ...} // linked
// In the view, every frame:
flow.Build(c)
spo2.Build(c)
// When discarding the charts:
flow.Close()
```

- **Linking.** Charts built with the same `*Range` zoom, pan and show the
  crosshair together. Each row's y-axis column has the same width, so stacked
  plots line up; set `HideXAxis` on all but the last row. `Range.Set`,
  `ZoomAt`, `Pan` and `Reset` move the range from code, and `Crosshair` reads
  the shared position. The range never leaves its bounds and never narrows
  below `Range.MinSpan` (a millionth of the bounds by default).
- **Data.** A `Provider` receives `(ctx, from, to, pixelWidth)` and returns
  ascending `Points` decimated for that width. The chart calls it off the UI
  thread for a window 25% wider than the view on each side, keeps drawing the
  previous points while it runs, drops answers for a view it has since left,
  and requests a frame when an answer arrives. It asks again only when the
  view leaves the fetched window or zooms in far enough to make it coarse.
  Providers should return when `ctx` is canceled and log their own errors: a
  failed window leaves the previous points on screen and is not retried until
  the view moves out of it. `Chart.Loaded` reports when the visible window has
  arrived, which is useful in tests.
- **In-memory samples.** `timechart.Decimate(xs, ys, from, to, pixelWidth)`
  finds the window by binary search and keeps the lowest and highest sample of
  each pixel column in time order, plus one neighbour past each edge. The
  result is at most `2*pixelWidth+2` points whatever the zoom, and narrow
  spikes survive. Use it as the provider for slices; a database-backed
  provider returns the same shape from a grouped query.
- **Series kinds.** `Line` joins points, `Step` holds each value until the
  next, and `Band` fills between the lowest and highest point of each pixel
  column, so the same decimated points serve any kind. Colors default to theme
  tokens by series order; set `Series.Color` or `Event.Color` to override.
- **Axes.** Time ticks are clock-aligned in `Chart.Location` (default local
  time), from milliseconds to days, with at most one label per 90 DIPs;
  midnight is labelled with the date. `Numeric` switches the x-axis to plain
  numbers. The y-axis fits the visible data unless `YMin`/`YMax` fix it;
  `FormatY` formats its labels and the readout.

`examples/timechart` stacks three linked charts over a synthetic eight-hour,
25 Hz night (720,000 samples per channel). Run it with
`go run ./examples/timechart`, or serve it to an MCP client with
`go run ./examples/timechart -mcp 127.0.0.1:8080`.
`go test -bench . ./examples/timechart` measures a full frame and the provider
work done off the UI thread.

## Translations

User-visible strings come from MyGo i18n catalogs, so a language can be added
without touching the views. `locales/en.json` is the only shipped catalog and
`mygo.json` enables it (`"i18n": {"dir": "locales", "source": "en"}`).
`locales/catalog_gen.go` is generated from the catalogs by `go tool mygo
generate`, and `framework dev` and `framework build` refresh it; commit it so
`go build` and `go test` work without the CLI.

```go
components.Checkbox(c, &s.prefs.Compact, locales.CompactSpacing())
components.Text(c, locales.ImportFailed(status.Err))
```

Views call the typed function per message, so text follows the language as it
changes. To add a language, copy `en.json` to `<tag>.json` (for example
`de.json`), translate the values, and run `go tool mygo generate`; the app
shows the catalog language that best matches the user's system languages. The
starter has no language picker. The application name and other product names
are not translated, and `framework make feature` writes plain English strings
that you move into the catalog when the feature is stable.

## Signed updates

`mygo.Updater` installs signed updates, and it is off until the build is
configured for it. Add an `updates` section with your public key and a release
location to `mygo.json`:

```json
{ "updates": { "publicKey": "<contents of mygo-update.pub>", "github": "you/my-app" } }
```

Create the key pair with `go tool mygo keygen` and keep `mygo-update.key` out of
the repository, in your CI secrets; `mygo build` signs the update files with it
through `MYGO_UPDATER_PRIVATE_KEY`. Without an `updates` section
`mygo.Updater.Enabled()` is false, so the starter never checks for updates and
contacts no server, and the screen shows nothing about updates. With one, the
starter checks once in the background at launch (`updates.go`). A failed check,
such as being offline, is logged and ignored. When a newer version is found, the
screen offers **Install update**, shows download progress, and then **Restart to
finish updating**. MyGo verifies the archive's signature against the public key
built into the app. Web projects call `mygo.Updater` from their own Go service
in the same way; the build configuration is `updates` in `mygo.config.ts`.

Updating a package-manager install, an AppImage, or an app directory the user
cannot write is left to MyGo, which reports it as a failed install.

## Profiles

`github.com/auron-labs/mygo-framework/profiles` gives each named profile its own SQLite file,
`<data dir>/profiles/<name>.db`, opened with the same `database.Open` and
migrations as the preferences database. The helper is not wired into the
starter's window.

```go
db, err := profiles.Open(ctx, dataDir, prefs.Profile, migrations.FS())
```

Nothing in the schema or any query has to know about profiles: the same
migrations and queries serve every profile, and deleting or exporting a profile
is a single-file operation. A query cannot span profiles; that needs opening
each file.

The active profile name stays in the preferences database as the existing
`native.profile` setting, so the profile choice is shared across profiles.
`profiles.Validate` (also the setting's validator) accepts 1 to 64 ASCII
letters, digits, spaces, underscores and hyphens, starting and ending with a
letter or digit, and rejects names Windows reserves (`CON`, `NUL`, `COM1`...).
Dots and path separators are not allowed, so a name cannot leave the profiles
directory. Names that differ only in case share a file, as they would on
macOS and Windows file systems. Switching profiles means closing the current
profile database and opening another; services holding the old handle must be
rebuilt.

## Reporting errors

The Go side reports unexpected errors and panics to Sentry when `SENTRY_DSN` is
set in the environment the app is launched from. With no DSN the SDK is not
initialised and nothing is sent. Only errors are reported: no tracing, profiling,
session replay or breadcrumbs. `github.com/auron-labs/mygo-framework/errorreport` wraps
`sentry-go` and is shared with the web starter.

```sh
SENTRY_DSN=https://<key>@<host>/<project> go run .
```

Reported: errors returned from `run` (startup failures), failed and panicked
background jobs (`jobs.Manager`), a failed preference save when closing, and a
panic that reaches `main`. Cancelled jobs are not reported. Events are flushed
before the process exits, and the home directory is replaced with `~` in error
text. Logging is unchanged.

## Reactive state

`github.com/auron-labs/mygo-framework/state` holds values that views, background goroutines and
storage share, and tells subscribers when they change. Both starters include
it, and the native starter keeps its preferences in a cell. It uses only the
standard library.

A `Cell` holds one value:

```go
prefs := state.NewCell(loaded)

p := prefs.Get()
prefs.Update(func(p *preferences) { p.Compact = true })
prefs.Set(next)
stop := prefs.Subscribe(func(p preferences) { log.Print(p.Theme) })
```

`Update` edits a copy and commits it; `Set` replaces the value. `Subscribe`
calls back after later changes, not with the current value, and returns a
function that stops it.

Use a cell for values that something besides the view reads or writes, such as
preferences, the results of background work, or anything you save. Keep values
only the view uses, such as a status message or a text draft, in plain fields.
Treat stored values as immutable and replace a slice or map rather than
changing it in place. `Update` runs its function while holding the cell's
lock, so the function must not call the same cell; if it panics, nothing is
committed.

Read the cell once per build. Bind controls to local copies and commit in
`OnChange`:

```go
p := s.prefs.Get()
compact := p.Compact
components.Checkbox(c, &compact, "Compact spacing").OnChange(func() {
	s.prefs.Update(func(p *preferences) { p.Compact = compact })
})
```

To redraw the window after any change, subscribe once the window exists and
stop when it closes. `Invalidate` is safe from any goroutine, and calls before
the next frame make one frame.

```go
win.OnClosed(s.prefs.Subscribe(func(preferences) { win.Invalidate() }))
```

**Threading.** Any goroutine may read or write a cell. Subscribers run on the
goroutine that wrote, outside the cell's lock, and must not block. The
callbacks of one cell never run at the same time and never see a value older
than one they have already seen. Rapid changes can arrive as one call, and a
write can return before its subscribers run when another goroutine is already
delivering. A subscriber may write to the cell it listens to. Call MyGo APIs
that are not documented as safe from other goroutines, such as
`mygo.Theme.SetSource`, through `mygo.RunOnMain`.

**Derived values.** `state.Select` derives a value from a cell:

```go
themes := state.Select(prefs, func(p preferences) string { return p.Theme })
themes.Subscribe(func(name string) { mygo.RunOnMain(func() { /* apply name */ }) })
```

Its `Get` recomputes only after the cell changes, and its `Subscribe` calls
back only when the selected value changes, so the starter's theme listener
ignores the other preferences. `SelectFunc` takes an equality function for
values that `==` cannot compare. A derived value can be selected from again,
and needs no cleanup.

**Autosave.** `state.Autosave(prefs, 300*time.Millisecond, save)` calls `save`
with the latest value once it has stayed unchanged for 300 ms. The value at the
time of the call counts as saved. Saves run on their own goroutine and never
overlap; a change during a save causes one more save. A failed save is retried
on the next change or `Flush`. `Err()` holds the latest failure, or nil after a
successful save, and can be subscribed to like a cell. `Flush(ctx)` saves now
and waits; if `ctx` ends first, the save keeps running. `Close(ctx)` saves
anything pending and stops the saver; if `ctx` ends first, it cancels the save.
Call it after the event loop stops and before closing the storage that `save`
writes to.

## Desktop integration

Both starters run one instance per user: starting the app again brings the
running window to the front instead of opening the database a second time.
The native starter skips the lock when you pass `-data-dir`, so separate data
directories can run side by side.

The native starter also shows:

- **A menu bar.** `desktop.go` builds it from `mygo.NewMenu`: **File** has
  **Choose folder...** (`Ctrl/Cmd+O`) and **Start import** (`Ctrl/Cmd+I`),
  then the standard Edit and Window menus. Menu items call the same methods as
  the buttons.
- **A notification** when an import finishes or fails while the window is not
  in front. Clicking it brings the window forward. Nothing is shown where the
  system has no notification service or the user has declined them.
- **A deep link**, `<app-slug>://import?folder=/path`. It selects the folder and
  brings the window forward; the user still starts the import. The scheme is
  listed under `urlSchemes` in `mygo.json`, which registers it when the app is
  installed. A packaged build registered another way can call
  `mygo.App.RegisterURLScheme`.

For an app that opens files, list the types in `fileAssociations` in
`mygo.json` and handle them with `mygo.App.OnOpenFile`, registered before
`Run`. With the single instance lock, files opened while the app runs reach
the first instance.

## Platform support

The native starter builds with `CGO_ENABLED=0` for Linux, macOS and Windows on
amd64 and arm64. CI runs the Go tests on Linux, macOS and Windows, and builds
the examples and freshly generated projects on macOS and Windows.

CI does not open a window, so check window behaviour, dialogs, dark mode, file
drops and packaging on each platform you ship to. Signing and notarisation need
your own certificates.

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

## Adding a feature

Run `framework make feature Bookmark` in a native project. It writes three
files and changes nothing else, so there are no adapters to regenerate:

```text
bookmark.go                                          model, store and view
bookmark_test.go                                     store and view tests
database/migrations/<sequence>_create_bookmark_records.sql
```

`BookmarkStore` (GORM over the shared pool) creates and lists records, and
`bookmarkView` is a Onevra view with a text box, an **Add** button and the
record list; the button runs the store call from its click callback, outside
view construction. The files are the starting point for your own code. Wire the
view in by creating `newBookmarkStore(db.GORM)`, loading it, and calling
`bookmarkView.view` from your screen. Existing files are never overwritten. A
web project (no `mygo.json`) still gets the web feature scaffold.

## Reusing the packages in a separate application

Generated applications import the framework packages from
`github.com/auron-labs/mygo-framework`, so they are versioned with Go modules
and update with `go get`:

```sh
go get github.com/auron-labs/mygo-framework@latest
go mod tidy
```

`framework new` pins the release the CLI was installed from; a CLI built from a
checkout uses `main`. To develop an application against a local framework
checkout, create it with `framework new NAME --framework ../mygo-framework`,
which adds a `replace` directive. Import `.../settings` and a store such as
`.../settings/sqlite` directly in any other application.

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
Framework web/MCP adapter generation targets the web starter. `framework make
feature NAME` detects a native project by its `mygo.json` and writes a native
feature instead; see [Adding a feature](#adding-a-feature).

Native dependencies still need their target-platform review. In particular,
MyGo's packager builds with cgo disabled; an application using a cgo-backed
storage engine must use a cgo-enabled Go build or provide a separate packaging
step. The example itself adds no cgo dependency.
